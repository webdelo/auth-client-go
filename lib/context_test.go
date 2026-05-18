package authclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser_IsZero(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		user User
		want func(*testing.T, bool)
	}{
		"happy_path": {
			user: User{},
			want: func(t *testing.T, got bool) {
				require.True(t, got)
			},
		},
		"with_uuid_returns_false": {
			user: User{UUID: "u"},
			want: func(t *testing.T, got bool) {
				require.False(t, got)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.want(t, tc.user.IsZero())
		})
	}
}

func TestUser_IsAdmin(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		user User
		want func(*testing.T, bool)
	}{
		"happy_path": {
			user: User{Role: RoleAdmin},
			want: func(t *testing.T, got bool) {
				require.True(t, got)
			},
		},
		"user_role_returns_false": {
			user: User{Role: RoleUser},
			want: func(t *testing.T, got bool) {
				require.False(t, got)
			},
		},
		"unknown_role_returns_false": {
			user: User{Role: RoleUnknown},
			want: func(t *testing.T, got bool) {
				require.False(t, got)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.want(t, tc.user.IsAdmin())
		})
	}
}

func TestUser_HasRole(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		user User
		role Role
		want func(*testing.T, bool)
	}{
		"happy_path": {
			user: User{Role: RoleAdmin},
			role: RoleAdmin,
			want: func(t *testing.T, got bool) {
				require.True(t, got)
			},
		},
		"mismatch_returns_false": {
			user: User{Role: RoleUser},
			role: RoleAdmin,
			want: func(t *testing.T, got bool) {
				require.False(t, got)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc.want(t, tc.user.HasRole(tc.role))
		})
	}
}

func TestUserFromContext(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ctx  func() context.Context
		want func(*testing.T, User, bool)
	}{
		"happy_path": {
			ctx: func() context.Context {
				return WithUser(context.Background(), User{UUID: "u1", Role: RoleUser})
			},
			want: func(t *testing.T, user User, ok bool) {
				require.True(t, ok)
				assert.Equal(t, "u1", user.UUID)
				assert.Equal(t, RoleUser, user.Role)
			},
		},
		"empty_ctx_returns_zero_and_false": {
			ctx: func() context.Context {
				return context.Background()
			},
			want: func(t *testing.T, user User, ok bool) {
				require.False(t, ok)
				assert.True(t, user.IsZero())
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			user, ok := UserFromContext(tc.ctx())
			tc.want(t, user, ok)
		})
	}
}
