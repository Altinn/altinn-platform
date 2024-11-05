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
subscriptionId = "test-subscription-id"
resourceGroup = "test-resource-group"
apimServiceName = "test-apim-service"
`
		configFile, err = os.CreateTemp("", "config-*.toml")
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.Write([]byte(configContent))
		Expect(err).NotTo(HaveOccurred())
		configFile.Close()
	})

	AfterEach(func() {
		os.Remove(configFile.Name())
	})

	It("should load config from file", func() {
		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := LoadConfig(configFile.Name(), flagset)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.SubscriptionId).To(Equal("test-subscription-id"))
		Expect(cfg.ResourceGroup).To(Equal("test-resource-group"))
		Expect(cfg.ApimServiceName).To(Equal("test-apim-service"))
	})

	It("should load config from environment variables", func() {
		os.Setenv("DISAPIM_SUBSCRIPTION_ID", "env-subscription-id")
		os.Setenv("DISAPIM_RESOURCE_GROUP", "env-resource-group")
		os.Setenv("DISAPIM_APIM_SERVICE_NAME", "env-apim-service")
		defer os.Unsetenv("DISAPIM_SUBSCRIPTION_ID")
		defer os.Unsetenv("DISAPIM_RESOURCE_GROUP")
		defer os.Unsetenv("DISAPIM_APIM_SERVICE_NAME")

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := LoadConfig(configFile.Name(), flagset)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.SubscriptionId).To(Equal("env-subscription-id"))
		Expect(cfg.ResourceGroup).To(Equal("env-resource-group"))
		Expect(cfg.ApimServiceName).To(Equal("env-apim-service"))
	})

	It("should load config from flags", func() {
		os.Setenv("DISAPIM_SUBSCRIPTION_ID", "env-subscription-id")
		os.Setenv("DISAPIM_RESOURCE_GROUP", "env-resource-group")
		os.Setenv("DISAPIM_APIM_SERVICE_NAME", "env-apim-service")
		defer os.Unsetenv("DISAPIM_SUBSCRIPTION_ID")
		defer os.Unsetenv("DISAPIM_RESOURCE_GROUP")
		defer os.Unsetenv("DISAPIM_APIM_SERVICE_NAME")
		flagset := pflag.NewFlagSet(configFile.Name(), pflag.ContinueOnError)
		flagset.String("subscriptionId", "flag-subscription-id", "subscription id")
		flagset.String("resourceGroup", "flag-resource-group", "resource group")
		flagset.String("apimServiceName", "flag-apim-service", "apim service name")
		flagset.Parse([]string{
			"--subscriptionId=flag-subscription-id",
			"--resourceGroup=flag-resource-group",
			"--apimServiceName=flag-apim-service",
		})

		cfg := LoadConfigOrDie("", flagset)
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
		badConfigFile.Close()
		defer os.Remove(badConfigFile.Name())
		Expect(func() {
			LoadConfigOrDie(badConfigFile.Name(), pflag.NewFlagSet("test", pflag.ExitOnError))
		}).To(Panic())
	})
})
