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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApiSpec defines the desired state of Api.
type ApiSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// DisplayName - The display name of the API. This name is used by the developer portal as the API name.
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`
	// Description - Description of the API. May include its purpose, where to get more information, and other relevant information.
	// +kubebuilder:validation:Optional
	Description *string `json:"description,omitempty"`
	// VersioningScheme - Indicates the versioning scheme used for the API. Possible values include, but are not limited to, "Segment", "Query", "Header". Default value is "Segment".
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="Segment"
	// +kubebuilder:validation:Enum:=Header;Query;Segment
	VersioningScheme APIVersionScheme `json:"versioningScheme,omitempty"`
	// Path - API prefix. The value is combined with the API version to form the URL of the API endpoint.
	// +kubebuilder:validation:Required
	Path string `json:"path"`
	// ApiType - Type of API.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="http"
	// +default:value:"http"
	// +kubebuilder:validation:Enum:=graphql;http;websocket
	ApiType *APIType `json:"apiType,omitempty"`
	// Contact - Contact details of the API owner.
	// +kubebuilder:validation:Optional
	Contact *APIContactInformation `json:"contact,omitempty"`
	// Versions - A list of API versions associated with the API. If the API is specified using the OpenAPI definition, then the API version is set by the version field of the OpenAPI definition.
	// +kubebuilder:validation:Required
	Versions []ApiVersionSubSpec `json:"versions"`
}

// ApiStatus defines the observed state of Api.
type ApiStatus struct {
	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Api resource.
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
	// ProvisioningState - The provisioning state of the API. Possible values are: Succeeded, Failed, Updating, Deleting.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:enum:=Succeeded;Failed;Updating;Deleting
	ProvisioningState ProvisioningState `json:"provisioningState,omitempty"`
	// ApiVersionSetID - The identifier of the API Version Set.
	// +kubebuilder:validation:Optional
	ApiVersionSetID string `json:"apiVersionSetID,omitempty"`
	// VersionStates - A list of API Version deployed in the API Management service and the current state of the API Version.
	// +kubebuilder:validation:Optional
	VersionStates map[string]ApiVersionStatus `json:"versionStates,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.provisioningState`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Api is the Schema for the apis API.
type Api struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Api
	// +required
	Spec ApiSpec `json:"spec"`

	// status defines the observed state of Api
	// +optional
	Status ApiStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ApiList contains a list of Api
type ApiList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Api `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Api{}, &ApiList{})
}

// GetApiAzureFullName returns the name of the Azure resource.
func (a *Api) GetApiAzureFullName() string {
	if a == nil {
		return ""
	}
	return fmt.Sprintf("%s-%s", a.Namespace, a.Name)
}

// ToAzureApiVersionSet returns an APIVersionSetContract object.
func (a *Api) ToAzureApiVersionSet() apim.APIVersionSetContract {
	if a == nil {
		return apim.APIVersionSetContract{}
	}
	return apim.APIVersionSetContract{
		Properties: &apim.APIVersionSetContractProperties{
			DisplayName:      &a.Spec.DisplayName,
			VersioningScheme: a.Spec.VersioningScheme.AzureAPIVersionScheme(),
			Description:      a.Spec.Description,
		},
		Name: ptr.To(a.GetApiAzureFullName()),
	}
}

// ToApiVersions returns a map of ApiVersion type.
func (a *Api) ToApiVersions() map[string]ApiVersion {
	apiVersions := make(map[string]ApiVersion)
	for _, version := range a.Spec.Versions {
		versionFullName := version.GetApiVersionFullName(a.Name)
		apiVersion := ApiVersion{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      versionFullName,
				Namespace: a.Namespace,
			},
			Spec: ApiVersionSpec{
				ApiVersionSetId:   a.Status.ApiVersionSetID,
				ApiVersionScheme:  a.Spec.VersioningScheme,
				Path:              a.Spec.Path,
				ApiType:           a.Spec.ApiType,
				ApiVersionSubSpec: version,
			},
		}
		apiVersions[version.GetApiVersionSpecifier()] = apiVersion
	}
	return apiVersions
}
