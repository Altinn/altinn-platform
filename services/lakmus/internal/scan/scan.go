package scan

import (
	"context"
	"log"

	secrets "github.com/Altinn/altinn-platform/services/lakmus/pkg/secrets"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	azsecrets "github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// MetricSetter lets callers record an expiry timestamp for a secret in a KV.
// Makes it independent of Prometheus, so it can use otel later if needed.
type MetricSetter func(kvName, secretName string, expiryUnix float64)

// Scan enumerates Key Vaults in a subscription and, for each vault, lists secret
// metadata and invokes setMetric for secrets that have an expiry timestamp.
//
// - cred: Workload Identity
// - armOpts / secretsOpts: usually nil in prod; used for fakes in tests
func Scan(
	ctx context.Context,
	subscriptionID string,
	cred azcore.TokenCredential,
	armOpts *arm.ClientOptions,
	secretsOpts *azsecrets.ClientOptions,
	setMetric MetricSetter,
) error {

	vaults, err := secrets.ListKeyVaults(ctx, subscriptionID, cred, armOpts)
	if err != nil {
		return err
	}

	for _, v := range vaults {
		// avoid calling the apis in case of ctx cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		props, err := secrets.ListSecrets(ctx, v.VaultURI, cred, secretsOpts)
		if err != nil {
			log.Printf("scan: list secrets failed for vault %q: %v", v.Name, err)
			continue
		}

		for _, sp := range props {
			// skip secrets without expiry
			// also skips nil sp or sp.ID
			if name, ts, ok := expiryFrom(sp); ok {
				setMetric(v.Name, name, ts)
			}
		}
	}

	return nil
}

// expiryFrom extracts the secret name and expiry timestamp from SecretProperties.
// If either is missing, ok=false is returned.
func expiryFrom(sp *azsecrets.SecretProperties) (name string, ts float64, ok bool) {
	if sp == nil || sp.ID == nil {
		return "", 0, false
	}
	a := sp.Attributes
	if a == nil || a.Expires == nil {
		return "", 0, false
	}
	return sp.ID.Name(), float64(a.Expires.Unix()), true
}
