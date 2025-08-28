package azfakes

import (
	"net/http"
	"time"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	armkeyvault "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	kvfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault/fake"
	azsecrets "github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	secfake "github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets/fake"
)

// VaultsServerTwoPages returns a fake Vaults server that paginates 3 vaults across 2 pages.
func VaultsServerTwoPages() kvfake.VaultsServer {
	return kvfake.VaultsServer{
		NewListBySubscriptionPager: func(*armkeyvault.VaultsClientListBySubscriptionOptions) (resp azfake.PagerResponder[armkeyvault.VaultsClientListBySubscriptionResponse]) {
			// Page 1: kv-one, kv-two
			resp.AddPage(http.StatusOK, armkeyvault.VaultsClientListBySubscriptionResponse{
				VaultListResult: armkeyvault.VaultListResult{
					Value: []*armkeyvault.Vault{
						{
							Name:     to.Ptr("kv-one"),
							ID:       to.Ptr("/subscriptions/0000/resourceGroups/rg1/providers/Microsoft.KeyVault/vaults/kv-one"),
							Location: to.Ptr("westeurope"),
							Properties: &armkeyvault.VaultProperties{
								VaultURI: to.Ptr("https://kv-one.vault.azure.net/"),
							},
						},
						{
							Name:     to.Ptr("kv-two"),
							ID:       to.Ptr("/subscriptions/0000/resourceGroups/rg2/providers/Microsoft.KeyVault/vaults/kv-two"),
							Location: to.Ptr("northeurope"),
							Properties: &armkeyvault.VaultProperties{
								VaultURI: to.Ptr("https://kv-two.vault.azure.net/"),
							},
						},
					},
				},
			}, nil)

			// Page 2: kv-three
			resp.AddPage(http.StatusOK, armkeyvault.VaultsClientListBySubscriptionResponse{
				VaultListResult: armkeyvault.VaultListResult{
					Value: []*armkeyvault.Vault{
						{
							Name:     to.Ptr("kv-three"),
							ID:       to.Ptr("/subscriptions/0000/resourceGroups/rg3/providers/Microsoft.KeyVault/vaults/kv-three"),
							Location: to.Ptr("westeurope"),
							Properties: &armkeyvault.VaultProperties{
								VaultURI: to.Ptr("https://kv-three.vault.azure.net/"),
							},
						},
					},
				},
			}, nil)

			return
		},
	}
}

// VaultsServerError: pager immediately errors with given status/message.
func VaultsServerError(status int, msg string) kvfake.VaultsServer {
	return kvfake.VaultsServer{
		NewListBySubscriptionPager: func(*armkeyvault.VaultsClientListBySubscriptionOptions) (resp azfake.PagerResponder[armkeyvault.VaultsClientListBySubscriptionResponse]) {
			resp.AddResponseError(status, msg)
			return
		},
	}
}

// SecretsServerExample returns a fake data-plane server with two pages — EXACTLY your pattern.
func SecretsServerExample() secfake.Server {
	return secfake.Server{
		NewListSecretPropertiesPager: func(options *azsecrets.ListSecretPropertiesOptions) (resp azfake.PagerResponder[azsecrets.ListSecretPropertiesResponse]) {
			// page 1
			p1 := azsecrets.ListSecretPropertiesResponse{
				SecretPropertiesListResult: azsecrets.SecretPropertiesListResult{
					Value: []*azsecrets.SecretProperties{
						{
							ID: to.Ptr(azsecrets.ID("https://kv-example.vault.azure.net/secrets/a")),
							Attributes: &azsecrets.SecretAttributes{
								Expires: to.Ptr(time.Unix(2_000, 0).UTC()),
								Enabled: to.Ptr(true),
							},
						},
						{
							ID: to.Ptr(azsecrets.ID("https://kv-example.vault.azure.net/secrets/b")),
							Attributes: &azsecrets.SecretAttributes{
								Enabled: to.Ptr(true),
							},
						},
					},
				},
			}
			resp.AddPage(http.StatusOK, p1, nil)

			// page 2
			p2 := azsecrets.ListSecretPropertiesResponse{
				SecretPropertiesListResult: azsecrets.SecretPropertiesListResult{
					Value: []*azsecrets.SecretProperties{
						{
							ID: to.Ptr(azsecrets.ID("https://kv-example.vault.azure.net/secrets/c")),
							Attributes: &azsecrets.SecretAttributes{
								Expires: to.Ptr(time.Unix(3_000, 0).UTC()),
								Enabled: to.Ptr(false),
							},
						},
					},
				},
			}
			resp.AddPage(http.StatusOK, p2, nil)

			return
		},
	}
}

// ---------- Fake Secret Servers ----------------------------------------------------
// SecretsServerTwoPages: 3 secrets across 2 pages (a,b,c) with expiries on a and c.
func SecretsServerTwoPages() secfake.Server {
	return secfake.Server{
		NewListSecretPropertiesPager: func(*azsecrets.ListSecretPropertiesOptions) (resp azfake.PagerResponder[azsecrets.ListSecretPropertiesResponse]) {
			// page 1
			resp.AddPage(http.StatusOK, azsecrets.ListSecretPropertiesResponse{
				SecretPropertiesListResult: azsecrets.SecretPropertiesListResult{
					Value: []*azsecrets.SecretProperties{
						{
							ID: to.Ptr(azsecrets.ID("https://kv-example.vault.azure.net/secrets/a")),
							Attributes: &azsecrets.SecretAttributes{
								Enabled: to.Ptr(true),
								Expires: to.Ptr(time.Unix(2_000, 0).UTC()),
							},
						},
						{
							ID: to.Ptr(azsecrets.ID("https://kv-example.vault.azure.net/secrets/b")),
							Attributes: &azsecrets.SecretAttributes{
								Enabled: to.Ptr(true), // no expiry
							},
						},
					},
				},
			}, nil)
			// page 2
			resp.AddPage(http.StatusOK, azsecrets.ListSecretPropertiesResponse{
				SecretPropertiesListResult: azsecrets.SecretPropertiesListResult{
					Value: []*azsecrets.SecretProperties{
						{
							ID: to.Ptr(azsecrets.ID("https://kv-example.vault.azure.net/secrets/c")),
							Attributes: &azsecrets.SecretAttributes{
								Enabled: to.Ptr(false),
								Expires: to.Ptr(time.Unix(3_000, 0).UTC()),
							},
						},
					},
				},
			}, nil)
			return
		},
	}
}

// SecretsServerEmpty: returns an empty page.
func SecretsServerEmpty() secfake.Server {
	return secfake.Server{
		NewListSecretPropertiesPager: func(*azsecrets.ListSecretPropertiesOptions) (resp azfake.PagerResponder[azsecrets.ListSecretPropertiesResponse]) {
			resp.AddPage(http.StatusOK, azsecrets.ListSecretPropertiesResponse{
				SecretPropertiesListResult: azsecrets.SecretPropertiesListResult{
					Value: []*azsecrets.SecretProperties{},
				},
			}, nil)
			return
		},
	}
}

// SecretsServerError: pager immediately errors with given status/message.
func SecretsServerError(status int, msg string) secfake.Server {
	return secfake.Server{
		NewListSecretPropertiesPager: func(*azsecrets.ListSecretPropertiesOptions) (resp azfake.PagerResponder[azsecrets.ListSecretPropertiesResponse]) {
			resp.AddResponseError(status, msg)
			return
		},
	}
}
