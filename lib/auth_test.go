package authclient_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authclient "github.com/webdelo/auth-client-go/lib"
)

// testMocks groups all test doubles for auth method tests. The logger is a
// real *slog.Logger wired to a recording handler so individual tests can
// inspect emitted records when the SDK's logging behavior is itself part of
// the contract (e.g., panic-recovery error logging).
type testMocks struct {
	auth   *authclient.FakeAuthClient
	logger *slog.Logger
	rec    *authclient.RecordingHandler
}

func newTestMocks(_ *testing.T) *testMocks {
	logger, rec := authclient.NewRecordingLogger()
	return &testMocks{
		auth:   &authclient.FakeAuthClient{},
		logger: logger,
		rec:    rec,
	}
}

func TestClient_Login(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		prepareMocks func(*testing.T, *testMocks)
		want         func(*testing.T, authclient.LoginOutput, error)
	}{
		"happy_path": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LoginFunc = func(_ context.Context, _ *auth_v1.LoginRequest, _ ...grpc.CallOption) (*auth_v1.LoginResponse, error) {
					return &auth_v1.LoginResponse{
						AccessToken:           "access-tok",
						RefreshToken:          "refresh-tok",
						AccessTokenExpiresAt:  time.Now().Add(15 * time.Minute).Unix(),
						RefreshTokenExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix(),
						User: &auth_v1.User{
							Uuid:  "u-123",
							Email: "user@example.com",
							Role:  "user",
						},
					}, nil
				}
			},
			want: func(t *testing.T, out authclient.LoginOutput, err error) {
				require.NoError(t, err)
				assert.Equal(t, "access-tok", out.AccessToken)
				assert.Equal(t, "refresh-tok", out.RefreshToken)
				assert.Greater(t, out.AccessExpiresIn, time.Duration(0))
				assert.Greater(t, out.RefreshExpiresIn, time.Duration(0))
				assert.Equal(t, "u-123", out.User.UUID)
				assert.Equal(t, "user@example.com", out.User.Email)
				assert.Equal(t, authclient.RoleUser, out.User.Role)
			},
		},
		"invalid_credentials_returns_invalid_token": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LoginFunc = func(_ context.Context, _ *auth_v1.LoginRequest, _ ...grpc.CallOption) (*auth_v1.LoginResponse, error) {
					return nil, status.Error(codes.Unauthenticated, "bad creds")
				}
			},
			want: func(t *testing.T, out authclient.LoginOutput, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrInvalidToken))
			},
		},
		"rate_limited_returns_rate_limited": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LoginFunc = func(_ context.Context, _ *auth_v1.LoginRequest, _ ...grpc.CallOption) (*auth_v1.LoginResponse, error) {
					return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
				}
			},
			want: func(t *testing.T, out authclient.LoginOutput, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrRateLimited))
			},
		},
		"unavailable_returns_unavailable": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LoginFunc = func(_ context.Context, _ *auth_v1.LoginRequest, _ ...grpc.CallOption) (*auth_v1.LoginResponse, error) {
					return nil, status.Error(codes.Unavailable, "auth down")
				}
			},
			want: func(t *testing.T, out authclient.LoginOutput, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrUnavailable))
			},
		},
		"nil_user_in_response_returns_zero_user": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LoginFunc = func(_ context.Context, _ *auth_v1.LoginRequest, _ ...grpc.CallOption) (*auth_v1.LoginResponse, error) {
					return &auth_v1.LoginResponse{
						AccessToken:  "tok",
						RefreshToken: "rtok",
						User:         nil,
					}, nil
				}
			},
			want: func(t *testing.T, out authclient.LoginOutput, err error) {
				require.NoError(t, err)
				assert.Equal(t, authclient.User{}, out.User)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(authclient.ClientConfig{
				LoginTimeout:    5 * time.Second,
				RefreshTimeout:  5 * time.Second,
				ValidateTimeout: 2 * time.Second,
			}, nil, m.auth, m.logger)

			out, err := c.Login(context.Background(), authclient.LoginInput{
				Email:    "user@example.com",
				Password: "password",
			})
			tc.want(t, out, err)
		})
	}
}

func TestClient_Refresh(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		prepareMocks func(*testing.T, *testMocks)
		clientIP     string
		userAgent    string
		refreshToken string
		want         func(*testing.T, authclient.RefreshOutput, error)
	}{
		"happy_path": {
			refreshToken: "old-rtok",
			clientIP:     "1.2.3.4",
			userAgent:    "test-agent",
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.RefreshTokenFunc = func(_ context.Context, _ *auth_v1.RefreshTokenRequest, _ ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
					return &auth_v1.RefreshTokenResponse{
						AccessToken:           "new-access-tok",
						RefreshToken:          "new-refresh-tok",
						AccessTokenExpiresAt:  time.Now().Add(15 * time.Minute).Unix(),
						RefreshTokenExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix(),
					}, nil
				}
			},
			want: func(t *testing.T, out authclient.RefreshOutput, err error) {
				require.NoError(t, err)
				assert.Equal(t, "new-access-tok", out.AccessToken)
				assert.Equal(t, "new-refresh-tok", out.RefreshToken)
				assert.Greater(t, out.AccessExpiresIn, time.Duration(0))
				assert.Greater(t, out.RefreshExpiresIn, time.Duration(0))
			},
		},
		"invalid_refresh_token_returns_invalid": {
			refreshToken: "bad-rtok",
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.RefreshTokenFunc = func(_ context.Context, _ *auth_v1.RefreshTokenRequest, _ ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
					return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
				}
			},
			want: func(t *testing.T, out authclient.RefreshOutput, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrInvalidToken))
			},
		},
		"unavailable_returns_unavailable": {
			refreshToken: "rtok-unavail",
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.RefreshTokenFunc = func(_ context.Context, _ *auth_v1.RefreshTokenRequest, _ ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
					return nil, status.Error(codes.Unavailable, "auth down")
				}
			},
			want: func(t *testing.T, out authclient.RefreshOutput, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrUnavailable))
			},
		},
		"ip_and_user_agent_propagated_to_grpc": {
			refreshToken: "rtok-fingerprint",
			clientIP:     "1.2.3.4",
			userAgent:    "test-agent",
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.RefreshTokenFunc = func(_ context.Context, req *auth_v1.RefreshTokenRequest, _ ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
					assert.Equal(t, "1.2.3.4", req.GetIp())
					assert.Equal(t, "test-agent", req.GetUserAgent())
					return &auth_v1.RefreshTokenResponse{
						AccessToken:  "a",
						RefreshToken: "r",
					}, nil
				}
			},
			want: func(t *testing.T, out authclient.RefreshOutput, err error) {
				require.NoError(t, err)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(authclient.ClientConfig{
				LoginTimeout:    5 * time.Second,
				RefreshTimeout:  5 * time.Second,
				ValidateTimeout: 2 * time.Second,
			}, nil, m.auth, m.logger)

			out, err := c.Refresh(context.Background(), tc.refreshToken, tc.clientIP, tc.userAgent)
			tc.want(t, out, err)
		})
	}
}

// UT-0002 exception: concurrent goroutine rendezvous / singleflight deduplication
// require dedicated setup (rendezvous channels, N goroutines, atomic call counter)
// that does not fit a flat table row alongside TestClient_Refresh's sequential cases.

// TestClient_Refresh_SingleFlight verifies that concurrent Refresh calls
// with the same refresh token result in exactly one gRPC call (singleflight
// deduplication), and both goroutines receive the same result.
func TestClient_Refresh_SingleFlight(t *testing.T) {
	t.Parallel()

	m := newTestMocks(t)

	started := make(chan struct{}, 1)
	release := make(chan struct{})

	m.auth.RefreshTokenFunc = func(_ context.Context, _ *auth_v1.RefreshTokenRequest, _ ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
		// Signal that the leader has entered fn (non-blocking, only first goroutine).
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return &auth_v1.RefreshTokenResponse{
			AccessToken:  "shared-access-tok",
			RefreshToken: "shared-refresh-tok",
		}, nil
	}

	c := authclient.NewTestClient(authclient.ClientConfig{
		LoginTimeout:    5 * time.Second,
		RefreshTimeout:  5 * time.Second,
		ValidateTimeout: 2 * time.Second,
	}, nil, m.auth, m.logger)

	outputs := make([]authclient.RefreshOutput, 2)
	errs := make([]error, 2)

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			outputs[i], errs[i] = c.Refresh(context.Background(), "same-refresh-token", "1.2.3.4", "agent")
		}()
	}

	// Wait for leader to enter fn, then give follower time to converge.
	<-started
	time.Sleep(50 * time.Millisecond)
	close(release)

	wg.Wait()

	assert.EqualValues(t, 1, m.auth.RefreshTokenCalls.Load(), "RefreshToken gRPC must be called exactly once")
	for i := 0; i < 2; i++ {
		require.NoError(t, errs[i], "goroutine %d must receive no error", i)
		assert.Equal(t, "shared-access-tok", outputs[i].AccessToken, "goroutine %d must receive shared access token", i)
		assert.Equal(t, "shared-refresh-tok", outputs[i].RefreshToken, "goroutine %d must receive shared refresh token", i)
	}
}

// UT-0002 exception: panic recovery requires a dedicated panic-inducing setup
// that does not fit a flat table row alongside TestClient_Refresh's sequential cases.

// TestClient_Refresh_FnPanicRecovered verifies that a panic inside
// RefreshTokenFunc is recovered, returning ErrUnavailable, AND that the SDK
// emits an Error-level log so the panic does not go silent in production.
func TestClient_Refresh_FnPanicRecovered(t *testing.T) {
	t.Parallel()

	m := newTestMocks(t)

	m.auth.RefreshTokenFunc = func(_ context.Context, _ *auth_v1.RefreshTokenRequest, _ ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
		panic("boom")
	}

	c := authclient.NewTestClient(authclient.ClientConfig{
		LoginTimeout:    5 * time.Second,
		RefreshTimeout:  5 * time.Second,
		ValidateTimeout: 2 * time.Second,
	}, nil, m.auth, m.logger)

	_, err := c.Refresh(context.Background(), "panic-token", "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, authclient.ErrUnavailable))

	// Behavioural check: the recovered panic must be visible at ERROR level —
	// otherwise an in-process panic would silently degrade to a generic
	// "unavailable" without operator-actionable diagnostics.
	var sawError bool
	for _, r := range m.rec.Records() {
		if r.Level == slog.LevelError {
			sawError = true
			break
		}
	}
	assert.True(t, sawError, "panic recovery must emit at least one Error-level record")
}

func TestClient_Logout(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		prepareMocks func(*testing.T, *testMocks)
		want         func(*testing.T, error)
	}{
		"happy_path": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LogoutFunc = func(_ context.Context, _ *auth_v1.LogoutRequest, _ ...grpc.CallOption) (*auth_v1.LogoutResponse, error) {
					return &auth_v1.LogoutResponse{}, nil
				}
			},
			want: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		"invalid_token_returns_invalid": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LogoutFunc = func(_ context.Context, _ *auth_v1.LogoutRequest, _ ...grpc.CallOption) (*auth_v1.LogoutResponse, error) {
					return nil, status.Error(codes.Unauthenticated, "invalid token")
				}
			},
			want: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrInvalidToken))
			},
		},
		"unavailable_returns_unavailable": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LogoutFunc = func(_ context.Context, _ *auth_v1.LogoutRequest, _ ...grpc.CallOption) (*auth_v1.LogoutResponse, error) {
					return nil, status.Error(codes.Unavailable, "auth down")
				}
			},
			want: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrUnavailable))
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(authclient.ClientConfig{
				LoginTimeout:    5 * time.Second,
				RefreshTimeout:  5 * time.Second,
				ValidateTimeout: 2 * time.Second,
			}, nil, m.auth, m.logger)

			err := c.Logout(context.Background(), "refresh-token")
			tc.want(t, err)
		})
	}
}

func TestClient_LogoutAll(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		prepareMocks func(*testing.T, *testMocks)
		want         func(*testing.T, error)
	}{
		"happy_path": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LogoutAllFunc = func(_ context.Context, _ *auth_v1.LogoutAllRequest, _ ...grpc.CallOption) (*auth_v1.LogoutAllResponse, error) {
					return &auth_v1.LogoutAllResponse{}, nil
				}
			},
			want: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		"invalid_token_returns_invalid": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LogoutAllFunc = func(_ context.Context, _ *auth_v1.LogoutAllRequest, _ ...grpc.CallOption) (*auth_v1.LogoutAllResponse, error) {
					return nil, status.Error(codes.Unauthenticated, "invalid token")
				}
			},
			want: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrInvalidToken))
			},
		},
		"unavailable_returns_unavailable": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.LogoutAllFunc = func(_ context.Context, _ *auth_v1.LogoutAllRequest, _ ...grpc.CallOption) (*auth_v1.LogoutAllResponse, error) {
					return nil, status.Error(codes.Unavailable, "auth down")
				}
			},
			want: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrUnavailable))
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(authclient.ClientConfig{
				LoginTimeout:    5 * time.Second,
				RefreshTimeout:  5 * time.Second,
				ValidateTimeout: 2 * time.Second,
			}, nil, m.auth, m.logger)

			err := c.LogoutAll(context.Background(), "access-token")
			tc.want(t, err)
		})
	}
}

func TestClient_ChangePassword(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		prepareMocks func(*testing.T, *testMocks)
		want         func(*testing.T, error)
	}{
		"happy_path": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.ChangePasswordFunc = func(_ context.Context, _ *auth_v1.ChangePasswordRequest, _ ...grpc.CallOption) (*auth_v1.ChangePasswordResponse, error) {
					return &auth_v1.ChangePasswordResponse{}, nil
				}
			},
			want: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		"invalid_token_returns_invalid": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.ChangePasswordFunc = func(_ context.Context, _ *auth_v1.ChangePasswordRequest, _ ...grpc.CallOption) (*auth_v1.ChangePasswordResponse, error) {
					return nil, status.Error(codes.Unauthenticated, "invalid token")
				}
			},
			want: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrInvalidToken))
			},
		},
		"unavailable_returns_unavailable": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.ChangePasswordFunc = func(_ context.Context, _ *auth_v1.ChangePasswordRequest, _ ...grpc.CallOption) (*auth_v1.ChangePasswordResponse, error) {
					return nil, status.Error(codes.Unavailable, "auth down")
				}
			},
			want: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrUnavailable))
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(authclient.ClientConfig{
				LoginTimeout:    5 * time.Second,
				RefreshTimeout:  5 * time.Second,
				ValidateTimeout: 2 * time.Second,
			}, nil, m.auth, m.logger)

			err := c.ChangePassword(context.Background(), "access-token", "old-pass", "new-pass")
			tc.want(t, err)
		})
	}
}

func TestClient_GetCurrentUser(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		prepareMocks func(*testing.T, *testMocks)
		want         func(*testing.T, authclient.User, error)
	}{
		"happy_path": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.GetCurrentUserFunc = func(_ context.Context, _ *auth_v1.GetCurrentUserRequest, _ ...grpc.CallOption) (*auth_v1.GetCurrentUserResponse, error) {
					return &auth_v1.GetCurrentUserResponse{
						User: &auth_v1.User{
							Uuid:  "u-456",
							Email: "admin@example.com",
							Role:  "admin",
						},
					}, nil
				}
			},
			want: func(t *testing.T, user authclient.User, err error) {
				require.NoError(t, err)
				assert.Equal(t, "u-456", user.UUID)
				assert.Equal(t, "admin@example.com", user.Email)
				assert.Equal(t, authclient.RoleAdmin, user.Role)
			},
		},
		"empty_user_in_response_returns_unavailable": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.GetCurrentUserFunc = func(_ context.Context, _ *auth_v1.GetCurrentUserRequest, _ ...grpc.CallOption) (*auth_v1.GetCurrentUserResponse, error) {
					return &auth_v1.GetCurrentUserResponse{User: nil}, nil
				}
			},
			want: func(t *testing.T, user authclient.User, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrUnavailable))
				assert.True(t, user.IsZero())
			},
		},
		"invalid_token_returns_invalid": {
			prepareMocks: func(t *testing.T, m *testMocks) {
				m.auth.GetCurrentUserFunc = func(_ context.Context, _ *auth_v1.GetCurrentUserRequest, _ ...grpc.CallOption) (*auth_v1.GetCurrentUserResponse, error) {
					return nil, status.Error(codes.Unauthenticated, "invalid token")
				}
			},
			want: func(t *testing.T, user authclient.User, err error) {
				require.Error(t, err)
				assert.True(t, errors.Is(err, authclient.ErrInvalidToken))
				assert.True(t, user.IsZero())
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newTestMocks(t)
			tc.prepareMocks(t, m)

			c := authclient.NewTestClient(authclient.ClientConfig{
				LoginTimeout:    5 * time.Second,
				RefreshTimeout:  5 * time.Second,
				ValidateTimeout: 2 * time.Second,
			}, nil, m.auth, m.logger)

			user, err := c.GetCurrentUser(context.Background(), "access-token")
			tc.want(t, user, err)
		})
	}
}
