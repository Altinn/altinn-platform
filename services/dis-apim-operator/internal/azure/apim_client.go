package azure

import (
	"context"
	"errors"
	"net/http"

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
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
func NewAPIMClient(config *ApimClientConfig) (*APIMClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(config.ClientOptions)
	if err != nil {
		return nil, err
	}
	clientFactory, err := apim.NewClientFactory(config.SubscriptionId, credential, config.FactoryOptions)
	if err != nil {
		return nil, err
	}
	return &APIMClient{
		ApimClientConfig:  *config,
		apimClientFactory: clientFactory,
	}, nil
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

func (c *APIMClient) DeleteApi(ctx context.Context, apiId string, etag string, options *apim.APIClientDeleteOptions) (apim.APIClientDeleteResponse, error) {
	client := c.apimClientFactory.NewAPIClient()
	return client.Delete(ctx, c.ApimClientConfig.ResourceGroup, c.ApimClientConfig.ApimServiceName, apiId, etag, options)
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
