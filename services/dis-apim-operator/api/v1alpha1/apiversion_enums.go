package v1alpha1

import (
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3"
	"k8s.io/utils/ptr"
)

// INSERT ADDITIONAL TYPES
// Important: Run "make" to regenerate code after modifying this file

// ContentFormat - Format of the Content in which the API is getting imported.
type ContentFormat string

const (
	// ContentFormatGraphqlLink - The GraphQL API endpoint hosted on a publicly accessible internet address.
	ContentFormatGraphqlLink ContentFormat = "graphql-link"
	// ContentFormatOpenapi - The contents are inline and Content Type is a OpenAPI 3.0 YAML Document.
	ContentFormatOpenapi ContentFormat = "openapi"
	// ContentFormatOpenapiJSON - The contents are inline and Content Type is a OpenAPI 3.0 JSON Document.
	ContentFormatOpenapiJSON ContentFormat = "openapi+json"
	// ContentFormatOpenapiJSONLink - The OpenAPI 3.0 JSON document is hosted on a publicly accessible internet address.
	ContentFormatOpenapiJSONLink ContentFormat = "openapi+json-link"
	// ContentFormatOpenapiLink - The OpenAPI 3.0 YAML document is hosted on a publicly accessible internet address.
	ContentFormatOpenapiLink ContentFormat = "openapi-link"
	// ContentFormatSwaggerJSON - The contents are inline and Content Type is a OpenAPI 2.0 JSON Document.
	ContentFormatSwaggerJSON ContentFormat = "swagger-json"
	// ContentFormatSwaggerLinkJSON - The OpenAPI 2.0 JSON document is hosted on a publicly accessible internet address.
	ContentFormatSwaggerLinkJSON ContentFormat = "swagger-link-json"
	// ContentFormatWadlLinkJSON - The WADL document is hosted on a publicly accessible internet address.
	ContentFormatWadlLinkJSON ContentFormat = "wadl-link-json"
	// ContentFormatWadlXML - The contents are inline and Content type is a WADL document.
	ContentFormatWadlXML ContentFormat = "wadl-xml"
)

func (c ContentFormat) AzureContentFormat() *apim.ContentFormat {
	contentFormat := apim.ContentFormat(c)
	return &contentFormat
}

type APIContactInformation struct {
	// The email address of the contact person/organization. MUST be in the format of an email address
	Email *string `json:"email,omitempty"`

	// The identifying name of the contact person/organization
	Name *string `json:"name,omitempty"`

	// The URL pointing to the contact information. MUST be in the format of a URL
	URL *string `json:"url,omitempty"`
}

func (a *APIContactInformation) AzureAPIContactInformation() *apim.APIContactInformation {
	if a == nil {
		return nil
	}
	return &apim.APIContactInformation{
		Email: a.Email,
		Name:  a.Name,
		URL:   a.URL,
	}
}

type APIVersionScheme string

const (
	// APIVersionSetContractDetailsVersioningSchemeHeader - The API Version is passed in a HTTP header.
	APIVersionSetContractDetailsVersioningSchemeHeader APIVersionScheme = "Header"
	// APIVersionSetContractDetailsVersioningSchemeQuery - The API Version is passed in a query parameter.
	APIVersionSetContractDetailsVersioningSchemeQuery APIVersionScheme = "Query"
	// APIVersionSetContractDetailsVersioningSchemeSegment - The API Version is passed in a path segment.
	APIVersionSetContractDetailsVersioningSchemeSegment APIVersionScheme = "Segment"
)

func (a *APIVersionScheme) AzureAPIVersionScheme() *apim.VersioningScheme {
	if a == nil {
		return nil
	}
	apiVersionScheme := apim.VersioningScheme(*a)
	return &apiVersionScheme
}

func (a *APIVersionScheme) AzureAPIVersionSetContractDetailsVersioningScheme() *apim.APIVersionSetContractDetailsVersioningScheme {
	if a == nil {
		return nil
	}
	apiVersionScheme := apim.APIVersionSetContractDetailsVersioningScheme(*a)
	return &apiVersionScheme
}

type Protocol string

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolHTTPS Protocol = "https"
	ProtocolWs    Protocol = "ws"
	ProtocolWss   Protocol = "wss"
)

func (p *Protocol) AzureProtocol() *apim.Protocol {
	if p == nil {
		return nil
	}
	protocol := apim.Protocol(*p)
	return &protocol
}

func ToApimProtocolSlice(protocols []Protocol) []*apim.Protocol {
	apimProtocols := make([]*apim.Protocol, len(protocols))
	for i, protocol := range protocols {
		apimProtocols[i] = ptr.To(apim.Protocol(protocol))
	}
	return apimProtocols
}

type PolicyFormat string

const (
	// PolicyContentFormatRawxml - The contents are inline and Content type is a non XML encoded policy document.
	PolicyContentFormatRawxml PolicyFormat = "rawxml"
	// PolicyContentFormatRawxmlLink - The policy document is not XML encoded and is hosted on a HTTP endpoint accessible from
	// the API Management service.
	PolicyContentFormatRawxmlLink PolicyFormat = "rawxml-link"
	// PolicyContentFormatXML - The contents are inline and Content type is an XML document.
	PolicyContentFormatXML PolicyFormat = "xml"
	// PolicyContentFormatXMLLink - The policy XML document is hosted on a HTTP endpoint accessible from the API Management service.
	PolicyContentFormatXMLLink PolicyFormat = "xml-link"
)

func (p *PolicyFormat) AzurePolicyFormat() *apim.PolicyContentFormat {
	if p == nil {
		return nil
	}
	policyFormat := apim.PolicyContentFormat(*p)
	return &policyFormat
}

// APIType - Type of API.
type APIType string

const (
	APITypeGraphql   APIType = "graphql"
	APITypeHTTP      APIType = "http"
	APITypeWebsocket APIType = "websocket"
)

func (a APIType) AzureApiType() *apim.APIType {
	apiType := apim.APIType(a)
	return &apiType
}

type ProvisioningState string

const (
	ProvisioningStateSucceeded ProvisioningState = "Succeeded"
	ProvisioningStateFailed    ProvisioningState = "Failed"
	ProvisioningStateUpdating  ProvisioningState = "Updating"
	ProvisioningStateDeleting  ProvisioningState = "Deleting"
	ProvisioningStateDeleted   ProvisioningState = "Deleted"
)
