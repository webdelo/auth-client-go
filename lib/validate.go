package authclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
)

// hashToken returns hex(sha256(token)) — used as the cache key so the raw
// access token never appears in logs, metrics, or heap dumps.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ValidateAccessToken validates accessToken against AUTH and returns the
// authenticated User on success. On cache hit returns the cached User; on
// cache miss issues a gRPC ValidateToken call with cfg.ValidateTimeout.
//
// Positive results are cached for cfg.ValidateCacheTTL; negative results
// (ErrInvalidToken / ErrExpired / ErrBlocked) and transport errors
// (ErrUnavailable, ErrRateLimited) are NOT cached.
//
// Empty accessToken returns ErrInvalidToken without contacting AUTH and
// without consulting the cache.
//
// Returned errors:
//   - ErrInvalidToken — empty token, invalid signature, unknown user.
//   - ErrExpired — token TTL elapsed or token_version bumped at AUTH.
//   - ErrBlocked — user account is blocked.
//   - ErrUnavailable — AUTH unreachable, timeout, or unexpected gRPC code.
//   - ErrRateLimited — AUTH rate-limited this caller.
//
// User.Email is always empty after ValidateAccessToken because the gRPC
// ValidateTokenResponse contract does not carry it. Fetch the full user
// profile via GetCurrentUser when an email is required.
func (c *Client) ValidateAccessToken(ctx context.Context, accessToken string) (User, error) {
	if accessToken == "" {
		return User{}, ErrInvalidToken
	}

	key := hashToken(accessToken)
	if user, ok := c.cache.Get(ctx, key); ok {
		return user, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, c.cfg.ValidateTimeout)
	defer cancel()

	resp, err := c.authClient.ValidateToken(callCtx, &auth_v1.ValidateTokenRequest{
		AccessToken: accessToken,
	})
	if err != nil {
		return User{}, mapGRPCError(err)
	}
	if !resp.IsValid {
		return User{}, mapErrorReason(resp.ErrorReason)
	}

	user := User{
		UUID: resp.UserUuid,
		Role: Role(resp.Role),
		// Email intentionally empty — see godoc above.
	}
	c.cache.Set(ctx, key, user, c.cfg.ValidateCacheTTL)
	return user, nil
}
