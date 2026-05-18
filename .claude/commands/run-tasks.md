---
description: Automatic task execution from tasks_list.yaml
argument-hint: <--dry-run | --phase N | --task TASK-ID>
---

# Run Tasks

Arguments: $ARGUMENTS

Orchestrate task execution via nested Claude sessions.

## 1. Rules

### 1.1 Nested Session Format
Inside Claude Code, env vars block nested sessions. Use stdin pipe:
```bash
echo "<prompt>" | env -u CLAUDECODE -u CLAUDE_CODE_SSE_PORT -u CLAUDE_CODE_ENTRYPOINT \
  claude -p --verbose --output-format stream-json --dangerously-skip-permissions \
  2>&1 | jq --unbuffered -r -f .claude/scripts/claude_stream_filter.jq
```

### 1.2 Git Rules
- FORBIDDEN: commit to main/master.
- FORBIDDEN: Co-Authored-By: Claude or any AI attribution.

### 1.3 Execution Phases

| Phase | Command | Mode |
|-------|---------|------|
| 1 | `/generate-prp` | parallel |
| 2 | `/execute-prp` | sequential |
| 3a | `/review-code` | parallel with 3b (after all phase 2 complete) |
| 3b | `/review-code-it` | parallel with 3a |
| 4 | `/execute-bug-fix` | sequential |

Tasks are MOVED between sections in tasks_list.yaml (pending → active → completed), never crossed out.

## 2. Analyze Task State

1. Record start time.
2. Read `.memory_bank/tasks_list.yaml`.
3. If `--phase {REQ-ID}` — filter tasks for that REQ only.
4. If `--task TASK-ID` — execute only that task.
5. If `--dry-run` — show plan only, do not execute.

## 3. Build Command List

For each task by status:

| Status | Command |
|--------|---------|
| pending PRP | `/generate-prp task_management/tasks/{TASK}.md` |
| PRP generated | `/execute-prp task_management/plans/{TASK}-PRP.md` |
| pending review | `/review-code task_management/plans/{TASK}-PRP.md` |
| needs fix | `/execute-bug-fix task_management/reviews/{TASK}-CODE-REVIEW.yaml` |

## 4. Phase 1: Generate PRP (parallel)

Blockers NOT checked — PRP generation is planning only.
Launch all tasks via Bash tool with run_in_background: true, timeout: 3600000.

## 5. Phase 2: Execute PRP (sequential)

Blockers CHECKED — task runs only if all blockers are completed/pending_review.
Execute one by one. Commit and push after each task.

## 6. Phase 3: Review (parallel 3a + 3b)

Run ONLY after ALL phase 2 tasks complete.
- Phase 3a: `/review-code` for each task (code review).
- Phase 3b: `/review-code-it` for each task (integration testing on running service).
- 3a and 3b can run in parallel for the same task.

## 7. Phase 4: Fix Bugs (sequential)

For each CODE-REVIEW with CHANGES_REQUESTED or BUG-REPORT with bugs.
Commit and push after each fix.

## 8. Summary

Print execution metrics table: task, duration, tokens, status. Total wall time, success rate.

## 9. Completion Checklist

- [ ] Start time recorded.
- [ ] tasks_list.yaml read and analyzed.
- [ ] Arguments processed (--dry-run, --phase, --task).
- [ ] Phase 1: all PRPs generated.
- [ ] Phase 2: all PRPs executed (with blocker checks).
- [ ] Phase 3: all reviews completed.
- [ ] Phase 4: all bugs fixed.
- [ ] Commits and pushes after each task.
- [ ] Execution metrics printed.
