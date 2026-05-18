---
description: Review tests against project standards in a git worktree
argument-hint: <test-files>
---

# Review Tests (Worktree)

Arguments: $ARGUMENTS

Review tests for compliance with `$PROJECT_ROOT/.claude/guides/golang/unit_tests_standards.md`, then fix and add missing tests.

## 0. Worktree Context

The worktree hook (`inject_worktree_hook.sh`) automatically discovers the worktree by TASK-ID.
Read the injected `# WORKTREE CONTEXT (auto-injected)` from system-reminder to get:
- `WORKTREE_DIR` — path where source code lives (run build/test/lint here)
- `PROJECT_ROOT` — path to main repo (task_management/, .memory_bank/, .claude/, .temp/)

1. **SET WORKING DIRECTORY**: Run `cd $WORKTREE_DIR` as your FIRST Bash command. This sets persistent cwd — all subsequent `make`, `go`, `lint` commands will run from worktree root automatically.
2. Verify WORKTREE_DIR exists from injected context. If missing — STOP.
3. GATE: `git -C $WORKTREE_DIR branch --show-current` — must NOT be main/master.
4. All `make`, `go build`, `go test` commands run from WORKTREE_DIR (set by step 1).
5. All `python3 .claude/scripts/...` commands: use `$PROJECT_ROOT` paths.
6. Agent prompts MUST include `WORKTREE_DIR` path and instruction to `cd $WORKTREE_DIR` before code ops.
7. Read/Edit/Write *.go files: use `$WORKTREE_DIR/...` paths.
8. task_management/, .memory_bank/ files: use `$PROJECT_ROOT/...` paths.

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
1. Read `$PROJECT_ROOT/.claude/guides/golang/unit_tests_standards.md`.
2. Launch code-reviewer agents for test files in `$WORKTREE_DIR/...` — review against standards.
3. Launch debugger for failing tests (debugger-complex for flaky/race).
4. Launch unit-test-writer for missing tests.

## 3. Fix and Verify
1. Apply fixes via appropriate agents (Read/Edit/Write test files using `$WORKTREE_DIR/...` paths).
2. `cd $WORKTREE_DIR && make test` — all tests pass.
3. `cd $WORKTREE_DIR && make test-race` — no race conditions.
4. Generate review report.
5. Do NOT remove the worktree — it will be cleaned up by `/cleanup-worktrees`.

## 4. Completion Checklist
- [ ] Worktree verified (from injected context).
- [ ] Standards guide read.
- [ ] All test files reviewed against checklist.
- [ ] Failing tests fixed.
- [ ] Missing tests added.
- [ ] `cd $WORKTREE_DIR && make test` passes.
- [ ] `cd $WORKTREE_DIR && make test-race` passes.
- [ ] Report generated.
