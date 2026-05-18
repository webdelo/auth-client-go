// Package authclient is the official Go SDK for the webdelo/auth service.
//
// The SDK is a thin wrapper over the gRPC contract from
// github.com/webdelo/auth-proto with non-trivial concerns baked in: a
// ValidateToken TTL cache, an HTTP middleware, a gRPC unary interceptor,
// refresh-race serialization, and typed errors. Create a Client with New,
// dispose with Close.
package authclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	cachepkg "github.com/webdelo/auth-client-go/lib/internal/cache"
	"github.com/webdelo/auth-client-go/lib/internal/refresh"
)

const (
	defaultValidateCacheTTL = 30 * time.Second
	defaultValidateTimeout  = 2 * time.Second
	defaultLoginTimeout     = 5 * time.Second
	defaultRefreshTimeout   = 5 * time.Second
	defaultDialOptionsCount = 3

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
// implementation is created by New when Config.Cache is nil and WithCache
// is not applied; supply a custom implementation via WithCache to override
// TTL semantics or storage backend.
//
// On a cancelled ctx, implementations should return the zero value
// (Get) or be a no-op (Set/Delete) immediately rather than blocking.
// Implementations must not use ctx to bound the TTL of stored entries —
// ttl is the sole TTL source.
type Cache interface {
	// Get returns the cached User and ok=true on hit.
	Get(ctx context.Context, key string) (user User, ok bool)
	// Set stores user with the given TTL. ttl <= 0 must be a no-op.
	Set(ctx context.Context, key string, user User, ttl time.Duration)
	// Delete removes the key. Idempotent.
	Delete(ctx context.Context, key string)
}

// Config bundles the constructor parameters. Endpoint is the only required
// field; all others have sensible defaults applied by New.
type Config struct {
	// Endpoint is the AUTH service address ("host:port"). Required.
	Endpoint string

	// ValidateCacheTTL is the TTL for ValidateToken results. Defaults to 30s
	// when negative. Zero means "cache disabled": New still allocates an
	// in-memory cache object, but Set is a no-op for TTL=0 so no entries are
	// ever stored. Effectively every ValidateAccessToken call reaches gRPC.
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
	// to override transport credentials (TLS), interceptors, etc.
	// Caller-supplied options come after SDK defaults so they win.
	//
	// SECURITY: the SDK default transport is insecure.NewCredentials()
	// (plaintext). Production deployments to a remote AUTH server MUST
	// supply grpc.WithTransportCredentials(...) here to enable TLS.
	// Caller-supplied options come after SDK defaults so they win.
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

// WithCache sets a custom ValidateToken result cache, replacing the SDK
// default MemoryCache. Resolution priority inside New: WithCache option
// (this function) > Config.Cache field > built-in MemoryCache with a
// cleanup goroutine bound to Client.Close. Passing nil here is a no-op
// for the option; the resolution then falls through to Config.Cache or
// the built-in default. Custom caches do NOT receive a background cleanup
// goroutine — the implementation is fully responsible for its own lifecycle.
func WithCache(cache Cache) Option {
	return func(c *Client) { c.cache = cache }
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

// authServiceClient is the consumer-side mockable subset of
// auth_v1.AuthServiceClient. The generated *auth_v1.AuthServiceClient stub
// satisfies this interface implicitly because Go interfaces are structurally
// typed. New methods are added here as future tasks require them (Login,
// Refresh, etc.).
type authServiceClient interface {
	ValidateToken(
		ctx context.Context,
		in *auth_v1.ValidateTokenRequest,
		opts ...grpc.CallOption,
	) (*auth_v1.ValidateTokenResponse, error)
	Login(ctx context.Context, in *auth_v1.LoginRequest, opts ...grpc.CallOption) (*auth_v1.LoginResponse, error)
	RefreshToken(ctx context.Context, in *auth_v1.RefreshTokenRequest, opts ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error)
	Logout(ctx context.Context, in *auth_v1.LogoutRequest, opts ...grpc.CallOption) (*auth_v1.LogoutResponse, error)
	LogoutAll(ctx context.Context, in *auth_v1.LogoutAllRequest, opts ...grpc.CallOption) (*auth_v1.LogoutAllResponse, error)
	ChangePassword(ctx context.Context, in *auth_v1.ChangePasswordRequest, opts ...grpc.CallOption) (*auth_v1.ChangePasswordResponse, error)
	GetCurrentUser(ctx context.Context, in *auth_v1.GetCurrentUserRequest, opts ...grpc.CallOption) (*auth_v1.GetCurrentUserResponse, error)
}

// Client is the SDK entry point. Create with New, dispose with Close.
// Safe for concurrent use by multiple goroutines (the underlying
// *grpc.ClientConn is goroutine-safe per gRPC concurrency guarantees).
type Client struct {
	cfg         Config
	grpcConn    *grpc.ClientConn
	authClient  authServiceClient
	cache       Cache                 // token validation cache (default = cacheAdapter wrapping cache.MemoryCache)
	memCache    *cachepkg.MemoryCache // non-nil only when SDK created the default MemoryCache
	logger      Logger
	failMode    FailMode
	openMethods map[string]struct{}

	// sf deduplicates concurrent Refresh calls with the same refresh token
	// (singleflight). Zero-value ready; created in New.
	sf *refresh.Group

	// bgCtx is the owner-lifetime context for background goroutines (cache
	// cleaner, etc.). It is NOT a request context. Cancelled by Close.
	// Storing this lifecycle context in the struct is intentional — it
	// scopes to Client itself, not to a single inbound request.
	bgCtx    context.Context
	cancelBG context.CancelFunc

	closeOnce sync.Once
	closeErr  error
}

// isLocalEndpoint reports whether the endpoint points to the local machine,
// used as a heuristic to suppress the insecure-transport warning.
func isLocalEndpoint(endpoint string) bool {
	return strings.HasPrefix(endpoint, "localhost") ||
		strings.HasPrefix(endpoint, "127.") ||
		strings.HasPrefix(endpoint, "[::1]") ||
		strings.HasPrefix(endpoint, "unix:")
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

	if !isLocalEndpoint(cfg.Endpoint) && len(cfg.DialOptions) == 0 {
		cfg.Logger.Warn(context.Background(),
			"authclient: insecure (plaintext) transport for non-local endpoint — supply WithTransportCredentials(TLS) in DialOptions for production",
			"endpoint", cfg.Endpoint,
		)
	}

	dialOpts := make([]grpc.DialOption, 0, defaultDialOptionsCount+len(cfg.DialOptions))
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
		sf:          &refresh.Group{},
		bgCtx:       bgCtx,
		cancelBG:    cancelBG,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Resolve cache: WithCache option > Config.Cache > default MemoryCache.
	if c.cache == nil && cfg.Cache != nil {
		c.cache = cfg.Cache
	}

	if c.cache == nil {
		cleanupInterval := cfg.ValidateCacheTTL / 2
		mc := cachepkg.New(bgCtx, cleanupInterval)
		c.memCache = mc
		c.cache = newCacheAdapter(mc)
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
		if c.memCache != nil {
			c.memCache.Wait()
		}
		c.closeErr = c.grpcConn.Close()
	})
	return c.closeErr
}

// cacheAdapter wraps *cache.MemoryCache to satisfy the Cache interface
// without forcing the internal cache package to depend on authclient.User
// (which would create an import cycle).
type cacheAdapter struct {
	inner *cachepkg.MemoryCache
}

// newCacheAdapter constructs a cacheAdapter around an existing MemoryCache.
func newCacheAdapter(inner *cachepkg.MemoryCache) *cacheAdapter {
	return &cacheAdapter{inner: inner}
}

// Get implements Cache. Type-asserts the opaque payload back to User.
// If the assertion fails (should not happen — only this adapter writes
// to the underlying MemoryCache), it is treated as a miss.
func (a *cacheAdapter) Get(ctx context.Context, key string) (User, bool) {
	v, ok := a.inner.Get(ctx, key)
	if !ok {
		return User{}, false
	}

	user, ok := v.(User)
	if !ok {
		return User{}, false
	}

	return user, true
}

// Set implements Cache.
func (a *cacheAdapter) Set(ctx context.Context, key string, user User, ttl time.Duration) {
	a.inner.Set(ctx, key, user, ttl)
}

// Delete implements Cache.
func (a *cacheAdapter) Delete(ctx context.Context, key string) {
	a.inner.Delete(ctx, key)
}

// Compile-time assertion that cacheAdapter implements Cache.
var _ Cache = (*cacheAdapter)(nil)
