package config

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("LoadConfig", func() {
	var (
		configFile *os.File
		err        error
	)

	BeforeEach(func() {
		configContent := `
namespaceSuffix = "test"
subscriptionId = "test-subscription-id"
resourceGroup = "test-resource-group"
apimServiceName = "test-apim-service"
`
		configFile, err = os.CreateTemp("", "config-*.toml")
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.Write([]byte(configContent))
		Expect(err).NotTo(HaveOccurred())
		_ = configFile.Close()
	})

	AfterEach(func() {
		Expect(os.Remove(configFile.Name())).To(Succeed())
	})

	It("should load config from file", func() {
		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := LoadConfig(configFile.Name(), flagset)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.NamespaceSuffix).To(Equal("test"))
		Expect(cfg.SubscriptionId).To(Equal("test-subscription-id"))
		Expect(cfg.ResourceGroup).To(Equal("test-resource-group"))
		Expect(cfg.ApimServiceName).To(Equal("test-apim-service"))
	})

	It("should load config from environment variables", func() {
		Expect(os.Setenv("DISAPIM_NAMESPACE_SUFFIX", "env")).To(Succeed())
		Expect(os.Setenv("DISAPIM_SUBSCRIPTION_ID", "env-subscription-id")).To(Succeed())
		Expect(os.Setenv("DISAPIM_RESOURCE_GROUP", "env-resource-group")).To(Succeed())
		Expect(os.Setenv("DISAPIM_APIM_SERVICE_NAME", "env-apim-service")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISAPIM_NAMESPACE_SUFFIX")
			_ = os.Unsetenv("DISAPIM_SUBSCRIPTION_ID")
			_ = os.Unsetenv("DISAPIM_RESOURCE_GROUP")
			_ = os.Unsetenv("DISAPIM_APIM_SERVICE_NAME")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := LoadConfig(configFile.Name(), flagset)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.NamespaceSuffix).To(Equal("env"))
		Expect(cfg.SubscriptionId).To(Equal("env-subscription-id"))
		Expect(cfg.ResourceGroup).To(Equal("env-resource-group"))
		Expect(cfg.ApimServiceName).To(Equal("env-apim-service"))
	})

	It("should ignore double underscores in non DISAPIM prefixed variables", func() {
		Expect(os.Setenv("DISAPIM_NAMESPACE_SUFFIX", "env")).To(Succeed())
		Expect(os.Setenv("_DISAPIM_SUBSCRIPTION__ID", "env-subscription-id")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISAPIM_NAMESPACE_SUFFIX")
			_ = os.Unsetenv("_DISAPIM_SUBSCRIPTION__ID")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := LoadConfig(configFile.Name(), flagset)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.NamespaceSuffix).To(Equal("env"))
		Expect(cfg.SubscriptionId).To(Equal("test-subscription-id"))
		Expect(cfg.ResourceGroup).To(Equal("test-resource-group"))
		Expect(cfg.ApimServiceName).To(Equal("test-apim-service"))
	})

	It("should panic if environment has double underscores", func() {
		Expect(os.Setenv("DISAPIM_NAMESPACE__SUFFIX", "env")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISAPIM_NAMESPACE__SUFFIX")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		Expect(func() { _, _ = LoadConfig(configFile.Name(), flagset) }).To(Panic())
	})

	It("should panic if environment has trailing underscore", func() {
		Expect(os.Setenv("DISAPIM_NAMESPACE_SUFFIX_", "env")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISAPIM_NAMESPACE_SUFFIX_")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		Expect(func() { _, _ = LoadConfig(configFile.Name(), flagset) }).To(Panic())
	})

	It("should load config from flags", func() {
		Expect(os.Setenv("DISAPIM_NAMESPACE_SUFFIX", "env")).To(Succeed())
		Expect(os.Setenv("DISAPIM_SUBSCRIPTION_ID", "env-subscription-id")).To(Succeed())
		Expect(os.Setenv("DISAPIM_RESOURCE_GROUP", "env-resource-group")).To(Succeed())
		Expect(os.Setenv("DISAPIM_APIM_SERVICE_NAME", "env-apim-service")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISAPIM_NAMESPACE_SUFFIX")
			_ = os.Unsetenv("DISAPIM_SUBSCRIPTION_ID")
			_ = os.Unsetenv("DISAPIM_RESOURCE_GROUP")
			_ = os.Unsetenv("DISAPIM_APIM_SERVICE_NAME")
		}()
		flagset := pflag.NewFlagSet(configFile.Name(), pflag.ContinueOnError)
		flagset.String("namespaceSuffix", "flag-namespace-suffix", "namespace suffix")
		flagset.String("subscriptionId", "flag-subscription-id", "subscription id")
		flagset.String("resourceGroup", "flag-resource-group", "resource group")
		flagset.String("apimServiceName", "flag-apim-service", "apim service name")
		_ = flagset.Parse([]string{
			"--namespaceSuffix=flag",
			"--subscriptionId=flag-subscription-id",
			"--resourceGroup=flag-resource-group",
			"--apimServiceName=flag-apim-service",
		})

		cfg := LoadConfigOrDie("", flagset)
		Expect(cfg.NamespaceSuffix).To(Equal("flag"))
		Expect(cfg.SubscriptionId).To(Equal("flag-subscription-id"))
		Expect(cfg.ResourceGroup).To(Equal("flag-resource-group"))
		Expect(cfg.ApimServiceName).To(Equal("flag-apim-service"))
	})
})

var _ = Describe("LoadConfigOrDie", func() {
	It("should panic on error", func() {
		badConfigContent := `
subscriptionId "test-subscription-id"
resourceGroup "test-resource-group"
apimServiceName "test-apim-service"
`
		badConfigFile, err := os.CreateTemp("", "config-*.toml")
		Expect(err).NotTo(HaveOccurred())
		_, err = badConfigFile.Write([]byte(badConfigContent))
		Expect(err).NotTo(HaveOccurred())
		_ = badConfigFile.Close()
		defer func() {
			_ = os.Remove(badConfigFile.Name())
		}()
		Expect(func() {
			LoadConfigOrDie(badConfigFile.Name(), pflag.NewFlagSet("test", pflag.ExitOnError))
		}).To(Panic())
	})
})
