package utils

import (
	"context"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	apimfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2/fake"
	"net/http"
)

func GetFakeBackendServer(getResponse apim.BackendClientGetResponse, getErrorCode *int, createOrUpdateResponse apim.BackendClientCreateOrUpdateResponse, createOrUpdateErrorCode *int, deleteResponse apim.BackendClientDeleteResponse, deleteErrorCode *int) *apimfake.BackendServer {
	fakeServer := &apimfake.BackendServer{
		CreateOrUpdate: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			parameters apim.BackendContract,
			options *apim.BackendClientCreateOrUpdateOptions,
		) (resp azfake.Responder[apim.BackendClientCreateOrUpdateResponse], errResp azfake.ErrorResponder) {

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
		) (resp azfake.Responder[apim.BackendClientDeleteResponse], errResp azfake.ErrorResponder) {
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
		) (resp azfake.Responder[apim.BackendClientGetResponse], errResp azfake.ErrorResponder) {
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
