package authclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrInvalidToken indicates the access token is malformed, unknown, or signed
// by a different key. The caller should re-authenticate.
var ErrInvalidToken = errors.New("authclient: invalid token")

// ErrExpired indicates the token signature is valid but its expiration has
// passed. The caller should refresh.
var ErrExpired = errors.New("authclient: token expired")

// ErrBlocked indicates the user account is blocked at the AUTH server.
// The caller should NOT retry — the block is persistent until admin action.
var ErrBlocked = errors.New("authclient: user blocked")

// ErrRateLimited indicates AUTH rate-limited this caller. Inspect the
// Retry-After hint via the RetryAfter helper to know when to retry.
var ErrRateLimited = errors.New("authclient: rate limited")

// ErrUnavailable indicates AUTH was unreachable (network, timeout, server
// down). HTTPMiddleware behavior depends on Config.FailMode.
var ErrUnavailable = errors.New("authclient: service unavailable")

// ErrRefreshRetry indicates Refresh was answered with gRPC ABORTED: the same
// refresh cookie is already being rotated, so the session is alive. The caller
// should repeat Refresh after RetryAfter(err); the hint is
// DefaultRefreshRetryAfter when AUTH sent no RetryInfo. It is distinct from
// ErrUnavailable: errors.Is(err, ErrUnavailable) is false for it.
var ErrRefreshRetry = errors.New("authclient: refresh in progress, retry")

// DefaultRefreshRetryAfter is the Retry-After hint used for ErrRefreshRetry
// when the ABORTED status carries no valid positive google.rpc.RetryInfo.
const DefaultRefreshRetryAfter = time.Second

// MaxRetryAfter is the upper bound applied to the delay parsed from
// google.rpc.RetryInfo, so a misbehaving server cannot make callers wait
// unreasonably long.
const MaxRetryAfter = 5 * time.Minute

// AuthError carries the sentinel error plus optional context (Retry-After for
// rate-limited and refresh-retry responses). Use errors.Is(err, ErrXxx) for
// sentinel detection and RetryAfter(err) for the retry hint.
type AuthError struct {
	// Err is the wrapped sentinel error (ErrRateLimited, etc.) used by
	// errors.Is for sentinel detection.
	Err error

	// RetryAfterHint is the Retry-After value for ErrRateLimited and
	// ErrRefreshRetry errors, taken from google.rpc.RetryInfo and clamped to
	// MaxRetryAfter. For ErrRefreshRetry it is DefaultRefreshRetryAfter when
	// AUTH sent none. Zero means "no hint". Read it via the RetryAfter helper
	// rather than the field directly when extracting from a wrapped error.
	RetryAfterHint time.Duration
}

// Error implements the error interface, delegating to the wrapped sentinel.
func (e *AuthError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}

	return e.Err.Error()
}

// Unwrap returns the wrapped sentinel error so errors.Is/As traverse it.
func (e *AuthError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// RetryAfter returns the Retry-After hint for ErrRateLimited and
// ErrRefreshRetry errors, never more than MaxRetryAfter. For ErrRefreshRetry
// without a server hint it returns DefaultRefreshRetryAfter. For every other
// error it returns 0: err is nil, is not an *AuthError, or the server did not
// include a hint and no default applies.
func RetryAfter(err error) time.Duration {
	var ae *AuthError
	if errors.As(err, &ae) {
		return ae.RetryAfterHint
	}

	return 0
}

// mapGRPCError converts a gRPC error into one of the SDK sentinel errors.
// Returns nil if err is nil. Unwraps through fmt.Errorf("...: %w", ...)
// chains because status.FromError does NOT follow errors.Unwrap.
func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := statusFromWrapped(err)
	if !ok {
		return fmt.Errorf("authclient: %w", ErrUnavailable)
	}

	switch st.Code() {
	case codes.Unauthenticated:
		return ErrInvalidToken
	case codes.PermissionDenied:
		return ErrBlocked
	case codes.ResourceExhausted:
		return &AuthError{Err: ErrRateLimited, RetryAfterHint: retryAfterFromStatus(st)}
	case codes.Unavailable, codes.DeadlineExceeded:
		return ErrUnavailable
	case codes.Canceled:
		return fmt.Errorf("authclient: %w", context.Canceled)
	default:
		return fmt.Errorf("authclient: unexpected grpc code %s: %w", st.Code(), ErrUnavailable)
	}
}

// mapRefreshError converts an error from the Refresh RPC into an SDK error.
// gRPC ABORTED becomes an *AuthError wrapping ErrRefreshRetry, with the
// RetryInfo delay as hint or DefaultRefreshRetryAfter when AUTH sent none.
// Every other error is delegated to mapGRPCError unchanged, so the shared
// mapper keeps ABORTED as ErrUnavailable for all other RPCs.
func mapRefreshError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := statusFromWrapped(err)
	if !ok || st.Code() != codes.Aborted {
		return mapGRPCError(err)
	}

	hint := retryAfterFromStatus(st)
	if hint <= 0 {
		hint = DefaultRefreshRetryAfter
	}

	return &AuthError{Err: ErrRefreshRetry, RetryAfterHint: hint}
}

// retryAfterFromStatus extracts the Retry-After hint from the
// google.rpc.RetryInfo entry in gRPC status details. Returns 0 when the
// details carry no RetryInfo or its retry_delay is missing, invalid, or not
// positive. The delay is clamped to MaxRetryAfter.
func retryAfterFromStatus(st *status.Status) time.Duration {
	if st == nil {
		return 0
	}

	for _, detail := range st.Details() {
		info, ok := detail.(*errdetails.RetryInfo)
		if !ok {
			continue
		}

		delay := info.GetRetryDelay()
		if delay.CheckValid() != nil {
			continue
		}

		if d := delay.AsDuration(); d > 0 {
			return min(d, MaxRetryAfter)
		}
	}

	return 0
}

// statusFromWrapped extracts a gRPC *Status from an error chain, walking
// through errors.Unwrap. The standard status.FromError does not unwrap, so
// callers that wrap gRPC errors with fmt.Errorf("...: %w", err) would lose
// the code without this helper.
func statusFromWrapped(err error) (*status.Status, bool) {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if st, ok := status.FromError(e); ok && st.Code() != codes.Unknown {
			return st, true
		}
	}

	return nil, false
}

// mapErrorReason converts ValidateTokenResponse.error_reason (returned when
// is_valid=false) into a sentinel. Unknown values map to ErrInvalidToken
// (conservative default — the caller should re-authenticate).
// "invalid_token_version" maps to ErrExpired (token rotation due to LogoutAll
// / password change — the token is structurally valid but superseded).
func mapErrorReason(reason string) error {
	switch reason {
	case "expired", "invalid_token_version":
		return ErrExpired
	case "blocked":
		return ErrBlocked
	default: // "invalid", "invalid_signature", "user_not_found", "internal_error", "empty_token", "", any unknown
		return ErrInvalidToken
	}
}
