---
description: Aggregate PR reviewer comments into USER-CODE-REVIEW report with fix plan
argument-hint: <pr-url|pr-number|task-id>
---

# Aggregate Users Code Review

Arguments: $ARGUMENTS

Collect reviewer comments from Pull Request, create structured report, prepare fix plan.
Report is compatible with `/execute-bug-fix`.
Idempotent: repeated runs add new comments without duplicating existing ones.

## 1. Pre-flight

1. `gh auth status` — if not authorized: STOP.
2. `gh repo view --json nameWithOwner -q .nameWithOwner` — save as {OWNER}/{REPO}.

## 2. Determine PR

Parse $ARGUMENTS:

| Format | Action |
|--------|--------|
| URL `https://github.com/.../pull/NNN` | Extract NNN |
| `#NNN` or number | Use as PR number |
| Text | `gh pr list --search "TEXT"` — first result |
| `REQ-XXX-TASK-XXX` | `gh pr list --state all` — find by branch |

1. Get PR info: `gh pr view {PR_NUMBER} --json number,title,body,headRefName,baseRefName,author`.

## 3. Read Comments

1. Inline code comments: `gh api repos/{OWNER}/{REPO}/pulls/{PR_NUMBER}/comments`.
2. Review summaries: `gh api repos/{OWNER}/{REPO}/pulls/{PR_NUMBER}/reviews`.
3. General comments: `gh api repos/{OWNER}/{REPO}/issues/{PR_NUMBER}/comments`.
4. Include ALL comments, including from the PR author.
5. Note: in zsh jq expressions, do NOT use `!=` (history expansion). Use `| length > 0` instead.

## 4. Categorize

For each comment determine type:
- ISSUE (must fix): error indication, keywords "fix", "bug", "must", "incorrect", standards violation.
- REQUEST (analyze): question, suggestion, keywords "consider", "maybe", "what about", subjective opinion.

## 5. Check Existing Report

Path: `task_management/reviews/{TASK_ID}-USER-CODE-REVIEW.yaml`.
- If exists: read, save existing comment_ids, preserve IDs for continuation, increment report_version.
- If not: create from template `.claude/templates/user_code_review_report.yaml`, version 1.

## 6. Build Report

For each new comment (not in existing report by comment_id):
1. Assign ID: ISSUE → `UCR-I{N}`, REQUEST → `UCR-R{N}`.
2. Fill: comment_id, reviewer, file, line, title (first 80 chars), comment (full body), code_context (from diff_hunk), recommended_fix: null, status: open/pending_analysis.
3. Update metadata: last_updated, total_comments, report_version, reviewers list.
4. Save report.

## 7. Analyze Comments

For each comment — read actual code from project file (file:line +/- 20 lines context).

ISSUES:
1. Understand problem. Form recommended_fix. Determine complexity (simple/medium/complex).
2. Add to fix_plan.items.

REQUESTS:
- Scenario A (obvious answer): form recommended_fix, add to fix_plan.
- Scenario B (ambiguous): form question for user, add to questions_to_user.

Update fix_plan.status: "ready" if no questions, "pending" if questions exist.

## 8. User Questions (if any)

1. Print all questions with context (file, line, code, reviewer comment).
2. Wait for user answers.
3. Save answers in questions_to_user[N].answer.

## 9. Finalize Plan

1. For each REQUEST with answered questions: form recommended_fix, add to fix_plan.
2. Update fix_plan.status: "approved".
3. Save updated report.

## 10. Output

Print summary: PR number, title, date, version, issue/request counts, questions status, report path.
Next step: `/execute-bug-fix task_management/reviews/{TASK_ID}-USER-CODE-REVIEW.yaml`

## 11. Completion Checklist

- [ ] gh auth verified.
- [ ] PR identified and info loaded.
- [ ] All comment types collected (inline, review, general).
- [ ] Comments categorized (issues vs requests).
- [ ] Existing report checked for idempotency.
- [ ] Report created/updated with all new comments.
- [ ] Each comment analyzed with real code context.
- [ ] recommended_fix filled for all issues.
- [ ] Questions asked and answered (if any).
- [ ] fix_plan finalized.
- [ ] Report saved.
- [ ] Summary printed.
