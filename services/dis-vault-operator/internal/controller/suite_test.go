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
	vaultv1alpha1 "github.com/Altinn/altinn-platform/services/dis-vault-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-vault-operator/internal/config"
	authorizationv1 "github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	keyvaultv1 "github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
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
	if os.Getenv("DISVAULT_SKIP_ENVTEST") == "1" {
		Skip("envtest disabled via DISVAULT_SKIP_ENVTEST")
	}

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())
	scheme := runtime.NewScheme()

	Expect(vaultv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(identityv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(keyvaultv1.AddToScheme(scheme)).To(Succeed())
	Expect(authorizationv1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping test environment")
	crdPaths := []string{
		filepath.Join("..", "..", "config", "crd", "bases"),
	}
	if path := asoCRDPath(); path != "" {
		crdPaths = append(crdPaths, path)
	}
	if path := disIdentityCRDPath(); path != "" {
		crdPaths = append(crdPaths, path)
	}

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     crdPaths,
		Scheme:                scheme,
		ErrorIfCRDPathMissing: true,
	}
	Expect(ensureCRD(crdPaths, "vault.dis.altinn.cloud", "Vault")).To(Succeed())
	Expect(ensureCRD(crdPaths, "application.dis.altinn.cloud", "ApplicationIdentity")).To(Succeed())
	Expect(ensureCRD(crdPaths, "keyvault.azure.com", "Vault")).To(Succeed())
	Expect(ensureCRD(crdPaths, "authorization.azure.com", "RoleAssignment")).To(Succeed())

	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sManager).NotTo(BeNil())

	reconciler := &VaultReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
		Config: config.OperatorConfig{
			SubscriptionID: "sub-123",
			ResourceGroup:  "rg-dis-dev",
			TenantID:       "tenant-123",
			Location:       "westeurope",
			Environment:    "dev",
			AKSSubnetIDs: []string{
				"/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-1",
			},
		},
	}
	Expect(reconciler.SetupWithManager(k8sManager)).To(Succeed())

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
	Expect(testEnv.Stop()).To(Succeed())
})

func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

func asoCRDPath() string {
	monorepoPath := filepath.Join("..", "..", "..", "dis-pgsql-operator", "bin", "aso-crds")
	if info, err := os.Stat(monorepoPath); err == nil && info.IsDir() {
		return monorepoPath
	}
	downloadedPath := filepath.Join("..", "..", "bin", "aso-crds")
	if info, err := os.Stat(downloadedPath); err == nil && info.IsDir() {
		return downloadedPath
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

func ensureCRD(paths []string, targetGroup, targetKind string) error {
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

	return fmt.Errorf("CRD (%s/%s) not found; run `make setup-envtest`", targetGroup, targetKind)
}
