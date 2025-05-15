package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// DisIdentityConfig contains the configuration for the dis-identity operator.
type DisIdentityConfig struct {
	// IssuerURL the issuer URL for the cluster running the instance of the operator.
	IssuerURL string `json:"issuerURL" koanf:"issuerURL" toml:"issuerURL"`
	// TargetResourceGroup the armID of the resource group where the managed identity will be created.
	TargetResourceGroup string `json:"targetResourceGroup" koanf:"targetResourceGroup" toml:"targetResourceGroup"`
}

const CONFIG_PREFIX = "DISID_"

func LoadConfig(configFile string, flagset *pflag.FlagSet) (*DisIdentityConfig, error) {
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
	err := k.Load(env.Provider(CONFIG_PREFIX, ".", func(s string) string {
		return toCamelCase(strings.ToLower(strings.TrimPrefix(s, CONFIG_PREFIX)))
	}), nil)

	if err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Load from flags
	err = k.Load(posflag.Provider(flagset, ".", k), nil)
	if err != nil {
		return nil, fmt.Errorf("error loading flags: %w", err)
	}
	var c DisIdentityConfig
	if err := k.Unmarshal("", &c); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}
	return &c, nil
}

func LoadConfigOrDie(configFile string, flagset *pflag.FlagSet) *DisIdentityConfig {
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
