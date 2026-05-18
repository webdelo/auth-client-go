---
description: Execute task from PRP (problem resolving plan)
argument-hint: <prp-file>
---

# Execute PRP

Arguments: $ARGUMENTS

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
- Read .claude/AGENTS.yaml ONLY ONCE (lazy loading).
- Include relevant scope file content inline in agent prompts. Do NOT tell agents to read guide files.

### 1.4 Agent Routing

| Target | Agent | Model |
|--------|-------|-------|
| *_test.go | unit-test-writer | sonnet |
| *.go new code | code-writer | sonnet |
| *.go fix error | debugger | sonnet |
| *.go mechanical | refactorer | haiku |
| Race/deadlock/flaky | debugger-complex | opus |

## 2. Create Feature Branch

1. Extract TASK_ID from PRP filename: `{TASK_ID}-PRP.md`.
2. Find jira_id in `.memory_bank/tasks_list.yaml` — branch = `feature/{jira_id}`.
3. If jira_id not found — read TASK file — find `**Jira**: {JIRA_ID}`.
4. If JIRA_ID not found — branch = `feature/{TASK_ID}`.
5. Run `git branch --show-current`. If already on correct branch — continue.
6. Otherwise: `git checkout -b {branch_name}`.

## 3. Read PRP and Load Rules

1. GATE: `git branch --show-current` — if main/master — STOP, return to section 2.
2. Move task to active: set status `in_progress` in tasks_list.yaml via `python3 .claude/scripts/req_status.py add-file {TASK-ID} prp task_management/plans/{TASK-ID}-PRP.md`
3. Read PRP file. Understand context, implementation, testing, validation.
4. Verify `.temp/rules/{TASK_ID}/` exists. If not: `python3 .claude/scripts/extract_rules.py analyze {PRP_FILE} --output-dir .temp/rules/{TASK_ID}/`
5. Read ALL scope files once:
   - `.temp/rules/{TASK_ID}/coding_scope.md`
   - `.temp/rules/{TASK_ID}/test_scope.md`
   - `.temp/rules/{TASK_ID}/review_scope.md`
6. Load libs: read `.claude/guides/libs_cheatsheet.md` → match task keywords against lib prefixes (TFY, MKR, GNK, UFX, CNC, SCR, CLG, FVO) → read full lib files for matched libs. Include relevant lib content inline in agent prompts alongside rule scopes.
7. If PRP context is insufficient — launch code-researcher / web-searcher.

## 4. Plan Work

1. Extract keywords, file_patterns, external_libs, technologies from PRP.
2. Read `.claude/AGENTS.yaml` (ONLY ONCE — lazy loading).
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
5. Tests: update .mockery.yaml (if new interfaces) — `make generate-mocks` — unit-test-writer.
6. If test_scope.md was empty, inject fallback UT rules: `python3 .claude/scripts/extract_rules.py select --rules UT-0001,UT-0002,UT-0010,UT-0020,UT-0021,UT-0022,UT-0023,UT-0024,UT-0026,UT-0030,UT-0032,UT-0040 --output-dir .temp/rules/{TASK_ID}/` — include content inline in unit-test-writer prompt.

## 6. Audit Changes

1. `make build` — FAIL: debugger — repeat until OK.
2. `make test` — FAIL in *_test.go: unit-test-writer | FAIL in *.go: debugger — repeat until OK.
3. `make test-race` — DATA RACE: debugger-complex — repeat until OK.
4. `make lint` — FAIL logic (errcheck/staticcheck/govet): debugger | mechanical (naming/fmt): refactorer — repeat until OK.
5. Launch test-reviewer IN PARALLEL (1/package) — save to `.memory_bank/agent_task_results/{DATE}_test-review_{pkg}.yaml`.
6. If CRITICAL in test-review — unit-test-writer — re-run test-reviewer.
7. `git diff` — identify changed packages — launch code-reviewer IN PARALLEL (1/package).
8. report-aggregator — create `task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`.
9. `python3 .claude/scripts/req_status.py add-file {TASK-ID} code_review task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`
10. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
11. DOD Validation: read `task_management/tasks/{TASK-ID}.md` — verify EVERY DOD criterion.
12. Run all grep/glob checks from DOD — record results.
13. If auth/credentials/sensitive data — launch security-auditor (optional).
14. If any DOD criterion not met — verdict = CHANGES_REQUESTED (even if code-reviewer gave APPROVED).
15. If verdict = CHANGES_REQUESTED — report-aggregator (bug mode) — create `task_management/bug_reports/{TASK-ID}-BUG-REPORT.yaml`.
16. `python3 .claude/scripts/req_status.py add-file {TASK-ID} bug_report task_management/bug_reports/{TASK-ID}-BUG-REPORT.yaml`

## 7. Fix Bugs

Condition: code-review verdict = CHANGES_REQUESTED or BUG-REPORT exists.

1. Read BUG-REPORT.yaml — group by severity.
2. For each CRITICAL/ERROR: race/deadlock/goroutine → debugger-complex | other → debugger.
3. WARNING mechanical → refactorer | requires logic → debugger.
4. `make build` + `make test` + `make test-race` + `make lint` — repeat until OK.
5. code-reviewer IN PARALLEL for fixed packages — report-aggregator — update CODE-REVIEW.yaml.
6. Expected: APPROVED / APPROVED_WITH_COMMENTS. If CHANGES_REQUESTED — return to step 2.
7. Set `BUG-REPORT.yaml: bugs_fixed: true` after all fixes verified.
8. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`

## 8. Finalize

1. If public API changed — update godoc/README.
2. Verify PRP: all tasks/gates/testing completed.
3. Verify: build + linter + tests + race + code review APPROVED.
4. Update `.memory_bank/project_map.yaml` and `external_libs.yaml` if project structure/deps changed.
5. Create result report: `task_management/results/{TASK-ID}-result-report.md` (template: `.claude/templates/result_report_template.md`).
6. `python3 .claude/scripts/req_status.py add-file {TASK-ID} result task_management/results/{TASK-ID}-result-report.md`
7. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
8. `python3 .claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates`
9. Finalization ends at pending_review — `/review-code` workflow takes over next.
10. Print summary: TASK-ID | Result/Code Review verdict | Commit | PR | Stats.
11. Auto-commit: invoke `/generate-commit` with the task file path from arguments.

## 9. Completion Checklist

- [ ] Feature branch created (not main/master).
- [ ] PRP file read and fully understood.
- [ ] Rules loaded from .temp/rules/{TASK_ID}/.
- [ ] Agents selected, plan printed.
- [ ] Code written via agents (orchestrator did NOT write code).
- [ ] Build passes (`make build`).
- [ ] Tests pass (`make test`, `make test-race`).
- [ ] Linter passes (`make lint`) — .golangci.yaml NOT modified.
- [ ] test-reviewer checked all test files.
- [ ] code-reviewer checked all changed packages.
- [ ] CODE-REVIEW.yaml created by report-aggregator.
- [ ] DOD validation: ALL criteria verified.
- [ ] BUG-REPORT fixed (if CHANGES_REQUESTED).
- [ ] Result report created.
- [ ] Lifecycle updated via req_status.py.
- [ ] No AI attribution in commits.
- [ ] Summary printed.
- [ ] Auto-commit completed (via /generate-commit).
