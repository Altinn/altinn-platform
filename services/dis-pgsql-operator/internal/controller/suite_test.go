package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/network"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
)

var (
	ctx        context.Context
	cancel     context.CancelFunc
	testEnv    *envtest.Environment
	cfg        *rest.Config
	k8sClient  client.Client
	k8sManager ctrl.Manager
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	if os.Getenv("DISPG_SKIP_ENVTEST") == "1" {
		Skip("envtest disabled via DISPG_SKIP_ENVTEST")
	}

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())
	scheme := runtime.NewScheme()
	var err error
	err = storagev1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = identityv1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = batchv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = networkv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = dbforpostgresqlv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	identityCRDPath := disIdentityCRDPath()
	crdPaths := []string{
		filepath.Join("..", "..", "config", "crd", "bases"),
		filepath.Join("..", "..", "bin", "aso-crds"),
	}
	if identityCRDPath != "" {
		crdPaths = append(crdPaths, identityCRDPath)
	}
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     crdPaths,
		Scheme:                scheme,
		ErrorIfCRDPathMissing: true,
	}
	Expect(ensureApplicationIdentityCRD(testEnv.CRDDirectoryPaths)).To(Succeed())

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sManager).NotTo(BeNil())

	// Build a fixed subnet catalog for tests.
	// A /24 can hold 16 /28 subnets, which matches production assumptions.
	testCatalog, err := network.NewSubnetCatalog(buildTestSubnets(16))

	Expect(err).NotTo(HaveOccurred())

	// Operator config for tests
	config := config.OperatorConfig{
		ResourceGroup:      "rg-dis-dev-network",
		DBVNetName:         "vnet-dis-dev-001",
		AKSVNetName:        "aks-vnet-dis-dev-001",
		SubscriptionId:     "my-subscription-id",
		TenantId:           "my-tenant-id",
		AKSResourceGroup:   "aks-vnet-rg",
		UserProvisionImage: "controller:latest",
	}
	err = (&DatabaseReconciler{
		Client:        k8sManager.GetClient(),
		Scheme:        k8sManager.GetScheme(),
		SubnetCatalog: testCatalog,
		Config:        config,
	}).SetupWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).NotTo(BeNil())

	go func() {
		defer GinkgoRecover()
		Expect(k8sManager.Start(ctx)).To(Succeed())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if testEnv == nil {
		return
	}
	if cancel != nil {
		cancel()
	}
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

func disIdentityCRDPath() string {
	monorepoPath := filepath.Join("..", "..", "..", "dis-identity-operator", "config", "crd", "bases")
	if info, err := os.Stat(monorepoPath); err == nil && info.IsDir() {
		return monorepoPath
	}
	downloadedPath := filepath.Join("..", "..", "bin", "dis-identity-crds")
	if info, err := os.Stat(downloadedPath); err == nil && info.IsDir() {
		return downloadedPath
	}
	return ""
}

func buildTestSubnets(count int) []network.SubnetInfo {
	out := make([]network.SubnetInfo, 0, count)
	for i := 0; i < count; i++ {
		thirdOctet := (i / 16) + 1
		fourthOctet := (i % 16) * 16
		out = append(out, network.SubnetInfo{
			Name: fmt.Sprintf("s%d", i+1),
			CIDR: fmt.Sprintf("10.100.%d.%d/28", thirdOctet, fourthOctet),
		})
	}
	return out
}

func ensureApplicationIdentityCRD(paths []string) error {
	const (
		targetGroup = "application.dis.altinn.cloud"
		targetKind  = "ApplicationIdentity"
	)

	for _, dir := range paths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("failed to read CRD directory %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			filePath := filepath.Join(dir, entry.Name())
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("open CRD file %s: %w", filePath, err)
			}
			decoder := yaml.NewYAMLOrJSONDecoder(file, 4096)
			for {
				var crd apiextensionsv1.CustomResourceDefinition
				if err := decoder.Decode(&crd); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					_ = file.Close()
					return fmt.Errorf("decode CRD file %s: %w", filePath, err)
				}
				if crd.Kind == "CustomResourceDefinition" &&
					crd.Spec.Group == targetGroup &&
					crd.Spec.Names.Kind == targetKind {
					_ = file.Close()
					return nil
				}
			}
			_ = file.Close()
		}
	}

	return fmt.Errorf(
		"ApplicationIdentity CRD (%s/%s) not found; run `make setup-envtest` (or `make test`) to install it",
		targetGroup,
		targetKind,
	)
}
