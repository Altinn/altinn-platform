package utils

import (
	"github.com/Altinn/altinn-platform/services/dis-apim-operator/internal/azure"
	apim "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
)

// NewAPIMClient creates a new APIMClient
func NewFakeAPIMClient(config *azure.ApimClientConfig) (*azure.APIMClient, error) {
	clientFactory, err := apim.NewClientFactory(config.SubscriptionId, nil, config.FactoryOptions)
	if err != nil {
		return nil, err
	}
	return azure.NewApimClientWithFactory(config, clientFactory), nil
}
