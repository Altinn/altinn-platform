/*
Copyright 2025.

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

// VaultSKU defines the allowed Azure Key Vault SKU values.
// +kubebuilder:validation:Enum=standard;premium
type VaultSKU string

const (
	// VaultSKUStandard uses Azure Key Vault Standard SKU.
	VaultSKUStandard VaultSKU = "standard"
	// VaultSKUPremium uses Azure Key Vault Premium SKU.
	VaultSKUPremium VaultSKU = "premium"
)

// VaultPublicNetworkAccess defines AKV public network access values in v1.
// +kubebuilder:validation:Enum=Enabled
type VaultPublicNetworkAccess string

const (
	// VaultPublicNetworkAccessEnabled is the only supported v1 value.
	VaultPublicNetworkAccessEnabled VaultPublicNetworkAccess = "Enabled"
)

// ApplicationIdentityRef references an ApplicationIdentity in the same namespace.
type ApplicationIdentityRef struct {
	// Name is the ApplicationIdentity name in the same namespace.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// VaultSpec defines the desired state of Vault.
type VaultSpec struct {
	// IdentityRef points to the owning ApplicationIdentity in the same namespace.
	IdentityRef ApplicationIdentityRef `json:"identityRef"`

	// SKU is the Key Vault SKU. Defaults to standard.
	// +optional
	// +kubebuilder:default=standard
	SKU VaultSKU `json:"sku,omitempty"`

	// PublicNetworkAccess is constrained to Enabled in v1.
	// +optional
	PublicNetworkAccess VaultPublicNetworkAccess `json:"publicNetworkAccess,omitempty"`

	// SoftDeleteRetentionDays controls soft-delete retention period.
	// +optional
	// +kubebuilder:default=90
	// +kubebuilder:validation:Minimum=7
	// +kubebuilder:validation:Maximum=90
	SoftDeleteRetentionDays int `json:"softDeleteRetentionDays,omitempty"`

	// PurgeProtectionEnabled controls purge protection. Defaults to true.
	// +optional
	// +kubebuilder:default=true
	PurgeProtectionEnabled bool `json:"purgeProtectionEnabled,omitempty"`

	// Tags are optional user-provided tags propagated to Azure resources.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

// VaultStatus defines the observed state of Vault.
type VaultStatus struct {
	// Conditions represent the current state of this Vault.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// AzureName is the computed Azure Key Vault name.
	// +optional
	AzureName string `json:"azureName,omitempty"`

	// ResourceID is the ARM resource ID of the vault.
	// +optional
	ResourceID string `json:"resourceId,omitempty"`

	// VaultURI is the HTTPS URI of the vault.
	// +optional
	VaultURI string `json:"vaultUri,omitempty"`

	// OwnerPrincipalID is the resolved owner principal ID.
	// +optional
	OwnerPrincipalID string `json:"ownerPrincipalId,omitempty"`

	// OwnerRoleAssignmentID is the ARM ID of the owner role assignment.
	// +optional
	OwnerRoleAssignmentID string `json:"ownerRoleAssignmentId,omitempty"`

	// ObservedGeneration is the latest generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ConditionType represents status condition type names used by Vault.
type ConditionType string

const (
	ConditionReady               ConditionType = "Ready"
	ConditionIdentityReady       ConditionType = "IdentityReady"
	ConditionVaultReady          ConditionType = "VaultReady"
	ConditionRoleAssignmentReady ConditionType = "RoleAssignmentReady"
	ConditionNetworkPolicyReady  ConditionType = "NetworkPolicyReady"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// Vault is the Schema for the vaults API.
type Vault struct {
	metav1.TypeMeta `json:",inline"`

	// Metadata is standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// Spec defines the desired state of Vault.
	// +required
	Spec VaultSpec `json:"spec"`

	// Status defines the observed state of Vault.
	// +optional
	Status VaultStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// VaultList contains a list of Vault.
type VaultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Vault `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Vault{}, &VaultList{})
}
