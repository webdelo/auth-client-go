package authclient

import (
	"net"
	"net/http"
	"strings"
)

// IPFromRequest extracts the client IP from r using a best-effort fallback
// chain suitable for production deployments behind a trusted reverse proxy:
//
//  1. X-Real-IP header (if non-empty after whitespace trimming and
//     net.ParseIP validation). Set by Nginx/Cloudflare/ALB to the
//     immediate client.
//  2. First entry of X-Forwarded-For (the original client per RFC 7239
//     convention). Whitespace is trimmed and the result is validated via
//     net.ParseIP. If invalid, falls through to the next source.
//  3. Host part of r.RemoteAddr extracted via net.SplitHostPort (handles
//     IPv4 "1.2.3.4:5678" → "1.2.3.4" and IPv6 "[::1]:5678" → "::1").
//  4. r.RemoteAddr verbatim if SplitHostPort errors (e.g., RemoteAddr lacks
//     a port, which Go's HTTP server doesn't normally produce but unit tests
//     might).
//
// X-Real-IP and XFF entries are whitespace-trimmed and parsed via
// net.ParseIP; syntactically invalid values (e.g., "not-an-ip", empty string
// after trim) are silently skipped and the function falls through to the next
// source. The result is either a syntactically valid IP address (from headers
// or RemoteAddr) or the raw r.RemoteAddr when SplitHostPort errors.
//
// Returns "" when all sources are empty.
//
// SECURITY: This function is spoofable in the absence of a trusted proxy.
// X-Real-IP and X-Forwarded-For are user-controllable. Deploy auth-client-go
// behind a reverse proxy that strips client-supplied values for these headers
// and sets them itself; otherwise a malicious client can inject a syntactically
// valid but spoofed IP (e.g., "1.2.3.4") and circumvent the AUTH fingerprint
// check. net.ParseIP rejects only syntactically invalid strings; a valid but
// spoofed IP still requires a trusted proxy stripping the header.
//
// v0.1.0 does not validate against a trusted-proxy list (planned as
// WithTrustedProxies in v0.2.0 — see REQ-001-ADR §A12).
func IPFromRequest(r *http.Request) string {
	if v := r.Header.Get("X-Real-IP"); v != "" {
		if ip := strings.TrimSpace(v); net.ParseIP(ip) != nil {
			return ip
		}
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); net.ParseIP(ip) != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// UserAgentFromRequest returns r.UserAgent(). Wrapper exists for API symmetry
// with IPFromRequest — callers pass IP and UA together to Login/Refresh, so a
// matching pair of helpers is clearer than mixing styles.
//
// Returns "" when the User-Agent header is absent.
func UserAgentFromRequest(r *http.Request) string {
	return r.UserAgent()
}
