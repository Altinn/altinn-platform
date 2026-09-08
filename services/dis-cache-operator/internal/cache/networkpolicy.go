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

	valkeyClientPort     = 6379
	valkeyClusterBusPort = 16379
)

// NetworkPolicyName returns the name of the NetworkPolicy for a Cache.
func NetworkPolicyName(cache *cachev1alpha1.Cache) string {
	return cache.Name + "-cache"
}

// BuildNetworkPolicy limits who can reach the Valkey pods of a Cache:
// pods in the same namespace and the valkey-operator on the client port,
// and the Valkey pods themselves on the client and cluster bus ports.
func BuildNetworkPolicy(cache *cachev1alpha1.Cache) *netv1.NetworkPolicy {
	tcp := corev1.ProtocolTCP
	clientPort := intstr.FromInt32(valkeyClientPort)
	busPort := intstr.FromInt32(valkeyClusterBusPort)

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
					},
				},
				{
					From: []netv1.NetworkPolicyPeer{
						{PodSelector: &valkeyPods},
					},
					Ports: []netv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &clientPort},
						{Protocol: &tcp, Port: &busPort},
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
						},
					},
					Ports: []netv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &clientPort},
					},
				},
			},
		},
	}
}
