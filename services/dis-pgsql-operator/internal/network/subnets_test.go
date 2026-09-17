package network

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"

	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/test/azfakes"
)

const (
	cidr0  = "10.100.0.0/28"
	cidr16 = "10.100.0.16/28"
	cidr32 = "10.100.0.32/28"
)

func TestNewSubnetCatalog_KeepsOrderAndValidates(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s2", CIDR: cidr16},
		{Name: "s1", CIDR: cidr0},
		{Name: "s3", CIDR: cidr32},
	}

	catalog, err := NewSubnetCatalog(input)
	if err != nil {
		t.Fatalf("NewSubnetCatalog returned error: %v", err)
	}

	all := catalog.All()
	if got, want := len(all), 3; got != want {
		t.Fatalf("expected %d subnets, got %d", want, got)
	}

	if all[0].Name != "s2" || all[0].CIDR != cidr16 {
		t.Errorf("all[0] = %+v, want Name=s2, CIDR=%s", all[0], cidr16)
	}
	if all[1].Name != "s1" || all[1].CIDR != cidr0 {
		t.Errorf("all[1] = %+v, want Name=s1, CIDR=%s", all[1], cidr0)
	}
	if all[2].Name != "s3" || all[2].CIDR != cidr32 {
		t.Errorf("all[2] = %+v, want Name=s3, CIDR=%s", all[2], cidr32)
	}
}

func TestNewSubnetCatalog_EmptyCIDR(t *testing.T) {
	input := []SubnetInfo{
		{Name: "bad", CIDR: ""},
	}

	if _, err := NewSubnetCatalog(input); err == nil {
		t.Fatalf("expected error for empty CIDR, got nil")
	}
}

func TestNewSubnetCatalog_DuplicateCIDR(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s1", CIDR: cidr0},
		{Name: "s2", CIDR: cidr0},
	}

	if _, err := NewSubnetCatalog(input); err == nil {
		t.Fatalf("expected error for duplicate CIDRs, got nil")
	}
}

func TestFirstFreeSubnet_NoUsed(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s1", CIDR: cidr0},
		{Name: "s2", CIDR: cidr16},
	}

	catalog, err := NewSubnetCatalog(input)
	if err != nil {
		t.Fatalf("NewSubnetCatalog returned error: %v", err)
	}

	free, err := catalog.FirstFreeSubnet(nil)
	if err != nil {
		t.Fatalf("FirstFreeSubnet returned error: %v", err)
	}

	// Should pick the first entry in the list.
	if free.Name != "s1" || free.CIDR != cidr0 {
		t.Fatalf("expected first free subnet s1 (%s), got %+v", cidr0, free)
	}
}

func TestFirstFreeSubnet_SomeUsed(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s2", CIDR: cidr16},
		{Name: "s1", CIDR: cidr0},
		{Name: "s3", CIDR: cidr32},
	}

	catalog, err := NewSubnetCatalog(input)
	if err != nil {
		t.Fatalf("NewSubnetCatalog returned error: %v", err)
	}

	used := []string{cidr16} // s2 is used

	free, err := catalog.FirstFreeSubnet(used)
	if err != nil {
		t.Fatalf("FirstFreeSubnet returned error: %v", err)
	}

	// First free in catalog order: s1.
	if free.Name != "s1" || free.CIDR != cidr0 {
		t.Fatalf("expected first free subnet s1 (%s), got %+v", cidr0, free)
	}
}

func TestFirstFreeSubnet_AllUsed(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s1", CIDR: cidr0},
		{Name: "s2", CIDR: cidr16},
	}

	catalog, err := NewSubnetCatalog(input)
	if err != nil {
		t.Fatalf("NewSubnetCatalog returned error: %v", err)
	}

	used := []string{cidr0, cidr16}

	if _, err := catalog.FirstFreeSubnet(used); err == nil {
		t.Fatalf("expected error when all subnets are used, got nil")
	}
}

// fakeSubnet builds an armnetwork.Subnet carrying its prefix in the singular
// addressPrefix field (when singular is non-empty), the plural addressPrefixes
// array (for every entry in plural), or neither.
func fakeSubnet(name, singular string, plural ...string) *armnetwork.Subnet {
	props := &armnetwork.SubnetPropertiesFormat{}

	if singular != "" {
		props.AddressPrefix = to.Ptr(singular)
	}
	for _, prefix := range plural {
		props.AddressPrefixes = append(props.AddressPrefixes, to.Ptr(prefix))
	}

	return &armnetwork.Subnet{
		Name:       to.Ptr(name),
		Properties: props,
	}
}

func fetchFakeCatalog(t *testing.T, subnets []*armnetwork.Subnet) (*SubnetCatalog, error) {
	t.Helper()

	env := azfakes.NewNetworkEnv(azfakes.SubnetsServerWithSubnets(subnets))
	cfg := &config.OperatorConfig{
		SubscriptionId: "00000000-0000-0000-0000-000000000000",
		ResourceGroup:  "rg-fake",
		DBVNetName:     "vnet-fake",
	}

	return FetchSubnetCatalog(context.Background(), cfg, env.Cred, env.ARM)
}

// Azure returns a subnet's prefix in whichever form created it and leaves the
// other field nil: terraform-azurerm wrote the singular addressPrefix before
// v5 and a one-element addressPrefixes array from v5 on, and it never rewrites
// existing subnets. One VNet can therefore hold both forms, and reading only
// addressPrefix drops every subnet a recent provider created.
func TestFetchSubnetCatalog_ReadsBothAddressPrefixForms(t *testing.T) {
	tests := []struct {
		name    string
		subnets []*armnetwork.Subnet
		want    []string
	}{
		{
			name:    "singular addressPrefix",
			subnets: []*armnetwork.Subnet{fakeSubnet("s0", cidr0), fakeSubnet("s1", cidr16)},
			want:    []string{cidr0, cidr16},
		},
		{
			name:    "plural addressPrefixes",
			subnets: []*armnetwork.Subnet{fakeSubnet("s0", "", cidr0), fakeSubnet("s1", "", cidr16)},
			want:    []string{cidr0, cidr16},
		},
		{
			name: "both forms in one VNet",
			subnets: []*armnetwork.Subnet{
				fakeSubnet("s0", cidr0),
				fakeSubnet("s1", "", cidr16),
				fakeSubnet("s2", cidr32),
			},
			want: []string{cidr0, cidr16, cidr32},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog, err := fetchFakeCatalog(t, tt.subnets)
			if err != nil {
				t.Fatalf("FetchSubnetCatalog returned error: %v", err)
			}

			all := catalog.All()
			if got, want := len(all), len(tt.want); got != want {
				t.Fatalf("expected %d subnets, got %d (%+v)", want, got, all)
			}

			for i, want := range tt.want {
				if all[i].CIDR != want {
					t.Errorf("all[%d].CIDR = %q, want %q", i, all[i].CIDR, want)
				}
			}
		})
	}
}

func TestFetchSubnetCatalog_SkipsSubnetsWithoutAnyPrefix(t *testing.T) {
	subnets := []*armnetwork.Subnet{
		fakeSubnet("no-prefix", ""),
		fakeSubnet("empty-plural", "", ""),
		fakeSubnet("usable", "", cidr0),
	}

	catalog, err := fetchFakeCatalog(t, subnets)
	if err != nil {
		t.Fatalf("FetchSubnetCatalog returned error: %v", err)
	}

	all := catalog.All()
	if got, want := len(all), 1; got != want {
		t.Fatalf("expected %d subnets, got %d (%+v)", want, got, all)
	}
	if all[0].CIDR != cidr0 {
		t.Errorf("all[0].CIDR = %q, want %q", all[0].CIDR, cidr0)
	}
}

func TestFetchSubnetCatalog_EmptyWhenNoSubnetHasAPrefix(t *testing.T) {
	_, err := fetchFakeCatalog(t, []*armnetwork.Subnet{fakeSubnet("no-prefix", "")})
	if !errors.Is(err, ErrEmptyCatalog) {
		t.Fatalf("expected ErrEmptyCatalog, got %v", err)
	}
}
