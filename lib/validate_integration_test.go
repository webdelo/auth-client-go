//go:build integration

package authclient_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	authclient "github.com/webdelo/auth-client-go/lib"
)

func TestClient_ValidateAccessToken_Integration(t *testing.T) {
	// Integration tests do not use t.Parallel — they share the real AUTH server.
	tests := map[string]struct {
		run func(t *testing.T, c *authclient.Client)
	}{
		"happy_path": {
			run: func(t *testing.T, _ *authclient.Client) {
				// Not parallel — shares real AUTH server (UT-0042 exception).
				t.Skip("requires authclient.Login (TASK-004) for token acquisition; see PRP Task 13 TODO")
			},
		},
		"invalid_token_returns_err_invalid_token": {
			run: func(t *testing.T, c *authclient.Client) {
				// Not parallel — shares real AUTH server (UT-0042 exception).
				_, err := c.ValidateAccessToken(context.Background(), "definitely-not-a-real-token")
				require.ErrorIs(t, err, authclient.ErrInvalidToken)
			},
		},
		"expired_token_returns_err_expired": {
			run: func(t *testing.T, _ *authclient.Client) {
				// Not parallel — shares real AUTH server (UT-0042 exception).
				t.Skip("requires waiting >1m for access_token TTL; deferred to TASK-007 release-readiness")
			},
		},
	}

	endpoint := os.Getenv("AUTH_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:50051"
	}

	for name, tc := range tests {
		tc := tc

		t.Run(name, func(t *testing.T) {
			c, err := authclient.New(authclient.Config{Endpoint: endpoint})
			require.NoError(t, err)
			t.Cleanup(func() { _ = c.Close() })
			tc.run(t, c)
		})
	}
}
