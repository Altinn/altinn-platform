/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const (
	// valkeyClusterLabel is the label the valkey-operator puts on every Valkey pod.
	valkeyClusterLabel = "valkey.io/cluster"
	// operatorNamespace is where the valkey-operator runs on the clusters.
	operatorNamespace = "valkey-operator-system"
	// operatorPodLabelValue is the app.kubernetes.io/name label value the
	// valkey-operator Helm chart puts on the operator pods.
	operatorPodLabelValue = "valkey-operator"

	valkeyClientPort     = 6379
	valkeyClusterBusPort = 16379
	// valkeyMetricsPort is the port of the metrics exporter container that
	// the valkey-operator adds to every Valkey pod.
	valkeyMetricsPort = 9121

	// linkerdInboundPort is where the linkerd proxy of a meshed pod accepts
	// connections from other meshed pods. The proxy of a meshed client dials
	// this port, not the application port. Each rule must allow it. If not,
	// the CNI drops the connection before linkerd can authorize it. This is
	// the chart default proxy.ports.inbound; the clusters do not change it.
	linkerdInboundPort = 4143
)

// NetworkPolicyName returns the name of the NetworkPolicy for a Cache.
func NetworkPolicyName(cache *cachev1alpha1.Cache) string {
	return cache.Name + "-cache"
}

// BuildNetworkPolicy limits who can reach the Valkey pods of a Cache:
// pods in the same namespace and the valkey-operator on the client port,
// the Valkey pods themselves on the client and cluster bus ports, and the
// metrics scraper on the linkerd inbound port. Each rule also allows the linkerd
// inbound port, because meshed traffic arrives there. For meshed peers the
// port split is then enforced by the linkerd Server and AuthorizationPolicy,
// which authorize the client by its identity. The application ports stay in
// the first three rules for clients without a proxy. The scraper rule names
// the inbound port only: the scraper is meshed, and the exporter port accepts
// its identity and nothing else. Kubelet probes and the proxy admin port 4191
// are not listed: the Valkey probes are exec probes, the exporter probes are
// HTTP requests from the node, and the CNI on the clusters lets host traffic
// through.
func BuildNetworkPolicy(cache *cachev1alpha1.Cache, scraper Scraper) *netv1.NetworkPolicy {
	tcp := corev1.ProtocolTCP
	clientPort := intstr.FromInt32(valkeyClientPort)
	busPort := intstr.FromInt32(valkeyClusterBusPort)
	meshPort := intstr.FromInt32(linkerdInboundPort)

	valkeyPods := metav1.LabelSelector{
		MatchLabels: map[string]string{valkeyClusterLabel: ValkeyClusterName(cache)},
	}

	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NetworkPolicyName(cache),
			Namespace: cache.Namespace,
			Labels:    Labels(cache),
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: valkeyPods,
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			Ingress: []netv1.NetworkPolicyIngressRule{
				{
					From: []netv1.NetworkPolicyPeer{
						{PodSelector: &metav1.LabelSelector{}},
					},
					Ports: []netv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &clientPort},
						{Protocol: &tcp, Port: &meshPort},
					},
				},
				{
					From: []netv1.NetworkPolicyPeer{
						{PodSelector: &valkeyPods},
					},
					Ports: []netv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &clientPort},
						{Protocol: &tcp, Port: &busPort},
						{Protocol: &tcp, Port: &meshPort},
					},
				},
				{
					From: []netv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									corev1.LabelMetadataName: operatorNamespace,
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": operatorPodLabelValue,
									"control-plane":          "controller-manager",
								},
							},
						},
					},
					Ports: []netv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &clientPort},
						{Protocol: &tcp, Port: &meshPort},
					},
				},
				{
					From: []netv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									corev1.LabelMetadataName: scraper.Namespace,
								},
							},
						},
					},
					Ports: []netv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &meshPort},
					},
				},
			},
		},
	}
}
