package config

// DisIdentityConfig contains the configuration for the dis-identity operator.
type DisIdentityConfig struct {
	// IssuerURL the issuer URL for the cluster running the instance of the operator.
	IssuerURL string `json:"issuerURL"`
	// TargetResourceGroup the armID of the resource group where the managed identity will be created.
	TargetResourceGroup string `json:"targetResourceGroup"`
}
