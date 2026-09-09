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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	cachev1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
)

const (
	// ManagedByLabel marks objects this operator manages.
	ManagedByLabel = "cache.dis.altinn.cloud/managed-by"
	// ManagedByValue is the value of ManagedByLabel.
	ManagedByValue = "dis-cache-operator"
	// CacheNameLabel carries the name of the owning Cache resource.
	CacheNameLabel = "cache.dis.altinn.cloud/cache"

	// Valkey configuration parameters.
	maxMemoryKey       = "maxmemory"
	maxMemoryPolicyKey = "maxmemory-policy"
	saveKey            = "save"
	appendOnlyKey      = "appendonly"

	// valkeyUID is the uid and gid of the valkey user in the official image.
	// The image entrypoint would switch to it, but the valkey-operator starts
	// valkey-server directly, so the pod must set the user itself.
	valkeyUID int64 = 999

	// Container names the valkey-operator uses in the Valkey pods.
	serverContainerName   = "server"
	exporterContainerName = "metrics-exporter"

	// valkeyServicePrefix is the prefix the valkey-operator puts on the
	// headless Service of a ValkeyCluster.
	valkeyServicePrefix = "valkey-"

	// Valkey ACL command categories.
	aclAllCommands       = "@all"
	aclAdminCommands     = "@admin"
	aclDangerousCommands = "@dangerous"
)

// Images names the container images the operator sets on a ValkeyCluster.
// The platform points them at the ACR pull-through cache. An empty value
// keeps the upstream default, which pulls from Docker Hub.
type Images struct {
	Valkey   string
	Exporter string
}

// ValkeyClusterName returns the name of the ValkeyCluster for a Cache.
// Kubernetes names are unique per namespace, so the Cache name is enough.
func ValkeyClusterName(cache *cachev1alpha1.Cache) string {
	return cache.Name
}

// ValkeyServiceName returns the name of the Valkey Service for a Cache.
func ValkeyServiceName(cache *cachev1alpha1.Cache) string {
	return valkeyServicePrefix + ValkeyClusterName(cache)
}

// Labels returns the labels for objects the operator creates for a Cache.
func Labels(cache *cachev1alpha1.Cache) map[string]string {
	return map[string]string{
		ManagedByLabel: ManagedByValue,
		CacheNameLabel: cache.Name,
	}
}

// BuildValkeyCluster maps a Cache to the ValkeyCluster the valkey-operator
// runs. The owner reference is set by the controller.
func BuildValkeyCluster(cache *cachev1alpha1.Cache, images Images) *valkeyv1alpha1.ValkeyCluster {
	profile := ProfileFor(cache.Spec.Size)

	config := map[string]string{
		maxMemoryKey:       profile.MaxMemory,
		maxMemoryPolicyKey: evictionPolicy(cache),
	}
	if !cache.Spec.Persistence {
		// Valkey keeps a built-in snapshot schedule unless save is cleared.
		// Without this, a cache without persistence still writes its data
		// to the node disk.
		config[saveKey] = ""
		config[appendOnlyKey] = "no"
	}

	cluster := &valkeyv1alpha1.ValkeyCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ValkeyClusterName(cache),
			Namespace: cache.Namespace,
			Labels:    Labels(cache),
		},
		Spec: valkeyv1alpha1.ValkeyClusterSpec{
			Image:              images.Valkey,
			Shards:             1,
			Replicas:           profile.Replicas,
			Resources:          profile.Resources(),
			Config:             config,
			Users:              valkeyUsers(cache),
			Exporter:           valkeyv1alpha1.ExporterSpec{Image: images.Exporter, Enabled: true},
			PodSecurityContext: podSecurityContext(),
			Containers:         hardenedContainers(),
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

// valkeyUsers returns the ACL users for a Cache: the locked built-in default
// user and the app user.
//
// The valkey-operator does not disable the built-in default user, and its
// `enabled` field is an omitempty bool with a CRD default of true, so
// "enabled: false" cannot be sent from Go. The default user is locked with
// resetpass (no password can match) and -@all (no permissions) instead.
func valkeyUsers(cache *cachev1alpha1.Cache) []valkeyv1alpha1.UserAclSpec {
	return []valkeyv1alpha1.UserAclSpec{
		{
			Name:      "default",
			Enabled:   true,
			ResetPass: true,
			Commands:  valkeyv1alpha1.CommandsAclSpec{Deny: []string{aclAllCommands}},
		},
		{
			Name:    AuthUsername,
			Enabled: true,
			PasswordSecret: valkeyv1alpha1.PasswordSecretSpec{
				Name: AuthSecretName(cache),
				Keys: []string{AuthSecretPasswordKey},
			},
			Commands: valkeyv1alpha1.CommandsAclSpec{
				Allow: []string{aclAllCommands},
				Deny:  []string{aclAdminCommands, aclDangerousCommands},
			},
			Keys:     valkeyv1alpha1.KeysAclSpec{ReadWrite: []string{"*"}},
			Channels: valkeyv1alpha1.ChannelsAclSpec{Patterns: []string{"*"}},
		},
	}
}

func podSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot:   ptr.To(true),
		RunAsUser:      ptr.To(valkeyUID),
		RunAsGroup:     ptr.To(valkeyUID),
		FSGroup:        ptr.To(valkeyUID),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// hardenedContainers returns strategic-merge patches for the containers the
// valkey-operator creates. Valkey writes only to /data, which is a volume.
func hardenedContainers() []corev1.Container {
	securityContext := func() *corev1.SecurityContext {
		return &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			ReadOnlyRootFilesystem:   ptr.To(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		}
	}

	return []corev1.Container{
		{Name: serverContainerName, SecurityContext: securityContext()},
		{Name: exporterContainerName, SecurityContext: securityContext()},
	}
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
