package authclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authclient "github.com/webdelo/auth-client-go/lib"
)

func TestSetRefreshCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		token string
		opts  []authclient.CookieOption
		want  func(t *testing.T, c *http.Cookie)
	}{
		"happy_path": {
			token: "abc.def.ghi",
			opts:  nil,
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "refresh_token", c.Name)
				assert.Equal(t, "abc.def.ghi", c.Value)
				assert.Equal(t, "/auth/refresh", c.Path)
				assert.Equal(t, "", c.Domain)
				assert.Equal(t, 14*24*60*60, c.MaxAge)
				assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
				assert.True(t, c.HttpOnly)
				assert.True(t, c.Secure)
			},
		},
		"with_domain": {
			token: "abc.def.ghi",
			opts:  []authclient.CookieOption{authclient.WithCookieDomain(".webdelo.trade")},
			want: func(t *testing.T, c *http.Cookie) {
				// RFC 6265 §5.2.3: Go strips the leading dot when serialising/parsing.
				assert.Equal(t, "webdelo.trade", c.Domain)
				assert.Equal(t, "/auth/refresh", c.Path)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"with_path": {
			token: "abc.def.ghi",
			opts:  []authclient.CookieOption{authclient.WithCookiePath("/refresh")},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "/refresh", c.Path)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"with_max_age": {
			token: "abc.def.ghi",
			opts:  []authclient.CookieOption{authclient.WithCookieMaxAge(7 * 24 * time.Hour)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, 7*24*60*60, c.MaxAge)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"with_same_site_lax": {
			token: "abc.def.ghi",
			opts:  []authclient.CookieOption{authclient.WithCookieSameSite(http.SameSiteLaxMode)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"all_options_combined": {
			token: "abc.def.ghi",
			opts: []authclient.CookieOption{
				authclient.WithCookieDomain(".webdelo.trade"),
				authclient.WithCookiePath("/session/refresh"),
				authclient.WithCookieMaxAge(time.Hour),
				authclient.WithCookieSameSite(http.SameSiteLaxMode),
			},
			want: func(t *testing.T, c *http.Cookie) {
				// RFC 6265 §5.2.3: Go strips the leading dot when serialising/parsing.
				assert.Equal(t, "webdelo.trade", c.Domain)
				assert.Equal(t, "/session/refresh", c.Path)
				assert.Equal(t, 3600, c.MaxAge)
				assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"multiple_path_options_last_wins": {
			token: "abc.def.ghi",
			opts: []authclient.CookieOption{
				authclient.WithCookiePath("/a"),
				authclient.WithCookiePath("/b"),
			},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "/b", c.Path)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"empty_token_value": {
			token: "",
			opts:  nil,
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "", c.Value)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"negative_max_age": {
			token: "abc.def.ghi",
			opts:  []authclient.CookieOption{authclient.WithCookieMaxAge(-time.Second)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, -1, c.MaxAge)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			authclient.SetRefreshCookie(rec, tc.token, tc.opts...)

			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1, "exactly one Set-Cookie header expected")
			tc.want(t, cookies[0])
		})
	}
}

func TestClearRefreshCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		opts []authclient.CookieOption
		want func(t *testing.T, c *http.Cookie)
	}{
		"happy_path": {
			opts: nil,
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "refresh_token", c.Name)
				assert.Equal(t, "", c.Value)
				assert.Equal(t, "/auth/refresh", c.Path)
				assert.Equal(t, "", c.Domain)
				assert.Equal(t, -1, c.MaxAge)
				assert.Equal(t, time.Unix(0, 0).UTC(), c.Expires.UTC())
				assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
				assert.True(t, c.HttpOnly)
				assert.True(t, c.Secure)
			},
		},
		"with_domain_preserved": {
			opts: []authclient.CookieOption{authclient.WithCookieDomain(".webdelo.trade")},
			want: func(t *testing.T, c *http.Cookie) {
				// RFC 6265 §5.2.3: Go strips the leading dot when serialising/parsing.
				assert.Equal(t, "webdelo.trade", c.Domain)
				assert.Equal(t, -1, c.MaxAge)
				assert.Equal(t, "", c.Value)
				assert.Equal(t, int64(0), c.Expires.Unix())
			},
		},
		"with_path_preserved": {
			opts: []authclient.CookieOption{authclient.WithCookiePath("/refresh")},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "/refresh", c.Path)
				assert.Equal(t, -1, c.MaxAge)
				assert.Equal(t, int64(0), c.Expires.Unix())
			},
		},
		"with_max_age_overridden_by_clear": {
			opts: []authclient.CookieOption{authclient.WithCookieMaxAge(7 * 24 * time.Hour)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, -1, c.MaxAge)
				assert.Equal(t, int64(0), c.Expires.Unix())
			},
		},
		"expires_is_epoch_for_legacy_compat": {
			opts: nil,
			want: func(t *testing.T, c *http.Cookie) {
				assert.False(t, c.Expires.IsZero())
				assert.Equal(t, int64(0), c.Expires.Unix())
			},
		},
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			authclient.ClearRefreshCookie(rec, tc.opts...)

			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1, "exactly one Set-Cookie header expected")
			tc.want(t, cookies[0])
		})
	}
}
