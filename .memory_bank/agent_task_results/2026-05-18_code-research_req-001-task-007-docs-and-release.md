# Code Research — REQ-001-TASK-007 (Documentation & Release)

**Date**: 2026-05-18  
**Branch**: feature/req-001-task-003  
**Scope**: Pre-flight research for writing the PRP for TASK-007 (README + godoc + CHANGELOG + git tag v0.1.0)

---

## 1. Current Documentation State

### 1.1 lib/doc.go

`lib/doc.go` **does not exist**. The file is absent from the repository entirely.

```
lib/ — no doc.go present (confirmed: ls returns nothing for that path)
```

The package-level godoc currently lives in `lib/errors.go:1-8` as a side-effect of that file being the first alphabetically to contain a package comment:

```go
// lib/errors.go:1-8
// Package authclient is the official Go SDK for the webdelo/auth service.
//
// The SDK is a thin wrapper over the gRPC contract from
// github.com/webdelo/auth-proto with non-trivial concerns baked in: a
// ValidateToken TTL cache, an HTTP middleware, a gRPC unary interceptor,
// refresh-race serialization, and typed errors. Create a Client with New,
// dispose with Close.
package authclient
```

This is a **gap**: the ADR and TASK-007 WHAT §1 require a dedicated `lib/doc.go` containing the package-level doc, quickstart snippet (5 lines), and links to the server and proto repos. The comment in `errors.go` covers only the one-paragraph description — no quickstart, no external links.

### 1.2 README.md

`README.md` currently has **4 sections**:

| Section | Present | Notes |
|---------|---------|-------|
| Installation | Yes | Uses `@latest` not `@v0.1.0`; no GOPRIVATE note paired with versioned install |
| Quickstart | **Missing** | No code example showing New + HTTPMiddleware + UserFromContext |
| Configuration table | **Missing** | No table of Config fields + defaults + all Option functions |
| FailMode section + CAPSLOCK warning | **Missing** | |
| Typed errors table | **Missing** | |
| Adapter interfaces (Logger, Cache) + slog-wrapper example | **Missing** | |
| Cookie helpers (defaults table, override patterns) | **Missing** | |
| HTTP request helpers (fingerprint rationale, spoofing caveat) | **Missing** | |
| Examples links | **Missing** | |
| Local integration testing | Yes (under Development) | Present but not under its own top-level section |
| TLS setup | **Missing** | |
| Troubleshooting | **Missing** | |
| Known limitations | **Missing** | |
| Versioning policy | **Missing** | |
| Development / Commands | Yes | Full `make` command list present |
| License | Yes | |

The current README covers the **developer-contributor perspective** (how to run tests, how to build) but is entirely missing the **consumer perspective** (how to integrate the SDK). This is the primary gap.

Installation section (`README.md:15`) uses `@latest` which will not resolve until a tag exists. For TASK-007 it must change to `@v0.1.0`.

`README.md:19-20` mentions `GOPRIVATE=github.com/webdelo/*` requirement — this is correct and should be kept/expanded in the versioned install example.

### 1.3 CHANGELOG.md

`CHANGELOG.md` currently contains:

```markdown
## [Unreleased]

### Added
- Initial library structure
- Basic version constant
- Example usage in lib/examples/basic/
```

**Gaps for TASK-007:**

1. The `[Unreleased]` section content is stale — it reflects pre-TASK-001 state (just "initial structure"). The full feature set from TASK-001 through TASK-006 needs to be listed in `## v0.1.0 — YYYY-MM-DD`.
2. There is no `## v0.1.0 — YYYY-MM-DD` entry at all.
3. Missing sections required by TASK-007: `Added` (full feature list), `Known limitations`, `Acknowledgements`.
4. The format header (`CHANGELOG.md:3-6`) is already Keep a Changelog compatible — that part is correct.
5. No `[v0.1.0]: https://github.com/webdelo/auth-client-go/compare/...` link at the bottom (standard KaC footer).

---

## 2. Godoc Coverage Audit (lib/*.go)

**Files in scope** (7 source files, excluding `*_test.go`, `export_test.go`, `mocks/`, `internal/`):
- `lib/client.go`
- `lib/errors.go`
- `lib/validate.go`
- `lib/context.go`
- `lib/logger.go`
- `lib/middleware_http.go`
- `lib/interceptor_grpc.go`

**Result: ALL existing exported declarations have godoc. ALL godoc comments start with the correct identifier name. Zero violations.**

Comprehensive automated check (walking multi-line comments to find the first line of each doc block) confirmed every exported symbol. Full enumeration:

### Types

| Identifier | File:Line | Has doc | Starts with name | Security WARNING |
|------------|-----------|---------|-----------------|-----------------|
| `FailMode` | `client.go:49` | YES | YES — "FailMode controls how..." | n/a |
| `FailClosed` (const) | `client.go:54` | YES | YES — "FailClosed (default) blocks..." | YES (inline: "OWASP-recommended") |
| `FailOpen` (const) | `client.go:59` | YES | YES — "FailOpen passes the request..." | YES — "SECURITY WARNING: misuse leads to auth bypass." |
| `Cache` | `client.go:66` | YES | YES — "Cache is the interface..." | n/a |
| `Config` | `client.go:77` | YES | YES — "Config bundles the constructor parameters..." | n/a |
| `Config.Endpoint` (field) | `client.go:79` | YES | YES | n/a |
| `Config.ValidateCacheTTL` (field) | `client.go:84` | YES | YES | n/a |
| `Config.FailMode` (field) | `client.go:88` | YES | YES | n/a |
| `Config.ValidateTimeout` (field) | `client.go:91` | YES | YES | n/a |
| `Config.LoginTimeout` (field) | `client.go:94` | YES | YES | n/a |
| `Config.RefreshTimeout` (field) | `client.go:97` | YES | YES | n/a |
| `Config.DialOptions` (field) | `client.go:99` | YES | YES | n/a |
| `Config.Logger` (field) | `client.go:105` | YES | YES | n/a |
| `Config.Cache` (field) | `client.go:108` | YES | YES | n/a |
| `Option` | `client.go:113` | YES | YES — "Option is a functional option..." | n/a |
| `Client` | `client.go:172` | YES | YES — "Client is the SDK entry point..." | n/a |
| `AuthError` | `errors.go:42` | YES | YES — "AuthError carries the sentinel error..." | n/a |
| `AuthError.Err` (field) | `errors.go:43` | NO explicit doc | n/a | n/a |
| `AuthError.RetryAfter` (field) | `errors.go:44` | NO explicit doc | n/a | n/a |
| `Role` | `context.go:9` | YES | YES — "Role is the user authorization role..." | n/a |
| `RoleUnknown` (const) | `context.go:13` | YES | YES | n/a |
| `RoleUser` (const) | `context.go:16` | YES | YES | n/a |
| `RoleAdmin` (const) | `context.go:19` | YES | YES | n/a |
| `User` | `context.go:28` | YES | YES — "User is the authenticated principal..." | n/a |
| `User.UUID` (field) | n/a | NO inline doc | — | n/a |
| `User.Role` (field) | n/a | NO inline doc | — | n/a |
| `User.Email` (field) | n/a | NO inline doc | — | n/a |
| `Logger` | `logger.go:12` | YES | YES — "Logger is the adapter interface..." | n/a |
| `NoopLogger` | `logger.go:20` | YES | YES — "NoopLogger is the silent default..." | n/a |

### Functions and Methods

| Identifier | File:Line | Has doc | Starts with name | Security WARNING |
|------------|-----------|---------|-----------------|-----------------|
| `WithFailMode` | `client.go:119` | YES | YES — "WithFailMode overrides Config.FailMode..." | YES — "SECURITY WARNING: WithFailMode(FailOpen) disables auth..." |
| `WithLogger` | `client.go:124` | YES | YES — "WithLogger overrides Config.Logger..." | n/a |
| `WithCache` | `client.go:140` | YES | YES — "WithCache sets a custom ValidateToken result cache..." | n/a |
| `WithOpenMethods` | `client.go:148` | YES | YES — "WithOpenMethods registers gRPC methods..." | n/a |
| `New` | `client.go:196` | YES | YES — "New creates a Client..." | n/a |
| `Client.Close` | `client.go:291` | YES | YES — "Close shuts down background goroutines..." | n/a |
| `Client.ValidateAccessToken` | `validate.go:39` | YES | YES — "ValidateAccessToken validates accessToken..." | n/a |
| `Client.HTTPMiddleware` | `middleware_http.go:49` | YES | YES — "HTTPMiddleware wraps next with an Authorization Bearer-token check..." | n/a |
| `Client.UnaryInterceptor` | `interceptor_grpc.go:25` | YES | YES — "UnaryInterceptor returns a gRPC UnaryServerInterceptor..." | n/a |
| `ErrInvalidToken` | `errors.go:21` | YES | YES — "ErrInvalidToken indicates the access token is malformed..." | n/a |
| `ErrExpired` | `errors.go:25` | YES | YES — "ErrExpired indicates the token signature is valid but its expiration has passed..." | n/a |
| `ErrBlocked` | `errors.go:29` | YES | YES — "ErrBlocked indicates the user account is blocked..." | n/a |
| `ErrRateLimited` | `errors.go:33` | YES | YES — "ErrRateLimited indicates AUTH rate-limited this caller..." | n/a |
| `ErrUnavailable` | `errors.go:37` | YES | YES — "ErrUnavailable indicates AUTH was unreachable..." | n/a |
| `RetryAfter` | `errors.go:66` | YES | YES — "RetryAfter returns the Retry-After hint..." | n/a |
| `AuthError.Error` | `errors.go:48` | YES | YES — "Error implements the error interface..." | n/a |
| `AuthError.Unwrap` | `errors.go:56` | YES | YES — "Unwrap returns the wrapped sentinel error..." | n/a |
| `User.IsZero` | `context.go:35` | YES | YES — "IsZero reports whether u is the zero value..." | n/a |
| `User.IsAdmin` | `context.go:38` | YES | YES — "IsAdmin reports whether u has the admin role." | n/a |
| `User.HasRole` | `context.go:41` | YES | YES — "HasRole reports whether u has the exact given role." | n/a |
| `WithUser` | `context.go:49` | YES | YES — "WithUser returns a copy of ctx with user attached..." | n/a |
| `UserFromContext` | `context.go:55` | YES | YES — "UserFromContext extracts the User stored by WithUser..." | n/a |
| `NoopLogger.Info` | `logger.go:23` | YES | YES — "Info implements Logger." | n/a |
| `NoopLogger.Warn` | `logger.go:26` | YES | YES — "Warn implements Logger." | n/a |
| `NoopLogger.Error` | `logger.go:29` | YES | YES — "Error implements Logger." | n/a |
| `Cache.Get` | `client.go:67` | YES (inline) | YES | n/a |
| `Cache.Set` | `client.go:69` | YES (inline) | YES | n/a |
| `Cache.Delete` | `client.go:71` | YES (inline) | YES | n/a |
| `Logger.Info/Warn/Error` | `logger.go:13-16` | NO inline method docs (interface body) | — | n/a |

**Minor gaps in existing code (not blocking):**

- `AuthError.Err` and `AuthError.RetryAfter` exported struct fields have no inline godoc (`errors.go:43-44`). Per standard Go style, struct fields without a doc comment are acceptable if the struct-level comment explains them.
- `User.UUID`, `User.Role`, `User.Email` struct fields have no inline godoc (`context.go:29-31`). The struct-level comment explains `Email`; `UUID` and `Role` are self-evident.
- `Logger` interface method signatures (`logger.go:13-16`) have no per-method docs inside the interface body. This is accepted style for simple interface methods.

**`WithCookieSameSite` security warning**: this function does not exist yet (lives in `cookies.go`, which is a TASK-005 deliverable not yet merged). The TASK-007 spec requires a security warning on this option when it arrives.

**Lint status**: `golangci-lint run ./lib/...` returns **0 issues** on the current branch. `revive` is enabled without custom settings, which means it uses its default ruleset. The default `revive` ruleset does **not** enforce exported-comment (that rule is opt-in in revive). `godot` linter is **not** enabled in `.golangci.yaml`. Therefore, missing or style-violating godoc will NOT be caught by CI — it must be verified manually via `go doc`.

---

## 3. Public Surface vs ADR §API

### ADR-defined v0.1.0 public surface (REQ-001-ADR.md:542-699)

The ADR "API" section (`REQ-001-ADR.md:542`) defines the complete mock of `lib/api.go`. Canonical list:

**Constructors & Config:**
- `type Config struct` (Endpoint, ValidateCacheTTL, FailMode, ValidateTimeout, LoginTimeout, RefreshTimeout, DialOptions, Logger, Cache)
- `type FailMode uint8` with `FailClosed`, `FailOpen`
- `type Option func(*Client)`
- `func WithFailMode(mode FailMode) Option`
- `func WithLogger(l Logger) Option`
- `func WithCache(c Cache) Option`
- `func WithOpenMethods(methods ...string) Option`
- `func New(cfg Config, opts ...Option) (*Client, error)`
- `func (c *Client) Close() error`

**Validate:**
- `func (c *Client) ValidateAccessToken(ctx context.Context, accessToken string) (User, error)`

**Auth flow (TASK-004 deliverables — NOT YET MERGED):**
- `type LoginInput struct` (Email, Password, ClientIP, UserAgent)
- `type LoginOutput struct` (AccessToken, RefreshToken, AccessExpiresIn, RefreshExpiresIn, User)
- `type RefreshOutput struct` (AccessToken, RefreshToken, AccessExpiresIn, RefreshExpiresIn)
- `func (c *Client) Login(ctx context.Context, in LoginInput) (LoginOutput, error)`
- `func (c *Client) Refresh(ctx context.Context, refreshToken, clientIP, userAgent string) (RefreshOutput, error)`
- `func (c *Client) Logout(ctx context.Context, refreshToken string) error`
- `func (c *Client) LogoutAll(ctx context.Context, accessToken string) error`
- `func (c *Client) ChangePassword(ctx context.Context, accessToken, currentPassword, newPassword string) error`
- `func (c *Client) GetCurrentUser(ctx context.Context, accessToken string) (User, error)`

**Middleware:**
- `func (c *Client) HTTPMiddleware(next http.Handler) http.Handler`
- `func (c *Client) UnaryInterceptor() grpc.UnaryServerInterceptor`

**User & Context (TASK-005 deliverables — partially merged via context.go in TASK-003):**
- `type Role string` with `RoleUnknown`, `RoleUser`, `RoleAdmin`
- `type User struct` (UUID, Role, Email) with `IsZero()`, `IsAdmin()`, `HasRole(role Role) bool`
- `func WithUser(ctx context.Context, user User) context.Context`
- `func UserFromContext(ctx context.Context) (User, bool)`

**Cookie helpers (TASK-005 — NOT YET MERGED):**
- `type CookieOption func(*http.Cookie)`
- `func WithCookieDomain(domain string) CookieOption`
- `func WithCookiePath(path string) CookieOption`
- `func WithCookieMaxAge(maxAge time.Duration) CookieOption`
- `func WithCookieSameSite(mode http.SameSite) CookieOption`
- `func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)`
- `func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)`

**HTTP request helpers (TASK-005 — NOT YET MERGED):**
- `func IPFromRequest(r *http.Request) string`
- `func UserAgentFromRequest(r *http.Request) string`

**Errors:**
- `var ErrInvalidToken`, `var ErrExpired`, `var ErrBlocked`, `var ErrRateLimited`, `var ErrUnavailable`
- `type AuthError struct` (Err error, RetryAfter time.Duration)
- `func RetryAfter(err error) time.Duration`

**Adapter interfaces:**
- `type Logger interface` (Info, Warn, Error)
- `type NoopLogger struct{}` + its 3 methods
- `type Cache interface` (Get, Set, Delete)

### Current state vs ADR (branch: feature/req-001-task-003)

| ADR symbol group | Present today | Missing (prereq TASK) |
|-----------------|---------------|----------------------|
| Config, FailMode, Option, 4 With* options, New, Close | YES | — |
| ValidateAccessToken | YES | — |
| HTTPMiddleware, UnaryInterceptor | YES | — |
| Role, User + methods, WithUser, UserFromContext | YES | — |
| Logger, NoopLogger, Cache interfaces | YES | — |
| 5 sentinel errors + AuthError + RetryAfter | YES | — |
| LoginInput, LoginOutput, RefreshOutput | NO | TASK-004 (auth.go) |
| Login, Refresh, Logout, LogoutAll, ChangePassword, GetCurrentUser | NO | TASK-004 (auth.go) |
| CookieOption + 4 With*Cookie* funcs | NO | TASK-005 (cookies.go) |
| SetRefreshCookie, ClearRefreshCookie | NO | TASK-005 (cookies.go) |
| IPFromRequest, UserAgentFromRequest | NO | TASK-005 (helpers.go) |
| lib/doc.go | NO | TASK-007 itself |

**Extra exported identifiers not in ADR** (present in export_test.go — only in test context, not public API):
- `FakeAuthClient`, `ClientConfig`, `NewTestClient`, `HashToken`, `DefaultOpenMethods`, `BearerToken`
These are test helpers in `export_test.go` (package `authclient`, not `authclient_test`) and ARE visible to `go doc` as exported symbols. The TASK-007 DOD requires `go doc github.com/webdelo/auth-client-go/lib` to match ADR — these test helpers would show up and violate the "no extra exported" requirement. **This is an open question for the PRP author** — see Section 12.

**DOD sanity check from TASK-007** (`lib/doc.go:68-73`): "5 sentinels, 4 With*Option, 5 With*CookieOption, 9 methods on Client, 4 helpers". Current count:
- 5 sentinels: YES (ErrInvalidToken, ErrExpired, ErrBlocked, ErrRateLimited, ErrUnavailable)
- 4 With*Option: YES (WithFailMode, WithLogger, WithCache, WithOpenMethods)
- 5 With*CookieOption: NO — 0 present (TASK-005 prereq)
- 9 methods on Client: NO — 3 present (ValidateAccessToken, HTTPMiddleware, UnaryInterceptor); 6 missing (TASK-004 prereq)
- 4 helpers: NO — 0 present (TASK-005 prereq)

---

## 4. Examples (lib/examples/)

**Current state**: one subdirectory exists — `lib/examples/basic/` — with a **placeholder** `main.go`.

```
lib/examples/basic/main.go:1-8  (entire file):
// Package main is a placeholder. The full demo lives under TASK-006.
package main

import "fmt"

func main() {
    fmt.Println("authclient SDK — see examples/basic/main.go in TASK-006")
}
```

**What TASK-007 expects** (depends-on: TASK-006):
- `lib/examples/basic/` — Login → Validate → Logout flow
- `lib/examples/http_middleware/` — Protected HTTP server
- `lib/examples/grpc_interceptor/` — Protected gRPC server

**All three subdirectories except `basic/` are completely absent.** TASK-007 is blocked on TASK-006 delivering real implementations of all three examples before docs and the tag can be created.

`make build-examples` (`Makefile:83-88`) iterates `lib/examples/*/` and compiles each. With only the placeholder, this builds successfully today but produces a trivial binary. After TASK-006 all three must compile cleanly.

`ci.yaml:70-73` (Build job) runs the same `go build "./${dir}"` loop — the placeholder currently satisfies CI; real examples must also pass.

---

## 5. Release Infrastructure

### 5.1 Makefile targets

Three release targets exist (`Makefile:103-115`):

```makefile
release:          ## Create a new release (interactive)
    @chmod +x dev/tools/release.sh
    @./dev/tools/release.sh

release-dry-run:  ## Preview release without making changes
    @chmod +x dev/tools/release.sh
    @./dev/tools/release.sh --dry-run

release-with-notes:  ## Create release with AI-generated notes (requires claude CLI)
    @chmod +x dev/tools/release.sh
    @./dev/tools/release.sh --generate-notes
```

No additional pre/post hooks — the Makefile delegates entirely to `release.sh`.

### 5.2 Release scripts

`dev/tools/release.sh` — full shell script (401 lines).

**Flow summary:**

1. **Prerequisites check**: requires `git`, `gh` CLI authenticated, no uncommitted changes (`dev/tools/release.sh:68-92`).
2. **Main branch detection**: reads `git remote show origin` → falls back to `main`/`master` (`release.sh:98-116`). Important: creates the tag on `origin/$MAIN_BRANCH`, not on the current branch.
3. **Tag detection**: `git describe --tags --abbrev=0` (`release.sh:171-175`). First run will find no tag and default to `v0.0.0` (`release.sh:343`).
4. **New commits check**: fails if no commits since last tag (`release.sh:135-168`).
5. **Interactive version prompt**: reads `X.Y.Z` from stdin, adds `v` prefix (`release.sh:197-224`).
6. **Release notes generation**: basic (git log formatted as markdown bullets) or via `claude --print` if `--generate-notes` (`release.sh:258-281`).
7. **Tag creation**: `git tag -a "$new_tag" -m "Release $new_tag" "$main_commit"` then `git push origin "$new_tag"` (`release.sh:299-308`).
8. **GitHub release**: `gh release create "$new_tag" --title "$new_tag" --notes-file -` (`release.sh:311-313`).

**Important caveats for PRP author:**

- The script creates the tag on `origin/main`'s HEAD commit (`release.sh:290-291`). This means TASK-007 work must be merged to `main` BEFORE running `make release`.
- `--dry-run` shows preview but does not tag or create GitHub release. Safe for testing.
- The script does NOT run `make audit` or `make build-examples` as a pre-release gate — those must be run manually beforehand.
- The GOPROXY verification command from TASK-007 (`GOPROXY=direct go list -m github.com/webdelo/auth-client-go@v0.1.0`) requires the tag to be **pushed to the remote** — local-only tags are not sufficient.

**Exact commands for PRP to reference:**

```bash
# 1. Ensure all prereqs are done (TASK-004 through TASK-006 merged to main)
make audit            # vet + lint + test
make build-examples   # all three examples build

# 2. Preview
make release-dry-run

# 3. Create release (interactive — enter "0.1.0" when prompted)
make release

# 4. Verify Go toolchain sees the tag
GOPROXY=direct go list -m github.com/webdelo/auth-client-go@v0.1.0
```

No `goreleaser` is used — the release process is entirely via `release.sh` + `gh release create`.

### 5.3 GitHub workflows / goreleaser

No `.goreleaser.yaml` exists. No goreleaser workflow. The only CI workflow is `ci.yaml`.

The `ci.yaml` has **no tag-triggered job** — it only triggers on push to `main`/`master` branches and `workflow_dispatch`. Creating the `v0.1.0` tag will NOT trigger any CI pipeline automatically. The PRP must note that `make audit` + `make build-examples` must be run locally before tagging.

---

## 6. CI Workflows

Single workflow: `.github/workflows/ci.yaml`.

| Job | Trigger | Action |
|-----|---------|--------|
| `lint` | push/PR to main | golangci-lint v2.11.4 |
| `test` | push/PR to main | `go test -v -race ./lib/...` |
| `build` | push/PR to main (needs lint+test) | `go build ./lib/...` + `go build` for each example |
| `integration` | `workflow_dispatch` with `run_integration=true` | `make test-integration` (continue-on-error: true) |

**No release/tag job exists.** Tag creation and GitHub release creation are manual via `make release`.

**Gap for TASK-007**: There is no automated "tag push → validate module is importable" gate. The PRP should note this is accepted for v0.1.0 — the BFF-validation smoke test serves as the manual gate.

---

## 7. Sibling Repo Precedents

### webdelo/auth (`../auth/CHANGELOG.md`)

Accessible at `/Users/a.popov/Development/TRADING/bots/auth/CHANGELOG.md`. The auth repo uses a **non-standard CHANGELOG format**: entries are organized by PR (e.g., `## [PR#4] REQ-001a-TASK-002: Fix scaffold build failures...`) rather than by semver version. Each entry lists bullet points of what changed. There is no `[Unreleased]` section — instead a `## [Unreleased]` / `## REQ-001a — Auth Core` heading structure is used. This is NOT Keep a Changelog format.

**For auth-client-go TASK-007**: the TASK explicitly requires Keep a Changelog format with `## v0.1.0 — YYYY-MM-DD` and Added/Changed/Known limitations sections. Do not follow the auth repo precedent.

### webdelo/auth-proto

No `CHANGELOG.md` exists in either `auth-proto/` root or `auth-proto/go/`. The proto repo is minimal (proto files + generated Go stubs, Makefile, README).

---

## 8. Linter Configuration for Docs

`.golangci.yaml` linters enabled:

```yaml
linters:
  enable:
    - revive       # <-- only linter that might check godoc
    - gocritic
    - staticcheck
    # ... (no godot, no godoc, no testifylint)
```

**Revive** is enabled (`golangci.yaml:15`) but has **no custom settings** section for `revive` in the config. By default, golangci-lint's revive integration does NOT enable the `exported` (exported-comment) rule — it uses a subset of revive's default rules, none of which enforce godoc presence.

**Godot** (checks that comments end with periods) is **not enabled**.

**Conclusion**: the current linter configuration will NOT catch:
- Missing godoc on any exported identifier
- Godoc not starting with the identifier name
- Godoc not ending with a period

The only programmatic check available is `go vet` (which does not check godoc style) and manual `go doc github.com/webdelo/auth-client-go/lib | grep -E "^(type|func|var|const)"`.

**Recommendation for PRP**: the godoc validation in the DOD (`TASK-007:68`) must be performed manually or via a `go doc` pipe command. There is no lint gate for it.

---

## 9. Audit / Build Targets

**`make audit`** (`Makefile:69`):
```makefile
audit: vet lint test ## Run all quality checks
    go mod verify
    go mod tidy -diff
```
Runs: `go vet ./...` → `golangci-lint run ./...` → unit tests → `go mod verify` → `go mod tidy -diff`.

**Does NOT run**: `make build-examples`, `make test-integration`, `make bench`, `make coverage`.

**`make build-examples`** (`Makefile:82-88`):
```makefile
build-examples: ## Build example programs
    @mkdir -p bin
    @for dir in lib/examples/*/; do
        name=$(basename $dir)
        go build -o bin/example-$name ./$dir
    done
```

Compiles each subdirectory under `lib/examples/`. Currently only `basic/` exists (placeholder). After TASK-006, all three must pass.

**No `make validate-godoc` target exists** — there is no programmatic way to check godoc completeness. Manual check required.

---

## 10. README Troubleshooting Items (with file refs)

All items listed in TASK-007 WHAT §3 "Troubleshooting" have code backing in the current codebase. Reference list for PRP:

### a) Debug 401 (enable Logger, check WWW-Authenticate)

Middleware always sets `WWW-Authenticate` header on 401 (`middleware_http.go:53`, `middleware_http.go:79`):
- Missing/malformed token: `WWW-Authenticate: Bearer realm="api"` (constant `wwwAuthMissing`, `middleware_http.go:14`)
- Invalid/expired/blocked token: `WWW-Authenticate: Bearer realm="api", error="invalid_token"` (constant `wwwAuthInvalid`, `middleware_http.go:15`)

### b) Fingerprint caveats (NAT, CGNAT, /24-binding)

ADR documents the exact fingerprint algorithm at `REQ-001-ADR.md:336`:
> `fingerprint = SHA-256(UserAgent + first-3-octets(IP))`
> Refresh issued for `203.0.113.55` will NOT work on `198.51.100.1` but WILL work on `203.0.113.99` (same /24).

`IPFromRequest` does not exist yet (`helpers.go` is TASK-005). The ADR specifies the extraction algorithm (`REQ-001-ADR.md:325-333`): `X-Real-IP` → `X-Forwarded-For[0]` → `RemoteAddr`. No trusted-proxies validation in v0.1.0 — spoofing risk documented.

### c) Keepalive auto-bump (Time < 10s → grpc-go silently bumps to 10s)

Documented in ADR (`REQ-001-ADR.md:190`):
> "Pitfall: `keepalive.ClientParameters{Time}` < 10s grpc-go молча подменяет на 10s без ошибки."

Current SDK sets `keepaliveTime = 30 * time.Second` (`client.go:25`) — well above the threshold. Consumer who passes their own `DialOptions` with a `WithKeepaliveParams` overriding Time could hit this. The warning should tell consumers "if you override keepalive via DialOptions, do not set Time < 10s — gRPC will silently bump it."

### d) Distributed singleflight absent (cross-replica BFF)

ADR (`REQ-001-ADR.md:135`): "works only within one process." 
`lib/internal/tools/tools.go` still has `golang.org/x/sync/singleflight` pinned as a build-tools dependency (not yet wired into any production code — TASK-004 prereq). The README should note: singleflight protects against parallel tabs in a single BFF instance; two BFF replicas can still trigger family detection in AUTH.

### e) TLS default is insecure (dev-only)

`client.go:205`: `grpc.WithTransportCredentials(insecure.NewCredentials())` is the first dial option. Consumer-supplied `DialOptions` come after (`client.go:213`) so a consumer-supplied `WithTransportCredentials(credentials.NewTLS(...))` wins by overriding the default (documented in `client.go:99-102` field comment). The README must make this explicit.

### f) cache TTL default and token_version invalidation lag

`client.go:19`: `defaultValidateCacheTTL = 30 * time.Second`. After `LogoutAll`/`BlockUser`/`ChangePassword`, the old access token stays valid in the cache for up to 30s. Document in README under Known Limitations and Troubleshooting. Consumer can set `ValidateCacheTTL: 0` to disable cache entirely.

---

## 11. go.mod / Module Path

`go.mod:1`: `module github.com/webdelo/auth-client-go` — confirmed correct (no `/lib` suffix, no submodule path).

`go.mod:3`: `go 1.24.0` — matches TASK requirement.

`go.mod` dependency versions:

| Dependency | Version | Notes |
|-----------|---------|-------|
| `github.com/webdelo/auth-proto/go` | v0.1.0 | Correct — matches DOD requirement "go mod why confirms v0.1.0" |
| `google.golang.org/grpc` | v1.79.3 | Pinned via auth-proto transitive, no manual override |
| `google.golang.org/protobuf` | v1.36.11 | Indirect |
| `golang.org/x/sync` | v0.19.0 | Used for singleflight (pinned in internal/tools) |
| `github.com/stretchr/testify` | v1.11.1 | Test-only |

No `replace` directives. No explicit grpc/protobuf pin that could conflict with auth-proto transitive — follows MVS correctly (Риск 1 from REQ-001 mitigated).

`go mod why github.com/webdelo/auth-proto/go` will confirm direct dependency since it appears in the `require` block (not just `// indirect`).

Tag format: `v0.1.0` (not `go/v0.1.0` — confirmed correct per TASK-007 accent: "not a submodule, the entire repo is the Go module").

---

## 12. Open Questions for PRP Author

**Q1: export_test.go exported symbols pollute `go doc`.**
`lib/export_test.go` (package `authclient`, not `authclient_test`) declares exported identifiers: `FakeAuthClient`, `ClientConfig`, `NewTestClient`, `HashToken`, `DefaultOpenMethods`, `BearerToken`. These are technically part of the `authclient` package and will appear in `go doc github.com/webdelo/auth-client-go/lib`. The TASK-007 DOD says the surface must match ADR §API ("no extra exported"). The PRP needs to address whether these test helpers are acceptable or whether they should be moved to an internal test package. **(Factual gap: the current state violates the DOD's "no extra exported" requirement.)**

**Q2: lib/examples/ — TASK-006 is a hard prereq.**
Currently `lib/examples/basic/main.go` is a placeholder (`main.go:1-8`). The subdirectories `http_middleware/` and `grpc_interceptor/` do not exist at all. TASK-007 depends on TASK-006 which in turn depends on TASK-004 and TASK-005. The PRP should clarify that TASK-007 cannot be executed (or even started) until TASK-006 is merged and all three examples are functional.

**Q3: `make build-examples` will fail if `http_middleware/` or `grpc_interceptor/` don't exist.**
Actually it will not fail — the shell `for dir in lib/examples/*/` loop only iterates existing directories. But after TASK-006 adds the dirs, they must compile. The PRP should include `make build-examples` as a DOD gate.

**Q4: No make target for `test-integration` in DOD — but DOD requires it to be green.**
TASK-007 DOD (`TASK-007.md:77`): "`make test-integration` зелёный". The `make test-integration` target exists in the Makefile (from TASK-001). The PRP should explicitly require running this before tagging, but note it requires Docker and the `webdelo/auth` repo at `../auth`.

**Q5: BFF-validation smoke test logistics.**
TASK-007 WHAT §7 requires a different developer (not the author) to attempt `go get github.com/webdelo/auth-client-go@v0.1.0` and build working middleware in <30 minutes using only the README. The result goes to `task_management/results/REQ-001-TASK-007-bff-validation.md`. The PRP should specify: (a) who the BFF developer is, (b) whether validation must happen BEFORE or AFTER tagging (it must be AFTER — the tag is what makes `go get @v0.1.0` work), (c) that the tag must be reverted/re-tagged if the README proves insufficient.

**Q6: CHANGELOG v0.1.0 entry must span TASK-001 through TASK-006 features.**
The `[Unreleased]` section currently only mentions "Initial library structure" and "Basic version constant" — both pre-TASK-001 scaffolding. The v0.1.0 entry needs to enumerate all delivered features. Since TASK-007 is the docs task, the author will need to pull the full feature list from REQ-001 DOD (`REQ-001.md:263-274`) and from each individual TASK deliverable.

**Q7: installation instruction — `@latest` vs `@v0.1.0`.**
`README.md:16` currently shows `go get github.com/webdelo/auth-client-go@latest`. After tagging, this should show `@v0.1.0`. The PRP should include updating this line as part of TASK-007.

**Q8: `lib/internal/refresh/` does not exist yet.**
The ADR and REQ-001 structure reference `lib/internal/refresh/` for the singleflight group (TASK-004). Only `lib/internal/cache/` and `lib/internal/tools/` exist. This is a TASK-004 prereq — not a TASK-007 issue, but confirms the dependency graph.

---

## Summary

| Area | Status |
|------|--------|
| lib/doc.go | Missing — must be created in TASK-007 |
| README.md | Shell exists; ~12 required sections absent; consumer-facing content entirely missing |
| CHANGELOG.md | Shell exists; v0.1.0 entry absent; [Unreleased] content stale |
| Godoc on existing exports | Complete and correct; all 35+ exported identifiers have compliant godoc |
| Missing files (TASK-004/005) | auth.go, cookies.go, helpers.go — all prereqs for TASK-007 |
| Examples | Only basic/ placeholder; http_middleware/ and grpc_interceptor/ absent (TASK-006 prereq) |
| Release tooling | make release + dev/tools/release.sh — fully functional, interactive, no goreleaser |
| v0.1.0 git tag | Does not exist yet; no existing tags in repo |
| CI on tag push | No tag-triggered CI job — manual audit required before tagging |
| Linter godoc enforcement | None — revive enabled but exported-comment rule not activated |
| go.mod module path | Correct: github.com/webdelo/auth-client-go |
| auth-proto dependency | v0.1.0 direct dependency — confirmed |
| export_test.go pollution | FakeAuthClient, ClientConfig, NewTestClient, HashToken, DefaultOpenMethods, BearerToken visible in go doc — potential DOD violation |
