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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RedisSKU defines the allowed Azure Managed Redis SKU values.
// +kubebuilder:validation:Enum=Balanced_B0;Balanced_B1;Balanced_B3;Balanced_B5;Balanced_B10;MemoryOptimized_M10;MemoryOptimized_M20
type RedisSKU string

const (
	RedisSKUBalancedB0   RedisSKU = "Balanced_B0"
	RedisSKUBalancedB1   RedisSKU = "Balanced_B1"
	RedisSKUBalancedB3   RedisSKU = "Balanced_B3"
	RedisSKUBalancedB5   RedisSKU = "Balanced_B5"
	RedisSKUBalancedB10  RedisSKU = "Balanced_B10"
	RedisSKUMemoryOptM10 RedisSKU = "MemoryOptimized_M10"
	RedisSKUMemoryOptM20 RedisSKU = "MemoryOptimized_M20"
)

// RedisClientProtocol defines the wire-level client protocol for the Redis database.
// +kubebuilder:validation:Enum=Encrypted;Plaintext
type RedisClientProtocol string

const (
	RedisClientProtocolEncrypted RedisClientProtocol = "Encrypted"
	RedisClientProtocolPlaintext RedisClientProtocol = "Plaintext"
)

// RedisEvictionPolicy defines the Redis cache eviction policy.
// +kubebuilder:validation:Enum=AllKeysLFU;AllKeysLRU;AllKeysRandom;VolatileLFU;VolatileLRU;VolatileRandom;VolatileTTL;NoEviction
type RedisEvictionPolicy string

const (
	RedisEvictionAllKeysLFU     RedisEvictionPolicy = "AllKeysLFU"
	RedisEvictionAllKeysLRU     RedisEvictionPolicy = "AllKeysLRU"
	RedisEvictionAllKeysRandom  RedisEvictionPolicy = "AllKeysRandom"
	RedisEvictionVolatileLFU    RedisEvictionPolicy = "VolatileLFU"
	RedisEvictionVolatileLRU    RedisEvictionPolicy = "VolatileLRU"
	RedisEvictionVolatileRandom RedisEvictionPolicy = "VolatileRandom"
	RedisEvictionVolatileTTL    RedisEvictionPolicy = "VolatileTTL"
	RedisEvictionNoEviction     RedisEvictionPolicy = "NoEviction"
)

// RedisModuleName defines the supported optional Redis modules.
// +kubebuilder:validation:Enum=RedisJSON;RediSearch;RedisTimeSeries;RedisBloom
type RedisModuleName string

// RedisModule enables a single optional Redis module on the database.
type RedisModule struct {
	// Name is the module identifier.
	// +kubebuilder:validation:Required
	Name RedisModuleName `json:"name"`

	// Args are optional, module-specific arguments.
	// +optional
	Args string `json:"args,omitempty"`
}

// RedisPersistence configures AOF / RDB persistence settings for the database.
// At most one of AOF or RDB may be enabled.
// +kubebuilder:validation:XValidation:rule="!(has(self.aof) && has(self.rdb))",message="Only one of 'aof' or 'rdb' may be set"
type RedisPersistence struct {
	// AOF enables append-only-file persistence with the specified frequency.
	// +optional
	// +kubebuilder:validation:Enum=Always;Every1Second
	AOF string `json:"aof,omitempty"`

	// RDB enables snapshot persistence with the specified frequency.
	// +optional
	// +kubebuilder:validation:Enum:="1h";"6h";"12h"
	RDB string `json:"rdb,omitempty"`
}

// ApplicationIdentityRef references an ApplicationIdentity in the same namespace.
type ApplicationIdentityRef struct {
	// Name is the ApplicationIdentity name in the same namespace.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// ServiceAccountRef references a ServiceAccount in the same namespace.
type ServiceAccountRef struct {
	// Name is the ServiceAccount name in the same namespace.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// RedisSpec defines the desired state of Redis.
// +kubebuilder:validation:XValidation:rule="has(self.identityRef) != has(self.serviceAccountRef)",message="exactly one of identityRef or serviceAccountRef must be set"
type RedisSpec struct {
	// IdentityRef points to the owning ApplicationIdentity in the same namespace.
	// +optional
	IdentityRef *ApplicationIdentityRef `json:"identityRef,omitempty"`

	// ServiceAccountRef points to the owning ServiceAccount in the same namespace.
	// +optional
	ServiceAccountRef *ServiceAccountRef `json:"serviceAccountRef,omitempty"`

	// SKU drives cluster capacity. Defaults to the smallest Balanced tier.
	// +optional
	// +kubebuilder:default=Balanced_B0
	SKU RedisSKU `json:"sku,omitempty"`

	// HighAvailability spreads the cluster across availability zones. Defaults to true.
	// +optional
	// +kubebuilder:default=true
	HighAvailability *bool `json:"highAvailability,omitempty"`

	// Version is the Redis version (e.g. "7", "7.4"). Optional; defaults to the ASO default.
	// +optional
	// +kubebuilder:validation:Pattern="^[0-9]+(\\.[0-9]+)?$"
	Version string `json:"version,omitempty"`

	// ClientProtocol selects between Encrypted (TLS) and Plaintext. Defaults to Encrypted.
	// +optional
	// +kubebuilder:default=Encrypted
	ClientProtocol RedisClientProtocol `json:"clientProtocol,omitempty"`

	// EvictionPolicy selects the database eviction policy. Defaults to NoEviction.
	// +optional
	// +kubebuilder:default=NoEviction
	EvictionPolicy RedisEvictionPolicy `json:"evictionPolicy,omitempty"`

	// Modules is the optional list of Redis modules enabled on the database.
	// +optional
	Modules []RedisModule `json:"modules,omitempty"`

	// Persistence configures optional AOF / RDB persistence. Defaults to no persistence.
	// +optional
	Persistence *RedisPersistence `json:"persistence,omitempty"`

	// Tags are optional user-provided tags propagated to Azure resources.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

// RedisStatus defines the observed state of Redis.
type RedisStatus struct {
	// Conditions represent the current state of this Redis.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AzureName is the computed Azure Redis Enterprise cluster name.
	// +optional
	AzureName string `json:"azureName,omitempty"`

	// ClusterResourceID is the ARM resource ID of the Redis Enterprise cluster.
	// +optional
	ClusterResourceID string `json:"clusterResourceId,omitempty"`

	// DatabaseResourceID is the ARM resource ID of the Redis Enterprise database.
	// +optional
	DatabaseResourceID string `json:"databaseResourceId,omitempty"`

	// HostName is the resolved DNS hostname of the cluster (e.g. "<azureName>.<region>.redis.azure.net").
	// +optional
	HostName string `json:"hostName,omitempty"`

	// Port is the database client port (defaults to 10000 for Redis Enterprise).
	// +optional
	Port int32 `json:"port,omitempty"`

	// OwnerPrincipalID is the resolved owner principal ID.
	// +optional
	OwnerPrincipalID string `json:"ownerPrincipalId,omitempty"`

	// AccessPolicyAssignmentName is the name of the managed access policy assignment.
	// +optional
	AccessPolicyAssignmentName string `json:"accessPolicyAssignmentName,omitempty"`

	// ObservedGeneration is the latest generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ConditionType represents status condition type names used by Redis.
type ConditionType string

const (
	ConditionReady                ConditionType = "Ready"
	ConditionIdentityReady        ConditionType = "IdentityReady"
	ConditionClusterReady         ConditionType = "ClusterReady"
	ConditionDatabaseReady        ConditionType = "DatabaseReady"
	ConditionPrivateEndpointReady ConditionType = "PrivateEndpointReady"
	ConditionPrivateDNSReady      ConditionType = "PrivateDNSReady"
	ConditionAccessPolicyReady    ConditionType = "AccessPolicyReady"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=redises
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="AzureName",type="string",JSONPath=".status.azureName"
// +kubebuilder:printcolumn:name="HostName",type="string",JSONPath=".status.hostName"

// Redis is the Schema for the redises API.
type Redis struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of Redis.
	// +required
	Spec RedisSpec `json:"spec"`

	// Status defines the observed state of Redis.
	// +optional
	Status RedisStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RedisList contains a list of Redis.
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Redis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}
