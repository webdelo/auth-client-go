package authclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	authclient "github.com/webdelo/auth-client-go/lib"
)

func TestIPFromRequest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		xRealIP    string
		xff        string
		remoteAddr string
		want       string
	}{
		"happy_path": {
			xRealIP: "203.0.113.1",
			want:    "203.0.113.1",
		},
		"xff_single_ip": {
			xff:  "203.0.113.1",
			want: "203.0.113.1",
		},
		"xff_multi_ip_first_wins": {
			xff:  "203.0.113.1, 10.0.0.1, 10.0.0.2",
			want: "203.0.113.1",
		},
		"xff_with_whitespace_trimmed": {
			xff:  " 203.0.113.1 ",
			want: "203.0.113.1",
		},
		"xff_multi_with_whitespace": {
			xff:  " 203.0.113.1 , 10.0.0.1",
			want: "203.0.113.1",
		},
		"remote_addr_ipv4_with_port": {
			remoteAddr: "192.0.2.1:54321",
			want:       "192.0.2.1",
		},
		"remote_addr_ipv6_with_port": {
			remoteAddr: "[::1]:54321",
			want:       "::1",
		},
		"remote_addr_without_port": {
			remoteAddr: "192.0.2.1",
			want:       "192.0.2.1",
		},
		"all_sources_empty": {
			remoteAddr: "",
			want:       "",
		},
		"x_real_ip_takes_priority_over_xff": {
			xRealIP: "203.0.113.1",
			xff:     "198.51.100.1",
			want:    "203.0.113.1",
		},
		"xff_takes_priority_over_remote_addr": {
			xff:        "203.0.113.1",
			remoteAddr: "192.0.2.1:54321",
			want:       "203.0.113.1",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xRealIP != "" {
				req.Header.Set("X-Real-IP", tc.xRealIP)
			}
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}

			got := authclient.IPFromRequest(req)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUserAgentFromRequest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ua   string
		want string
	}{
		"happy_path": {
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X)",
			want: "Mozilla/5.0 (Macintosh; Intel Mac OS X)",
		},
		"empty_user_agent": {
			ua:   "",
			want: "",
		},
		"unicode_user_agent": {
			ua:   "MyApp/1.0 (📱 iOS)",
			want: "MyApp/1.0 (📱 iOS)",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.ua != "" {
				req.Header.Set("User-Agent", tc.ua)
			}

			got := authclient.UserAgentFromRequest(req)

			assert.Equal(t, tc.want, got)
		})
	}
}
