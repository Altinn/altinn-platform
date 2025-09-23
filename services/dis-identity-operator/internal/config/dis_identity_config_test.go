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
issuerUrl = "https://test-issuer-url.local"
targetResourceGroup = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/dis-operator-test"
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
		Expect(cfg.IssuerURL).To(Equal("https://test-issuer-url.local"))
		Expect(cfg.TargetResourceGroup).To(Equal("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/dis-operator-test"))
	})

	It("should load config from environment variables", func() {
		Expect(os.Setenv("DISID_ISSUER_URL", "https://env-issuer-url.local")).To(Succeed())
		Expect(os.Setenv("DISID_TARGET_RESOURCE_GROUP", "env-resource-group")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISID_ISSUER_URL")
		}()
		defer func() {
			_ = os.Unsetenv("DISID_TARGET_RESOURCE_GROUP")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := LoadConfig(configFile.Name(), flagset)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.IssuerURL).To(Equal("https://env-issuer-url.local"))
		Expect(cfg.TargetResourceGroup).To(Equal("env-resource-group"))
	})

	It("should ignore double underscores in non DISID prefixed variables", func() {
		Expect(os.Setenv("DISID_ISSUER_URL", "https://env-issuer-url.local")).To(Succeed())
		Expect(os.Setenv("_DISID_SUBSCRIPTION__ID", "env-subscription-id")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISID_ISSUER_URL")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg, err := LoadConfig(configFile.Name(), flagset)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.IssuerURL).To(Equal("https://env-issuer-url.local"))
		Expect(cfg.TargetResourceGroup).To(Equal("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/dis-operator-test"))
	})

	It("should panic if environment has double underscores", func() {
		Expect(os.Setenv("DISID_ISSUER__URL", "https://env-issuer-url.local")).To(Succeed())
		Expect(os.Setenv("DISID_TARGET_RESOURCE_GROUP", "env-resource-group")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISID_ISSUER__URL")
		}()
		defer func() {
			_ = os.Unsetenv("DISID_TARGET_RESOURCE_GROUP")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		Expect(func() { _, _ = LoadConfig(configFile.Name(), flagset) }).To(Panic())
	})

	It("should panic if environment has trailing underscore", func() {
		Expect(os.Setenv("DISID_ISSUER_URL_", "https://env-issuer-url.local")).To(Succeed())
		Expect(os.Setenv("DISID_TARGET_RESOURCE_GROUP", "env-resource-group")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISID_ISSUER_URL_")
		}()
		defer func() {
			_ = os.Unsetenv("DISID_TARGET_RESOURCE_GROUP")
		}()

		flagset := pflag.NewFlagSet("test", pflag.ContinueOnError)
		Expect(func() { _, _ = LoadConfig(configFile.Name(), flagset) }).To(Panic())
	})

	It("should load config from flags", func() {
		Expect(os.Setenv("DISID_ISSUER_URL", "https://env-issuer-url.local")).To(Succeed())
		Expect(os.Setenv("DISID_TARGET_RESOURCE_GROUP", "env-resource-group")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("DISID_ISSUER_URL")
		}()
		defer func() {
			_ = os.Unsetenv("DISID_TARGET_RESOURCE_GROUP")
		}()
		flagset := pflag.NewFlagSet(configFile.Name(), pflag.ContinueOnError)
		flagset.String("issuerUrl", "flag-subscription-id", "subscription id")
		flagset.String("targetResourceGroup", "flag-resource-group", "resource group")
		_ = flagset.Parse([]string{
			"--issuerUrl=https://flag-issuer-url.local",
			"--targetResourceGroup=flag-resource-group",
		})

		cfg := LoadConfigOrDie("", flagset)
		Expect(cfg.IssuerURL).To(Equal("https://flag-issuer-url.local"))
		Expect(cfg.TargetResourceGroup).To(Equal("flag-resource-group"))
	})
})

var _ = Describe("LoadConfigOrDie", func() {
	It("should panic on error", func() {
		badConfigContent := `
issuerUrl "test-subscription-id"
targetResourceGroup "test-resource-group"
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
