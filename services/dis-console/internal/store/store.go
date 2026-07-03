// Package store persists normalized Flux resource snapshots in PostgreSQL and
// serves the read queries behind the JSON API. Each sweep upserts the current
// rows, records a history event whenever a resource's ready/reason/revision
// changes, and prunes rows for objects that have disappeared from the cluster.
package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// historyLimit caps how many recent status events the detail endpoint returns.
const historyLimit = 50

// ErrNotFound is returned by Get when no row matches.
var ErrNotFound = errors.New("resource not found")

// Store reads and writes Flux resource state in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New wraps an existing pgxpool. The store does not own the pool's lifecycle
// beyond Close, which the caller may invoke on shutdown.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Close releases the underlying connection pool.
func (s *Store) Close() { s.pool.Close() }

// Ping verifies the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Migrate applies the embedded schema (idempotent CREATE ... IF NOT EXISTS).
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// SchemaVersion is the version of this tenant database's schema, stamped into
// the meta row at startup. The central console reads it to stay tolerant of
// agents rolling out across the fleet at different times.
//
// Version 2 added the DIS projection columns (azure_resource_id, parent_kind,
// parent_name). Version 3 added the applied-by columns (applied_by_name,
// applied_by_namespace — the owning Kustomization). ChangedSince selects per
// version so the server can still pull version-2 tenants whose agents have not
// migrated yet; version 1 fell out of the supported window (schemaSupported
// covers the current version and the previous one).
const SchemaVersion = 3

// Meta is the single bookkeeping row each agent maintains in its tenant DB.
type Meta struct {
	SchemaVersion int
	AgentVersion  string
	LastSweepAt   time.Time
}

const initMetaStmt = `
INSERT INTO meta (id, schema_version, agent_version)
VALUES (true, $1, $2)
ON CONFLICT (id) DO UPDATE SET
    schema_version = EXCLUDED.schema_version,
    agent_version  = EXCLUDED.agent_version`

// InitMeta seeds (or refreshes) the singleton meta row with this agent's schema
// version and build. It leaves last_sweep_at untouched. Call once at startup,
// before the first sweep.
func (s *Store) InitMeta(ctx context.Context, agentVersion string) error {
	if _, err := s.pool.Exec(ctx, initMetaStmt, SchemaVersion, agentVersion); err != nil {
		return fmt.Errorf("init meta: %w", err)
	}
	return nil
}

// touchMetaStmt advances last_sweep_at; run inside the Sync transaction so the
// recorded time and the data it describes commit together.
const touchMetaStmt = `UPDATE meta SET last_sweep_at = now()`

const getMetaStmt = `SELECT schema_version, agent_version, last_sweep_at FROM meta WHERE id`

// GetMeta returns the bookkeeping row. last_sweep_at is the zero time until the
// first sweep commits.
func (s *Store) GetMeta(ctx context.Context) (Meta, error) {
	var m Meta
	var lastSweep *time.Time
	err := s.pool.QueryRow(ctx, getMetaStmt).Scan(&m.SchemaVersion, &m.AgentVersion, &lastSweep)
	if err != nil {
		return Meta{}, fmt.Errorf("get meta: %w", err)
	}
	if lastSweep != nil {
		m.LastSweepAt = lastSweep.UTC()
	}
	return m, nil
}

// SyncStats reports what a Sync did.
type SyncStats struct {
	Upserted int   // rows seen this sweep
	Changed  int   // rows whose ready/reason/revision changed (history events written)
	Pruned   int64 // rows deleted because their object disappeared from the cluster
}

// parentCols splits an optional parent into its nullable column pair.
func parentCols(p *flux.ParentRef) (kind, name *string) {
	if p == nil {
		return nil, nil
	}
	return &p.Kind, &p.Name
}

// parentRef rebuilds the optional parent from its nullable column pair.
func parentRef(kind, name *string) *flux.ParentRef {
	if kind == nil || name == nil {
		return nil
	}
	return &flux.ParentRef{Kind: *kind, Name: *name}
}

// appliedByCols splits an optional applied-by into its nullable column pair.
func appliedByCols(a *flux.AppliedBy) (name, namespace *string) {
	if a == nil {
		return nil, nil
	}
	return &a.Name, &a.Namespace
}

// appliedByRef rebuilds the optional applied-by from its nullable column pair.
func appliedByRef(name, namespace *string) *flux.AppliedBy {
	if name == nil || namespace == nil {
		return nil
	}
	return &flux.AppliedBy{Name: *name, Namespace: *namespace}
}

// upsertStmt upserts one resource and, in the same statement, writes a history
// event when the row is new or its ready/reason/revision changed. The `prev`
// CTE reads the pre-upsert row (CTEs see the snapshot from the start of the
// statement), so the comparison is against the previously stored values. The
// command tag reflects the trailing INSERT, so RowsAffected() is 1 exactly when
// a history event was written.
//
// raw and updated_at are rewritten only when content_hash changed: the small
// projected columns are cheap to overwrite (and identical when unchanged), but
// re-storing an unchanged raw blob would re-TOAST it and churn WAL on every
// sweep across the whole fleet. Keeping r.raw on the unchanged branch reuses the
// existing TOAST datum. last_seen is always bumped so prune can tell which rows
// were seen this sweep.
const upsertStmt = `
WITH prev AS (
    SELECT ready, reason, revision
    FROM flux_resource
    WHERE kind = $1 AND namespace = $3 AND name = $4
),
up AS (
    INSERT INTO flux_resource AS r (
        kind, api_version, namespace, name,
        ready, reason, message, revision, suspended,
        generation, observed_generation, last_transition, raw, content_hash,
        azure_resource_id, parent_kind, parent_name,
        applied_by_name, applied_by_namespace,
        first_seen, last_seen, updated_at
    ) VALUES (
        $1, $2, $3, $4,
        $5, $6, $7, $8, $9,
        $10, $11, $12, $13, $14,
        $15, $16, $17,
        $18, $19,
        now(), now(), now()
    )
    ON CONFLICT (kind, namespace, name) DO UPDATE SET
        api_version          = EXCLUDED.api_version,
        ready                = EXCLUDED.ready,
        reason               = EXCLUDED.reason,
        message              = EXCLUDED.message,
        revision             = EXCLUDED.revision,
        azure_resource_id    = EXCLUDED.azure_resource_id,
        parent_kind          = EXCLUDED.parent_kind,
        parent_name          = EXCLUDED.parent_name,
        applied_by_name      = EXCLUDED.applied_by_name,
        applied_by_namespace = EXCLUDED.applied_by_namespace,
        suspended            = EXCLUDED.suspended,
        generation           = EXCLUDED.generation,
        observed_generation  = EXCLUDED.observed_generation,
        last_transition      = EXCLUDED.last_transition,
        raw                  = CASE
            WHEN r.content_hash IS DISTINCT FROM EXCLUDED.content_hash
            THEN EXCLUDED.raw ELSE r.raw END,
        content_hash         = EXCLUDED.content_hash,
        last_seen            = now(),
        updated_at           = CASE
            WHEN r.content_hash IS DISTINCT FROM EXCLUDED.content_hash
            THEN now() ELSE r.updated_at END
    RETURNING ready, reason, message, revision
)
INSERT INTO flux_status_event (kind, namespace, name, ready, reason, message, revision)
SELECT $1, $3, $4, up.ready, up.reason, up.message, up.revision
FROM up
WHERE NOT EXISTS (SELECT 1 FROM prev)
   OR up.ready    IS DISTINCT FROM (SELECT ready FROM prev)
   OR up.reason   IS DISTINCT FROM (SELECT reason FROM prev)
   OR up.revision IS DISTINCT FROM (SELECT revision FROM prev)`

// pruneStmt deletes rows not touched by the current sweep. Every present row
// had its last_seen set to the transaction timestamp (now() is constant within
// a transaction); absent rows keep an earlier last_seen from a prior sweep, so
// `< now()` matches exactly the objects that disappeared. Using the DB clock
// avoids any app/DB clock-skew that a wall-clock cutoff would introduce.
const pruneStmt = `DELETE FROM flux_resource WHERE last_seen < now()`

// Sync upserts every resource and prunes objects that disappeared, all in one
// transaction so the API never observes a partial sweep.
func (s *Store) Sync(ctx context.Context, resources []flux.Resource) (SyncStats, error) {
	stats := SyncStats{Upserted: len(resources)}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return stats, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if len(resources) > 0 {
		batch := &pgx.Batch{}
		for i := range resources {
			r := &resources[i]
			raw := r.Raw
			if len(raw) == 0 {
				raw = json.RawMessage("{}")
			}
			parentKind, parentName := parentCols(r.Parent)
			appliedByName, appliedByNamespace := appliedByCols(r.AppliedBy)
			batch.Queue(upsertStmt,
				r.Kind, r.APIVersion, r.Namespace, r.Name,
				r.Ready, r.Reason, r.Message, r.Revision, r.Suspended,
				r.Generation, r.ObservedGeneration, r.LastTransition, []byte(raw), r.ContentHash,
				r.AzureResourceID, parentKind, parentName,
				appliedByName, appliedByNamespace,
			)
		}
		br := tx.SendBatch(ctx, batch)
		for range resources {
			tag, err := br.Exec()
			if err != nil {
				_ = br.Close()
				return stats, fmt.Errorf("upsert: %w", err)
			}
			if tag.RowsAffected() > 0 {
				stats.Changed++
			}
		}
		if err := br.Close(); err != nil {
			return stats, fmt.Errorf("close batch: %w", err)
		}
	}

	tag, err := tx.Exec(ctx, pruneStmt)
	if err != nil {
		return stats, fmt.Errorf("prune: %w", err)
	}
	stats.Pruned = tag.RowsAffected()

	// When resources disappeared, drop their now-orphaned history rows: Get
	// requires the parent resource row, so those events are unreachable and
	// would otherwise accumulate unbounded.
	if stats.Pruned > 0 {
		if _, err := tx.Exec(ctx, pruneEventsStmt); err != nil {
			return stats, fmt.Errorf("prune events: %w", err)
		}
	}

	// Record the sweep time atomically with its data. No-op until InitMeta has
	// seeded the singleton row (the agent does so at startup, before sweeping).
	if _, err := tx.Exec(ctx, touchMetaStmt); err != nil {
		return stats, fmt.Errorf("update meta: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
}

// pruneEventsStmt removes history rows whose resource no longer exists. Run only
// after a resource prune actually deleted rows.
const pruneEventsStmt = `
DELETE FROM flux_status_event e
WHERE NOT EXISTS (
    SELECT 1 FROM flux_resource r
    WHERE r.kind = e.kind AND r.namespace = e.namespace AND r.name = e.name
)`

const lastSweepStmt = `SELECT max(last_seen) FROM flux_resource`

// LastSweep returns the timestamp of the most recent sweep — the maximum
// last_seen across all rows, which every sweep sets to its transaction time.
// Returns the zero time when the table is empty. Sourcing the API's updatedAt
// from this (rather than process memory) keeps it correct across restarts and
// free of app/DB clock skew.
func (s *Store) LastSweep(ctx context.Context) (time.Time, error) {
	var t *time.Time
	if err := s.pool.QueryRow(ctx, lastSweepStmt).Scan(&t); err != nil {
		return time.Time{}, fmt.Errorf("last sweep query: %w", err)
	}
	if t == nil {
		return time.Time{}, nil
	}
	return t.UTC(), nil
}

// KindCount is the per-kind ready-state breakdown for the summary endpoint.
type KindCount struct {
	Kind      string
	Total     int
	Ready     int
	NotReady  int
	Unknown   int
	Suspended int
}

const summaryStmt = `
SELECT kind,
       count(*),
       count(*) FILTER (WHERE ready = 'True'),
       count(*) FILTER (WHERE ready = 'False'),
       count(*) FILTER (WHERE ready NOT IN ('True', 'False')),
       count(*) FILTER (WHERE suspended)
FROM flux_resource
GROUP BY kind
ORDER BY kind`

// Summary returns per-kind counts by ready state plus a suspended count.
func (s *Store) Summary(ctx context.Context) ([]KindCount, error) {
	rows, err := s.pool.Query(ctx, summaryStmt)
	if err != nil {
		return nil, fmt.Errorf("summary query: %w", err)
	}
	defer rows.Close()

	out := []KindCount{}
	for rows.Next() {
		var c KindCount
		if err := rows.Scan(&c.Kind, &c.Total, &c.Ready, &c.NotReady, &c.Unknown, &c.Suspended); err != nil {
			return nil, fmt.Errorf("scan summary: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const listStmt = `
SELECT kind, api_version, namespace, name, ready, reason, message, revision,
       suspended, generation, observed_generation, last_transition,
       COALESCE(azure_resource_id, ''), parent_kind, parent_name,
       applied_by_name, applied_by_namespace
FROM flux_resource
WHERE ($1 = '' OR lower(kind) = lower($1))
  AND ($2 = '' OR namespace = $2)
  AND ($3 = '' OR lower(ready) = lower($3))
ORDER BY kind, namespace, name`

// List returns normalized rows (without the raw payload) filtered by the
// optional kind/namespace/ready arguments; an empty argument means "any".
func (s *Store) List(ctx context.Context, kind, namespace, ready string) ([]flux.Resource, error) {
	rows, err := s.pool.Query(ctx, listStmt, kind, namespace, ready)
	if err != nil {
		return nil, fmt.Errorf("list query: %w", err)
	}
	defer rows.Close()

	out := []flux.Resource{}
	for rows.Next() {
		var r flux.Resource
		var parentKind, parentName *string
		var appliedByName, appliedByNamespace *string
		if err := rows.Scan(
			&r.Kind, &r.APIVersion, &r.Namespace, &r.Name, &r.Ready, &r.Reason,
			&r.Message, &r.Revision, &r.Suspended, &r.Generation,
			&r.ObservedGeneration, &r.LastTransition,
			&r.AzureResourceID, &parentKind, &parentName,
			&appliedByName, &appliedByNamespace,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		r.Parent = parentRef(parentKind, parentName)
		r.AppliedBy = appliedByRef(appliedByName, appliedByNamespace)
		out = append(out, r)
	}
	return out, rows.Err()
}

const getStmt = `
SELECT kind, api_version, namespace, name, ready, reason, message, revision,
       suspended, generation, observed_generation, last_transition, raw,
       COALESCE(azure_resource_id, ''), parent_kind, parent_name,
       applied_by_name, applied_by_namespace
FROM flux_resource
WHERE lower(kind) = lower($1) AND namespace = $2 AND name = $3`

const historyStmt = `
SELECT ready, COALESCE(reason, ''), COALESCE(revision, ''), observed_at
FROM flux_status_event
WHERE kind = $1 AND namespace = $2 AND name = $3
ORDER BY observed_at DESC
LIMIT $4`

// Event is one recorded status transition for a resource.
type Event struct {
	Ready      string    `json:"ready"`
	Reason     string    `json:"reason,omitempty"`
	Revision   string    `json:"revision,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

// Get returns one resource (including its raw payload) plus its recent status
// history, newest first. It returns ErrNotFound when no row matches.
func (s *Store) Get(ctx context.Context, kind, namespace, name string) (*flux.Resource, []Event, error) {
	var r flux.Resource
	var raw []byte
	var parentKind, parentName *string
	var appliedByName, appliedByNamespace *string
	err := s.pool.QueryRow(ctx, getStmt, kind, namespace, name).Scan(
		&r.Kind, &r.APIVersion, &r.Namespace, &r.Name, &r.Ready, &r.Reason,
		&r.Message, &r.Revision, &r.Suspended, &r.Generation,
		&r.ObservedGeneration, &r.LastTransition, &raw,
		&r.AzureResourceID, &parentKind, &parentName,
		&appliedByName, &appliedByNamespace,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get query: %w", err)
	}
	r.Raw = raw
	r.Parent = parentRef(parentKind, parentName)
	r.AppliedBy = appliedByRef(appliedByName, appliedByNamespace)

	rows, err := s.pool.Query(ctx, historyStmt, r.Kind, r.Namespace, r.Name, historyLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("history query: %w", err)
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Ready, &e.Reason, &e.Revision, &e.ObservedAt); err != nil {
			return nil, nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return &r, events, nil
}

// ResourceKey identifies a stored resource. The server uses the full key set to
// reconcile a tenant's current resources against the central mirror (pruning
// the ones that disappeared).
type ResourceKey struct {
	Kind      string
	Namespace string
	Name      string
}

// ChangedResource is a resource row plus its updated_at, so the server can
// advance its per-cluster sync cursor to the newest row it pulled.
type ChangedResource struct {
	flux.Resource
	UpdatedAt time.Time
}

const changedSinceStmt = `
SELECT kind, api_version, namespace, name, ready, reason, message, revision,
       suspended, generation, observed_generation, last_transition, raw, content_hash, updated_at,
       COALESCE(azure_resource_id, ''), parent_kind, parent_name,
       applied_by_name, applied_by_namespace
FROM flux_resource
WHERE updated_at > $1
ORDER BY updated_at`

// changedSinceStmtV2 is the pull for tenants still at schema version 2, whose
// databases predate the applied-by columns; that field stays nil.
const changedSinceStmtV2 = `
SELECT kind, api_version, namespace, name, ready, reason, message, revision,
       suspended, generation, observed_generation, last_transition, raw, content_hash, updated_at,
       COALESCE(azure_resource_id, ''), parent_kind, parent_name
FROM flux_resource
WHERE updated_at > $1
ORDER BY updated_at`

// ChangedSince returns the resources whose content changed after cursor (the
// server's per-cluster high-water), including the raw payload. Because the
// agent only advances updated_at on a real content change, an idle cluster
// returns no rows and transfers no payloads.
//
// schemaVersion is the tenant's meta.schema_version: the SELECT is keyed on it
// so the server can pull tenants whose agents have not migrated to the current
// schema yet (the server rolls out first, then the agents). Versions outside
// the schemaSupported window are the caller's job to skip.
func (s *Store) ChangedSince(ctx context.Context, cursor time.Time, schemaVersion int) ([]ChangedResource, error) {
	stmt := changedSinceStmt
	withAppliedBy := schemaVersion >= 3
	if !withAppliedBy {
		stmt = changedSinceStmtV2
	}
	rows, err := s.pool.Query(ctx, stmt, cursor)
	if err != nil {
		return nil, fmt.Errorf("changed-since query: %w", err)
	}
	defer rows.Close()

	out := []ChangedResource{}
	for rows.Next() {
		var c ChangedResource
		var raw []byte
		var hash *string
		var parentKind, parentName *string
		var appliedByName, appliedByNamespace *string
		dest := []any{
			&c.Kind, &c.APIVersion, &c.Namespace, &c.Name, &c.Ready, &c.Reason,
			&c.Message, &c.Revision, &c.Suspended, &c.Generation,
			&c.ObservedGeneration, &c.LastTransition, &raw, &hash, &c.UpdatedAt,
			&c.AzureResourceID, &parentKind, &parentName,
		}
		if withAppliedBy {
			dest = append(dest, &appliedByName, &appliedByNamespace)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan changed row: %w", err)
		}
		c.Raw = raw
		if hash != nil {
			c.ContentHash = *hash
		}
		c.Parent = parentRef(parentKind, parentName)
		c.AppliedBy = appliedByRef(appliedByName, appliedByNamespace)
		out = append(out, c)
	}
	return out, rows.Err()
}

const keysStmt = `SELECT kind, namespace, name FROM flux_resource`

// Keys returns the identity of every resource currently stored, so the server
// can prune central rows for resources that no longer exist in the tenant.
func (s *Store) Keys(ctx context.Context) ([]ResourceKey, error) {
	rows, err := s.pool.Query(ctx, keysStmt)
	if err != nil {
		return nil, fmt.Errorf("keys query: %w", err)
	}
	defer rows.Close()

	out := []ResourceKey{}
	for rows.Next() {
		var k ResourceKey
		if err := rows.Scan(&k.Kind, &k.Namespace, &k.Name); err != nil {
			return nil, fmt.Errorf("scan key: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// HistoryEvent is one status transition the server copies from a tenant into
// the central event log. ID is the tenant-local flux_status_event id, used as
// the server's per-cluster copy high-water.
type HistoryEvent struct {
	ID         int64
	Kind       string
	Namespace  string
	Name       string
	Ready      string
	Reason     string
	Message    string
	Revision   string
	ObservedAt time.Time
}

const eventsSinceStmt = `
SELECT id, kind, namespace, name, ready,
       COALESCE(reason, ''), COALESCE(message, ''), COALESCE(revision, ''), observed_at
FROM flux_status_event
WHERE id > $1
ORDER BY id`

// EventsSince returns status events with a tenant id greater than cursorID
// (the server's per-cluster event high-water), oldest first. The agent writes
// events one sweep at a time, so ids land in commit order with no visibility
// gaps — a high-water id cursor never skips an event the way an updated_at
// cursor could.
func (s *Store) EventsSince(ctx context.Context, cursorID int64) ([]HistoryEvent, error) {
	rows, err := s.pool.Query(ctx, eventsSinceStmt, cursorID)
	if err != nil {
		return nil, fmt.Errorf("events-since query: %w", err)
	}
	defer rows.Close()

	out := []HistoryEvent{}
	for rows.Next() {
		var e HistoryEvent
		if err := rows.Scan(&e.ID, &e.Kind, &e.Namespace, &e.Name, &e.Ready,
			&e.Reason, &e.Message, &e.Revision, &e.ObservedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
