---
description: Automatic task execution from tasks_list.yaml using git worktrees
argument-hint: <--dry-run | --phase N | --task TASK-ID>
---

# Run Tasks (Worktree)

Arguments: $ARGUMENTS

Orchestrate task execution via nested Claude sessions using git worktrees.
Each task in Phase 2+ gets its own isolated worktree.

## 0. Worktree Context

The worktree hook (`inject_worktree_hook.sh`) automatically discovers the worktree by TASK-ID.
Read the injected `# WORKTREE CONTEXT (auto-injected)` from system-reminder to get:
- `WORKTREE_DIR` — path where source code lives (run build/test/lint here)
- `PROJECT_ROOT` — path to main repo (task_management/, .memory_bank/, .claude/, .temp/)

1. **SET WORKING DIRECTORY**: Run `cd $WORKTREE_DIR` as your FIRST Bash command. This sets persistent cwd — all subsequent `make`, `go`, `lint` commands will run from worktree root automatically.
2. Verify WORKTREE_DIR exists from injected context. If missing — STOP.
3. All code operations: use `$WORKTREE_DIR/...` paths.
4. task_management/, .memory_bank/ files: use `$PROJECT_ROOT/...` paths.

## 1. Rules

### 1.1 Nested Session Format
```bash
echo "<prompt>" | env -u CLAUDECODE -u CLAUDE_CODE_SSE_PORT -u CLAUDE_CODE_ENTRYPOINT \
  claude -p --verbose --output-format stream-json --dangerously-skip-permissions \
  2>&1 | jq --unbuffered -r -f $PROJECT_ROOT/.claude/scripts/claude_stream_filter.jq
```

### 1.2 Git Rules
- FORBIDDEN: commit to main/master.
- FORBIDDEN: Co-Authored-By: Claude or any AI attribution.

### 1.3 Execution Phases
| Phase | Command | Mode | Worktree |
|-------|---------|------|----------|
| 1 | `/generate-prp` | parallel | No — planning files only |
| 2 | `/execute-prp-worktree` | sequential | Yes — each task gets own worktree |
| 3a | `/review-code-worktree` | parallel with 3b | Yes — discovers existing worktree |
| 3b | `/review-code-it-worktree` | parallel with 3a | Yes — discovers existing worktree |
| 4 | `/execute-bug-fix-worktree` | sequential | Yes — discovers existing worktree |

## 2. Analyze Task State
1. Read `$PROJECT_ROOT/.memory_bank/tasks_list.yaml`.
2. Handle --phase, --task, --dry-run flags.
3. Filter tasks by requested phase or task ID.

## 3. Build Command List
Map task status to command:

| Status | Phase | Command |
|--------|-------|---------|
| todo | 1 | `/generate-prp {TASK-ID}` |
| prp-ready | 2 | `/execute-prp-worktree {TASK-ID}` |
| in-review | 3a+3b | `/review-code-worktree {TASK-ID}` + `/review-code-it-worktree {TASK-ID}` |
| bug-fix | 4 | `/execute-bug-fix-worktree {TASK-ID}` |

## 4. Phase 1 — Generate PRPs (parallel)
Run `/generate-prp` for all `todo` tasks in parallel nested sessions.
This phase does NOT use worktrees — it creates planning files in `$PROJECT_ROOT/task_management/tasks/`.

## 5. Phase 2 — Execute PRPs (sequential, worktrees)
For each `prp-ready` task:
1. Run `/execute-prp-worktree {TASK-ID}` in a nested session.
2. The hook creates `worktrees/{TASK-ID}/` automatically.
3. Wait for completion before starting next task.

## 6. Phase 3 — Review (parallel, discover worktrees)
For each `in-review` task, run in parallel:
- `/review-code-worktree {TASK-ID}` — static code review.
- `/review-code-it-worktree {TASK-ID}` — integration testing review.
Both commands discover `worktrees/{TASK-ID}/` automatically via the hook.

## 7. Phase 4 — Bug Fix (sequential, discover worktrees)
For each `bug-fix` task:
1. Run `/execute-bug-fix-worktree {TASK-ID}` in a nested session.
2. The hook discovers `worktrees/{TASK-ID}/` automatically.
3. Wait for completion before starting next task.

## 8. Summary
Print metrics:
- Tasks processed per phase.
- Success / failure counts.
- Worktrees created / reused.

## 9. Completion Checklist
- [ ] `$PROJECT_ROOT/.memory_bank/tasks_list.yaml` read.
- [ ] Flags parsed (--dry-run, --phase, --task).
- [ ] Task list filtered and mapped to commands.
- [ ] Phase 1 (generate-prp) completed in parallel if applicable.
- [ ] Phase 2 (execute-prp-worktree) completed sequentially if applicable.
- [ ] Phase 3 (review-code-worktree + review-code-it-worktree) completed in parallel if applicable.
- [ ] Phase 4 (execute-bug-fix-worktree) completed sequentially if applicable.
- [ ] Summary printed.
