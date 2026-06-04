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

// SyncStats reports what a Sync did.
type SyncStats struct {
	Upserted int   // rows seen this sweep
	Changed  int   // rows whose ready/reason/revision changed (history events written)
	Pruned   int64 // rows deleted because their object disappeared from the cluster
}

// upsertStmt upserts one resource and, in the same statement, writes a history
// event when the row is new or its ready/reason/revision changed. The `prev`
// CTE reads the pre-upsert row (CTEs see the snapshot from the start of the
// statement), so the comparison is against the previously stored values. The
// command tag reflects the trailing INSERT, so RowsAffected() is 1 exactly when
// a history event was written.
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
        generation, observed_generation, last_transition, raw,
        first_seen, last_seen, updated_at
    ) VALUES (
        $1, $2, $3, $4,
        $5, $6, $7, $8, $9,
        $10, $11, $12, $13,
        now(), now(), now()
    )
    ON CONFLICT (kind, namespace, name) DO UPDATE SET
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
        last_seen           = now(),
        updated_at          = CASE
            WHEN r.ready    IS DISTINCT FROM EXCLUDED.ready
              OR r.reason   IS DISTINCT FROM EXCLUDED.reason
              OR r.revision IS DISTINCT FROM EXCLUDED.revision
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
			batch.Queue(upsertStmt,
				r.Kind, r.APIVersion, r.Namespace, r.Name,
				r.Ready, r.Reason, r.Message, r.Revision, r.Suspended,
				r.Generation, r.ObservedGeneration, r.LastTransition, []byte(raw),
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

	if err := tx.Commit(ctx); err != nil {
		return stats, fmt.Errorf("commit: %w", err)
	}
	return stats, nil
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
       suspended, generation, observed_generation, last_transition
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
		if err := rows.Scan(
			&r.Kind, &r.APIVersion, &r.Namespace, &r.Name, &r.Ready, &r.Reason,
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
SELECT kind, api_version, namespace, name, ready, reason, message, revision,
       suspended, generation, observed_generation, last_transition, raw
FROM flux_resource
WHERE lower(kind) = lower($1) AND namespace = $2 AND name = $3`

const historyStmt = `
SELECT ready, reason, revision, observed_at
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
	err := s.pool.QueryRow(ctx, getStmt, kind, namespace, name).Scan(
		&r.Kind, &r.APIVersion, &r.Namespace, &r.Name, &r.Ready, &r.Reason,
		&r.Message, &r.Revision, &r.Suspended, &r.Generation,
		&r.ObservedGeneration, &r.LastTransition, &raw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get query: %w", err)
	}
	r.Raw = raw

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
