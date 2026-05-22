package azfakes

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	networkfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7/fake"
)

// NetworkEnv bundles a fake credential + ARM client options
// so armnetwork clients talk to the fake SubnetsServer.
type NetworkEnv struct {
	Cred azcore.TokenCredential
	ARM  *arm.ClientOptions
}

// NewNetworkEnv builds an env where armnetwork.SubnetsClient will use the
// given fake SubnetsServer instead of real HTTP calls.
func NewNetworkEnv(subnetsSrv *networkfake.SubnetsServer) *NetworkEnv {
	env := &NetworkEnv{

		Cred: &azfake.TokenCredential{},
	}

	if subnetsSrv != nil {
		env.ARM = &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: networkfake.NewSubnetsServerTransport(subnetsSrv),
			},
		}
	} else {
		env.ARM = &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{},
		}
	}

	return env
}
