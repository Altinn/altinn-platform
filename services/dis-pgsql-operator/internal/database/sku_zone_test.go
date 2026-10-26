package database

import (
	"testing"

	to "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	dbforpostgresqlv1 "github.com/Azure/azure-service-operator/v2/api/dbforpostgresql/v20250801"
)

func TestResolveSku(t *testing.T) {
	legacyProd := &dbforpostgresqlv1.Sku{
		Name: to.Ptr(legacyProdSku),
		Tier: to.Ptr(dbforpostgresqlv1.SkuTier_GeneralPurpose),
	}
	dev := &dbforpostgresqlv1.Sku{
		Name: to.Ptr("Standard_B1ms"),
		Tier: to.Ptr(dbforpostgresqlv1.SkuTier_Burstable),
	}

	tests := []struct {
		name        string
		profile     Profile
		current     *dbforpostgresqlv1.Sku
		provisioned bool
		wantName    string
		wantTier    dbforpostgresqlv1.SkuTier
	}{
		{
			name:        "new server gets the default SKU",
			profile:     prodProfile,
			current:     nil,
			provisioned: false,
			wantName:    prodProfile.DefaultSkuName(),
			wantTier:    prodProfile.SkuTier,
		},
		{
			name:        "provisioned server keeps an accepted SKU",
			profile:     prodProfile,
			current:     legacyProd,
			provisioned: true,
			wantName:    legacyProdSku,
			wantTier:    dbforpostgresqlv1.SkuTier_GeneralPurpose,
		},
		{
			name:        "server that was never provisioned gets the default SKU",
			profile:     prodProfile,
			current:     legacyProd,
			provisioned: false,
			wantName:    prodProfile.DefaultSkuName(),
			wantTier:    prodProfile.SkuTier,
		},
		{
			name:        "provisioned dev server gets the default SKU after a change to prod",
			profile:     prodProfile,
			current:     dev,
			provisioned: true,
			wantName:    prodProfile.DefaultSkuName(),
			wantTier:    prodProfile.SkuTier,
		},
		{
			name:        "provisioned server with an empty SKU name gets the default SKU",
			profile:     prodProfile,
			current:     &dbforpostgresqlv1.Sku{Name: to.Ptr(" "), Tier: to.Ptr(dbforpostgresqlv1.SkuTier_GeneralPurpose)},
			provisioned: true,
			wantName:    prodProfile.DefaultSkuName(),
			wantTier:    prodProfile.SkuTier,
		},
		{
			name:        "provisioned server with a SKU outside the list gets the default SKU",
			profile:     prodProfile,
			current:     &dbforpostgresqlv1.Sku{Name: to.Ptr("Standard_D8ds_v5"), Tier: to.Ptr(dbforpostgresqlv1.SkuTier_GeneralPurpose)},
			provisioned: true,
			wantName:    prodProfile.DefaultSkuName(),
			wantTier:    prodProfile.SkuTier,
		},
		{
			name:        "provisioned server keeps an accepted SKU without a tier",
			profile:     prodProfile,
			current:     &dbforpostgresqlv1.Sku{Name: to.Ptr(legacyProdSku)},
			provisioned: true,
			wantName:    legacyProdSku,
			wantTier:    dbforpostgresqlv1.SkuTier_GeneralPurpose,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSku(tt.profile, tt.current, tt.provisioned)
			if got == nil || got.Name == nil || got.Tier == nil {
				t.Fatalf("ResolveSku() = %+v, want name and tier set", got)
			}
			if *got.Name != tt.wantName {
				t.Fatalf("ResolveSku() name = %q, want %q", *got.Name, tt.wantName)
			}
			if *got.Tier != tt.wantTier {
				t.Fatalf("ResolveSku() tier = %q, want %q", *got.Tier, tt.wantTier)
			}
		})
	}
}

func TestResolveSkuDoesNotAliasCurrent(t *testing.T) {
	current := &dbforpostgresqlv1.Sku{
		Name: to.Ptr(legacyProdSku),
		Tier: to.Ptr(dbforpostgresqlv1.SkuTier_GeneralPurpose),
	}

	got := ResolveSku(prodProfile, current, true)
	*got.Name = "changed"

	if *current.Name != legacyProdSku {
		t.Fatalf("ResolveSku() returned a SKU that shares memory with current")
	}
}

func TestResolveZone(t *testing.T) {
	tests := []struct {
		name        string
		current     *string
		provisioned bool
		want        string
	}{
		{name: "new server gets the default zone", current: nil, provisioned: false, want: "1"},
		{name: "provisioned server keeps its zone", current: to.Ptr("3"), provisioned: true, want: "3"},
		{name: "server that was never provisioned gets the default zone", current: to.Ptr("3"), provisioned: false, want: "1"},
		{name: "provisioned server without a zone gets the default zone", current: nil, provisioned: true, want: "1"},
		{name: "provisioned server with an empty zone gets the default zone", current: to.Ptr(" "), provisioned: true, want: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveZone("1", tt.current, tt.provisioned); got != tt.want {
				t.Fatalf("ResolveZone() = %q, want %q", got, tt.want)
			}
		})
	}
}
