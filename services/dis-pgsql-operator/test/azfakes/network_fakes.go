package azfakes

import (
	"fmt"
	"net/http"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armnetwork "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	networkfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7/fake"
)

// SubnetsServerOneVNet returns a fake SubnetsServer that serves a single page
// containing all the given CIDRs as subnets.
func SubnetsServerOneVNet(subnetCIDRs []string) *networkfake.SubnetsServer {
	return &networkfake.SubnetsServer{
		// NewListPager is the fake for SubnetsClient.NewListPager.
		NewListPager: func(resourceGroupName string, virtualNetworkName string, options *armnetwork.SubnetsClientListOptions) (resp azfake.PagerResponder[armnetwork.SubnetsClientListResponse]) {
			// Build the slice of fake subnets.
			subnets := make([]*armnetwork.Subnet, 0, len(subnetCIDRs))
			for i, cidr := range subnetCIDRs {
				name := fmt.Sprintf("subnet-fake-%d", i)
				subnets = append(subnets, &armnetwork.Subnet{
					Name: to.Ptr(name),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr(cidr),
					},
				})
			}

			// One page containing all subnets.
			page := armnetwork.SubnetsClientListResponse{
				SubnetListResult: armnetwork.SubnetListResult{
					Value: subnets,
				},
			}

			// Configure the pager with a single HTTP 200 page.
			resp.AddPage(http.StatusOK, page, nil)
			return
		},
	}
}
