# REQ-003-TASK-001: Map Refresh ABORTED to a retryable ErrRefreshRetry (v0.4.0)

| Task | Jira | PR | Date |
|------|------|----|------|
| REQ-003-TASK-001 | FB-1602 | — | 2026-11-12 |

## What changed

`(*Client).Refresh` now returns the new `ErrRefreshRetry` sentinel when AUTH answers gRPC `ABORTED` (the same refresh cookie is already being rotated, the session is alive). It is distinct from `ErrUnavailable`, so a BFF can answer 409 `REFRESH_RETRY` instead of 503. `RetryAfter(err)` reads `google.rpc.RetryInfo` from the status details for `ErrRefreshRetry` and `ErrRateLimited`, clamped to the exported `MaxRetryAfter` (5m); without a valid positive delay `ErrRefreshRetry` falls back to `DefaultRefreshRetryAfter` (1s). `HTTPMiddleware` now rounds `Retry-After` up to whole seconds (minimum 1). README and CHANGELOG describe v0.4.0.

## Deviations from the PRP and ADR

None.

## Decisions made along the way

- `ABORTED` is mapped only on the `Refresh` path. `ValidateAccessToken`, the HTTP middleware and the gRPC interceptor keep mapping it to `ErrUnavailable`, so their fail-closed behaviour is unchanged.
- `ErrRefreshRetry` is a standalone sentinel: `errors.Is(err, ErrUnavailable)` is false for it. Making it match `ErrUnavailable` would keep the 503 for any consumer that checks `ErrUnavailable` first, which is the defect being fixed. Consumers that check `ErrUnavailable` after `Refresh` must check `ErrRefreshRetry` first; this is flagged in the CHANGELOG (pre-v1.0.0 breaking change allowed). The SDK no longer logs a Warn for this outcome.
- The default hint is 1s because AUTH sends no `RetryInfo` today and its own HTTP API answers `Retry-After: 1`.
- `Refresh` tests use the existing `FakeAuthClient` harness; moving the harness to mockery is outside this task.

## Not done or moved

- Publishing the `v0.4.0` tag happens after the merge via `make release` (it tags `origin/main`).
- Checking `go get github.com/webdelo/auth-client-go@v0.4.0` from the tsa repository with the deploy PAT is left to the owner: it needs access outside this repository.

## Gate

The project declares no `precommit` slot. `make test`: 253 tests passed. `make lint` fails on the pinned linter binary (built with Go 1.26, local Go is 1.27); the equivalent direct golangci-lint run shows no findings on the changed code, only three pre-existing gosec G124 findings in the untouched cookie helpers.
