// Package authclient is the official Go SDK for the webdelo/auth service.
//
// The SDK is a thin wrapper over the gRPC contract from
// github.com/webdelo/auth-proto with non-trivial concerns baked in: a
// ValidateToken TTL cache, an HTTP middleware, a gRPC unary interceptor,
// refresh-race serialization, and typed errors. Create a Client with New,
// dispose with Close.
package authclient

import (
	"errors"
	"fmt"
	"time"

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

// AuthError carries the sentinel error plus optional context (Retry-After for
// rate-limited responses). Use errors.Is(err, ErrXxx) for sentinel detection
// and RetryAfter(err) for the retry hint.
type AuthError struct {
	Err        error
	RetryAfter time.Duration
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

// RetryAfter returns the Retry-After hint for ErrRateLimited errors. Returns
// 0 if err is nil, is not an *AuthError, or the server did not include the
// hint.
func RetryAfter(err error) time.Duration {
	var ae *AuthError
	if errors.As(err, &ae) {
		return ae.RetryAfter
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
		return &AuthError{Err: ErrRateLimited, RetryAfter: retryAfterFromStatus(st)}
	case codes.Unavailable, codes.DeadlineExceeded:
		return ErrUnavailable
	default:
		return fmt.Errorf("authclient: unexpected grpc code %s: %w", st.Code(), ErrUnavailable)
	}
}

// retryAfterFromStatus best-effort extracts the Retry-After hint from gRPC
// status details. AUTH v0.1.0 does not include this field — returns 0 today;
// kept as a future-compat placeholder for google.rpc.RetryInfo.
func retryAfterFromStatus(st *status.Status) time.Duration {
	_ = st
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
func mapErrorReason(reason string) error {
	switch reason {
	case "expired":
		return ErrExpired
	case "blocked":
		return ErrBlocked
	default: // "invalid", "", any unknown
		return ErrInvalidToken
	}
}
