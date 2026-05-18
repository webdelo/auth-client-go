---
description: Create pull request based on completed tasks and current branch changes
argument-hint: <REQ or TASK file (optional)>
---

# Generate Pull Request

Arguments: $ARGUMENTS

## 1. Rules

### 1.1 Language
All PR content (title, body, all sections) MUST be in English. No Russian or mixed languages.

### 1.2 Attribution
No AI attribution anywhere. No Co-Authored-By: Claude, no "Generated with Claude Code", no Anthropic references. PR appears 100% as user's work.

### 1.3 Protected Branches
NEVER create PR from main or master. Source branch must be feature/bugfix/hotfix/refactor/docs.

## 2. Branch Check

1. `git branch --show-current` — if main/master: STOP. Create feature branch first.
2. `gh auth status` — if not authorized: STOP, run `gh auth login`.
3. `git remote -v` — verify remote exists.

## 3. Analyze Changes

1. Detect base branch (main or master).
2. `git diff {base}...HEAD --stat` and `git log {base}..HEAD --oneline` — analyze changed files and commits.
3. Check attribution: `git log {base}..HEAD --format='%s %b'` — search for forbidden patterns (Co-Authored-By: Claude, Anthropic, Generated with Claude Code). If found: STOP.
4. Identify tasks: from $ARGUMENTS, or search task_management/tasks/ by branch name. Read TASK and REQ files. Check tasks_list.yaml for jira_id.

## 4. Sync with Base Branch

1. `git fetch origin {base}`.
2. `git rev-list --count HEAD..origin/{base}` — check lag.
3. If new commits in base — **merge by default**:
   - **DEFAULT:** `git merge origin/{base}`. Then `git push`.
   - **Rebase ONLY** when explicitly justified (e.g., trivial branch with 1-2 commits and no shared history, or user explicitly requested). When using rebase, print the reason: `⚠️ Rebase reason: {reason why merge is not suitable}`.
   - If rebase is used: `git rebase origin/{base}`, then `git push --force-with-lease`.
   - **FORBIDDEN:** Using rebase as the default strategy without stating the reason.
4. On conflict: STOP, inform user to resolve manually.

## 5. Validation

1. `make build` — FAIL: STOP, fix first.
2. `make lint` — FAIL: STOP, fix first.
3. `make test` — FAIL: STOP, fix first.

## 6. Create PR

1. Title format:
   - With jira_id: `{jira_id}: Brief description` (e.g. `FB-73: Add interactive wizard`).
   - Without: `{type}/{REQ-XXX} - Brief description`.
2. Generate description sections (ALL in English):
   - **Tasks**: `- [x] TASK-XXX: Description`. Group by REQ if multiple.
   - **Description**: Overview (2-3 sentences), Key Changes (bullets), Technical Details.
   - **Fixes**: if bugs were fixed.
   - **Testing**: test coverage, validation commands run.
   - **Breaking Changes**: list or "None".
   - **Additional Notes**: optional (deployment, migration steps).
3. `git push -u origin {branch-name}`.
4. Create PR:
```bash
gh pr create \
  --title "{title}" \
  --base master \
  --assignee @me \
  --body "$(cat <<'EOF'
## Tasks
{task list}

## Description
{description sections}

## Testing
{testing info}

## Breaking Changes
**None**
EOF
)"
```

## 7. Completion

1. Wait 30s, then `gh pr checks {PR_NUMBER}`. On failure: get logs, fix, push, recheck.
2. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
3. Print: PR URL, type, task count, change summary, next steps.

## 8. Completion Checklist

- [ ] Branch is NOT main/master.
- [ ] gh auth verified.
- [ ] No AI attribution in commits.
- [ ] Base branch synced via merge (rebase only with stated reason).
- [ ] Build, lint, tests pass.
- [ ] PR title and body in English only.
- [ ] PR created with --base master.
- [ ] CI checks verified.
- [ ] Lifecycle updated.
- [ ] PR URL printed.
