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
	getResponse apim.BackendClientGetResponse,
	getErrorCode *int,
	createOrUpdateResponse apim.BackendClientCreateOrUpdateResponse,
	createOrUpdateErrorCode *int,
	deleteResponse apim.BackendClientDeleteResponse,
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

			response := createOrUpdateResponse

			responder := azfake.Responder[apim.BackendClientCreateOrUpdateResponse]{}

			errResponder := azfake.ErrorResponder{}
			if createOrUpdateErrorCode != nil {
				errResponder.SetResponseError(*createOrUpdateErrorCode, "Some fake error occurred")
			} else {
				response.Properties = parameters.Properties
			}
			responder.SetResponse(http.StatusOK, response, nil)

			return responder, errResponder
		},
		Delete: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			ifMatch string,
			options *apim.BackendClientDeleteOptions,
		) (azfake.Responder[apim.BackendClientDeleteResponse], azfake.ErrorResponder) {
			response := deleteResponse
			responder := azfake.Responder[apim.BackendClientDeleteResponse]{}

			responder.SetResponse(http.StatusOK, response, nil)

			errorResponder := azfake.ErrorResponder{}
			if deleteErrorCode != nil {
				errorResponder.SetResponseError(*deleteErrorCode, "Some fake error occurred")
			}
			return responder, errorResponder
		},
		Get: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			options *apim.BackendClientGetOptions,
		) (azfake.Responder[apim.BackendClientGetResponse], azfake.ErrorResponder) {
			response := getResponse
			responder := azfake.Responder[apim.BackendClientGetResponse]{}

			responder.SetResponse(http.StatusOK, response, nil)
			errResponder := azfake.ErrorResponder{}
			if getErrorCode != nil {
				errResponder.SetResponseError(*getErrorCode, "Some fake error occurred")
			}
			return responder, errResponder
		},
		GetEntityTag:          nil,
		NewListByServicePager: nil,
		Reconnect:             nil,
		Update:                nil,
	}
	return fakeServer
}
