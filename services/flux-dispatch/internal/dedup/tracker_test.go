package dedup

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newTestGauge() prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flux_dispatch_dedup_entries",
		Help: "Current number of entries in the dedup tracker.",
	})
}

// clock is a manually advanced time source injected into the tracker so TTL
// behaviour can be tested without sleeping.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestTracker(t *testing.T, maxEntries int) (*Tracker, *clock, prometheus.Gauge) {
	t.Helper()
	gauge := newTestGauge()
	tr := New(time.Hour, maxEntries, gauge)
	c := &clock{now: time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)}
	tr.now = c.Now
	return tr, c, gauge
}

func TestKey(t *testing.T) {
	tr, _, _ := newTestTracker(t, 10)

	got := tr.Key("dialogporten", "at23", "ReconciliationSucceeded", "sha256:aabbccdd", "Altinn/dialogporten")
	want := "dialogporten/at23/ReconciliationSucceeded/sha256:aabbccdd/Altinn/dialogporten"
	if got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}

func TestSeenAfterRecord(t *testing.T) {
	tr, _, gauge := newTestTracker(t, 10)

	key := tr.Key("dialogporten", "at23", "ReconciliationSucceeded", "sha256:aabbccdd", "Altinn/dialogporten")
	if tr.Seen(key) {
		t.Fatal("Seen() = true before Record()")
	}
	if got := testutil.ToFloat64(gauge); got != 0 {
		t.Errorf("gauge = %v before Record, want 0", got)
	}

	tr.Record(key)

	if !tr.Seen(key) {
		t.Fatal("Seen() = false after Record()")
	}
	if got := testutil.ToFloat64(gauge); got != 1 {
		t.Errorf("gauge = %v after Record, want 1", got)
	}

	// Re-recording the same key must not grow the tracker.
	tr.Record(key)
	if got := testutil.ToFloat64(gauge); got != 1 {
		t.Errorf("gauge = %v after re-Record of same key, want 1", got)
	}
}

// TestKeyComponentsAreDistinguishing covers the RFC corner cases: a failure and
// a success for the same digest are distinct events, and the same Kustomization
// dispatching to two repos must not deduplicate the second dispatch.
func TestKeyComponentsAreDistinguishing(t *testing.T) {
	tr, _, _ := newTestTracker(t, 100)

	base := tr.Key("dialogporten", "at23", "ReconciliationSucceeded", "sha256:aabbccdd", "Altinn/dialogporten")
	tr.Record(base)

	variants := map[string]string{
		"different reason":  tr.Key("dialogporten", "at23", "ReconciliationFailed", "sha256:aabbccdd", "Altinn/dialogporten"),
		"different repo":    tr.Key("dialogporten", "at23", "ReconciliationSucceeded", "sha256:aabbccdd", "Altinn/other"),
		"different digest":  tr.Key("dialogporten", "at23", "ReconciliationSucceeded", "sha256:eeffgghh", "Altinn/dialogporten"),
		"different env":     tr.Key("dialogporten", "tt02", "ReconciliationSucceeded", "sha256:aabbccdd", "Altinn/dialogporten"),
		"different product": tr.Key("correspondence", "at23", "ReconciliationSucceeded", "sha256:aabbccdd", "Altinn/dialogporten"),
	}
	for name, key := range variants {
		if tr.Seen(key) {
			t.Errorf("%s: Seen() = true, want false", name)
		}
	}
}

func TestRecordEvictsOldestAtCapacity(t *testing.T) {
	tr, c, gauge := newTestTracker(t, 2)

	first := "a"
	second := "b"
	third := "c"

	tr.Record(first)
	c.Advance(time.Second)
	tr.Record(second)
	c.Advance(time.Second)

	if got := testutil.ToFloat64(gauge); got != 2 {
		t.Fatalf("gauge = %v at capacity, want 2", got)
	}

	tr.Record(third)

	if tr.Seen(first) {
		t.Error("oldest entry was not evicted at capacity")
	}
	if !tr.Seen(second) {
		t.Error("second entry was evicted, want retained")
	}
	if !tr.Seen(third) {
		t.Error("newest entry was not recorded")
	}
	if got := testutil.ToFloat64(gauge); got != 2 {
		t.Errorf("gauge = %v after eviction, want 2", got)
	}
}

func TestExpiredEntriesAreNotSeen(t *testing.T) {
	tr, c, _ := newTestTracker(t, 10)

	tr.Record("key")
	c.Advance(30 * time.Minute)
	if !tr.Seen("key") {
		t.Fatal("Seen() = false within TTL")
	}

	c.Advance(31 * time.Minute)
	if tr.Seen("key") {
		t.Error("Seen() = true past TTL")
	}
}

func TestEvictExpiredSweepsAndUpdatesGauge(t *testing.T) {
	tr, c, gauge := newTestTracker(t, 10)

	tr.Record("old")
	c.Advance(90 * time.Minute)
	tr.Record("fresh")

	if got := testutil.ToFloat64(gauge); got != 2 {
		t.Fatalf("gauge = %v before sweep, want 2", got)
	}

	tr.evictExpired()

	if got := testutil.ToFloat64(gauge); got != 1 {
		t.Errorf("gauge = %v after sweep, want 1", got)
	}
	if tr.Seen("old") {
		t.Error("expired entry survived the sweep")
	}
	if !tr.Seen("fresh") {
		t.Error("fresh entry was swept")
	}
}

func TestStartEvictionSweepsInBackground(t *testing.T) {
	tr, c, gauge := newTestTracker(t, 10)

	tr.Record("old")
	c.Advance(90 * time.Minute)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tr.StartEviction(ctx, time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for testutil.ToFloat64(gauge) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("background eviction did not run: gauge = %v", testutil.ToFloat64(gauge))
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStartEvictionStopsOnContextCancel(t *testing.T) {
	tr, c, gauge := newTestTracker(t, 10)

	ctx, cancel := context.WithCancel(context.Background())
	tr.StartEviction(ctx, time.Millisecond)
	cancel()
	// Give a still-running sweeper time to tick many times before checking.
	time.Sleep(20 * time.Millisecond)

	tr.Record("old")
	c.Advance(90 * time.Minute)
	time.Sleep(50 * time.Millisecond)

	if got := testutil.ToFloat64(gauge); got != 1 {
		t.Errorf("gauge = %v after context cancel, want 1 (sweeper should have stopped)", got)
	}
}

func TestConcurrentSeenAndRecord(t *testing.T) {
	tr, _, _ := newTestTracker(t, 64)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tr.StartEviction(ctx, time.Millisecond)

	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range 200 {
				key := tr.Key("p", "at23", "ReconciliationSucceeded",
					fmt.Sprintf("sha256:%d-%d", worker, i), "Altinn/repo")
				if !tr.Seen(key) {
					tr.Record(key)
				}
				_ = tr.Seen(key)
			}
		}(worker)
	}
	wg.Wait()
}

// TestClaimBlocksConcurrentDuplicate covers the reservation that closes the
// window between the dedup lookup and the record.
func TestClaimBlocksConcurrentDuplicate(t *testing.T) {
	tracker := New(time.Hour, 100, nil)

	if !tracker.Claim("k") {
		t.Fatal("first Claim = false, want true")
	}
	if tracker.Claim("k") {
		t.Error("second Claim = true, want false while the first is in flight")
	}
	if !tracker.Seen("k") {
		t.Error("Seen = false, want true for a claimed key")
	}
}

// TestReleaseAllowsRetry covers a dispatch that failed: the claim must not
// suppress the redelivery.
func TestReleaseAllowsRetry(t *testing.T) {
	tracker := New(time.Hour, 100, nil)

	if !tracker.Claim("k") {
		t.Fatal("Claim = false, want true")
	}
	tracker.Release("k")

	if tracker.Seen("k") {
		t.Error("Seen = true after Release, want false")
	}
	if !tracker.Claim("k") {
		t.Error("Claim after Release = false, want true")
	}
}

// TestReleaseAfterRecordIsNoop guards the confirmed case: once a dispatch has
// succeeded, a stray Release must not un-deduplicate it.
func TestReleaseAfterRecordIsNoop(t *testing.T) {
	tracker := New(time.Hour, 100, nil)

	tracker.Claim("k")
	tracker.Record("k")
	tracker.Release("k")

	if !tracker.Seen("k") {
		t.Error("Seen = false after Record, want true: Release must not drop a confirmed entry")
	}
}

// TestClaimTakesOverExpiredEntry covers an entry past its TTL that the sweeper
// has not reached yet.
func TestClaimTakesOverExpiredEntry(t *testing.T) {
	now := time.Now()
	tracker := New(time.Hour, 100, nil)
	tracker.now = func() time.Time { return now }

	tracker.Record("k")
	now = now.Add(2 * time.Hour)

	if !tracker.Claim("k") {
		t.Error("Claim = false on an expired entry, want true")
	}
}

// TestConcurrentClaimYieldsSingleWinner is the property the handler depends on:
// exactly one caller may own a key at a time.
func TestConcurrentClaimYieldsSingleWinner(t *testing.T) {
	tracker := New(time.Hour, 1000, nil)

	const goroutines = 64
	var wg sync.WaitGroup
	var winners atomic.Int64

	start := make(chan struct{})
	for range goroutines {
		wg.Go(func() {
			<-start
			if tracker.Claim("contended") {
				winners.Add(1)
			}
		})
	}
	close(start)
	wg.Wait()

	if got := winners.Load(); got != 1 {
		t.Errorf("winners = %d, want exactly 1", got)
	}
}
