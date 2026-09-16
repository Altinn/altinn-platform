// Package sweepcache remembers a fingerprint of every swept object in a
// Valkey cache, so the agent can skip the full database upsert for objects
// that did not change since the last sweep. The cache is an optimization
// only: without it, or when it fails, the agent upserts everything.
//
// The fingerprint must cover everything a row is built from: the object's
// own content hash and the fields the sweep derives from other objects.
// Today that is the applying Kustomization or HelmRelease (AppliedBy).
package sweepcache

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

const (
	// ttl bounds how long a remembered fingerprint lives. Every object gets
	// one full upsert per day at most, which also refreshes the database
	// projections for rows whose object did not change.
	ttl = 24 * time.Hour
	// dialTimeout and connectTimeout keep a dead cache from delaying a sweep.
	// The agent has its own fallback, so the client does not retry either.
	dialTimeout    = 2 * time.Second
	connectTimeout = 5 * time.Second
)

// Config holds the connection settings. An empty Address disables the cache.
type Config struct {
	Address  string
	Username string
	Password string
}

// Client is a fingerprint memory backed by Valkey. A nil *Client is valid
// and remembers nothing, so callers need no nil checks.
type Client struct {
	client valkey.Client
	prefix string
}

// New connects to the cache. It returns nil, nil when cfg.Address is empty.
// The prefix separates entries of different agent builds, when the build
// stamps a version: after an upgrade every object misses once, which
// refreshes every row.
func New(ctx context.Context, cfg Config, prefix string) (*Client, error) {
	if cfg.Address == "" {
		return nil, nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{cfg.Address},
		Username:    cfg.Username,
		Password:    cfg.Password,
		Dialer:      net.Dialer{Timeout: dialTimeout},
		// Client-side caching is not used; this skips its handshake and
		// its per-connection memory.
		DisableCache: true,
		DisableRetry: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", cfg.Address, err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := client.Do(pingCtx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping %s: %w", cfg.Address, err)
	}

	return &Client{client: client, prefix: prefix}, nil
}

// Close releases the connection. Safe on a nil Client.
func (c *Client) Close() {
	if c != nil {
		c.client.Close()
	}
}

// key names the cache entry of a swept object.
func key(prefix string, r *flux.Resource) string {
	return prefix + ":" + r.Kind + "/" + r.Namespace + "/" + r.Name
}

// fingerprint is the value remembered per object.
func fingerprint(r *flux.Resource) string {
	appliedBy := ""
	if r.AppliedBy != nil {
		appliedBy = r.AppliedBy.Namespace + "/" + r.AppliedBy.Name
	}

	return r.ContentHash + "|" + appliedBy
}

// Hashes returns the remembered fingerprint per cache key for the given
// objects. Objects with no entry are absent from the map. A nil Client
// returns an empty map.
func (c *Client) Hashes(ctx context.Context, resources []flux.Resource) (map[string]string, error) {
	hashes := map[string]string{}
	if c == nil || len(resources) == 0 {
		return hashes, nil
	}
	keys := make([]string, len(resources))
	for i := range resources {
		keys[i] = key(c.prefix, &resources[i])
	}
	// MGet groups the keys per cluster slot, so it also works on a Valkey
	// cluster with many shards.
	found, err := valkey.MGet(c.client, ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("read fingerprints: %w", err)
	}
	for k, msg := range found {
		if msg.IsNil() {
			continue
		}
		value, err := msg.ToString()
		if err != nil {
			return nil, fmt.Errorf("read fingerprint %s: %w", k, err)
		}
		hashes[k] = value
	}

	return hashes, nil
}

// Remember stores the fingerprint of the given objects. A nil Client does
// nothing.
func (c *Client) Remember(ctx context.Context, resources []flux.Resource) error {
	if c == nil || len(resources) == 0 {
		return nil
	}
	cmds := make(valkey.Commands, 0, len(resources))
	for i := range resources {
		r := &resources[i]
		cmds = append(cmds, c.client.B().Set().Key(key(c.prefix, r)).Value(fingerprint(r)).Ex(ttl).Build())
	}
	for _, result := range c.client.DoMulti(ctx, cmds...) {
		if err := result.Error(); err != nil {
			return fmt.Errorf("write fingerprints: %w", err)
		}
	}

	return nil
}

// Split separates the swept objects into the ones whose remembered
// fingerprint equals the current one (unchanged) and all others (changed).
// Objects with no entry count as changed. A nil Client returns every object
// as changed.
func (c *Client) Split(resources []flux.Resource, known map[string]string) (changed, unchanged []flux.Resource) {
	if c == nil {
		return resources, nil
	}
	for i := range resources {
		r := &resources[i]
		if value, ok := known[key(c.prefix, r)]; ok && value == fingerprint(r) {
			unchanged = append(unchanged, *r)
			continue
		}
		changed = append(changed, *r)
	}

	return changed, unchanged
}
