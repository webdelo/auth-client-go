---
description: Create and push a git commit in a git worktree
argument-hint: <REQ-XXX-TASK-YYY or task file path>
---

# Generate Commit (Worktree)

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
### 1.1 Forbidden Actions
- Committing to main/master.
- Adding Co-Authored-By: Claude / Generated with Claude Code / Anthropic.
- Committing files with secrets.
- Committing with merge conflicts.
### 1.2 Commit Message Format
Prefix priority: 1. jira_id → `{jira_id}:` 2. TASK-ID → `REQ-XXX-TASK-XXX:` 3. REQ → `REQ-XXX:` 4. type

## 2. Resolve Branch
1. Extract TASK_ID from arguments or current branch name.
2. Find jira_id in `$PROJECT_ROOT/.memory_bank/tasks_list.yaml`.
3. Run `git -C $WORKTREE_DIR branch --show-current` — verify it is NOT main/master. The worktree is already on the correct branch; do NOT run `git checkout -b`.

## 3. Analyze Changes
1. `git -C $WORKTREE_DIR status` and `git -C $WORKTREE_DIR diff`. If no changes — stop.
2. Read task file from `$PROJECT_ROOT/task_management/` for context.

## 4. Stage Files
1. Include source files, tests, configs, docs, migrations, task_management/ files.
2. Exclude .env, credentials, bin/, .build/.
3. Stage source files: `git -C $WORKTREE_DIR add <file1> <file2> ...` (not `git add -A`).
4. Stage task_management/ files using full paths: `git -C $WORKTREE_DIR add $PROJECT_ROOT/task_management/<file>`.
5. Check for secrets in staged diff: `git -C $WORKTREE_DIR diff --cached`.

## 5. Create Commit
1. Determine PREFIX. Build summary (max 72 chars). Build body.
2. Verify no AI attribution.
3. `git -C $WORKTREE_DIR commit -m "..."`.

## 6. Push and Verify
1. `git -C $WORKTREE_DIR push` (or `git -C $WORKTREE_DIR push -u origin {branch_name}`).
2. If diverged: merge by default, rebase only with stated reason.
3. Verify no AI attribution: `git -C $WORKTREE_DIR log -1`.
4. Print: commit hash, task ID, prefix, file count, branch name.
5. Do NOT remove the worktree — it will be cleaned up by `/cleanup-worktrees`.

## 7. Completion Checklist
- [ ] Worktree verified (from injected context).
- [ ] Branch is NOT main/master.
- [ ] Branch matches the task.
- [ ] No AI attribution in commit.
- [ ] Prefix is correct.
- [ ] Summary line <= 72 characters.
- [ ] Body contains at least 1 bullet point.
- [ ] Only relevant files staged, no secrets.
- [ ] Merge used by default.
- [ ] Push to remote completed.
- [ ] Commit hash and summary printed.
