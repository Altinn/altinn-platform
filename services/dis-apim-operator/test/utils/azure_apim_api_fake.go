package utils

import (
	"context"
	"net/http"
	"regexp"

	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/utils"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3"
	apimfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v3/fake"
)

type AzureApimFake struct {
	APIMVersionSets         map[string]apim.APIVersionSetContract
	APIMVersions            map[string]apim.APIContract
	Backends                map[string]apim.BackendContract
	Policies                map[string]apim.PolicyContract
	ApiDiagnostics          map[string]apim.DiagnosticContract
	FakeApiServer           apimfake.APIServer
	FakeApiVersionServer    apimfake.APIVersionSetServer
	FakeBackendServer       apimfake.BackendServer
	FakeApiPolicyServer     apimfake.APIPolicyServer
	FakeApiDiagnosticServer apimfake.APIDiagnosticServer
	FakeLoggerServer        apimfake.LoggerServer
	createUpdateServerError bool
	getServerError          bool
	deleteServerError       bool
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

// NewFakeAPIMClient creates a new APIMClient
func NewFakeAPIMClientStruct() AzureApimFake {
	aaf := AzureApimFake{
		APIMVersionSets:         map[string]apim.APIVersionSetContract{},
		APIMVersions:            map[string]apim.APIContract{},
		Backends:                map[string]apim.BackendContract{},
		Policies:                map[string]apim.PolicyContract{},
		ApiDiagnostics:          map[string]apim.DiagnosticContract{},
		createUpdateServerError: false,
		deleteServerError:       false,
		getServerError:          false,
	}
	aaf.FakeApiServer = aaf.GetFakeApiServer()
	aaf.FakeApiVersionServer = aaf.GetFakeApiVersionServer()
	aaf.FakeBackendServer = aaf.GetFakeBackendServer()
	aaf.FakeApiPolicyServer = aaf.GetFakeAPIPolicyServer()
	aaf.FakeApiDiagnosticServer = aaf.GetFakeApiDiagnosticServer()
	aaf.FakeLoggerServer = aaf.GetFakeLoggerServer()
	return aaf
}

func (a *AzureApimFake) GetFakeBackendServer() apimfake.BackendServer {
	fakeServer := apimfake.BackendServer{
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
			if a.createUpdateServerError {
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
			if a.deleteServerError {
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
			if a.getServerError {
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

func (a *AzureApimFake) GetFakeApiServer() apimfake.APIServer {
	fakeServer := apimfake.APIServer{
		BeginCreateOrUpdate: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, parameters apim.APICreateOrUpdateParameter, options *apim.APIClientBeginCreateOrUpdateOptions) (azfake.PollerResponder[apim.APIClientCreateOrUpdateResponse], azfake.ErrorResponder) {
			responder := azfake.PollerResponder[apim.APIClientCreateOrUpdateResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.createUpdateServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIClientCreateOrUpdateResponse{
					APIContract: apim.APIContract{
						ID:   utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Api/" + apiID),
						Name: utils.ToPointer(apiID),
						Type: utils.ToPointer("Microsoft.ApiManagement/service/apis"),
						Properties: &apim.APIContractProperties{
							Path:                          parameters.Properties.Path,
							APIRevision:                   parameters.Properties.APIRevision,
							APIRevisionDescription:        parameters.Properties.APIRevisionDescription,
							APIType:                       parameters.Properties.APIType,
							APIVersion:                    parameters.Properties.APIVersion,
							APIVersionDescription:         parameters.Properties.APIVersionDescription,
							APIVersionSet:                 parameters.Properties.APIVersionSet,
							APIVersionSetID:               parameters.Properties.APIVersionSetID,
							AuthenticationSettings:        parameters.Properties.AuthenticationSettings,
							Contact:                       parameters.Properties.Contact,
							Description:                   parameters.Properties.Description,
							DisplayName:                   parameters.Properties.DisplayName,
							IsCurrent:                     parameters.Properties.IsCurrent,
							License:                       parameters.Properties.License,
							Protocols:                     parameters.Properties.Protocols,
							ServiceURL:                    parameters.Properties.ServiceURL,
							SourceAPIID:                   parameters.Properties.SourceAPIID,
							SubscriptionKeyParameterNames: parameters.Properties.SubscriptionKeyParameterNames,
							SubscriptionRequired:          parameters.Properties.SubscriptionRequired,
							TermsOfServiceURL:             parameters.Properties.TermsOfServiceURL,
							IsOnline:                      parameters.Properties.IsOnline,
						},
					},
				}
				a.APIMVersions[*response.Name] = response.APIContract
				responder.SetTerminalResponse(http.StatusOK, response, nil)
			}
			return responder, errResponder
		},
		BeginDelete: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, ifMatch string, options *apim.APIClientBeginDeleteOptions) (azfake.PollerResponder[apim.APIClientDeleteResponse], azfake.ErrorResponder) {
			responder := azfake.PollerResponder[apim.APIClientDeleteResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.deleteServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIClientDeleteResponse{}
				delete(a.APIMVersions, apiID)
				responder.SetTerminalResponse(http.StatusAccepted, response, nil)
			}
			return responder, errResponder
		},
		Get: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, options *apim.APIClientGetOptions) (azfake.Responder[apim.APIClientGetResponse], azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIClientGetResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.getServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIClientGetResponse{}
				if _, ok := a.APIMVersions[apiID]; ok {
					response.APIContract = a.APIMVersions[apiID]
					response.ETag = utils.ToPointer("fake-etag")
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "API not found")
				}
			}
			return responder, errResponder
		},
		GetEntityTag:          nil,
		NewListByServicePager: nil,
		NewListByTagsPager:    nil,
		Update:                nil,
	}
	return fakeServer
}

func (a *AzureApimFake) GetFakeApiVersionServer() apimfake.APIVersionSetServer {
	fakeServer := apimfake.APIVersionSetServer{
		CreateOrUpdate: func(ctx context.Context, resourceGroupName string, serviceName string, apiVersionSetID string, parameters apim.APIVersionSetContract, options *apim.APIVersionSetClientCreateOrUpdateOptions) (azfake.Responder[apim.APIVersionSetClientCreateOrUpdateResponse], azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIVersionSetClientCreateOrUpdateResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.createUpdateServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIVersionSetClientCreateOrUpdateResponse{
					APIVersionSetContract: apim.APIVersionSetContract{
						ID:         utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/ApiVersionSet/" + apiVersionSetID),
						Name:       utils.ToPointer(apiVersionSetID),
						Type:       utils.ToPointer("Microsoft.ApiManagement/service/apiVersionSets"),
						Properties: parameters.Properties,
					},
				}
				a.APIMVersionSets[*response.Name] = response.APIVersionSetContract
				responder.SetResponse(http.StatusOK, response, nil)
			}
			return responder, errResponder
		},
		Delete: func(ctx context.Context, resourceGroupName string, serviceName string, apiVersionSetID string, ifMatch string, options *apim.APIVersionSetClientDeleteOptions) (azfake.Responder[apim.APIVersionSetClientDeleteResponse], azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIVersionSetClientDeleteResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.deleteServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIVersionSetClientDeleteResponse{}
				if _, ok := a.APIMVersionSets[apiVersionSetID]; ok {
					delete(a.APIMVersionSets, apiVersionSetID)
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "APIVersionSet not found")
				}
			}
			return responder, errResponder
		},
		Get: func(ctx context.Context, resourceGroupName string, serviceName string, apiVersionSetID string, options *apim.APIVersionSetClientGetOptions) (azfake.Responder[apim.APIVersionSetClientGetResponse], azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIVersionSetClientGetResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.getServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIVersionSetClientGetResponse{}
				if _, ok := a.APIMVersionSets[apiVersionSetID]; ok {
					response.APIVersionSetContract = a.APIMVersionSets[apiVersionSetID]
					response.ETag = utils.ToPointer("fake-etag")
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "APIVersionSet not found")
				}
			}
			return responder, errResponder
		},
		GetEntityTag:          nil,
		NewListByServicePager: nil,
		Update:                nil,
	}
	return fakeServer
}

func (a *AzureApimFake) GetFakeAPIPolicyServer() apimfake.APIPolicyServer {
	fakeServer := apimfake.APIPolicyServer{
		CreateOrUpdate: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, policyID apim.PolicyIDName, parameters apim.PolicyContract, options *apim.APIPolicyClientCreateOrUpdateOptions) (resp azfake.Responder[apim.APIPolicyClientCreateOrUpdateResponse], errResp azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIPolicyClientCreateOrUpdateResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.createUpdateServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIPolicyClientCreateOrUpdateResponse{
					PolicyContract: apim.PolicyContract{
						ID:         utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Api/" + apiID + "/policies/" + string(policyID)),
						Name:       utils.ToPointer(string(policyID)),
						Type:       utils.ToPointer("Microsoft.ApiManagement/service/apis/policies"),
						Properties: parameters.Properties,
					},
				}
				a.Policies[*response.Name] = response.PolicyContract
				responder.SetResponse(http.StatusOK, response, nil)
			}
			return responder, errResponder
		},
		Delete: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, policyID apim.PolicyIDName, ifMatch string, options *apim.APIPolicyClientDeleteOptions) (resp azfake.Responder[apim.APIPolicyClientDeleteResponse], errResp azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIPolicyClientDeleteResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.deleteServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIPolicyClientDeleteResponse{}
				if _, ok := a.Policies[string(policyID)]; ok {
					delete(a.Policies, string(policyID))
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "Policy not found")
				}
			}
			return responder, errResponder
		},
		Get: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, policyID apim.PolicyIDName, options *apim.APIPolicyClientGetOptions) (resp azfake.Responder[apim.APIPolicyClientGetResponse], errResp azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIPolicyClientGetResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.getServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIPolicyClientGetResponse{}
				if _, ok := a.Policies[string(policyID)]; ok {
					response.PolicyContract = a.Policies[string(policyID)]
					response.ETag = utils.ToPointer("fake-etag")
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "Policy not found")
				}
			}
			return responder, errResponder
		},
		GetEntityTag: nil,
		ListByAPI:    nil,
	}
	return fakeServer
}

func (a *AzureApimFake) GetFakeApiDiagnosticServer() apimfake.APIDiagnosticServer {
	fakeServer := apimfake.APIDiagnosticServer{
		CreateOrUpdate: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, diagnosticID string, parameters apim.DiagnosticContract, options *apim.APIDiagnosticClientCreateOrUpdateOptions) (resp azfake.Responder[apim.APIDiagnosticClientCreateOrUpdateResponse], errResp azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIDiagnosticClientCreateOrUpdateResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.createUpdateServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIDiagnosticClientCreateOrUpdateResponse{
					DiagnosticContract: apim.DiagnosticContract{
						ID:         utils.ToPointer("/subscriptions/fake-subscription/resourceGroups/fake-resource-group/providers/APIM/Api/" + apiID + "/diagnostics/" + diagnosticID),
						Name:       utils.ToPointer(diagnosticID),
						Type:       utils.ToPointer("Microsoft.ApiManagement/service/apis/diagnostics"),
						Properties: parameters.Properties,
					},
				}
				a.ApiDiagnostics[*response.Name] = response.DiagnosticContract
				responder.SetResponse(http.StatusOK, response, nil)
			}
			return responder, errResponder
		},
		Delete: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, diagnosticID string, ifMatch string, options *apim.APIDiagnosticClientDeleteOptions) (resp azfake.Responder[apim.APIDiagnosticClientDeleteResponse], errResp azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIDiagnosticClientDeleteResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.deleteServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIDiagnosticClientDeleteResponse{}
				if _, ok := a.ApiDiagnostics[diagnosticID]; ok {
					delete(a.ApiDiagnostics, diagnosticID)
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "Diagnostic not found")
				}
			}
			return responder, errResponder
		},
		Get: func(ctx context.Context, resourceGroupName string, serviceName string, apiID string, diagnosticID string, options *apim.APIDiagnosticClientGetOptions) (resp azfake.Responder[apim.APIDiagnosticClientGetResponse], errResp azfake.ErrorResponder) {
			responder := azfake.Responder[apim.APIDiagnosticClientGetResponse]{}
			errResponder := azfake.ErrorResponder{}
			if a.getServerError {
				errResponder.SetResponseError(http.StatusInternalServerError, "Some fake internal server error occurred")
			} else {
				response := apim.APIDiagnosticClientGetResponse{}
				if _, ok := a.ApiDiagnostics[diagnosticID]; ok {
					response.DiagnosticContract = a.ApiDiagnostics[diagnosticID]
					response.ETag = utils.ToPointer("fake-etag")
					responder.SetResponse(http.StatusOK, response, nil)
				} else {
					errResponder.SetResponseError(http.StatusNotFound, "Diagnostic not found")
				}
			}
			return responder, errResponder
		},
		GetEntityTag:          nil,
		NewListByServicePager: nil,
		Update:                nil,
	}
	return fakeServer
}

func (a *AzureApimFake) GetFakeLoggerServer() apimfake.LoggerServer {
	fakeServer := apimfake.LoggerServer{
		CreateOrUpdate: nil,
		Delete:         nil,
		Get:            nil,
		GetEntityTag:   nil,
		NewListByServicePager: func(resourceGroupName string, serviceName string, options *apim.LoggerClientListByServiceOptions) (resp azfake.PagerResponder[apim.LoggerClientListByServiceResponse]) {
			responder := azfake.PagerResponder[apim.LoggerClientListByServiceResponse]{}
			response := apim.LoggerClientListByServiceResponse{}
			filterRegex := regexp.MustCompile(`name eq '(.*)'`)
			if options.Filter != nil {
				matches := filterRegex.FindStringSubmatch(*options.Filter)
				if len(matches) > 1 {
					response.Value = []*apim.LoggerContract{
						{
							ID:   utils.ToPointer("subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/apim-fake/providers/Microsoft.ApiManagement/service/apim-fake/loggers/fake-logger"),
							Name: &matches[1],
							Type: nil,
						},
					}
				}
			}
			responder.AddPage(http.StatusOK, response, nil)
			return responder
		},
		Update: nil,
	}
	return fakeServer
}
