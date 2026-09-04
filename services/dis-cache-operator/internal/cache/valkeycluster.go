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

// Package cache maps Cache resources to the objects the operator manages.
package cache

import (
	valkeyv1alpha1 "github.com/valkey-io/valkey-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const (
	// ManagedByLabel marks objects this operator manages.
	ManagedByLabel = "cache.dis.altinn.cloud/managed-by"
	// ManagedByValue is the value of ManagedByLabel.
	ManagedByValue = "dis-cache-operator"
	// CacheNameLabel carries the name of the owning Cache resource.
	CacheNameLabel = "cache.dis.altinn.cloud/cache"

	// maxMemoryKey and maxMemoryPolicyKey are Valkey configuration parameters.
	maxMemoryKey       = "maxmemory"
	maxMemoryPolicyKey = "maxmemory-policy"
)

// ValkeyClusterName returns the name of the ValkeyCluster for a Cache.
// Kubernetes names are unique per namespace, so the Cache name is enough.
func ValkeyClusterName(cache *cachev1alpha1.Cache) string {
	return cache.Name
}

// Labels returns the labels for objects the operator creates for a Cache.
func Labels(cache *cachev1alpha1.Cache) map[string]string {
	return map[string]string{
		ManagedByLabel: ManagedByValue,
		CacheNameLabel: cache.Name,
	}
}

// BuildValkeyCluster maps a Cache to the ValkeyCluster the valkey-operator
// runs. The owner reference and the access objects (TLS, users) are set by
// the controller.
func BuildValkeyCluster(cache *cachev1alpha1.Cache) *valkeyv1alpha1.ValkeyCluster {
	profile := ProfileFor(cache.Spec.Size)

	cluster := &valkeyv1alpha1.ValkeyCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ValkeyClusterName(cache),
			Namespace: cache.Namespace,
			Labels:    Labels(cache),
		},
		Spec: valkeyv1alpha1.ValkeyClusterSpec{
			Shards:    1,
			Replicas:  profile.Replicas,
			Resources: profile.Resources(),
			Config: map[string]string{
				maxMemoryKey:       profile.MaxMemory,
				maxMemoryPolicyKey: evictionPolicy(cache),
			},
		},
	}

	if cache.Spec.Persistence {
		cluster.Spec.Persistence = &valkeyv1alpha1.PersistenceSpec{
			Size: profile.PVCSize,
			// Cache data is disposable: remove the volume with the node.
			ReclaimPolicy: valkeyv1alpha1.PersistenceReclaimPolicyDelete,
		}
	}

	return cluster
}

// evictionPolicy returns the maxmemory-policy value for a Cache. An empty
// value returns noeviction: the CRD defaults spec.evictionPolicy, so an empty
// value only appears on objects that did not pass the API server.
func evictionPolicy(cache *cachev1alpha1.Cache) string {
	if cache.Spec.EvictionPolicy == "" {
		return string(cachev1alpha1.CacheEvictionNoEviction)
	}

	return string(cache.Spec.EvictionPolicy)
}
