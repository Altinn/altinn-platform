package config

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	subnetARMIDPattern = regexp.MustCompile(`^/subscriptions/[^/]+/resourceGroups/[^/]+/providers/Microsoft\.Network/virtualNetworks/[^/]+/subnets/[^/]+$`)
	vnetARMIDPattern   = regexp.MustCompile(`^/subscriptions/[^/]+/resourceGroups/[^/]+/providers/Microsoft\.Network/virtualNetworks/[^/]+$`)
	tenantIDPattern    = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)
)

// OperatorConfig is runtime configuration for the Redis operator.
type OperatorConfig struct {
	SubscriptionID       string
	ResourceGroup        string
	TenantID             string
	Location             string
	Environment          string
	AKSSubnetIDs         []string
	AKSVNetID            string
	DNSZoneResourceGroup string
}

// ParseSubnetIDs parses and validates comma-separated subnet ARM IDs.
func ParseSubnetIDs(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if !subnetARMIDPattern.MatchString(id) {
			return nil, fmt.Errorf("invalid subnet ARM ID: %s", id)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("aks-subnet-ids must contain at least one subnet ARM ID")
	}
	return ids, nil
}

// NewOperatorConfig validates and returns operator config values.
func NewOperatorConfig(
	subscriptionID,
	resourceGroup,
	tenantID,
	location,
	environment,
	rawSubnetIDs,
	aksVNetID,
	dnsZoneResourceGroup string,
) (*OperatorConfig, error) {
	var missing []string

	subscriptionID = strings.TrimSpace(subscriptionID)
	resourceGroup = strings.TrimSpace(resourceGroup)
	tenantID = strings.TrimSpace(tenantID)
	location = strings.TrimSpace(location)
	environment = strings.TrimSpace(environment)
	aksVNetID = strings.TrimSpace(aksVNetID)
	dnsZoneResourceGroup = strings.TrimSpace(dnsZoneResourceGroup)

	if subscriptionID == "" {
		missing = append(missing, "subscription-id")
	}
	if resourceGroup == "" {
		missing = append(missing, "resource-group")
	}
	if tenantID == "" {
		missing = append(missing, "tenant-id")
	}
	if location == "" {
		missing = append(missing, "location")
	}
	if environment == "" {
		missing = append(missing, "env")
	}
	if aksVNetID == "" {
		missing = append(missing, "aks-vnet-id")
	}
	if dnsZoneResourceGroup == "" {
		missing = append(missing, "dns-zone-resource-group")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	if !tenantIDPattern.MatchString(tenantID) {
		return nil, fmt.Errorf("invalid tenant-id: must be a UUID")
	}
	if !vnetARMIDPattern.MatchString(aksVNetID) {
		return nil, fmt.Errorf("invalid aks-vnet-id: %s", aksVNetID)
	}

	subnetIDs, err := ParseSubnetIDs(rawSubnetIDs)
	if err != nil {
		return nil, err
	}

	return &OperatorConfig{
		SubscriptionID:       subscriptionID,
		ResourceGroup:        resourceGroup,
		TenantID:             tenantID,
		Location:             location,
		Environment:          environment,
		AKSSubnetIDs:         subnetIDs,
		AKSVNetID:            aksVNetID,
		DNSZoneResourceGroup: dnsZoneResourceGroup,
	}, nil
}

// PrimarySubnetID returns the subnet ARM ID where private endpoints land.
func (c OperatorConfig) PrimarySubnetID() string {
	if len(c.AKSSubnetIDs) == 0 {
		return ""
	}
	return c.AKSSubnetIDs[0]
}
