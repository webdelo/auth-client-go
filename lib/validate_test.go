package authclient_test

// fakeAuthClient (exported as FakeAuthClient) is defined in export_test.go
// in package authclient. The cache is mocked via mockery's MockCache.
// The import cycle lib_test→lib/mocks→lib is avoided by using package authclient_test
// rather than package authclient for test files that import lib/mocks.

import (
	"context"
	"errors"
	"testing"
	"time"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authclient "github.com/webdelo/auth-client-go/lib"
	mocks "github.com/webdelo/auth-client-go/lib/mocks"
)

type validateTestMocks struct {
	cache *mocks.MockCache
	auth  *authclient.FakeAuthClient
}

func newValidateTestMocks(t *testing.T) *validateTestMocks {
	t.Helper()
	return &validateTestMocks{
		cache: mocks.NewMockCache(t),
		auth:  &authclient.FakeAuthClient{},
	}
}

func TestClient_ValidateAccessToken(t *testing.T) {
	t.Parallel()

	const (
		cacheTTL        = 30 * time.Second
		validateTimeout = time.Second
		tokenABC        = "token-abc"
	)

	tests := map[string]struct {
		accessToken  string
		prepareMocks func(*testing.T, *validateTestMocks)
		want         func(*testing.T, authclient.User, error, *validateTestMocks)
	}{
		"happy_path": {
			accessToken: "tok-u1",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("tok-u1")
				expectedUser := authclient.User{UUID: "u1", Role: authclient.RoleUser}

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, in *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
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
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.NoError(t, err)
				if diff := cmp.Diff(authclient.User{UUID: "u1", Role: authclient.RoleUser}, user); diff != "" {
					t.Errorf("user mismatch (-want +got):\n%s", diff)
				}
				assert.Equal(t, 1, m.auth.ValidateCalls)
			},
		},
		"cache_hit_returns_without_grpc": {
			accessToken: "cached-token",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("cached-token")
				cachedUser := authclient.User{UUID: "u1", Role: authclient.RoleAdmin}

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(cachedUser, true).
					Once()
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.NoError(t, err)
				assert.Equal(t, authclient.User{UUID: "u1", Role: authclient.RoleAdmin}, user)
				assert.Equal(t, 0, m.auth.ValidateCalls)
			},
		},
		"empty_token_returns_invalid_token": {
			accessToken: "",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				// Cache.Get and authClient must NOT be called for empty token.
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrInvalidToken)
				assert.True(t, user.IsZero())
				assert.Equal(t, 0, m.auth.ValidateCalls)
			},
		},
		"is_valid_false_expired_returns_err_expired": {
			accessToken: "expired-tok",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("expired-tok")

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
				// Cache.Set must NOT be called on invalid result.
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrExpired)
				assert.True(t, user.IsZero())
			},
		},
		"is_valid_false_invalid_token_version_returns_err_expired": {
			accessToken: "old-version-tok",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("old-version-tok")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:     false,
						ErrorReason: "invalid_token_version",
					}, nil
				}
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrExpired)
				assert.True(t, user.IsZero())
			},
		},
		"is_valid_false_blocked_returns_err_blocked": {
			accessToken: "blocked-tok",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
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
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrBlocked)
				assert.True(t, user.IsZero())
			},
		},
		"is_valid_false_unknown_reason_returns_err_invalid_token": {
			accessToken: "bad-tok",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("bad-tok")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:     false,
						ErrorReason: "user_not_found",
					}, nil
				}
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrInvalidToken)
				assert.True(t, user.IsZero())
			},
		},
		"is_valid_false_empty_reason_returns_err_invalid_token": {
			accessToken: "empty-reason-tok",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("empty-reason-tok")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:     false,
						ErrorReason: "",
					}, nil
				}
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrInvalidToken)
				assert.True(t, user.IsZero())
			},
		},
		"grpc_unavailable_returns_err_unavailable": {
			accessToken: "tok-unavail",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("tok-unavail")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.Unavailable, "down")
				}
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrUnavailable)
				assert.True(t, user.IsZero())
			},
		},
		"grpc_deadline_exceeded_returns_err_unavailable": {
			accessToken: "tok-deadline",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("tok-deadline")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.DeadlineExceeded, "timed out")
				}
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.ErrorIs(t, err, authclient.ErrUnavailable)
				assert.True(t, user.IsZero())
			},
		},
		"grpc_resource_exhausted_returns_err_rate_limited": {
			accessToken: "tok-throttled",
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken("tok-throttled")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.ResourceExhausted, "throttled")
				}
			},
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.True(t, errors.Is(err, authclient.ErrRateLimited))
				assert.True(t, user.IsZero())
			},
		},
		"cache_key_is_hex_sha256": {
			accessToken: tokenABC,
			prepareMocks: func(t *testing.T, m *validateTestMocks) {
				expectedKey := authclient.HashToken(tokenABC)
				expectedUser := authclient.User{UUID: "uABC", Role: authclient.RoleUser}

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:  true,
						UserUuid: "uABC",
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
			want: func(t *testing.T, user authclient.User, err error, m *validateTestMocks) {
				require.NoError(t, err)
				if diff := cmp.Diff(authclient.User{UUID: "uABC", Role: authclient.RoleUser}, user); diff != "" {
					t.Errorf("user mismatch (-want +got):\n%s", diff)
				}
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newValidateTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(
				authclient.ClientConfig{
					ValidateCacheTTL: cacheTTL,
					ValidateTimeout:  validateTimeout,
				},
				m.cache,
				m.auth,
				nil,
			)

			user, err := c.ValidateAccessToken(context.Background(), tc.accessToken)
			tc.want(t, user, err, m)
		})
	}
}
