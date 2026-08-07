// Package dedup remembers which events have already been dispatched so a
// re-emitted Flux event for an unchanged artifact does not trigger a second
// workflow run. State is in-memory only: on pod restart the worst case is one
// extra dispatch per environment, which is harmless for idempotent workflows.
package dedup

import (
	"container/list"
	"context"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// entry is one remembered dispatch, held in insertion order so the oldest can
// be evicted in O(1) when the tracker hits its capacity.
type entry struct {
	key        string
	recordedAt time.Time
}

// Tracker is a bounded, TTL'd set of dedup keys. It is safe for concurrent use.
type Tracker struct {
	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List // front = oldest, back = newest

	ttl        time.Duration
	maxEntries int
	gauge      prometheus.Gauge

	// now is injectable so TTL behaviour can be tested without sleeping.
	now func() time.Time
}

// New returns a Tracker holding at most maxEntries keys for ttl each. The gauge
// (may be nil) is kept in sync with the number of tracked entries.
func New(ttl time.Duration, maxEntries int, gauge prometheus.Gauge) *Tracker {
	return &Tracker{
		entries:    make(map[string]*list.Element),
		order:      list.New(),
		ttl:        ttl,
		maxEntries: maxEntries,
		gauge:      gauge,
		now:        time.Now,
	}
}

// Key builds the dedup key for an event. Including reason keeps a failure and a
// success for the same digest distinct, and including repo keeps a
// Kustomization that dispatches to two repos from deduplicating the second.
func (t *Tracker) Key(product, env, reason, digest, repo string) string {
	return strings.Join([]string{product, env, reason, digest, repo}, "/")
}

// Seen reports whether key was recorded and has not yet expired. It does not
// mutate the tracker, so an expired-but-unswept entry still reads as unseen.
func (t *Tracker) Seen(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	element, ok := t.entries[key]
	if !ok {
		return false
	}
	return !t.expired(element.Value.(*entry), t.now())
}

// Record marks key as dispatched, evicting the oldest entry when the tracker is
// at capacity.
func (t *Tracker) Record(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()

	if element, ok := t.entries[key]; ok {
		// Refresh in place and move to the back: this key is the newest again.
		element.Value.(*entry).recordedAt = now
		t.order.MoveToBack(element)
		t.setGauge()
		return
	}

	for t.maxEntries > 0 && t.order.Len() >= t.maxEntries {
		t.removeElement(t.order.Front())
	}

	t.entries[key] = t.order.PushBack(&entry{key: key, recordedAt: now})
	t.setGauge()
}

// StartEviction runs a background sweep every `every` until ctx is done,
// dropping entries older than the TTL.
func (t *Tracker) StartEviction(ctx context.Context, every time.Duration) {
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.evictExpired()
			}
		}
	}()
}

// evictExpired drops every entry older than the TTL. Entries sit in insertion
// order, so the sweep can stop at the first entry that is still fresh.
func (t *Tracker) evictExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	for element := t.order.Front(); element != nil; element = t.order.Front() {
		if !t.expired(element.Value.(*entry), now) {
			break
		}
		t.removeElement(element)
	}
	t.setGauge()
}

// expired reports whether e is older than the TTL relative to now.
func (t *Tracker) expired(e *entry, now time.Time) bool {
	return now.Sub(e.recordedAt) >= t.ttl
}

// removeElement drops one list element and its map index. Caller holds the lock.
func (t *Tracker) removeElement(element *list.Element) {
	if element == nil {
		return
	}
	delete(t.entries, element.Value.(*entry).key)
	t.order.Remove(element)
}

// setGauge publishes the current entry count. Caller holds the lock.
func (t *Tracker) setGauge() {
	if t.gauge != nil {
		t.gauge.Set(float64(t.order.Len()))
	}
}
