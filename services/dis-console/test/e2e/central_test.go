//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/central"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/dbauth"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newCentral creates a fresh central database (and one tenant database for the
// discovery test) on the test server, then migrates the central schema. It
// exercises dbauth.NewPoolForDatabase — the same database-name override the
// server uses to reach sibling databases on the shared server.
func newCentral(t *testing.T) (*central.Store, *pgxpool.Pool) {
	t.Helper()
	uri := os.Getenv("DISCONSOLE_TEST_DB_URI")
	if uri == "" {
		t.Skip("DISCONSOLE_TEST_DB_URI not set; run via `make test-e2e-kind-ci`")
	}
	ctx := context.Background()

	admin, err := dbauth.NewPool(ctx, uri, nil)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	defer admin.Close()

	var pingErr error
	for i := 0; i < 30; i++ {
		pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		pingErr = admin.Ping(pctx)
		cancel()
		if pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		t.Fatalf("database never became reachable: %v", pingErr)
	}

	// Recreate the central database and a tenant database from scratch.
	for _, db := range []string{"dis_console_central_e2e", "dis_console_ttd_at23"} {
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+db+" WITH (FORCE)"); err != nil {
			t.Fatalf("drop %s: %v", db, err)
		}
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+db); err != nil {
			t.Fatalf("create %s: %v", db, err)
		}
	}

	pool, err := dbauth.NewPoolForDatabase(ctx, uri, "dis_console_central_e2e", nil)
	if err != nil {
		t.Fatalf("central pool: %v", err)
	}
	t.Cleanup(pool.Close)

	cs := central.New(pool)
	if err := cs.Migrate(ctx); err != nil {
		t.Fatalf("migrate central: %v", err)
	}
	return cs, pool
}

func changed(name, kind, ready, hash string, at time.Time) store.ChangedResource {
	return store.ChangedResource{
		Resource: flux.Resource{
			Kind:        kind,
			APIVersion:  "kustomize.toolkit.fluxcd.io/v1",
			Namespace:   "flux-system",
			Name:        name,
			Ready:       ready,
			ContentHash: hash,
			Raw:         json.RawMessage(`{"kind":"` + kind + `"}`),
		},
		UpdatedAt: at,
	}
}

// TestCentralApplyUpsertPruneCursor drives the central store contract against
// real PostgreSQL: a first apply mirrors rows and records the cluster_report; a
// second apply upserts a changed row, prunes a disappeared one, and advances
// the cursor — all scoped to the one cluster.
func TestCentralApplyUpsertPruneCursor(t *testing.T) {
	cs, pool := newCentral(t)
	ctx := context.Background()

	t1 := time.Now().UTC().Truncate(time.Microsecond)
	if err := cs.Apply(ctx, central.ClusterState{
		Cluster:     "ttd_at23",
		Environment: "at23",
		Changed: []store.ChangedResource{
			changed("apps", "Kustomization", flux.ReadyTrue, "h-apps-1", t1),
			changed("chart", "HelmRelease", flux.ReadyFalse, "h-chart-1", t1),
		},
		Keys: []store.ResourceKey{
			{Kind: "Kustomization", Namespace: "flux-system", Name: "apps"},
			{Kind: "HelmRelease", Namespace: "flux-system", Name: "chart"},
		},
		Cursor:        t1,
		SchemaVersion: store.SchemaVersion,
		AgentVersion:  "test",
	}); err != nil {
		t.Fatalf("apply 1: %v", err)
	}

	if n := countRows(t, pool, "ttd_at23"); n != 2 {
		t.Fatalf("after apply 1: expected 2 rows, got %d", n)
	}
	if cur, err := cs.Cursor(ctx, "ttd_at23"); err != nil || !cur.Equal(t1) {
		t.Fatalf("cursor after apply 1: got %v err %v, want %v", cur, err, t1)
	}
	var env string
	var count int
	if err := pool.QueryRow(ctx,
		"SELECT environment, resource_count FROM cluster_report WHERE cluster=$1", "ttd_at23",
	).Scan(&env, &count); err != nil {
		t.Fatalf("cluster_report: %v", err)
	}
	if env != "at23" || count != 2 {
		t.Fatalf("cluster_report: env=%q count=%d", env, count)
	}

	// Second sweep: "apps" recovers (changed), "chart" disappears (pruned).
	t2 := t1.Add(time.Minute)
	if err := cs.Apply(ctx, central.ClusterState{
		Cluster:       "ttd_at23",
		Environment:   "at23",
		Changed:       []store.ChangedResource{changed("apps", "Kustomization", flux.ReadyFalse, "h-apps-2", t2)},
		Keys:          []store.ResourceKey{{Kind: "Kustomization", Namespace: "flux-system", Name: "apps"}},
		Cursor:        t2,
		SchemaVersion: store.SchemaVersion,
		AgentVersion:  "test",
	}); err != nil {
		t.Fatalf("apply 2: %v", err)
	}

	if n := countRows(t, pool, "ttd_at23"); n != 1 {
		t.Fatalf("after apply 2: expected 1 row (chart pruned), got %d", n)
	}
	var ready string
	if err := pool.QueryRow(ctx,
		"SELECT ready FROM flux_resource WHERE cluster=$1 AND name='apps'", "ttd_at23",
	).Scan(&ready); err != nil {
		t.Fatalf("read apps: %v", err)
	}
	if ready != flux.ReadyFalse {
		t.Fatalf("apps ready: got %q, want %q", ready, flux.ReadyFalse)
	}
	if cur, err := cs.Cursor(ctx, "ttd_at23"); err != nil || !cur.Equal(t2) {
		t.Fatalf("cursor after apply 2: got %v err %v, want %v", cur, err, t2)
	}
}

// TestCentralDiscover checks tenant discovery returns the dis_console_* tenant
// databases and excludes the console's own (central) database.
func TestCentralDiscover(t *testing.T) {
	cs, _ := newCentral(t)
	dbs, err := cs.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	var sawTenant, sawCentral bool
	for _, db := range dbs {
		switch db {
		case "dis_console_ttd_at23":
			sawTenant = true
		case "dis_console_central_e2e":
			sawCentral = true
		}
	}
	if !sawTenant {
		t.Fatalf("discover should include the tenant db, got %v", dbs)
	}
	if sawCentral {
		t.Fatalf("discover must exclude the central (current) db, got %v", dbs)
	}
}

func countRows(t *testing.T, pool *pgxpool.Pool, cluster string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM flux_resource WHERE cluster=$1", cluster).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

// TestCentralReads exercises the fleet-API read methods (Clusters/Summary/List/
// Get + staleness) against real PostgreSQL after one apply.
func TestCentralReads(t *testing.T) {
	cs, _ := newCentral(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := cs.Apply(ctx, central.ClusterState{
		Cluster:     "ttd_at23",
		Environment: "at23",
		Changed: []store.ChangedResource{
			changed("apps", "Kustomization", flux.ReadyTrue, "h1", now),
			changed("broken", "Kustomization", flux.ReadyFalse, "h2", now),
		},
		Keys: []store.ResourceKey{
			{Kind: "Kustomization", Namespace: "flux-system", Name: "apps"},
			{Kind: "Kustomization", Namespace: "flux-system", Name: "broken"},
		},
		Cursor:        now,
		SchemaVersion: store.SchemaVersion,
		AgentVersion:  "test",
		LastSweepAt:   now,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Clusters: present and fresh with a generous threshold.
	clusters, err := cs.Clusters(ctx, time.Hour)
	if err != nil {
		t.Fatalf("clusters: %v", err)
	}
	if len(clusters) != 1 || clusters[0].Cluster != "ttd_at23" || clusters[0].Environment != "at23" ||
		clusters[0].ResourceCount != 2 || clusters[0].Stale {
		t.Fatalf("unexpected clusters: %+v", clusters)
	}
	// Stale with a sub-tick threshold.
	if stale, err := cs.Clusters(ctx, time.Nanosecond); err != nil || len(stale) != 1 || !stale[0].Stale {
		t.Fatalf("expected stale with tiny threshold: %+v err=%v", stale, err)
	}

	// Summary across the fleet.
	sum, err := cs.Summary(ctx, "")
	if err != nil || len(sum) != 1 || sum[0].Kind != "Kustomization" ||
		sum[0].Total != 2 || sum[0].Ready != 1 || sum[0].NotReady != 1 {
		t.Fatalf("unexpected summary: %+v err=%v", sum, err)
	}

	// List with a ready filter, scoped to the cluster.
	rows, err := cs.List(ctx, "ttd_at23", "", "", flux.ReadyFalse)
	if err != nil || len(rows) != 1 || rows[0].Name != "broken" || rows[0].Cluster != "ttd_at23" {
		t.Fatalf("unexpected list: %+v err=%v", rows, err)
	}

	// Get a resource (case-insensitive kind) including its raw payload.
	r, err := cs.Get(ctx, "ttd_at23", "kustomization", "flux-system", "apps")
	if err != nil || r.Name != "apps" || r.Cluster != "ttd_at23" || len(r.Raw) == 0 {
		t.Fatalf("unexpected get: %+v err=%v", r, err)
	}

	// Get a missing resource.
	if _, err := cs.Get(ctx, "ttd_at23", "Kustomization", "flux-system", "missing"); !errors.Is(err, central.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
