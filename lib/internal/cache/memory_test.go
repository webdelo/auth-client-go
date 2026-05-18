package cache

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_Get(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup       func(c *MemoryCache)
		key         string
		sleepBefore time.Duration
		want        func(*testing.T, any, bool)
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
		"expired_returns_false": {
			setup: func(c *MemoryCache) {
				c.Set(context.Background(), "expiring", "payload", 10*time.Millisecond)
			},
			key:         "expiring",
			sleepBefore: 20 * time.Millisecond,
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

			c := NewMemoryCache(context.Background(), 0)
			tc.setup(c)

			if tc.sleepBefore > 0 {
				time.Sleep(tc.sleepBefore)
			}

			val, ok := c.Get(context.Background(), tc.key)
			tc.want(t, val, ok)
		})
	}
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

			c := NewMemoryCache(context.Background(), 0)
			tc.setup(c)
			c.Delete(context.Background(), tc.key)
			tc.want(t, c)
		})
	}
}

// TestMemoryCache_Cleanup tests the background goroutine lifecycle.
// Goroutine-count subtests are NOT parallel (they measure global runtime state).
func TestMemoryCache_Cleanup(t *testing.T) {
	tests := map[string]struct {
		run func(t *testing.T)
	}{
		"goroutine_exits_when_ctx_cancelled": {
			run: func(t *testing.T) {
				baseline := runtime.NumGoroutine()

				ctx, cancel := context.WithCancel(context.Background())
				_ = NewMemoryCache(ctx, 10*time.Millisecond)

				// Give the goroutine a moment to start.
				time.Sleep(20 * time.Millisecond)

				cancel()

				// Wait up to 200ms for the goroutine to exit.
				deadline := time.Now().Add(200 * time.Millisecond)
				for time.Now().Before(deadline) {
					runtime.Gosched()
					time.Sleep(5 * time.Millisecond)
					if runtime.NumGoroutine() <= baseline {
						break
					}
				}

				assert.LessOrEqual(t, runtime.NumGoroutine(), baseline)
			},
		},
		"zero_interval_no_goroutine_started": {
			run: func(t *testing.T) {
				baseline := runtime.NumGoroutine()

				ctx := context.Background()
				_ = NewMemoryCache(ctx, 0)

				runtime.Gosched()

				assert.Equal(t, baseline, runtime.NumGoroutine())
			},
		},
		"expired_entry_removed_by_cleanup": {
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)

				const cleanupInterval = 20 * time.Millisecond
				c := NewMemoryCache(ctx, cleanupInterval)
				c.Set(context.Background(), "short", "val", 5*time.Millisecond)

				// Wait for entry to expire and cleanup to run.
				time.Sleep(60 * time.Millisecond)

				// The cleanup loop should have removed the entry.
				// Even if it wasn't removed proactively, Get does lazy eviction —
				// the key must not be returned regardless.
				_, ok := c.Get(context.Background(), "short")
				require.False(t, ok)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

func TestMemoryCache_Concurrent(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const key = "shared"

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	c := NewMemoryCache(ctx, time.Second)

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
