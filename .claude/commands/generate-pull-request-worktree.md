---
description: Create pull request from a git worktree branch
argument-hint: <REQ or TASK file (optional)>
---

# Generate Pull Request (Worktree)

Arguments: $ARGUMENTS

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
### 1.1 Language
All PR content MUST be in English.
### 1.2 Attribution
No AI attribution anywhere.
### 1.3 Protected Branches
NEVER create PR from main or master.

## 2. Branch Check
1. `git -C $WORKTREE_DIR branch --show-current` — if main/master: STOP.
2. `gh auth status`.
3. `git -C $WORKTREE_DIR remote -v`.

## 3. Analyze Changes
1. Detect base branch (main or master).
2. `git -C $WORKTREE_DIR diff {base}...HEAD --stat` and `git -C $WORKTREE_DIR log {base}..HEAD --oneline`.
3. Check attribution: `git -C $WORKTREE_DIR log {base}..HEAD --format="%B"` — ensure no AI references.
4. Identify tasks from $ARGUMENTS or by branch name. Read task files from `$PROJECT_ROOT/task_management/`.

## 4. Sync with Base Branch
1. `git -C $WORKTREE_DIR fetch origin {base}`.
2. Check lag.
3. Merge by default: `git -C $WORKTREE_DIR merge origin/{base}`. Rebase only with stated reason.

## 5. Validation
1. `cd $WORKTREE_DIR && make build` — FAIL: STOP.
2. `cd $WORKTREE_DIR && make lint` — FAIL: STOP.
3. `cd $WORKTREE_DIR && make test` — FAIL: STOP.

## 6. Create PR
1. Title: `{jira_id}: Brief description` or `{type}/{REQ-XXX} - Brief description`.
2. Generate description (English): Tasks, Description, Testing, Breaking Changes.
3. `git -C $WORKTREE_DIR push -u origin {branch-name}`.
4. `cd $WORKTREE_DIR && gh pr create --title "..." --base master --assignee @me --body "..."`.

## 7. Completion
1. Check PR CI. Fix if needed.
2. Update lifecycle in `$PROJECT_ROOT/task_management/`.
3. Print PR URL.
4. Do NOT remove the worktree — it will be cleaned up by `/cleanup-worktrees`.

## 8. Completion Checklist
- [ ] Worktree verified (from injected context).
- [ ] Branch is NOT main/master.
- [ ] gh auth verified.
- [ ] No AI attribution in commits.
- [ ] Base branch synced via merge.
- [ ] Build, lint, tests pass.
- [ ] PR title and body in English only.
- [ ] PR created with --base master.
- [ ] CI checks verified.
- [ ] Lifecycle updated.
- [ ] PR URL printed.
