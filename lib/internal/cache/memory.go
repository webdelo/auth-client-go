// Package cache contains internal SDK cache implementations. Public API of
// the SDK references the Cache interface in package authclient, not these
// types directly.
package cache

import (
	"context"
	"sync"
	"time"
)

// entry is the stored payload — caller-supplied value with absolute expiry.
type entry struct {
	user      any
	expiresAt time.Time
}

// MemoryCache is a TTL cache backed by sync.Map.
//
// Keys are caller-supplied (the SDK uses hex(sha256(access_token))). Values
// are opaque (any) — the wrapping adapter in package authclient type-asserts
// to authclient.User. Read access is lock-free via sync.Map. A background
// goroutine drains tombstones on cleanupInterval; it exits when the bgCtx
// passed to NewMemoryCache is cancelled, ensuring no goroutine leak.
type MemoryCache struct {
	m sync.Map // key string -> entry
}

// NewMemoryCache constructs a MemoryCache and, when cleanupInterval > 0,
// launches a background cleanup goroutine.
//
// bgCtx controls the cleanup goroutine lifecycle. Cancel it to stop the
// goroutine. Pass context.Background() for a permanent cache; pass the SDK
// Client's background context to bind the cleanup lifetime to Client.Close.
// A cleanupInterval of 0 or less skips the goroutine entirely — expired
// entries are still evicted lazily inside Get.
func NewMemoryCache(bgCtx context.Context, cleanupInterval time.Duration) *MemoryCache {
	c := &MemoryCache{}

	if cleanupInterval > 0 {
		go c.cleanupLoop(bgCtx, cleanupInterval)
	}

	return c
}

// Get returns the cached value and ok=true on cache hit. Lazily evicts the
// entry when it has expired.
func (c *MemoryCache) Get(_ context.Context, key string) (any, bool) {
	v, ok := c.m.Load(key)
	if !ok {
		return nil, false
	}

	e := v.(entry)
	if time.Now().After(e.expiresAt) {
		c.m.Delete(key)
		return nil, false
	}

	return e.user, true
}

// Set stores user under key with the given TTL. A TTL of 0 or less is a
// no-op, which preserves the "ValidateCacheTTL == 0 disables caching"
// contract at the entry level.
func (c *MemoryCache) Set(_ context.Context, key string, user any, ttl time.Duration) {
	if ttl <= 0 {
		return
	}

	c.m.Store(key, entry{user: user, expiresAt: time.Now().Add(ttl)})
}

// Delete removes the entry for key. Idempotent — safe to call when the key
// does not exist.
func (c *MemoryCache) Delete(_ context.Context, key string) {
	c.m.Delete(key)
}

// cleanupLoop scans the map once per interval and removes expired entries.
// Exits when ctx is cancelled.
func (c *MemoryCache) cleanupLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			c.m.Range(func(k, v any) bool {
				if now.After(v.(entry).expiresAt) {
					c.m.Delete(k)
				}
				return true
			})
		}
	}
}
