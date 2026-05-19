package authclient

// This file exposes internal SDK internals to the external test package
// (package authclient_test) to break the import cycle:
//   authclient_test → lib/mocks → lib → (cycle if internal test)
//
// By keeping tests in authclient_test and using these helpers, we avoid
// the cycle while retaining access to unexported fields.

import (
	"context"
	"sync/atomic"
	"time"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"google.golang.org/grpc"

	"github.com/webdelo/auth-client-go/lib/internal/refresh"
)

// FakeAuthClient is a hand-written test double for the unexported
// authServiceClient interface. The interface is unexported, so a
// mockery-generated mock would need to be defined inside package authclient
// (which then collides with the lib/mocks → lib import-cycle). This file
// (export_test.go) is the canonical home for cross-package test helpers
// requiring access to unexported types.
//
// UT-0020 waiver: hand-written double justified by the unexported-interface
// constraint above. Reconsider if authServiceClient is ever exported.
type FakeAuthClient struct {
	ValidateFunc  func(ctx context.Context, in *auth_v1.ValidateTokenRequest, opts ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error)
	ValidateCalls int

	LoginFunc  func(ctx context.Context, in *auth_v1.LoginRequest, opts ...grpc.CallOption) (*auth_v1.LoginResponse, error)
	LoginCalls int

	// RefreshTokenCalls uses atomic.Int32 because TestClient_Refresh_SingleFlight
	// invokes RefreshToken from multiple goroutines and asserts .Load() == 1.
	RefreshTokenFunc  func(ctx context.Context, in *auth_v1.RefreshTokenRequest, opts ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error)
	RefreshTokenCalls atomic.Int32

	LogoutFunc  func(ctx context.Context, in *auth_v1.LogoutRequest, opts ...grpc.CallOption) (*auth_v1.LogoutResponse, error)
	LogoutCalls int

	LogoutAllFunc  func(ctx context.Context, in *auth_v1.LogoutAllRequest, opts ...grpc.CallOption) (*auth_v1.LogoutAllResponse, error)
	LogoutAllCalls int

	ChangePasswordFunc  func(ctx context.Context, in *auth_v1.ChangePasswordRequest, opts ...grpc.CallOption) (*auth_v1.ChangePasswordResponse, error)
	ChangePasswordCalls int

	GetCurrentUserFunc  func(ctx context.Context, in *auth_v1.GetCurrentUserRequest, opts ...grpc.CallOption) (*auth_v1.GetCurrentUserResponse, error)
	GetCurrentUserCalls int
}

// ValidateToken implements authServiceClient.
func (f *FakeAuthClient) ValidateToken(ctx context.Context, in *auth_v1.ValidateTokenRequest, opts ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
	f.ValidateCalls++
	return f.ValidateFunc(ctx, in, opts...)
}

// Login implements authServiceClient.
func (f *FakeAuthClient) Login(ctx context.Context, in *auth_v1.LoginRequest, opts ...grpc.CallOption) (*auth_v1.LoginResponse, error) {
	f.LoginCalls++
	return f.LoginFunc(ctx, in, opts...)
}

// RefreshToken implements authServiceClient.
func (f *FakeAuthClient) RefreshToken(ctx context.Context, in *auth_v1.RefreshTokenRequest, opts ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
	f.RefreshTokenCalls.Add(1)
	return f.RefreshTokenFunc(ctx, in, opts...)
}

// Logout implements authServiceClient.
func (f *FakeAuthClient) Logout(ctx context.Context, in *auth_v1.LogoutRequest, opts ...grpc.CallOption) (*auth_v1.LogoutResponse, error) {
	f.LogoutCalls++
	return f.LogoutFunc(ctx, in, opts...)
}

// LogoutAll implements authServiceClient.
func (f *FakeAuthClient) LogoutAll(ctx context.Context, in *auth_v1.LogoutAllRequest, opts ...grpc.CallOption) (*auth_v1.LogoutAllResponse, error) {
	f.LogoutAllCalls++
	return f.LogoutAllFunc(ctx, in, opts...)
}

// ChangePassword implements authServiceClient.
func (f *FakeAuthClient) ChangePassword(ctx context.Context, in *auth_v1.ChangePasswordRequest, opts ...grpc.CallOption) (*auth_v1.ChangePasswordResponse, error) {
	f.ChangePasswordCalls++
	return f.ChangePasswordFunc(ctx, in, opts...)
}

// GetCurrentUser implements authServiceClient.
func (f *FakeAuthClient) GetCurrentUser(ctx context.Context, in *auth_v1.GetCurrentUserRequest, opts ...grpc.CallOption) (*auth_v1.GetCurrentUserResponse, error) {
	f.GetCurrentUserCalls++
	return f.GetCurrentUserFunc(ctx, in, opts...)
}

// ClientConfig groups the parameters used by NewTestClient.
type ClientConfig struct {
	ValidateCacheTTL time.Duration
	ValidateTimeout  time.Duration
	LoginTimeout     time.Duration
	RefreshTimeout   time.Duration
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
			LoginTimeout:     cfg.LoginTimeout,
			RefreshTimeout:   cfg.RefreshTimeout,
		},
		cache:       cache,
		authClient:  auth,
		logger:      logger,
		failMode:    cfg.FailMode,
		openMethods: openMethods,
		sf:          &refresh.Group{},
	}
}

// HashToken is the exported alias of hashToken for use in external test packages.
func HashToken(token string) string { return hashToken(token) }

// DefaultOpenMethods exposes the default open methods map for external tests.
func DefaultOpenMethods() map[string]struct{} { return defaultOpenMethods() }

// BearerToken exposes the unexported bearerToken function for external tests.
func BearerToken(authz string) (string, bool) { return bearerToken(authz) }
