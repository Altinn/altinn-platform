package central

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/dbauth"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/jackc/pgx/v5"
)

// Engine incrementally syncs every tenant database on the shared server into
// the central read model. It connects to each tenant by overriding the database
// name on its own (central) connection URI, so they share host/user/auth.
type Engine struct {
	central  *Store
	baseURI  string
	cred     azcore.TokenCredential
	interval time.Duration
}

// NewEngine builds a sync engine. baseURI is the central database URI; tenant
// connections derive from it (same server, different database). cred is nil for
// Kind/CI/local (trust/PGPASSWORD).
func NewEngine(central *Store, baseURI string, cred azcore.TokenCredential, interval time.Duration) *Engine {
	return &Engine{central: central, baseURI: baseURI, cred: cred, interval: interval}
}

// Run syncs all tenant databases once, calls ready (so /readyz can flip), then
// repeats every interval until ctx is cancelled.
func (e *Engine) Run(ctx context.Context, ready func()) {
	e.syncOnce(ctx)
	ready()

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.syncOnce(ctx)
		}
	}
}

// syncOnce discovers the tenant databases and syncs each in turn. One tenant's
// failure is logged and skipped so it cannot stall the rest of the fleet.
func (e *Engine) syncOnce(ctx context.Context) {
	dbs, err := e.central.Discover(ctx)
	if err != nil {
		log.Printf("discover tenant databases failed: %v", err)
		return
	}

	var synced, failed int
	for _, db := range dbs {
		if ctx.Err() != nil {
			return
		}
		if err := e.syncCluster(ctx, db); err != nil {
			failed++
			log.Printf("sync %s failed, keeping previous data: %v", db, err)
			continue
		}
		synced++
	}
	log.Printf("synced %d/%d tenant databases (%d failed)", synced, len(dbs), failed)
}

// syncCluster mirrors one tenant database into the central read model: pull the
// rows changed since the cursor plus the full key set, then apply both (upsert
// + prune + report) in one central transaction.
func (e *Engine) syncCluster(ctx context.Context, dbName string) error {
	pool, err := dbauth.NewPoolForDatabase(ctx, e.baseURI, dbName, e.cred)
	if err != nil {
		return fmt.Errorf("tenant pool: %w", err)
	}
	defer pool.Close()
	ts := store.New(pool)

	cluster := clusterID(dbName)

	meta, err := ts.GetMeta(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		// A tenant database without a meta row predates schema versioning;
		// treat it as version 0 (legacy) rather than failing the cluster.
		meta = store.Meta{SchemaVersion: 0}
	} else if err != nil {
		return fmt.Errorf("read tenant meta: %w", err)
	}
	if !schemaSupported(meta.SchemaVersion) {
		log.Printf("skip %s: unsupported tenant schema_version %d (server supports %d and %d)",
			cluster, meta.SchemaVersion, store.SchemaVersion-1, store.SchemaVersion)
		return nil
	}

	cursor, err := e.central.Cursor(ctx, cluster)
	if err != nil {
		return err
	}
	changed, err := ts.ChangedSince(ctx, cursor)
	if err != nil {
		return err
	}
	keys, err := ts.Keys(ctx)
	if err != nil {
		return err
	}

	return e.central.Apply(ctx, ClusterState{
		Cluster:       cluster,
		Environment:   environmentOf(cluster),
		Changed:       changed,
		Keys:          keys,
		Cursor:        advanceCursor(cursor, changed),
		SchemaVersion: meta.SchemaVersion,
		AgentVersion:  meta.AgentVersion,
		LastSweepAt:   meta.LastSweepAt,
	})
}
