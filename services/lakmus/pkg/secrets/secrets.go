package secrets

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azsecrets "github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// ListSecrets enumerates secret metadata in a Key Vault (no values are fetched).
// - vaultURL should be like: https://<kv-name>.vault.azure.net/
// - cred can be DefaultAzureCredential (workload identity) or any azcore.TokenCredential
// - opts is optional; pass nil for defaults
//
// Returns all SecretProperties across pages, in the order received from the service.
func ListSecrets(ctx context.Context, vaultURL string, cred azcore.TokenCredential, opts *azsecrets.ClientOptions) ([]*azsecrets.SecretProperties, error) {
	client, err := azsecrets.NewClient(vaultURL, cred, opts)
	if err != nil {
		return nil, err
	}

	pager := client.NewListSecretPropertiesPager(nil)

	var out []*azsecrets.SecretProperties
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		// Append properties from this page
		if page.SecretPropertiesListResult.Value != nil {
			out = append(out, page.SecretPropertiesListResult.Value...)
		}
	}

	return out, nil
}
