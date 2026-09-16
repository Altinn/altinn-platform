// Package sweepcache remembers the content hash of every swept object in a
// Valkey cache, so the agent can skip the full database upsert for objects
// that did not change since the last sweep. The cache is an optimization
// only: without it, or when it fails, the agent upserts everything.
package sweepcache

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

// ttl bounds how long a remembered hash lives. Every object gets one full
// upsert per day at most, which also keeps the database projections fresh.
const ttl = 24 * time.Hour

// Config holds the connection settings. An empty Address disables the cache.
type Config struct {
	Address  string
	Username string
	Password string
}

// Client is a hash memory backed by Valkey. A nil *Client is valid and
// remembers nothing, so callers need no nil checks.
type Client struct {
	client valkey.Client
	prefix string
}

// New connects to the cache. It returns nil, nil when cfg.Address is empty.
// The prefix separates entries of different agent builds: after an upgrade
// every object misses once, which backfills projections the new build added.
func New(ctx context.Context, cfg Config, prefix string) (*Client, error) {
	if cfg.Address == "" {
		return nil, nil
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{cfg.Address},
		Username:     cfg.Username,
		Password:     cfg.Password,
		DisableCache: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", cfg.Address, err)
	}
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
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

// Key names the cache entry of a swept object.
func Key(prefix string, r *flux.Resource) string {
	return prefix + ":" + r.Kind + "/" + r.Namespace + "/" + r.Name
}

// Hashes returns the remembered content hash per cache key for the given
// objects. Objects with no entry are absent from the map. A nil Client
// returns an empty map.
func (c *Client) Hashes(ctx context.Context, resources []flux.Resource) (map[string]string, error) {
	hashes := map[string]string{}
	if c == nil || len(resources) == 0 {
		return hashes, nil
	}
	keys := make([]string, len(resources))
	for i := range resources {
		keys[i] = Key(c.prefix, &resources[i])
	}
	// MGet groups the keys per cluster slot, so it also works on a Valkey
	// cluster with many shards.
	found, err := valkey.MGet(c.client, ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("read hashes: %w", err)
	}
	for key, msg := range found {
		if msg.IsNil() {
			continue
		}
		hash, err := msg.ToString()
		if err != nil {
			return nil, fmt.Errorf("read hash %s: %w", key, err)
		}
		hashes[key] = hash
	}

	return hashes, nil
}

// Remember stores the content hash of the given objects. A nil Client does
// nothing.
func (c *Client) Remember(ctx context.Context, resources []flux.Resource) error {
	if c == nil || len(resources) == 0 {
		return nil
	}
	cmds := make(valkey.Commands, 0, len(resources))
	for i := range resources {
		r := &resources[i]
		cmds = append(cmds, c.client.B().Set().Key(Key(c.prefix, r)).Value(r.ContentHash).ExSeconds(int64(ttl.Seconds())).Build())
	}
	for _, result := range c.client.DoMulti(ctx, cmds...) {
		if err := result.Error(); err != nil {
			return fmt.Errorf("write hashes: %w", err)
		}
	}

	return nil
}

// Split separates the swept objects into the ones whose remembered hash
// equals the current one (unchanged) and all others (changed). Objects with
// no entry count as changed.
func Split(prefix string, resources []flux.Resource, known map[string]string) (changed, unchanged []flux.Resource) {
	for i := range resources {
		r := &resources[i]
		if hash, ok := known[Key(prefix, r)]; ok && hash != "" && hash == r.ContentHash {
			unchanged = append(unchanged, *r)
			continue
		}
		changed = append(changed, *r)
	}

	return changed, unchanged
}
