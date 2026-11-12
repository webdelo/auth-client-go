package authclient

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// maxValidDurationSeconds is the largest seconds value a valid
// google.protobuf.Duration may carry (about 10000 years).
const maxValidDurationSeconds = 315576000000

func TestMapGRPCError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input func(t *testing.T) error
		want  func(t *testing.T, got error)
	}{
		"happy_path": {
			input: func(*testing.T) error { return status.Error(codes.Unauthenticated, "no token") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrInvalidToken))
			},
		},
		"permission_denied_returns_blocked": {
			input: func(*testing.T) error { return status.Error(codes.PermissionDenied, "blocked") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrBlocked))
			},
		},
		"resource_exhausted_returns_rate_limited_with_zero_retry_after": {
			input: func(*testing.T) error { return status.Error(codes.ResourceExhausted, "slow down") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrRateLimited))
				var ae *AuthError
				assert.True(t, errors.As(got, &ae))
				assert.Equal(t, time.Duration(0), RetryAfter(got))
			},
		},
		"unavailable_returns_unavailable": {
			input: func(*testing.T) error { return status.Error(codes.Unavailable, "down") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"deadline_exceeded_returns_unavailable": {
			input: func(*testing.T) error { return status.Error(codes.DeadlineExceeded, "timeout") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"unknown_grpc_code_returns_wrapped_unavailable": {
			input: func(*testing.T) error { return status.Error(codes.Internal, "server error") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"non_grpc_error_returns_wrapped_unavailable": {
			input: func(*testing.T) error { return errors.New("plain error") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
			},
		},
		"wrapped_grpc_error_unwraps_correctly": {
			input: func(*testing.T) error { return fmt.Errorf("context: %w", status.Error(codes.Unauthenticated, "bad")) },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrInvalidToken))
			},
		},
		"aborted_stays_unavailable_in_shared_mapper": {
			input: func(*testing.T) error { return status.Error(codes.Aborted, "retry window") },
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrUnavailable))
				assert.False(t, errors.Is(got, ErrRefreshRetry))
			},
		},
		"resource_exhausted_with_retry_info_carries_hint": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.ResourceExhausted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(7 * time.Second)})
				require.NoError(t, err)

				return st.Err()
			},
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrRateLimited))
				assert.Equal(t, 7*time.Second, RetryAfter(got))
			},
		},
		"resource_exhausted_with_24h_retry_info_is_clamped": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.ResourceExhausted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(24 * time.Hour)})
				require.NoError(t, err)

				return st.Err()
			},
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrRateLimited))
				assert.Equal(t, MaxRetryAfter, RetryAfter(got))
			},
		},
		"resource_exhausted_with_max_valid_retry_info_is_clamped": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.ResourceExhausted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: &durationpb.Duration{Seconds: maxValidDurationSeconds}})
				require.NoError(t, err)

				return st.Err()
			},
			want: func(t *testing.T, got error) {
				require.Error(t, got)
				assert.True(t, errors.Is(got, ErrRateLimited))
				assert.Equal(t, MaxRetryAfter, RetryAfter(got))
			},
		},
		"nil_error_returns_nil": {
			input: func(*testing.T) error { return nil },
			want: func(t *testing.T, got error) {
				assert.NoError(t, got)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := mapGRPCError(tc.input(t))
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
		input     func(t *testing.T) error
		wantIs    error
		wantNotIs error
		wantAfter time.Duration
	}{
		"happy_path": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(3 * time.Second)})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: 3 * time.Second,
		},
		"aborted_without_details_uses_default": {
			input:     func(*testing.T) error { return status.Error(codes.Aborted, "retry window") },
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: time.Second,
		},
		"aborted_with_zero_delay_uses_default": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(0)})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: time.Second,
		},
		"aborted_with_negative_delay_uses_default": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(-5 * time.Second)})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: time.Second,
		},
		"aborted_with_invalid_delay_uses_default": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: &durationpb.Duration{Seconds: 400000000000}})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: time.Second,
		},
		"aborted_with_nil_delay_uses_default": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: nil})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: time.Second,
		},
		"aborted_with_24h_delay_is_clamped": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(24 * time.Hour)})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: MaxRetryAfter,
		},
		"aborted_with_max_valid_delay_is_clamped": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: &durationpb.Duration{Seconds: maxValidDurationSeconds}})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: MaxRetryAfter,
		},
		"aborted_with_saturated_nanos_delay_is_clamped": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(time.Duration(math.MaxInt64))})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: MaxRetryAfter,
		},
		"wrapped_aborted_with_retry_info": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(4 * time.Second)})
				require.NoError(t, err)

				return fmt.Errorf("refresh: %w", st.Err())
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: 4 * time.Second,
		},
		"wrapped_result_keeps_errors_is_and_retry_after": {
			input: func(*testing.T) error {
				return fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", status.Error(codes.Aborted, "x")))
			},
			wantIs:    ErrRefreshRetry,
			wantNotIs: ErrUnavailable,
			wantAfter: time.Second,
		},
		"unavailable_stays_unavailable": {
			input:     func(*testing.T) error { return status.Error(codes.Unavailable, "down") },
			wantIs:    ErrUnavailable,
			wantNotIs: ErrRefreshRetry,
			wantAfter: 0,
		},
		"deadline_exceeded_stays_unavailable": {
			input:     func(*testing.T) error { return status.Error(codes.DeadlineExceeded, "timeout") },
			wantIs:    ErrUnavailable,
			wantNotIs: ErrRefreshRetry,
			wantAfter: 0,
		},
		"internal_delegates_to_shared_mapper_as_unavailable": {
			input:     func(*testing.T) error { return status.Error(codes.Internal, "server error") },
			wantIs:    ErrUnavailable,
			wantNotIs: ErrRefreshRetry,
			wantAfter: 0,
		},
		"unauthenticated_delegates_to_shared_mapper": {
			input:     func(*testing.T) error { return status.Error(codes.Unauthenticated, "bad cookie") },
			wantIs:    ErrInvalidToken,
			wantNotIs: ErrUnavailable,
			wantAfter: 0,
		},
		"resource_exhausted_with_retry_info_returns_rate_limited_with_hint": {
			input: func(t *testing.T) error {
				st, err := status.New(codes.ResourceExhausted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(7 * time.Second)})
				require.NoError(t, err)

				return st.Err()
			},
			wantIs:    ErrRateLimited,
			wantNotIs: ErrRefreshRetry,
			wantAfter: 7 * time.Second,
		},
		"resource_exhausted_without_details_returns_rate_limited_without_hint": {
			input:     func(*testing.T) error { return status.Error(codes.ResourceExhausted, "slow down") },
			wantIs:    ErrRateLimited,
			wantNotIs: ErrRefreshRetry,
			wantAfter: 0,
		},
		"non_grpc_error_delegates_to_shared_mapper": {
			input:     func(*testing.T) error { return errors.New("plain error") },
			wantIs:    ErrUnavailable,
			wantNotIs: ErrRefreshRetry,
			wantAfter: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := mapRefreshError(tc.input(t))

			require.Error(t, got)
			assert.True(t, errors.Is(got, tc.wantIs))
			assert.False(t, errors.Is(got, tc.wantNotIs))
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
		input func(t *testing.T) *status.Status
		want  time.Duration
	}{
		"happy_path": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.ResourceExhausted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(2500 * time.Millisecond)})
				require.NoError(t, err)

				return st
			},
			want: 2500 * time.Millisecond,
		},
		"retry_info_among_other_details": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.ErrorInfo{Reason: "X"}, &errdetails.RetryInfo{RetryDelay: durationpb.New(9 * time.Second)})
				require.NoError(t, err)

				return st
			},
			want: 9 * time.Second,
		},
		"no_details_returns_zero": {
			input: func(*testing.T) *status.Status { return status.New(codes.ResourceExhausted, "slow down") },
			want:  0,
		},
		"only_unrelated_details_returns_zero": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.ErrorInfo{Reason: "X"})
				require.NoError(t, err)

				return st
			},
			want: 0,
		},
		"zero_delay_returns_zero": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(0)})
				require.NoError(t, err)

				return st
			},
			want: 0,
		},
		"negative_delay_returns_zero": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(-time.Second)})
				require.NoError(t, err)

				return st
			},
			want: 0,
		},
		"invalid_delay_returns_zero": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: &durationpb.Duration{Seconds: 400000000000}})
				require.NoError(t, err)

				return st
			},
			want: 0,
		},
		"missing_delay_returns_zero": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{})
				require.NoError(t, err)

				return st
			},
			want: 0,
		},
		"delay_equal_to_cap_is_kept": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(MaxRetryAfter)})
				require.NoError(t, err)

				return st
			},
			want: MaxRetryAfter,
		},
		"delay_above_cap_is_clamped": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(24 * time.Hour)})
				require.NoError(t, err)

				return st
			},
			want: MaxRetryAfter,
		},
		"max_valid_delay_is_clamped": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: &durationpb.Duration{Seconds: maxValidDurationSeconds}})
				require.NoError(t, err)

				return st
			},
			want: MaxRetryAfter,
		},
		"saturated_delay_is_clamped": {
			input: func(t *testing.T) *status.Status {
				st, err := status.New(codes.Aborted, "msg").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(time.Duration(math.MaxInt64))})
				require.NoError(t, err)

				return st
			},
			want: MaxRetryAfter,
		},
		"nil_status_returns_zero": {
			input: func(*testing.T) *status.Status { return nil },
			want:  0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, retryAfterFromStatus(tc.input(t)))
		})
	}
}
