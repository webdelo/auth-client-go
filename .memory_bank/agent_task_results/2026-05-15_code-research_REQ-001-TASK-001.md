# Code Research Report — REQ-001 TASK-001: Bootstrap Deps + Integration Test Infra

**Date:** 2026-05-15
**Researcher:** code-researcher (claude-sonnet-4-6)
**Task:** REQ-001-TASK-001 — go.mod deps, compose.test.yaml, env-up/env-down/test-integration Makefile targets, CI integration job

---

## Section 1: Sibling Repo Patterns

### 1.1 auth repo (`/Users/a.popov/Development/TRADING/bots/auth/`)

The auth repo is the primary reference and the server that compose.test.yaml must bring up.

**Dockerfile strategy** (`/Users/a.popov/Development/TRADING/bots/auth/Dockerfile`):

Multi-stage: `ci` (golang:1.24-alpine + golangci-lint) → `build` (compiles to `/auth`) → `production` (alpine:3.20 + ca-certificates + tzdata + wget).

The `production` stage copies the binary **and** `./migrations` directory (needed for the migrate container). The binary implements a built-in `healthcheck` subcommand:

```dockerfile
FROM alpine:3.20 AS production

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

COPY --from=build /auth .
COPY --from=build /app/migrations ./migrations

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ./auth healthcheck --http http://localhost:53000/healthcheck --grpc localhost:50051 || exit 1

ENTRYPOINT ["./auth"]
```

The `healthcheck` subcommand is a Cobra command in `/Users/a.popov/Development/TRADING/bots/auth/cmd/healthcheck.go`. It calls `system.GRPCChecker` which dials the gRPC server and calls `grpc_health_v1.Health/Check`. This uses the standard gRPC health protocol — so `grpc_health_probe` **or** the built-in `./auth healthcheck --grpc` can both be used.

**Port layout** (from `generated/scratch/system/config.go`):
- `APP_ADDR` → `:8080` (HTTP REST, only when `FEATURES_HTTP_ENABLED=true`)
- `GRPC_ADDR` → `:50051` (gRPC, always)
- `SYSTEM_ADDR` → `:53000` (internal system/health HTTP, always)

**compose.yaml profiles** (`/Users/a.popov/Development/TRADING/bots/auth/compose.yaml`):

```yaml
services:
  auth-app:
    build: { context: ., target: ci, args: [GO_VERSION=1.24] }
    env_file: [dev/deployments/compose.dev.env]
    ports: ["8080:8080", "50051:50051"]
    depends_on:
      auth-migrate: { condition: service_completed_successfully }
    profiles: [app]
    healthcheck:
      test: [CMD, go, run, main.go, healthcheck, --http, http://localhost:53000/healthcheck, --grpc, localhost:50051]
      interval: 30s  timeout: 10s  start_period: 10s  retries: 3

  auth-postgres:
    image: postgres:16-alpine
    environment: { POSTGRES_USER: auth, POSTGRES_PASSWORD: auth, POSTGRES_DB: auth, POSTGRES_HOST_AUTH_METHOD: trust }
    ports: ["5432:5432"]
    volumes: [postgres_data:/var/lib/postgresql/data]
    healthcheck: { test: [CMD-SHELL, "pg_isready -U auth"], interval: 5s, timeout: 5s, retries: 5 }
    profiles: [app, dependencies, test]

  auth-migrate:
    image: migrate/migrate:v4.17.0
    depends_on: { auth-postgres: { condition: service_healthy } }
    volumes: [./migrations:/migrations]
    command: [-path, /migrations/, -database, "postgres://auth:auth@auth-postgres:5432/auth?sslmode=disable", up]
    profiles: [app, dependencies, test]
```

Key observations:
- `auth-postgres` appears in the `test` profile — we can reuse it directly with `--profile test`
- `auth-migrate` also appears in `test` profile
- `auth-app` is only in `app` profile — **there is no `test` profile entry for auth-app itself**. This is because auth's integration tests use bufconn (in-process server), not a real server over TCP.
- The compose.yaml dev `auth-app` healthcheck uses `go run main.go healthcheck` — slow. The `production` stage uses `./auth healthcheck` — fast. **For compose.test.yaml, build the `production` target**.

**auth ENV variables** (from `dev/deployments/compose.test.env` and `auth.env`):

```dotenv
# Required minimum for compose.test.yaml:
LOGGER_LEVEL=debug
FEATURES_LOG_INCOMING_REQUESTS=false
FEATURES_HTTP_ENABLED=true          # need HTTP for healthcheck at :53000
INTERNAL_TIMEOUT=5s
EXTERNAL_TIMEOUT=10s
POSTGRES_HOST=auth-postgres:5432    # docker service name
POSTGRES_USER=auth
POSTGRES_PASSWORD=auth
POSTGRES_DATABASE=auth
AUTH_JWT_KEY_PATH=/tmp/auth-test-keys
AUTH_JWT_ACCESS_TTL=15m
AUTH_JWT_REFRESH_TTL=336h
AUTH_JWT_ISSUER=auth-test
AUTH_JWT_AUDIENCE=webdelo-test
AUTH_PEPPER_VALUE=test-pepper-value-for-testing-min32!!
AUTH_PEPPER_VERSION=1
AUTH_HASHER_MEMORY=65536
AUTH_HASHER_ITERATIONS=1            # low for test speed
AUTH_HASHER_PARALLELISM=1
AUTH_HASHER_SALT_LENGTH=16
AUTH_HASHER_KEY_LENGTH=32
AUTH_RATE_LIMIT_IP_MAX=1000         # high to not block tests
AUTH_RATE_LIMIT_IP_WINDOW=1m
AUTH_RATE_LIMIT_EMAIL_MAX=100
AUTH_RATE_LIMIT_EMAIL_WINDOW=15m
AUTH_RATE_LIMIT_LOCKOUT_BASE=1m
AUTH_RATE_LIMIT_LOCKOUT_MAX=15m
AUTH_RATE_LIMIT_LOCKOUT_MULTIPLIER=2.0
AUTH_RATE_LIMIT_CLEANUP_INTERVAL=10m
AUTH_RATE_LIMIT_ADMIN_WRITE_MAX=1000
AUTH_RATE_LIMIT_ADMIN_WRITE_WINDOW=1m
AUTH_RATE_LIMIT_SESSION_WRITE_MAX=1000
AUTH_RATE_LIMIT_SESSION_WRITE_WINDOW=1m
AUTH_HTTP_COOKIE_DOMAIN=
AUTH_HTTP_COOKIE_SECURE=false
AUTH_HTTP_COOKIE_SAME_SITE=strict
```

Note: `AUTH_ADMIN_SEED_EMAIL` and `AUTH_ADMIN_SEED_PASSWORD` are left empty (seed disabled in tests).

**JWT keys**: AUTH expects RSA/EC keys at `AUTH_JWT_KEY_PATH`. For integration tests, the test runner must generate keys at `/tmp/auth-test-keys` before starting the container — or mount a `keys/` directory. The auth repo has a `keys/` directory in repo root (excluded from git). For compose.test.yaml, we need to either: (a) use a Docker volume with pre-generated keys, or (b) use a `command` that generates keys on startup. The simplest approach: bind-mount a host-side keys directory, or generate keys as a one-shot init container.

**gRPC health service registration** (`generated/scratch/grpc/health/server.go`):

```go
package health

import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
)

func New() *Server { return &Server{healthServer: health.NewServer()} }

func (s *Server) Register(server *grpc.Server) {
    grpc_health_v1.RegisterHealthServer(server, s.healthServer)
}
```

This is registered in `internal/app/app_create.go` via `fxs_grpc_health.Server`. Conclusion: **AUTH registers `grpc.health.v1.Health` — grpc_health_probe is viable**.

The auth `healthcheck` subcommand itself uses `system.GRPCChecker` (from `generated/scratch/system/healthcheck.go`) which calls `grpc_health_v1.NewHealthClient(conn).Check(...)`. So the built-in `./auth healthcheck --grpc localhost:50051` is functionally equivalent to `grpc_health_probe -addr=localhost:50051`.

**Makefile patterns** (`/Users/a.popov/Development/TRADING/bots/auth/Makefile`):

```makefile
test-integration: $(TEST_RUNNER) ## Run integration tests with coverage
	@$(MAKE) dev-up-test-env
	@mkdir -p $(COVERAGE_DIR)
	@APP_CONF=$(CURDIR)/dev/deployments/compose.test.env RUN_INTEGRATION_TESTS=true \
		$(TEST_RUNNER) -tags integration -integration -title "Integration Tests" ./tests/...; \
	TEST_EXIT=$$?; \
	$(MAKE) dev-down-test-env; \
	exit $$TEST_EXIT

env-up: dev-up-db ## Alias for dev-up-db

env-down: dev-down ## Alias for dev-down
```

Note: auth uses `TEST_EXIT=$$?; ...; exit $$TEST_EXIT` pattern rather than bash `trap`. The auth `dev-up-test-env` in `extra.mk` is currently a **no-op** (prints "Starting test environment..." and "Test environment ready" with no actual docker compose command). This is because auth's integration tests use bufconn and connect directly to a real PostgreSQL, not a real auth server over TCP.

**Integration test build tag and bootstrap** (`/Users/a.popov/Development/TRADING/bots/auth/tests/main_test.go`):

```go
//go:build integration

package tests_test

import (
    "fmt"
    "os"
    "testing"
    "github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
    if cfgFile := os.Getenv("APP_CONF"); cfgFile != "" {
        if err := godotenv.Load(cfgFile); err != nil {
            fmt.Fprintf(os.Stderr, "tests: load env from %s: %v\n", cfgFile, err)
            os.Exit(1)
        }
    }
    os.Exit(m.Run())
}
```

The SDK integration tests will be simpler (no need for godotenv — we'll export ENV vars directly from make). The pattern to borrow: `//go:build integration` tag on every test file in `lib/`, and a `TestMain` that skips unless `RUN_INTEGRATION_TESTS=true`.

**CI** (`/Users/a.popov/Development/TRADING/bots/auth/.github/workflows/ci.yaml`): No integration job — only lint, test (unit), build. The `workflow_dispatch` example exists in `openapi-publish.yaml` — shows how to add an optional manual trigger.

---

### 1.2 signal-analyzer (`/Users/a.popov/Development/TRADING/bots/signal-analyzer/`)

**Makefile test-integration target** (`/Users/a.popov/Development/TRADING/bots/signal-analyzer/Makefile`):

```makefile
test-integration: $(TEST_RUNNER) ## Run integration tests (with auto env setup)
	@$(MAKE) dev-up-test-env
	@APP_CONF=$(CURDIR)/dev/deployments/compose.test.env RUN_INTEGRATION_TESTS=true \
		$(TEST_RUNNER) -tags integration -integration -title "Integration Tests" ./tests/...; \
	TEST_EXIT=$$?; \
	$(MAKE) dev-down-test-env; \
	exit $$TEST_EXIT
```

**extra.mk dev-up-test-env** (`/Users/a.popov/Development/TRADING/bots/signal-analyzer/dev/deployments/extra.mk`):

```makefile
dev-up-test-env: ## Start test environment (PostgreSQL + ClickHouse + migrations)
	@echo "Starting test environment..."
	${_COMPOSE} --profile test up -d --wait trade-signal-analyzer-postgres trade-signal-analyzer-clickhouse
	@echo "Running PostgreSQL migrations..."
	${_COMPOSE} run --rm trade-signal-analyzer-migrate
	@echo "Running ClickHouse migrations..."
	${_COMPOSE} run --rm trade-signal-analyzer-clickhouse-migrate
	@echo "Test environment ready"

dev-down-test-env: ## Stop test environment
	@echo "Stopping test environment..."
	${_COMPOSE} --profile test down --remove-orphans -v
	@echo "Test environment stopped"
```

Key pattern: `docker compose --profile test up -d --wait <services>` — the `--wait` flag blocks until named services are healthy. Then migrations run as one-shot containers. **This is the cleanest pattern for auth-client-go**: bring up auth-postgres with `--wait`, then run auth-migrate, then bring up auth-app with `--wait`.

**Integration test build tag** (`signal-analyzer/tests/main_test.go`):

```go
//go:build integration

package tests

import (
    "os"
    "testing"
)

func TestMain(m *testing.M) {
    if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
        return
    }
    os.Exit(m.Run())
}
```

**Aliases** in signal-analyzer Makefile:
```makefile
env-up: dev-up-db ## Alias for dev-up-db
env-down: dev-down ## Alias for dev-down
```

These env-up/env-down aliases are for the dev database, not the full test environment. For TASK-001 in auth-client-go, `env-up` should probably alias `dev-up-test-env` (full auth stack), not just the DB.

**CI** (`/Users/a.popov/Development/TRADING/bots/signal-analyzer/.github/workflows/ci.yaml`): Three jobs — lint, test (unit), build — all at `go-version: '1.24'`. No integration job, no `workflow_dispatch`. No label-based gating.

---

### 1.3 funding-bot (`/Users/a.popov/Development/TRADING/bots/funding/src/`)

**test-integration with TEST_EXIT pattern** (`/Users/a.popov/Development/TRADING/bots/funding/src/makefile`, lines 301-313):

```makefile
test-integration: $(TEST_RUNNER) ## Run integration tests (with auto env setup)
	@echo "━━━ Integration Tests ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@$(MAKE) --no-print-directory dev-up-test-env
	@ENV=test RUN_INTEGRATION_TESTS=true $(TEST_RUNNER) --integration --tags=integration \
		--title="Integration Tests" ./tests/...; \
	TEST_EXIT=$$?; \
	echo ""; \
	echo "Stopping test environment..."; \
	$(MAKE) --no-print-directory dev-down-test-env; \
	exit $$TEST_EXIT
```

The `dev-up-test-env` in funding-bot uses a loop: `until curl -sf http://localhost:4771/api/health/readiness > /dev/null 2>&1; do sleep 1; done`. For auth-client-go, `docker compose up -d --wait` is cleaner (no polling loop in shell).

**No GitHub workflows found** for funding-bot (no `.github/workflows/` directory).

---

### 1.4 auth-proto (`/Users/a.popov/Development/TRADING/bots/auth-proto/`)

**Tag confirmed**: `go/v0.1.0` exists in the repo.

**go.mod** (`/Users/a.popov/Development/TRADING/bots/auth-proto/go/go.mod`):

```
module github.com/webdelo/auth-proto/go

go 1.24.0

require (
    google.golang.org/grpc v1.79.3
    google.golang.org/protobuf v1.36.11
)

require (
    golang.org/x/net v0.48.0 // indirect
    golang.org/x/sys v0.39.0 // indirect
    golang.org/x/text v0.32.0 // indirect
    google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)
```

**Transitive versions that MVS will select for auth-client-go:**
- `google.golang.org/grpc v1.79.3` (direct from auth-proto/go)
- `google.golang.org/protobuf v1.36.11` (direct from auth-proto/go)
- `golang.org/x/net v0.48.0` (indirect)
- `golang.org/x/sys v0.39.0` (indirect)
- `golang.org/x/text v0.32.0` (indirect)
- `google.golang.org/genproto/googleapis/rpc` (indirect)

After adding `golang.org/x/sync` directly for singleflight, MVS will pick a version >= what auth-proto/go's transitive graph already has (auth server's `go.sum` shows `golang.org/x/sync v0.20.0`).

**Package structure** (`auth-proto/go/auth_v1/`): `auth_service.pb.go` + `auth_service_grpc.pb.go` — only two files, clean import.

**Import path**: `github.com/webdelo/auth-proto/go/auth_v1` — note the module name is `github.com/webdelo/auth-proto/go`, package path adds `/auth_v1`.

---

## Section 2: Current State of auth-client-go

**go.mod** (`/Users/a.popov/Development/TRADING/bots/auth-client-go/go.mod`):

```
module github.com/webdelo/auth-client-go

go 1.26.2
```

Only the module declaration. No dependencies at all.

**Makefile** (`/Users/a.popov/Development/TRADING/bots/auth-client-go/Makefile`):

Existing targets: `test`, `test-race`, `bench`, `coverage`, `fmt`, `vet`, `lint`, `lint-fix`, `audit`, `tidy`, `clean`, `build-examples`, `generate-mocks`, `init-mem-bank`, `update-mem-bank`, `release`, `release-dry-run`, `release-with-notes`, `help`.

Missing targets that TASK-001 must add: `env-up`, `env-down`, `test-integration`.

The Makefile does NOT include `dev/deployments/extra.mk` (only includes `dev/tools/tools.mk`). TASK-001 must either add the `-include dev/deployments/extra.mk` line or put `env-up`/`env-down`/`test-integration` directly in the Makefile.

**Existing test runner** (`dev/tools/test-runner-main.go` + `dev/tools/bin/test-runner`): The test runner binary already exists and is used by `make test`. The `test-integration` target should use it with `-tags integration` flag, same as auth/signal-analyzer.

**CI** (`/Users/a.popov/Development/TRADING/bots/auth-client-go/.github/workflows/ci.yaml`):

Jobs: lint, test (unit), build. All at `go-version: '1.24'`. GOPRIVATE not set — will need `git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"` for auth-proto/go (private module).

No `workflow_dispatch`, no integration job.

**lib/** skeleton:
- `lib/lib.go`: `const Version = "0.0.1"` — placeholder only
- `lib/lib_test.go`: `TestVersion` — unit test placeholder

**dev/deployments/**: Does NOT exist. Must be created.

---

## Section 3: Go Version Mismatch — go.mod 1.26.2 vs CI 1.24

**Finding: Critical inconsistency.**

| Source | Go version |
|--------|-----------|
| `go.mod` | `go 1.26.2` |
| `.github/workflows/ci.yaml` | `go-version: '1.24'` |
| `CLAUDE.md` | "Go 1.24" |
| `tech_stack.yaml` | `language_version: "1.24"` |
| `auth-proto/go/go.mod` | `go 1.24.0` |
| `auth/go.mod` | `go 1.25.0` |
| `signal-analyzer/Makefile` | `GO_VERSION ?= 1.24` |
| `auth/Makefile` | `GO_VERSION ?= 1.24` |

`go.mod` says `1.26.2` — this is inconsistent with all other references. Go 1.26 does not exist as of the knowledge cutoff (latest stable is 1.24.x / 1.25.x toolchain). This is likely a bootstrap artifact from the `scratch` tool generating with a future toolchain version or auto-detecting the local Go installation.

**Decision for TASK-001**: Align `go.mod` downward to `go 1.24` to match CLAUDE.md, tech_stack.yaml, auth-proto/go, and CI. All sibling repos use 1.24 in their CI. The CI already pins `go-version: '1.24'` — a `go.mod` of `1.26.2` will cause `go mod tidy` to emit a warning about toolchain mismatch in CI.

Alternatively: align CI up to `1.25` or whatever the auth server uses. But auth server itself uses 1.24 in CI (only `go.mod` says 1.25, which is a similar issue there). The safest choice is `go 1.24` everywhere.

---

## Section 4: gRPC Health Probe vs TCP Probe

**AUTH server DOES register `grpc.health.v1.Health`.**

Evidence chain:
1. `auth/generated/scratch/grpc/health/server.go` calls `grpc_health_v1.RegisterHealthServer(server, s.healthServer)`.
2. This is wired into the app via `fxs_grpc_health.Server` in `internal/app/app_create.go`.
3. The auth `Dockerfile` healthcheck uses `./auth healthcheck --grpc localhost:50051` which internally calls `grpc_health_v1.NewHealthClient.Check()`.
4. The auth compose.yaml dev healthcheck uses the same via `go run main.go healthcheck --grpc localhost:50051`.

**Recommendation: Use `./auth healthcheck --grpc localhost:50051` as the HEALTHCHECK in compose.test.yaml.**

This avoids pulling an external `grpc_health_probe` binary into the Docker image. The auth `production` image already includes the `./auth` binary which has the `healthcheck` subcommand built in.

**Fallback**: If we build the `production` target, the HEALTHCHECK is already baked into the image (see Dockerfile). We don't need to override it in compose.test.yaml — we just inherit it.

**Alternative (simple TCP probe)**: For a lightweight compose.test.yaml that doesn't need the full healthcheck chain, a TCP probe works too:

```yaml
healthcheck:
  test: ["CMD-SHELL", "nc -z localhost 50051 || exit 1"]
  interval: 5s
  timeout: 3s
  start_period: 20s
  retries: 10
```

But the built-in `./auth healthcheck --grpc` is more semantically correct (verifies the gRPC service is actually responding, not just the TCP port is open).

---

## Section 5: CI Gating Pattern

**None of the sibling repos (auth, signal-analyzer, funding-bot) have an integration CI job.** The auth repo has `workflow_dispatch` in `openapi-publish.yaml` but only for that specific publish workflow.

**No label-gate or workflow_dispatch integration job exists anywhere in the platform** to copy from directly. TASK-001 must introduce the pattern.

ADR §A13 specifies: `if: ${{ inputs.run_integration }}` (workflow_dispatch input).

**Recommended pattern** (based on ADR §A13 and auth CI structure):

```yaml
# In .github/workflows/ci.yaml, add this job:
on:
  push:
    branches: [main, master]
  pull_request:
    branches: [main, master]
  workflow_dispatch:
    inputs:
      run_integration:
        description: 'Run integration tests (requires Docker + AUTH image)'
        type: boolean
        default: false

jobs:
  # ... existing lint, test, build jobs ...

  integration:
    name: Integration Tests
    runs-on: ubuntu-latest
    if: ${{ inputs.run_integration == true }}
    continue-on-error: true   # non-blocking: PR can merge even if this fails
    env:
      GOPRIVATE: github.com/webdelo/*
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: false

      - name: Configure git for private modules
        run: git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"

      - name: Download modules
        run: go mod download

      - name: Run integration tests
        run: make test-integration
```

**Why `continue-on-error: true`**: ADR §A13 says "non-blocking". `continue-on-error` makes the job appear yellow (warning) but does not block PR merge. Alternatively, omit `needs: [integration]` from any dependent jobs.

**Why `workflow_dispatch` input rather than label gate**: Simpler, no need to create/manage labels. Label gate (`if: contains(github.event.pull_request.labels.*.name, 'integration')`) is an alternative but requires label management. The `workflow_dispatch` input is triggered manually (`gh workflow run ci.yaml -f run_integration=true`) or from GitHub UI.

**Caveat**: When triggered by `push` or `pull_request` (not `workflow_dispatch`), `inputs.run_integration` is `null`, so the condition `${{ inputs.run_integration == true }}` evaluates to `false` — the job is skipped. This is the desired behavior.

---

## Section 6: Structural Findings and Additional Notes

### 6.1 compose.test.yaml — Auth image strategy

The auth `production` Docker image must be built locally from the auth server repo. There is no GHCR-published image yet (the ADR acknowledges this as accepted limitation). **Strategy for compose.test.yaml:**

```yaml
services:
  auth-app:
    build:
      context: /path/to/auth  # local build
      target: production
    # OR once GHCR is set up:
    # image: ghcr.io/webdelo/auth/production:latest
```

For local development, the `context` can be a relative or absolute path to the auth repo. For CI, either:
- Build the auth image as a CI step before running tests, or
- Pull from GHCR (future, pending DevOps REQ)

In TASK-001, use the local build strategy (`context:` pointing to the auth repo) with a clear comment that this requires the auth repo to be checked out at the sibling path.

### 6.2 compose.test.yaml — Key generation

AUTH requires RSA/ECDSA keys at `AUTH_JWT_KEY_PATH`. Options:
1. **Simplest**: Use a separate `keygen` init container (one-shot) that generates keys into a shared Docker volume, then auth-app reads from that volume.
2. **Alternative**: Pre-commit a test keypair in `dev/deployments/keys/` (not secrets — test-only throwaway keys). Mount as bind-mount.

The auth repo itself has a `keys/` directory (gitignored). Option 2 (committed test keys) is simplest for reproducibility. Auth's `compose.test.env` sets `AUTH_JWT_KEY_PATH=/tmp/auth-test-keys` — implying key generation happens externally. Need to investigate how auth generates test keys.

### 6.3 `make test-integration` — trap vs TEST_EXIT pattern

Both auth and signal-analyzer use the `TEST_EXIT=$$?; $(MAKE) dev-down-test-env; exit $$TEST_EXIT` pattern. Neither uses bash `trap`.

The ADR says "через `trap` чтобы env гарантированно гасился" — but the sibling repos prove `TEST_EXIT` pattern is equivalent and simpler in Makefile context (bash `trap` in Makefile recipes is fragile because each recipe line runs in a subshell).

**Recommendation**: Use the `TEST_EXIT` pattern from auth/signal-analyzer, not bash `trap`. In a Makefile recipe, the entire `@...;` chain within a single recipe step runs in one shell, so `TEST_EXIT=$$?; exit $$TEST_EXIT` achieves the same cleanup guarantee.

### 6.4 GOPRIVATE in CI

auth-client-go imports `github.com/webdelo/auth-proto/go` (private). The existing CI does NOT have `GOPRIVATE` set — only has the `git config` workaround. auth repo's CI does:

```yaml
env:
  GOPRIVATE: github.com/webdelo/*
...
- name: Configure git for private modules
  run: git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"
```

TASK-001 should add `GOPRIVATE: github.com/webdelo/*` to the CI env (both existing jobs and the new integration job).

### 6.5 Test package layout

auth-client-go SDK tests will live in `lib/` (not a separate `tests/` directory like auth server), because SDK integration tests test the client, not a server. File: `lib/client_integration_test.go` with `//go:build integration`.

The `TestMain` for SDK integration tests does NOT need godotenv or APP_CONF — env vars from `test.env` will be exported directly by the Makefile with `AUTH_ENDPOINT=localhost:50051`.

---

## Recommendations for the PRP

1. **Align go.mod to `go 1.24`** (not 1.26.2). All sibling repos, CI, CLAUDE.md, and auth-proto/go all use 1.24. The current `go 1.26.2` is a bootstrap artifact that will cause `go mod tidy` warnings in CI.

2. **Use local `build: { context: ../auth, target: production }` in compose.test.yaml** for the auth-app service. The `production` target bakes in the `HEALTHCHECK` instruction already (`./auth healthcheck --grpc localhost:50051`), so Docker will automatically use it. No need to specify `grpc_health_probe` as a separate binary.

3. **Healthcheck strategy: use the built-in `./auth healthcheck` subcommand**, not `grpc_health_probe`. The `production` image includes it. For compose.test.yaml service spec, rely on the Dockerfile HEALTHCHECK being inherited. In `dev-up-test-env`, use `docker compose up -d --wait auth-app` to block until the HEALTHCHECK passes.

4. **Copy signal-analyzer's `dev-up-test-env` / `dev-down-test-env` pattern** from `dev/deployments/extra.mk`: `docker compose --profile test up -d --wait <services>`, followed by one-shot migrate container. Then add `-include dev/deployments/extra.mk` to the Makefile.

5. **Use `TEST_EXIT=$$?; $(MAKE) dev-down-test-env; exit $$TEST_EXIT` in test-integration** (not bash trap). This is the established platform pattern, works reliably in Makefile recipes, and is simpler.

6. **For CI integration job gating**, use `workflow_dispatch` with a boolean input `run_integration`. The condition `if: ${{ inputs.run_integration == true }}` auto-skips on push/PR, and `continue-on-error: true` makes it non-blocking. Also add `GOPRIVATE: github.com/webdelo/*` to all CI job envs.

7. **Handle JWT keys**: Commit a throwaway RSA test keypair to `dev/deployments/keys/` (bind-mounted as a volume in compose.test.yaml). Set `AUTH_JWT_KEY_PATH=/keys` inside the container. This avoids the need for a key-generation init container and is self-contained for CI.

8. **`make env-up` should alias `dev-up-test-env`** (full auth stack), not `dev-up-db` (which would only be PostgreSQL). The test.env file needs `AUTH_ENDPOINT=localhost:50051` plus the full set of AUTH env vars documented in Section 1.1. Do NOT add deps to the SDK module from the test env file (the env file is pure shell, not Go code).

---

## Key File Paths Reference

| Purpose | Path |
|---------|------|
| AUTH Dockerfile (multi-stage, production target) | `/Users/a.popov/Development/TRADING/bots/auth/Dockerfile` |
| AUTH compose.yaml (service profiles, healthchecks) | `/Users/a.popov/Development/TRADING/bots/auth/compose.yaml` |
| AUTH compose.test.env (full ENV layout) | `/Users/a.popov/Development/TRADING/bots/auth/dev/deployments/compose.test.env` |
| AUTH auth.env (JWT/pepper/rate-limit secrets template) | `/Users/a.popov/Development/TRADING/bots/auth/dev/deployments/auth.env` |
| AUTH extra.mk (dev-up/down-test-env stubs) | `/Users/a.popov/Development/TRADING/bots/auth/dev/deployments/extra.mk` |
| AUTH gRPC health server registration | `/Users/a.popov/Development/TRADING/bots/auth/generated/scratch/grpc/health/server.go` |
| AUTH system/healthcheck.go (GRPCChecker impl) | `/Users/a.popov/Development/TRADING/bots/auth/generated/scratch/system/healthcheck.go` |
| AUTH port config (APP=8080, GRPC=50051, SYSTEM=53000) | `/Users/a.popov/Development/TRADING/bots/auth/generated/scratch/system/config.go:23-25` |
| auth-proto go.mod (grpc v1.79.3, protobuf v1.36.11) | `/Users/a.popov/Development/TRADING/bots/auth-proto/go/go.mod` |
| signal-analyzer extra.mk (dev-up-test-env with --wait) | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/dev/deployments/extra.mk` |
| signal-analyzer Makefile test-integration | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/Makefile:65-71` |
| signal-analyzer tests/main_test.go (TestMain pattern) | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/tests/main_test.go` |
| auth Makefile test-integration | `/Users/a.popov/Development/TRADING/bots/auth/Makefile:55-62` |
| auth tests/main_test.go (APP_CONF/godotenv pattern) | `/Users/a.popov/Development/TRADING/bots/auth/tests/main_test.go` |
| auth-client-go go.mod (currently bare) | `/Users/a.popov/Development/TRADING/bots/auth-client-go/go.mod` |
| auth-client-go Makefile (existing, no env-up/test-integration) | `/Users/a.popov/Development/TRADING/bots/auth-client-go/Makefile` |
| auth-client-go CI (no integration job, no GOPRIVATE) | `/Users/a.popov/Development/TRADING/bots/auth-client-go/.github/workflows/ci.yaml` |
| auth-client-go lib placeholder | `/Users/a.popov/Development/TRADING/bots/auth-client-go/lib/lib.go` |
