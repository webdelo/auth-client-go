---
description: Create and push a git commit with proper formatting
argument-hint: <REQ-XXX-TASK-YYY or task file path>
---

# Generate Commit

Arguments: $ARGUMENTS

## 1. Rules

### 1.1 Forbidden Actions
- Committing to main/master — automatically create a feature branch.
- Adding Co-Authored-By: Claude / Generated with Claude Code / Anthropic.
- Committing files with secrets (.env, credentials, private keys).
- Committing with merge conflicts.

### 1.2 Commit Message Format

Prefix priority:
1. Task has jira_id in tasks_list.yaml → `{jira_id}:` (e.g. `FB-01:`)
2. Single task without jira_id → `REQ-XXX-TASK-YYY:`
3. Multiple tasks → `REQ-XXX:`
4. No task (hotfix, docs) → `{type}:`

Structure:
```
<PREFIX> <Summary line — max 72 chars, imperative mood>

- <Change 1>
- <Change 2>
- <Change 3>
```

## 2. Resolve Branch

1. Extract TASK_ID from arguments or current branch name.
2. Find jira_id in `.memory_bank/tasks_list.yaml` → expected branch = `feature/{jira_id}`.
3. If jira_id not found — read TASK file → `**Jira**: {JIRA_ID}`.
4. If JIRA_ID not found → expected branch = `feature/{TASK_ID}`.
5. Run `git branch --show-current`.
   - If main/master: `git checkout -b {expected_branch}`.
   - If mismatch: `git checkout -b {expected_branch}`.
   - If correct: continue.

## 3. Analyze Changes

1. Run `git status` and `git diff` (unstaged + staged). If no changes — inform user and stop.
2. Read `task_management/tasks/{TASK_ID}.md`. Extract task title and key changes.

## 4. Stage Files

1. For each file from `git status`: include or exclude per rules:
   - Include: source files, tests, configs (.yaml, .toml), docs (.md), migrations, task_management/ files.
   - Exclude: .env, credentials, *.pem, *.key, bin/, .build/, dist/, *.tmp, .DS_Store, unrelated files.
2. Run `git add <file1> <file2> ...` (by name, not `git add -A`).
3. Check for secrets: `git diff --cached | grep -i 'password\|secret\|private_key\|api_key\|token'`.
4. If secrets found — stop, remove from staging: `git reset HEAD <file>`, inform user.

## 5. Create Commit

1. Determine PREFIX by priority from section 1.2.
2. Build summary line from task title (max 72 chars, imperative mood).
3. Build body: bullet points — WHAT changed (from staged files + task DOD).
4. Verify message does NOT contain "Co-Authored-By", "Claude", "Anthropic", "Generated with".
5. Run:
```bash
git commit -m "$(cat <<'EOF'
{PREFIX} {summary line}

- {bullet 1}
- {bullet 2}
- {bullet 3}
EOF
)"
```

## 6. Push and Verify

1. `git push`. If branch is new: `git push -u origin {branch_name}`.
2. If push failed due to diverged history:
   - **DEFAULT:** `git pull --merge` (merge strategy), then `git push`.
   - **Rebase ONLY** if there is a clear, justified reason (e.g., single unpushed commit on top of updated base, or user explicitly requested rebase). When using rebase, print the reason: `⚠️ Rebase reason: {reason why merge is not suitable}`.
   - **FORBIDDEN:** Using rebase silently without stating the reason.
3. Verify no AI attribution: `git log -1 --pretty=full | grep -i 'co-authored-by.*claude\|anthropic\|generated with claude'`.
4. If found: amend commit to remove forbidden lines, repeat push.
5. Print: commit hash, task ID, prefix used, file count, branch name.

## 7. Completion Checklist

- [ ] Branch is NOT main/master.
- [ ] Branch matches the task (jira_id or TASK_ID).
- [ ] No AI attribution in commit.
- [ ] Prefix is correct (jira_id > TASK_ID > REQ_ID > type).
- [ ] Summary line <= 72 characters, imperative mood.
- [ ] Body contains at least 1 bullet point.
- [ ] Only relevant files staged, no secrets.
- [ ] Merge used by default (rebase only with stated reason).
- [ ] Push to remote completed.
- [ ] Commit hash and summary printed.
