package cache

import (
	policyv1alpha1 "github.com/linkerd/linkerd2/controller/gen/apis/policy/v1alpha1"
	serverv1beta3 "github.com/linkerd/linkerd2/controller/gen/apis/server/v1beta3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const (
	// operatorServiceAccount is the service account the valkey-operator Helm
	// chart creates. Together with operatorNamespace it forms the operator's
	// mesh identity.
	operatorServiceAccount = "valkey-operator"

	// meshTrustDomain is the linkerd identity trust domain on the DIS clusters
	// (the linkerd default; the clusters set no override).
	meshTrustDomain = "cluster.local"

	linkerdPolicyGroup  = "policy.linkerd.io"
	proxyProtocolOpaque = "opaque"
)

// MeshPolicies are the linkerd objects that let clients reach a cache under
// the clusters' default inbound policy (deny): one Server per Valkey port,
// one authentication with the allowed identities, and one authorization per
// Server that binds the two.
type MeshPolicies struct {
	ClientServer   *serverv1beta3.Server
	BusServer      *serverv1beta3.Server
	Authentication *policyv1alpha1.MeshTLSAuthentication
	ClientPolicy   *policyv1alpha1.AuthorizationPolicy
	BusPolicy      *policyv1alpha1.AuthorizationPolicy
}

// ClientServerName returns the name of the Server for the Valkey client port.
func ClientServerName(cache *cachev1alpha1.Cache) string {
	return cache.Name + "-cache-client"
}

// BusServerName returns the name of the Server for the Valkey cluster bus port.
func BusServerName(cache *cachev1alpha1.Cache) string {
	return cache.Name + "-cache-bus"
}

// MeshAuthenticationName returns the name of the MeshTLSAuthentication for a Cache.
func MeshAuthenticationName(cache *cachev1alpha1.Cache) string {
	return cache.Name + "-cache-clients"
}

// MeshIdentity returns the linkerd identity of a service account.
func MeshIdentity(serviceAccount, namespace string) string {
	return serviceAccount + "." + namespace + ".serviceaccount.identity.linkerd." + meshTrustDomain
}

// BuildMeshPolicies maps a Cache to its linkerd policies. Allowed clients are
// every service account in the cache's namespace and the valkey-operator.
// Both Valkey ports are opaque TCP: the proxy must not try to detect a
// protocol on them.
func BuildMeshPolicies(cache *cachev1alpha1.Cache) MeshPolicies {
	clientServer := buildServer(cache, ClientServerName(cache), valkeyClientPort)
	busServer := buildServer(cache, BusServerName(cache), valkeyClusterBusPort)

	authentication := &policyv1alpha1.MeshTLSAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MeshAuthenticationName(cache),
			Namespace: cache.Namespace,
			Labels:    Labels(cache),
		},
		Spec: policyv1alpha1.MeshTLSAuthenticationSpec{
			Identities: []string{
				MeshIdentity("*", cache.Namespace),
				MeshIdentity(operatorServiceAccount, operatorNamespace),
			},
		},
	}

	return MeshPolicies{
		ClientServer:   clientServer,
		BusServer:      busServer,
		Authentication: authentication,
		ClientPolicy:   buildAuthorizationPolicy(cache, clientServer, authentication),
		BusPolicy:      buildAuthorizationPolicy(cache, busServer, authentication),
	}
}

func buildServer(cache *cachev1alpha1.Cache, name string, port int32) *serverv1beta3.Server {
	return &serverv1beta3.Server{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cache.Namespace,
			Labels:    Labels(cache),
		},
		Spec: serverv1beta3.ServerSpec{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{valkeyClusterLabel: ValkeyClusterName(cache)},
			},
			Port:          intstr.FromInt32(port),
			ProxyProtocol: proxyProtocolOpaque,
		},
	}
}

func buildAuthorizationPolicy(
	cache *cachev1alpha1.Cache,
	server *serverv1beta3.Server,
	authentication *policyv1alpha1.MeshTLSAuthentication,
) *policyv1alpha1.AuthorizationPolicy {
	return &policyv1alpha1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: cache.Namespace,
			Labels:    Labels(cache),
		},
		Spec: policyv1alpha1.AuthorizationPolicySpec{
			TargetRef: policyTargetRef("Server", server.Name),
			RequiredAuthenticationRefs: []gatewayv1alpha2.PolicyTargetReference{
				policyTargetRef("MeshTLSAuthentication", authentication.Name),
			},
		},
	}
}

func policyTargetRef(kind, name string) gatewayv1alpha2.PolicyTargetReference {
	return gatewayv1alpha2.PolicyTargetReference{
		Group: linkerdPolicyGroup,
		Kind:  gatewayv1alpha2.Kind(kind),
		Name:  gatewayv1alpha2.ObjectName(name),
	}
}
