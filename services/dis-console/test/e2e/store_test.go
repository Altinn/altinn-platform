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
	if _, err := pool.Exec(ctx, "TRUNCATE flux_resource, flux_status_event, meta"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := s.InitMeta(ctx, "e2e"); err != nil {
		t.Fatalf("init meta: %v", err)
	}
	return s, pool
}

// res mirrors what flux.normalize produces: ContentHash is derived from the
// content, so an identical resource hashes the same (the store skips the
// rewrite) and a status change hashes differently (the store rewrites).
func res(name, ready, reason, revision string) flux.Resource {
	return flux.Resource{
		Kind:        "Kustomization",
		APIVersion:  "kustomize.toolkit.fluxcd.io/v1",
		Namespace:   "flux-system",
		Name:        name,
		Ready:       ready,
		Reason:      reason,
		Revision:    revision,
		Raw:         json.RawMessage(`{"kind":"Kustomization"}`),
		ContentHash: ready + "|" + reason + "|" + revision,
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

// TestSyncRoundTripsDISColumns checks the schema-v2 columns against real
// PostgreSQL: a swept DIS resource's azure_resource_id and parent pair survive
// Sync and come back out of List, Get and ChangedSince.
func TestSyncRoundTripsDISColumns(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()

	armID := "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/kv-app"
	vault := flux.Resource{
		Kind:            "Vault",
		APIVersion:      "vault.dis.altinn.cloud/v1alpha1",
		Namespace:       "team-a",
		Name:            "kv-app",
		Ready:           flux.ReadyTrue,
		Reason:          "Provisioned",
		AzureResourceID: armID,
		Raw:             json.RawMessage(`{"kind":"Vault"}`),
		ContentHash:     "vault-1",
	}
	database := flux.Resource{
		Kind:        "Database",
		APIVersion:  "storage.dis.altinn.cloud/v1alpha1",
		Namespace:   "team-a",
		Name:        "appdb",
		Ready:       flux.ReadyFalse,
		Reason:      "Provisioning",
		Parent:      &flux.ParentRef{Kind: "DatabaseServer", Name: "pg-main"},
		Raw:         json.RawMessage(`{"kind":"Database"}`),
		ContentHash: "db-1",
	}
	if _, err := s.Sync(ctx, []flux.Resource{vault, database}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	rows, err := s.List(ctx, "Vault", "", "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("list vaults: %+v err=%v", rows, err)
	}
	if rows[0].AzureResourceID != armID || rows[0].Parent != nil {
		t.Fatalf("unexpected vault projection: %+v", rows[0])
	}

	got, _, err := s.Get(ctx, "Database", "team-a", "appdb")
	if err != nil {
		t.Fatalf("get database: %v", err)
	}
	if got.Parent == nil || got.Parent.Kind != "DatabaseServer" || got.Parent.Name != "pg-main" {
		t.Fatalf("unexpected database parent: %+v", got.Parent)
	}
	if got.AzureResourceID != "" {
		t.Fatalf("expected empty azure id on the database, got %q", got.AzureResourceID)
	}

	changed, err := s.ChangedSince(ctx, time.Time{}, store.SchemaVersion)
	if err != nil || len(changed) != 2 {
		t.Fatalf("changed-since: %d rows err=%v", len(changed), err)
	}
	for _, c := range changed {
		switch c.Kind {
		case "Vault":
			if c.AzureResourceID != armID {
				t.Fatalf("changed vault lost azure id: %+v", c.Resource)
			}
		case "Database":
			if c.Parent == nil || c.Parent.Name != "pg-main" {
				t.Fatalf("changed database lost parent: %+v", c.Resource)
			}
		}
	}
}

// TestSyncContentHashSkipsUnchangedRewrite asserts the write-hygiene contract:
// an unchanged sweep leaves updated_at alone (no row/raw rewrite), while a
// content change advances it. The central sync loop pulls on updated_at, so
// this is what keeps idle resources from churning the fleet.
func TestSyncContentHashSkipsUnchangedRewrite(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()

	updatedAt := func() time.Time {
		var ts time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM flux_resource WHERE name = $1", "apps").Scan(&ts); err != nil {
			t.Fatalf("query updated_at: %v", err)
		}
		return ts
	}

	if _, err := s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyTrue, "OK", "sha-1")}); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	first := updatedAt()

	// Identical content => same content_hash => updated_at must not move.
	if _, err := s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyTrue, "OK", "sha-1")}); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	if got := updatedAt(); !got.Equal(first) {
		t.Fatalf("updated_at moved on an unchanged sweep: %v -> %v", first, got)
	}

	// A content change must advance updated_at.
	if _, err := s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyFalse, "Boom", "sha-2")}); err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if got := updatedAt(); !got.After(first) {
		t.Fatalf("updated_at did not advance on a content change: %v -> %v", first, got)
	}
}

// TestMetaRecordedOnSync checks the meta row: InitMeta stamps schema/agent
// version with no sweep time yet, and a sweep records last_sweep_at.
func TestMetaRecordedOnSync(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()

	m, err := s.GetMeta(ctx)
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	if m.SchemaVersion != store.SchemaVersion || m.AgentVersion != "e2e" {
		t.Fatalf("unexpected meta after init: %+v", m)
	}
	if !m.LastSweepAt.IsZero() {
		t.Fatalf("expected zero last_sweep_at before any sweep, got %v", m.LastSweepAt)
	}

	if _, err := s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyTrue, "OK", "sha-1")}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	m, err = s.GetMeta(ctx)
	if err != nil {
		t.Fatalf("get meta after sync: %v", err)
	}
	if m.LastSweepAt.IsZero() {
		t.Fatalf("expected last_sweep_at set after sweep")
	}
}
