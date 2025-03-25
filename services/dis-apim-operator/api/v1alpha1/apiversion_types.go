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
	"reflect"

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApiVersionSpec defines the desired state of ApiVersion.
type ApiVersionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ApiVersionSetId - The identifier of the API Version Set this version is a part of.
	// +kubebuilder:validation:Required
	ApiVersionSetId string `json:"apiVersionSetId"`
	// ApiVersionScheme - The scheme of the API version. Default value is "Segment".
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=Segment
	ApiVersionScheme APIVersionScheme `json:"apiVersionScheme,omitempty"`
	// Path - API prefix. The value is combined with the API version to form the URL of the API endpoint.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
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
	// ApiVersionSubSpec defines the desired state of ApiVersion
	ApiVersionSubSpec `json:",inline"`
}

// ApiVersionSubSpec defines the desired state of ApiVersion
type ApiVersionSubSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Name - Name of the API Version. If no name is provided this will be the default version
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`
	// DisplayName - The display name of the API Version. This name is used by the developer portal as the API Version name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	DisplayName string `json:"displayName"`
	// Description - Description of the API Version. May include its purpose, where to get more information, and other relevant information.
	// +kubebuilder:validation:Optional
	Description string `json:"description,omitempty"`
	// ServiceUrl - Absolute URL of the backend service implementing this API. Cannot be more than 2000 characters long.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength:=2000
	ServiceUrl *string `json:"serviceUrl,omitempty"`
	// Products - Products that the API is associated with. Products are groups of APIs.
	// +kubebuilder:validation:Optional
	Products []string `json:"products,omitempty"`
	// ContentFormat - Format of the Content in which the API is getting imported. Default value is openapi+json.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=openapi+json
	ContentFormat *ContentFormat `json:"contentFormat,omitempty"`
	// Content - The contents of the API. The value is a string containing the content of the API.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Content *string `json:"content"`
	// SubscriptionRquired - Indicates if subscription is required to access the API. Default value is true.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=true
	SubscriptionRequired *bool `json:"subscriptionRequired,omitempty"`
	// Protocols - Describes protocols over which API is made available. Default value is https.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:={https}
	Protocols []Protocol `json:"protocols,omitempty"`
	// IsCurrent - Indicates if API Version is the current api version. Default value is true.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=true
	IsCurrent *bool `json:"isCurrent,omitempty"`
	// Policies - The API Version Policy description.
	// +kubebuilder:validation:Optional
	Policies *ApiPolicySpec `json:"policies,omitempty"`
	// Diagnostics - The API Version Diagnostic settings.
	// +kubebuilder:validation:Optional
	Diagnostics *ApiDiagnosticSpec `json:"diagnostics,omitempty"`
}

// ApiPolicySpec defines the desired policy of ApiVersion
type ApiPolicySpec struct {
	// PolicyContent - The contents of the Policy as string.
	// +kubebuilder:validation:Required
	PolicyContent *string `json:"policyContent"`
	// PolicyFormat - Format of the Policy in which the API is getting imported.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=rawxml
	// +kubebuilder:validation:Enum:=xml;xml-link;rawxml;rawxml-link
	PolicyFormat *PolicyFormat `json:"policyFormat,omitempty"`
	// PolicyValues Value references for replacing policy expressions.
	// +kubebuilder:validation:Optional
	PolicyValues []PolicyValue `json:"policyValues,omitempty"`
}

// PolicyValue defines the desired state of ApiVersion
// +kubebuilder:validation:XValidation:rule="!has(self.value) || !has(self.idFromBackend)",message="Either value or idFromBackend must be set, but not both"
// +kubebuilder:validation:XValidation:rule="has(self.value) || has(self.idFromBackend)",message="Either value or idFromBackend must be set"
type PolicyValue struct {
	// Name - The key of the policy value.
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// Value - The value of the policy value.
	// +kubebuilder:validation:Optional
	Value *string `json:"value,omitempty"`
	// IdFromBackend references a backend defined in the same namespace. The PolicyValue.Name will be replaced in the ApiPolicySpec with the id of the backend in Azure.
	// +kubebuilder:validation:Optional
	IdFromBackend *FromBackend `json:"idFromBackend,omitempty"`
}

// FromBackend defines the desired state of ApiVersion
type FromBackend struct {
	// Name
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// Namespace Namespace where the backend is defined. Default value is the same namespace as the API Version.
	// +kubebuilder:validation:Optional
	Namespace *string `json:"namespace,omitempty"`
}

// ApiDiagnosticSpec defines the desired diagnostic settings for the ApiVersion.
type ApiDiagnosticSpec struct {
	// LoggerName - The name of the logger to receive the diagnostic data. Operator will lookup the loggerId by this name
	// +kubebuilder:validation:Optional
	LoggerName *string `json:"loggerName,omitempty"`
	// SamplingPercentage - Percentage of the calls to log.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:validation:Maximum:=100
	SamplingPercentage *int32 `json:"samplingPercentage,omitempty"`
	// EnbaleMetrics - Indicates if metrics should be collected.
	EnableMetrics *bool `json:"enableMetrics,omitempty"`
	// Frontend Diagnostic settings for incoming/outgoing HTTP messages to the Gateway. If not specified, the default values are used.
	// +kubebuilder:validation:Optional
	Frontend *PipelineDiagnosticSettings `json:"frontend,omitempty"`
	// Backend Diagnostic settings for incoming/outgoing HTTP messages to the Backend. If not specified, the default values are used.
	// +kubebuilder:validation:Optional
	Backend *PipelineDiagnosticSettings `json:"backend,omitempty"`
}

// PipelineDiagnosticSettings defines the desired diagnostic settings for the ApiVersion.
type PipelineDiagnosticSettings struct {
	// Request - Diagnostic settings for incoming HTTP messages. If not specified, the default values are used.
	// +kubebuilder:validation:Optional
	Request *HttpMessageDiagnostic `json:"request,omitempty"`
	// Response - Diagnostic settings for outgoing HTTP messages. If not specified, the default values are used.
	// +kubebuilder:validation:Optional
	Response *HttpMessageDiagnostic `json:"response,omitempty"`
}

// HttpMessageDiagnostic defines the desired diagnostic settings for the ApiVersion.
type HttpMessageDiagnostic struct {
	// Headers - Array of HTTP Headers to log. Defaults to [Ocp-Apim-Subscription-Key, Content-Type, X-Forwarded-For].
	// +kubebuilder:validation:Optional
	Headers []*string `json:"headers,omitempty"`
}

// ApiVersionStatus defines the observed state of ApiVersion.
type ApiVersionStatus struct {
	// ProvisioningState - The provisioning state of the API. Possible values are: Succeeded, Failed, Updating, Deleting.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:enum:=Succeeded;Failed;Updating;Deleting
	ProvisioningState ProvisioningState `json:"provisioningState,omitempty"`
	// ResumeToken - The token used to track long-running operations.
	// +kubebuilder:validation:Optional
	ResumeToken string `json:"resumeToken,omitempty"`
	// LastAppliedSpecSha - The sha256 of the last applied spec.
	// +kubebuilder:validation:Optional
	LastAppliedSpecSha string `json:"lastAppliedSpecSha,omitempty"`
	// LastAppliedPolicySha - The sha256 of the last applied policy.
	// +kubebuilder:validation:Optional
	LastAppliedPolicySha string `json:"lastAppliedPolicySha,omitempty"`
	// LastAppliedPolicyBase64 - The base64 of the last applied spec.
	// +kubebuilder:validation:Optional
	LastAppliedPolicyBase64 string `json:"lastAppliedPolicyBase64,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.provisioningState`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ApiVersion is the Schema for the apiversions API.
type ApiVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of ApiVersion
	Spec ApiVersionSpec `json:"spec,omitempty"`
	// Status defines the observed state of ApiVersion
	Status ApiVersionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApiVersionList contains a list of ApiVersion.
type ApiVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApiVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApiVersion{}, &ApiVersionList{})
}

func (avss *ApiVersionSubSpec) GetApiVersionSpecifier() string {
	versionSpecifier := avss.Name
	if versionSpecifier == nil || *versionSpecifier == "" {
		versionSpecifier = utils.ToPointer("default")
	}
	return *versionSpecifier
}

func (avss *ApiVersionSubSpec) GetApiVersionFullName(apiFullName string) string {
	return fmt.Sprintf("%s-%s", apiFullName, avss.GetApiVersionSpecifier())
}

func (av *ApiVersion) GetApiVersionAzureFullName() string {
	return fmt.Sprintf("%s-%s", av.Namespace, av.Name)
}

func (av *ApiVersion) GetApiVersionDiagnosticAzureFullName() string {
	return "applicationinsights"
}

func (a *ApiVersion) RequireUpdate(new ApiVersion) bool {
	return !a.Matches(new)
}

func (a *ApiVersion) Matches(new ApiVersion) bool {
	return a.Spec.Path == new.Spec.Path &&
		a.Spec.ApiVersionScheme == new.Spec.ApiVersionScheme &&
		pointerValueEqual(a.Spec.ApiType, new.Spec.ApiType) &&
		pointerValueEqual(a.Spec.Contact, new.Spec.Contact) &&
		pointerValueEqual(a.Spec.ApiVersionSubSpec.Name, new.Spec.ApiVersionSubSpec.Name) &&
		a.Spec.ApiVersionSubSpec.DisplayName == new.Spec.ApiVersionSubSpec.DisplayName &&
		a.Spec.ApiVersionSubSpec.Description == new.Spec.ApiVersionSubSpec.Description &&
		pointerValueEqual(a.Spec.ApiVersionSubSpec.ServiceUrl, new.Spec.ApiVersionSubSpec.ServiceUrl) &&
		reflect.DeepEqual(a.Spec.ApiVersionSubSpec.Products, new.Spec.ApiVersionSubSpec.Products) &&
		pointerValueEqual(a.Spec.ApiVersionSubSpec.ContentFormat, new.Spec.ApiVersionSubSpec.ContentFormat) &&
		pointerValueEqual(a.Spec.ApiVersionSubSpec.Content, new.Spec.ApiVersionSubSpec.Content) &&
		pointerValueEqual(a.Spec.ApiVersionSubSpec.SubscriptionRequired, new.Spec.ApiVersionSubSpec.SubscriptionRequired) &&
		reflect.DeepEqual(a.Spec.ApiVersionSubSpec.Protocols, new.Spec.ApiVersionSubSpec.Protocols) &&
		pointerValueEqual(a.Spec.ApiVersionSubSpec.IsCurrent, new.Spec.ApiVersionSubSpec.IsCurrent) &&
		((a.Spec.ApiVersionSubSpec.Policies == nil && new.Spec.ApiVersionSubSpec.Policies == nil) ||
			(a.Spec.ApiVersionSubSpec.Policies != nil && new.Spec.ApiVersionSubSpec.Policies != nil &&
				pointerValueEqual(a.Spec.ApiVersionSubSpec.Policies.PolicyContent, new.Spec.ApiVersionSubSpec.Policies.PolicyContent) &&
				pointerValueEqual(a.Spec.ApiVersionSubSpec.Policies.PolicyFormat, new.Spec.ApiVersionSubSpec.Policies.PolicyFormat))) &&
		((a.Spec.ApiVersionSubSpec.Diagnostics == nil && new.Spec.ApiVersionSubSpec.Diagnostics == nil) ||
			(a.Spec.ApiVersionSubSpec.Diagnostics != nil && new.Spec.ApiVersionSubSpec.Diagnostics != nil &&
				pointerValueEqual(a.Spec.ApiVersionSubSpec.Diagnostics.LoggerName, new.Spec.ApiVersionSubSpec.Diagnostics.LoggerName) &&
				pointerValueEqual(a.Spec.ApiVersionSubSpec.Diagnostics.SamplingPercentage, new.Spec.ApiVersionSubSpec.Diagnostics.SamplingPercentage) &&
				pointerValueEqual(a.Spec.ApiVersionSubSpec.Diagnostics.EnableMetrics, new.Spec.ApiVersionSubSpec.Diagnostics.EnableMetrics) &&
				reflect.DeepEqual(a.Spec.ApiVersionSubSpec.Diagnostics.Frontend, new.Spec.ApiVersionSubSpec.Diagnostics.Frontend) &&
				reflect.DeepEqual(a.Spec.ApiVersionSubSpec.Diagnostics.Backend, new.Spec.ApiVersionSubSpec.Diagnostics.Backend)))
}

func (a *ApiVersion) ToAzureCreateOrUpdateParameter() apim.APICreateOrUpdateParameter {
	apiCreateOrUpdateParams := apim.APICreateOrUpdateParameter{
		Properties: &apim.APICreateOrUpdateProperties{
			Path:                 &a.Spec.Path,
			APIType:              a.Spec.ApiType.AzureApiType(),
			Description:          &a.Spec.Description,
			DisplayName:          &a.Spec.DisplayName,
			Format:               a.Spec.ContentFormat.AzureContentFormat(),
			IsCurrent:            a.Spec.IsCurrent,
			Protocols:            ToApimProtocolSlice(a.Spec.Protocols),
			ServiceURL:           a.Spec.ServiceUrl,
			SubscriptionRequired: a.Spec.SubscriptionRequired,
			Value:                a.Spec.Content,
			APIVersionSetID:      utils.ToPointer(a.Spec.ApiVersionSetId),
			APIVersion:           a.Spec.Name,
		},
	}
	if a.Spec.Contact != nil {
		apiCreateOrUpdateParams.Properties.Contact = a.Spec.Contact.AzureAPIContactInformation()
	}
	return apiCreateOrUpdateParams
}

func (a *ApiVersion) GetAzureAPIAppInsightsDiagnosticSettings(loggerId string) apim.DiagnosticContract {
	defaultSettings := getDefaultDiagnosticSettings(loggerId, false)
	if a.Spec.Diagnostics != nil {
		return overrideDefaults(defaultSettings, a.Spec.Diagnostics)
	}

	return defaultSettings
}

func (a *ApiVersion) GetAzureAPIAzureMonitorDiagnosticSettings(loggerId string) apim.DiagnosticContract {
	defaultSettings := getDefaultDiagnosticSettings(loggerId, true)
	if a.Spec.Diagnostics != nil {
		return overrideDefaults(defaultSettings, a.Spec.Diagnostics)
	}

	return defaultSettings
}

func getDefaultDiagnosticSettings(loggerId string, azureMonitor bool) apim.DiagnosticContract {
	defaultSettings := apim.DiagnosticContract{
		Properties: &apim.DiagnosticContractProperties{
			LoggerID:  &loggerId,
			AlwaysLog: utils.ToPointer(apim.AlwaysLogAllErrors),
			Backend: &apim.PipelineDiagnosticSettings{
				Request: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: utils.ToPointer(int32(0)),
					},
					DataMasking: nil,
					Headers: []*string{
						utils.ToPointer("Ocp-Apim-Subscription-Key"),
						utils.ToPointer("Content-Type"),
						utils.ToPointer("X-Forwarded-For"),
					},
				},
				Response: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: utils.ToPointer(int32(0)),
					},
					DataMasking: nil,
					Headers: []*string{
						utils.ToPointer("Ocp-Apim-Subscription-Key"),
						utils.ToPointer("Content-Type"),
						utils.ToPointer("X-Forwarded-For"),
					},
				},
			},
			Frontend: &apim.PipelineDiagnosticSettings{
				Request: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: utils.ToPointer(int32(0)),
					},
					DataMasking: nil,
					Headers: []*string{
						utils.ToPointer("Ocp-Apim-Subscription-Key"),
						utils.ToPointer("Content-Type"),
						utils.ToPointer("X-Forwarded-For"),
					},
				},
				Response: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: utils.ToPointer(int32(0)),
					},
					DataMasking: nil,
					Headers: []*string{
						utils.ToPointer("Ocp-Apim-Subscription-Key"),
						utils.ToPointer("Content-Type"),
						utils.ToPointer("X-Forwarded-For"),
					},
				},
			},
			Metrics: utils.ToPointer(true),
			Sampling: &apim.SamplingSettings{
				Percentage:   utils.ToPointer(50.0),
				SamplingType: utils.ToPointer(apim.SamplingTypeFixed),
			},
			LogClientIP: utils.ToPointer(true),
			Verbosity:   utils.ToPointer(apim.VerbosityError),
		},
	}
	if !azureMonitor {
		defaultSettings.Properties.HTTPCorrelationProtocol = utils.ToPointer(apim.HTTPCorrelationProtocolW3C)
	}
	return defaultSettings
}

func overrideDefaults(defaults apim.DiagnosticContract, overrides *ApiDiagnosticSpec) apim.DiagnosticContract {
	if overrides.SamplingPercentage != nil {
		defaults.Properties.Sampling.Percentage = utils.ToPointer(float64(*overrides.SamplingPercentage))
	}

	if overrides.EnableMetrics != nil {
		defaults.Properties.Metrics = overrides.EnableMetrics
	}
	if overrides.Frontend != nil {
		if overrides.Frontend.Request != nil {
			if overrides.Frontend.Request.Headers != nil {
				defaults.Properties.Frontend.Request.Headers = overrides.Frontend.Request.Headers
			}
		}
		if overrides.Frontend.Response != nil {
			if overrides.Frontend.Response.Headers != nil {
				defaults.Properties.Frontend.Response.Headers = overrides.Frontend.Response.Headers
			}
		}
	}
	if overrides.Backend != nil {
		if overrides.Backend.Request != nil {
			if overrides.Backend.Request.Headers != nil {
				defaults.Properties.Backend.Request.Headers = overrides.Backend.Request.Headers
			}
		}
		if overrides.Backend.Response != nil {
			if overrides.Backend.Response.Headers != nil {
				defaults.Properties.Backend.Response.Headers = overrides.Backend.Response.Headers
			}
		}
	}

	return defaults
}

func pointerValueEqual[T comparable](a *T, b *T) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
