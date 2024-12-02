package utils

import (
	"context"
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	apimfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2/fake"
	"net/http"
)

type AzureApimFake struct {
	APIMVersionSets map[string]SimpleApimApiVersionSet
	APIMVersions    map[string]SimpleApimApiVersion
	Backends        map[string]apim.BackendContract
}

type SimpleApimApiVersionSet struct {
	ApiVersionSetId   string
	ApiVersionSetName string
}

type SimpleApimApiVersion struct {
	ApiVersionId   string
	ApiVersionName string
	ApiContent     string
}

type SimpleApimBackend struct {
	BackendId   string
	BackendName string
	BackendURL  string
}

type FakeClientType string

const (
	ApiVersionSetClient FakeClientType = "ApiVersionSetClient"
	ApiVersionClient    FakeClientType = "ApiVersionClient"
	BackendClient       FakeClientType = "BackendClient"
)

// NewFakeAPIMClient creates a new APIMClient
func NewFakeAPIMClientStruct() *AzureApimFake {
	return &AzureApimFake{
		APIMVersionSets: map[string]SimpleApimApiVersionSet{},
		APIMVersions:    map[string]SimpleApimApiVersion{},
		Backends:        map[string]apim.BackendContract{},
	}
}

func (a *AzureApimFake) GetFakeBackendServer(createUpdateServerError bool, getServerError bool, deleteServerError bool) *apimfake.BackendServer {
	fakeServer := &apimfake.BackendServer{
		CreateOrUpdate: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			parameters apim.BackendContract,
			options *apim.BackendClientCreateOrUpdateOptions,
		) (azfake.Responder[apim.BackendClientCreateOrUpdateResponse], azfake.ErrorResponder) {
			responder := azfake.Responder[apim.BackendClientCreateOrUpdateResponse]{}
			errResponder := azfake.ErrorResponder{}
			if createUpdateServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.BackendClientCreateOrUpdateResponse{
					BackendContract: apim.BackendContract{
						ID:         utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Backend/" + backendID),
						Name:       utils.ToPointer(backendID),
						Type:       utils.ToPointer("Microsoft.ApiManagement/service/backends"),
						Properties: parameters.Properties,
					},
				}
				a.Backends[*response.Name] = response.BackendContract
				responder.SetResponse(http.StatusOK, response, nil)
			}
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
			responder := azfake.Responder[apim.BackendClientDeleteResponse]{}
			errResponder := azfake.ErrorResponder{}
			if deleteServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.BackendClientDeleteResponse{}
				if _, ok := a.Backends[backendID]; ok {
					delete(a.Backends, backendID)
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "Backend not found")
				}
			}
			return responder, errResponder
		},
		Get: func(
			ctx context.Context,
			resourceGroupName string,
			serviceName string,
			backendID string,
			options *apim.BackendClientGetOptions,
		) (azfake.Responder[apim.BackendClientGetResponse], azfake.ErrorResponder) {
			responder := azfake.Responder[apim.BackendClientGetResponse]{}
			errResponder := azfake.ErrorResponder{}
			if getServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.BackendClientGetResponse{}
				if _, ok := a.Backends[backendID]; ok {
					response.BackendContract = a.Backends[backendID]
					response.ETag = utils.ToPointer("fake-etag")
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "Backend not found")
				}
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

func (a *AzureApimFake) genericFakeResponder(returnServerError bool) {

}
