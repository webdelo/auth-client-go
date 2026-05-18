---
description: QA integration testing of a running service in a git worktree
argument-hint: <test-case-file>
---

# Execute QA Integration Testing (Worktree)

Arguments: $ARGUMENTS

Perform full QA integration testing of a running service based on a test case description. Orchestrator acts as a QA engineer.

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

## 1. Rules
### 1.1 Orchestrator Role — QA Engineer
- Orchestrator acts as a **QA engineer**: plans tests, executes them, documents results.
- Does NOT write application code. Coordination and verification only.
- Allowed: read test cases, launch agents, run build/test/lint, execute API calls via agents, ask user.
- Forbidden: Edit/Write for .go files (except test files via unit-test-writer agent).
### 1.2 Mandatory Execution
- ALL test scenarios from the test case are MANDATORY to execute.
- ALL found bugs are documented with full evidence.
- Skipping a scenario — ONLY with explicit user consent (recorded in report).
### 1.3 Bug Severity Levels
- BLOCKER — service crash, data loss, security vulnerability.
- CRITICAL — core functionality broken, no workaround.
- MAJOR — functionality broken, workaround exists.
- MINOR — cosmetic, UX, non-critical deviation from spec.
### 1.4 Agent Routing
- Environment management → it-test-agent (mandatory).
- API calls and verification on service → it-test-agent.
- Bug analysis (if needed) → code-researcher.
- Bug fix (if requested by user) → code-writer / debugger.
- Test writing (if requested) → unit-test-writer.

## 2. Load Context
1. Read test case from $ARGUMENTS — determine test scope, target service, scenarios.
2. Identify target service (signal-analyzer, strategy-tester, bff, data-loader).
3. Extract all test scenarios with expected results.
4. Build a testing checklist: scenario ID, description, preconditions, steps, expected result.

## 3. Pre-validation (GATE)
1. `cd $WORKTREE_DIR && go build ./...` — on fail: debugger → fix → retry.
2. `cd $WORKTREE_DIR && make test` — on fail: debugger → fix → retry.
3. `cd $WORKTREE_DIR && make lint` — on fail: refactorer or debugger → fix → retry.
4. `cd $WORKTREE_DIR && make test-race` — on fail: debugger-complex → fix → retry.

## 4. Start Environment
1. Launch it-test-agent: Start docker-compose, health check, build and start target service, health check endpoints. Agent must `cd $WORKTREE_DIR` before environment commands.
2. Verify: all dependencies + service are running and responding.

## 5. Test Execution
For EACH scenario from the test case:
### 5.1 Precondition Setup
- Prepare test data if specified.
### 5.2 Execute Test Steps
- Execute each step via it-test-agent. Record: request, response, logs, timing. Compare actual vs expected.
### 5.3 Classify Result
- PASS or FAIL with bug entry (severity, reproduction steps, evidence).
### 5.4 Cleanup
- Reset test data for next scenario isolation.

## 6. Edge Case & Negative Testing
1. Invalid input: empty fields, wrong types, boundary values, oversized payloads.
2. Error handling: correct error codes, meaningful error messages.
3. Concurrency: parallel requests (if applicable).
4. Idempotency: repeated requests produce consistent results (if applicable).

## 7. QA Report
1. Collect all results with evidence.
2. Path: `qa/test_data/{service}/test_results/{TEST-CASE-ID}-QA-REPORT.yaml`
3. Bug files: `qa/test_data/{service}/qa_bug_reports/BUG-{NNN}.md`
4. Validate YAML.

## 8. Finalization
1. STOP environment: `cd $WORKTREE_DIR && make dev-down-v`. Verify: `docker ps` — empty.
2. Determine verdict: ALL_PASSED / BUGS_FOUND / BLOCKED.
3. Print summary.
4. Do NOT remove the worktree — it will be cleaned up by `/cleanup-worktrees`.
5. If BUGS_FOUND: suggest `/execute-it-bug-fix {qa_report_path}`.

## 9. Completion Checklist
- [ ] Worktree verified (from injected context).
- [ ] Test case file read and analyzed.
- [ ] All scenarios extracted and checklist built.
- [ ] Pre-validation passed.
- [ ] Environment started, service healthy.
- [ ] ALL scenarios executed on running service.
- [ ] Edge case & negative testing completed.
- [ ] Each result documented with evidence.
- [ ] QA report created and YAML validated.
- [ ] Environment STOPPED (docker ps — empty).
- [ ] Summary printed with verdict.
