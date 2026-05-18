# Code Research Report: REQ-001-TASK-005 — context.go / cookies.go / helpers.go

**Date:** 2026-05-18
**Researcher:** code-researcher (claude-sonnet)
**Task:** REQ-001-TASK-005
**PRP:** [REQ-001-TASK-005-PRP.md](../../task_management/plans/REQ-001-TASK-005-PRP.md)

---

## 1. Godoc Style and Package Name in lib/

**File:** `lib/lib.go:1-2`

Current placeholder:
```go
// Package lib provides auth-client-go functionality.
package lib
```

Per TASK-002-PRP Gotcha G1 (`task_management/plans/REQ-001-TASK-002-PRP.md:134`):
> `lib/lib.go` сейчас `package lib`. ADR §API требует `package authclient`. Все новые файлы — `package authclient`. Удалить `lib/lib.go` + `lib/lib_test.go`.

All new files (including TASK-005 files) must use:
```go
// Package authclient is the official Go SDK for the webdelo/auth service.
package authclient
```

Module is `github.com/webdelo/auth-client-go`, consumer imports as `import authclient "github.com/webdelo/auth-client-go/lib"`.

**Godoc pattern** (from TASK-002-PRP errors.go section at `task_management/plans/REQ-001-TASK-002-PRP.md:342-346`):

- Package doc: `// Package authclient is the official Go SDK for the webdelo/auth service.`
- Each exported identifier: starts with the exported name, English (CS-0060, CS-0070).
- Section separators: `// ─── Section Name ─────────────────` (em-dash rulers, 80-char wide).

**Godoc example from TASK-003-PRP context.go Data Models** (`task_management/plans/REQ-001-TASK-003-PRP.md:186-233`):
```go
// Role is the user authorization role.
//
// Values mirror auth.v1 proto contract. New roles are additive after v1.0.0;
// adding a new constant must NOT renumber the existing ones.
type Role string

// User is the authenticated principal extracted from a valid access token.
//
// Email is populated only by Login and GetCurrentUser. After ValidateAccessToken
// the Email is always empty because the gRPC ValidateTokenResponse does not
// carry it (proto v0.1.0 limitation — see REQ-001-ADR §R10).
type User struct {
    UUID  string
    Role  Role
    Email string
}

// userCtxKey is the unexported context key for User (EC-0056).
type userCtxKey struct{}

// WithUser returns a copy of ctx with user attached.
func WithUser(ctx context.Context, user User) context.Context {
    return context.WithValue(ctx, userCtxKey{}, user)
}

// UserFromContext extracts the User stored by WithUser. The bool is false if
// no User is attached.
func UserFromContext(ctx context.Context) (User, bool) {
    user, ok := ctx.Value(userCtxKey{}).(User)
    return user, ok
}
```

---

## 2. Existing Test Style in lib/

**File:** `lib/lib_test.go:1-9`

The only existing test file is a stub:
```go
package lib

import "testing"

func TestVersion(t *testing.T) {
    if Version == "" {
        t.Error("Version should not be empty")
    }
}
```

No table-driven tests exist yet. The PRPs specify the required style. From TASK-002-PRP Applied Standards (`task_management/plans/REQ-001-TASK-002-PRP.md:67`):
> UT-0001 (table-driven), UT-0002 (one method = one test), UT-0010 (TestType_Method), UT-0011 (happy_path first), UT-0012 (snake_case), UT-0027 (no decorative separators), UT-0028 (no helper funcs), UT-0030/0031 (require/assert), UT-0040/0041 (t.Parallel), UT-0100 (one test file per source file)

**Required table-driven test structure** — `map[string]struct{...}` (not slice), confirmed in `auth` sibling at `bots/auth/internal/service/httpcookies/refresh_test.go:18` and `bots/auth/internal/handlers/http/auth/cookie_test.go:16`:

```go
func TestFoo_Bar(t *testing.T) {
    t.Parallel()

    tests := map[string]struct {
        // fields
    }{
        "happy_path": {
            // ...
        },
        "some_error_case": {
            // ...
        },
    }

    for name, tc := range tests {
        tc := tc
        t.Run(name, func(t *testing.T) {
            t.Parallel()
            // assertions via require/assert (testify)
        })
    }
}
```

The `want` field uses `func(t *testing.T, ...)` for complex assertions (pattern from `bots/auth/internal/service/httpcookies/refresh_test.go:22-24`).

---

## 3. Cookie Patterns in Sibling Repos

### auth service (`bots/auth/`)

**Primary reference:** `bots/auth/internal/service/httpcookies/`

Three files:

**`config.go`** — `Name`, `Path` constants; `Config{Domain, Secure, SameSite}` struct; `NewConfig(ConfigParams)` panics on invalid config (`SameSite=None+!Secure`, domain starting with `.`):
```go
// Name is the cookie name for refresh tokens.
const Name = "auth_refresh_token"

// Path is the URL path scoping the refresh-token cookie to all /auth/* endpoints
const Path = "/auth"

type Config struct {
    Domain   string
    Secure   bool
    SameSite http.SameSite
}
```
(`bots/auth/internal/service/httpcookies/config.go:11-24`)

**`refresh.go`** — `Set(w, cfg, token, maxAge)` and `Clear(w, cfg)` functions. `HttpOnly` is hard-coded to `true` and not configurable through `Config`. `Clear` emits both `MaxAge: -1` AND `Expires: time.Unix(0, 0)` for RFC 6265 §5.3 legacy-client compat:
```go
func Set(w http.ResponseWriter, cfg Config, token string, maxAge time.Duration) {
    http.SetCookie(w, &http.Cookie{
        Name:     Name,
        Value:    token,
        Path:     Path,
        Domain:   cfg.Domain,
        MaxAge:   int(maxAge.Seconds()),
        HttpOnly: true,
        Secure:   cfg.Secure,
        SameSite: cfg.SameSite,
    })
}

func Clear(w http.ResponseWriter, cfg Config) {
    http.SetCookie(w, &http.Cookie{
        Name:     Name,
        Value:    "",
        Path:     Path,
        Domain:   cfg.Domain,
        MaxAge:   -1,
        Expires:  time.Unix(0, 0),
        HttpOnly: true,
        Secure:   cfg.Secure,
        SameSite: cfg.SameSite,
    })
}
```
(`bots/auth/internal/service/httpcookies/refresh.go:21-66`)

**Important difference vs SDK design**: Auth service uses a `Config` struct passed to each call. The SDK's TASK-005 uses `CookieOption func(*http.Cookie)` functional options instead (per ADR §A9 — `SetRefreshCookie(w, token, ...CookieOption)` / `ClearRefreshCookie(w, ...CookieOption)`).

**Note on Path:** Auth service uses `Path = "/auth"` (not `/auth/refresh`). ADR §A9 specifies `Path="/auth/refresh"` for the SDK (narrower scope = smaller attack surface). Follow ADR.

**Key learning to incorporate into PRP:** `ClearRefreshCookie` should set `Expires: time.Unix(0, 0)` in addition to `MaxAge: -1` for legacy-browser compat (RFC 6265 §5.3).

### fundingbot and signal-analyzer

No cookie-related Go files found in `GOLANG/fundingbot/` or `bots/signal-analyzer/`. Neither has HTTP cookie handling.

---

## 4. IPFromRequest / X-Forwarded-For Patterns

### auth service — primary reference

**File:** `bots/auth/internal/handlers/http/auth/requestctx.go:88-113`

The auth service has a proxy-trust-aware version with CIDR list validation:
```go
const (
    headerXForwardedFor = "X-Forwarded-For"
    headerXRealIP       = "X-Real-IP"
)

// extractIP returns the real client IP.
// X-Forwarded-For / X-Real-IP are honoured ONLY when r.RemoteAddr is in
// trustedProxies; otherwise the RemoteAddr host is used.
func extractIP(r *http.Request, trustedProxies []*net.IPNet) string {
    host, _, splitErr := net.SplitHostPort(r.RemoteAddr)
    if splitErr != nil {
        host = r.RemoteAddr
    }

    if !proxyTrusted(host, trustedProxies) {
        return host
    }

    if h := r.Header.Get(headerXForwardedFor); h != "" {
        if comma := strings.IndexByte(h, ','); comma > 0 {
            return strings.TrimSpace(h[:comma])
        }
        return strings.TrimSpace(h)
    }

    if h := r.Header.Get(headerXRealIP); h != "" {
        return strings.TrimSpace(h)
    }

    return host
}
```

**SDK version (from ADR §A12):** Simpler — no trusted-proxy validation in v0.1.0. Order: `X-Real-IP → X-Forwarded-For[0] → SplitHostPort(RemoteAddr) → raw RemoteAddr`. This differs from auth's order (auth starts with RemoteAddr check then conditionally trusts headers). The SDK always checks headers first without trust-list gate.

Test for the auth version at `bots/auth/internal/handlers/http/auth/requestctx_test.go` covers: XFF from trusted proxy, ignored XFF from untrusted peer, empty trusted list ignores XFF, X-Real-IP fallback, raw RemoteAddr with port.

### invest bot, fundingbot, signal-analyzer

- No `X-Forwarded-For`, `X-Real-IP`, `RemoteAddr`, or `SplitHostPort` found in `bots/invest/` or `GOLANG/fundingbot/`.
- Signal-analyzer: one occurrence at `signal-analyzer/generated/scratch/logger/logger.go:120` — simple `r.RemoteAddr` split on `:` without `net.SplitHostPort`. Not applicable.

**Pattern to apply:** Use `strings.IndexByte` or `strings.SplitN` for XFF first-element extraction. Use `strings.TrimSpace`. Auth uses `IndexByte(h, ',')`; SDK PRP uses `SplitN(h, ",", 2)[0]` — both correct, both O(n) to first comma. Style choice; PRP keeps `SplitN` for clarity.

---

## 5. Functional Options Style

**From TASK-002-PRP** (`task_management/plans/REQ-001-TASK-002-PRP.md:647-683`):

```go
// Option is a functional option for New.
type Option func(*Client)

// WithFailMode overrides Config.FailMode.
//
// SECURITY WARNING: WithFailMode(FailOpen) disables auth on AUTH-unavailable.
// Use only for endpoints where this is acceptable.
func WithFailMode(mode FailMode) Option {
    return func(c *Client) { c.failMode = mode }
}

// WithLogger overrides Config.Logger. nil collapses to NoopLogger{}.
func WithLogger(l Logger) Option {
    return func(c *Client) {
        if l == nil {
            l = NoopLogger{}
        }
        c.logger = l
    }
}
```

**For `CookieOption`** (TASK-005, ADR §A9):

Mirror exactly: closure returning a function that mutates the target. `HttpOnly` and `Secure` are NOT exposed as `CookieOption` (security invariants — always `true` for `SetRefreshCookie`).

---

## 6. Context-Key Pattern in lib/

Grepping `lib/` for `userCtxKey`, `CtxKey`, `ctxKey` returns **nothing** — no context key is declared yet (TASK-003 not yet executed).

**Template from TASK-003-PRP** (`task_management/plans/REQ-001-TASK-003-PRP.md:219-232`):
```go
// userCtxKey is the unexported context key for User (EC-0056).
type userCtxKey struct{}

// WithUser returns a copy of ctx with user attached.
func WithUser(ctx context.Context, user User) context.Context {
    return context.WithValue(ctx, userCtxKey{}, user)
}

// UserFromContext extracts the User stored by WithUser. The bool is false if
// no User is attached.
func UserFromContext(ctx context.Context) (User, bool) {
    user, ok := ctx.Value(userCtxKey{}).(User)
    return user, ok
}
```

**EC-0056** requires unexported struct type (not string/int) as context key. The auth service sibling uses the same pattern at `bots/auth/internal/handlers/http/auth/requestctx.go:37`: `type requestInfoKey struct{}`.

**TASK-005 verifies this surface exists from TASK-003** (Task 0 in PRP) — does NOT create it.

---

## 7. Makefile Targets

**File:** `Makefile`

Relevant targets for TASK-005:
- `make test` — unit tests via test-runner: `./lib/...` (line 26)
- `make test-race` — `CGO_ENABLED=1` + `--race` (line 29)
- `make coverage` — `-coverprofile=coverage/coverage.out` → `coverage.html` (line 35)
- `make lint` — `golangci-lint run ./... --timeout=5m` (line 48)
- `make fmt` — `go fmt ./...` (line 43)
- `make vet` — `go vet ./...` (line 46)
- `make audit` — vet + lint + test + `go mod verify` + `go mod tidy -diff` (line 69)
- `make generate-mocks` — `mockery --config mockery.yaml` (line 91) — NOT used in TASK-005

Tests run with a custom `test-runner` binary (`dev/tools/test-runner-main.go`), not raw `go test`. Pattern: `$(TEST_RUNNER) --title="Unit Tests" ./lib/...`. The runner wraps `go test` with formatted output.

**CRITICAL policy** (`CLAUDE.md:58-61`): ALWAYS use `make` commands. FORBIDDEN: `go build`/`go test -c` without `-o .temp/` flag.

---

## Summary Table

| Item | Finding |
|------|---------|
| Package name for new files | `package authclient` (NOT `package lib`) |
| Godoc style | CS-0060: comment starts with exported name, English; em-dash section rulers |
| Test style | `map[string]struct{}` table-driven, `happy_path` first, snake_case keys, `t.Parallel()` on each subtest, `want func(t *testing.T, ...)` for complex assertions |
| Cookie reference impl | `bots/auth/internal/service/httpcookies/` — `Set`/`Clear` with `Config` struct (SDK uses `CookieOption func(*http.Cookie)` instead) |
| Cookie Path discrepancy | Auth uses `"/auth"`, ADR §A9 specifies `"/auth/refresh"` for SDK — follow ADR |
| Clear cookie defensive pattern | `MaxAge: -1` + `Expires: time.Unix(0, 0)` (RFC 6265 §5.3 legacy-browser compat). PRP updated to include both. |
| IPFromRequest algorithm | ADR §A12: `X-Real-IP → XFF[0] → SplitHostPort(RemoteAddr) → raw RemoteAddr` (no trust-list in v0.1.0) |
| XFF parsing | `strings.SplitN(h, ",", 2)[0]` + `strings.TrimSpace` |
| `userCtxKey` | Not in `lib/` yet; TASK-005 Task 0 verifies it exists from TASK-003 |
| Functional options type | `type CookieOption func(*http.Cookie)` mirrors `type Option func(*Client)` |
| HttpOnly/Secure | Hard-coded `true` after opts (security invariant; not exposed as `CookieOption`) |
| Test commands | `make test`, `make test-race`, `make coverage`, `make audit` |
| Sibling cookie code | fundingbot, signal-analyzer: no HTTP cookie handling |
| Sibling IP code | invest, fundingbot: no IP extraction patterns |
