// Package central holds the console's central read model — a cluster-keyed
// mirror of every tenant database on the shared server — plus the sync engine
// that incrementally fills it. The fleet API (a later slice) reads only this
// central database; agents never read it.
package central

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// dbPrefix is the naming prefix of the per-tenant databases on the shared
// server. The server discovers tenants by this prefix and derives a cluster id
// by stripping it.
const dbPrefix = "dis_console_"

// Store is the central read model in PostgreSQL (the console's own database).
type Store struct {
	pool *pgxpool.Pool
}

// New wraps an existing pool connected to the central database.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping verifies the central database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Migrate applies the embedded central schema (idempotent).
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply central schema: %w", err)
	}
	return nil
}

const cursorStmt = `SELECT sync_cursor FROM cluster_report WHERE cluster = $1`

// Cursor returns a cluster's last synced high-water (the newest tenant
// updated_at mirrored so far). Zero time means the cluster has never synced.
func (s *Store) Cursor(ctx context.Context, cluster string) (time.Time, error) {
	var t *time.Time
	err := s.pool.QueryRow(ctx, cursorStmt, cluster).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("cursor query: %w", err)
	}
	if t == nil {
		return time.Time{}, nil
	}
	return t.UTC(), nil
}

const discoverStmt = `
SELECT datname FROM pg_database
WHERE starts_with(datname, $1) AND datname <> current_database() AND datistemplate = false
ORDER BY datname`

// Discover lists the tenant databases on the shared server (by the dis_console_
// prefix), excluding the console's own central database.
func (s *Store) Discover(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, discoverStmt, dbPrefix)
	if err != nil {
		return nil, fmt.Errorf("discover databases: %w", err)
	}
	defer rows.Close()

	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan datname: %w", err)
		}
		dbs = append(dbs, name)
	}
	return dbs, rows.Err()
}

// ClusterState is one cluster's sync result, applied to the central DB atomically.
type ClusterState struct {
	Cluster       string
	Environment   string
	Changed       []store.ChangedResource // rows to upsert (content changed since the cursor)
	Keys          []store.ResourceKey     // the tenant's full current key set (prune basis)
	Cursor        time.Time               // new high-water
	SchemaVersion int
	AgentVersion  string
	LastSweepAt   time.Time // agent's last sweep (data freshness), from tenant meta
}

const upsertStmt = `
INSERT INTO flux_resource (
    cluster, kind, api_version, namespace, name,
    ready, reason, message, revision, suspended,
    generation, observed_generation, last_transition, raw, content_hash,
    updated_at, synced_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, now()
)
ON CONFLICT (cluster, kind, namespace, name) DO UPDATE SET
    api_version         = EXCLUDED.api_version,
    ready               = EXCLUDED.ready,
    reason              = EXCLUDED.reason,
    message             = EXCLUDED.message,
    revision            = EXCLUDED.revision,
    suspended           = EXCLUDED.suspended,
    generation          = EXCLUDED.generation,
    observed_generation = EXCLUDED.observed_generation,
    last_transition     = EXCLUDED.last_transition,
    raw                 = EXCLUDED.raw,
    content_hash        = EXCLUDED.content_hash,
    updated_at          = EXCLUDED.updated_at,
    synced_at           = now()`

// pruneStmt deletes the cluster's mirrored rows whose identity is not in the
// tenant's current key set (passed as parallel arrays). An empty key set drops
// every row for the cluster, which is correct when the tenant has none.
const pruneStmt = `
DELETE FROM flux_resource c
WHERE c.cluster = $1
  AND NOT EXISTS (
    SELECT 1 FROM unnest($2::text[], $3::text[], $4::text[]) AS k(kind, namespace, name)
    WHERE k.kind = c.kind AND k.namespace = c.namespace AND k.name = c.name
  )`

const reportStmt = `
INSERT INTO cluster_report (
    cluster, environment, sync_cursor, last_synced_at, last_sweep_at, agent_version, schema_version, resource_count
) VALUES ($1, $2, $3, now(), $4, $5, $6, $7)
ON CONFLICT (cluster) DO UPDATE SET
    environment    = EXCLUDED.environment,
    sync_cursor    = EXCLUDED.sync_cursor,
    last_synced_at = now(),
    last_sweep_at  = EXCLUDED.last_sweep_at,
    agent_version  = EXCLUDED.agent_version,
    schema_version = EXCLUDED.schema_version,
    resource_count = EXCLUDED.resource_count`

// Apply mirrors one cluster's state into the central DB in a single transaction
// — upsert the changed rows, prune the ones that disappeared, and record the
// cluster_report — so readers never observe a half-synced cluster.
func (s *Store) Apply(ctx context.Context, st ClusterState) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(st.Changed) > 0 {
		batch := &pgx.Batch{}
		for i := range st.Changed {
			c := &st.Changed[i]
			raw := c.Raw
			if len(raw) == 0 {
				raw = json.RawMessage("{}")
			}
			batch.Queue(upsertStmt,
				st.Cluster, c.Kind, c.APIVersion, c.Namespace, c.Name,
				c.Ready, c.Reason, c.Message, c.Revision, c.Suspended,
				c.Generation, c.ObservedGeneration, c.LastTransition, []byte(raw), c.ContentHash,
				c.UpdatedAt,
			)
		}
		br := tx.SendBatch(ctx, batch)
		for range st.Changed {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return fmt.Errorf("upsert cluster %s: %w", st.Cluster, err)
			}
		}
		if err := br.Close(); err != nil {
			return fmt.Errorf("close batch: %w", err)
		}
	}

	kinds, namespaces, names := splitKeys(st.Keys)
	if _, err := tx.Exec(ctx, pruneStmt, st.Cluster, kinds, namespaces, names); err != nil {
		return fmt.Errorf("prune cluster %s: %w", st.Cluster, err)
	}

	var cursor, lastSweep *time.Time
	if !st.Cursor.IsZero() {
		c := st.Cursor.UTC()
		cursor = &c
	}
	if !st.LastSweepAt.IsZero() {
		ls := st.LastSweepAt.UTC()
		lastSweep = &ls
	}
	if _, err := tx.Exec(ctx, reportStmt,
		st.Cluster, st.Environment, cursor, lastSweep, st.AgentVersion, st.SchemaVersion, len(st.Keys),
	); err != nil {
		return fmt.Errorf("cluster_report %s: %w", st.Cluster, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func splitKeys(keys []store.ResourceKey) (kinds, namespaces, names []string) {
	kinds = make([]string, len(keys))
	namespaces = make([]string, len(keys))
	names = make([]string, len(keys))
	for i, k := range keys {
		kinds[i], namespaces[i], names[i] = k.Kind, k.Namespace, k.Name
	}
	return kinds, namespaces, names
}

// clusterID strips the tenant-database prefix to get the cluster identifier
// (e.g. "dis_console_ttd_at23" -> "ttd_at23").
func clusterID(dbName string) string { return strings.TrimPrefix(dbName, dbPrefix) }

// environmentOf derives the environment from a cluster id as the segment after
// the last underscore (e.g. "ttd_at23" -> "at23"); "" when there is none.
func environmentOf(cluster string) string {
	if i := strings.LastIndex(cluster, "_"); i >= 0 && i < len(cluster)-1 {
		return cluster[i+1:]
	}
	return ""
}

// schemaSupported reports whether the server can read a tenant at the given
// meta.schema_version. It supports the current version and the previous one;
// anything else is flagged by the caller and skipped rather than misread.
func schemaSupported(version int) bool {
	return version >= store.SchemaVersion-1 && version <= store.SchemaVersion
}

// advanceCursor returns the new high-water: the newest updated_at among the
// pulled rows, or the old cursor when nothing changed (it never regresses).
func advanceCursor(old time.Time, changed []store.ChangedResource) time.Time {
	next := old
	for i := range changed {
		if changed[i].UpdatedAt.After(next) {
			next = changed[i].UpdatedAt
		}
	}
	return next
}

// --- read side (the fleet API reads only these) ---

// ErrNotFound is returned by Get when no central row matches.
var ErrNotFound = errors.New("resource not found")

// Resource is a mirrored resource tagged with the cluster it came from.
type Resource struct {
	flux.Resource
	Cluster string `json:"cluster"`
}

// Cluster is a tenant's sync status, served by /api/clusters.
type Cluster struct {
	Cluster       string    `json:"cluster"`
	Environment   string    `json:"environment,omitempty"`
	LastSweepAt   time.Time `json:"lastSweepAt"`
	LastSyncedAt  time.Time `json:"lastSyncedAt"`
	AgentVersion  string    `json:"agentVersion,omitempty"`
	SchemaVersion int       `json:"schemaVersion"`
	ResourceCount int       `json:"resourceCount"`
	Stale         bool      `json:"stale"`
}

const clustersStmt = `
SELECT cluster, environment, last_sweep_at, last_synced_at, agent_version, schema_version, resource_count
FROM cluster_report
ORDER BY cluster`

// Clusters returns every synced cluster with a staleness flag — set when the
// agent stopped sweeping or the console stopped syncing it (either timestamp
// older than staleAfter).
func (s *Store) Clusters(ctx context.Context, staleAfter time.Duration) ([]Cluster, error) {
	rows, err := s.pool.Query(ctx, clustersStmt)
	if err != nil {
		return nil, fmt.Errorf("clusters query: %w", err)
	}
	defer rows.Close()

	out := []Cluster{}
	for rows.Next() {
		var c Cluster
		var sweep, synced *time.Time
		if err := rows.Scan(&c.Cluster, &c.Environment, &sweep, &synced,
			&c.AgentVersion, &c.SchemaVersion, &c.ResourceCount); err != nil {
			return nil, fmt.Errorf("scan cluster: %w", err)
		}
		if sweep != nil {
			c.LastSweepAt = sweep.UTC()
		}
		if synced != nil {
			c.LastSyncedAt = synced.UTC()
		}
		c.Stale = staleSince(c.LastSweepAt, staleAfter) || staleSince(c.LastSyncedAt, staleAfter)
		out = append(out, c)
	}
	return out, rows.Err()
}

// staleSince reports whether t is missing or older than d relative to now.
func staleSince(t time.Time, d time.Duration) bool {
	return t.IsZero() || time.Since(t) > d
}

const summaryStmt = `
SELECT kind,
       count(*),
       count(*) FILTER (WHERE ready = 'True'),
       count(*) FILTER (WHERE ready = 'False'),
       count(*) FILTER (WHERE ready NOT IN ('True', 'False')),
       count(*) FILTER (WHERE suspended)
FROM flux_resource
WHERE ($1 = '' OR cluster = $1)
GROUP BY kind
ORDER BY kind`

// Summary returns per-kind ready-state counts across the fleet, or for one
// cluster when cluster is non-empty.
func (s *Store) Summary(ctx context.Context, cluster string) ([]store.KindCount, error) {
	rows, err := s.pool.Query(ctx, summaryStmt, cluster)
	if err != nil {
		return nil, fmt.Errorf("summary query: %w", err)
	}
	defer rows.Close()

	out := []store.KindCount{}
	for rows.Next() {
		var c store.KindCount
		if err := rows.Scan(&c.Kind, &c.Total, &c.Ready, &c.NotReady, &c.Unknown, &c.Suspended); err != nil {
			return nil, fmt.Errorf("scan summary: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const listStmt = `
SELECT cluster, kind, api_version, namespace, name, ready, reason, message, revision,
       suspended, generation, observed_generation, last_transition
FROM flux_resource
WHERE ($1 = '' OR cluster = $1)
  AND ($2 = '' OR lower(kind) = lower($2))
  AND ($3 = '' OR namespace = $3)
  AND ($4 = '' OR lower(ready) = lower($4))
ORDER BY cluster, kind, namespace, name`

// List returns matching resources (without the raw payload) across the fleet,
// or for one cluster when cluster is non-empty. An empty kind/namespace/ready
// means "any".
func (s *Store) List(ctx context.Context, cluster, kind, namespace, ready string) ([]Resource, error) {
	rows, err := s.pool.Query(ctx, listStmt, cluster, kind, namespace, ready)
	if err != nil {
		return nil, fmt.Errorf("list query: %w", err)
	}
	defer rows.Close()

	out := []Resource{}
	for rows.Next() {
		var r Resource
		if err := rows.Scan(
			&r.Cluster, &r.Kind, &r.APIVersion, &r.Namespace, &r.Name, &r.Ready, &r.Reason,
			&r.Message, &r.Revision, &r.Suspended, &r.Generation,
			&r.ObservedGeneration, &r.LastTransition,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const getStmt = `
SELECT cluster, kind, api_version, namespace, name, ready, reason, message, revision,
       suspended, generation, observed_generation, last_transition, raw
FROM flux_resource
WHERE cluster = $1 AND lower(kind) = lower($2) AND namespace = $3 AND name = $4`

// Get returns one resource (including its raw payload) in a cluster, or
// ErrNotFound when no row matches.
func (s *Store) Get(ctx context.Context, cluster, kind, namespace, name string) (*Resource, error) {
	var r Resource
	var raw []byte
	err := s.pool.QueryRow(ctx, getStmt, cluster, kind, namespace, name).Scan(
		&r.Cluster, &r.Kind, &r.APIVersion, &r.Namespace, &r.Name, &r.Ready, &r.Reason,
		&r.Message, &r.Revision, &r.Suspended, &r.Generation,
		&r.ObservedGeneration, &r.LastTransition, &raw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get query: %w", err)
	}
	r.Raw = raw
	return &r, nil
}
