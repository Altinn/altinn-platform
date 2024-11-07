package utils

import (
	"context"
	"net/http"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	armalertsmanagement "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement"
	alertsmanagement_fake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/alertsmanagement/armalertsmanagement/fake"
	armresources "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	armresources_fake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/fake"
)

func NewFakeDeploymentsServer() armresources_fake.DeploymentsServer {
	return armresources_fake.DeploymentsServer{
		BeginCreateOrUpdate: func(ctx context.Context, resourceGroupName, deploymentName string, parameters armresources.Deployment, options *armresources.DeploymentsClientBeginCreateOrUpdateOptions) (resp azfake.PollerResponder[armresources.DeploymentsClientCreateOrUpdateResponse], errResp azfake.ErrorResponder) {
			// Set the values for the success response
			resp.SetTerminalResponse(http.StatusCreated, armresources.DeploymentsClientCreateOrUpdateResponse{}, nil)
			// Set the values for the error response; mutually exclusive. If both configured, the error response prevails
			// errResp.SetResponseError(http.StatusBadRequest, "ThisIsASimulatedError")
			return
		},
	}
}

func NewFakeNewPrometheusRuleGroupsServer() alertsmanagement_fake.PrometheusRuleGroupsServer {
	return alertsmanagement_fake.PrometheusRuleGroupsServer{
		Delete: func(ctx context.Context, resourceGroupName, ruleGroupName string, options *armalertsmanagement.PrometheusRuleGroupsClientDeleteOptions) (resp azfake.Responder[armalertsmanagement.PrometheusRuleGroupsClientDeleteResponse], errResp azfake.ErrorResponder) {
			resp.SetResponse(http.StatusOK, armalertsmanagement.PrometheusRuleGroupsClientDeleteResponse{}, nil)
			// errResp.SetResponseError(http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		},
	}
}
