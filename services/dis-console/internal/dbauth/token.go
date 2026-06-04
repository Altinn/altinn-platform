// Package dbauth builds a pgxpool connection pool for a DIS-provisioned Azure
// PostgreSQL Flexible Server. In the cluster it authenticates with a
// workload-identity Entra token; for Kind/CI/local runs (no workload identity)
// it falls back to PGPASSWORD or trust auth.
package dbauth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tokenScope is the OAuth scope for Azure Database for PostgreSQL Entra auth.
// The access token for this scope is presented as the connection password.
const tokenScope = "https://ossrdbms-aad.database.windows.net/.default"

// maxConnLifetime recycles pooled connections well within the ~1h Entra token
// lifetime so new connections re-run BeforeConnect and pick up a fresh token,
// keeping the pool healthy across token rotation.
const maxConnLifetime = 30 * time.Minute

// NewPool builds a pgxpool from dbURI, which carries no password — the user,
// host, database and sslmode all come from the URI (in production, the DIS
// operator's connection ConfigMap).
//
// When cred is non-nil, a BeforeConnect hook authenticates every new connection
// with a fresh Entra access token (the DIS workload-identity path): the token
// is presented as the Postgres password, so ~hourly expiry is handled
// transparently. When cred is nil, Entra is disabled (Kind/CI/local, where no
// workload identity exists): the connection uses PGPASSWORD if set, otherwise
// no password (e.g. a trust-auth Postgres).
func NewPool(ctx context.Context, dbURI string, cred azcore.TokenCredential) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURI)
	if err != nil {
		return nil, fmt.Errorf("parse db uri: %w", err)
	}
	cfg.MaxConnLifetime = maxConnLifetime

	if cred != nil {
		cfg.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
			tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{tokenScope}})
			if err != nil {
				return fmt.Errorf("get entra token: %w", err)
			}
			cc.Password = tok.Token
			return nil
		}
	} else if pw := os.Getenv("PGPASSWORD"); pw != "" {
		cfg.ConnConfig.Password = pw
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}
	return pool, nil
}
