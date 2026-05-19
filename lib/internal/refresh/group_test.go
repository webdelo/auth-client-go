package refresh

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroup_Do(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(*testing.T, *Group)
	}{
		"happy_path": {
			run: func(t *testing.T, g *Group) {
				calls := 0
				v, err := g.Do("key", func() (any, error) {
					calls++
					return "result", nil
				})
				require.NoError(t, err)
				assert.Equal(t, "result", v)
				assert.Equal(t, 1, calls, "fn must run exactly once")
			},
		},
		"different_keys_independent": {
			run: func(t *testing.T, g *Group) {
				var callCount atomic.Int32
				fn := func() (any, error) {
					callCount.Add(1)
					return "result", nil
				}

				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					_, _ = g.Do("key-a", fn)
				}()
				go func() {
					defer wg.Done()
					_, _ = g.Do("key-b", fn)
				}()
				wg.Wait()

				assert.EqualValues(t, 2, callCount.Load(), "fn must run once per distinct key")
			},
		},
		"key_forgotten_next_call_reruns": {
			run: func(t *testing.T, g *Group) {
				calls := 0
				fn := func() (any, error) {
					calls++
					return calls, nil
				}

				// First call: fn runs, key is forgotten after Do returns.
				v1, err1 := g.Do("k", fn)
				require.NoError(t, err1)
				assert.Equal(t, 1, v1)

				// Second call with same key: fn must run again (key was forgotten by first Do).
				v2, err2 := g.Do("k", fn)
				require.NoError(t, err2)
				assert.Equal(t, 2, v2)

				assert.Equal(t, 2, calls, "fn must run twice — Forget guarantees subsequent calls re-execute fn")
			},
		},
		"fn_error_propagates": {
			run: func(t *testing.T, g *Group) {
				const n = 5

				sentinel := errors.New("boom")
				started := make(chan struct{}, 1)
				release := make(chan struct{})

				fn := func() (any, error) {
					select {
					case started <- struct{}{}:
					default:
					}
					<-release
					return nil, sentinel
				}

				results := make([]any, n)
				errs := make([]error, n)

				var wg sync.WaitGroup
				wg.Add(n)
				for i := 0; i < n; i++ {
					i := i
					go func() {
						defer wg.Done()
						results[i], errs[i] = g.Do("err-key", fn)
					}()
				}

				<-started
				time.Sleep(50 * time.Millisecond)
				close(release)

				wg.Wait()

				for i := 0; i < n; i++ {
					assert.Nil(t, results[i], "goroutine %d must receive nil value on error", i)
					require.Error(t, errs[i], "goroutine %d must receive an error", i)
					assert.True(t, errors.Is(errs[i], sentinel), "goroutine %d error must wrap sentinel", i)
				}
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var g Group
			tc.run(t, &g)
		})
	}
}

// TestGroup_Do_ConcurrentDedup verifies that N concurrent callers sharing the
// same key result in exactly one fn invocation and all receive the shared result.
//
// UT-0002 exception: rendezvous channels and N=10 goroutines do not fit a flat
// table row — this test requires dedicated goroutine orchestration that cannot
// be expressed as a simple (args, want) tuple alongside sequential cases.
func TestGroup_Do_ConcurrentDedup(t *testing.T) {
	t.Parallel()

	const n = 10
	var g Group
	var callCount atomic.Int32

	started := make(chan struct{}, 1)
	release := make(chan struct{})

	fn := func() (any, error) {
		callCount.Add(1)
		// Signal leader entry (non-blocking so only first goroutine signals).
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return "shared", nil
	}

	results := make([]any, n)
	errs := make([]error, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i], errs[i] = g.Do("same-key", fn)
		}()
	}

	// Wait for at least one goroutine to enter fn, then give followers time
	// to converge on the same-key call before releasing.
	<-started
	time.Sleep(50 * time.Millisecond)
	close(release)

	wg.Wait()

	assert.EqualValues(t, 1, callCount.Load(), "fn must run exactly once for concurrent callers with same key")
	for i := 0; i < n; i++ {
		require.NoError(t, errs[i], "goroutine %d must receive no error", i)
		assert.Equal(t, "shared", results[i], "goroutine %d must receive shared result", i)
	}
}
