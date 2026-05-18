---
description: Generate responses to PR reviewer comments based on applied fixes
argument-hint: <path-to-USER-CODE-REVIEW.yaml>
---

# Generate Users Review Response

Arguments: $ARGUMENTS

Generate responses to reviewer comments from first person (PR author). Short and to the point.
No publication without user approval.

## 1. Pre-flight

1. `gh auth status` — if not authorized: STOP.
2. Read USER-CODE-REVIEW.yaml from $ARGUMENTS.

## 2. Load Data

1. Parse issues and requests from report.
2. Get PR number from report metadata.
3. Get repo info: `gh repo view --json nameWithOwner`.

## 3. Check Existing Replies

1. Via GitHub API: determine which comments already have a reply from PR author.
2. Filter out already-answered comments.

## 4. Analyze Changes

For each unanswered comment:
1. Read current code at the file:line referenced.
2. Determine what was changed and how it addresses the comment.

## 5. Generate Responses

For each unanswered comment:
- "Done" — if issue fully addressed.
- 1-3 sentences — if requires explanation or was done differently.

## 6. User Confirmation

1. Show all generated responses as preview.
2. Wait for user approval or corrections.
3. User may edit individual responses.

## 7. Publish

After confirmation:
1. Post replies via GitHub API: `gh api repos/{OWNER}/{REPO}/pulls/{PR_NUMBER}/comments/{COMMENT_ID}/replies`.
2. Idempotent: skip already-replied comments.

## 8. Completion Checklist

- [ ] gh auth verified.
- [ ] Report read and parsed.
- [ ] Existing replies checked.
- [ ] Current code analyzed for each comment.
- [ ] Responses generated.
- [ ] User confirmed all responses.
- [ ] Responses published to GitHub.
