package authclient_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authclient "github.com/webdelo/auth-client-go/lib"
	mocks "github.com/webdelo/auth-client-go/lib/mocks"
)

type interceptorTestMocks struct {
	cache  *mocks.MockCache
	auth   *authclient.FakeAuthClient
	logger *mocks.MockLogger
}

func newInterceptorTestMocks(t *testing.T) *interceptorTestMocks {
	t.Helper()
	return &interceptorTestMocks{
		cache:  mocks.NewMockCache(t),
		auth:   &authclient.FakeAuthClient{},
		logger: mocks.NewMockLogger(t),
	}
}

func TestClient_UnaryInterceptor(t *testing.T) {
	t.Parallel()

	const (
		cacheTTL        = 30 * time.Second
		validateTimeout = time.Second
		healthMethod    = "/grpc.health.v1.Health/Check"
		appMethod       = "/myapp.Service/Method"
	)

	makeCtxWithBearer := func(token string) context.Context {
		md := metadata.Pairs("authorization", "Bearer "+token)
		return metadata.NewIncomingContext(context.Background(), md)
	}

	makeInfo := func(method string) *grpc.UnaryServerInfo {
		return &grpc.UnaryServerInfo{FullMethod: method}
	}

	noopHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	tests := map[string]struct {
		ctx          func() context.Context
		info         *grpc.UnaryServerInfo
		handler      grpc.UnaryHandler
		failMode     authclient.FailMode
		extraMethods map[string]struct{}
		prepareMocks func(*testing.T, *interceptorTestMocks)
		want         func(*testing.T, interface{}, error)
	}{
		"happy_path": {
			ctx:      func() context.Context { return makeCtxWithBearer("valid-tok") },
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("valid-tok")
				expectedUser := authclient.User{UUID: "u1", Role: authclient.RoleUser}

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:  true,
						UserUuid: "u1",
						Role:     "user",
					}, nil
				}

				m.cache.EXPECT().
					Set(
						mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
						expectedKey,
						expectedUser,
						cacheTTL,
					).
					Return().
					Once()
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.NoError(t, err)
				assert.Equal(t, "ok", resp)
			},
		},
		"open_method_skips_auth": {
			ctx:      func() context.Context { return context.Background() },
			info:     makeInfo(healthMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(_ *testing.T, _ *interceptorTestMocks) {
				// Health check is open — cache and auth must NOT be called.
				// fakeAuthClient.ValidateFunc is nil; if called it will panic.
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.NoError(t, err)
				assert.Equal(t, "ok", resp)
			},
		},
		"with_open_methods_custom_method_skips_auth": {
			ctx:      func() context.Context { return context.Background() },
			info:     makeInfo("/myapp.v1.Public/Method"),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			extraMethods: map[string]struct{}{
				"/myapp.v1.Public/Method": {},
			},
			prepareMocks: func(_ *testing.T, _ *interceptorTestMocks) {
				// Custom open method — cache and auth must NOT be called.
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.NoError(t, err)
				assert.Equal(t, "ok", resp)
			},
		},
		"missing_metadata_returns_unauthenticated": {
			ctx:      func() context.Context { return context.Background() },
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(_ *testing.T, _ *interceptorTestMocks) {
				// No metadata, no cache/auth calls.
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Unauthenticated, status.Code(err))
			},
		},
		"missing_authorization_returns_unauthenticated": {
			ctx: func() context.Context {
				md := metadata.Pairs("x-custom", "value")
				return metadata.NewIncomingContext(context.Background(), md)
			},
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(_ *testing.T, _ *interceptorTestMocks) {
				// No authorization key in metadata.
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Unauthenticated, status.Code(err))
			},
		},
		"malformed_bearer_returns_unauthenticated": {
			ctx: func() context.Context {
				md := metadata.Pairs("authorization", "Token abc")
				return metadata.NewIncomingContext(context.Background(), md)
			},
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(_ *testing.T, _ *interceptorTestMocks) {
				// Malformed scheme — no cache/auth calls.
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Unauthenticated, status.Code(err))
			},
		},
		"invalid_token_returns_unauthenticated": {
			ctx:      func() context.Context { return makeCtxWithBearer("bad-sig") },
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("bad-sig")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:     false,
						ErrorReason: "invalid_signature",
					}, nil
				}
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Unauthenticated, status.Code(err))
			},
		},
		"expired_token_returns_unauthenticated": {
			ctx:      func() context.Context { return makeCtxWithBearer("exp-tok") },
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("exp-tok")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:     false,
						ErrorReason: "expired",
					}, nil
				}
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Unauthenticated, status.Code(err))
			},
		},
		"blocked_returns_unauthenticated": {
			ctx:      func() context.Context { return makeCtxWithBearer("blocked-tok") },
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("blocked-tok")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:     false,
						ErrorReason: "blocked",
					}, nil
				}
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Unauthenticated, status.Code(err))
			},
		},
		"unavailable_failclosed_returns_unavailable": {
			ctx:      func() context.Context { return makeCtxWithBearer("tok-down") },
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("tok-down")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.Unavailable, "auth down")
				}
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Unavailable, status.Code(err))
			},
		},
		"unavailable_failopen_calls_handler_with_empty_user": {
			ctx:      func() context.Context { return makeCtxWithBearer("tok-down-open") },
			info:     makeInfo(appMethod),
			failMode: authclient.FailOpen,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("tok-down-open")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.Unavailable, "auth down")
				}

				m.logger.EXPECT().
					Warn(
						mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
						"authclient: AUTH unavailable, fail-open pass-through",
					).
					Return().
					Once()
			},
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				user, ok := authclient.UserFromContext(ctx)
				if !ok {
					return nil, status.Error(codes.Internal, "no user in ctx")
				}
				if !user.IsZero() {
					return nil, status.Error(codes.Internal, "expected zero user")
				}
				return "pass-through", nil
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.NoError(t, err)
				assert.Equal(t, "pass-through", resp)
			},
		},
		"rate_limited_returns_resource_exhausted": {
			ctx:      func() context.Context { return makeCtxWithBearer("tok-throttled") },
			info:     makeInfo(appMethod),
			handler:  noopHandler,
			failMode: authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("tok-throttled")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.ResourceExhausted, "rate limit")
				}
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.ResourceExhausted, status.Code(err))
			},
		},
		"panic_in_handler_returns_internal": {
			ctx:      func() context.Context { return makeCtxWithBearer("ok-tok") },
			info:     makeInfo(appMethod),
			failMode: authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *interceptorTestMocks) {
				expectedKey := authclient.HashToken("ok-tok")
				expectedUser := authclient.User{UUID: "uid-panic", Role: authclient.RoleUser}

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:  true,
						UserUuid: "uid-panic",
						Role:     "user",
					}, nil
				}

				m.cache.EXPECT().
					Set(
						mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
						expectedKey,
						expectedUser,
						cacheTTL,
					).
					Return().
					Once()

				m.logger.EXPECT().
					Error(
						mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
						"authclient: grpc handler panic",
						nil,
						"panic", "boom",
						"stack", mock.Anything,
					).
					Return().
					Once()
			},
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				panic("boom")
			},
			want: func(t *testing.T, resp interface{}, err error) {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, codes.Internal, status.Code(err))
				assert.Contains(t, err.Error(), "internal error")
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newInterceptorTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(
				authclient.ClientConfig{
					ValidateCacheTTL: cacheTTL,
					ValidateTimeout:  validateTimeout,
					FailMode:         tc.failMode,
					OpenMethods:      tc.extraMethods,
				},
				m.cache,
				m.auth,
				m.logger,
			)

			interceptor := c.UnaryInterceptor()

			resp, err := interceptor(tc.ctx(), "request", tc.info, tc.handler)
			tc.want(t, resp, err)
		})
	}
}
