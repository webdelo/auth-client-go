# auth-client-go

[![Go Reference](https://pkg.go.dev/badge/github.com/webdelo/auth-client-go.svg)](https://pkg.go.dev/github.com/webdelo/auth-client-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/webdelo/auth-client-go)](https://goreportcard.com/report/github.com/webdelo/auth-client-go)

Go SDK for [webdelo/auth](https://github.com/webdelo/auth). Thin wrapper over the gRPC contract
published in [webdelo/auth-proto](https://github.com/webdelo/auth-proto) with non-trivial
concerns baked in: ValidateToken cache, HTTP middleware, gRPC unary interceptor, refresh-race
serialization, typed errors, and fail-closed-by-default semantics.

## Installation

```bash
go get github.com/webdelo/auth-client-go@v0.1.0
```

For the most recent release use `@latest`. The SDK and its contract dependency
`github.com/webdelo/auth-proto/go` are public; no `GOPRIVATE` or Personal Access Token is
required. The actual AUTH service (`webdelo/auth`) remains private — only the SDK and its
gRPC contract are needed to integrate.

## Quickstart

```go
package main

import (
    "log"
    "net/http"

    authclient "github.com/webdelo/auth-client-go/lib"
)

func main() {
    client, err := authclient.New(authclient.Config{Endpoint: "auth:50051"})
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // The middleware guarantees a user is present, so the ok return is safe to drop.
        user, _ := authclient.UserFromContext(r.Context())
        w.Write([]byte("hello, " + user.UUID))
    })

    http.Handle("/api/me", client.HTTPMiddleware(handler))
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

`User` exposes three small helpers for role checks inside protected handlers:

```go
user, _ := authclient.UserFromContext(r.Context())
if !user.IsAdmin() {
    http.Error(w, "forbidden", http.StatusForbidden)
    return
}
// user.HasRole(authclient.RoleUser) — exact-match role check.
// user.IsZero() — true only if the middleware was skipped (FailOpen on AUTH-down).
```

```bash
curl -H 'Authorization: Bearer <ACCESS_TOKEN>' http://localhost:8080/api/me
# 200 OK  → "hello, <user-uuid>"
# 401     → invalid / expired / missing Bearer
# 503     → AUTH unreachable and FailMode=FailClosed (default)
```

## Configuration

`Config` fields (all optional except `Endpoint`):

| Field              | Default                      | Description                                                                                                |
|--------------------|------------------------------|------------------------------------------------------------------------------------------------------------|
| `Endpoint`         | (required)                   | gRPC host:port of AUTH (e.g. `auth:50051`, `auth.example.com:443`).                                        |
| `ValidateCacheTTL` | `0` (cache disabled)         | Positive-result TTL of the in-memory `ValidateToken` cache. The Go zero-value is `0` — caching is OFF until you set this explicitly. Set to `30 * time.Second` for the SDK-recommended TTL, or to a negative value to apply that recommended default automatically. `0` keeps every call reaching AUTH (`Set` is a no-op for `TTL=0`). |
| `FailMode`         | `FailClosed`                 | Behaviour when AUTH is unavailable — see [Fail Mode](#fail-mode).                                          |
| `ValidateTimeout`  | `2s`                         | Per-call deadline for `ValidateToken` / `LogoutAll` / `GetCurrentUser`.                                    |
| `LoginTimeout`     | `5s`                         | Per-call deadline for `Login` / `ChangePassword` (bcrypt ~100ms upstream).                                 |
| `RefreshTimeout`   | `5s`                         | Per-call deadline for `Refresh` / `Logout`.                                                                |
| `DialOptions`      | `[]grpc.DialOption{}`        | Extra `grpc.Dial` options (TLS credentials, keepalive overrides, interceptors). See [TLS Setup](#tls-setup).|
| `Logger`           | `slog.New(slog.DiscardHandler)` | A `*slog.Logger` for diagnostic logs. The SDK uses `log/slog` from stdlib directly — no custom interface. See [Logging](#logging). |
| `Cache`            | built-in `MemoryCache`       | Adapter for `ValidateToken` result cache.                                                                  |

Functional options (applied after `Config`):

| Option                       | Purpose                                                                                       |
|------------------------------|-----------------------------------------------------------------------------------------------|
| `WithFailMode(mode)`         | Overrides `Config.FailMode`. See [Fail Mode](#fail-mode) — `FailOpen` is auth bypass.         |
| `WithLogger(l)`              | Overrides `Config.Logger`. `nil` collapses to `slog.New(slog.DiscardHandler)`.                |
| `WithCache(c)`               | Overrides `Config.Cache` with a custom implementation. `nil` is a no-op (falls through to `Config.Cache` or the built-in `MemoryCache`). To disable caching set `Config.ValidateCacheTTL = 0`. |
| `WithOpenMethods(methods…)`  | gRPC interceptor — whitelist of full method names that skip auth (e.g. health probes).        |

## Fail Mode

`FailMode` controls what the HTTP middleware and gRPC interceptor do when AUTH is unreachable
or returns transient errors.

- `FailClosed` (default) — return `503 Service Unavailable` (HTTP) /
  `codes.Unavailable` (gRPC). The downstream handler is **never** called. This is the
  OWASP-recommended behaviour ([A07:2025 — Authentication Failures](https://owasp.org/Top10/)).
- `FailOpen` — pass the request through with an empty `User{}` in context. The middleware
  calls `WithUser(ctx, User{})`, so `UserFromContext` returns `(User{}, true)` — the
  boolean is `true` because the key is present. Use `user.IsZero()` to detect the
  FailOpen pass-through:

  ```go
  user, _ := authclient.UserFromContext(r.Context())
  if user.IsZero() {
      // AUTH was unreachable and FailOpen is configured; treat as unauthenticated.
      http.Error(w, "unauthorized", http.StatusUnauthorized)
      return
  }
  ```

> ⚠️ **SECURITY WARNING — FAILOPEN MODE**
>
> WHEN AUTH IS UNAVAILABLE AND FAILOPEN IS SET, EVERY PROTECTED ENDPOINT WILL EXECUTE WITHOUT
> AUTHENTICATION. THIS IS AN AUTH BYPASS BY DESIGN AND VIOLATES OWASP A07. USE ONLY ON HEALTH /
> PROBE ENDPOINTS, NEVER ON ENDPOINTS HANDLING USER DATA, BILLING, OR PRIVILEGED OPERATIONS.

Configure via `Config.FailMode` or `WithFailMode(authclient.FailOpen)`. For gRPC, the safer way
to expose unauthenticated probe RPCs is `WithOpenMethods("/grpc.health.v1.Health/Check", …)` —
those methods skip auth entirely without flipping the global failure semantics.

## Typed Errors

All client methods return one of the SDK sentinels. Use `errors.Is` to branch.

| SDK sentinel        | gRPC code from AUTH         | HTTP status (middleware) | What the caller should do                                                |
|---------------------|------------------------------|--------------------------|--------------------------------------------------------------------------|
| `ErrInvalidToken`   | `Unauthenticated`            | `401`                    | Re-authenticate (`Login`).                                               |
| `ErrExpired`        | `Unauthenticated` w/ reason  | `401`                    | Call `Refresh` to mint a new access token.                               |
| `ErrBlocked`        | `PermissionDenied`           | `401`                    | Display "account blocked" — re-auth will not help.                       |
| `ErrRateLimited`    | `ResourceExhausted`          | `429` + `Retry-After`    | Wait `authclient.RetryAfter(err)` before retrying.                       |
| `ErrUnavailable`    | `Unavailable` / deadline     | `503` (FailClosed)       | Retry with backoff; surface to caller as transient.                      |

Example:

```go
out, err := client.Login(ctx, authclient.LoginInput{
    Email: email, Password: pw,
    ClientIP: authclient.IPFromRequest(r),
    UserAgent: authclient.UserAgentFromRequest(r),
})
switch {
case errors.Is(err, authclient.ErrInvalidToken):
    http.Error(w, "invalid credentials", http.StatusUnauthorized)
case errors.Is(err, authclient.ErrRateLimited):
    w.Header().Set("Retry-After", authclient.RetryAfter(err).Round(time.Second).String())
    http.Error(w, "slow down", http.StatusTooManyRequests)
case errors.Is(err, authclient.ErrUnavailable):
    http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
case err != nil:
    http.Error(w, "internal", http.StatusInternalServerError)
default:
    authclient.SetRefreshCookie(w, out.RefreshToken)
    // emit out.AccessToken to the body / response header
}
```

## Logging

The SDK uses `log/slog` from the Go standard library directly — no custom logger
interface, no adapter required. Pass any `*slog.Logger` and the SDK calls the
`*Context` variants (`InfoContext` / `WarnContext` / `ErrorContext`) so your
handlers see the request context.

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
client, _ := authclient.New(authclient.Config{
    Endpoint: "auth:50051",
    Logger:   logger,
})

// Or via option:
client, _ = authclient.New(cfg, authclient.WithLogger(slog.Default()))
```

The default when `Config.Logger == nil` is `slog.New(slog.DiscardHandler)` — silent,
zero allocations per call. To wire in a different backend (zap, zerolog, OpenTelemetry,
file, syslog), use any `slog.Handler` implementation: the SDK does not care which.

The SDK never logs token contents, passwords, or full IP addresses (only `user_uuid`
and operation name).

## Adapter Interfaces

The SDK has one consumer-facing adapter interface: `Cache`. Logging is handled via
the standard `*slog.Logger` (see [Logging](#logging) above), and the SDK does not
provide a metrics adapter in this release.

### Cache

```go
type Cache interface {
    Get(ctx context.Context, key string) (user User, ok bool)
    Set(ctx context.Context, key string, user User, ttl time.Duration)
    Delete(ctx context.Context, key string)
}
```

Default is the built-in `lib/internal/cache.MemoryCache` — an in-process TTL map with periodic
sweep. For multi-replica deployments where a single warm cache across replicas matters, plug
in a Redis-backed implementation:

```go
client, _ := authclient.New(cfg, authclient.WithCache(myRedisCache))
```

Cache keys are `sha256(accessToken)`; the raw token never reaches the cache. `Set` must be a
no-op when `ttl <= 0` (see `Config.ValidateCacheTTL = 0` to disable caching).

## Cookie Helpers

`SetRefreshCookie` / `ClearRefreshCookie` produce `Set-Cookie` headers with hardened defaults:

| Property   | Default                       | Override via                     |
|------------|-------------------------------|----------------------------------|
| `Name`     | `refresh_token`               | (not configurable)               |
| `HttpOnly` | `true`                        | (always asserted; not overridable) |
| `Secure`   | `true`                        | (always asserted; not overridable) |
| `SameSite` | `http.SameSiteStrictMode`     | `WithCookieSameSite`             |
| `Path`     | `/auth/refresh`               | `WithCookiePath`                 |
| `MaxAge`   | `14 * 24h`                    | `WithCookieMaxAge`               |
| `Domain`   | `""` (host-only)              | `WithCookieDomain`               |

`HttpOnly` and `Secure` are intentionally not exposed as options — flipping them off is a
security regression. For local development without TLS, run behind a `localhost` reverse proxy
that handles TLS termination.

Multi-domain example (BFF on `app.example.com` calling AUTH on `auth.example.com`):

```go
authclient.SetRefreshCookie(w, out.RefreshToken,
    authclient.WithCookieDomain(".example.com"),
    authclient.WithCookiePath("/"),
)

// On logout, mirror the same options or the browser keeps the cookie:
authclient.ClearRefreshCookie(w,
    authclient.WithCookieDomain(".example.com"),
    authclient.WithCookiePath("/"),
)
```

## HTTP Request Helpers

```go
ip := authclient.IPFromRequest(r)   // X-Real-IP → first X-Forwarded-For → RemoteAddr
ua := authclient.UserAgentFromRequest(r)
out, err := client.Login(ctx, authclient.LoginInput{
    Email: email, Password: pw, ClientIP: ip, UserAgent: ua,
})
```

**Why fingerprint binding goes through the SDK and not BFF's own IP.** AUTH computes the
refresh-token fingerprint as `SHA-256(UA || first-3-octets(client_IP))`. Passing BFF's own
connection IP would bind the refresh to BFF, so the very next refresh from the user (with a
different upstream IP / UA) breaks. Always forward what the user sent.

**Spoofing caveat.** `IPFromRequest` trusts `X-Real-IP` and `X-Forwarded-For` from the
incoming request. Deploy behind an ingress (Nginx, ALB, Cloudflare) that strips those headers
from untrusted clients before passing the request through. v0.1.0 does **not** ship a
trusted-proxies allowlist — that is planned for v0.2.0.

## Connection Health

`(*Client).ConnHealth()` returns a read-only, by-value snapshot of the underlying gRPC
connection — its current `connectivity.State` and dial `Target`. It is O(1), performs **no**
network call, and is safe for concurrent use. The snapshot intentionally does **not** expose
`*grpc.ClientConn`, so callers cannot close or mutate the connection.

```go
import "google.golang.org/grpc/connectivity"

h := client.ConnHealth()
// h.State  — connectivity.State (Idle / Connecting / Ready / TransientFailure / Shutdown)
// h.Target — dial-target the client connects to ("host:port")
```

When the client has no live connection (`grpcConn == nil`), `ConnHealth` reports
`{State: connectivity.Shutdown, Target: ""}`.

**Health-endpoint integration** — map the connection state to a traffic-light status (the
pattern a BFF `GET /api/v1/health` uses across its gRPC dependencies):

```go
func authStatus(client *authclient.Client) (string, string) {
    h := client.ConnHealth()
    switch h.State {
    case connectivity.Ready, connectivity.Idle:
        return "ok", h.Target
    case connectivity.Connecting:
        return "degraded", h.Target
    default: // TransientFailure, Shutdown
        return "down", h.Target
    }
}
```

> **Connection-level only.** `ConnHealth` reflects the transport state, not the result of an
> active health-probe RPC against the AUTH server. An explicit `Ping(ctx)` probe is out of
> scope for now and may be added later as a separate, non-breaking method.

## Examples

Runnable, end-to-end samples live under [`lib/examples/`](lib/examples/):

- [`lib/examples/basic/`](lib/examples/basic/) — CLI: `Login` → `ValidateAccessToken` →
  `Logout`. Reads credentials from environment variables.
- [`lib/examples/http_middleware/`](lib/examples/http_middleware/) — protected HTTP server with
  `/auth/login`, `/auth/logout`, `/api/profile`. Uses an slog adapter and `SetRefreshCookie`
  with `IPFromRequest` / `UserAgentFromRequest`.
- [`lib/examples/grpc_interceptor/`](lib/examples/grpc_interceptor/) — protected gRPC server
  with `WithOpenMethods` for health probes.

Build them all locally: `make build-examples` (binaries land in `.temp/examples/`).

## Local Integration Testing

Integration tests run the SDK against a real `webdelo/auth` gRPC server in Docker.

**Prerequisites:**

- Docker + the docker-compose plugin available.
- `webdelo/auth` repo cloned as a sibling — `dev/deployments/compose.test.yaml` builds the
  auth image from `../../../auth` relative to the compose file. Override the location with
  `AUTH_REPO_PATH=/abs/path/to/auth make env-up`.

```bash
make env-up              # Build AUTH image, start Postgres + migrate + auth, wait for healthy.
make env-down            # Tear down and delete volumes.
make test-integration    # env-up → run ./... -tags=integration → env-down (always).
```

`make test-integration` always tears the environment down, even on test failure. If the run is
interrupted (`Ctrl+C`) or `env-up` errors out partway, run `make env-down` manually to clean
up leftover containers.

## TLS Setup

The SDK dials AUTH over **insecure** transport by default — this is appropriate for
single-host dev deployments and for service-mesh setups where mTLS is terminated by sidecars.

For direct production deployments (BFF talking to AUTH over the public internet or an
untrusted network), supply your own transport credentials via `DialOptions`:

```go
client, err := authclient.New(authclient.Config{
    Endpoint: "auth.example.com:443",
    DialOptions: []grpc.DialOption{
        grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
            ServerName: "auth.example.com",
            MinVersion: tls.VersionTLS12,
        })),
    },
})
```

Consumer-supplied transport credentials override the SDK default — the SDK does not double-wrap.

## Troubleshooting

### Debugging 401 responses

1. Plug in a real `*slog.Logger` (see [Logging](#logging)) and re-run — the SDK logs
   the operation (`validate` / `login` / `refresh` / …) and the mapped sentinel.
2. Inspect `WWW-Authenticate` on the 401:
   - `Bearer realm="api"` — the `Authorization` header was missing or malformed.
   - `Bearer realm="api", error="invalid_token"` — AUTH rejected the token.
3. Cross-check by calling `client.ValidateAccessToken(ctx, token)` directly and printing
   `err` — you get the typed sentinel (`ErrInvalidToken` / `ErrExpired` / `ErrBlocked`).

### Fingerprint caveats (NAT, CGNAT, /24 binding)

Refresh-token fingerprint = `SHA-256(UA || first-3-octets(IP))`. Concretely:

- Refresh issued to `203.0.113.55` keeps working from `203.0.113.99` (same `/24`).
- It does **not** work from `203.0.114.55` (different `/24`).
- Corporate NAT typically rolls users through a single `/24` — fine.
- CGNAT can rotate users between `/24`s — expect periodic refresh failures; clients fall
  back to `Login`.

Mitigation if your trust model allows: harden the fingerprint upstream (e.g., bind to a full
IPv6 address) by sending custom values to `Login` / `Refresh` instead of `IPFromRequest`.

### gRPC keepalive auto-bump

`grpc-go` silently bumps `keepalive.ClientParameters.Time` to 10s when a smaller value is
configured. The SDK ships `Time: 30s` which is safe. If you override `DialOptions` with your
own `WithKeepaliveParams(...)`, keep `Time >= 10s` to avoid the silent bump.

### Distributed refresh-race protection is not supported

`Refresh` is singleflight-deduplicated **only within a single process**. Two BFF replicas
hitting AUTH with the same refresh token simultaneously may trigger AUTH family detection and
log the user out. Mitigations:

- Sticky sessions in the load balancer keyed on the refresh cookie.
- Single-replica BFF for low-volume deployments.
- Distributed singleflight via Redis — planned for v0.2.0.

## Known Limitations

- **No distributed singleflight** — see Troubleshooting above.
- **No trusted-proxies validation** in `IPFromRequest` — spoofing risk behind an untrusted
  reverse proxy. Plan: `WithTrustedProxies` in v0.2.0.
- **No `Metrics` interface** — planned for v0.2.0.
- **`User.Email` is empty after `ValidateAccessToken`** — `auth-proto` v0.1.0 omits it from
  `ValidateTokenResponse`. Use `GetCurrentUser` for the full profile.
- **Cache invalidation lag** — after AUTH bumps `token_version`
  (`LogoutAll` / `BlockUser` / `ChangePassword`), an old access token remains valid in the
  SDK cache for up to the configured `ValidateCacheTTL` (the Go zero-value is `0` /
  cache disabled; the SDK-recommended value is `30s`). Setting `Config.ValidateCacheTTL = 0`
  disables the cache entirely (every call reaches AUTH).
- **`make release` is interactive** — reads version from `/dev/tty`. No `--version=X.Y.Z`
  flag yet.

## Versioning Policy

This SDK follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

- **Pre-v1.0.0**: breaking changes are allowed but must be flagged in `CHANGELOG.md`.
- **Post-v1.0.0**: breaking changes only on major bumps (`v2.0.0`). Within a major:
  - **Frozen**: method signatures on `*Client`, names / values of `Role` and `FailMode`
    constants, sentinel error names, exported struct field names and types, `Cache`
    method signatures, the `*slog.Logger` shape of `Config.Logger` and `WithLogger`,
    cookie defaults.
  - **Additive**: new methods, new `Role` / `FailMode` constants, new sentinel errors, new
    struct fields (zero-value-compatible), new `With…` options, future `Metrics` interface.

## Development

### Prerequisites

- Go 1.24+
- golangci-lint v2.11.4
- (For integration tests only) Docker + docker-compose plugin
- (For integration tests only) `webdelo/auth` repo cloned as sibling: `../auth` relative to
  this repo.

### Commands

```bash
make help                # Show all available commands
make test                # Run unit tests
make test-race           # Run unit tests with race detector
make coverage            # Generate coverage report
make lint                # Run linter
make audit               # vet + lint + test
make build-examples      # Build example programs
make tidy                # go mod tidy
```

### CI

Integration tests are an opt-in job triggered via `workflow_dispatch` with
`run_integration=true`:

```bash
gh workflow run ci.yaml -f run_integration=true
```

This job has `continue-on-error: true` — it does not block PR merges. The auth image is not yet
published to GHCR (tracked separately under a DevOps REQ); for now the CI job checks out the
`webdelo/auth` repo and builds the image on every run.

## License

Internal — webdelo organization.
