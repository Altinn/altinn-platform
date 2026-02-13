package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

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
	err = batchv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = networkv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = dbforpostgresqlv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "bin", "aso-crds"),
			filepath.Join("..", "..", "testdata", "crds"),
		},
		Scheme:                scheme,
		ErrorIfCRDPathMissing: true,
	}

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

	// Build a fixed subnet catalog for tests
	testCatalog, err := network.NewSubnetCatalog([]network.SubnetInfo{
		{Name: "s1", CIDR: "10.100.1.0/28"},
		{Name: "s2", CIDR: "10.100.1.16/28"},
		{Name: "s3", CIDR: "10.100.1.32/28"},
		{Name: "s4", CIDR: "10.100.1.48/28"},
		{Name: "s5", CIDR: "10.100.1.64/28"},
		{Name: "s6", CIDR: "10.100.1.80/28"},
		{Name: "s7", CIDR: "10.100.1.96/28"},
		{Name: "s8", CIDR: "10.100.1.112/28"},
		{Name: "s9", CIDR: "10.100.1.128/28"},
		{Name: "s10", CIDR: "10.100.1.144/28"},
		{Name: "s11", CIDR: "10.100.1.160/28"},
		{Name: "s12", CIDR: "10.100.1.176/28"},
		{Name: "s13", CIDR: "10.100.1.192/28"},
		{Name: "s14", CIDR: "10.100.1.208/28"},
		{Name: "s15", CIDR: "10.100.1.224/28"},
		{Name: "s16", CIDR: "10.100.1.240/28"},
	})

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
