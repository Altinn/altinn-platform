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

	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
	// LoggerName - The name of the logger to receive the diagnostic data. Operator will lookup the loggerId by this name. Default logger set at runtime
	// +kubebuilder:validation:Optional
	LoggerName *string `json:"loggerName,omitempty"`
	// SamplingPercentage - Percentage of the calls to log. Default value set at runtime.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:validation:Maximum:=100
	SamplingPercentage *int32 `json:"samplingPercentage,omitempty"`
	// EnableMetrics - Indicates if metrics should be collected. Defaults to true, value set at runtime.
	EnableMetrics *bool `json:"enableMetrics,omitempty"`
	// Frontend Diagnostic settings for incoming/outgoing HTTP messages to the Gateway. If not specified, the default values are set at runtime.
	// +kubebuilder:validation:Optional
	Frontend *PipelineDiagnosticSettings `json:"frontend,omitempty"`
	// Backend Diagnostic settings for incoming/outgoing HTTP messages to the Backend. If not specified, the default values are set at runtime.
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
	// Headers - Array of HTTP Headers to log. Defaults to [Ocp-Apim-Subscription-Key, Content-Type, X-Forwarded-For]. Defaults set at runtime.
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
		versionSpecifier = ptr.To("default")
	}
	return *versionSpecifier
}

func (avss *ApiVersionSubSpec) GetApiVersionFullName(apiFullName string) string {
	return fmt.Sprintf("%s-%s", apiFullName, avss.GetApiVersionSpecifier())
}

func (av *ApiVersion) GetApiVersionAzureFullName() string {
	return fmt.Sprintf("%s-%s", av.Namespace, av.Name)
}

func (a *ApiVersion) RequireUpdate(new ApiVersion) bool {
	return !a.Matches(new)
}

func (a *ApiVersion) Matches(new ApiVersion) bool {
	return a.Spec.Path == new.Spec.Path &&
		a.Spec.ApiVersionScheme == new.Spec.ApiVersionScheme &&
		ptr.Equal(a.Spec.ApiType, new.Spec.ApiType) &&
		reflect.DeepEqual(a.Spec.Contact, new.Spec.Contact) &&
		ptr.Equal(a.Spec.Name, new.Spec.Name) &&
		a.Spec.DisplayName == new.Spec.DisplayName &&
		a.Spec.Description == new.Spec.Description &&
		ptr.Equal(a.Spec.ServiceUrl, new.Spec.ServiceUrl) &&
		reflect.DeepEqual(a.Spec.Products, new.Spec.Products) &&
		ptr.Equal(a.Spec.ContentFormat, new.Spec.ContentFormat) &&
		ptr.Equal(a.Spec.Content, new.Spec.Content) &&
		ptr.Equal(a.Spec.SubscriptionRequired, new.Spec.SubscriptionRequired) &&
		reflect.DeepEqual(a.Spec.Protocols, new.Spec.Protocols) &&
		ptr.Equal(a.Spec.IsCurrent, new.Spec.IsCurrent) &&
		((a.Spec.Policies == nil && new.Spec.Policies == nil) ||
			(a.Spec.Policies != nil && new.Spec.Policies != nil &&
				ptr.Equal(a.Spec.Policies.PolicyContent, new.Spec.Policies.PolicyContent) &&
				ptr.Equal(a.Spec.Policies.PolicyFormat, new.Spec.Policies.PolicyFormat))) &&
		((a.Spec.Diagnostics == nil && new.Spec.Diagnostics == nil) ||
			(a.Spec.Diagnostics != nil && new.Spec.Diagnostics != nil &&
				ptr.Equal(a.Spec.Diagnostics.LoggerName, new.Spec.Diagnostics.LoggerName) &&
				ptr.Equal(a.Spec.Diagnostics.SamplingPercentage, new.Spec.Diagnostics.SamplingPercentage) &&
				ptr.Equal(a.Spec.Diagnostics.EnableMetrics, new.Spec.Diagnostics.EnableMetrics) &&
				reflect.DeepEqual(a.Spec.Diagnostics.Frontend, new.Spec.Diagnostics.Frontend) &&
				reflect.DeepEqual(a.Spec.Diagnostics.Backend, new.Spec.Diagnostics.Backend)))
}

func (a *ApiVersion) MatchesAzureResource(current apim.APIClientGetResponse) bool {
	if current.Properties == nil {
		return false
	}
	if !ptr.Equal(a.Spec.ContentFormat, ptr.To(ContentFormatGraphqlLink)) {
		if (current.Properties.Contact == nil) != (a.Spec.Contact == nil) ||
			(a.Spec.Contact != nil && current.Properties.Contact != nil &&
				(!ptr.Equal(a.Spec.Contact.Name, current.Properties.Contact.Name) ||
					!ptr.Equal(a.Spec.Contact.Email, current.Properties.Contact.Email) ||
					!ptr.Equal(a.Spec.Contact.URL, current.Properties.Contact.URL))) {
			return false
		}
	}
	if a.Spec.Path != ptr.Deref(current.Properties.Path, "") ||
		!ptr.Equal(a.Spec.ApiType.AzureApiType(), current.Properties.APIType) ||
		!ptr.Equal(a.Spec.Name, current.Properties.APIVersion) ||
		a.Spec.DisplayName != ptr.Deref(current.Properties.DisplayName, "") ||
		a.Spec.Description != ptr.Deref(current.Properties.Description, "") ||
		!ptr.Equal(a.Spec.ServiceUrl, current.Properties.ServiceURL) ||
		!ptr.Equal(a.Spec.SubscriptionRequired, current.Properties.SubscriptionRequired) ||
		!protocolsEqual(ToApimProtocolSlice(a.Spec.Protocols), current.Properties.Protocols) ||
		!ptr.Equal(a.Spec.IsCurrent, current.Properties.IsCurrent) {
		return false
	}

	return true
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
			APIVersionSetID:      ptr.To(a.Spec.ApiVersionSetId),
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
	var defaultHeaders = []*string{
		ptr.To("Ocp-Apim-Subscription-Key"),
		ptr.To("Content-Type"),
		ptr.To("X-Forwarded-For"),
	}
	defaultSettings := apim.DiagnosticContract{
		Properties: &apim.DiagnosticContractProperties{
			LoggerID:  &loggerId,
			AlwaysLog: ptr.To(apim.AlwaysLogAllErrors),
			Backend: &apim.PipelineDiagnosticSettings{
				Request: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: ptr.To(int32(0)),
					},
					DataMasking: nil,
					Headers:     defaultHeaders,
				},
				Response: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: ptr.To(int32(0)),
					},
					DataMasking: nil,
					Headers:     defaultHeaders,
				},
			},
			Frontend: &apim.PipelineDiagnosticSettings{
				Request: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: ptr.To(int32(0)),
					},
					DataMasking: nil,
					Headers:     defaultHeaders,
				},
				Response: &apim.HTTPMessageDiagnostic{
					Body: &apim.BodyDiagnosticSettings{
						Bytes: ptr.To(int32(0)),
					},
					DataMasking: nil,
					Headers:     defaultHeaders,
				},
			},
			Metrics: ptr.To(true),
			Sampling: &apim.SamplingSettings{
				Percentage:   ptr.To(50.0),
				SamplingType: ptr.To(apim.SamplingTypeFixed),
			},
			LogClientIP: ptr.To(true),
			Verbosity:   ptr.To(apim.VerbosityError),
		},
	}
	if !azureMonitor {
		defaultSettings.Properties.HTTPCorrelationProtocol = ptr.To(apim.HTTPCorrelationProtocolW3C)
	}
	return defaultSettings
}

func overrideDefaults(defaults apim.DiagnosticContract, overrides *ApiDiagnosticSpec) apim.DiagnosticContract {
	if overrides.SamplingPercentage != nil {
		defaults.Properties.Sampling.Percentage = ptr.To(float64(*overrides.SamplingPercentage))
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

func protocolsEqual(a, b []*apim.Protocol) bool {
	if len(a) != len(b) {
		return false
	}

	counts := make(map[apim.Protocol]int)
	nilCount := 0

	for _, p := range a {
		if p == nil {
			nilCount++
		} else {
			counts[*p]++
		}
	}

	for _, p := range b {
		if p == nil {
			nilCount--
		} else {
			counts[*p]--
		}
	}

	if nilCount != 0 {
		return false
	}

	for _, count := range counts {
		if count != 0 {
			return false
		}
	}

	return true
}
