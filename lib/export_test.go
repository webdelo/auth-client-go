package authclient

// This file exposes internal SDK internals to the external test package
// (package authclient_test) to break the import cycle:
//   authclient_test → lib/mocks → lib → (cycle if internal test)
//
// By keeping tests in authclient_test and using these helpers, we avoid
// the cycle while retaining access to unexported fields.

import (
	"context"
	"time"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"google.golang.org/grpc"
)

// FakeAuthClient is an exported test double for the unexported authServiceClient
// interface. It is defined here (package authclient) so external test packages
// can reference it without triggering an import cycle via lib/mocks.
type FakeAuthClient struct {
	ValidateFunc  func(ctx context.Context, in *auth_v1.ValidateTokenRequest, opts ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error)
	ValidateCalls int
}

// ValidateToken implements authServiceClient.
func (f *FakeAuthClient) ValidateToken(ctx context.Context, in *auth_v1.ValidateTokenRequest, opts ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
	f.ValidateCalls++
	return f.ValidateFunc(ctx, in, opts...)
}

// ClientConfig groups the parameters used by NewTestClient.
type ClientConfig struct {
	ValidateCacheTTL time.Duration
	ValidateTimeout  time.Duration
	FailMode         FailMode
	OpenMethods      map[string]struct{}
}

// NewTestClient constructs a Client with injected dependencies, bypassing the
// gRPC dial that New performs. Used exclusively by external test packages.
func NewTestClient(cfg ClientConfig, cache Cache, auth *FakeAuthClient, logger Logger) *Client {
	if logger == nil {
		logger = NoopLogger{}
	}
	openMethods := defaultOpenMethods()
	for m := range cfg.OpenMethods {
		openMethods[m] = struct{}{}
	}
	return &Client{
		cfg: Config{
			ValidateCacheTTL: cfg.ValidateCacheTTL,
			ValidateTimeout:  cfg.ValidateTimeout,
		},
		cache:       cache,
		authClient:  auth,
		logger:      logger,
		failMode:    cfg.FailMode,
		openMethods: openMethods,
	}
}

// HashToken is the exported alias of hashToken for use in external test packages.
func HashToken(token string) string { return hashToken(token) }

// DefaultOpenMethods exposes the default open methods map for external tests.
func DefaultOpenMethods() map[string]struct{} { return defaultOpenMethods() }

// BearerToken exposes the unexported bearerToken function for external tests.
func BearerToken(authz string) (string, bool) { return bearerToken(authz) }
