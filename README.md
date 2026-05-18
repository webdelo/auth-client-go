# auth-client-go

[![Go Reference](https://pkg.go.dev/badge/github.com/webdelo/auth-client-go.svg)](https://pkg.go.dev/github.com/webdelo/auth-client-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/webdelo/auth-client-go)](https://goreportcard.com/report/github.com/webdelo/auth-client-go)

Go SDK for [webdelo/auth](https://github.com/webdelo/auth). Thin wrapper over the gRPC contract from
[webdelo/auth-proto](https://github.com/webdelo/auth-proto) with non-trivial concerns baked in:
ValidateToken cache, HTTP middleware, gRPC unary interceptor, refresh race serialization, typed errors.

> Status: **v0.1.0 — in development.** Public API is being filled in across REQ-001 tasks.
> See [task_management/tasks/REQ-001.md](task_management/tasks/REQ-001.md) for the umbrella plan.

## Installation

```bash
go get github.com/webdelo/auth-client-go@latest
```

Requires `GOPRIVATE=github.com/webdelo/*` and a Personal Access Token with read access to the
`webdelo` organization (the contract module `github.com/webdelo/auth-proto/go` is private).

## Development

### Prerequisites

- Go 1.24+
- golangci-lint v2.11.4
- (For integration tests only) Docker + docker-compose plugin
- (For integration tests only) `webdelo/auth` repo cloned as sibling: `../auth` relative to this repo

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

### Local integration testing

Integration tests run the SDK against a real `webdelo/auth` gRPC server in Docker.

**Prerequisites**:

- Docker + docker-compose plugin available.
- `webdelo/auth` repo cloned alongside this one — by default
  `dev/deployments/compose.test.yaml` builds the auth image from `../../../auth` (relative to
  the compose file). Override via `AUTH_REPO_PATH` env var if your layout differs.

**Commands**:

```bash
make env-up              # Bring up AUTH (Postgres + migrations + auth server), wait for healthy
make env-down            # Tear down AUTH and delete volumes
make test-integration    # Full cycle: env-up → run integration tests → env-down (guaranteed)
```

`make test-integration` always tears down the environment, even when tests fail.

**Known limitations (v0.1.0):**

- If `make test-integration` is interrupted (Ctrl+C), Docker containers may remain running.
  Run `make env-down` manually to clean up.
- If `make env-up` fails partway through (for example, the AUTH image build errors out),
  `env-down` is not invoked automatically — leftover Postgres or migrate containers may stay.
  Run `make env-down` to reset before retrying.

**CI**: integration tests are an opt-in job triggered via `workflow_dispatch` with
`run_integration=true`. Run via the GitHub Actions UI or:

```bash
gh workflow run ci.yaml -f run_integration=true
```

This job has `continue-on-error: true` — it does not block PR merges. The auth image is not yet
published to GHCR (tracked separately under a DevOps REQ); for now the CI job checks out the
`webdelo/auth` repo and builds the image on every run.

## License

Internal — webdelo organization.
