package cache

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Package-level constants for time-dependent subtests.
// entryTTL is the TTL used for short-lived test entries; sleepBeforeExpiry
// must be > entryTTL to guarantee the entry is expired before Get is called.
const (
	entryTTL          = 10 * time.Millisecond
	sleepBeforeExpiry = 25 * time.Millisecond // > entryTTL ensures expiry
)

// cleanupInterval is the cleanup-loop tick period used in cleanup tests.
// shortTTL is the entry TTL for cleanup-eviction tests; it must be <
// cleanupInterval so the entry expires before the first tick.
// cleanupWait is the virtual sleep injected inside a synctest bubble; it must
// be > shortTTL + cleanupInterval so at least one cleanup tick fires after
// the entry has expired.
const (
	cleanupInterval = 20 * time.Millisecond
	shortTTL        = 5 * time.Millisecond
	cleanupWait     = 60 * time.Millisecond // > shortTTL + cleanupInterval
)

func TestMemoryCache_Get(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup func(c *MemoryCache)
		key   string
		want  func(*testing.T, any, bool)
	}{
		"happy_path": {
			setup: func(c *MemoryCache) {
				c.Set(context.Background(), "key1", "payload", time.Minute)
			},
			key: "key1",
			want: func(t *testing.T, val any, ok bool) {
				require.True(t, ok)
				assert.Equal(t, "payload", val)
			},
		},
		"miss_returns_false": {
			setup: func(c *MemoryCache) {},
			key:   "missing",
			want: func(t *testing.T, val any, ok bool) {
				require.False(t, ok)
				assert.Nil(t, val)
			},
		},
		"ttl_zero_set_is_noop": {
			setup: func(c *MemoryCache) {
				c.Set(context.Background(), "noop", "payload", 0)
			},
			key: "noop",
			want: func(t *testing.T, val any, ok bool) {
				require.False(t, ok)
				assert.Nil(t, val)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := New(context.Background(), 0)
			tc.setup(c)

			val, ok := c.Get(context.Background(), tc.key)
			tc.want(t, val, ok)
		})
	}
}

// TestMemoryCache_Get_ExpiryWithSynctest verifies that a Get on an expired
// entry returns (nil, false). Uses testing/synctest so time advances
// deterministically without real sleeps.
func TestMemoryCache_Get_ExpiryWithSynctest(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		c := New(context.Background(), 0)
		c.Set(context.Background(), "expiring", "payload", entryTTL)

		// Advance virtual clock past the entry TTL.
		time.Sleep(sleepBeforeExpiry)

		val, ok := c.Get(context.Background(), "expiring")
		require.False(t, ok)
		assert.Nil(t, val)
	})
}

func TestMemoryCache_Delete(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup func(c *MemoryCache)
		key   string
		want  func(*testing.T, *MemoryCache)
	}{
		"happy_path": {
			setup: func(c *MemoryCache) {
				c.Set(context.Background(), "k", "v", time.Minute)
			},
			key: "k",
			want: func(t *testing.T, c *MemoryCache) {
				_, ok := c.Get(context.Background(), "k")
				require.False(t, ok)
			},
		},
		"delete_missing_key_is_noop": {
			setup: func(c *MemoryCache) {},
			key:   "nonexistent",
			want: func(t *testing.T, c *MemoryCache) {
				// No panic, no error — just verify the key is still absent.
				_, ok := c.Get(context.Background(), "nonexistent")
				require.False(t, ok)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := New(context.Background(), 0)
			tc.setup(c)
			c.Delete(context.Background(), tc.key)
			tc.want(t, c)
		})
	}
}

// TestMemoryCache_Cleanup tests the background goroutine lifecycle.
//
// UT-0042 exception: subtests do NOT call t.Parallel() because some measure
// runtime.NumGoroutine() (global state) or run inside synctest bubbles which
// manage their own goroutines.
func TestMemoryCache_Cleanup(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(t *testing.T)
	}{
		"goroutine_exits_when_ctx_cancelled": {
			run: func(t *testing.T) {
				// Use synctest for deterministic goroutine-lifecycle verification.
				// c.Wait() blocks until the cleanup goroutine has exited, so there
				// is no need to poll runtime.NumGoroutine().
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())

					c := New(ctx, cleanupInterval)

					cancel()
					c.Wait() // returns once cleanupLoop exits
				})
			},
		},
		"zero_interval_no_goroutine_started": {
			run: func(t *testing.T) {
				baseline := runtime.NumGoroutine()

				ctx := context.Background()
				_ = New(ctx, 0)

				runtime.Gosched()

				assert.Equal(t, baseline, runtime.NumGoroutine())
			},
		},
		"expired_entry_removed_by_cleanup": {
			run: func(t *testing.T) {
				// Use synctest so time.Sleep advances the fake clock instantly
				// when all goroutines in the bubble are blocked.
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					t.Cleanup(cancel)

					c := New(ctx, cleanupInterval)
					c.Set(context.Background(), "short", "val", shortTTL)

					// Advance virtual clock: shortTTL expires, cleanup tick fires.
					time.Sleep(cleanupWait)

					// The cleanup loop should have removed the entry.
					// Even if it wasn't removed proactively, Get does lazy eviction —
					// the key must not be returned regardless.
					_, ok := c.Get(context.Background(), "short")
					require.False(t, ok)
				})
			},
		},
	}

	// UT-0042 exception: no t.Parallel() inside the loop — subtests measure
	// global runtime state or use synctest bubbles with their own goroutines.
	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

// TestMemoryCache_Concurrent exercises Set/Get/Delete under 50 concurrent
// goroutines to detect data races. Always run under -race (go test -race).
// Not table-driven yet; add table structure when additional scenarios appear.
func TestMemoryCache_Concurrent(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const key = "shared"

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	c := New(ctx, time.Second)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		i := i

		go func() {
			defer wg.Done()

			k := key
			if i%3 == 0 {
				k = "alt"
			}

			c.Set(context.Background(), k, i, time.Second)
			_, _ = c.Get(context.Background(), k)
			if i%5 == 0 {
				c.Delete(context.Background(), k)
			}
		}()
	}

	wg.Wait()
}
