package azfakes

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	networkfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7/fake"
)

// NetworkEnv bundles a fake credential and ARM client options.
// armnetwork clients built with them talk to the fake servers.
type NetworkEnv struct {
	Cred azcore.TokenCredential
	ARM  *arm.ClientOptions
}

// NewNetworkEnv builds an env for the armnetwork fakes.
// SubnetsClient uses subnetsSrv. VirtualNetworksClient uses vnetSrv.
// A nil server leaves that client without a fake. Calls to it fail.
func NewNetworkEnv(
	subnetsSrv *networkfake.SubnetsServer,
	vnetSrv *networkfake.VirtualNetworksServer,
) *NetworkEnv {
	factory := &networkfake.ServerFactory{}
	if subnetsSrv != nil {
		factory.SubnetsServer = *subnetsSrv
	}
	if vnetSrv != nil {
		factory.VirtualNetworksServer = *vnetSrv
	}

	return &NetworkEnv{
		Cred: &azfake.TokenCredential{},
		ARM: &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: networkfake.NewServerFactoryTransport(factory),
			},
		},
	}
}
