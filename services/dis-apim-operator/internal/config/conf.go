package config

import (
	"fmt"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
	"os"
	"strings"
)

type AzureConfig struct {
	SubscriptionId  string `json:"subscriptionId,omitempty" koanf:"subscriptionId" toml:"subscriptionId"`
	ResourceGroup   string `json:"resourceGroup,omitempty" koanf:"resourceGroup" toml:"resourceGroup"`
	ApimServiceName string `json:"apimServiceName,omitempty" koanf:"apimServiceName" toml:"apimServiceName"`
}

func LoadConfig(configFile string, flagset *pflag.FlagSet) (*AzureConfig, error) {
	k := koanf.New(".")

	// Load from file
	if configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			err := k.Load(file.Provider(configFile), toml.Parser())
			if err != nil {
				return nil, fmt.Errorf("error loading config file: %w", err)
			}
		}
	}

	// Load from environment
	err := k.Load(env.Provider("DISAPIM_", ".", func(s string) string {
		return toCamelCase(strings.ToLower(strings.TrimPrefix(s, "DISAPIM_")))
	}), nil)

	if err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Load from flags
	err = k.Load(posflag.Provider(flagset, ".", k), nil)
	if err != nil {
		return nil, fmt.Errorf("error loading flags: %w", err)
	}
	var c AzureConfig
	if err := k.Unmarshal("", &c); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}
	return &c, nil
}

func LoadConfigOrDie(configFile string, flagset *pflag.FlagSet) *AzureConfig {
	c, err := LoadConfig(configFile, flagset)
	if err != nil {
		panic(err)
	}
	return c
}

func toCamelCase(snake string) string {
	parts := strings.Split(snake, "_")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.Title(parts[i])
	}
	return strings.Join(parts, "")
}
