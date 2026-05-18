---
description: Aggregate PR reviewer comments in a git worktree context
argument-hint: <pr-url|pr-number|task-id>
---

# Aggregate Users Code Review (Worktree)

Arguments: $ARGUMENTS

Collect reviewer comments from Pull Request, create structured report, prepare fix plan.
Report is compatible with `/execute-bug-fix-worktree`.
Idempotent: repeated runs add new comments without duplicating existing ones.

## 0. Worktree Context

The worktree hook (`inject_worktree_hook.sh`) automatically discovers the worktree by TASK-ID.
Read the injected `# WORKTREE CONTEXT (auto-injected)` from system-reminder to get:
- `WORKTREE_DIR` — path where source code lives (run build/test/lint here)
- `PROJECT_ROOT` — path to main repo (task_management/, .memory_bank/, .claude/, .temp/)

1. **SET WORKING DIRECTORY**: Run `cd $WORKTREE_DIR` as your FIRST Bash command. This sets persistent cwd — all subsequent `make`, `go`, `lint` commands will run from worktree root automatically.
2. Verify WORKTREE_DIR exists from injected context. If missing — STOP.
3. All code operations: use `$WORKTREE_DIR/...` paths.
4. task_management/, .memory_bank/ files: use `$PROJECT_ROOT/...` paths.

## 1. Pre-flight
1. `gh auth status` — if not authorized: STOP.
2. `gh repo view --json nameWithOwner -q .nameWithOwner` — save as {OWNER}/{REPO}.

## 2. Determine PR
Parse $ARGUMENTS: URL, #NNN, text search, or TASK-ID by branch.

## 3. Read Comments
1. Inline: `gh api repos/{OWNER}/{REPO}/pulls/{PR_NUMBER}/comments`.
2. Reviews: `gh api repos/{OWNER}/{REPO}/pulls/{PR_NUMBER}/reviews`.
3. General: `gh api repos/{OWNER}/{REPO}/issues/{PR_NUMBER}/comments`.
4. Include ALL comments, including from the PR author.

## 4. Categorize
- ISSUE (must fix): error indication, "fix", "bug", "must", "incorrect".
- REQUEST (analyze): question, suggestion, "consider", "maybe".

## 5. Check Existing Report
Path: `$PROJECT_ROOT/task_management/reviews/{TASK_ID}-USER-CODE-REVIEW.yaml`.
If exists: read, preserve IDs, increment version.

## 6. Build Report
For each new comment: assign ID (UCR-I{N} or UCR-R{N}), fill fields, save.

## 7. Analyze Comments
Read actual code from `$WORKTREE_DIR/{file}` (file:line +/- 20 lines).
ISSUES: form recommended_fix, determine complexity.
REQUESTS: obvious → recommended_fix, ambiguous → question for user.

## 8. User Questions (if any)
Print questions, wait for answers, save in report.

## 9. Finalize Plan
Update fix_plan.status: "approved". Save.

## 10. Output
Print summary. Next step: `/execute-bug-fix-worktree $PROJECT_ROOT/task_management/reviews/{TASK_ID}-USER-CODE-REVIEW.yaml`

## 11. Completion Checklist
- [ ] gh auth verified.
- [ ] PR identified and info loaded.
- [ ] All comment types collected.
- [ ] Comments categorized.
- [ ] Existing report checked for idempotency.
- [ ] Report created/updated.
- [ ] Each comment analyzed with real code context from $WORKTREE_DIR.
- [ ] recommended_fix filled for all issues.
- [ ] Questions asked and answered (if any).
- [ ] fix_plan finalized.
- [ ] Report saved to $PROJECT_ROOT/task_management/reviews/.
- [ ] Summary printed.
