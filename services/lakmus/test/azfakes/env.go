package azfakes

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	kvfake "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault/fake"
	azsecrets "github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	secfake "github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets/fake"
)

// Env bundles a fake credential and client options (ARM + Secrets) that route
// SDK calls to in-process fake servers (no network).
type Env struct {
	Cred    azcore.TokenCredential
	ARM     *arm.ClientOptions
	Secrets *azsecrets.ClientOptions
}

// NewEnv builds an Env. Pass nil for either server to omit that client option.
func NewEnv(kvSrv *kvfake.VaultsServer, secSrv *secfake.Server) *Env {
	e := &Env{
		Cred: &azfake.TokenCredential{},
	}
	if kvSrv != nil {
		e.ARM = &arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: kvfake.NewVaultsServerTransport(kvSrv),
			},
		}
	}
	if secSrv != nil {
		e.Secrets = &azsecrets.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: secfake.NewServerTransport(secSrv),
			},
		}
	}
	return e
}
