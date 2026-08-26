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

// CacheSize selects the capacity tier of a cache.
// +kubebuilder:validation:Enum=small;medium;large
type CacheSize string

const (
	CacheSizeSmall  CacheSize = "small"
	CacheSizeMedium CacheSize = "medium"
	CacheSizeLarge  CacheSize = "large"
)

// CacheEvictionPolicy selects the Valkey maxmemory-policy.
// +kubebuilder:validation:Enum=noeviction;allkeys-lru;allkeys-lfu;allkeys-random;volatile-lru;volatile-lfu;volatile-random;volatile-ttl
type CacheEvictionPolicy string

const (
	CacheEvictionNoEviction     CacheEvictionPolicy = "noeviction"
	CacheEvictionAllKeysLRU     CacheEvictionPolicy = "allkeys-lru"
	CacheEvictionAllKeysLFU     CacheEvictionPolicy = "allkeys-lfu"
	CacheEvictionAllKeysRandom  CacheEvictionPolicy = "allkeys-random"
	CacheEvictionVolatileLRU    CacheEvictionPolicy = "volatile-lru"
	CacheEvictionVolatileLFU    CacheEvictionPolicy = "volatile-lfu"
	CacheEvictionVolatileRandom CacheEvictionPolicy = "volatile-random"
	CacheEvictionVolatileTTL    CacheEvictionPolicy = "volatile-ttl"
)

// CacheSpec defines the desired state of Cache.
type CacheSpec struct {
	// Size selects the capacity tier. The platform maps each size to CPU,
	// memory, and replica values.
	// +optional
	// +kubebuilder:default=small
	Size CacheSize `json:"size,omitempty"`

	// Persistence keeps cache data on disk. Off by default: a cache does not
	// keep data.
	// +optional
	Persistence bool `json:"persistence,omitempty"`

	// EvictionPolicy selects the Valkey maxmemory-policy. Defaults to noeviction.
	// +optional
	// +kubebuilder:default=noeviction
	EvictionPolicy CacheEvictionPolicy `json:"evictionPolicy,omitempty"`
}

// CacheStatus defines the observed state of Cache.
type CacheStatus struct {
	// Conditions represent the current state of this Cache.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Host is the in-cluster DNS name of the Valkey service.
	// +optional
	Host string `json:"host,omitempty"`

	// Port is the Valkey port.
	// +optional
	Port int32 `json:"port,omitempty"`

	// ObservedGeneration is the latest generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ConditionType represents status condition type names used by Cache.
type ConditionType string

const (
	// ConditionReady aggregates the readiness of everything the operator manages for this Cache.
	// Follow-up changes add the per-dependency condition types as they implement them.
	ConditionReady ConditionType = "Ready"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".status.host"

// Cache is the Schema for the caches API.
type Cache struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of Cache.
	// +required
	Spec CacheSpec `json:"spec"`

	// Status defines the observed state of Cache.
	// +optional
	Status CacheStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// CacheList contains a list of Cache.
type CacheList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Cache `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cache{}, &CacheList{})
}
