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

// TestSyncAppliedByRoundTrip checks the schema-v3 columns against real
// PostgreSQL: a HelmRelease's projected owning Kustomization survives Sync and
// comes back out of Get and ChangedSince (the path the server mirrors it on),
// while an unowned root stays nil.
func TestSyncAppliedByRoundTrip(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()

	child := flux.Resource{
		Kind:        "HelmRelease",
		APIVersion:  "helm.toolkit.fluxcd.io/v2",
		Namespace:   "grafana",
		Name:        "grafana-operator",
		Ready:       flux.ReadyTrue,
		AppliedBy:   &flux.AppliedBy{Name: "grafana-operator-grafana-operator", Namespace: "flux-system"},
		Raw:         json.RawMessage(`{"kind":"HelmRelease"}`),
		ContentHash: "child-1",
	}
	root := flux.Resource{
		Kind:        "Kustomization",
		APIVersion:  "kustomize.toolkit.fluxcd.io/v1",
		Namespace:   "flux-system",
		Name:        "root",
		Ready:       flux.ReadyTrue,
		Raw:         json.RawMessage(`{"kind":"Kustomization"}`),
		ContentHash: "root-1",
	}
	if _, err := s.Sync(ctx, []flux.Resource{child, root}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	got, _, err := s.Get(ctx, "HelmRelease", "grafana", "grafana-operator")
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if got.AppliedBy == nil || got.AppliedBy.Name != "grafana-operator-grafana-operator" || got.AppliedBy.Namespace != "flux-system" {
		t.Fatalf("child Get AppliedBy: %+v", got.AppliedBy)
	}

	gotRoot, _, err := s.Get(ctx, "Kustomization", "flux-system", "root")
	if err != nil {
		t.Fatalf("get root: %v", err)
	}
	if gotRoot.AppliedBy != nil {
		t.Fatalf("root Get AppliedBy should be nil: %+v", gotRoot.AppliedBy)
	}

	changed, err := s.ChangedSince(ctx, time.Time{}, store.SchemaVersion)
	if err != nil {
		t.Fatalf("changed-since: %v", err)
	}
	var seenChild bool
	for _, c := range changed {
		if c.Name == "grafana-operator" {
			seenChild = true
			if c.AppliedBy == nil || c.AppliedBy.Name != "grafana-operator-grafana-operator" {
				t.Fatalf("ChangedSince child AppliedBy: %+v", c.AppliedBy)
			}
		}
	}
	if !seenChild {
		t.Fatalf("ChangedSince did not return child row: %+v", changed)
	}
}

// TestSyncBaseLayerRoundTrip checks the schema-v4 columns against real
// PostgreSQL: an OCIRepository's url/origin and a Kustomization's sourceRef +
// inventory survive Sync and come back out of Get and ChangedSince, while List
// carries everything except the inventory payload (detail-only, like raw), and
// the previous-version (v4) pull still selects them — they predate v5.
func TestSyncBaseLayerRoundTrip(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()

	repo := flux.Resource{
		Kind: "OCIRepository", APIVersion: "source.toolkit.fluxcd.io/v1",
		Namespace: "product-team-a", Name: "team-a-ab12",
		Ready: flux.ReadyTrue, Revision: "at23@sha256:abc",
		SourceURL:      "oci://registry.example.com/team-a/syncroot",
		OriginRevision: "main/0c2a3b4", OriginSource: "https://git.example.com/team-a/syncroot",
		Raw: json.RawMessage(`{"kind":"OCIRepository"}`), ContentHash: "repo-1",
	}
	kust := flux.Resource{
		Kind: "Kustomization", APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Namespace: "product-team-a", Name: "team-a-ab12-team-a",
		Ready: flux.ReadyTrue, Revision: "at23@sha256:abc",
		SourceRef: &flux.SourceRef{Kind: "OCIRepository", Name: "team-a-ab12", Namespace: "product-team-a"},
		Inventory: []flux.InventoryEntry{
			{ID: "product-team-a_appdb_storage.dis.altinn.cloud_Database", Version: "v1alpha1"},
			{ID: "product-team-a_app_apps_Deployment", Version: "v1"},
		},
		Raw: json.RawMessage(`{"kind":"Kustomization"}`), ContentHash: "kust-1",
	}
	if _, err := s.Sync(ctx, []flux.Resource{repo, kust}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	gotKust, _, err := s.Get(ctx, "Kustomization", "product-team-a", "team-a-ab12-team-a")
	if err != nil {
		t.Fatalf("get kustomization: %v", err)
	}
	if gotKust.SourceRef == nil || gotKust.SourceRef.Kind != "OCIRepository" ||
		gotKust.SourceRef.Name != "team-a-ab12" || gotKust.SourceRef.Namespace != "product-team-a" {
		t.Fatalf("kustomization Get sourceRef: %+v", gotKust.SourceRef)
	}
	if len(gotKust.Inventory) != 2 ||
		gotKust.Inventory[0].ID != "product-team-a_appdb_storage.dis.altinn.cloud_Database" ||
		gotKust.Inventory[0].Version != "v1alpha1" {
		t.Fatalf("kustomization Get inventory: %+v", gotKust.Inventory)
	}

	gotRepo, _, err := s.Get(ctx, "OCIRepository", "product-team-a", "team-a-ab12")
	if err != nil {
		t.Fatalf("get repository: %v", err)
	}
	if gotRepo.SourceURL != "oci://registry.example.com/team-a/syncroot" ||
		gotRepo.OriginRevision != "main/0c2a3b4" || gotRepo.OriginSource != "https://git.example.com/team-a/syncroot" {
		t.Fatalf("repository Get base-layer fields: %+v", gotRepo)
	}

	rows, err := s.List(ctx, "Kustomization", "", "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("list kustomizations: %+v err=%v", rows, err)
	}
	if rows[0].SourceRef == nil || rows[0].SourceRef.Name != "team-a-ab12" {
		t.Fatalf("list should carry sourceRef: %+v", rows[0])
	}
	if rows[0].Inventory != nil {
		t.Fatalf("list must omit the inventory payload, got %+v", rows[0].Inventory)
	}

	changed, err := s.ChangedSince(ctx, time.Time{}, store.SchemaVersion)
	if err != nil || len(changed) != 2 {
		t.Fatalf("changed-since: %d rows err=%v", len(changed), err)
	}
	for _, c := range changed {
		switch c.Kind {
		case "Kustomization":
			if c.SourceRef == nil || len(c.Inventory) != 2 {
				t.Fatalf("changed kustomization lost base-layer fields: %+v", c.Resource)
			}
		case "OCIRepository":
			if c.SourceURL == "" || c.OriginRevision == "" {
				t.Fatalf("changed repository lost base-layer fields: %+v", c.Resource)
			}
		}
	}

	// The previous-version SELECT (schema 4 — what the server uses against
	// tenants whose agents have not migrated to v5 yet) still carries the
	// base-layer fields: they predate v5, only images are gated on it (see
	// TestSyncImagesRoundTrip).
	v4, err := s.ChangedSince(ctx, time.Time{}, store.SchemaVersion-1)
	if err != nil {
		t.Fatalf("v4 changed-since: %v", err)
	}
	for _, c := range v4 {
		switch c.Kind {
		case "Kustomization":
			if c.SourceRef == nil || len(c.Inventory) != 2 {
				t.Fatalf("v4 pull lost base-layer fields: %+v", c.Resource)
			}
		case "OCIRepository":
			if c.SourceURL == "" || c.OriginRevision == "" {
				t.Fatalf("v4 pull lost base-layer fields: %+v", c.Resource)
			}
		}
	}
}

// TestSyncBaseLayerBackfillAdvancesUpdatedAt reproduces the v3→v4 upgrade:
// rows written before the base-layer projection have the columns NULL with an
// unchanged content hash (sourceRef, url and inventory were already inside the
// hashed object). The first sweep that projects them must advance updated_at
// so the central pull mirrors the backfilled row — and later identical sweeps
// must not churn it. Same contract the appliedBy backfill established.
func TestSyncBaseLayerBackfillAdvancesUpdatedAt(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()

	updatedAt := func() time.Time {
		var ts time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM flux_resource WHERE name = $1", "team-a-ab12-team-a").Scan(&ts); err != nil {
			t.Fatalf("query updated_at: %v", err)
		}
		return ts
	}

	// Sweep 1: the pre-v4 shape (same object, projections not derived yet).
	pre := flux.Resource{
		Kind: "Kustomization", APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Namespace: "product-team-a", Name: "team-a-ab12-team-a",
		Ready: flux.ReadyTrue,
		Raw:   json.RawMessage(`{"kind":"Kustomization"}`), ContentHash: "same-object",
	}
	if _, err := s.Sync(ctx, []flux.Resource{pre}); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	first := updatedAt()

	// Sweep 2: identical object + hash, but the agent now projects the fields.
	post := pre
	post.SourceRef = &flux.SourceRef{Kind: "OCIRepository", Name: "team-a-ab12", Namespace: "product-team-a"}
	post.Inventory = []flux.InventoryEntry{{ID: "product-team-a_app_apps_Deployment", Version: "v1"}}
	if _, err := s.Sync(ctx, []flux.Resource{post}); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	second := updatedAt()
	if !second.After(first) {
		t.Fatalf("updated_at must advance when base-layer fields are backfilled: %v -> %v", first, second)
	}

	changed, err := s.ChangedSince(ctx, first, store.SchemaVersion)
	if err != nil {
		t.Fatalf("changed-since: %v", err)
	}
	if len(changed) != 1 || changed[0].SourceRef == nil || len(changed[0].Inventory) != 1 {
		t.Fatalf("backfilled row not pulled: %+v", changed)
	}

	// Sweep 3: identical again (projections unchanged) => no churn.
	if _, err := s.Sync(ctx, []flux.Resource{post}); err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if got := updatedAt(); !got.Equal(second) {
		t.Fatalf("updated_at churned on an unchanged base-layer sweep: %v -> %v", second, got)
	}
}

// TestSyncImagesRoundTrip checks the schema-v5 column against real PostgreSQL:
// a mirrored workload's container images survive Sync and come back out of
// List (the UI reads the list — no detail fetch), Get and ChangedSince, while
// a version-4 pull (agent not yet migrated) omits them entirely.
func TestSyncImagesRoundTrip(t *testing.T) {
	s, _ := newStore(t)
	ctx := context.Background()

	deploy := flux.Resource{
		Kind: "Deployment", APIVersion: "apps/v1",
		Namespace: "product-team-a", Name: "app",
		Ready: flux.ReadyTrue, Reason: "MinimumReplicasAvailable",
		AppliedBy: &flux.AppliedBy{Name: "team-a-ab12-team-a", Namespace: "product-team-a"},
		Images: []flux.ContainerImage{
			{Container: "app", Image: "registry.example.com/team-a/app:v42"},
			{Container: "sidecar", Image: "registry.example.com/team-a/sidecar:v1"},
		},
		Raw: json.RawMessage(`{"kind":"Deployment"}`), ContentHash: "deploy-1",
	}
	kust := flux.Resource{
		Kind: "Kustomization", APIVersion: "kustomize.toolkit.fluxcd.io/v1",
		Namespace: "product-team-a", Name: "team-a-ab12-team-a",
		Ready: flux.ReadyTrue,
		Raw:   json.RawMessage(`{"kind":"Kustomization"}`), ContentHash: "kust-1",
	}
	if _, err := s.Sync(ctx, []flux.Resource{deploy, kust}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	rows, err := s.List(ctx, "Deployment", "", "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("list deployments: %+v err=%v", rows, err)
	}
	if len(rows[0].Images) != 2 ||
		rows[0].Images[0] != (flux.ContainerImage{Container: "app", Image: "registry.example.com/team-a/app:v42"}) ||
		rows[0].Images[1] != (flux.ContainerImage{Container: "sidecar", Image: "registry.example.com/team-a/sidecar:v1"}) {
		t.Fatalf("list must carry images: %+v", rows[0].Images)
	}

	got, _, err := s.Get(ctx, "Deployment", "product-team-a", "app")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if len(got.Images) != 2 || got.Images[0].Container != "app" {
		t.Fatalf("get lost images: %+v", got.Images)
	}

	gotKust, _, err := s.Get(ctx, "Kustomization", "product-team-a", "team-a-ab12-team-a")
	if err != nil {
		t.Fatalf("get kustomization: %v", err)
	}
	if gotKust.Images != nil {
		t.Fatalf("non-workload rows must keep nil images, got %+v", gotKust.Images)
	}

	changed, err := s.ChangedSince(ctx, time.Time{}, store.SchemaVersion)
	if err != nil || len(changed) != 2 {
		t.Fatalf("changed-since: %d rows err=%v", len(changed), err)
	}
	for _, c := range changed {
		if c.Kind == "Deployment" && len(c.Images) != 2 {
			t.Fatalf("changed deployment lost images: %+v", c.Resource)
		}
	}

	// The version-4 SELECT (what the server uses against tenants whose agents
	// have not migrated yet) must omit images but still succeed.
	v4, err := s.ChangedSince(ctx, time.Time{}, store.SchemaVersion-1)
	if err != nil {
		t.Fatalf("v4 changed-since: %v", err)
	}
	for _, c := range v4 {
		if c.Images != nil {
			t.Fatalf("v4 pull must not select images: %+v", c.Resource)
		}
	}
}

// TestSyncImagesBackfillAdvancesUpdatedAt reproduces the v4→v5 upgrade for a
// projection derived from already-hashed content: a workload row written
// before the images projection has the column NULL with an unchanged content
// hash. The first sweep that projects images must advance updated_at so the
// central pull mirrors the backfilled row — and later identical sweeps must
// not churn it. Same contract as the appliedBy and base-layer backfills.
func TestSyncImagesBackfillAdvancesUpdatedAt(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()

	updatedAt := func() time.Time {
		var ts time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM flux_resource WHERE name = $1", "app").Scan(&ts); err != nil {
			t.Fatalf("query updated_at: %v", err)
		}
		return ts
	}

	// Sweep 1: the pre-v5 shape (same object, images not projected yet).
	pre := flux.Resource{
		Kind: "Deployment", APIVersion: "apps/v1",
		Namespace: "product-team-a", Name: "app",
		Ready: flux.ReadyTrue,
		Raw:   json.RawMessage(`{"kind":"Deployment"}`), ContentHash: "same-object",
	}
	if _, err := s.Sync(ctx, []flux.Resource{pre}); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	first := updatedAt()

	// Sweep 2: identical object + hash, but the agent now projects images.
	post := pre
	post.Images = []flux.ContainerImage{{Container: "app", Image: "registry.example.com/team-a/app:v42"}}
	if _, err := s.Sync(ctx, []flux.Resource{post}); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	second := updatedAt()
	if !second.After(first) {
		t.Fatalf("updated_at must advance when images are backfilled: %v -> %v", first, second)
	}

	changed, err := s.ChangedSince(ctx, first, store.SchemaVersion)
	if err != nil {
		t.Fatalf("changed-since: %v", err)
	}
	if len(changed) != 1 || len(changed[0].Images) != 1 {
		t.Fatalf("backfilled row not pulled: %+v", changed)
	}

	// Sweep 3: identical again (images unchanged) => no churn.
	if _, err := s.Sync(ctx, []flux.Resource{post}); err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if got := updatedAt(); !got.Equal(second) {
		t.Fatalf("updated_at churned on an unchanged images sweep: %v -> %v", second, got)
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

// TestSyncAppliedByBackfillAdvancesUpdatedAt reproduces the v1.4.0→v1.5.0
// upgrade: rows written before the appliedBy projection have it NULL with an
// unchanged content hash (the labels were already in the hashed object). The
// first sweep that projects appliedBy must advance updated_at so the central
// pull (updated_at > cursor) mirrors the backfilled row — and later identical
// sweeps must not churn it.
func TestSyncAppliedByBackfillAdvancesUpdatedAt(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()

	updatedAt := func() time.Time {
		var ts time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM flux_resource WHERE name = $1", "grafana-operator").Scan(&ts); err != nil {
			t.Fatalf("query updated_at: %v", err)
		}
		return ts
	}

	// Sweep 1: the pre-appliedBy shape (same object, projection not derived yet).
	pre := flux.Resource{
		Kind:        "HelmRelease",
		APIVersion:  "helm.toolkit.fluxcd.io/v2",
		Namespace:   "grafana",
		Name:        "grafana-operator",
		Ready:       flux.ReadyTrue,
		Raw:         json.RawMessage(`{"kind":"HelmRelease"}`),
		ContentHash: "same-object",
	}
	if _, err := s.Sync(ctx, []flux.Resource{pre}); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	first := updatedAt()

	// Sweep 2: identical object + hash, but the agent now projects appliedBy.
	post := pre
	post.AppliedBy = &flux.AppliedBy{Name: "grafana-operator-grafana-operator", Namespace: "flux-system"}
	if _, err := s.Sync(ctx, []flux.Resource{post}); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	second := updatedAt()
	if !second.After(first) {
		t.Fatalf("updated_at must advance when appliedBy is backfilled: %v -> %v", first, second)
	}

	// The advanced updated_at is what makes the row visible to the central pull.
	changed, err := s.ChangedSince(ctx, first, store.SchemaVersion)
	if err != nil {
		t.Fatalf("changed-since: %v", err)
	}
	if len(changed) != 1 || changed[0].AppliedBy == nil || changed[0].AppliedBy.Name != "grafana-operator-grafana-operator" {
		t.Fatalf("backfilled row not pulled: %+v", changed)
	}

	// Sweep 3: identical again (appliedBy unchanged) => no churn.
	if _, err := s.Sync(ctx, []flux.Resource{post}); err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if got := updatedAt(); !got.Equal(second) {
		t.Fatalf("updated_at churned on an unchanged appliedBy sweep: %v -> %v", second, got)
	}
}

// TestSyncEventRetention drives the time-based event purge against real
// PostgreSQL: with a retention window set, Sync deletes history events older
// than the window inside the sweep's transaction, fresh events and the
// resource rows themselves survive, and with retention unset (the default)
// nothing is ever purged.
func TestSyncEventRetention(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()

	countEvents := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM flux_status_event").Scan(&n); err != nil {
			t.Fatalf("count events: %v", err)
		}
		return n
	}

	// Two sweeps: initial upsert, then a status change => two history events.
	if _, err := s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyTrue, "OK", "sha-1")}); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	if _, err := s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyFalse, "Boom", "sha-2")}); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	if n := countEvents(); n != 2 {
		t.Fatalf("expected 2 events after two sweeps, got %d", n)
	}

	// Age the first event past the window configured below.
	if _, err := pool.Exec(ctx,
		"UPDATE flux_status_event SET observed_at = now() - interval '48 hours' WHERE ready = $1",
		flux.ReadyTrue); err != nil {
		t.Fatalf("backdate event: %v", err)
	}

	// Retention disabled (the default): a sweep must not purge anything.
	stats, err := s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyFalse, "Boom", "sha-2")})
	if err != nil {
		t.Fatalf("sync 3: %v", err)
	}
	if stats.EventsExpired != 0 || countEvents() != 2 {
		t.Fatalf("disabled retention purged events: stats=%+v events=%d", stats, countEvents())
	}

	// 24h retention: the next sweep drops the aged event and keeps the fresh one.
	s.SetEventRetention(24 * time.Hour)
	stats, err = s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyFalse, "Boom", "sha-2")})
	if err != nil {
		t.Fatalf("sync 4: %v", err)
	}
	if stats.EventsExpired != 1 || countEvents() != 1 {
		t.Fatalf("expected exactly the aged event purged: stats=%+v events=%d", stats, countEvents())
	}

	// The resource row is untouched and the surviving history is the fresh event.
	row, history, err := s.Get(ctx, "Kustomization", "flux-system", "apps")
	if err != nil {
		t.Fatalf("get apps: %v", err)
	}
	if row.Ready != flux.ReadyFalse {
		t.Fatalf("purge disturbed the resource row: %+v", row)
	}
	if len(history) != 1 || history[0].Ready != flux.ReadyFalse {
		t.Fatalf("unexpected history after purge: %+v", history)
	}

	// Steady state: nothing is old enough anymore, so the next sweep purges nothing.
	stats, err = s.Sync(ctx, []flux.Resource{res("apps", flux.ReadyFalse, "Boom", "sha-2")})
	if err != nil {
		t.Fatalf("sync 5: %v", err)
	}
	if stats.EventsExpired != 0 || countEvents() != 1 {
		t.Fatalf("steady-state sweep must purge nothing: stats=%+v events=%d", stats, countEvents())
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
