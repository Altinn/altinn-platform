package config

import "fmt"

type OperatorConfig struct {
	// Namespace where ASO resources should be written to
	WriteNs string

	// ResourceGroup is the Azure resource group that owns the VNet and
	// the Private DNS zones
	ResourceGroup  string
	DBVNetName     string
	AKSVNetName    string
	SubscriptionId string
}

// NewOperatorConfig builds and validates the OperatorConfig from already-parsed
// flag values. It does NOT read environment variables itself.
func NewOperatorConfig(writeNs, resourceGroup, dbVnetName, aksVnetName, subscriptionId string) (OperatorConfig, error) {
	cfg := OperatorConfig{
		WriteNs:        writeNs,
		ResourceGroup:  resourceGroup,
		DBVNetName:     dbVnetName,
		AKSVNetName:    aksVnetName,
		SubscriptionId: subscriptionId,
	}

	if cfg.WriteNs == "" {
		return OperatorConfig{}, fmt.Errorf("write namespace must be set (flag --write-namespace)")
	}

	if cfg.ResourceGroup == "" {
		return OperatorConfig{}, fmt.Errorf("resource group must be set (flag --resource-group)")
	}

	return cfg, nil
}
