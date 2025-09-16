package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

type AzureConfig struct {
	NamespaceSuffix string `json:"namespaceSuffix,omitempty" koanf:"namespaceSuffix" toml:"namespaceSuffix"`
	SubscriptionId  string `json:"subscriptionId,omitempty" koanf:"subscriptionId" toml:"subscriptionId"`
	ResourceGroup   string `json:"resourceGroup,omitempty" koanf:"resourceGroup" toml:"resourceGroup"`
	ApimServiceName string `json:"apimServiceName,omitempty" koanf:"apimServiceName" toml:"apimServiceName"`
	DefaultLoggerId string `json:"defaultLoggerId,omitempty" koanf:"defaultLoggerId" toml:"defaultLoggerId"`
}

const CONFIG_PREFIX = "DISAPIM_"

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
	err := k.Load(env.Provider(".", env.Opt{
		Prefix: CONFIG_PREFIX,
		TransformFunc: func(k, v string) (string, any) {
			return toCamelCase(strings.ToLower(strings.TrimPrefix(k, CONFIG_PREFIX))), v
		},
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
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, "")
}
