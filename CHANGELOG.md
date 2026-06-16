# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.3.0] — 2026-06-16

### Added

- **`(*Client).ConnHealth() ConnHealth`** — a read-only, by-value snapshot of the client's
  underlying gRPC connection for health-endpoint diagnostics. Returns the current
  `connectivity.State` (`ClientConn.GetState()`) and dial `Target` (`ClientConn.Target()`).
  O(1), no network call, safe for concurrent use. Reports
  `{State: connectivity.Shutdown, Target: ""}` when there is no live connection.

  ```go
  h := client.ConnHealth()
  // h.State  — connectivity.State
  // h.Target — dial-target ("host:port")
  ```

- **`ConnHealth` struct** (`{State connectivity.State; Target string}`) — the snapshot value
  type. Designed to be extended additively (e.g. future `LastError`, `LatencyMs` fields).

  Rationale: consumers (notably the funding-bot BFF `GET /api/v1/health` endpoint) need the
  **real** connection state and target of the AUTH dependency to drive an ok/degraded/down
  traffic-light. The snapshot deliberately does **not** expose `*grpc.ClientConn`, preserving
  the SDK's encapsulation and connection lifecycle — external code cannot close or mutate it.
  This is an additive, non-breaking change (minor bump, pre-1.0). The state reflects the
  transport connection only; an active health-probe RPC remains out of scope.

## [v0.2.0] — 2026-05-22

### Changed (BREAKING)

- **Logger contract switched to `*slog.Logger`.** The custom `authclient.Logger`
  interface and `authclient.NoopLogger` type are removed. `Config.Logger` and
  `WithLogger` now accept `*slog.Logger` directly; `nil` collapses to
  `slog.New(slog.DiscardHandler)`. Consumers that previously wrote a
  `slog -> authclient.Logger` adapter should delete that adapter and pass
  their `*slog.Logger` straight in.

  Migration:

  ```go
  // Before (v0.1.0):
  client, _ := authclient.New(cfg, authclient.WithLogger(slogAdapter{l: logger}))

  // After (v0.2.0+):
  client, _ := authclient.New(cfg, authclient.WithLogger(logger))
  ```

  Rationale: `log/slog` has been part of the stdlib since Go 1.21 and is the
  de-facto standard for structured logging. The SDK's custom interface forced
  every consumer to maintain a near-identical adapter for no extension benefit
  — `slog.Handler` is already the swap-point for backends (JSON / text / zap /
  OTLP / custom). Removing the wrapper is a one-line change for consumers and
  drops ~80 LOC of adapter + mock + test from the SDK.

### Removed

- `authclient.Logger` interface (replaced by `*slog.Logger`).
- `authclient.NoopLogger` type (replaced by `slog.New(slog.DiscardHandler)`).
- `lib/mocks/mock_Logger.go` (no longer needed — slog is exercised via a
  recording `slog.Handler` in tests).

## [v0.1.0] — 2026-05-19

Initial release of the Go SDK for [webdelo/auth](https://github.com/webdelo/auth).

### Added

- **Client lifecycle**:
  - `authclient.New(Config, ...Option) (*Client, error)` — constructor with functional options.
  - `(*Client).Close() error` — graceful shutdown (gRPC connection + background goroutines).
  - `Config` with fields: `Endpoint`, `ValidateCacheTTL`, `FailMode`, `ValidateTimeout`,
    `LoginTimeout`, `RefreshTimeout`, `DialOptions`, `Logger`, `Cache`.
  - Options: `WithFailMode`, `WithLogger`, `WithCache`, `WithOpenMethods`.

- **Token validation**:
  - `(*Client).ValidateAccessToken(ctx, accessToken) (User, error)` — gRPC `ValidateToken` with
    positive-result TTL cache (SDK-recommended TTL: `30s`; `Config.ValidateCacheTTL` Go
    zero-value is `0` = cache disabled; key = `sha256(token)`).
  - Built-in in-memory cache (`lib/internal/cache.MemoryCache`) with background sweep;
    swappable via `Cache` interface.

- **Auth flow**:
  - `(*Client).Login(ctx, LoginInput) (LoginOutput, error)`.
  - `(*Client).Refresh(ctx, refreshToken, clientIP, userAgent) (RefreshOutput, error)` —
    singleflight-protected per `sha256(refreshToken)` (intra-process race serialization).
  - `(*Client).Logout(ctx, refreshToken) error`.
  - `(*Client).LogoutAll(ctx, accessToken) error`.
  - `(*Client).ChangePassword(ctx, accessToken, current, new) error`.
  - `(*Client).GetCurrentUser(ctx, accessToken) (User, error)`.

- **Middleware**:
  - `(*Client).HTTPMiddleware(next http.Handler) http.Handler` — Bearer-token check,
    401 / 503 / 429 semantics, `FailMode`-aware.
  - `(*Client).UnaryInterceptor() grpc.UnaryServerInterceptor` — same semantics for gRPC;
    `WithOpenMethods` whitelist; default-open: `/grpc.health.v1.Health/Check`,
    `/grpc.health.v1.Health/Watch`.

- **Context conventions**:
  - `User{UUID, Role, Email}` (Role: `RoleUnknown`, `RoleUser`, `RoleAdmin`).
  - `WithUser(ctx, user) context.Context`, `UserFromContext(ctx) (User, bool)` —
    unexported context key.

- **Cookie helpers**:
  - `SetRefreshCookie(w, token, ...CookieOption)` — defaults: `HttpOnly+Secure+SameSite=Strict+Path=/auth/refresh+MaxAge=14d`.
  - `ClearRefreshCookie(w, ...CookieOption)`.
  - Options: `WithCookieDomain`, `WithCookiePath`, `WithCookieMaxAge`,
    `WithCookieSameSite` (`SameSite=Lax/None` carries explicit SECURITY WARNING in godoc).

- **HTTP request helpers**:
  - `IPFromRequest(r)` — `X-Real-IP` → `X-Forwarded-For[0]` → `RemoteAddr`
    (no trusted-proxies validation in v0.1.0).
  - `UserAgentFromRequest(r)`.

- **Typed errors**:
  - Sentinels: `ErrInvalidToken`, `ErrExpired`, `ErrBlocked`, `ErrRateLimited`, `ErrUnavailable`.
  - `AuthError{Err, RetryAfterHint}` wrapper.
  - `RetryAfter(err) time.Duration` helper for `ErrRateLimited`.

- **Adapter interfaces**:
  - `Logger` (`Info` / `Warn` / `Error`) with slog-style variadic fields; `NoopLogger{}` default.
    *(Replaced by `*slog.Logger` in [Unreleased] — see migration note above.)*
  - `Cache` (`Get` / `Set` / `Delete`) — built-in TTL implementation; consumer can swap.

- **Examples** ([`lib/examples/`](lib/examples/)):
  `basic` (CLI Login→Validate→Logout), `http_middleware` (slog adapter,
  refresh cookie, `/api/profile`), `grpc_interceptor` (Health/List behind auth).

- **Integration testing**: `make env-up` / `env-down` / `test-integration` with `webdelo/auth`
  server in Docker Compose.

### Known limitations

- **Distributed singleflight not supported** — refresh-race protection works only within a
  single SDK process. Multi-replica BFF deployments may trigger AUTH family detection under
  concurrent refresh load.
- **No trusted-proxies validation** in `IPFromRequest` — caller must terminate untrusted
  `X-Forwarded-For` headers at the ingress.
- **No `Metrics` interface** — deferred to v0.2.0.
- **`User.Email` empty after `ValidateAccessToken`** — `ValidateTokenResponse`
  (auth-proto v0.1.0) does not carry it. Use `GetCurrentUser` for the full profile.
- **Cache invalidation lag** — after AUTH bumps `token_version` (LogoutAll / BlockUser /
  ChangePassword), an old access token remains valid in the SDK cache for up to the
  configured `ValidateCacheTTL` (Go zero-value `0` = cache disabled; SDK-recommended
  value `30s`). Set `ValidateCacheTTL: 0` to disable cache entirely.
- **`make release` is interactive** — no `--version=X.Y.Z` flag yet; the release script reads
  from `/dev/tty`.

### Acknowledgements

- Based on the gRPC contract published in
  [`github.com/webdelo/auth-proto`](https://github.com/webdelo/auth-proto) v0.1.0.
- Follows
  [OWASP Session Management Cheat Sheet](https://owasp.org/www-project-cheat-sheets/cheatsheets/Session_Management_Cheat_Sheet.html)
  for cookie defaults.
- Follows
  [OWASP A07:2025 — Authentication Failures](https://owasp.org/Top10/) for fail-closed
  semantics.

[Unreleased]: https://github.com/webdelo/auth-client-go/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/webdelo/auth-client-go/releases/tag/v0.3.0
[v0.2.0]: https://github.com/webdelo/auth-client-go/releases/tag/v0.2.0
[v0.1.0]: https://github.com/webdelo/auth-client-go/releases/tag/v0.1.0
