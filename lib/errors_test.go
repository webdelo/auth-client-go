package authclient

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapGRPCError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input error
		want  func(t *testing.T, got error)
	}{
		"happy_path": {
			input: status.Error(codes.Unauthenticated, "no token"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrInvalidToken))
			},
		},
		"permission_denied_returns_blocked": {
			input: status.Error(codes.PermissionDenied, "blocked"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrBlocked))
			},
		},
		"resource_exhausted_returns_rate_limited_with_zero_retry_after": {
			input: status.Error(codes.ResourceExhausted, "slow down"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrRateLimited))
				var ae *AuthError
				assert.True(t, errors.As(got, &ae))
				assert.Equal(t, time.Duration(0), RetryAfter(got))
			},
		},
		"unavailable_returns_unavailable": {
			input: status.Error(codes.Unavailable, "down"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"deadline_exceeded_returns_unavailable": {
			input: status.Error(codes.DeadlineExceeded, "timeout"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"unknown_grpc_code_returns_wrapped_unavailable": {
			input: status.Error(codes.Internal, "server error"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"non_grpc_error_returns_wrapped_unavailable": {
			input: errors.New("plain error"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"wrapped_grpc_error_unwraps_correctly": {
			input: fmt.Errorf("context: %w", status.Error(codes.Unauthenticated, "bad")),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrInvalidToken))
			},
		},
		"nil_error_returns_nil": {
			input: nil,
			want: func(t *testing.T, got error) {
				assert.NoError(t, got)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := mapGRPCError(tc.input)
			tc.want(t, got)
		})
	}
}

func TestMapErrorReason(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		reason string
		want   error
	}{
		"happy_path": {
			reason: "expired",
			want:   ErrExpired,
		},
		"reason_blocked_returns_blocked": {
			reason: "blocked",
			want:   ErrBlocked,
		},
		"reason_invalid_returns_invalid_token": {
			reason: "invalid",
			want:   ErrInvalidToken,
		},
		"reason_empty_returns_invalid_token": {
			reason: "",
			want:   ErrInvalidToken,
		},
		"reason_unknown_returns_invalid_token": {
			reason: "some_random",
			want:   ErrInvalidToken,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := mapErrorReason(tc.reason)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRetryAfter(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input error
		want  time.Duration
	}{
		"happy_path": {
			input: &AuthError{Err: ErrRateLimited, RetryAfter: 5 * time.Second},
			want:  5 * time.Second,
		},
		"non_auth_error_returns_zero": {
			input: errors.New("x"),
			want:  0,
		},
		"nil_returns_zero": {
			input: nil,
			want:  0,
		},
		"wrapped_auth_error_returns_value": {
			input: fmt.Errorf("ctx: %w", &AuthError{Err: ErrRateLimited, RetryAfter: 3 * time.Second}),
			want:  3 * time.Second,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := RetryAfter(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAuthError_Error(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input *AuthError
		want  string
	}{
		"happy_path": {
			input: &AuthError{Err: ErrRateLimited},
			want:  ErrRateLimited.Error(),
		},
		"nil_receiver_returns_empty_string": {
			input: (*AuthError)(nil),
			want:  "",
		},
		"nil_inner_err_returns_empty_string": {
			input: &AuthError{Err: nil},
			want:  "",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := tc.input.Error()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAuthError_Unwrap(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input *AuthError
		want  func(t *testing.T, got error)
	}{
		"happy_path": {
			input: &AuthError{Err: ErrRateLimited},
			want: func(t *testing.T, got error) {
				assert.True(t, errors.Is(got, ErrRateLimited))
			},
		},
		"nil_receiver_returns_nil": {
			input: (*AuthError)(nil),
			want: func(t *testing.T, got error) {
				assert.Nil(t, got)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := tc.input.Unwrap()
			tc.want(t, got)
		})
	}
}

func TestStatusFromWrapped(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input error
		want  func(t *testing.T, st *status.Status, ok bool)
	}{
		"happy_path": {
			input: status.Error(codes.Unauthenticated, "x"),
			want: func(t *testing.T, st *status.Status, ok bool) {
				require.True(t, ok)
				require.NotNil(t, st)
				assert.Equal(t, codes.Unauthenticated, st.Code())
			},
		},
		"wrapped_grpc_error": {
			input: fmt.Errorf("ctx: %w", status.Error(codes.Unauthenticated, "x")),
			want: func(t *testing.T, st *status.Status, ok bool) {
				require.True(t, ok)
				require.NotNil(t, st)
				assert.Equal(t, codes.Unauthenticated, st.Code())
			},
		},
		"double_wrapped": {
			input: fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", status.Error(codes.Unauthenticated, "x"))),
			want: func(t *testing.T, st *status.Status, ok bool) {
				require.True(t, ok)
				require.NotNil(t, st)
				assert.Equal(t, codes.Unauthenticated, st.Code())
			},
		},
		"non_grpc_error": {
			input: errors.New("plain"),
			want: func(t *testing.T, st *status.Status, ok bool) {
				assert.False(t, ok)
				assert.Nil(t, st)
			},
		},
		"nil_returns_nil_false": {
			input: nil,
			want: func(t *testing.T, st *status.Status, ok bool) {
				assert.False(t, ok)
				assert.Nil(t, st)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			st, ok := statusFromWrapped(tc.input)
			tc.want(t, st, ok)
		})
	}
}
