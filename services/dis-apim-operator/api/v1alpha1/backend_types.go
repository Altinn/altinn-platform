/*
Copyright 2024 altinn.

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
	"fmt"

	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// BackendSpec defines the desired state of Backend.
type BackendSpec struct {
	// Title - Title of the Backend. May include its purpose, where to get more information, and other relevant information.
	// +kubebuilder:validation:Required
	Title string `json:"title"`
	// Description - Description of the Backend. May include its purpose, where to get more information, and other relevant information.
	// +kubebuilder:validation:Optional
	Description *string `json:"description,omitempty"`
	// Url - URL of the Backend.
	// +kubebuilder:validation:Required
	Url string `json:"url"`
	// ValidateCertificateChain - Whether to validate the certificate chain when using the backend.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=true
	ValidateCertificateChain *bool `json:"validateCertificateChain,omitempty"`
	// ValidateCertificateName - Whether to validate the certificate name when using the backend.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=true
	ValidateCertificateName *bool `json:"validateCertificateName,omitempty"`
	// AzureResourceUidPrefix - The prefix to use for the Azure resource.
	// +kubebuilder:validation:Optional
	AzureResourcePrefix *string `json:"azureResourceUidPrefix,omitempty"`
}

// BackendStatus defines the observed state of Backend.
type BackendStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Backend resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// BackendID - The identifier of the Backend.
	// +kubebuilder:validation:Optional
	BackendID string `json:"backendID,omitempty"`
	// ProvisioningState - The provisioning state of the Backend.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:enum:=Succeeded;Failed
	ProvisioningState BackendProvisioningState `json:"provisioningState,omitempty"`
	// LastProvisioningError - The last error that occurred during provisioning.
	// +kubebuilder:validation:Optional
	LastProvisioningError string `json:"lastProvisioningError,omitempty"`
}

// BackendProvisioningState defines the provisioning state of the Backend.
type BackendProvisioningState string

const (
	// BackendProvisioningStateSucceeded - The Backend has been successfully provisioned.
	BackendProvisioningStateSucceeded BackendProvisioningState = "Succeeded"
	// BackendProvisioningStateFailed - The Backend has failed to be provisioned.
	BackendProvisioningStateFailed BackendProvisioningState = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.provisioningState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Backend is the Schema for the backends API.
type Backend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Backend
	// +required
	Spec BackendSpec `json:"spec"`

	// status defines the observed state of Backend
	// +optional
	Status BackendStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// BackendList contains a list of Backend
type BackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Backend `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backend{}, &BackendList{})
}

// MatchesActualState returns true if the actual state of the resource in azure (apim.BackendContract) matches the desired state defined in the spec.
func (b *Backend) MatchesActualState(actual *apim.BackendClientGetResponse) bool {
	return b.Spec.Title == *actual.Properties.Title &&
		*b.Spec.Description == *actual.Properties.Description &&
		b.Spec.Url == *actual.Properties.URL &&
		*b.Spec.ValidateCertificateChain == *actual.Properties.TLS.ValidateCertificateChain &&
		*b.Spec.ValidateCertificateName == *actual.Properties.TLS.ValidateCertificateName
}

// ToAzureBackend converts the Backend to an apim.BackendContract.
func (b *Backend) ToAzureBackend() apim.BackendContract {
	return apim.BackendContract{
		Properties: &apim.BackendContractProperties{
			Protocol:    ptr.To(apim.BackendProtocolHTTP),
			URL:         ptr.To(b.Spec.Url),
			Description: b.Spec.Description,
			TLS: &apim.BackendTLSProperties{
				ValidateCertificateChain: b.Spec.ValidateCertificateChain,
				ValidateCertificateName:  b.Spec.ValidateCertificateName,
			},
			Title: ptr.To(b.Spec.Title),
		},
	}
}

// GetAzureResourceName returns the name of the Azure resource.
func (b *Backend) GetAzureResourceName() string {
	if b.Spec.AzureResourcePrefix != nil {
		return fmt.Sprintf("%s-%s", *b.Spec.AzureResourcePrefix, b.Name)
	}
	return fmt.Sprintf("%s-%s", b.Namespace, b.Name)
}
