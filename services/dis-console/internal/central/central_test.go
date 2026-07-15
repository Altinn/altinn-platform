package central

import (
	"context"
	"testing"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
)

func TestClusterID(t *testing.T) {
	if got := clusterID("dis_console_ttd_at23"); got != "ttd_at23" {
		t.Fatalf("clusterID: got %q", got)
	}
	// No prefix: returned unchanged.
	if got := clusterID("weird_name"); got != "weird_name" {
		t.Fatalf("clusterID without prefix: got %q", got)
	}
}

func TestEnvironmentOf(t *testing.T) {
	cases := map[string]string{
		"ttd_at23": "at23",
		"skd_tt02": "tt02",
		"prod":     "", // no underscore
		"a_":       "", // trailing underscore, no segment
	}
	for in, want := range cases {
		if got := environmentOf(in); got != want {
			t.Fatalf("environmentOf(%q): got %q, want %q", in, got, want)
		}
	}
}

func TestSchemaSupported(t *testing.T) {
	// Supports the current version and the previous one; flags the rest.
	if !schemaSupported(store.SchemaVersion) {
		t.Fatalf("current schema_version %d should be supported", store.SchemaVersion)
	}
	if !schemaSupported(store.SchemaVersion - 1) {
		t.Fatalf("previous schema_version %d should be supported", store.SchemaVersion-1)
	}
	if schemaSupported(store.SchemaVersion + 1) {
		t.Fatalf("newer schema_version %d must not be claimed supported", store.SchemaVersion+1)
	}
	if schemaSupported(store.SchemaVersion - 2) {
		t.Fatalf("older schema_version %d must not be claimed supported", store.SchemaVersion-2)
	}
}

func TestAdvanceCursor(t *testing.T) {
	old := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)

	// No changed rows: cursor must not move.
	if got := advanceCursor(old, nil); !got.Equal(old) {
		t.Fatalf("empty changed should keep cursor, got %v", got)
	}

	newer := old.Add(2 * time.Minute)
	older := old.Add(-time.Minute)
	changed := []store.ChangedResource{
		{UpdatedAt: older},
		{UpdatedAt: newer},
		{UpdatedAt: old.Add(time.Minute)},
	}
	if got := advanceCursor(old, changed); !got.Equal(newer) {
		t.Fatalf("advanceCursor should pick the newest updated_at %v, got %v", newer, got)
	}

	// A batch all older than the cursor (shouldn't happen via the query, but the
	// function must never regress) keeps the old cursor.
	if got := advanceCursor(old, []store.ChangedResource{{UpdatedAt: older}}); !got.Equal(old) {
		t.Fatalf("advanceCursor must not regress, got %v", got)
	}
}

func TestAdvanceEventCursor(t *testing.T) {
	// No events: cursor must not move.
	if got := advanceEventCursor(7, nil); got != 7 {
		t.Fatalf("empty events should keep cursor, got %d", got)
	}
	events := []store.HistoryEvent{{ID: 3}, {ID: 9}, {ID: 5}}
	if got := advanceEventCursor(7, events); got != 9 {
		t.Fatalf("advanceEventCursor should pick the largest id 9, got %d", got)
	}
	// All ids at/below the cursor: must never regress.
	if got := advanceEventCursor(20, events); got != 20 {
		t.Fatalf("advanceEventCursor must not regress, got %d", got)
	}
}

func TestPurgeExpiredEventsDisabled(t *testing.T) {
	// retention <= 0 disables the purge entirely: the method must return
	// before running any SQL — the nil pool would panic if it did.
	s := New(nil)
	for _, d := range []time.Duration{0, -time.Hour} {
		n, err := s.PurgeExpiredEvents(context.Background(), d)
		if err != nil || n != 0 {
			t.Fatalf("PurgeExpiredEvents(%s): n=%d err=%v, want a no-op", d, n, err)
		}
	}
}

func TestStaleSince(t *testing.T) {
	d := time.Minute
	if !staleSince(time.Time{}, d) {
		t.Fatalf("zero time (never synced) should be stale")
	}
	if staleSince(time.Now(), d) {
		t.Fatalf("a just-now timestamp should not be stale")
	}
	if !staleSince(time.Now().Add(-2*time.Minute), d) {
		t.Fatalf("2m old should be stale with a 1m threshold")
	}
}
