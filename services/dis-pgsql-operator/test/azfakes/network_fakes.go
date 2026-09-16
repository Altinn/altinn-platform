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
// containing all the given CIDRs as subnets, each carrying its prefix in the
// singular addressPrefix field.
func SubnetsServerOneVNet(subnetCIDRs []string) *networkfake.SubnetsServer {
	subnets := make([]*armnetwork.Subnet, 0, len(subnetCIDRs))
	for i, cidr := range subnetCIDRs {
		subnets = append(subnets, &armnetwork.Subnet{
			Name: to.Ptr(fmt.Sprintf("subnet-fake-%d", i)),
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix: to.Ptr(cidr),
			},
		})
	}

	return SubnetsServerWithSubnets(subnets)
}

// SubnetsServerWithSubnets returns a fake SubnetsServer that serves the given
// subnets as a single page.
//
// Use it when a test needs to control how each subnet carries its prefix —
// Azure returns either the singular addressPrefix or a one-element
// addressPrefixes array depending on which form created the subnet.
func SubnetsServerWithSubnets(subnets []*armnetwork.Subnet) *networkfake.SubnetsServer {
	return &networkfake.SubnetsServer{
		// NewListPager is the fake for SubnetsClient.NewListPager.
		NewListPager: func(
			resourceGroupName string,
			virtualNetworkName string,
			options *armnetwork.SubnetsClientListOptions,
		) (resp azfake.PagerResponder[armnetwork.SubnetsClientListResponse]) {
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
