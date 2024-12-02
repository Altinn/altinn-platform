package utils

import (
	"context"
	"net/http"

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/azure"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	apimfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2/fake"
)

// NewAPIMClient creates a new APIMClient
func NewFakeAPIMClient(config *azure.ApimClientConfig) (*azure.APIMClient, error) {
	clientFactory, err := apim.NewClientFactory(config.SubscriptionId, nil, config.FactoryOptions)
	if err != nil {
		return nil, err
	}
	return azure.NewApimClientWithFactory(config, clientFactory), nil
}

func GetFakeBackendServer(
	getResponse *apim.BackendClientGetResponse,
	getErrorCode *int,
	createOrUpdateResponse *apim.BackendClientCreateOrUpdateResponse,
	createOrUpdateErrorCode *int,
	deleteResponse *apim.BackendClientDeleteResponse,
	deleteErrorCode *int,
) *apimfake.BackendServer {
	fakeServer := &apimfake.BackendServer{
		CreateOrUpdate: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			parameters apim.BackendContract,
			options *apim.BackendClientCreateOrUpdateOptions,
		) (azfake.Responder[apim.BackendClientCreateOrUpdateResponse], azfake.ErrorResponder) {
			return genericFakeResponder[apim.BackendClientCreateOrUpdateResponse](createOrUpdateResponse, createOrUpdateErrorCode)
		},
		Delete: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			ifMatch string,
			options *apim.BackendClientDeleteOptions,
		) (azfake.Responder[apim.BackendClientDeleteResponse], azfake.ErrorResponder) {
			return genericFakeResponder[apim.BackendClientDeleteResponse](deleteResponse, deleteErrorCode)
		},
		Get: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			options *apim.BackendClientGetOptions,
		) (azfake.Responder[apim.BackendClientGetResponse], azfake.ErrorResponder) {
			return genericFakeResponder[apim.BackendClientGetResponse](getResponse, getErrorCode)
		},
		GetEntityTag:          nil,
		NewListByServicePager: nil,
		Reconnect:             nil,
		Update:                nil,
	}
	return fakeServer
}

func GetFakeApiVersionSetClient(
	creaateOrUpdateResponse *apim.APIVersionSetClientCreateOrUpdateResponse,
	createOrUpdateErrorCode *int,
	getResponse *apim.APIVersionSetClientGetResponse,
	getErrorCode *int,
	deleteResponse *apim.APIVersionSetClientDeleteResponse,
	deleteErrorCode *int,
	createCallback func(input apim.APIVersionSetContract, errorResponse bool),
	getCallback func(versionSetID string, errorResponse bool),
) *apimfake.APIVersionSetServer {
	return &apimfake.APIVersionSetServer{
		CreateOrUpdate: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			versionSetID string,
			parameters apim.APIVersionSetContract,
			options *apim.APIVersionSetClientCreateOrUpdateOptions,
		) (
			azfake.Responder[apim.APIVersionSetClientCreateOrUpdateResponse],
			azfake.ErrorResponder,
		) {
			if createCallback != nil {
				createCallback(parameters, createOrUpdateErrorCode != nil)
			}
			return genericFakeResponder[apim.APIVersionSetClientCreateOrUpdateResponse](creaateOrUpdateResponse, createOrUpdateErrorCode)
		},
		Delete: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			versionSetID string,
			ifMatch string,
			options *apim.APIVersionSetClientDeleteOptions,
		) (
			azfake.Responder[apim.APIVersionSetClientDeleteResponse],
			azfake.ErrorResponder,
		) {
			return genericFakeResponder[apim.APIVersionSetClientDeleteResponse](deleteResponse, deleteErrorCode)
		},
		Get: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			versionSetID string,
			options *apim.APIVersionSetClientGetOptions,
		) (
			azfake.Responder[apim.APIVersionSetClientGetResponse],
			azfake.ErrorResponder,
		) {
			if getCallback != nil {
				getCallback(versionSetID, getErrorCode != nil)
			}
			return genericFakeResponder[apim.APIVersionSetClientGetResponse](getResponse, getErrorCode)
		},
		GetEntityTag:          nil,
		Update:                nil,
		NewListByServicePager: nil,
	}
}

func GetFakeApiClient(
	beginCreateUpdateResponse *apim.APIClientCreateOrUpdateResponse,
	beginCreateUpdateErrorCode *int,
	getResponse *apim.APIClientGetResponse,
	getErrorCode *int,
	deleteResponse *apim.APIClientDeleteResponse,
	deleteErrorCode *int,
) *apimfake.APIServer {
	fakeServer := &apimfake.APIServer{
		BeginCreateOrUpdate: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			apiID string,
			parameters apim.APICreateOrUpdateParameter,
			options *apim.APIClientBeginCreateOrUpdateOptions,
		) (
			azfake.PollerResponder[apim.APIClientCreateOrUpdateResponse],
			azfake.ErrorResponder,
		) {
			return genericFakePollerResponder[apim.APIClientCreateOrUpdateResponse](beginCreateUpdateResponse, beginCreateUpdateErrorCode)
		},
		Delete: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			apiID string,
			ifMatch string,
			options *apim.APIClientDeleteOptions,
		) (
			azfake.Responder[apim.APIClientDeleteResponse],
			azfake.ErrorResponder,
		) {
			return genericFakeResponder[apim.APIClientDeleteResponse](deleteResponse, deleteErrorCode)
		},
		Get: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			apiID string,
			options *apim.APIClientGetOptions,
		) (
			azfake.Responder[apim.APIClientGetResponse],
			azfake.ErrorResponder,
		) {
			return genericFakeResponder[apim.APIClientGetResponse](getResponse, getErrorCode)
		},
		GetEntityTag:          nil,
		NewListByServicePager: nil,
		NewListByTagsPager:    nil,
		Update:                nil,
	}
	return fakeServer
}

func genericFakeResponder[T any](response *T, errorCode *int) (azfake.Responder[T], azfake.ErrorResponder) {
	responder := azfake.Responder[T]{}
	errResponder := azfake.ErrorResponder{}
	if errorCode != nil {
		errResponder.SetResponseError(*errorCode, "Some fake error occurred")
	}
	if response != nil {
		responder.SetResponse(http.StatusOK, *response, nil)
	}
	return responder, errResponder
}

func genericFakePollerResponder[T any](response *T, errorCode *int) (azfake.PollerResponder[T], azfake.ErrorResponder) {
	responder := azfake.PollerResponder[T]{}
	errResponder := azfake.ErrorResponder{}
	if errorCode != nil {
		errResponder.SetResponseError(*errorCode, "Some fake error occurred")
	}
	if response != nil {
		responder.SetTerminalResponse(http.StatusOK, *response, nil)
	}
	return responder, errResponder
}
