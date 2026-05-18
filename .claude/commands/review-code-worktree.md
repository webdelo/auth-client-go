---
description: Code review for completed task in a git worktree
argument-hint: <prp-file>
---

# Code Review (Worktree)

Arguments: $ARGUMENTS

Code review does NOT fix code — only documents issues.
To fix issues: `/execute-bug-fix $PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`

## 0. Worktree Context

The worktree hook (`inject_worktree_hook.sh`) automatically discovers the worktree.
Read the injected `# WORKTREE CONTEXT (auto-injected)` from system-reminder to get:
- `WORKTREE_DIR` — path where source code lives
- `PROJECT_ROOT` — path to main repo (task_management/, .memory_bank/, .claude/, .temp/)

1. **SET WORKING DIRECTORY**: Run `cd $WORKTREE_DIR` as your FIRST Bash command. This sets persistent cwd — all subsequent `make`, `go`, `lint` commands will run from worktree root automatically.
2. Verify WORKTREE_DIR exists from injected context. If missing — STOP.
3. All code analysis and `git diff` commands: run from WORKTREE_DIR via `cd $WORKTREE_DIR && ...` or `git -C $WORKTREE_DIR ...`
4. Agent prompts MUST include `WORKTREE_DIR` path and instruction to `cd $WORKTREE_DIR` before code operations.
5. Read/Edit *.go files: use `$WORKTREE_DIR/...` paths.
6. task_management/, .memory_bank/ files: use `$PROJECT_ROOT/...` paths.
7. Guide/rule files: use `$PROJECT_ROOT/.claude/guides/...`

## 1. Rules

### 1.1 Forbidden Actions
- Fixing code — only documenting issues.
- Manual report aggregation — ONLY via report-aggregator agent.
- Skipping packages/issues without user consent.
- Orchestrator writing code. All code analysis — via agents.
- Editing tasks_list.yaml directly — use ONLY req_status.py.

### 1.2 Output Files
- Intermediate: `$PROJECT_ROOT/.memory_bank/agent_task_results/{DATE}_code-review_{package}.yaml`
- Intermediate: `$PROJECT_ROOT/.memory_bank/agent_task_results/{DATE}_test-review_{package}.yaml`
- Final: `$PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`

## 2. Preparation

1. Read `$PROJECT_ROOT/task_management/tasks/{TASK-ID}.md` — save DOD criteria for section 5.
2. Read `$PROJECT_ROOT/task_management/plans/{TASK-ID}-PRP.md` — understand scope and changes.
3. Determine scope:
   - From PRP: "Affected files" section.
   - `git -C $WORKTREE_DIR diff --name-only | grep '\.go$' | xargs -I{} dirname {} | sort -u`
4. Extract keywords from TASK/PRP: technologies, patterns, package types, concurrency primitives.
5. Load rules: verify `$PROJECT_ROOT/.temp/rules/{TASK_ID}/` exists. If not: `python3 $PROJECT_ROOT/.claude/scripts/extract_rules.py analyze {PRP_FILE} --output-dir $PROJECT_ROOT/.temp/rules/{TASK_ID}/`
6. Read ALL scope files once (coding_scope.md, test_scope.md, review_scope.md).
7. Load libs: read `$PROJECT_ROOT/.claude/guides/libs_cheatsheet.md` → match task keywords against lib prefixes (TFY, MKR, GNK, UFX, CNC, SCR, CLG, FVO) → read full lib files for matched libs. Include relevant lib content inline in agent prompts alongside rule scopes.

## 3. Data Collection (Parallel Agents)

1. Launch code-reviewer agents IN PARALLEL (1 per package):
   - Include scope file content inline in prompt.
   - Include `WORKTREE_DIR` and instruction to `cd $WORKTREE_DIR` before code operations.
   - Save to `$PROJECT_ROOT/.memory_bank/agent_task_results/{DATE}_code-review_{package}.yaml`.
2. Launch test-reviewer agents IN PARALLEL:
   - Include `WORKTREE_DIR` and instruction to `cd $WORKTREE_DIR` before code operations.
   - Save to `$PROJECT_ROOT/.memory_bank/agent_task_results/{DATE}_test-review_{package}.yaml`.
3. If code touches auth/credentials/sensitive data — launch security-auditor.
4. Validate YAML of all agent reports: `python3 -c "import yaml; yaml.safe_load(open('{path}'))"`.
5. Fix invalid YAML before proceeding to aggregation.

## 4. Aggregation

1. Launch report-aggregator agent (do NOT aggregate manually):
   - Input: all `{DATE}_code-review_*.yaml` + `{DATE}_test-review_*.yaml`.
   - Output: `$PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`.
   - Requirements: include ALL issues, validate counts, assign global IDs (CR-001, CR-002, ...), preserve `original_id` and `source_package` per issue, fill `execution_metrics` section (agent, duration, tokens, cost).
   - Pass `sub_agents_metrics` collected from agent results into aggregator prompt.
2. Validate aggregation: all sub-agent reports processed, counts match, all IDs from sources present, `execution_metrics` filled, YAML valid.
3. If validation fails — retry report-aggregator.

## 5. DOD Validation

Code-reviewer checks QUALITY, DOD checks COMPLETENESS — both mandatory.

1. For each DOD criterion: Grep/Glob check (run from `$WORKTREE_DIR`), compare with expected, record: criterion, result, status (MET | NOT MET).
2. If any DOD criterion = NOT MET — verdict = CHANGES_REQUESTED.

## 6. Finalization

1. Determine verdict:
   - APPROVED: no CRITICAL/WARNING + DOD PASS.
   - APPROVED_WITH_COMMENTS: no CRITICAL, has WARNING/SUGGESTION + DOD PASS.
   - CHANGES_REQUESTED: has CRITICAL or DOD FAIL.
2. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py add-file {TASK-ID} code_review $PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`
3. `python3 $PROJECT_ROOT/.claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
4. Print summary: TASK-ID, date, verdict, issue counts by package, DOD status, execution metrics (per-agent duration/tokens/cost + totals).
5. If CHANGES_REQUESTED: print next step — `/execute-bug-fix $PROJECT_ROOT/task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`

## 7. Completion Checklist

- [ ] TASK and PRP files read.
- [ ] Scope determined (packages + files).
- [ ] Rules loaded from $PROJECT_ROOT/.temp/rules/{TASK_ID}/.
- [ ] code-reviewer launched for all changed packages (with WORKTREE_DIR).
- [ ] test-reviewer launched for all test files (with WORKTREE_DIR).
- [ ] All agent reports are valid YAML.
- [ ] report-aggregator created final CODE-REVIEW.yaml.
- [ ] DOD validation completed — ALL criteria checked.
- [ ] Verdict determined.
- [ ] CODE-REVIEW.yaml registered via req_status.py.
- [ ] Lifecycle updated.
- [ ] Summary printed.
