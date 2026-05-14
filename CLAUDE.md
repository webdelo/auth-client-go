# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**auth-client-go** — Go SDK for [webdelo/auth](https://github.com/webdelo/auth) service. Thin wrapper over the gRPC contract from [webdelo/auth-proto](https://github.com/webdelo/auth-proto) with non-trivial concerns baked in: ValidateToken cache, HTTP middleware, gRPC unary interceptor, refresh race serialization, typed errors. Go 1.24, lib-type project scaffolded with `scratch`.

Consumers: BFF, simple-trader, arbitrage-trader, and future Go services that need to validate AUTH tokens or proxy login/refresh flows.

## Quick Reference

| Resource | Path |
|----------|------|
| **Memory Bank** | |
| Tech Stack | `.memory_bank/tech_stack.yaml:1` |
| Project Map | `.memory_bank/project_map.yaml:1` |
| External Libs | `.memory_bank/external_libs.yaml:1` |
| Dev Commands | `.memory_bank/dev_commands.yaml:1` |
| Tasks List | `.memory_bank/tasks_list.yaml:1` |
| **Guides** | |
| Rules Index | `.claude/guides/rules_index.yaml:1` |
| Agent Catalog | `.claude/AGENTS.yaml:1` |
| **Task Management** | `task_management/` (tasks, plans, results, reviews, specs, bug_reports, debug_reports, session_analysis) |

## Commands

```bash
# Tests (ALWAYS use make, never raw go commands)
make test                     # Unit tests
make test-race                # Unit tests with race detector
make bench                    # Benchmarks
make coverage                 # Coverage report

# Build
make build-examples           # Build all examples in lib/examples/

# Lint & format
make lint                     # golangci-lint (config: .golangci.yaml)
make lint-fix                 # Auto-fix lint issues
make fmt                      # go fmt
make vet                      # go vet

# Code generation
make generate-mocks           # mockery (config: mockery.yaml)

# Full quality check
make audit                    # vet + lint + test

# Dependencies
make tidy                     # go mod tidy

# Release
make release                  # Interactive release
make release-dry-run          # Preview release
```

> **Build/Test Policy:**
> 1. **ALWAYS** use `make` commands from `.memory_bank/dev_commands.yaml:1`
> 2. If no suitable `make` command exists → use `-o .temp/` flag explicitly
> 3. **FORBIDDEN:** `go build` or `go test -c` without output flag — pollutes project root

## Architecture

**SDK design (after REQ-006 implementation):**

```
lib/
├── client.go              ← Client constructor, dial, options
├── validate.go            ← ValidateAccessToken + TTL cache
├── auth.go                ← Login / Refresh / Logout / LogoutAll / ChangePassword / GetCurrentUser
├── middleware_http.go     ← HTTP middleware for protected handlers
├── interceptor_grpc.go    ← gRPC unary interceptor for protected services
├── context.go             ← WithUser, UserFromContext
├── cookies.go             ← SetRefreshCookie, ClearRefreshCookie (HttpOnly; Secure; SameSite=Strict)
├── errors.go              ← Typed errors (ErrInvalidToken, ErrExpired, ErrBlocked, ErrRateLimited, ErrUnavailable)
├── helpers.go             ← IPFromRequest, UserAgentFromRequest
├── internal/
│   ├── cache/             ← TTL cache implementation
│   └── refresh/           ← singleflight for refresh race
└── examples/
    ├── basic/             ← Login + Validate flow
    ├── http_middleware/   ← Protected HTTP server
    └── grpc_interceptor/  ← Protected gRPC server
```

**Key design principles:**

- **No DI framework** — SDK uses plain constructors + functional options. Consumer brings own DI.
- **Adapter interfaces** for logger and metrics — SDK does not depend on concrete logger/Prometheus libs.
- **Fail-closed by default** — when AUTH is unavailable, middleware blocks the request. Override with `FailOpen` option.
- **Token version awareness** — cache invalidates entries when AUTH bumps `token_version` (via short TTL, 30s default).
- **Fingerprint binding** — SDK helpers extract client IP / UserAgent from `http.Request` and forward to AUTH (NOT BFF's own IP — that would break security).
- **Single source of truth for protocol** — SDK imports `github.com/webdelo/auth-proto/go/auth_v1` (external module, contract-only).

## Key Dependencies

| Library | Version | Purpose |
|---------|---------|---------|
| `github.com/webdelo/auth-proto/go` | v0.1.0 | gRPC contract for `auth.v1` (single source of truth, shared with AUTH server) |
| `google.golang.org/grpc` | pinned | gRPC framework — client side |
| `google.golang.org/protobuf` | pinned | Protobuf runtime |

**Intentionally excluded:** any DI framework (`fx`, `wire`), any concrete logger (`zap`, `logrus`), Prometheus client. These are consumer's choice.

## Scope Boundaries

**In scope:**
- Token operations: Login, Refresh, Logout, LogoutAll, ValidateToken, ChangePassword, GetCurrentUser
- HTTP middleware + gRPC unary interceptor for server-side AUTH-checks
- Context / cookie / HTTP request helpers
- Typed errors

**Out of scope:**
- Admin operations (ListUsers, CreateUser, UpdateUserRole, BlockUser) — consumer calls AUTH directly
- Session management (ListSessions, RevokeSession) — same
- Token storage policy — consumer's choice
- DI / logger / metrics implementations — adapter interfaces only

---

## CRITICAL POLICIES

### 1. Mandatory Task Execution

**ALL items are MANDATORY** — PRP points, DOD criteria, CRITICAL/WARNING/SUGGESTIONS from reviews.

- **FORBIDDEN:** Decide "out of scope" or "not critical" without user approval
- **FORBIDDEN:** Skip SUGGESTIONS as "optional"
- **REQUIRED:** Execute ALL items or explicitly ask user for permission to skip

### 2. Rules Loading

Before ANY coding task:
1. Read `.claude/guides/rules_index.yaml:1`
2. Find triggers matching task (golang, grpc, testing...)
3. Load relevant guides
4. Apply rules

Note: `.claude/guides/dba/` is largely irrelevant for this SDK (no DB layer). `.claude/guides/golang/` is the primary set.

### 3. Agent Usage

- Use `run_in_background: true` for all agents
- See `.claude/AGENTS.yaml:1` for agent catalog
- See `.claude/guides/rules_index.yaml:29` for orchestrator vs agent decision

### 4. Git Commits

- **FORBIDDEN:** Any AI attribution (Co-Authored-By, Claude, Anthropic)
- **REQUIRED:** Commits authored exclusively by user

### 5. File References

| Context | Format | Example |
|---------|--------|---------|
| CLI output | `path:line` | `lib/validate.go:42` |
| MD files | markdown links or plain path | `[validate.go](lib/validate.go)` |

### 6. File Organization

- Reports → `task_management/reviews/` or `.memory_bank/agent_task_results/`
- Results → `task_management/results/`
- Temp files → `.temp/`
- **Never create files in project root**

### 7. Public API Stability

- SDK is consumed by external code (BFF, other Go services).
- Until tag `v1.0.0` — breaking changes allowed but should be flagged in CHANGELOG.
- After `v1.0.0` — semver discipline: breaking changes only on major bumps.
- Internal package (`lib/internal/`) is free to change.
