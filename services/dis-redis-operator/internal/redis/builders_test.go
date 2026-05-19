package redis

import (
	"testing"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-redis-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-redis-operator/internal/config"
	cachev1 "github.com/Azure/azure-service-operator/v2/api/cache/v1api20250401"
)

const (
	testRedisName    = "my-cache"
	testNamespace    = "default"
	testIdentityName = "my-identity"
)

func testConfig() config.OperatorConfig {
	return config.OperatorConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "rg-dis-dev",
		TenantID:       "00000000-0000-0000-0000-000000000000",
		Location:       "norwayeast",
		Environment:    "dev",
		AKSSubnetIDs: []string{
			"/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet/subnets/aks-1",
		},
		AKSVNetID:            "/subscriptions/sub-123/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet",
		DNSZoneResourceGroup: "rg-dis-dev",
	}
}

func testRedis() *redisv1alpha1.Redis {
	return &redisv1alpha1.Redis{
		Spec: redisv1alpha1.RedisSpec{
			IdentityRef: &redisv1alpha1.ApplicationIdentityRef{Name: testIdentityName},
		},
	}
}

func TestBuildASORedisEnterpriseDefaults(t *testing.T) {
	t.Parallel()

	r := testRedis()
	r.Name = testRedisName
	r.Namespace = testNamespace

	cluster, err := BuildASORedisEnterprise(r, testConfig(), "my-cache-dev-12345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster == nil {
		t.Fatalf("expected non-nil cluster")
	}

	if cluster.Spec.AzureName != "my-cache-dev-12345678" {
		t.Fatalf("expected AzureName to be set, got %q", cluster.Spec.AzureName)
	}
	if cluster.Spec.Sku == nil || cluster.Spec.Sku.Name == nil || *cluster.Spec.Sku.Name != cachev1.Sku_Name_Balanced_B0 {
		t.Fatalf("expected default SKU Balanced_B0, got %#v", cluster.Spec.Sku)
	}
	if cluster.Spec.HighAvailability == nil || *cluster.Spec.HighAvailability != cachev1.ClusterProperties_HighAvailability_Enabled {
		t.Fatalf("expected HA enabled by default")
	}
	if len(cluster.Spec.Zones) != 3 {
		t.Fatalf("expected 3 availability zones for HA cluster, got %d", len(cluster.Spec.Zones))
	}
}

func TestBuildASORedisEnterpriseNonHA(t *testing.T) {
	t.Parallel()

	disabled := false
	r := testRedis()
	r.Name = testRedisName
	r.Namespace = testNamespace
	r.Spec.HighAvailability = &disabled

	cluster, err := BuildASORedisEnterprise(r, testConfig(), "name-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *cluster.Spec.HighAvailability != cachev1.ClusterProperties_HighAvailability_Disabled {
		t.Fatalf("expected HA disabled")
	}
	if len(cluster.Spec.Zones) != 0 {
		t.Fatalf("expected no zones for non-HA cluster, got %d", len(cluster.Spec.Zones))
	}
}

func TestBuildASODatabaseDefaults(t *testing.T) {
	t.Parallel()

	r := testRedis()
	r.Name = testRedisName
	r.Namespace = testNamespace

	db, err := BuildASODatabase(r, ClusterKubernetesName(testRedisName))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db.Spec.AccessKeysAuthentication == nil || *db.Spec.AccessKeysAuthentication != cachev1.DatabaseProperties_AccessKeysAuthentication_Disabled {
		t.Fatalf("expected access keys disabled by default")
	}
	if db.Spec.ClientProtocol == nil || *db.Spec.ClientProtocol != cachev1.DatabaseProperties_ClientProtocol_Encrypted {
		t.Fatalf("expected encrypted client protocol by default")
	}
	if db.Spec.Port == nil || *db.Spec.Port != DefaultDatabasePort {
		t.Fatalf("expected default port %d, got %v", DefaultDatabasePort, db.Spec.Port)
	}
	if db.Spec.Owner == nil || db.Spec.Owner.Name != ClusterKubernetesName(testRedisName) {
		t.Fatalf("expected owner to reference cluster")
	}
}

func TestBuildPrivateEndpoint(t *testing.T) {
	t.Parallel()

	r := testRedis()
	r.Name = testRedisName
	r.Namespace = testNamespace

	pe, err := BuildPrivateEndpoint(r, testConfig(), ClusterKubernetesName(testRedisName))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pe.Spec.Subnet == nil || pe.Spec.Subnet.Reference == nil {
		t.Fatalf("expected subnet to be referenced")
	}
	if len(pe.Spec.PrivateLinkServiceConnections) != 1 {
		t.Fatalf("expected one private link service connection")
	}
}

func TestBuildSharedPrivateDNSZone(t *testing.T) {
	t.Parallel()

	zone := BuildSharedPrivateDNSZone(testNamespace, testConfig())
	if zone.Name != RedisPrivateLinkZoneName {
		t.Fatalf("expected zone name %q, got %q", RedisPrivateLinkZoneName, zone.Name)
	}
	if zone.Labels[ManagedByLabel] != ManagedByValue {
		t.Fatalf("expected managed-by label on shared zone")
	}
}

func TestSharedVNetLinkName(t *testing.T) {
	t.Parallel()

	if got := SharedVNetLinkName("Dev"); got != "dev-aks-link" {
		t.Fatalf("expected dev-aks-link, got %q", got)
	}
	if got := SharedVNetLinkName(""); got != "dis-aks-link" {
		t.Fatalf("expected fallback dis-aks-link, got %q", got)
	}
}
