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

// TestCentralAppliedByRoundTrip checks the projected owning Kustomization is
// carried end-to-end into the central read model: List (no raw) and Get both
// surface appliedBy, while an unowned root stays nil. The list carrying it is
// what lets the UI group child HelmReleases under their app without detail
// fetches.
func TestCentralAppliedByRoundTrip(t *testing.T) {
	cs, _ := newCentral(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	child := changed("grafana-operator", "HelmRelease", flux.ReadyTrue, "h1", now)
	child.AppliedBy = &flux.AppliedBy{Name: "grafana-operator-grafana-operator", Namespace: "flux-system"}
	root := changed("apps", "Kustomization", flux.ReadyTrue, "h2", now)
	if err := cs.Apply(ctx, central.ClusterState{
		Cluster:     "ttd_at23",
		Environment: "at23",
		Changed:     []store.ChangedResource{child, root},
		Keys: []store.ResourceKey{
			{Kind: "HelmRelease", Namespace: "flux-system", Name: "grafana-operator"},
			{Kind: "Kustomization", Namespace: "flux-system", Name: "apps"},
		},
		Cursor:        now,
		SchemaVersion: store.SchemaVersion,
		AgentVersion:  "test",
		LastSweepAt:   now,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	rows, err := cs.List(ctx, "ttd_at23", "", "", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var seenChild, seenRoot bool
	for _, r := range rows {
		switch r.Name {
		case "grafana-operator":
			seenChild = true
			if r.AppliedBy == nil || r.AppliedBy.Name != "grafana-operator-grafana-operator" || r.AppliedBy.Namespace != "flux-system" {
				t.Fatalf("child HelmRelease List should carry AppliedBy: %+v", r.AppliedBy)
			}
		case "apps":
			seenRoot = true
			if r.AppliedBy != nil {
				t.Fatalf("root Kustomization List should have nil AppliedBy: %+v", r.AppliedBy)
			}
		}
	}
	if !seenChild || !seenRoot {
		t.Fatalf("list missing rows: child=%v root=%v rows=%+v", seenChild, seenRoot, rows)
	}

	got, err := cs.Get(ctx, "ttd_at23", "HelmRelease", "flux-system", "grafana-operator")
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if got.AppliedBy == nil || got.AppliedBy.Name != "grafana-operator-grafana-operator" {
		t.Fatalf("child Get should carry AppliedBy: %+v", got.AppliedBy)
	}
}

// TestCentralDISFieldsEndToEnd walks a DIS resource through the full pipeline
// against real PostgreSQL: agent Sync into the tenant database, server pull
// (ChangedSince at the current schema) and Apply, then the fleet-API reads —
// asserting the azure id and the parent pair survive every hop.
func TestCentralDISFieldsEndToEnd(t *testing.T) {
	cs, _ := newCentral(t)
	ctx := context.Background()
	uri := os.Getenv("DISCONSOLE_TEST_DB_URI")

	tpool, err := dbauth.NewPoolForDatabase(ctx, uri, "dis_console_ttd_at23", nil)
	if err != nil {
		t.Fatalf("tenant pool: %v", err)
	}
	defer tpool.Close()
	ts := store.New(tpool)
	if err := ts.Migrate(ctx); err != nil {
		t.Fatalf("migrate tenant: %v", err)
	}
	if err := ts.InitMeta(ctx, "e2e"); err != nil {
		t.Fatalf("init tenant meta: %v", err)
	}

	armID := "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/kv-app"
	if _, err := ts.Sync(ctx, []flux.Resource{
		{
			Kind: "Vault", APIVersion: "vault.dis.altinn.cloud/v1alpha1",
			Namespace: "team-a", Name: "kv-app",
			Ready: flux.ReadyTrue, AzureResourceID: armID,
			Raw: json.RawMessage(`{"kind":"Vault"}`), ContentHash: "vault-1",
		},
		{
			Kind: "Database", APIVersion: "storage.dis.altinn.cloud/v1alpha1",
			Namespace: "team-a", Name: "appdb",
			Ready: flux.ReadyFalse, Parent: &flux.ParentRef{Kind: "DatabaseServer", Name: "pg-main"},
			Raw: json.RawMessage(`{"kind":"Database"}`), ContentHash: "db-1",
		},
	}); err != nil {
		t.Fatalf("tenant sync: %v", err)
	}

	pullAndApply(t, cs, ts, "ttd_at23")

	vault, err := cs.Get(ctx, "ttd_at23", "Vault", "team-a", "kv-app")
	if err != nil {
		t.Fatalf("central get vault: %v", err)
	}
	if vault.AzureResourceID != armID || vault.Parent != nil {
		t.Fatalf("vault lost its projection in central: %+v", vault)
	}

	dbs, err := cs.List(ctx, "ttd_at23", "Database", "", "")
	if err != nil || len(dbs) != 1 {
		t.Fatalf("central list databases: %+v err=%v", dbs, err)
	}
	if dbs[0].Parent == nil || dbs[0].Parent.Kind != "DatabaseServer" || dbs[0].Parent.Name != "pg-main" {
		t.Fatalf("database lost its parent in central: %+v", dbs[0])
	}
	if dbs[0].AzureResourceID != "" {
		t.Fatalf("expected empty azure id on the database, got %q", dbs[0].AzureResourceID)
	}
}

// TestCentralSyncSchemaV4Tenant proves the rollout order the schema gate
// promises: a server at schema 5 can still pull a tenant whose agent is at
// schema 4 (its database lacks the images column entirely). The tenant is
// built by hand — migrate, then drop the v5 column and stamp schema_version 4
// — because a v5 agent binary can no longer write the v4 shape.
func TestCentralSyncSchemaV4Tenant(t *testing.T) {
	cs, _ := newCentral(t)
	ctx := context.Background()
	uri := os.Getenv("DISCONSOLE_TEST_DB_URI")

	tpool, err := dbauth.NewPoolForDatabase(ctx, uri, "dis_console_ttd_at23", nil)
	if err != nil {
		t.Fatalf("tenant pool: %v", err)
	}
	defer tpool.Close()
	ts := store.New(tpool)
	if err := ts.Migrate(ctx); err != nil {
		t.Fatalf("migrate tenant: %v", err)
	}
	if _, err := tpool.Exec(ctx, "ALTER TABLE flux_resource DROP COLUMN images"); err != nil {
		t.Fatalf("shape tenant as v4: %v", err)
	}
	if _, err := tpool.Exec(ctx,
		"INSERT INTO meta (id, schema_version, agent_version) VALUES (true, 4, 'v4-agent')"); err != nil {
		t.Fatalf("stamp v4 meta: %v", err)
	}
	// The full column set a v4 agent writes: its upsert always passes the Go
	// zero values, so reason/message/revision land as '' (never NULL) and the
	// DIS/applied-by/base-layer columns as NULL for a plain root Flux object.
	if _, err := tpool.Exec(ctx, `INSERT INTO flux_resource
		(kind, api_version, namespace, name, ready, reason, message, revision,
		 azure_resource_id, suspended, generation, observed_generation, raw, content_hash)
		VALUES ('Kustomization', 'kustomize.toolkit.fluxcd.io/v1', 'flux-system', 'apps', 'True', '', '', '',
		 '', false, 0, 0, '{}', 'h1')`); err != nil {
		t.Fatalf("insert v4 row: %v", err)
	}

	meta, err := ts.GetMeta(ctx)
	if err != nil || meta.SchemaVersion != 4 {
		t.Fatalf("tenant meta: %+v err=%v", meta, err)
	}

	// The version-keyed pull must succeed against the columnless table.
	changed, err := ts.ChangedSince(ctx, time.Time{}, meta.SchemaVersion)
	if err != nil {
		t.Fatalf("changed-since on a v4 tenant: %v", err)
	}
	if len(changed) != 1 || changed[0].Images != nil {
		t.Fatalf("unexpected v4 pull: %+v", changed)
	}
	keys, err := ts.Keys(ctx)
	if err != nil {
		t.Fatalf("keys: %v", err)
	}

	if err := cs.Apply(ctx, central.ClusterState{
		Cluster:       "ttd_at23",
		Environment:   "at23",
		Changed:       changed,
		Keys:          keys,
		Cursor:        changed[0].UpdatedAt,
		SchemaVersion: meta.SchemaVersion,
		AgentVersion:  meta.AgentVersion,
	}); err != nil {
		t.Fatalf("apply v4 tenant: %v", err)
	}

	rows, err := cs.List(ctx, "ttd_at23", "", "", "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("central list after v4 sync: %+v err=%v", rows, err)
	}
	if rows[0].Images != nil {
		t.Fatalf("v4 row should have nil images, got %+v", rows[0].Images)
	}
}

// TestCentralBaseLayerRoundTrip checks the base-layer projections are carried
// end-to-end into the central read model: List (no inventory) surfaces
// sourceRef/url/origin for the artifacts view, and Get returns the full
// inventory for the tree expansion.
func TestCentralBaseLayerRoundTrip(t *testing.T) {
	cs, _ := newCentral(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	repo := changed("team-a-ab12", "OCIRepository", flux.ReadyTrue, "h1", now)
	repo.SourceURL = "oci://registry.example.com/team-a/syncroot"
	repo.OriginRevision = "main/0c2a3b4"
	repo.OriginSource = "https://git.example.com/team-a/syncroot"
	kust := changed("team-a-ab12-team-a", "Kustomization", flux.ReadyTrue, "h2", now)
	kust.SourceRef = &flux.SourceRef{Kind: "OCIRepository", Name: "team-a-ab12", Namespace: "flux-system"}
	kust.Inventory = []flux.InventoryEntry{
		{ID: "product-team-a_appdb_storage.dis.altinn.cloud_Database", Version: "v1alpha1"},
	}
	if err := cs.Apply(ctx, central.ClusterState{
		Cluster:     "ttd_at23",
		Environment: "at23",
		Changed:     []store.ChangedResource{repo, kust},
		Keys: []store.ResourceKey{
			{Kind: "OCIRepository", Namespace: "flux-system", Name: "team-a-ab12"},
			{Kind: "Kustomization", Namespace: "flux-system", Name: "team-a-ab12-team-a"},
		},
		Cursor:        now,
		SchemaVersion: store.SchemaVersion,
		AgentVersion:  "e2e",
		LastSweepAt:   now,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	rows, err := cs.List(ctx, "ttd_at23", "", "", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, r := range rows {
		switch r.Kind {
		case "OCIRepository":
			if r.SourceURL != "oci://registry.example.com/team-a/syncroot" ||
				r.OriginRevision != "main/0c2a3b4" || r.OriginSource != "https://git.example.com/team-a/syncroot" {
				t.Fatalf("repository List lost base-layer fields: %+v", r)
			}
		case "Kustomization":
			if r.SourceRef == nil || r.SourceRef.Name != "team-a-ab12" {
				t.Fatalf("kustomization List lost sourceRef: %+v", r)
			}
			if r.Inventory != nil {
				t.Fatalf("List must omit the inventory payload: %+v", r.Inventory)
			}
		}
	}

	got, err := cs.Get(ctx, "ttd_at23", "Kustomization", "flux-system", "team-a-ab12-team-a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Inventory) != 1 || got.Inventory[0].ID != "product-team-a_appdb_storage.dis.altinn.cloud_Database" {
		t.Fatalf("kustomization Get inventory: %+v", got.Inventory)
	}
}

// TestCentralImagesEndToEnd walks a GitOps-applied Deployment through the full
// pipeline against real PostgreSQL: agent Sync into the tenant database,
// server pull (ChangedSince at the current schema) and Apply, then the
// fleet-API reads — asserting the container images survive every hop and ride
// the list payload (the UI's "which image runs where" needs no detail fetch).
func TestCentralImagesEndToEnd(t *testing.T) {
	cs, _ := newCentral(t)
	ctx := context.Background()
	uri := os.Getenv("DISCONSOLE_TEST_DB_URI")

	tpool, err := dbauth.NewPoolForDatabase(ctx, uri, "dis_console_ttd_at23", nil)
	if err != nil {
		t.Fatalf("tenant pool: %v", err)
	}
	defer tpool.Close()
	ts := store.New(tpool)
	if err := ts.Migrate(ctx); err != nil {
		t.Fatalf("migrate tenant: %v", err)
	}
	if err := ts.InitMeta(ctx, "e2e"); err != nil {
		t.Fatalf("init tenant meta: %v", err)
	}

	if _, err := ts.Sync(ctx, []flux.Resource{{
		Kind: "Deployment", APIVersion: "apps/v1",
		Namespace: "product-team-a", Name: "app",
		Ready: flux.ReadyTrue, Reason: "MinimumReplicasAvailable",
		AppliedBy: &flux.AppliedBy{Name: "team-a-ab12-team-a", Namespace: "product-team-a"},
		Images: []flux.ContainerImage{
			{Container: "app", Image: "registry.example.com/team-a/app:v42"},
		},
		Raw: json.RawMessage(`{"kind":"Deployment"}`), ContentHash: "deploy-1",
	}}); err != nil {
		t.Fatalf("tenant sync: %v", err)
	}

	pullAndApply(t, cs, ts, "ttd_at23")

	rows, err := cs.List(ctx, "ttd_at23", "Deployment", "", "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("central list deployments: %+v err=%v", rows, err)
	}
	if len(rows[0].Images) != 1 ||
		rows[0].Images[0] != (flux.ContainerImage{Container: "app", Image: "registry.example.com/team-a/app:v42"}) {
		t.Fatalf("deployment lost its images in the central list: %+v", rows[0].Images)
	}
	if rows[0].AppliedBy == nil || rows[0].AppliedBy.Name != "team-a-ab12-team-a" {
		t.Fatalf("deployment lost appliedBy in central: %+v", rows[0].AppliedBy)
	}

	got, err := cs.Get(ctx, "ttd_at23", "Deployment", "product-team-a", "app")
	if err != nil {
		t.Fatalf("central get deployment: %v", err)
	}
	if len(got.Images) != 1 || got.Images[0].Container != "app" {
		t.Fatalf("deployment lost its images in central Get: %+v", got.Images)
	}
}

// TestCentralEventCopyAndHistory drives the status-history pipeline against real
// PostgreSQL: an agent writes status events into its tenant database, the server
// copies them into the cluster-keyed central event log (incremental, id-cursored),
// History serves them newest-first, a redundant sync doesn't duplicate, and a
// pruned resource loses its events.
func TestCentralEventCopyAndHistory(t *testing.T) {
	cs, cpool := newCentral(t)
	ctx := context.Background()
	uri := os.Getenv("DISCONSOLE_TEST_DB_URI")

	// Agent side: a tenant store on the tenant database newCentral created.
	tpool, err := dbauth.NewPoolForDatabase(ctx, uri, "dis_console_ttd_at23", nil)
	if err != nil {
		t.Fatalf("tenant pool: %v", err)
	}
	defer tpool.Close()
	ts := store.New(tpool)
	if err := ts.Migrate(ctx); err != nil {
		t.Fatalf("migrate tenant: %v", err)
	}
	if err := ts.InitMeta(ctx, "e2e"); err != nil {
		t.Fatalf("init tenant meta: %v", err)
	}

	// Sweep 1: two resources => two status events in the tenant.
	if _, err := ts.Sync(ctx, []flux.Resource{
		res("apps", flux.ReadyTrue, "ReconciliationSucceeded", "sha-1"),
		res("infra", flux.ReadyFalse, "BuildFailed", "sha-1"),
	}); err != nil {
		t.Fatalf("tenant sync 1: %v", err)
	}

	// Server pulls resources + events from the cursors and applies to central.
	pullAndApply(t, cs, ts, "ttd_at23")

	if ec, err := cs.EventCursor(ctx, "ttd_at23"); err != nil || ec == 0 {
		t.Fatalf("event cursor should have advanced past 0, got %d err %v", ec, err)
	}
	if h, err := cs.History(ctx, "ttd_at23", "Kustomization", "flux-system", "infra"); err != nil ||
		len(h) != 1 || h[0].Ready != flux.ReadyFalse {
		t.Fatalf("infra history after sweep 1: %+v err=%v", h, err)
	}

	// Sweep 2: infra recovers (new event); apps disappears (pruned in the tenant).
	if _, err := ts.Sync(ctx, []flux.Resource{
		res("infra", flux.ReadyTrue, "ReconciliationSucceeded", "sha-2"),
	}); err != nil {
		t.Fatalf("tenant sync 2: %v", err)
	}
	pullAndApply(t, cs, ts, "ttd_at23")

	// infra now has two events, newest first (case-insensitive kind lookup).
	h, err := cs.History(ctx, "ttd_at23", "kustomization", "flux-system", "infra")
	if err != nil || len(h) != 2 || h[0].Ready != flux.ReadyTrue || h[1].Ready != flux.ReadyFalse {
		t.Fatalf("infra history after sweep 2: %+v err=%v", h, err)
	}

	// apps was pruned, so its central events were pruned too (no orphans).
	var appsEvents int
	if err := cpool.QueryRow(ctx,
		"SELECT count(*) FROM flux_status_event WHERE cluster=$1 AND name=$2", "ttd_at23", "apps",
	).Scan(&appsEvents); err != nil {
		t.Fatalf("count apps events: %v", err)
	}
	if appsEvents != 0 {
		t.Fatalf("expected apps history pruned in central, got %d orphaned events", appsEvents)
	}

	// A redundant sync (nothing new) must not duplicate events.
	pullAndApply(t, cs, ts, "ttd_at23")
	if h, err := cs.History(ctx, "ttd_at23", "Kustomization", "flux-system", "infra"); err != nil || len(h) != 2 {
		t.Fatalf("redundant sync changed history: %+v err=%v", h, err)
	}
}

// TestCentralEventRetention proves the safety contract behind the central
// purge: the per-cluster event copy cursor lives in cluster_report
// (event_cursor), not in the event table itself, so purging aged central
// events must not regress the cursor, must not resurrect the purged rows on
// the next sync, and must not stop newer tenant events from flowing.
func TestCentralEventRetention(t *testing.T) {
	cs, cpool := newCentral(t)
	ctx := context.Background()
	uri := os.Getenv("DISCONSOLE_TEST_DB_URI")

	tpool, err := dbauth.NewPoolForDatabase(ctx, uri, "dis_console_ttd_at23", nil)
	if err != nil {
		t.Fatalf("tenant pool: %v", err)
	}
	defer tpool.Close()
	ts := store.New(tpool)
	if err := ts.Migrate(ctx); err != nil {
		t.Fatalf("migrate tenant: %v", err)
	}
	if err := ts.InitMeta(ctx, "e2e"); err != nil {
		t.Fatalf("init tenant meta: %v", err)
	}

	// One tenant event, copied into the central log.
	if _, err := ts.Sync(ctx, []flux.Resource{res("infra", flux.ReadyFalse, "BuildFailed", "sha-1")}); err != nil {
		t.Fatalf("tenant sync 1: %v", err)
	}
	pullAndApply(t, cs, ts, "ttd_at23")

	cursorBefore, err := cs.EventCursor(ctx, "ttd_at23")
	if err != nil || cursorBefore == 0 {
		t.Fatalf("event cursor after first sync: %d err=%v", cursorBefore, err)
	}

	// Age every central event past the window, then purge.
	if _, err := cpool.Exec(ctx,
		"UPDATE flux_status_event SET observed_at = now() - interval '48 hours'"); err != nil {
		t.Fatalf("backdate central events: %v", err)
	}
	purged, err := cs.PurgeExpiredEvents(ctx, 24*time.Hour)
	if err != nil || purged != 1 {
		t.Fatalf("purge: purged=%d err=%v, want 1", purged, err)
	}
	if h, err := cs.History(ctx, "ttd_at23", "Kustomization", "flux-system", "infra"); err != nil || len(h) != 0 {
		t.Fatalf("history should be empty after purge: %+v err=%v", h, err)
	}

	// The copy cursor is untouched: it lives in cluster_report, not in the
	// purged table.
	if c, err := cs.EventCursor(ctx, "ttd_at23"); err != nil || c != cursorBefore {
		t.Fatalf("purge disturbed the event cursor: %d -> %d err=%v", cursorBefore, c, err)
	}

	// A redundant sync must not resurrect the purged event: the tenant still
	// has it, but the cursor records it as already copied.
	pullAndApply(t, cs, ts, "ttd_at23")
	if h, err := cs.History(ctx, "ttd_at23", "Kustomization", "flux-system", "infra"); err != nil || len(h) != 0 {
		t.Fatalf("redundant sync resurrected purged events: %+v err=%v", h, err)
	}

	// Newer tenant events still flow after a purge.
	if _, err := ts.Sync(ctx, []flux.Resource{res("infra", flux.ReadyTrue, "ReconciliationSucceeded", "sha-2")}); err != nil {
		t.Fatalf("tenant sync 2: %v", err)
	}
	pullAndApply(t, cs, ts, "ttd_at23")
	h, err := cs.History(ctx, "ttd_at23", "Kustomization", "flux-system", "infra")
	if err != nil || len(h) != 1 || h[0].Ready != flux.ReadyTrue {
		t.Fatalf("new event after purge: %+v err=%v", h, err)
	}
	if c, err := cs.EventCursor(ctx, "ttd_at23"); err != nil || c <= cursorBefore {
		t.Fatalf("cursor should advance past %d for the new event, got %d err=%v", cursorBefore, c, err)
	}

	// Rows inside the window survive a purge.
	if purged, err := cs.PurgeExpiredEvents(ctx, 24*time.Hour); err != nil || purged != 0 {
		t.Fatalf("fresh event must survive the purge: purged=%d err=%v", purged, err)
	}
}

// pullAndApply mirrors the engine's per-cluster sync against a tenant store: pull
// the rows + events newer than the central cursors, then apply both (with the
// advanced cursors) to the central store in one transaction.
func pullAndApply(t *testing.T, cs *central.Store, ts *store.Store, cluster string) {
	t.Helper()
	ctx := context.Background()
	cur, err := cs.Cursor(ctx, cluster)
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	changed, err := ts.ChangedSince(ctx, cur, store.SchemaVersion)
	if err != nil {
		t.Fatalf("changed-since: %v", err)
	}
	keys, err := ts.Keys(ctx)
	if err != nil {
		t.Fatalf("keys: %v", err)
	}
	ec, err := cs.EventCursor(ctx, cluster)
	if err != nil {
		t.Fatalf("event cursor: %v", err)
	}
	events, err := ts.EventsSince(ctx, ec)
	if err != nil {
		t.Fatalf("events-since: %v", err)
	}
	newCur := cur
	for _, c := range changed {
		if c.UpdatedAt.After(newCur) {
			newCur = c.UpdatedAt
		}
	}
	newEC := ec
	for _, e := range events {
		if e.ID > newEC {
			newEC = e.ID
		}
	}
	if err := cs.Apply(ctx, central.ClusterState{
		Cluster:       cluster,
		Environment:   "at23",
		Changed:       changed,
		Keys:          keys,
		Cursor:        newCur,
		Events:        events,
		EventCursor:   newEC,
		SchemaVersion: store.SchemaVersion,
		AgentVersion:  "e2e",
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
}
