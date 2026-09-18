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

	// PasswordRotation controls the password rotation of this Cache. The
	// defaults apply when the field is not set.
	// +optional
	// +kubebuilder:default={}
	PasswordRotation *PasswordRotationSpec `json:"passwordRotation,omitempty"`
}

// PasswordRotationSpec controls the password rotation. A rotation stores a
// new password under the Secret key "password" and keeps the old value valid
// under "password-previous" for a period, so applications can change to the
// new password without downtime. The key names do not change, so
// applications keep their manifests.
// +kubebuilder:validation:XValidation:rule="!has(self.intervalDays) || self.intervalDays == 0 || self.intervalDays >= 7",message="intervalDays is 0 (off) or at least 7"
// +kubebuilder:validation:XValidation:rule="!has(self.previousValidFor) || duration(self.previousValidFor) <= duration('168h')",message="previousValidFor is at most 168h (7 days)"
// +kubebuilder:validation:XValidation:rule="!has(self.previousValidFor) || !has(self.intervalDays) || self.intervalDays == 0 || duration(self.previousValidFor).getSeconds() <= self.intervalDays * 43200",message="previousValidFor is at most half of the interval"
type PasswordRotationSpec struct {
	// IntervalDays rotates the password on a schedule, every IntervalDays
	// days after the last rotation. 0 turns the schedule off. The platform
	// default is 0 until applications on the platform reload the password
	// without a restart. The target default is 90.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=365
	IntervalDays int32 `json:"intervalDays,omitempty"`

	// RequestedAt asks for one rotation. A value later than
	// status.passwordRotation.requestedAt starts it. The operator does not
	// wait for this time. A value in the future starts the rotation now. The
	// operator carries out a request made while a previous password is still
	// valid when that period ends.
	// +optional
	RequestedAt *metav1.Time `json:"requestedAt,omitempty"`

	// PreviousValidFor is how long the previous password stays valid after a
	// rotation. Zero removes it at once, for a password that may have leaked.
	// A removed password does not end open connections. An operator flag sets
	// the default, 7 days on the platform. The maximum is 168h.
	// +optional
	PreviousValidFor *metav1.Duration `json:"previousValidFor,omitempty"`
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

	// PasswordRotation records the last password rotation.
	// +optional
	PasswordRotation *PasswordRotationStatus `json:"passwordRotation,omitempty"`
}

// PasswordRotationStatus records the last password rotation.
type PasswordRotationStatus struct {
	// RequestedAt is the spec.passwordRotation.requestedAt value of the last
	// request the operator carried out. A request with a later value starts
	// the next rotation.
	// +optional
	RequestedAt *metav1.Time `json:"requestedAt,omitempty"`

	// LastRotatedAt is the time of the last rotation. The schedule counts
	// from this time.
	// +optional
	LastRotatedAt *metav1.Time `json:"lastRotatedAt,omitempty"`

	// PreviousValidUntil is the time when the previous password stops being
	// valid. It is unset when no previous password exists.
	// +optional
	PreviousValidUntil *metav1.Time `json:"previousValidUntil,omitempty"`
}

// MaxCacheNameLength is the longest Cache name the CRD admits. The
// valkey-operator names the headless Service valkey-<name>, and a Service
// name is a DNS label of at most 63 characters.
const MaxCacheNameLength = 56

// ConditionType represents status condition type names used by Cache.
type ConditionType string

const (
	// ConditionReady aggregates the readiness of everything the operator manages for this Cache.
	// Follow-up changes add the per-dependency condition types as they implement them.
	ConditionReady ConditionType = "Ready"

	// ConditionPasswordRotated reports the password rotation state. True with
	// reason Rotated when no rotation is in progress. False with reason
	// Rotating while the operator changes the Secret and the ValkeyCluster,
	// and with reason PreviousPasswordValid while the previous password is
	// still valid.
	ConditionPasswordRotated ConditionType = "PasswordRotated"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".status.host"
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 56",message="metadata.name must be at most 56 characters: the Valkey Service is named valkey-<name> and a Service name has at most 63 characters"
// +kubebuilder:validation:XValidation:rule="!self.metadata.name.contains('.')",message="metadata.name must not contain a dot: the Valkey Service is named valkey-<name> and a Service name is a single DNS label"

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
