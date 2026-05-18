---
description: Execute task from PRP in a git worktree
argument-hint: <prp-file>
---

# Execute PRP (Worktree)

Arguments: $ARGUMENTS

## 0. Worktree Setup

The worktree hook (`inject_worktree_hook.sh`) automatically creates/finds the worktree.
Read the injected `# WORKTREE CONTEXT (auto-injected)` from system-reminder to get:
- `WORKTREE_DIR` — path where source code lives (run build/test/lint here)
- `PROJECT_ROOT` — path to main repo (task_management/, .memory_bank/, .claude/, .temp/)

1. **SET WORKING DIRECTORY**: Run `cd $WORKTREE_DIR` as your FIRST Bash command. This sets persistent cwd — all subsequent `make`, `go`, `lint` commands will run from worktree root automatically.
2. Verify WORKTREE_DIR exists from injected context. If missing — STOP.
3. GATE: `git -C $WORKTREE_DIR branch --show-current` — must NOT be main/master.
4. All `make`, `go build`, `go test` commands run from WORKTREE_DIR (set by step 1).
5. All `python3 .claude/scripts/...` commands: use `$PROJECT_ROOT` paths.
6. Agent prompts MUST include `WORKTREE_DIR` path and instruction to `cd $WORKTREE_DIR` before code ops.
7. Read/Edit/Write *.go files: use `$WORKTREE_DIR/internal/...` paths.
8. task_management/, .memory_bank/ files: use `$PROJECT_ROOT/...` paths.

## 1. Rules

### 1.1 Orchestrator Role
- Orchestrator does NOT write code — only coordinates agents.
- Allowed: read/analyze PRP, launch agents, run shell commands (git, make), edit YAML/MD files.
- Exception: orchestrator MAY directly make 1-2 line trivial fixes or edit YAML/MD files.

### 1.2 Forbidden Actions
- Writing production code in *.go files.
- Writing tests (*_test.go files).
- Modifying .golangci.yaml.
- Adding Co-Authored-By: Claude or any AI attribution to commits.
- Continuing work on main/master branch.
- Setting status: completed without a real CODE-REVIEW.yaml file.
- Editing tasks_list.yaml directly — use ONLY req_status.py and req_refresh.py.
- Skipping PRP items without explicit user consent.
- Treating SUGGESTIONS from code review as optional — they are MANDATORY.

### 1.3 Agent Rules
- All agents launched with run_in_background: true.
- Multiple Agent tool calls in ONE message for parallel execution.
- Max 5-7 agents at once.
- Read $PROJECT_ROOT/.claude/AGENTS.yaml ONLY ONCE (lazy loading).
- Include relevant scope file content inline in agent prompts. Do NOT tell agents to read guide files.

### 1.4 Agent Routing

| Target | Agent | Model |
|--------|-------|-------|
| *_test.go | unit-test-writer | sonnet |
| *.go new code | code-writer | sonnet |
| *.go fix error | debugger | sonnet |
| *.go mechanical | refactorer | haiku |
| Race/deadlock/flaky | debugger-complex | opus |

## 2. Verify Worktree Branch

The worktree hook already created the branch and worktree.
1. Read WORKTREE_DIR and WORKTREE_BRANCH from injected context.
2. Verify: `git -C $WORKTREE_DIR branch --show-current` matches expected branch.
3. If mismatch — STOP with error.

## 3. Read PRP and Load Rules

1. GATE: `git -C $WORKTREE_DIR branch --show-current` — if main/master — STOP, return to section 2.
2. Move task to active: set status `in_progress` in tasks_list.yaml via `python3 $PROJECT_ROOT/.claude/scripts/req_status.py add-file {TASK-ID} prp $PROJECT_ROOT/task_management/plans/{TASK-ID}-PRP.md`
3. Read PRP file. Understand context, implementation, testing, validation.
4. Verify `$PROJECT_ROOT/.temp/rules/{TASK_ID}/` exists. If not: `python3 $PROJECT_ROOT/.claude/scripts/extract_rules.py analyze {PRP_FILE} --output-dir $PROJECT_ROOT/.temp/rules/{TASK_ID}/`
5. Read ALL scope files once:
   - `$PROJECT_ROOT/.temp/rules/{TASK_ID}/coding_scope.md`
   - `$PROJECT_ROOT/.temp/rules/{TASK_ID}/test_scope.md`
   - `$PROJECT_ROOT/.temp/rules/{TASK_ID}/review_scope.md`
6. Load libs: read `$PROJECT_ROOT/.claude/guides/libs_cheatsheet.md` → match task keywords against lib prefixes (TFY, MKR, GNK, UFX, CNC, SCR, CLG, FVO) → read full lib files for matched libs. Include relevant lib content inline in agent prompts alongside rule scopes.
7. If PRP context is insufficient — launch code-researcher / web-searcher.

## 4. Plan Work

1. Extract keywords, file_patterns, external_libs, technologies from PRP.
2. Read `$PROJECT_ROOT/.claude/AGENTS.yaml` (ONLY ONCE — lazy loading).
3. Select agents by triggers + add mandatory agents (unit-test-writer, code-reviewer, test-reviewer, report-aggregator).
4. Print agent table with selection reasons.
5. Decompose PRP tasks into subtasks with agent assignments.
6. Build dependency graph (B depends on A if: import, shared type, same file).
7. Topological sort — waves: Wave 1 (no dependencies), Wave 2, ... Wave N.
8. TaskCreate for each subtask (UI progress tracking).

## 5. Execute Changes

1. For each wave: run subtasks IN PARALLEL (multiple Agent calls in one message, run_in_background: true).
2. Monitor agents: code-writer/refactorer 10m | unit-test-writer/debugger 12m | other 15m. Max 3 retries.
3. If agent failed — debugger — retry.
4. Production code via: code-writer / refactorer / repository-* / api-designer.
5. Tests: update .mockery.yaml (if new interfaces) — `cd $WORKTREE_DIR && make generate-mocks` — unit-test-writer.
6. If test_scope.md was empty, inject fallback UT rules: `python3 $PROJECT_ROOT/.claude/scripts/extract_rules.py select --rules UT-0001,UT-0002,UT-0010,UT-0020,UT-0021,UT-0022,UT-0023,UT-0024,UT-0026,UT-0030,UT-0032,UT-0040 --output-dir $PROJECT_ROOT/.temp/rules/{TASK_ID}/` — include content inline in unit-test-writer prompt.

## 6. Audit Changes

1. `cd $WORKTREE_DIR && make build` — FAIL: debugger — repeat until OK.
2. `cd $WORKTREE_DIR && make test` — FAIL in *_test.go: unit-test-writer | FAIL in *.go: debugger — repeat until OK.
3. `cd $WORKTREE_DIR && make test-race` — DATA RACE: debugger-complex — repeat until OK.
4. `cd $WORKTREE_DIR && make lint` — FAIL logic (errcheck/staticcheck/govet): debugger | mechanical (naming/fmt): refactorer — repeat until OK.
5. Launch test-reviewer IN PARALLEL (1/package) — save to `$PROJECT_ROOT/.memory_bank/agent_task_results/{DATE}_test-review_{pkg}.yaml`.
6. If CRITICAL in test-review — unit-test-writer — re-run test-reviewer.
7. `git -C $WORKTREE_DIR diff` — identify changed packages — launch code-reviewer IN PARALLEL (1/package).
8. report-aggregator — create `$PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`.
9. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py add-file {TASK-ID} code_review $PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`
10. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
11. DOD Validation: read `$PROJECT_ROOT/task_management/tasks/{TASK-ID}.md` — verify EVERY DOD criterion.
12. Run all grep/glob checks from DOD — record results.
13. If auth/credentials/sensitive data — launch security-auditor (optional).
14. If any DOD criterion not met — verdict = CHANGES_REQUESTED (even if code-reviewer gave APPROVED).
15. If verdict = CHANGES_REQUESTED — report-aggregator (bug mode) — create `$PROJECT_ROOT/task_management/bug_reports/{TASK-ID}-BUG-REPORT.yaml`.
16. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py add-file {TASK-ID} bug_report $PROJECT_ROOT/task_management/bug_reports/{TASK-ID}-BUG-REPORT.yaml`

## 7. Fix Bugs

Condition: code-review verdict = CHANGES_REQUESTED or BUG-REPORT exists.

1. Read BUG-REPORT.yaml — group by severity.
2. For each CRITICAL/ERROR: race/deadlock/goroutine → debugger-complex | other → debugger.
3. WARNING mechanical → refactorer | requires logic → debugger.
4. `cd $WORKTREE_DIR && make build` + `cd $WORKTREE_DIR && make test` + `cd $WORKTREE_DIR && make test-race` + `cd $WORKTREE_DIR && make lint` — repeat until OK.
5. code-reviewer IN PARALLEL for fixed packages — report-aggregator — update CODE-REVIEW.yaml.
6. Expected: APPROVED / APPROVED_WITH_COMMENTS. If CHANGES_REQUESTED — return to step 2.
7. Set `BUG-REPORT.yaml: bugs_fixed: true` after all fixes verified.
8. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`

## 8. Finalize

1. If public API changed — update godoc/README.
2. Verify PRP: all tasks/gates/testing completed.
3. Verify: build + linter + tests + race + code review APPROVED.
4. Update `$PROJECT_ROOT/.memory_bank/project_map.yaml` and `external_libs.yaml` if project structure/deps changed.
5. Create result report: `$PROJECT_ROOT/task_management/results/{TASK-ID}-result-report.md` (template: `$PROJECT_ROOT/.claude/templates/result_report_template.md`).
6. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py add-file {TASK-ID} result $PROJECT_ROOT/task_management/results/{TASK-ID}-result-report.md`
7. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
8. `python3 $PROJECT_ROOT/.claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates`
9. Finalization ends at pending_review — `/review-code` workflow takes over next.
10. Print summary: TASK-ID | Result/Code Review verdict | Commit | PR | Stats.
11. Do NOT remove the worktree — it will be cleaned up by `/cleanup-worktrees`.
12. Auto-commit: invoke `/generate-commit-worktree` with the task file path from arguments.

## 9. Completion Checklist

- [ ] Worktree verified (from injected context).
- [ ] PRP file read and fully understood.
- [ ] Rules loaded from $PROJECT_ROOT/.temp/rules/{TASK_ID}/.
- [ ] Agents selected, plan printed.
- [ ] Code written via agents (orchestrator did NOT write code).
- [ ] Build passes (`cd $WORKTREE_DIR && make build`).
- [ ] Tests pass (`cd $WORKTREE_DIR && make test`, `cd $WORKTREE_DIR && make test-race`).
- [ ] Linter passes (`cd $WORKTREE_DIR && make lint`) — .golangci.yaml NOT modified.
- [ ] test-reviewer checked all test files.
- [ ] code-reviewer checked all changed packages.
- [ ] CODE-REVIEW.yaml created by report-aggregator.
- [ ] DOD validation: ALL criteria verified.
- [ ] BUG-REPORT fixed (if CHANGES_REQUESTED).
- [ ] Result report created.
- [ ] Lifecycle updated via req_status.py.
- [ ] No AI attribution in commits.
- [ ] Summary printed.
- [ ] Auto-commit completed (via /generate-commit-worktree).
