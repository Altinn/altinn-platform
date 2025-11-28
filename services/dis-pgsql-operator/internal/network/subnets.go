package network

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

// SubnetInfo represents a single subnet with a name (e.g. Azure subnet name)
// and a CIDR string like "10.100.0.0/28".
type SubnetInfo struct {
	Name string
	CIDR string
}

// SubnetCatalog holds an in-memory list of existing subnets and supports finding
// the first free one given a set of used CIDRs.
//
// The catalog itself is static (loaded at startup). Which DB uses which subnet
// is persisted in Kubernetes objects (e.g. Database.status.subnetCIDR).
type SubnetCatalog struct {
	subnets []SubnetInfo
}

// NewSubnetCatalog creates a new catalog from the given subnets.
//
// Subnets are kept in the order they are passed in (typically the Azure API order).
func NewSubnetCatalog(infos []SubnetInfo) (*SubnetCatalog, error) {
	if len(infos) == 0 {
		return &SubnetCatalog{subnets: nil}, nil
	}
	// seen helps to check duplicates
	seen := make(map[string]struct{}, len(infos))
	out := make([]SubnetInfo, 0, len(infos))

	for _, in := range infos {
		if in.CIDR == "" {
			return nil, fmt.Errorf("subnet %q has empty CIDR", in.Name)
		}

		if _, exists := seen[in.CIDR]; exists {
			return nil, fmt.Errorf("duplicate subnet CIDR %q", in.CIDR)
		}
		seen[in.CIDR] = struct{}{}

		out = append(out, SubnetInfo{
			Name: in.Name,
			CIDR: in.CIDR,
		})
	}

	return &SubnetCatalog{subnets: out}, nil
}

// FirstFreeSubnet returns the first subnet in the catalog whose CIDR
// is not present in used.
//
// `used` should contain already allocated subnet CIDR strings, these
// come from Database.status.subnetCIDR
//
// Returns an error if all subnets are used.
func (c *SubnetCatalog) FirstFreeSubnet(used []string) (SubnetInfo, error) {
	if len(c.subnets) == 0 {
		return SubnetInfo{}, fmt.Errorf("subnet catalog is empty")
	}

	usedSet := make(map[string]struct{}, len(used))
	for _, u := range used {
		if u == "" {
			continue
		}
		usedSet[u] = struct{}{}
	}

	for _, s := range c.subnets {
		if _, taken := usedSet[s.CIDR]; !taken {
			return s, nil
		}
	}

	return SubnetInfo{}, fmt.Errorf("no free subnets available")
}

// All returns a copy of all subnets in the catalog, in the catalog's order.
func (c *SubnetCatalog) All() []SubnetInfo {
	out := make([]SubnetInfo, len(c.subnets))
	copy(out, c.subnets)
	return out
}

// FetchSubnetCatalog connects to Azure, lists all subnets in the given VNet,
// and returns them as a SubnetCatalog.
func FetchSubnetCatalog(ctx context.Context, subscriptionID, rgName, vnetName string) (*SubnetCatalog, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := armnetwork.NewSubnetsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnets client: %w", err)
	}

	pager := client.NewListPager(rgName, vnetName, nil)

	var infos []SubnetInfo

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed paging subnets: %w", err)
		}

		for _, s := range page.Value {
			if s == nil || s.Properties == nil || s.Properties.AddressPrefix == nil {
				continue
			}

			name := ""
			if s.Name != nil {
				name = *s.Name
			}

			infos = append(infos, SubnetInfo{
				Name: name,
				CIDR: *s.Properties.AddressPrefix,
			})
		}
	}

	return NewSubnetCatalog(infos)
}
