package authclient

import (
	"net/http"
	"time"
)

const (
	defaultRefreshCookieName   = "refresh_token"
	defaultRefreshCookiePath   = "/auth/refresh"
	defaultRefreshCookieMaxAge = 14 * 24 * 60 * 60 // 14 days in seconds, synced with AUTH refresh-token TTL.
)

// CookieOption customizes the refresh-cookie produced by SetRefreshCookie and
// ClearRefreshCookie. Options run AFTER the SDK defaults are applied; the
// SDK then re-asserts HttpOnly=true and Secure=true so options cannot
// silently disable security attributes.
//
// Multiple options targeting the same field apply in call order (last wins).
type CookieOption func(*http.Cookie)

// WithCookieDomain sets Cookie.Domain. Use "" (default) for host-only cookies
// (recommended). Use "example.com" or ".example.com" to share between
// subdomains (e.g., when BFF on app.example.com and AUTH on auth.example.com).
func WithCookieDomain(domain string) CookieOption {
	return func(c *http.Cookie) { c.Domain = domain }
}

// WithCookiePath overrides the default "/auth/refresh" Path. Narrowest path
// minimises attack surface — cookie is sent only when the browser requests
// that path. Wider paths (e.g., "/") expose the cookie to every endpoint.
func WithCookiePath(path string) CookieOption {
	return func(c *http.Cookie) { c.Path = path }
}

// WithCookieMaxAge overrides the default 14-day MaxAge. Duration is converted
// to integer seconds via maxAge.Seconds(); fractional seconds are truncated.
// Negative durations expire the cookie immediately — use ClearRefreshCookie
// to delete instead of passing a negative duration here.
func WithCookieMaxAge(maxAge time.Duration) CookieOption {
	return func(c *http.Cookie) { c.MaxAge = int(maxAge.Seconds()) }
}

// WithCookieSameSite overrides the default SameSite=Strict. Lax weakens CSRF
// protection — only use when SPA requires cross-site initial cookie delivery
// (typically when login completes on a different origin and redirects).
func WithCookieSameSite(mode http.SameSite) CookieOption {
	return func(c *http.Cookie) { c.SameSite = mode }
}

// SetRefreshCookie writes a Set-Cookie header with the refresh token. Defaults:
//   - Name:     "refresh_token"
//   - HttpOnly: true (always; not overridable)
//   - Secure:   true (always; not overridable)
//   - SameSite: Strict
//   - Path:     "/auth/refresh"
//   - MaxAge:   14 days
//   - Domain:   "" (host-only)
//
// HttpOnly and Secure are deliberately not exposed via CookieOption — disabling
// them is a security regression. If you need to test locally without TLS,
// provide a custom transport that strips Secure; do NOT add a SDK option.
//
// Path defaults to "/auth/refresh" so the cookie is sent only on refresh calls
// and is NOT exposed to /api/* or other endpoints (reduces XSS impact and
// avoids server-side accidental logging of the refresh token).
func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption) {
	http.SetCookie(w, newRefreshCookie(refreshToken, opts))
}

// ClearRefreshCookie writes a Set-Cookie header that instructs the browser to
// delete the refresh cookie set by SetRefreshCookie. Path and Domain are
// inherited from the same defaults (and the same opts) — they MUST match the
// original SetRefreshCookie call, otherwise RFC 6265 cookie identity does not
// match and the browser keeps the cookie.
//
// If SetRefreshCookie was called with WithCookieDomain("example.com"), call
// ClearRefreshCookie with the same WithCookieDomain.
func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption) {
	cookie := newRefreshCookie("", opts)
	cookie.MaxAge = -1
	cookie.Value = ""
	// Expires=epoch is set in addition to MaxAge=-1 for RFC 6265 §5.3 compat
	// with legacy clients that ignore MaxAge but honour an Expires in the past.
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, cookie)
}

// newRefreshCookie builds an http.Cookie with SDK defaults, applies opts, then
// re-asserts HttpOnly=true and Secure=true. This ordering is the security
// invariant: any option attempting to disable HttpOnly/Secure is silently
// overridden. The CookieOption type intentionally lacks WithCookieInsecure /
// WithCookieAllowJS — disabling these is forbidden by the SDK contract.
func newRefreshCookie(value string, opts []CookieOption) *http.Cookie {
	cookie := &http.Cookie{
		Name:     defaultRefreshCookieName,
		Value:    value,
		Path:     defaultRefreshCookiePath,
		MaxAge:   defaultRefreshCookieMaxAge,
		SameSite: http.SameSiteStrictMode,
	}
	for _, opt := range opts {
		opt(cookie)
	}
	cookie.HttpOnly = true
	cookie.Secure = true
	return cookie
}
