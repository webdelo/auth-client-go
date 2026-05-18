---
description: Review tests against project standards with fixes
argument-hint: <test-files>
---

# Review Tests

Arguments: $ARGUMENTS

Review tests for compliance with `.claude/guides/golang/unit_tests_standards.md`, then fix and add missing tests.

## 1. Standards Checklist

- Naming: `Test<Type>_<Method>`.
- First case: `"happy_path"`.
- Mocks with `.Once()`, `.Times(0)`, `.Times(n)`.
- No `gomock.Any()` for business data (only for context, loggers).
- Table-driven tests.
- `require` for critical checks, `assert` for additional.
- No logic (if/switch) in tests — use closures in `want`.
- No magic numbers — named constants.
- Complex objects in private functions (`newTestOrder`, `newTestData`).
- `t.Parallel()` in test and each subtest.
- `tc := tc` when using t.Parallel().
- New mocks for each subtest (no sharing).

## 2. Review

1. Read `.claude/guides/golang/unit_tests_standards.md`.
2. Launch code-reviewer agents for test files — review against standards.
3. Launch debugger for failing tests (debugger-complex for flaky/race).
4. Launch unit-test-writer for missing tests.

## 3. Fix and Verify

1. Apply fixes via appropriate agents.
2. `make test` — all tests pass.
3. `make test-race` — no race conditions.
4. Generate review report.

## 4. Completion Checklist

- [ ] Standards guide read.
- [ ] All test files reviewed against checklist.
- [ ] Failing tests fixed.
- [ ] Missing tests added.
- [ ] `make test` passes.
- [ ] `make test-race` passes.
- [ ] Report generated.
