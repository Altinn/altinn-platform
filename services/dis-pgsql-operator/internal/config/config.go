package config

import (
	"fmt"
	"strings"
)

type OperatorConfig struct {
	// ResourceGroup is the Azure resource group that owns the VNet and
	// the Private DNS zones
	ResourceGroup  string
	DBVNetName     string
	AKSVNetName    string
	SubscriptionId string
	TenantId       string
	DBLocation     string
}

// NewOperatorConfig builds and validates the OperatorConfig from already-parsed
// flag values. It does NOT read environment variables itself.
func NewOperatorConfig(resourceGroup, dbVnetName, aksVnetName, subscriptionId, tenantId string) (*OperatorConfig, error) {
	var missing []string

	subscriptionId = strings.TrimSpace(subscriptionId)
	resourceGroup = strings.TrimSpace(resourceGroup)
	dbVnetName = strings.TrimSpace(dbVnetName)
	aksVnetName = strings.TrimSpace(aksVnetName)
	tenantId = strings.TrimSpace(tenantId)

	if subscriptionId == "" {
		missing = append(missing, "subscription-id")
	}
	if resourceGroup == "" {
		missing = append(missing, "resource-group")
	}
	if dbVnetName == "" {
		missing = append(missing, "db-vnet-name")
	}
	if aksVnetName == "" {
		missing = append(missing, "aks-vnet-name")
	}
	if tenantId == "" {
		missing = append(missing, "tenant-id")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return &OperatorConfig{
		SubscriptionId: subscriptionId,
		ResourceGroup:  resourceGroup,
		DBVNetName:     dbVnetName,
		AKSVNetName:    aksVnetName,
		TenantId:       tenantId,
	}, nil
}
