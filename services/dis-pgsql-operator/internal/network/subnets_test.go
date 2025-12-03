package network

import "testing"

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
