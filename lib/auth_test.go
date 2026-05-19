package authclient_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authclient "github.com/webdelo/auth-client-go/lib"
	"github.com/webdelo/auth-client-go/lib/mocks"
)

// testMocks groups all test doubles for auth method tests.
type testMocks struct {
	auth   *authclient.FakeAuthClient
	logger *mocks.MockLogger
}

func newTestMocks(t *testing.T) *testMocks {
	t.Helper()
	return &testMocks{
		auth:   &authclient.FakeAuthClient{},
		logger: mocks.NewMockLogger(t),
	}
}

// matchAnyCtx returns a testify matcher that accepts any non-nil context.Context.
// Implemented as a function (not a package-level variable) per UT-0029.
func matchAnyCtx() interface{} {
	return mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: login success", "user_uuid", "u-123").
					Return().
					Once()
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
				// No logger.Warn — not an unavailable error.
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
				// No logger.Warn — not an unavailable error.
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
				m.logger.EXPECT().
					Warn(matchAnyCtx(), "authclient: AUTH unavailable", "method", "login").
					Return().
					Once()
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: login success", "user_uuid", "").
					Return().
					Once()
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: refresh success").
					Return().
					Once()
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
				// No logger.Warn for non-unavailable errors.
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
				m.logger.EXPECT().
					Warn(matchAnyCtx(), "authclient: AUTH unavailable", "method", "refresh").
					Return().
					Once()
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: refresh success").
					Return().
					Once()
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

	// Both goroutines receive the success log.
	m.logger.EXPECT().
		Info(matchAnyCtx(), "authclient: refresh success").
		Return().
		Times(2)

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
// RefreshTokenFunc is recovered, returning ErrUnavailable, with appropriate
// Error and Warn log calls.
func TestClient_Refresh_FnPanicRecovered(t *testing.T) {
	t.Parallel()

	m := newTestMocks(t)

	m.auth.RefreshTokenFunc = func(_ context.Context, _ *auth_v1.RefreshTokenRequest, _ ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
		panic("boom")
	}

	// The panic is recovered; logger.Error is called with the panic info.
	m.logger.EXPECT().
		Error(
			matchAnyCtx(),
			"authclient: refresh fn panic",
			mock.MatchedBy(func(err error) bool { return errors.Is(err, authclient.ErrUnavailable) }),
			"recover", "boom",
		).
		Return().
		Once()

	// After error is mapped, logUnavailableIfNeeded emits Warn.
	m.logger.EXPECT().
		Warn(matchAnyCtx(), "authclient: AUTH unavailable", "method", "refresh").
		Return().
		Once()

	c := authclient.NewTestClient(authclient.ClientConfig{
		LoginTimeout:    5 * time.Second,
		RefreshTimeout:  5 * time.Second,
		ValidateTimeout: 2 * time.Second,
	}, nil, m.auth, m.logger)

	_, err := c.Refresh(context.Background(), "panic-token", "", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, authclient.ErrUnavailable))
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: logout success").
					Return().
					Once()
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
				m.logger.EXPECT().
					Warn(matchAnyCtx(), "authclient: AUTH unavailable", "method", "logout").
					Return().
					Once()
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: logout_all success").
					Return().
					Once()
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
				m.logger.EXPECT().
					Warn(matchAnyCtx(), "authclient: AUTH unavailable", "method", "logout_all").
					Return().
					Once()
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: change_password success").
					Return().
					Once()
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
				m.logger.EXPECT().
					Warn(matchAnyCtx(), "authclient: AUTH unavailable", "method", "change_password").
					Return().
					Once()
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
				m.logger.EXPECT().
					Info(matchAnyCtx(), "authclient: get_current_user success", "user_uuid", "u-456").
					Return().
					Once()
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
				// No logger expectation — authclient returns fmt.Errorf wrapping ErrUnavailable
				// but does NOT call logUnavailableIfNeeded (that's only for gRPC errors).
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
