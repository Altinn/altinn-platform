/*
Copyright 2025 Altinn.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApplicationIdentitySpec defines the desired state of ApplicationIdentity.
type ApplicationIdentitySpec struct {
	// AzureAudiences list of audiences that can appear in the issued token from Azure. Defaults to: [api://AzureADTokenExchange]
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={"api://AzureADTokenExchange"}
	AzureAudiences []string `json:"azureAudiences,omitempty"`
	// Tags is a map of tags to be added to identities created by this ApplicationIdentity.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	Tags map[string]string `json:"tags,omitempty"`
}

// ApplicationIdentityStatus defines the observed state of ApplicationIdentity.
type ApplicationIdentityStatus struct {
	// AzureAudiences list of audiences that can appear in the issued token from Azure.
	// +kubebuilder:validation:Optional
	AzureAudiences []string `json:"azureAudiences,omitempty"`
	// Conditions is a list of conditions that apply to the ApplicationIdentity.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// PrincipalID is the ID of the managed identity in Azure.
	// +kubebuilder:validation:Optional
	PrincipalID *string `json:"principalId,omitempty"`
	// ClientID is the client ID of the managed identity in Azure.
	// +kubebuilder:validation:Optional
	ClientID *string `json:"clientId,omitempty"`
	// ManagedIdentityName is the name of the managed identity in Azure.
	// +kubebuilder:validation:Optional
	ManagedIdentityName *string `json:"managedIdentityName,omitempty"`
}

type ConditionType string

const (
	// ConditionReady indicates the overall ApplicationIdentity status.
	ConditionReady ConditionType = "Ready"
	// ConditionUserAssignedIdentityType indicates the state of the user assigned identity.
	ConditionUserAssignedIdentityType ConditionType = "UserAssignedIdentityReady"
	// ConditionFederatedIdentityType indicates the state of the federated identity.
	ConditionFederatedIdentityType ConditionType = "FederatedIdentityReady"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"

// ApplicationIdentity is the Schema for the applicationidentities API.
type ApplicationIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationIdentitySpec   `json:"spec,omitempty"`
	Status ApplicationIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationIdentityList contains a list of ApplicationIdentity.
type ApplicationIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApplicationIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApplicationIdentity{}, &ApplicationIdentityList{})
}
