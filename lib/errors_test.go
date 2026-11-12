package authclient

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/durationpb"
)

// statusWithRetryInfo builds a gRPC status error of the given code carrying a
// google.rpc.RetryInfo detail with the given delay.
func statusWithRetryInfo(t *testing.T, code codes.Code, delay *durationpb.Duration) error {
	t.Helper()

	st, err := status.New(code, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: delay})
	require.NoError(t, err)

	return st.Err()
}

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
		"aborted_stays_unavailable_in_shared_mapper": {
			input: status.Error(codes.Aborted, "retry window"),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
				assert.False(t, errors.Is(got, ErrRefreshRetry))
			},
		},
		"resource_exhausted_with_retry_info_carries_hint": {
			input: statusWithRetryInfo(t, codes.ResourceExhausted, durationpb.New(7*time.Second)),
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrRateLimited))
				assert.Equal(t, 7*time.Second, RetryAfter(got))
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
			input: &AuthError{Err: ErrRateLimited, RetryAfterHint: 5 * time.Second},
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
			input: fmt.Errorf("ctx: %w", &AuthError{Err: ErrRateLimited, RetryAfterHint: 3 * time.Second}),
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

func TestMapRefreshError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input     error
		wantIs    error
		wantNotIs error
		wantAfter time.Duration
	}{
		"happy_path": {
			input:     statusWithRetryInfo(t, codes.Aborted, durationpb.New(3*time.Second)),
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: 3 * time.Second,
		},
		"aborted_without_details_uses_default": {
			input:     status.Error(codes.Aborted, "retry window"),
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: time.Second,
		},
		"aborted_with_zero_delay_uses_default": {
			input:     statusWithRetryInfo(t, codes.Aborted, durationpb.New(0)),
			wantIs:    ErrRefreshRetry,
			wantAfter: time.Second,
		},
		"aborted_with_negative_delay_uses_default": {
			input:     statusWithRetryInfo(t, codes.Aborted, durationpb.New(-5*time.Second)),
			wantIs:    ErrRefreshRetry,
			wantAfter: time.Second,
		},
		"aborted_with_invalid_delay_uses_default": {
			input:     statusWithRetryInfo(t, codes.Aborted, &durationpb.Duration{Seconds: 400000000000}),
			wantIs:    ErrRefreshRetry,
			wantAfter: time.Second,
		},
		"aborted_with_nil_delay_uses_default": {
			input:     statusWithRetryInfo(t, codes.Aborted, nil),
			wantIs:    ErrRefreshRetry,
			wantAfter: time.Second,
		},
		"wrapped_aborted_with_retry_info": {
			input:     fmt.Errorf("refresh: %w", statusWithRetryInfo(t, codes.Aborted, durationpb.New(4*time.Second))),
			wantIs:    ErrRefreshRetry,
			wantAfter: 4 * time.Second,
		},
		"wrapped_result_keeps_errors_is_and_retry_after": {
			input:     fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", status.Error(codes.Aborted, "x"))),
			wantIs:    ErrRefreshRetry,
			wantAfter: time.Second,
		},
		"unavailable_stays_unavailable": {
			input:     status.Error(codes.Unavailable, "down"),
			wantIs:    ErrUnavailable,
			wantNotIs: ErrRefreshRetry,
		},
		"deadline_exceeded_stays_unavailable": {
			input:     status.Error(codes.DeadlineExceeded, "timeout"),
			wantIs:    ErrUnavailable,
			wantNotIs: ErrRefreshRetry,
		},
		"unauthenticated_delegates_to_shared_mapper": {
			input:  status.Error(codes.Unauthenticated, "bad cookie"),
			wantIs: ErrInvalidToken,
		},
		"non_grpc_error_delegates_to_shared_mapper": {
			input:     errors.New("plain error"),
			wantIs:    ErrUnavailable,
			wantNotIs: ErrRefreshRetry,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := mapRefreshError(tc.input)

			require.Error(t, got)
			assert.True(t, errors.Is(got, tc.wantIs))
			if tc.wantNotIs != nil {
				assert.False(t, errors.Is(got, tc.wantNotIs))
			}
			assert.Equal(t, tc.wantAfter, RetryAfter(got))
			assert.Equal(t, tc.wantAfter, RetryAfter(fmt.Errorf("caller: %w", got)))
		})
	}

	t.Run("nil_error_returns_nil", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, mapRefreshError(nil))
	})
}

func TestRetryAfterFromStatus(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input *status.Status
		want  time.Duration
	}{
		"happy_path": {
			input: statusOf(t, codes.ResourceExhausted, &errdetails.RetryInfo{RetryDelay: durationpb.New(2500 * time.Millisecond)}),
			want:  2500 * time.Millisecond,
		},
		"retry_info_among_other_details": {
			input: statusOf(t, codes.Aborted,
				&errdetails.ErrorInfo{Reason: "X"},
				&errdetails.RetryInfo{RetryDelay: durationpb.New(9 * time.Second)}),
			want: 9 * time.Second,
		},
		"no_details_returns_zero": {
			input: status.New(codes.ResourceExhausted, "slow down"),
			want:  0,
		},
		"only_unrelated_details_returns_zero": {
			input: statusOf(t, codes.Aborted, &errdetails.ErrorInfo{Reason: "X"}),
			want:  0,
		},
		"zero_delay_returns_zero": {
			input: statusOf(t, codes.Aborted, &errdetails.RetryInfo{RetryDelay: durationpb.New(0)}),
			want:  0,
		},
		"negative_delay_returns_zero": {
			input: statusOf(t, codes.Aborted, &errdetails.RetryInfo{RetryDelay: durationpb.New(-time.Second)}),
			want:  0,
		},
		"invalid_delay_returns_zero": {
			input: statusOf(t, codes.Aborted, &errdetails.RetryInfo{RetryDelay: &durationpb.Duration{Seconds: 400000000000}}),
			want:  0,
		},
		"missing_delay_returns_zero": {
			input: statusOf(t, codes.Aborted, &errdetails.RetryInfo{}),
			want:  0,
		},
		"nil_status_returns_zero": {
			input: nil,
			want:  0,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, retryAfterFromStatus(tc.input))
		})
	}
}

// statusOf builds a status with the given proto details attached.
func statusOf(t *testing.T, code codes.Code, details ...protoadapt.MessageV1) *status.Status {
	t.Helper()

	st, err := status.New(code, "msg").WithDetails(details...)
	require.NoError(t, err)

	return st
}
