---
description: Write Gonkey integration tests (YAML-based) for REST/JSON API
argument-hint: <endpoint-or-feature>
---

# Write Gonkey Tests

Arguments: $ARGUMENTS

## 1. Required Reading

Before writing tests, read:
1. `.claude/guides/libs/gonkey.md` — Gonkey framework reference.
2. `.claude/guides/golang/integration_tests_standards.md` — integration test standards.

## 2. Standards

### 2.1 Project Structure
- Tests: `tests/cases/<category>/<endpoint>/`.
- Fixtures: `tests/fixtures/` (postgres/, valkey/, base_templates.yaml, empty_db.yaml).
- Mocks: `tests/mocks_data/<service>/<endpoint>/`.
- Runner registration: `tests/integration_test.go`.

### 2.2 File Naming
- `+happy_path.yaml` — positive tests.
- `-error_case.yaml` — negative tests.

### 2.3 Test Case Format
- BDD naming: "WHEN [condition] THEN [result]".
- Each test starts with `fixtures: [empty_db, ...]`.
- `$matchRegexp()` for dynamic fields (UUID, timestamps).
- `comparisonParams.ignoreArraysOrdering: true` where needed.

### 2.4 Mocks
- `calls: N` — ALWAYS specify call count.
- `requestConstraints` — validate requests to external services.
- `calls: 0` + `strategy: nop` for verifying no calls made.

### 2.5 Database
- `dbQuery` + `dbResponse` for DB state verification.
- JSON aggregation for readable comparison.
- `ignoreDbOrdering: true/false` explicitly set.

### 2.6 Critical
- Every directory in `tests/cases/` must contain at least one `.yaml` file.
- Do NOT add empty directories to `CaseDirs`.

## 3. Process

### 3.1 Analyze API
1. Study handler or OpenAPI spec.
2. Identify all endpoints, request/response formats.
3. Identify DB and external service dependencies.

### 3.2 Prepare Structure
```bash
mkdir -p tests/cases/<category>/<endpoint>
mkdir -p tests/fixtures/postgres/<feature>
mkdir -p tests/mocks_data/<service>/<endpoint>
```

### 3.3 Create Fixtures
- `base_templates.yaml` with common templates.
- `empty_db.yaml` for table cleanup.
- Feature-specific fixtures with test data.

### 3.4 Write Tests
- Happy path (`+*.yaml`).
- Error cases (`-*.yaml`).
- Each test isolated (starts with empty_db).

### 3.5 Register in integration_test.go
```go
func TestFeatureCases(t *testing.T) {
    (&runner.TestRunner{
        T: t,
        CaseDirs: []runner.TestCase{
            {Dir: "cases/<category>/<endpoint>", Name: "<category>/<endpoint>"},
        },
    }).Run(t)
}
```

### 3.6 Run and Verify
```bash
RUN_INTEGRATION_TESTS=true \
OUTBOXER_TEST_CONF=${PWD}/dev/deployments/test.env \
go test -v ./tests/... -run TestFeatureCases
```

## 4. Coverage Categories

- Happy path — successful execution.
- Validation errors — invalid input (400).
- Not found — nonexistent resources (404).
- Unauthorized — missing/invalid auth (401/403).
- External service errors — mock failures.
- Database state — verify DB changes via dbQuery.

## 5. Commit

1. Auto-commit: invoke `/generate-commit` with the task file path from arguments.

## 6. Completion Checklist

- [ ] gonkey.md and integration_tests_standards.md read.
- [ ] API analyzed (endpoints, formats, dependencies).
- [ ] Directory structure created.
- [ ] Fixtures created (empty_db + feature-specific).
- [ ] Happy path tests written.
- [ ] Error case tests written.
- [ ] Mocks with `calls: N` and `requestConstraints`.
- [ ] DB verification with `dbQuery`/`dbResponse`.
- [ ] `$matchRegexp()` for dynamic fields.
- [ ] No empty directories in CaseDirs.
- [ ] Registered in integration_test.go.
- [ ] Tests pass.
- [ ] Auto-commit completed (via /generate-commit).
