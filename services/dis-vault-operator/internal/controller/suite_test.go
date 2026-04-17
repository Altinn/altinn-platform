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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
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
	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
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

	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(vaultv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(identityv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(keyvaultv1.AddToScheme(scheme)).To(Succeed())
	Expect(authorizationv1.AddToScheme(scheme)).To(Succeed())
	Expect(esov1.AddToScheme(scheme)).To(Succeed())

	By("bootstrapping test environment")
	crds := []*apiextensionsv1.CustomResourceDefinition{
		mustLoadCRD(filepath.Join("..", "..", "config", "crd", "bases"), "vault.dis.altinn.cloud", "Vault"),
		mustLoadCRD(disIdentityCRDPath(), "application.dis.altinn.cloud", "ApplicationIdentity"),
		mustLoadCRD(asoCRDPath(), "keyvault.azure.com", "Vault"),
		mustLoadCRD(asoCRDPath(), "authorization.azure.com", "RoleAssignment"),
		mustLoadCRD(externalSecretsCRDPath(), "external-secrets.io", "SecretStore"),
	}

	testEnv = &envtest.Environment{
		Scheme: scheme,
		CRDInstallOptions: envtest.CRDInstallOptions{
			CRDs:         crds,
			Scheme:       scheme,
			MaxTime:      30 * time.Second,
			PollInterval: 250 * time.Millisecond,
		},
	}

	// Envtest installs CRDs only. It does not run ASO admission webhooks, so these
	// controller tests must model write-once replacement behavior directly rather than
	// relying on webhook rejections to catch invalid RoleAssignment updates.

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
			TenantID:       "00000000-0000-0000-0000-000000000000",
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

func externalSecretsCRDPath() string {
	path := filepath.Join("..", "..", "bin", "external-secrets-crds", "current")
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return ""
}

func mustLoadCRD(path, targetGroup, targetKind string) *apiextensionsv1.CustomResourceDefinition {
	crd, err := loadCRD(path, targetGroup, targetKind)
	Expect(err).NotTo(HaveOccurred())
	Expect(crd).NotTo(BeNil())
	return crd
}

func loadCRD(path, targetGroup, targetKind string) (*apiextensionsv1.CustomResourceDefinition, error) {
	if path == "" {
		return nil, fmt.Errorf("CRD (%s/%s) path is empty; run `make setup-envtest`", targetGroup, targetKind)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat CRD path %s: %w", path, err)
	}
	if !info.IsDir() {
		return loadCRDFromFile(path, targetGroup, targetKind)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read CRD directory %s: %w", path, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		crd, err := loadCRDFromFile(filepath.Join(path, entry.Name()), targetGroup, targetKind)
		if err != nil {
			return nil, err
		}
		if crd != nil {
			return crd, nil
		}
	}

	return nil, fmt.Errorf("CRD (%s/%s) not found in %s; run `make setup-envtest`", targetGroup, targetKind, path)
}

func loadCRDFromFile(filePath, targetGroup, targetKind string) (*apiextensionsv1.CustomResourceDefinition, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open CRD file %s: %w", filePath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	decoder := yaml.NewYAMLOrJSONDecoder(file, 4096)
	for {
		var crd apiextensionsv1.CustomResourceDefinition
		if err := decoder.Decode(&crd); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode CRD file %s: %w", filePath, err)
		}
		if crd.Kind == "CustomResourceDefinition" &&
			crd.Spec.Group == targetGroup &&
			crd.Spec.Names.Kind == targetKind {
			return crd.DeepCopy(), nil
		}
	}

	return nil, nil
}
