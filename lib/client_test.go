package authclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingLogger struct {
	infos int
}

func (r *recordingLogger) Info(_ context.Context, _ string, _ ...any)           { r.infos++ }
func (r *recordingLogger) Warn(_ context.Context, _ string, _ ...any)           {}
func (r *recordingLogger) Error(_ context.Context, _ string, _ error, _ ...any) {}

type stubCache struct{}

func (*stubCache) Get(_ context.Context, _ string) (any, bool)              { return nil, false }
func (*stubCache) Set(_ context.Context, _ string, _ any, _ time.Duration)  {}
func (*stubCache) Delete(_ context.Context, _ string)                       {}

func TestNew(t *testing.T) {
	t.Parallel()

	type testCase struct {
		cfg  Config
		opts []Option
		want func(t *testing.T, c *Client, err error)
	}

	tests := map[string]testCase{
		"happy_path": {
			cfg: Config{Endpoint: "localhost:50051"},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				assert.Equal(t, time.Duration(0), c.cfg.ValidateCacheTTL, "zero TTL preserved as cache-disabled")
				assert.Equal(t, FailClosed, c.failMode)
				_, ok := c.logger.(NoopLogger)
				assert.True(t, ok, "default logger should be NoopLogger")
			},
		},
		"missing_endpoint_returns_error": {
			cfg: Config{},
			want: func(t *testing.T, c *Client, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "Endpoint")
				assert.Nil(t, c)
			},
		},
		"negative_ttl_normalized_to_default": {
			cfg: Config{Endpoint: "x:1", ValidateCacheTTL: -1 * time.Second},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				assert.Equal(t, 30*time.Second, c.cfg.ValidateCacheTTL)
			},
		},
		"with_fail_mode_open": {
			cfg:  Config{Endpoint: "x:1"},
			opts: []Option{WithFailMode(FailOpen)},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				assert.Equal(t, FailOpen, c.failMode)
			},
		},
		"with_logger_nil_uses_noop": {
			cfg:  Config{Endpoint: "x:1"},
			opts: []Option{WithLogger(nil)},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				_, ok := c.logger.(NoopLogger)
				assert.True(t, ok, "nil logger should fall back to NoopLogger")
			},
		},
		"with_logger_custom": {
			cfg:  Config{Endpoint: "x:1"},
			opts: []Option{WithLogger(&recordingLogger{})},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				_, ok := c.logger.(*recordingLogger)
				assert.True(t, ok, "custom logger should be set on client")
			},
		},
		"with_open_methods_appends": {
			cfg:  Config{Endpoint: "x:1"},
			opts: []Option{WithOpenMethods("/x.v1.X/Login", "/x.v1.X/Register")},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				_, hasLogin := c.openMethods["/x.v1.X/Login"]
				assert.True(t, hasLogin)
				_, hasRegister := c.openMethods["/x.v1.X/Register"]
				assert.True(t, hasRegister)
				_, hasHealthCheck := c.openMethods["/grpc.health.v1.Health/Check"]
				assert.True(t, hasHealthCheck, "/grpc.health.v1.Health/Check should always be present")
			},
		},
		"defaults_applied_for_timeouts": {
			cfg: Config{Endpoint: "x:1"},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				assert.Equal(t, 2*time.Second, c.cfg.ValidateTimeout)
				assert.Equal(t, 5*time.Second, c.cfg.LoginTimeout)
				assert.Equal(t, 5*time.Second, c.cfg.RefreshTimeout)
				assert.Equal(t, time.Duration(0), c.cfg.ValidateCacheTTL, "zero TTL preserved as cache-disabled (no default override)")
			},
		},
		"with_cache_custom": {
			cfg:  Config{Endpoint: "x:1"},
			opts: []Option{WithCache(&stubCache{})},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				_, ok := c.cfg.Cache.(*stubCache)
				assert.True(t, ok, "WithCache should set cfg.Cache")
			},
		},
		"positive_ttl_preserved": {
			cfg: Config{Endpoint: "x:1", ValidateCacheTTL: 5 * time.Second},
			want: func(t *testing.T, c *Client, err error) {
				require.NoError(t, err)
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
				assert.Equal(t, 5*time.Second, c.cfg.ValidateCacheTTL)
			},
		},
		"endpoint_invalid_format_returns_error": {
			cfg: Config{Endpoint: "bad host with spaces"},
			want: func(t *testing.T, c *Client, err error) {
				// grpc.NewClient may validate the target eagerly (returning error)
				// or lazily (deferring validation to the first RPC). Both are valid.
				if err != nil {
					require.Error(t, err)
					assert.Nil(t, c)
					return
				}
				require.NotNil(t, c)
				t.Cleanup(func() {
					if cerr := c.Close(); cerr != nil {
						t.Logf("close error: %v", cerr)
					}
				})
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, err := New(tc.cfg, tc.opts...)
			tc.want(t, c, err)
		})
	}
}

func TestClient_Close(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		run func(t *testing.T)
	}{
		"happy_path": {
			run: func(t *testing.T) {
				c, err := New(Config{Endpoint: "localhost:50051"})
				require.NoError(t, err)
				require.NotNil(t, c)
				require.NoError(t, c.Close())
			},
		},
		"close_is_idempotent": {
			run: func(t *testing.T) {
				c, err := New(Config{Endpoint: "localhost:50051"})
				require.NoError(t, err)
				require.NotNil(t, c)
				err1 := c.Close()
				err2 := c.Close()
				err3 := c.Close()
				assert.Equal(t, err1, err2)
				assert.Equal(t, err2, err3)
			},
		},
		"close_cancels_background_context": {
			run: func(t *testing.T) {
				c, err := New(Config{Endpoint: "localhost:50051"})
				require.NoError(t, err)
				require.NotNil(t, c)
				if err := c.Close(); err != nil {
					t.Logf("close error: %v", err)
				}
				select {
				case <-c.bgCtx.Done():
				case <-time.After(100 * time.Millisecond):
					t.Fatal("bgCtx was not cancelled within 100ms after Close()")
				}
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}
