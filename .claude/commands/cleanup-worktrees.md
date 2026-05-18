---
description: Remove merged worktrees and prune stale branches
argument-hint: (no arguments)
---

# Cleanup Worktrees

Scan `worktrees/` directory. For each worktree, check if its PR has been merged.
Remove only worktrees with merged PRs.

## 1. Rules

- ONLY remove worktrees whose PR is confirmed merged via `gh pr list`.
- NEVER remove worktrees with open or draft PRs.
- NEVER remove worktrees without any PR (they may be work-in-progress).
- Always run `git worktree prune` after removals.
- Clear stale `.temp/active_worktree.env` if it references a removed worktree.

## 2. Scan Worktrees

1. `ls worktrees/` — list all TASK-ID directories.
2. If directory is empty — print "No worktrees found." and stop.
3. For each directory:
   a. Read `.worktree-meta.yaml` → extract `branch`.
   b. If no meta file: `git -C worktrees/{TASK-ID} branch --show-current` → get branch.
   c. Query PR status: `gh pr list --head {branch} --state all --json number,state,mergedAt --limit 1`.

## 3. Classify

For each worktree, determine action:

| PR State | Action |
|----------|--------|
| merged | REMOVE — safe to delete |
| open / draft | KEEP — work in progress |
| closed (not merged) | KEEP — may need investigation |
| no PR found | KEEP — not yet pushed/created |

## 4. Remove Merged

For each worktree marked REMOVE:
1. `git worktree remove worktrees/{TASK-ID} --force`
2. `git branch -d {branch}` (delete local branch, safe — already merged)
3. If branch delete fails (not fully merged warning): `git branch -D {branch}` after verifying PR was merged.

## 5. Cleanup

1. `git worktree prune` — remove stale worktree references.
2. If `.temp/active_worktree.env` exists and references a removed worktree → `rm .temp/active_worktree.env`.

## 6. Summary

Print table:

```
## Worktree Cleanup Results

| TASK-ID | Branch | PR | Status | Action |
|---------|--------|----|--------|--------|
| REQ-001-TASK-082 | feature/FB-120 | #42 | merged | REMOVED |
| REQ-001-TASK-083 | feature/FB-121 | #43 | open | KEPT |
| REQ-002-TASK-010 | feature/FB-130 | — | no PR | KEPT |

Removed: N worktrees
Kept: M worktrees
```

## 7. Completion Checklist

- [ ] worktrees/ directory scanned.
- [ ] Each worktree's PR status checked via gh.
- [ ] Only merged worktrees removed.
- [ ] Local branches deleted for removed worktrees.
- [ ] `git worktree prune` executed.
- [ ] Stale active_worktree.env cleared.
- [ ] Summary printed.
