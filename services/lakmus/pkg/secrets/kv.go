package secrets

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	armkeyvault "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
)

type VaultInfo struct {
	ID            string
	Name          string
	ResourceGroup string
	Location      string
	VaultURI      string
}

// ListKeyVaults enumerates all Microsoft.KeyVault/vaults in a subscription.
// - cred should be a Workload Identity-capable credential (e.g., azidentity.NewDefaultAzureCredential(nil)).
// - opts is optional; pass nil for default Azure public cloud.
func ListKeyVaults(ctx context.Context, subscriptionID string, cred azcore.TokenCredential, opts *arm.ClientOptions) ([]VaultInfo, error) {
	client, err := armkeyvault.NewVaultsClient(subscriptionID, cred, opts)
	if err != nil {
		return nil, err
	}

	pager := client.NewListBySubscriptionPager(nil)

	out := []VaultInfo{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Value {
			var vi VaultInfo

			if v.ID != nil {
				vi.ID = *v.ID
				if rid, perr := arm.ParseResourceID(vi.ID); perr == nil {
					vi.ResourceGroup = rid.ResourceGroupName
				}
			}
			if v.Name != nil {
				vi.Name = *v.Name
			}
			if v.Location != nil {
				vi.Location = *v.Location
			}
			if v.Properties != nil && v.Properties.VaultURI != nil {
				vi.VaultURI = *v.Properties.VaultURI
			}

			out = append(out, vi)
		}
	}

	return out, nil
}
