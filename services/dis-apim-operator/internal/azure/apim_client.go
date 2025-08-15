package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3"
	"k8s.io/utils/ptr"
)

// APIMClient is a client for interacting with the Azure API Management service
type APIMClient struct {
	// ApimClientConfig is the configuration for the APIM client
	ApimClientConfig  ApimClientConfig
	apimClientFactory *apim.ClientFactory
}

// ApimClientConfig is the configuration for the APIMClient
type ApimClientConfig struct {
	config.AzureConfig `json:",inline"`
	ClientOptions      *azidentity.DefaultAzureCredentialOptions `json:"clientOptions,omitempty"`
	FactoryOptions     *arm.ClientOptions                        `json:"factoryOptions,omitempty"`
}

// NewAPIMClient creates a new APIMClient
func NewAPIMClient(clientConfig *ApimClientConfig) (*APIMClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(clientConfig.ClientOptions)
	if err != nil {
		return nil, err
	}
	clientFactory, err := apim.NewClientFactory(clientConfig.SubscriptionId, credential, clientConfig.FactoryOptions)
	if err != nil {
		return nil, err
	}
	return &APIMClient{
		ApimClientConfig:  *clientConfig,
		apimClientFactory: clientFactory,
	}, nil
}

// NewApimClientWithFactory creates a new APIMClient with a given client factory
func NewApimClientWithFactory(clientConfig *ApimClientConfig, factory *apim.ClientFactory) *APIMClient {
	return &APIMClient{
		ApimClientConfig:  *clientConfig,
		apimClientFactory: factory,
	}
}

func (c *APIMClient) GetApiVersionSet(ctx context.Context, apiVersionSetName string, options *apim.APIVersionSetClientGetOptions) (apim.APIVersionSetClientGetResponse, error) {
	client := c.apimClientFactory.NewAPIVersionSetClient()
	return client.Get(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiVersionSetName, options)
}

func (c *APIMClient) CreateUpdateApiVersionSet(ctx context.Context, apiVersionSetName string, parameters apim.APIVersionSetContract, options *apim.APIVersionSetClientCreateOrUpdateOptions) (apim.APIVersionSetClientCreateOrUpdateResponse, error) {
	client := c.apimClientFactory.NewAPIVersionSetClient()
	return client.CreateOrUpdate(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiVersionSetName, parameters, options)
}

func (c *APIMClient) DeleteApiVersionSet(ctx context.Context, apiVersionSetName string, etag string, options *apim.APIVersionSetClientDeleteOptions) (apim.APIVersionSetClientDeleteResponse, error) {
	client := c.apimClientFactory.NewAPIVersionSetClient()
	return client.Delete(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiVersionSetName, etag, options)
}

func (c *APIMClient) GetApi(ctx context.Context, apiId string, options *apim.APIClientGetOptions) (apim.APIClientGetResponse, error) {
	client := c.apimClientFactory.NewAPIClient()
	return client.Get(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, options)
}

func (c *APIMClient) CreateUpdateApi(ctx context.Context, apiId string, parameters apim.APICreateOrUpdateParameter, options *apim.APIClientBeginCreateOrUpdateOptions) (*runtime.Poller[apim.APIClientCreateOrUpdateResponse], error) {
	client := c.apimClientFactory.NewAPIClient()
	return client.BeginCreateOrUpdate(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, parameters, options)
}

func (c *APIMClient) DeleteApi(ctx context.Context, apiId string, etag string, options *apim.APIClientBeginDeleteOptions) (*runtime.Poller[apim.APIClientDeleteResponse], error) {
	client := c.apimClientFactory.NewAPIClient()
	return client.BeginDelete(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, etag, options)
}

func (c *APIMClient) GetApiPolicy(ctx context.Context, apiId string, options *apim.APIPolicyClientGetOptions) (apim.APIPolicyClientGetResponse, error) {
	client := c.apimClientFactory.NewAPIPolicyClient()
	return client.Get(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, apim.PolicyIDNamePolicy, options)
}

func (c *APIMClient) CreateUpdateApiPolicy(ctx context.Context, apiId string, parameters apim.PolicyContract, options *apim.APIPolicyClientCreateOrUpdateOptions) (apim.APIPolicyClientCreateOrUpdateResponse, error) {
	client := c.apimClientFactory.NewAPIPolicyClient()
	return client.CreateOrUpdate(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, apim.PolicyIDNamePolicy, parameters, options)
}

func (c *APIMClient) DeleteApiPolicy(ctx context.Context, apiId string, etag string, options *apim.APIPolicyClientDeleteOptions) (apim.APIPolicyClientDeleteResponse, error) {
	client := c.apimClientFactory.NewAPIPolicyClient()
	return client.Delete(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, apim.PolicyIDNamePolicy, etag, options)
}

func (c *APIMClient) GetBackend(ctx context.Context, backendId string, options *apim.BackendClientGetOptions) (apim.BackendClientGetResponse, error) {
	client := c.apimClientFactory.NewBackendClient()
	return client.Get(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, backendId, options)
}

func (c *APIMClient) CreateUpdateBackend(ctx context.Context, backendId string, parameters apim.BackendContract, options *apim.BackendClientCreateOrUpdateOptions) (apim.BackendClientCreateOrUpdateResponse, error) {
	client := c.apimClientFactory.NewBackendClient()
	return client.CreateOrUpdate(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, backendId, parameters, options)
}

func (c *APIMClient) DeleteBackend(ctx context.Context, backendId string, etag string, options *apim.BackendClientDeleteOptions) (apim.BackendClientDeleteResponse, error) {
	client := c.apimClientFactory.NewBackendClient()
	return client.Delete(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, backendId, etag, options)
}

func (c *APIMClient) GetApiDiagnosticSettings(ctx context.Context, apiId string, diagnosticsId string, options *apim.APIDiagnosticClientGetOptions) (apim.APIDiagnosticClientGetResponse, error) {
	client := c.apimClientFactory.NewAPIDiagnosticClient()
	return client.Get(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, diagnosticsId, options)
}

func (c *APIMClient) CreateUpdateApiDiagnosticSettings(ctx context.Context, apiId string, diagnosticsType DiagnosticsType, parameters apim.DiagnosticContract, options *apim.APIDiagnosticClientCreateOrUpdateOptions) (apim.APIDiagnosticClientCreateOrUpdateResponse, error) {
	client := c.apimClientFactory.NewAPIDiagnosticClient()
	return client.CreateOrUpdate(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, string(diagnosticsType), parameters, options)
}

func (c *APIMClient) DeleteApiDiagnosticSettings(ctx context.Context, apiId string, diagnosticsId string, etag string, options *apim.APIDiagnosticClientDeleteOptions) (apim.APIDiagnosticClientDeleteResponse, error) {
	client := c.apimClientFactory.NewAPIDiagnosticClient()
	return client.Delete(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, diagnosticsId, etag, options)
}

func (c *APIMClient) GetLoggerByName(ctx context.Context, loggerName string) (*string, error) {
	client := c.apimClientFactory.NewLoggerClient()
	pager := client.NewListByServicePager(c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, &apim.LoggerClientListByServiceOptions{
		// Remove all ' in loggerName input as this isn't a valid char in the name and can lead to query escape
		Filter: ptr.To(fmt.Sprintf("name eq '%s'", strings.ReplaceAll(loggerName, "'", ""))),
	})
	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		if len(page.Value) == 0 {
			return nil, fmt.Errorf("no logger found with name %s", loggerName)
		}
		if len(page.Value) > 1 {
			return nil, fmt.Errorf("multiple loggers found with name %s", loggerName)
		}
		for _, logger := range page.Value {
			if *logger.Name == loggerName {
				return logger.ID, nil
			}
		}
	}
	return nil, fmt.Errorf("no logger found with name %s", loggerName)
}

func IsNotFoundError(err error) bool {
	var responseError *azcore.ResponseError
	if errors.As(err, &responseError) {
		return responseError.StatusCode == http.StatusNotFound
	}
	return false
}

func IgnoreNotFound(err error) error {
	if IsNotFoundError(err) {
		return nil
	}
	return err
}
