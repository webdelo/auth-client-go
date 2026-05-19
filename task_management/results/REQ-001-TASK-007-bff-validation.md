# REQ-001-TASK-007 — BFF Validation Smoke Test

## Metadata

| Field | Value |
|-------|-------|
| Validator | _PENDING — to be assigned after `v0.1.0` tag is published_ |
| Date | _PENDING_ |
| SDK version | `github.com/webdelo/auth-client-go@v0.1.0` |
| README revision | _to be filled with the commit hash containing the published README_ |

## Status

**DEFERRED** until two prerequisites are satisfied:

1. `make release` is executed by the maintainer (interactive — see PRP Task 7) and the tag
   `v0.1.0` is published to `origin/main`.
2. A second Go developer (not the author of REQ-001 / TASK-001..007) is identified as the
   BFF-validator.

## Self-review (interim)

In lieu of an external validator, the Quickstart from the published `README.md` was copied
1:1 into a fresh `go.mod` project with a `replace` directive pointing at the local working
tree. Result: `go mod tidy && go build ./...` produced a working binary in **under 5 minutes**
(install + copy-paste + build).

```bash
mkdir /tmp/authclient-smoke && cd /tmp/authclient-smoke
go mod init smoke
echo 'replace github.com/webdelo/auth-client-go => /Users/a.popov/Development/TRADING/bots/auth-client-go' >> go.mod
# main.go = README Quickstart copy-paste
go mod tidy
go build ./...  # -rwxr-xr-x  17M  smoke  → exit 0
```

This is a weaker check than the BFF-developer flow (the smoke author has full context on the
SDK and curated the README), but it rules out the most common failure modes:

- Misnamed imports.
- Method signatures that don't match the code.
- Missing types or options in the Quickstart.

## Action items if real BFF-validator reports issues

If the assigned BFF developer encounters > 30 minutes or out-of-README hints:

1. Capture the friction points in a follow-up issue.
2. Fix the README on `main`.
3. Publish a `v0.1.1` semver-patch tag — **do not re-tag `v0.1.0`** (already in
   `proxy.golang.org` cache).

## Time

- Start (smoke): 2026-05-19 12:42 UTC
- First successful `go build`: 2026-05-19 12:42 UTC
- Total (smoke): ~3 minutes (excluding README rewrite time)

## Pass / Fail

- [x] PASS — README Quickstart copy-paste compiles cleanly via a `replace` directive (smoke).
- [ ] PASS — external BFF developer completes the same flow against `@v0.1.0` in < 30 minutes (deferred).
- [ ] FAIL — `>` 30 minutes OR required out-of-README hints (deferred).

## Issues encountered (smoke)

None. The only manual fix-up needed was setting `AUTH_REPO_PATH` for the integration-test
environment, which is documented in the README ("Local Integration Testing" section).

## README change requests (smoke)

None at this time. To be filled by the external validator.
