---
description: Fix bugs/issues from Bug Report or Code Review Report in a git worktree
argument-hint: <report-file>
---

# Execute Bug Fix (Worktree)

Arguments: $ARGUMENTS

Fix ALL issues from the report. ALL severity levels (CRITICAL, WARNING, SUGGESTIONS) are MANDATORY.

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

### 1.1 Mandatory Execution
- ALL issues are mandatory to fix — no exceptions without user consent.
- If unable to fix: report to user, ask "Skip issue {ID}?", wait for consent, record in report.

### 1.2 Orchestrator Role
- Orchestrator does NOT write code. Coordination and control only.
- Allowed: read reports, launch agents, run build/test/lint, update lifecycle.
- Forbidden: Edit/Write for .go files, editing tasks_list.yaml directly (use ONLY req_status.py/req_refresh.py).

### 1.3 Agent Routing

| Issue | Agent | Model | Trigger |
|-------|-------|-------|---------|
| Fix test assertion | unit-test-writer | sonnet | got X want Y in *_test.go |
| Fix mock issue | unit-test-writer | sonnet | mock setup in *_test.go |
| Fix test compile | unit-test-writer | sonnet | undefined in *_test.go |
| Add missing test | unit-test-writer | sonnet | "add test", "coverage" |
| Compile error | debugger | sonnet | undefined, type mismatch |
| Runtime error | debugger | sonnet | nil pointer, panic |
| Linter (logic) | debugger | sonnet | errcheck, staticcheck |
| Logic bug | debugger | sonnet | wrong result, off-by-one |
| DI error | debugger | sonnet | missing dependency |
| DATA RACE | debugger-complex | opus | race detected |
| Deadlock | debugger-complex | opus | all goroutines asleep |
| Goroutine leak | debugger-complex | opus | leaked goroutine |
| Flaky test | debugger-complex | opus | intermittent, sometimes |
| Multi-file bug | debugger-complex | opus | 3+ files in trace |
| Rename/format | refactorer | haiku | naming, gofmt |
| Remove dead code | refactorer | haiku | unused, delete |
| Magic number | refactorer | haiku | replace with constant |
| Linter (style) | refactorer | haiku | naming, formatting |
| Add nolint comment | refactorer | haiku | add comment |
| New function | code-writer | sonnet | "create", "implement" |
| New validation | code-writer | sonnet | "add check", "validate" |
| Auth/credentials | security-auditor | sonnet | auth, token, secret |
| Input validation | security-auditor | sonnet | injection, XSS, OWASP |

### 1.4 Agent Rules
- Catalog: `$PROJECT_ROOT/.claude/AGENTS.yaml` (read ONCE — lazy loading).
- All agents: run_in_background: true. Max 5-7 concurrent.
- Do not run multiple agents for the same file.
- Mandatory agents for validation: code-reviewer (sonnet), report-aggregator (sonnet).
- Agent prompt must include: relevant scope file content inline, guide paths by keywords. If files are in repository/ or infra/ — include DBA/DevOps rules.
- If test_scope.md is empty, inject fallback rules: `python3 $PROJECT_ROOT/.claude/scripts/extract_rules.py select --rules UT-0001,UT-0002,UT-0010,UT-0020,UT-0021,UT-0022,UT-0023,UT-0024,UT-0026,UT-0030,UT-0032,UT-0040 --output-dir $PROJECT_ROOT/.temp/rules/{TASK_ID}/`

## 2. Load Context and Rules

1. Verify `$PROJECT_ROOT/.temp/rules/{TASK_ID}/` exists. If not: `python3 $PROJECT_ROOT/.claude/scripts/extract_rules.py analyze {REPORT_FILE} --output-dir $PROJECT_ROOT/.temp/rules/{TASK_ID}/`
2. Read ALL scope files (coding_scope.md, test_scope.md, review_scope.md).

## 3. Analyze Report

1. Read report file from $ARGUMENTS. Determine type (BUG-REPORT / CODE-REVIEW).
2. Find task in `$PROJECT_ROOT/.memory_bank/tasks_list.yaml` — status: in_progress.
3. Group issues by priority: CRITICAL → WARNING → SUGGESTIONS.
4. For each issue: identify files, keywords, error type → select agent via routing table.
5. Group issues by agent (batch: one agent — all its issues).
6. Determine dependencies between groups → waves (parallel where possible).
7. Print plan: `FIX: {error_type} -> {agent} ({model}) | Trigger: {trigger}`

## 4. Pre-validation

1. Run: `cd $WORKTREE_DIR && go build ./...` && `cd $WORKTREE_DIR && make test` && `cd $WORKTREE_DIR && make lint` && `cd $WORKTREE_DIR && make test-race`.
2. If errors — select agent via routing table (compile→debugger, race→debugger-complex, style→refactorer).
3. Fix ONLY baseline errors — do NOT touch code from report issues.
4. Repeat checks until all PASS.

## 5. Fix Issues

1. For each priority (CRITICAL → WARNING → SUGGESTIONS):
   - Launch agents per wave plan (parallel within wave).
   - Each agent receives ALL issues of its group in one call.
2. Agent prompt must contain: issue list with file:line, ID, description, recommended_fix, relevant scope file content. **Include WORKTREE_DIR path — agents must use `$WORKTREE_DIR/...` for all file paths.**
3. If unable to fix: ask user, record consent in report.

## 6. Validate Fixes

1. Static checks: `cd $WORKTREE_DIR && go build ./...` && `cd $WORKTREE_DIR && make test` && `cd $WORKTREE_DIR && make lint` && `cd $WORKTREE_DIR && make test-race`. On failure: select agent → fix → repeat.
2. Code review: code-reviewer in parallel for changed packages → report-aggregator → `$PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`. If CHANGES_REQUESTED: return to section 5.
3. DOD Validation: read `$PROJECT_ROOT/task_management/tasks/{TASK-ID}.md` → find DOD → verify EVERY criterion. If any NOT MET → return to section 5.
4. Completeness: cross-check fixed issues against original report. All must be fixed or agreed with user.

## 7. Finalize

1. Create debug report: `$PROJECT_ROOT/task_management/debug_reports/{TASK-ID}-DEBUG-REPORT.yaml`.
   - Sections: metadata, pre_validation, fixes, skipped (with user consent records), summary, final_validation, dod_validation, verdict.
   - Verdict: ALL_FIXED | PARTIAL (with consent) | FAILED.
2. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py add-file {TASK-ID} debug_report $PROJECT_ROOT/task_management/debug_reports/{TASK-ID}-DEBUG-REPORT.yaml`
3. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
4. `python3 $PROJECT_ROOT/.claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates`
5. Do NOT remove the worktree — it will be cleaned up by `/cleanup-worktrees`.
6. Print summary: report file, verdict, issue counts by type (total/fixed/skipped), DOD status, check results.
7. Auto-commit: invoke `/generate-commit-worktree` with the task file path from arguments.

## 8. Completion Checklist

- [ ] Worktree verified (from injected context).
- [ ] Report file read and analyzed.
- [ ] Rules loaded from $PROJECT_ROOT/.temp/rules/{TASK_ID}/.
- [ ] Fix plan printed with agent assignments.
- [ ] Pre-validation passed (build, test, lint, race) in WORKTREE_DIR.
- [ ] ALL CRITICAL issues fixed.
- [ ] ALL WARNING issues fixed.
- [ ] ALL SUGGESTIONS fixed.
- [ ] Skipped issues have explicit user consent recorded.
- [ ] Static checks pass in WORKTREE_DIR (build, test, lint, race).
- [ ] Code review: APPROVED or APPROVED_WITH_COMMENTS.
- [ ] DOD validation passed.
- [ ] Debug report created.
- [ ] Lifecycle updated via req_status.py.
- [ ] Summary printed.
- [ ] Auto-commit completed (via /generate-commit-worktree).
