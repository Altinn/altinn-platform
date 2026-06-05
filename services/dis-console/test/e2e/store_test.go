//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/dbauth"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newStore connects to the Postgres named by DISCONSOLE_TEST_DB_URI (set by the
// `make test-e2e-kind-ci` target, which port-forwards the Kind Postgres), waits
// for it to accept connections, migrates the schema and truncates the tables.
// It uses dbauth with a nil credential, exercising the no-Entra (trust/PGPASSWORD)
// auth path the service uses in Kind/CI.
func newStore(t *testing.T) (*store.Store, *pgxpool.Pool) {
	t.Helper()
	uri := os.Getenv("DISCONSOLE_TEST_DB_URI")
	if uri == "" {
		t.Skip("DISCONSOLE_TEST_DB_URI not set; run via `make test-e2e-kind-ci`")
	}
	ctx := context.Background()

	pool, err := dbauth.NewPool(ctx, uri, nil) // nil cred => trust/PGPASSWORD auth (no Entra in Kind)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	s := store.New(pool)

	// Wait for the port-forwarded database to accept connections.
	var pingErr error
	for i := 0; i < 30; i++ {
		pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		pingErr = s.Ping(pctx)
		cancel()
		if pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		t.Fatalf("database never became reachable: %v", pingErr)
	}

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE flux_resource, flux_status_event"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return s, pool
}

func res(name, ready, reason, revision string) flux.Resource {
	return flux.Resource{
		Kind:       "Kustomization",
		APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Namespace:  "flux-system",
		Name:       name,
		Ready:      ready,
		Reason:     reason,
		Revision:   revision,
		Raw:        json.RawMessage(`{"kind":"Kustomization"}`),
	}
}

// TestSyncUpsertHistoryAndPrune drives the full store contract against real
// PostgreSQL: initial upsert writes history, an unchanged sweep is a no-op, a
// status change writes a new history row, a disappeared object is pruned, and
// the summary/list/get queries reflect the final state.
func TestSyncUpsertHistoryAndPrune(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()

	// First sweep: two resources, both new => two history events.
	stats, err := s.Sync(ctx, []flux.Resource{
		res("apps", flux.ReadyTrue, "ReconciliationSucceeded", "sha-1"),
		res("infra", flux.ReadyFalse, "BuildFailed", "sha-1"),
	})
	if err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	if stats.Changed != 2 || stats.Pruned != 0 || stats.Upserted != 2 {
		t.Fatalf("sync 1 stats: %+v", stats)
	}

	// Second sweep: identical => no change, no history, no prune.
	stats, err = s.Sync(ctx, []flux.Resource{
		res("apps", flux.ReadyTrue, "ReconciliationSucceeded", "sha-1"),
		res("infra", flux.ReadyFalse, "BuildFailed", "sha-1"),
	})
	if err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	if stats.Changed != 0 || stats.Pruned != 0 {
		t.Fatalf("sync 2 stats: %+v", stats)
	}

	// Third sweep: "infra" recovers (status change) and "apps" disappears.
	stats, err = s.Sync(ctx, []flux.Resource{
		res("infra", flux.ReadyTrue, "ReconciliationSucceeded", "sha-2"),
	})
	if err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if stats.Changed != 1 {
		t.Fatalf("expected 1 changed, got %+v", stats)
	}
	if stats.Pruned != 1 {
		t.Fatalf("expected 1 pruned (apps removed), got %+v", stats)
	}

	// "apps" is gone.
	if _, _, err := s.Get(ctx, "Kustomization", "flux-system", "apps"); err != store.ErrNotFound {
		t.Fatalf("expected apps pruned, got err=%v", err)
	}

	// Its history rows were pruned too (no orphaned, unreachable events).
	var appsEvents int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM flux_status_event WHERE namespace = $1 AND name = $2",
		"flux-system", "apps",
	).Scan(&appsEvents); err != nil {
		t.Fatalf("count apps events: %v", err)
	}
	if appsEvents != 0 {
		t.Fatalf("expected apps history pruned, got %d orphaned events", appsEvents)
	}

	// "infra" has two history events (initial False, then recovered True), newest first.
	row, history, err := s.Get(ctx, "kustomization", "flux-system", "infra") // case-insensitive kind
	if err != nil {
		t.Fatalf("get infra: %v", err)
	}
	if row.Ready != flux.ReadyTrue || row.Revision != "sha-2" {
		t.Fatalf("unexpected infra row: %+v", row)
	}
	if len(history) != 2 || history[0].Ready != flux.ReadyTrue || history[1].Ready != flux.ReadyFalse {
		t.Fatalf("unexpected infra history: %+v", history)
	}

	// Summary: one Kustomization, ready.
	counts, err := s.Summary(ctx)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(counts) != 1 || counts[0].Total != 1 || counts[0].Ready != 1 {
		t.Fatalf("unexpected summary: %+v", counts)
	}

	// List ready=False is now empty.
	notReady, err := s.List(ctx, "", "", flux.ReadyFalse)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(notReady) != 0 {
		t.Fatalf("expected no not-ready rows, got %d", len(notReady))
	}
}
