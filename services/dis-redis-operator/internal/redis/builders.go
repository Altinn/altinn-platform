package redis

import (
	"fmt"
	"maps"
	"strings"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-redis-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-redis-operator/internal/config"
	cachev1 "github.com/Azure/azure-service-operator/v2/api/cache/v1api20250401"
	pev1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// RedisPrivateLinkZoneName is the well-known private DNS zone used for Azure Managed Redis.
	RedisPrivateLinkZoneName = "privatelink.redis.azure.net"

	// DefaultDatabasePort is the Redis Enterprise database client port.
	DefaultDatabasePort int = 10000

	// DefaultDatabaseAzureName is the default database name used per cluster.
	DefaultDatabaseAzureName = "default"

	clusterKubernetesSuffix     = "cluster"
	databaseKubernetesSuffix    = "db"
	privateEndpointSuffix       = "pe"
	privateDNSZoneGroupSuffix   = "pdzg"
	dnsZoneVNetLinkBaseName     = "aks-link"
	privateEndpointConnectionID = "redis-enterprise"
)

// ClusterKubernetesName returns the Kubernetes name used for the ASO RedisEnterprise CR.
func ClusterKubernetesName(redisName string) string {
	return DeterministicKubernetesName(redisName, clusterKubernetesSuffix)
}

// DatabaseKubernetesName returns the Kubernetes name used for the ASO RedisEnterpriseDatabase CR.
func DatabaseKubernetesName(redisName string) string {
	return DeterministicKubernetesName(redisName, databaseKubernetesSuffix)
}

// PrivateEndpointKubernetesName returns the Kubernetes name used for the PrivateEndpoint CR.
func PrivateEndpointKubernetesName(redisName string) string {
	return DeterministicKubernetesName(redisName, privateEndpointSuffix)
}

// PrivateDNSZoneGroupKubernetesName returns the Kubernetes name used for the PrivateDnsZoneGroup CR.
func PrivateDNSZoneGroupKubernetesName(redisName string) string {
	return DeterministicKubernetesName(redisName, privateDNSZoneGroupSuffix)
}

// SharedVNetLinkName returns the Kubernetes name used for the shared AKS VNet link.
func SharedVNetLinkName(environment string) string {
	env := sanitizeKubernetesName(environment)
	if env == "" {
		env = "dis"
	}
	return env + "-" + dnsZoneVNetLinkBaseName
}

// BuildASORedisEnterprise returns the desired RedisEnterprise cluster spec.
func BuildASORedisEnterprise(r *redisv1alpha1.Redis, cfg config.OperatorConfig, azureName string) (*cachev1.RedisEnterprise, error) {
	if r == nil {
		return nil, fmt.Errorf("redis must not be nil")
	}
	if strings.TrimSpace(azureName) == "" {
		return nil, fmt.Errorf("azureName must not be empty")
	}

	location := cfg.Location

	skuName := cachev1.Sku_Name(r.Spec.SKU)
	if r.Spec.SKU == "" {
		skuName = cachev1.Sku_Name_Balanced_B0
	}

	ha := cachev1.ClusterProperties_HighAvailability_Enabled
	if r.Spec.HighAvailability != nil && !*r.Spec.HighAvailability {
		ha = cachev1.ClusterProperties_HighAvailability_Disabled
	}

	tls := cachev1.ClusterProperties_MinimumTlsVersion("1.2")

	zones := []string{}
	if r.Spec.HighAvailability == nil || *r.Spec.HighAvailability {
		zones = []string{"1", "2", "3"}
	}

	tags := maps.Clone(r.Spec.Tags)
	if len(tags) == 0 {
		tags = nil
	}

	cluster := &cachev1.RedisEnterprise{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ClusterKubernetesName(r.Name),
			Namespace: r.Namespace,
			Labels: map[string]string{
				ManagedResourceOwnerLabel: r.Name,
			},
		},
		Spec: cachev1.RedisEnterprise_Spec{
			AzureName:         azureName,
			Location:          &location,
			HighAvailability:  &ha,
			MinimumTlsVersion: &tls,
			Owner: &genruntime.KnownResourceReference{
				ARMID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", cfg.SubscriptionID, cfg.ResourceGroup),
			},
			Sku: &cachev1.Sku{
				Name: &skuName,
			},
			Tags:  tags,
			Zones: zones,
		},
	}

	return cluster, nil
}

// BuildASODatabase returns the desired RedisEnterpriseDatabase spec for the cluster.
func BuildASODatabase(r *redisv1alpha1.Redis, clusterKubernetesName string) (*cachev1.RedisEnterpriseDatabase, error) {
	if r == nil {
		return nil, fmt.Errorf("redis must not be nil")
	}
	if strings.TrimSpace(clusterKubernetesName) == "" {
		return nil, fmt.Errorf("clusterKubernetesName must not be empty")
	}

	clientProtocol := cachev1.DatabaseProperties_ClientProtocol_Encrypted
	if r.Spec.ClientProtocol == redisv1alpha1.RedisClientProtocolPlaintext {
		clientProtocol = cachev1.DatabaseProperties_ClientProtocol_Plaintext
	}

	evictionPolicy := cachev1.DatabaseProperties_EvictionPolicy_NoEviction
	if r.Spec.EvictionPolicy != "" {
		evictionPolicy = cachev1.DatabaseProperties_EvictionPolicy(r.Spec.EvictionPolicy)
	}

	accessKeysDisabled := cachev1.DatabaseProperties_AccessKeysAuthentication_Disabled
	port := DefaultDatabasePort

	modules := buildModules(r.Spec.Modules)
	persistence := buildPersistence(r.Spec.Persistence)

	db := &cachev1.RedisEnterpriseDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DatabaseKubernetesName(r.Name),
			Namespace: r.Namespace,
			Labels: map[string]string{
				ManagedResourceOwnerLabel: r.Name,
			},
		},
		Spec: cachev1.RedisEnterpriseDatabase_Spec{
			AzureName:                DefaultDatabaseAzureName,
			AccessKeysAuthentication: &accessKeysDisabled,
			ClientProtocol:           &clientProtocol,
			EvictionPolicy:           &evictionPolicy,
			Modules:                  modules,
			Persistence:              persistence,
			Port:                     &port,
			Owner: &genruntime.KnownResourceReference{
				Name: clusterKubernetesName,
			},
		},
	}

	return db, nil
}

// BuildPrivateEndpoint returns the desired PrivateEndpoint for a Redis CR's cluster.
func BuildPrivateEndpoint(r *redisv1alpha1.Redis, cfg config.OperatorConfig, clusterKubernetesName string) (*pev1.PrivateEndpoint, error) {
	if r == nil {
		return nil, fmt.Errorf("redis must not be nil")
	}
	subnetID := cfg.PrimarySubnetID()
	if subnetID == "" {
		return nil, fmt.Errorf("no AKS subnet configured for private endpoint")
	}

	location := cfg.Location
	connName := PrivateEndpointKubernetesName(r.Name)

	groupID := "redisEnterprise"
	pe := &pev1.PrivateEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrivateEndpointKubernetesName(r.Name),
			Namespace: r.Namespace,
			Labels: map[string]string{
				ManagedResourceOwnerLabel: r.Name,
			},
		},
		Spec: pev1.PrivateEndpoint_Spec{
			AzureName: PrivateEndpointKubernetesName(r.Name),
			Location:  &location,
			Owner: &genruntime.KnownResourceReference{
				ARMID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", cfg.SubscriptionID, cfg.ResourceGroup),
			},
			Subnet: &pev1.Subnet_PrivateEndpoint_SubResourceEmbedded{
				Reference: &genruntime.ResourceReference{
					ARMID: subnetID,
				},
			},
			PrivateLinkServiceConnections: []pev1.PrivateLinkServiceConnection{
				{
					Name: &connName,
					PrivateLinkServiceReference: &genruntime.ResourceReference{
						Group: cachev1.GroupVersion.Group,
						Kind:  "RedisEnterprise",
						Name:  clusterKubernetesName,
					},
					GroupIds: []string{groupID},
				},
			},
		},
	}

	return pe, nil
}

// BuildSharedPrivateDNSZone returns the desired shared privatelink.redis.azure.net zone.
func BuildSharedPrivateDNSZone(namespace string, cfg config.OperatorConfig) *networkv1.PrivateDnsZone {
	loc := "global"
	return &networkv1.PrivateDnsZone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RedisPrivateLinkZoneName,
			Namespace: namespace,
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
			},
		},
		Spec: networkv1.PrivateDnsZone_Spec{
			AzureName: RedisPrivateLinkZoneName,
			Location:  &loc,
			Owner: &genruntime.KnownResourceReference{
				ARMID: fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", cfg.SubscriptionID, cfg.DNSZoneResourceGroup),
			},
		},
	}
}

// BuildSharedVNetLink returns the desired AKS VNet link for the shared zone.
func BuildSharedVNetLink(namespace string, cfg config.OperatorConfig) *networkv1.PrivateDnsZonesVirtualNetworkLink {
	loc := "global"
	regFalse := false
	linkName := SharedVNetLinkName(cfg.Environment)
	return &networkv1.PrivateDnsZonesVirtualNetworkLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      linkName,
			Namespace: namespace,
			Labels: map[string]string{
				ManagedByLabel: ManagedByValue,
			},
		},
		Spec: networkv1.PrivateDnsZonesVirtualNetworkLink_Spec{
			AzureName:           linkName,
			Location:            &loc,
			RegistrationEnabled: &regFalse,
			Owner: &genruntime.KnownResourceReference{
				Name: RedisPrivateLinkZoneName,
			},
			VirtualNetwork: &networkv1.SubResource{
				Reference: &genruntime.ResourceReference{
					ARMID: cfg.AKSVNetID,
				},
			},
		},
	}
}

func buildModules(in []redisv1alpha1.RedisModule) []cachev1.Module {
	if len(in) == 0 {
		return nil
	}
	out := make([]cachev1.Module, 0, len(in))
	for _, m := range in {
		name := string(m.Name)
		args := m.Args
		mod := cachev1.Module{Name: &name}
		if args != "" {
			mod.Args = &args
		}
		out = append(out, mod)
	}
	return out
}

func buildPersistence(in *redisv1alpha1.RedisPersistence) *cachev1.Persistence {
	if in == nil {
		return nil
	}
	if in.AOF == "" && in.RDB == "" {
		return nil
	}

	out := &cachev1.Persistence{}
	if in.AOF != "" {
		enabled := true
		freq := cachev1.Persistence_AofFrequency(aofFrequencyFromSpec(in.AOF))
		out.AofEnabled = &enabled
		out.AofFrequency = &freq
	}
	if in.RDB != "" {
		enabled := true
		freq := cachev1.Persistence_RdbFrequency(in.RDB)
		out.RdbEnabled = &enabled
		out.RdbFrequency = &freq
	}
	return out
}

func aofFrequencyFromSpec(value string) string {
	switch value {
	case "Always":
		return "always"
	case "Every1Second":
		return "1s"
	default:
		return value
	}
}
