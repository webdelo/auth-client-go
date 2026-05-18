package authclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authclient "github.com/webdelo/auth-client-go/lib"
	mocks "github.com/webdelo/auth-client-go/lib/mocks"
)

func TestBearerToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		authz string
		want  func(*testing.T, string, bool)
	}{
		"happy_path": {
			authz: "Bearer eyJabc",
			want: func(t *testing.T, token string, ok bool) {
				require.True(t, ok)
				assert.Equal(t, "eyJabc", token)
			},
		},
		"case_insensitive_scheme": {
			authz: "bearer eyJ",
			want: func(t *testing.T, token string, ok bool) {
				require.True(t, ok)
				assert.Equal(t, "eyJ", token)
			},
		},
		"empty_authz_returns_false": {
			authz: "",
			want: func(t *testing.T, token string, ok bool) {
				require.False(t, ok)
				assert.Empty(t, token)
			},
		},
		"no_bearer_prefix_returns_false": {
			authz: "Basic dXNlcjpwYXNz",
			want: func(t *testing.T, token string, ok bool) {
				require.False(t, ok)
				assert.Empty(t, token)
			},
		},
		"empty_token_after_prefix_returns_false": {
			authz: "Bearer ",
			want: func(t *testing.T, token string, ok bool) {
				require.False(t, ok)
				assert.Empty(t, token)
			},
		},
		"token_with_inner_whitespace_returns_false": {
			authz: "Bearer ab cd",
			want: func(t *testing.T, token string, ok bool) {
				require.False(t, ok)
				assert.Empty(t, token)
			},
		},
		"token_too_long_returns_false": {
			authz: "Bearer " + strings.Repeat("a", 8193),
			want: func(t *testing.T, token string, ok bool) {
				require.False(t, ok)
				assert.Empty(t, token)
			},
		},
		"leading_whitespace_trimmed": {
			authz: " \tBearer eyJ",
			want: func(t *testing.T, token string, ok bool) {
				require.True(t, ok)
				assert.Equal(t, "eyJ", token)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			token, ok := authclient.BearerToken(tc.authz)
			tc.want(t, token, ok)
		})
	}
}

type httpMiddlewareTestMocks struct {
	cache *mocks.MockCache
	auth  *authclient.FakeAuthClient
}

func newHTTPMiddlewareTestMocks(t *testing.T) *httpMiddlewareTestMocks {
	t.Helper()
	return &httpMiddlewareTestMocks{
		cache: mocks.NewMockCache(t),
		auth:  &authclient.FakeAuthClient{},
	}
}

func TestClient_HTTPMiddleware(t *testing.T) {
	t.Parallel()

	const (
		cacheTTL        = 30 * time.Second
		validateTimeout = time.Second
	)

	tests := map[string]struct {
		setupRequest func(*http.Request) // never nil — even no-op cases must pass func(*http.Request) {}
		failMode     authclient.FailMode
		prepareMocks func(*testing.T, *httpMiddlewareTestMocks)
		want         func(*testing.T, *httptest.ResponseRecorder, bool)
	}{
		"happy_path": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Bearer valid-token") },
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
				expectedKey := authclient.HashToken("valid-token")
				expectedUser := authclient.User{UUID: "uid1", Role: authclient.RoleUser}

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return &auth_v1.ValidateTokenResponse{
						IsValid:  true,
						UserUuid: "uid1",
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
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.True(t, nextCalled)
			},
		},
		"missing_authorization_returns_401": {
			setupRequest: func(*http.Request) {},
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
				// Cache and auth must not be touched.
			},
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.Equal(t, `Bearer realm="api"`, rec.Header().Get("WWW-Authenticate"))
				assert.False(t, nextCalled)
			},
		},
		"malformed_bearer_returns_401": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Token xyz") },
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
				// Cache and auth must not be touched.
			},
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.False(t, nextCalled)
			},
		},
		"invalid_token_returns_401_with_error_attribute": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Bearer abc") },
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
				expectedKey := authclient.HashToken("abc")

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
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.Equal(t, `Bearer realm="api", error="invalid_token"`, rec.Header().Get("WWW-Authenticate"))
				assert.False(t, nextCalled)
			},
		},
		"expired_token_returns_401": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Bearer expired-tok") },
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
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
			},
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.False(t, nextCalled)
			},
		},
		"blocked_user_returns_401": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Bearer blocked-tok") },
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
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
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.False(t, nextCalled)
			},
		},
		"unavailable_failclosed_returns_503": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok-down") },
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
				expectedKey := authclient.HashToken("tok-down")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.Unavailable, "service down")
				}
			},
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
				assert.Equal(t, "5", rec.Header().Get("Retry-After"))
				assert.False(t, nextCalled)
			},
		},
		"unavailable_failopen_passes_through_with_empty_user": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok-down-open") },
			failMode:     authclient.FailOpen,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
				expectedKey := authclient.HashToken("tok-down-open")

				m.cache.EXPECT().
					Get(mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }), expectedKey).
					Return(authclient.User{}, false).
					Once()

				m.auth.ValidateFunc = func(ctx context.Context, _ *auth_v1.ValidateTokenRequest, _ ...grpc.CallOption) (*auth_v1.ValidateTokenResponse, error) {
					_, hasDeadline := ctx.Deadline()
					require.True(t, hasDeadline, "ValidateToken ctx must carry a deadline")
					return nil, status.Error(codes.Unavailable, "service down")
				}
			},
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.True(t, nextCalled)
			},
		},
		"rate_limited_returns_429_no_header": {
			setupRequest: func(r *http.Request) { r.Header.Set("Authorization", "Bearer tok-throttled") },
			failMode:     authclient.FailClosed,
			prepareMocks: func(t *testing.T, m *httpMiddlewareTestMocks) {
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
			want: func(t *testing.T, rec *httptest.ResponseRecorder, nextCalled bool) {
				assert.Equal(t, http.StatusTooManyRequests, rec.Code)
				// AUTH v0.1.0 does not carry RetryInfo → RetryAfter == 0 → no header.
				assert.Empty(t, rec.Header().Get("Retry-After"))
				assert.False(t, nextCalled)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newHTTPMiddlewareTestMocks(t)
			tc.prepareMocks(t, m)

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			c := authclient.NewTestClient(
				authclient.ClientConfig{
					ValidateCacheTTL: cacheTTL,
					ValidateTimeout:  validateTimeout,
					FailMode:         tc.failMode,
				},
				m.cache,
				m.auth,
				nil,
			)

			handler := c.HTTPMiddleware(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.setupRequest(req)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			tc.want(t, rec, nextCalled)
		})
	}
}
