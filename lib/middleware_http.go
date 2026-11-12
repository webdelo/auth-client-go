package authclient

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
)

const (
	bearerPrefix   = "Bearer "
	maxBearerLen   = 8192
	retryAfterUnav = "5"
	wwwAuthMissing = `Bearer realm="api"`
	wwwAuthInvalid = `Bearer realm="api", error="invalid_token"`
)

// bearerToken extracts the token from an Authorization header value. The
// scheme match is case-insensitive (RFC 7235). The token must be non-empty,
// must not contain inner whitespace, and must be ≤ 8 KiB.
func bearerToken(authz string) (string, bool) {
	authz = strings.TrimLeft(authz, " \t")
	if len(authz) < len(bearerPrefix) || len(authz) > len(bearerPrefix)+maxBearerLen+1 ||
		!strings.EqualFold(authz[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}

	token := strings.TrimSpace(authz[len(bearerPrefix):])
	if token == "" ||
		len(token) > maxBearerLen ||
		strings.ContainsAny(token, " \t\r\n") {
		return "", false
	}

	return token, true
}

// HTTPMiddleware wraps next with an Authorization Bearer-token check. It
// validates the token via ValidateAccessToken and either:
//   - calls next with WithUser(ctx, user) on success;
//   - 401 with WWW-Authenticate when the header is missing/malformed or the
//     token is invalid/expired/blocked;
//   - 503 with Retry-After: 5 when AUTH is unavailable and FailMode == FailClosed;
//   - calls next with WithUser(ctx, User{}) when AUTH is unavailable and
//     FailMode == FailOpen (logs a warning);
//   - 429 with Retry-After: N (omitted if N == 0) on ErrRateLimited.
//
// The middleware always writes a response body via http.Error for non-2xx
// cases (Content-Type: text/plain; charset=utf-8). Every error branch returns
// without invoking next.
func (c *Client) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			w.Header().Set("WWW-Authenticate", wwwAuthMissing)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := c.ValidateAccessToken(r.Context(), token)
		switch {
		case errors.Is(err, ErrUnavailable):
			if c.failMode == FailOpen {
				c.logger.WarnContext(r.Context(), "authclient: AUTH unavailable, fail-open pass-through")
				next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), User{})))
				return
			}
			w.Header().Set("Retry-After", retryAfterUnav)
			http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
			return

		case errors.Is(err, ErrRateLimited):
			if d := RetryAfter(err); d > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.Seconds()))))
			}
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return

		case err != nil: // ErrInvalidToken / ErrExpired / ErrBlocked
			w.Header().Set("WWW-Authenticate", wwwAuthInvalid)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}
