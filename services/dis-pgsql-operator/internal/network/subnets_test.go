package network

import "testing"

func TestNewSubnetCatalog_KeepsOrderAndValidates(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s2", CIDR: "10.100.0.16/28"},
		{Name: "s1", CIDR: "10.100.0.0/28"},
		{Name: "s3", CIDR: "10.100.0.32/28"},
	}

	catalog, err := NewSubnetCatalog(input)
	if err != nil {
		t.Fatalf("NewSubnetCatalog returned error: %v", err)
	}

	all := catalog.All()
	if got, want := len(all), 3; got != want {
		t.Fatalf("expected %d subnets, got %d", want, got)
	}

	if all[0].Name != "s2" || all[0].CIDR != "10.100.0.16/28" {
		t.Errorf("all[0] = %+v, want Name=s2, CIDR=10.100.0.16/28", all[0])
	}
	if all[1].Name != "s1" || all[1].CIDR != "10.100.0.0/28" {
		t.Errorf("all[1] = %+v, want Name=s1, CIDR=10.100.0.0/28", all[1])
	}
	if all[2].Name != "s3" || all[2].CIDR != "10.100.0.32/28" {
		t.Errorf("all[2] = %+v, want Name=s3, CIDR=10.100.0.32/28", all[2])
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
		{Name: "s1", CIDR: "10.100.0.0/28"},
		{Name: "s2", CIDR: "10.100.0.0/28"},
	}

	if _, err := NewSubnetCatalog(input); err == nil {
		t.Fatalf("expected error for duplicate CIDRs, got nil")
	}
}

func TestFirstFreeSubnet_NoUsed(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s1", CIDR: "10.100.0.0/28"},
		{Name: "s2", CIDR: "10.100.0.16/28"},
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
	if free.Name != "s1" || free.CIDR != "10.100.0.0/28" {
		t.Fatalf("expected first free subnet s1 (10.100.0.0/28), got %+v", free)
	}
}

func TestFirstFreeSubnet_SomeUsed(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s2", CIDR: "10.100.0.16/28"},
		{Name: "s1", CIDR: "10.100.0.0/28"},
		{Name: "s3", CIDR: "10.100.0.32/28"},
	}

	catalog, err := NewSubnetCatalog(input)
	if err != nil {
		t.Fatalf("NewSubnetCatalog returned error: %v", err)
	}

	used := []string{"10.100.0.16/28"} // s2 is used

	free, err := catalog.FirstFreeSubnet(used)
	if err != nil {
		t.Fatalf("FirstFreeSubnet returned error: %v", err)
	}

	// First free in catalog order: s1.
	if free.Name != "s1" || free.CIDR != "10.100.0.0/28" {
		t.Fatalf("expected first free subnet s1 (10.100.0.0/28), got %+v", free)
	}
}

func TestFirstFreeSubnet_AllUsed(t *testing.T) {
	input := []SubnetInfo{
		{Name: "s1", CIDR: "10.100.0.0/28"},
		{Name: "s2", CIDR: "10.100.0.16/28"},
	}

	catalog, err := NewSubnetCatalog(input)
	if err != nil {
		t.Fatalf("NewSubnetCatalog returned error: %v", err)
	}

	used := []string{"10.100.0.0/28", "10.100.0.16/28"}

	if _, err := catalog.FirstFreeSubnet(used); err == nil {
		t.Fatalf("expected error when all subnets are used, got nil")
	}
}
