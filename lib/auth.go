package authclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
)

// LoginInput contains the credentials and fingerprint metadata for Login.
//
// ClientIP and UserAgent are forwarded to AUTH for refresh-token fingerprint
// binding (AUTH computes fingerprint = SHA-256(UserAgent + first 3 octets of
// IP)). The caller MUST extract these from the actual end-user HTTP request
// (use IPFromRequest / UserAgentFromRequest helpers — TASK-005), NOT from
// BFF's own connection metadata — otherwise AUTH binds the refresh to BFF's
// IP and the user gets logged out on the next refresh.
type LoginInput struct {
	Email     string
	Password  string
	ClientIP  string
	UserAgent string
}

// LoginOutput is the result of Login. Expiries are converted from absolute
// Unix-seconds (proto contract) to durations relative to time.Now.
//
// User.Email is populated by Login (unlike ValidateAccessToken, which leaves
// it empty due to the ValidateTokenResponse proto contract).
type LoginOutput struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  time.Duration
	RefreshExpiresIn time.Duration
	User             User
}

// RefreshOutput is the result of Refresh. RefreshToken here is the ROTATED
// token — the OLD token passed to Refresh is server-side invalidated. The
// caller MUST replace the stored refresh token with this new value (and
// re-set the refresh cookie via SetRefreshCookie — TASK-005).
type RefreshOutput struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  time.Duration
	RefreshExpiresIn time.Duration
}

// expiresIn converts an absolute Unix-seconds timestamp into a duration
// relative to time.Now. Returns 0 for the proto-default 0 (missing field).
// May return a negative duration for timestamps in the past — caller's
// choice whether to treat as error.
func expiresIn(unixSeconds int64) time.Duration {
	if unixSeconds <= 0 {
		return 0
	}
	return time.Until(time.Unix(unixSeconds, 0))
}

// Login authenticates the user with email+password and returns access+refresh
// tokens plus user info. Forwards ClientIP/UserAgent to AUTH for fingerprint
// binding.
//
// Wraps ctx with cfg.LoginTimeout (default 5s) before the gRPC call.
//
// Returns:
//   - ErrInvalidToken for invalid credentials.
//   - ErrRateLimited when AUTH rate-limits the caller (inspect via
//     RetryAfter helper).
//   - ErrUnavailable when AUTH is unreachable or returns an unexpected code.
//
// Logs success at Info level with user_uuid only — no email, IP, or tokens.
func (c *Client) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.LoginTimeout)
	defer cancel()

	resp, err := c.authClient.Login(callCtx, &auth_v1.LoginRequest{
		Email:     in.Email,
		Password:  in.Password,
		Ip:        in.ClientIP,
		UserAgent: in.UserAgent,
	})
	if err != nil {
		mapped := mapGRPCError(err)
		c.logUnavailableIfNeeded(ctx, "login", mapped)
		return LoginOutput{}, mapped
	}

	out := LoginOutput{
		AccessToken:      resp.GetAccessToken(),
		RefreshToken:     resp.GetRefreshToken(),
		AccessExpiresIn:  expiresIn(resp.GetAccessTokenExpiresAt()),
		RefreshExpiresIn: expiresIn(resp.GetRefreshTokenExpiresAt()),
	}
	if u := resp.GetUser(); u != nil {
		out.User = User{
			UUID:  u.GetUuid(),
			Role:  Role(u.GetRole()),
			Email: u.GetEmail(),
		}
	}

	c.logger.Info(ctx, "authclient: login success", "user_uuid", out.User.UUID)
	return out, nil
}

// Refresh exchanges the given refresh token for a new pair of tokens. Refresh
// is the rotation point: AUTH issues a new refresh_token and invalidates the
// old one. RefreshOutput.RefreshToken is the NEW token — store it and discard
// the previous one.
//
// Concurrent Refresh calls with the same refreshToken within this Client are
// deduplicated via internal singleflight: ONLY ONE RefreshToken RPC reaches
// AUTH; other waiters share the result. This prevents AUTH family detection
// from killing the session when two browser tabs simultaneously refresh.
//
// LIMITATION: dedup is per-process. Multi-replica BFF deployments have no
// cross-instance protection in v0.1.0 (REQ-001-ADR §R3). If you operate
// multi-replica and observe family-detection logouts under load, consider
// sticky sessions or a v0.2.0+ distributed lock.
//
// clientIP and userAgent MUST come from the end-user request (see
// IPFromRequest in TASK-005). AUTH binds the refresh to fingerprint(UA, /24
// IP) — passing BFF's own IP will break the next refresh after IP/UA changes.
//
// Wraps ctx with cfg.RefreshTimeout (default 5s). gRPC retry policy excludes
// RefreshToken (REQ-001-ADR §R11) — failures are returned to the caller
// without retry to avoid burning the refresh chain.
//
// Returns:
//   - ErrInvalidToken when the refresh token is unknown, malformed, or
//     reused (AUTH family detection).
//   - ErrUnavailable when AUTH is unreachable.
//   - ErrRateLimited when rate-limited.
func (c *Client) Refresh(ctx context.Context, refreshToken, clientIP, userAgent string) (RefreshOutput, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.RefreshTimeout)
	defer cancel()

	key := hashToken(refreshToken)
	v, err := c.sf.Do(key, func() (result any, fnErr error) {
		defer func() {
			if r := recover(); r != nil {
				fnErr = fmt.Errorf("authclient: refresh fn panic: %v: %w", r, ErrUnavailable)
				c.logger.Error(ctx, "authclient: refresh fn panic", fnErr, "recover", fmt.Sprint(r))
			}
		}()

		return c.authClient.RefreshToken(callCtx, &auth_v1.RefreshTokenRequest{
			RefreshToken: refreshToken,
			Ip:           clientIP,
			UserAgent:    userAgent,
		})
	})
	if err != nil {
		mapped := mapGRPCError(err)
		c.logUnavailableIfNeeded(ctx, "refresh", mapped)
		return RefreshOutput{}, mapped
	}

	// Type-assert guard: defensive against future refactors that might wrap
	// or transform the singleflight value. Unreachable through the current
	// AuthServiceClient interface (RefreshToken always returns
	// *RefreshTokenResponse). Not exercised by unit tests — covering it
	// would require either an internal API hook or unsafe singleflight
	// override, both of which pollute production code.
	resp, ok := v.(*auth_v1.RefreshTokenResponse)
	if !ok {
		return RefreshOutput{}, fmt.Errorf(
			"authclient: refresh unexpected response type %T: %w",
			v, ErrUnavailable,
		)
	}

	out := RefreshOutput{
		AccessToken:      resp.GetAccessToken(),
		RefreshToken:     resp.GetRefreshToken(),
		AccessExpiresIn:  expiresIn(resp.GetAccessTokenExpiresAt()),
		RefreshExpiresIn: expiresIn(resp.GetRefreshTokenExpiresAt()),
	}
	c.logger.Info(ctx, "authclient: refresh success")
	return out, nil
}

// Logout invalidates the given refresh token at AUTH. The corresponding
// access token remains valid until its natural expiry (ValidateAccessToken
// cache lag — up to cfg.ValidateCacheTTL after server-side invalidation).
// To invalidate ALL of a user's sessions, use LogoutAll.
//
// Wraps ctx with cfg.RefreshTimeout (logout operates on refresh tokens).
//
// Returns ErrInvalidToken if refreshToken is unknown, ErrUnavailable if AUTH
// is unreachable. Logout is idempotent at AUTH — a second logout of an
// already-invalid token returns success or ErrInvalidToken depending on
// server impl; do NOT rely on idempotence.
func (c *Client) Logout(ctx context.Context, refreshToken string) error {
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.RefreshTimeout)
	defer cancel()

	_, err := c.authClient.Logout(callCtx, &auth_v1.LogoutRequest{
		RefreshToken: refreshToken,
	})
	if err != nil {
		mapped := mapGRPCError(err)
		c.logUnavailableIfNeeded(ctx, "logout", mapped)
		return mapped
	}

	c.logger.Info(ctx, "authclient: logout success")
	return nil
}

// LogoutAll invalidates ALL of the user's sessions by bumping the
// token_version at AUTH. All access tokens issued before the bump start
// returning is_valid=false from ValidateToken; the SDK cache picks this up
// after cfg.ValidateCacheTTL (default 30s).
//
// Wraps ctx with cfg.ValidateTimeout (this is a fast DB write at AUTH).
//
// Returns ErrInvalidToken if accessToken is invalid, ErrUnavailable if AUTH
// is down.
func (c *Client) LogoutAll(ctx context.Context, accessToken string) error {
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.ValidateTimeout)
	defer cancel()

	_, err := c.authClient.LogoutAll(callCtx, &auth_v1.LogoutAllRequest{
		AccessToken: accessToken,
	})
	if err != nil {
		mapped := mapGRPCError(err)
		c.logUnavailableIfNeeded(ctx, "logout_all", mapped)
		return mapped
	}

	c.logger.Info(ctx, "authclient: logout_all success")
	return nil
}

// ChangePassword changes the user's password at AUTH. AUTH verifies
// currentPassword (bcrypt — ~100ms) and, on success, bumps token_version,
// invalidating all sessions. The caller MUST re-Login after a successful
// ChangePassword.
//
// Wraps ctx with cfg.LoginTimeout (bcrypt verification — same order as Login).
//
// Returns:
//   - ErrInvalidToken if accessToken is invalid (AUTH returns Unauthenticated).
//   - ErrUnavailable (wrapped) for wrong-current-password / weak-new-password
//     (AUTH returns InvalidArgument; v0.1.0 mapGRPCError does NOT distinguish
//     this from real Unavailable — caller must inspect err.Error() to tell
//     apart). This is a documented limitation; v0.2.0 will add an explicit
//     ErrInvalidPassword sentinel.
//   - ErrUnavailable if AUTH is unreachable.
func (c *Client) ChangePassword(ctx context.Context, accessToken, currentPassword, newPassword string) error {
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.LoginTimeout)
	defer cancel()

	_, err := c.authClient.ChangePassword(callCtx, &auth_v1.ChangePasswordRequest{
		AccessToken:     accessToken,
		CurrentPassword: currentPassword,
		NewPassword:     newPassword,
	})
	if err != nil {
		mapped := mapGRPCError(err)
		c.logUnavailableIfNeeded(ctx, "change_password", mapped)
		return mapped
	}

	c.logger.Info(ctx, "authclient: change_password success")
	return nil
}

// GetCurrentUser returns the user identified by accessToken, INCLUDING email
// (unlike ValidateAccessToken, which leaves email empty due to the
// ValidateTokenResponse proto contract — see REQ-001-ADR §R10).
//
// Use this for UI "my account" pages where email display is needed.
//
// Wraps ctx with cfg.ValidateTimeout (simple lookup at AUTH).
func (c *Client) GetCurrentUser(ctx context.Context, accessToken string) (User, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.ValidateTimeout)
	defer cancel()

	resp, err := c.authClient.GetCurrentUser(callCtx, &auth_v1.GetCurrentUserRequest{
		AccessToken: accessToken,
	})
	if err != nil {
		mapped := mapGRPCError(err)
		c.logUnavailableIfNeeded(ctx, "get_current_user", mapped)
		return User{}, mapped
	}

	u := resp.GetUser()
	if u == nil {
		return User{}, fmt.Errorf("authclient: get_current_user empty user: %w", ErrUnavailable)
	}

	user := User{
		UUID:  u.GetUuid(),
		Role:  Role(u.GetRole()),
		Email: u.GetEmail(),
	}
	c.logger.Info(ctx, "authclient: get_current_user success", "user_uuid", user.UUID)
	return user, nil
}

// logUnavailableIfNeeded emits a Warn log when the mapped error is
// ErrUnavailable. Other errors (invalid/expired/blocked/rate_limited) are
// not logged at SDK level — they are business outcomes for the caller.
func (c *Client) logUnavailableIfNeeded(ctx context.Context, method string, err error) {
	if errors.Is(err, ErrUnavailable) {
		c.logger.Warn(ctx, "authclient: AUTH unavailable", "method", method)
	}
}
