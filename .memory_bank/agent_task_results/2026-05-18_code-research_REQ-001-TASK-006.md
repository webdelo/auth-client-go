# Code Research Report: REQ-001-TASK-006 Examples

## metadata
- date: "2026-05-18"
- researcher: "code-researcher"
- query: "Research public API surface for three auth-client-go SDK examples (basic, http_middleware, grpc_interceptor)"
- task_slug: "REQ-001-TASK-006"

---

## 1. EXISTING IMPLEMENTATION (TASK-002 — already in code)

### lib/client.go
**Package:** `authclient` (`github.com/webdelo/auth-client-go/lib`)

**Config struct** (all fields):
```go
type Config struct {
    Endpoint         string
    ValidateCacheTTL time.Duration  // default 30s; 0 = cache disabled
    FailMode         FailMode       // default FailClosed
    ValidateTimeout  time.Duration  // default 2s
    LoginTimeout     time.Duration  // default 5s
    RefreshTimeout   time.Duration  // default 5s
    DialOptions      []grpc.DialOption
    Logger           Logger
    Cache            Cache
}
```

**FailMode constants:**
```go
type FailMode uint8
const (
    FailClosed FailMode = iota  // 0 — 503 on AUTH unavailable
    FailOpen                    // 1 — pass-through with empty User
)
```

**Cache interface** (as defined in TASK-002 — will be UPDATED in TASK-003 to use `User` not `any`):
```go
type Cache interface {
    Get(ctx context.Context, key string) (user any, ok bool)
    Set(ctx context.Context, key string, user any, ttl time.Duration)
    Delete(ctx context.Context, key string)
}
```

**Option type:** `type Option func(*Client)`

**Constructors and methods:**
```go
func New(cfg Config, opts ...Option) (*Client, error)
func (c *Client) Close() error  // idempotent
```

**Functional options:**
```go
func WithFailMode(mode FailMode) Option
func WithLogger(l Logger) Option
func WithCache(cache Cache) Option
func WithOpenMethods(methods ...string) Option  // adds to always-open registry
```

**Default open methods** (gRPC interceptor bypass — hardcoded):
- `/grpc.health.v1.Health/Check`
- `/grpc.health.v1.Health/Watch`

### lib/errors.go

**Sentinel errors:**
```go
var ErrInvalidToken = errors.New("authclient: invalid token")
var ErrExpired      = errors.New("authclient: token expired")
var ErrBlocked      = errors.New("authclient: user blocked")
var ErrRateLimited  = errors.New("authclient: rate limited")
var ErrUnavailable  = errors.New("authclient: service unavailable")
```

**AuthError type:**
```go
type AuthError struct {
    Err        error
    RetryAfter time.Duration
}
func (e *AuthError) Error() string
func (e *AuthError) Unwrap() error
```

**Helper:**
```go
func RetryAfter(err error) time.Duration  // returns 0 if not *AuthError or no hint
```

### lib/logger.go

**Logger interface (exactly):**
```go
type Logger interface {
    Info(ctx context.Context, msg string, fields ...any)
    Warn(ctx context.Context, msg string, fields ...any)
    Error(ctx context.Context, msg string, err error, fields ...any)
}
```
Note: `Error` has a dedicated `err error` param between msg and fields — NOT the same shape as `slog.Logger`. The slog adapter in examples must bridge this.

**NoopLogger:**
```go
type NoopLogger struct{}
func (NoopLogger) Info(context.Context, string, ...any)       {}
func (NoopLogger) Warn(context.Context, string, ...any)       {}
func (NoopLogger) Error(context.Context, string, error, ...any) {}
```

---

## 2. TASK-003 API (ValidateAccessToken + HTTPMiddleware + UnaryInterceptor + context.go)

### lib/context.go (new file)

```go
type Role string

const (
    RoleUnknown Role = ""
    RoleUser    Role = "user"
    RoleAdmin   Role = "admin"
)

type User struct {
    UUID  string
    Role  Role
    Email string  // empty after ValidateAccessToken; set by Login/GetCurrentUser
}

func (u User) IsZero() bool              { return u.UUID == "" }
func (u User) IsAdmin() bool             { return u.Role == RoleAdmin }
func (u User) HasRole(role Role) bool    { return u.Role == role }

type userCtxKey struct{}  // unexported

func WithUser(ctx context.Context, user User) context.Context
func UserFromContext(ctx context.Context) (User, bool)  // comma-ok
```

### lib/validate.go (new file)

```go
// hashToken is private: hex(sha256(token))
func (c *Client) ValidateAccessToken(ctx context.Context, accessToken string) (User, error)
```
- Empty `accessToken` → immediate `ErrInvalidToken` (no gRPC call).
- Cache-key = `hashToken(accessToken)`.
- Positive results cached for `cfg.ValidateCacheTTL`; negative never cached.
- `User.Email` is always `""` after this call.

### lib/middleware_http.go (new file)

```go
func (c *Client) HTTPMiddleware(next http.Handler) http.Handler
```
- Extracts `Bearer` token from `Authorization` header (case-insensitive prefix, max 8192 chars).
- Missing/malformed → `401` + `WWW-Authenticate: Bearer realm="api"`.
- `ErrUnavailable` + `FailClosed` → `503` + `Retry-After: 5`.
- `ErrUnavailable` + `FailOpen` → calls `next` with `WithUser(ctx, User{})`.
- `ErrRateLimited` → `429` + optional `Retry-After: N`.
- Other errors (Invalid/Expired/Blocked) → `401` + `WWW-Authenticate: Bearer realm="api", error="invalid_token"`.
- Success → calls `next` with `r.WithContext(WithUser(r.Context(), user))`.

### lib/interceptor_grpc.go (new file)

```go
func (c *Client) UnaryInterceptor() grpc.UnaryServerInterceptor
```
- Uses named return `(resp interface{}, err error)` for panic recovery.
- `defer func() { if r := recover(); r != nil { err = status.Errorf(codes.Internal, ...) } }()` — covers handler AND auth-check panics.
- Methods in `c.openMethods` → pass through directly.
- Reads `"authorization"` from gRPC metadata (lowercase key).
- Missing metadata → `codes.Unauthenticated`.
- Missing/malformed Bearer → `codes.Unauthenticated`.
- `ErrUnavailable` + `FailClosed` → `codes.Unavailable`.
- `ErrUnavailable` + `FailOpen` → calls `handler(WithUser(ctx, User{}), req)`.
- `ErrRateLimited` → `codes.ResourceExhausted`.
- Other errors → `codes.Unauthenticated`.
- Success → `handler(WithUser(ctx, user), req)`.

---

## 3. TASK-004 API (Login / Refresh / Logout / LogoutAll / ChangePassword / GetCurrentUser)

### lib/auth.go (new file)

**Input/output types:**
```go
type LoginInput struct {
    Email     string
    Password  string
    ClientIP  string  // from IPFromRequest(r) — REAL client IP, not BFF
    UserAgent string  // from UserAgentFromRequest(r)
}

type LoginOutput struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresIn  time.Duration  // relative to time.Now()
    RefreshExpiresIn time.Duration
    User             User           // includes Email
}

type RefreshOutput struct {
    AccessToken      string
    RefreshToken     string  // ROTATED — replace stored token with this
    AccessExpiresIn  time.Duration
    RefreshExpiresIn time.Duration
}
```

**Methods:**
```go
func (c *Client) Login(ctx context.Context, in LoginInput) (LoginOutput, error)
// Timeout: cfg.LoginTimeout (5s). Errors: ErrInvalidToken, ErrRateLimited, ErrUnavailable.

func (c *Client) Refresh(ctx context.Context, refreshToken, clientIP, userAgent string) (RefreshOutput, error)
// Timeout: cfg.RefreshTimeout (5s). Singleflight-dedup per hashToken(refreshToken).
// Key-forgotten after Do (Refresh is single-use). DOES NOT cancel fn on ctx cancel.
// Errors: ErrInvalidToken (reuse/unknown), ErrUnavailable, ErrRateLimited.

func (c *Client) Logout(ctx context.Context, refreshToken string) error
// Timeout: cfg.RefreshTimeout. Errors: ErrInvalidToken, ErrUnavailable.

func (c *Client) LogoutAll(ctx context.Context, accessToken string) error
// Timeout: cfg.ValidateTimeout. Bumps token_version — all sessions invalidated.
// Errors: ErrInvalidToken, ErrUnavailable.

func (c *Client) ChangePassword(ctx context.Context, accessToken, currentPassword, newPassword string) error
// Timeout: cfg.LoginTimeout (bcrypt verify ~100ms). Known v0.1.0 limitation:
// wrong-current-password returns ErrUnavailable (codes.InvalidArgument not mapped).
// Errors: ErrInvalidToken, ErrUnavailable.

func (c *Client) GetCurrentUser(ctx context.Context, accessToken string) (User, error)
// Timeout: cfg.ValidateTimeout. Returns User with Email populated.
// nil response.User → fmt.Errorf("authclient: get_current_user empty user: %w", ErrUnavailable).
// Errors: ErrInvalidToken, ErrUnavailable.
```

**Private helper in auth.go (not for examples to call):**
```go
func expiresIn(unixSeconds int64) time.Duration  // returns 0 for <=0 input
func (c *Client) logUnavailableIfNeeded(ctx context.Context, method string, err error)
```

---

## 4. TASK-005 API (cookies.go + helpers.go)

### lib/cookies.go (new file)

```go
type CookieOption func(*http.Cookie)

func WithCookieDomain(domain string) CookieOption
func WithCookiePath(path string) CookieOption
func WithCookieMaxAge(maxAge time.Duration) CookieOption  // converts to int seconds
func WithCookieSameSite(mode http.SameSite) CookieOption

func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)
// Defaults: Name="refresh_token", Path="/auth/refresh", MaxAge=14*24*60*60,
//           SameSite=Strict, HttpOnly=true (hardcoded), Secure=true (hardcoded), Domain=""

func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)
// Sets MaxAge=-1, Value="", Expires=time.Unix(0,0) (RFC 6265 §5.3 compat).
// SAME defaults for Path/Domain as SetRefreshCookie — must match for cookie identity.
```

### lib/helpers.go (new file)

```go
func IPFromRequest(r *http.Request) string
// Fallback chain:
// 1. r.Header.Get("X-Real-IP") if non-empty
// 2. First entry of X-Forwarded-For (TrimSpace, SplitN(",",2)[0])
// 3. host from net.SplitHostPort(r.RemoteAddr) if no error
// 4. r.RemoteAddr verbatim (including "" for tests)
// SECURITY WARNING: spoofable without trusted proxy.

func UserAgentFromRequest(r *http.Request) string
// Returns r.UserAgent() — pure passthrough.
```

---

## 5. INTERNAL PACKAGES (lib/internal/) — available to lib/ but NOT to examples/

### lib/internal/cache/memory.go

Package: `cache`. Type: `*cache.MemoryCache`. Methods: `Get/Set/Delete`. Used internally by `Client.New` via `cacheAdapter`. Examples MUST NOT import this.

### lib/internal/refresh/group.go

Package: `refresh`. Type: `*refresh.Group`. Method: `Do(key, fn)` with mandatory Forget after Do. Used internally by `Client.Refresh`. Examples MUST NOT import this.

---

## 6. GRPC HEALTH PACKAGE — confirmed available

The `google.golang.org/grpc` module (v1.79.3, already in `go.mod`) bundles the health subpackages. No new `go.sum` entries needed.

**Import paths (no new go.sum entries):**
```go
"google.golang.org/grpc/health"           // package health — Server type
"google.golang.org/grpc/health/grpc_health_v1"  // package grpc_health_v1 — interfaces + protos
```

**Key types:**
```go
// grpc_health_v1 package:
type HealthServer interface {
    Check(ctx, *HealthCheckRequest) (*HealthCheckResponse, error)
    List(ctx, *HealthListRequest) (*HealthListResponse, error)
    Watch(*HealthCheckRequest, Health_WatchServer) error
}
func RegisterHealthServer(s grpc.ServiceRegistrar, srv HealthServer)
func NewHealthClient(cc grpc.ClientConnInterface) HealthClient

// health package (server implementation):
func NewServer() *health.Server  // implements HealthServer + UnimplementedHealthServer
func (s *Server) SetServingStatus(service string, servingStatus grpc_health_v1.HealthCheckResponse_ServingStatus)
```

`health.NewServer()` returns a `*health.Server` that embeds `UnimplementedHealthServer` — can be registered directly.

---

## 7. SLOG ADAPTER PATTERN for http_middleware example

The SDK `Logger` interface has a signature difference from `*slog.Logger`:
```go
// SDK Logger:
Error(ctx context.Context, msg string, err error, fields ...any)
// slog:
slog.ErrorContext(ctx, msg, args ...any)  // no dedicated err param
```

The adapter must handle the `err error` param in `Error`. Recommended pattern:
```go
type slogAdapter struct{ l *slog.Logger }

func (a slogAdapter) Info(ctx context.Context, msg string, fields ...any) {
    a.l.InfoContext(ctx, msg, fields...)
}
func (a slogAdapter) Warn(ctx context.Context, msg string, fields ...any) {
    a.l.WarnContext(ctx, msg, fields...)
}
func (a slogAdapter) Error(ctx context.Context, msg string, err error, fields ...any) {
    args := make([]any, 0, len(fields)+2)
    if err != nil {
        args = append(args, "error", err)
    }
    args = append(args, fields...)
    a.l.ErrorContext(ctx, msg, args...)
}
```
This is pure stdlib — no new deps. `slog` is available since Go 1.21; project uses Go 1.24.

---

## 8. MAKEFILE: build-examples TARGET — DISCREPANCY

**Current Makefile (lines 81-88):**
```makefile
build-examples:
    @mkdir -p bin
    @for dir in lib/examples/*/; do \
        name=$$(basename $$dir); \
        echo "  Building example-$$name..."; \
        go build -o bin/example-$$name ./$$dir; \
    done
    @echo "✅ Examples built in bin/"
```

**TASK-006 DOD says:** `go build -o .temp/examples/<name>` (not `bin/`).

**TASK-006 WHAT item 6 says:** "Должен собирать все три примера (`go build -o .temp/examples/<name> ./lib/examples/<name>`) без warnings."

**PRP MUST update Makefile:**
- Change `mkdir -p bin` → `mkdir -p .temp/examples`
- Change `-o bin/example-$$name` → `-o .temp/examples/$$name`
- Update `clean` target accordingly (currently `rm -rf $(COVERAGE_DIR) bin/`).
- `.temp/` should be in `.gitignore` (check if already there).

---

## 9. LINT RULES AFFECTING EXAMPLES (.golangci.yaml)

Enabled linters relevant to example code:
- `lll` — line-length: **140** chars (not 120 as in guides — use 140).
- `goconst` — min-len: 3, min-occurrences: 3. Avoid repeating string literals 3+ times.
- `gocritic` — experimental tags enabled; `hugeParam` disabled. Watch for `appendAssign`, `paramTypeCombine`.
- `gosec` — excludes G306 (file perms), G401 (weak hash — sha256 ok), G501 (md5). `gosec` may flag `os.Getenv` in examples for `G104` (error return) — but `os.Getenv` has no error return so this is not an issue.
- `revive` — enabled. Common triggers: exported functions without godoc, unused parameters.
- `unused` — examples are `main` packages; all exported symbols must be used.
- `unparam` — checks for always-same params.
- `ineffassign` — check all assignments.

**Exclusions in .golangci.yaml:**
```yaml
exclusions:
  paths:
    - mocks
    - ".*_test\\.go$"
```
Examples are NOT excluded — they run through full linting. This means:
- All exported functions in examples need godoc.
- No unused variables.
- No global variables that could trigger `gochecknoglobals` — but that linter is NOT enabled in this config.

**Formatters:** `gofmt` + `goimports` with local-prefix `github.com/webdelo/auth-client-go`.

---

## 10. EXISTING EXAMPLES STRUCTURE

Current state:
```
lib/examples/
└── basic/
    └── main.go  ← PLACEHOLDER (8 lines, prints text, no real code)
```
`http_middleware/` and `grpc_interceptor/` directories do NOT exist yet — they must be created.

**Existing placeholder** (`lib/examples/basic/main.go`):
```go
// Package main is a placeholder. The full demo lives under TASK-006.
package main

import "fmt"

func main() {
    fmt.Println("authclient SDK — see examples/basic/main.go in TASK-006")
}
```

---

## 11. ENV VARIABLES FOR basic EXAMPLE

Per TASK-006 WHAT item 1:
- `AUTH_ENDPOINT` — AUTH gRPC address (e.g., `localhost:50051`)
- `AUTH_TEST_EMAIL` — test user email
- `AUTH_TEST_PASSWORD` — test user password

Pattern: `os.Getenv("AUTH_ENDPOINT")`. If empty → print usage and `os.Exit(1)`.

---

## 12. go.mod CONFIRMED DEPENDENCIES

```
module github.com/webdelo/auth-client-go
go 1.24.0

require:
  github.com/stretchr/testify v1.11.1
  github.com/webdelo/auth-proto/go v0.1.0
  golang.org/x/sync v0.19.0
  google.golang.org/grpc v1.79.3

indirect:
  golang.org/x/net v0.48.0
  google.golang.org/protobuf v1.36.11
  ... (yaml, difflib, spew)
```

**auth_v1 import path:** `auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"`

**grpc health import paths (transitive, no new go.sum entries):**
- `"google.golang.org/grpc/health"` (package `health`) — `NewServer()`, `SetServingStatus()`
- `"google.golang.org/grpc/health/grpc_health_v1"` (package `grpc_health_v1`) — `RegisterHealthServer`, proto types

---

## 13. CRITICAL API SHAPE NOTES FOR EXAMPLES

### basic/main.go flow:
```go
client, err := authclient.New(authclient.Config{Endpoint: endpoint, Logger: myLogger})
// handle err
defer client.Close()

out, err := client.Login(ctx, authclient.LoginInput{
    Email:     email,
    Password:  password,
    ClientIP:  "127.0.0.1",   // demo only; real: authclient.IPFromRequest(r)
    UserAgent: "demo-client",  // demo only
})
// out.AccessToken, out.User.UUID, out.User.Role, out.User.Email

user, err := client.ValidateAccessToken(ctx, out.AccessToken)
// user.UUID, user.Role (Email is "" here)

err = client.Logout(ctx, out.RefreshToken)

// print first 8 chars of token for demo: out.AccessToken[:8] + "..."
```

### http_middleware/main.go key shapes:
```go
// slog adapter (inline struct in example):
type slogAdapter struct{ l *slog.Logger }
// implement Info/Warn/Error

// Client init:
client, _ := authclient.New(authclient.Config{
    Endpoint: endpoint,
    Logger:   slogAdapter{slog.Default()},
})

// Protected route:
mux.Handle("/api/profile", client.HTTPMiddleware(http.HandlerFunc(profileHandler)))
// In profileHandler:
user, ok := authclient.UserFromContext(r.Context())

// Login handler:
// - Read JSON body {email, password}
// - out, err := client.Login(ctx, authclient.LoginInput{
//       Email: req.Email, Password: req.Password,
//       ClientIP: authclient.IPFromRequest(r), UserAgent: authclient.UserAgentFromRequest(r),
//   })
// - authclient.SetRefreshCookie(w, out.RefreshToken)
// - JSON response {access_token, expires_in, user: {uuid, role, email}}

// Logout handler:
// - cookie, err := r.Cookie("refresh_token")
// - client.Logout(ctx, cookie.Value)
// - authclient.ClearRefreshCookie(w)
```

### grpc_interceptor/main.go key shapes:
```go
// gRPC server with interceptor:
grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(client.UnaryInterceptor()))

// Register health service — builtin, no custom proto:
healthSrv := health.NewServer()
healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)

// Demo: /grpc.health.v1.Health/Check is in default openMethods → no auth required.
// All other methods require valid Bearer in gRPC metadata.

// gRPC client demo (3 calls):
// 1. Health/Check without token — should succeed
// 2. Some protected method with valid access token in metadata
// 3. Some protected method without token — should get Unauthenticated

// Setting auth metadata on client side:
md := metadata.Pairs("authorization", "Bearer "+accessToken)
ctx = metadata.NewOutgoingContext(ctx, md)
```

**For the "protected method" in grpc_interceptor**: since the example cannot define custom proto (no new deps), options are:
- Use `health.Watch` (streaming — not unary, won't work with UnaryInterceptor).
- Register a second service. But any second service needs a proto stub.
- Alternative: demonstrate interceptor behavior via the demo gRPC client calling `Health/Check` (open), and then calling a custom grpc.ServiceDesc registered inline using raw gRPC server-side approach.

**Recommended approach** (simplest, no new deps): Use `google.golang.org/grpc/reflection` package (also bundled in grpc module) to register a second handler, OR implement a minimal custom service using `grpc.ServiceDesc` + `grpc.MethodDesc` directly. The reflection package is at `"google.golang.org/grpc/reflection"`.

Actually the cleanest approach: define a minimal in-process "echo" service via `grpc.ServiceDesc` without any proto-generated code. The example just needs to show the interceptor skip behavior. The grpc_health_v1.Health/Check being in openMethods is the key demo — the client calls Check without a token and it works. Then the example shows that any OTHER method (implemented via a raw `grpc.ServiceDesc`) requires a token.

---

## 14. SUMMARY

| Item | Finding |
|------|---------|
| Module | `github.com/webdelo/auth-client-go`, package `authclient` |
| Existing code (TASK-002) | `New`, `Close`, `Config`, `FailMode`, `Option`, `Logger`/`NoopLogger`, `Cache` interface, 5 sentinels, `AuthError`, `RetryAfter` |
| TASK-003 new API | `User`, `Role`, `WithUser`, `UserFromContext`, `ValidateAccessToken`, `HTTPMiddleware`, `UnaryInterceptor` |
| TASK-004 new API | `LoginInput`, `LoginOutput`, `RefreshOutput`, `Login`, `Refresh`, `Logout`, `LogoutAll`, `ChangePassword`, `GetCurrentUser` |
| TASK-005 new API | `CookieOption`, `WithCookieDomain/Path/MaxAge/SameSite`, `SetRefreshCookie`, `ClearRefreshCookie`, `IPFromRequest`, `UserAgentFromRequest` |
| grpc health pkg | Available at `google.golang.org/grpc/health` + `/health/grpc_health_v1` — no new go.sum entries |
| slog adapter | Needs custom `Error(ctx, msg, err, fields...)` bridge — pure stdlib, no new deps |
| Makefile discrepancy | `build-examples` currently outputs to `bin/` — TASK-006 requires `.temp/examples/` |
| Lint config | lll=140 chars, gosec active (no G401/G501 issues with sha256), no gochecknoglobals, examples NOT excluded |
| examples structure | Only `lib/examples/basic/main.go` exists (placeholder); `http_middleware/` and `grpc_interceptor/` must be created |
| Logger.Error signature | `Error(ctx, msg, err error, fields...)` — NOT slog-compatible directly; adapter must handle `err` param |
| FORBIDDEN in examples | `lib/internal/*` imports; new go.sum entries; `mock.Anything` in any tests |
