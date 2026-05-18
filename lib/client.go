package authclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const (
	defaultValidateCacheTTL = 30 * time.Second
	defaultValidateTimeout  = 2 * time.Second
	defaultLoginTimeout     = 5 * time.Second
	defaultRefreshTimeout   = 5 * time.Second

	keepaliveTime    = 30 * time.Second
	keepaliveTimeout = 10 * time.Second
)

// retryServiceConfig configures gRPC retry policy ONLY for ValidateToken.
// RefreshToken is intentionally excluded — retrying a refresh that succeeded
// at AUTH but lost the response would trigger family detection and burn the
// user's refresh chain. maxAttempts=4 means one initial attempt plus three
// retries with exponential backoff 100ms→1s.
const retryServiceConfig = `{
  "methodConfig": [{
    "name": [{"service": "auth.v1.AuthService", "method": "ValidateToken"}],
    "retryPolicy": {
      "maxAttempts": 4,
      "initialBackoff": "0.1s",
      "maxBackoff": "1s",
      "backoffMultiplier": 2,
      "retryableStatusCodes": ["UNAVAILABLE"]
    }
  }]
}`

// FailMode controls how the HTTP middleware and gRPC unary interceptor react
// when the AUTH service is unavailable. See WithFailMode.
type FailMode uint8

const (
	// FailClosed (default) blocks the request with 503 if AUTH is unreachable.
	// This is the OWASP-recommended behavior (A07 Authentication Failures).
	FailClosed FailMode = iota

	// FailOpen passes the request through with an empty User in context.
	// Use ONLY for non-critical endpoints (health checks, public probes).
	// SECURITY WARNING: misuse leads to auth bypass.
	FailOpen
)

// Cache is the interface for ValidateToken result caching. The default
// implementation is created by New when Config.Cache is nil; supply a custom
// implementation via WithCache to override TTL or storage backend.
type Cache interface {
	// Get returns the cached User and ok=true on hit.
	Get(ctx context.Context, key string) (user any, ok bool)
	// Set stores user with the given TTL.
	Set(ctx context.Context, key string, user any, ttl time.Duration)
	// Delete removes the key.
	Delete(ctx context.Context, key string)
}

// Config bundles the constructor parameters. Endpoint is the only required
// field; all others have sensible defaults applied by New.
type Config struct {
	// Endpoint is the AUTH service address ("host:port"). Required.
	Endpoint string

	// ValidateCacheTTL is the TTL for ValidateToken results. Defaults to 30s
	// when negative. Zero means "cache disabled" and is preserved as-is by
	// New; the cache layer skips creation when TTL is zero.
	ValidateCacheTTL time.Duration

	// FailMode controls middleware behavior when AUTH is unreachable.
	// Defaults to FailClosed (block the request with 503).
	FailMode FailMode

	// ValidateTimeout is the per-call timeout for ValidateToken. Defaults to 2s.
	ValidateTimeout time.Duration

	// LoginTimeout is the per-call timeout for Login. Defaults to 5s.
	LoginTimeout time.Duration

	// RefreshTimeout is the per-call timeout for Refresh. Defaults to 5s.
	RefreshTimeout time.Duration

	// DialOptions are appended to the SDK defaults, allowing the consumer
	// to override transport credentials (TLS), interceptors, etc. The SDK
	// insecure.NewCredentials() comes first; user options come after, so a
	// user-supplied WithTransportCredentials wins.
	DialOptions []grpc.DialOption

	// Logger is the diagnostic logger. Defaults to NoopLogger if nil.
	Logger Logger

	// Cache is the ValidateToken result cache. Defaults to nil (no cache).
	Cache Cache
}

// Option is a functional option for New.
type Option func(*Client)

// WithFailMode overrides Config.FailMode.
//
// SECURITY WARNING: WithFailMode(FailOpen) disables auth on AUTH-unavailable.
// Use only for endpoints where this is acceptable.
func WithFailMode(mode FailMode) Option {
	return func(c *Client) { c.failMode = mode }
}

// WithLogger overrides Config.Logger. nil collapses to NoopLogger{}.
func WithLogger(l Logger) Option {
	return func(c *Client) {
		if l == nil {
			l = NoopLogger{}
		}
		c.logger = l
	}
}

// WithCache overrides the ValidateToken result cache. Pass nil to disable caching.
func WithCache(cache Cache) Option {
	return func(c *Client) { c.cfg.Cache = cache }
}

// WithOpenMethods registers gRPC methods (FullMethod, e.g.
// "/myapp.v1.UserService/Login") that bypass the authentication interceptor.
// The health-check endpoints /grpc.health.v1.Health/Check and /Watch are
// always in the open set.
func WithOpenMethods(methods ...string) Option {
	return func(c *Client) {
		for _, m := range methods {
			c.openMethods[m] = struct{}{}
		}
	}
}

// Client is the SDK entry point. Create with New, dispose with Close.
// Safe for concurrent use by multiple goroutines (the underlying
// *grpc.ClientConn is goroutine-safe per gRPC concurrency guarantees).
type Client struct {
	cfg         Config
	grpcConn    *grpc.ClientConn
	authClient  auth_v1.AuthServiceClient
	logger      Logger
	failMode    FailMode
	openMethods map[string]struct{}

	// bgCtx is the owner-lifetime context for background goroutines (cache
	// cleaner, etc.). It is NOT a request context. Cancelled by Close.
	// Storing it in the struct is intentional — this is a lifecycle context,
	// not a per-request context (EC-0051 exception).
	bgCtx    context.Context
	cancelBG context.CancelFunc

	closeOnce sync.Once
	closeErr  error
}

// New creates a Client. It validates Endpoint, applies defaults, sets up the
// gRPC connection (lazy — does NOT block on dial), and registers the default
// open methods for the interceptor. The returned Client must be closed with
// Close to release the gRPC connection and stop background goroutines.
func New(cfg Config, opts ...Option) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("authclient: Config.Endpoint is required")
	}

	cfg = applyDefaults(cfg)

	dialOpts := make([]grpc.DialOption, 0, 3+len(cfg.DialOptions))
	dialOpts = append(dialOpts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,
			Timeout:             keepaliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultServiceConfig(retryServiceConfig),
	)
	dialOpts = append(dialOpts, cfg.DialOptions...)

	conn, err := grpc.NewClient(cfg.Endpoint, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("authclient: grpc dial %q: %w", cfg.Endpoint, err)
	}

	bgCtx, cancelBG := newBackgroundContext()

	c := &Client{
		cfg:         cfg,
		grpcConn:    conn,
		authClient:  auth_v1.NewAuthServiceClient(conn),
		logger:      cfg.Logger,
		failMode:    cfg.FailMode,
		openMethods: defaultOpenMethods(),
		bgCtx:       bgCtx,
		cancelBG:    cancelBG,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// newBackgroundContext creates the cancellable background context whose
// CancelFunc is owned by Client and invoked by Close. The helper exists to
// document that the cancel ownership transfers to Client — the cancellation
// guarantee lives in Close, not in the caller of this helper.
func newBackgroundContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// applyDefaults fills in zero-value Config fields with SDK defaults.
// Negative timeouts are treated as "use default" since negative values have
// no sensible interpretation. ValidateCacheTTL is special: negative defaults
// to 30s, but zero is preserved (means "cache disabled").
func applyDefaults(cfg Config) Config {
	if cfg.ValidateCacheTTL < 0 {
		cfg.ValidateCacheTTL = defaultValidateCacheTTL
	}
	if cfg.ValidateTimeout <= 0 {
		cfg.ValidateTimeout = defaultValidateTimeout
	}
	if cfg.LoginTimeout <= 0 {
		cfg.LoginTimeout = defaultLoginTimeout
	}
	if cfg.RefreshTimeout <= 0 {
		cfg.RefreshTimeout = defaultRefreshTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = NoopLogger{}
	}
	return cfg
}

// defaultOpenMethods is the always-open registry: health checks bypass auth.
func defaultOpenMethods() map[string]struct{} {
	return map[string]struct{}{
		"/grpc.health.v1.Health/Check": {},
		"/grpc.health.v1.Health/Watch": {},
	}
}

// Close shuts down background goroutines and closes the underlying gRPC
// connection. Idempotent — subsequent calls return the same error from the
// first call. Should be called via `defer c.Close()` after a successful New.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		c.cancelBG()
		c.closeErr = c.grpcConn.Close()
	})
	return c.closeErr
}
