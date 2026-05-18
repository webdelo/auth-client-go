---
description: Run tasks in parallel via separate Claude sessions
argument-hint: [--max N] <task-description | --config file.yaml>
---

# Run Parallel

Arguments: $ARGUMENTS

Launch tasks in parallel Claude sessions.

## 0. Parse Arguments

Extract `--max N` from arguments (default: 10, range: 1–10).
Remaining text after removing `--max N` is the task description or `--config`.

Example: `--max 3 code review domain/services/*` → max=3, task="code review domain/services/*"

## 1. Rules

### 1.1 Nested Session Format
```bash
echo "<prompt>" | env -u CLAUDECODE -u CLAUDE_CODE_SSE_PORT -u CLAUDE_CODE_ENTRYPOINT \
  /home/gremlin/.local/bin/claude -p --verbose --output-format stream-json --dangerously-skip-permissions \
  2>&1 | python3 -u .claude/scripts/claude_stream_filter.py | tee .temp/<task-name>.log
```

### 1.2 Monitoring
Shell details does not show nested session output. Log files via `tee`.
Manual check: `tail -f .temp/<task-name>.log`.
Orchestrator waits passively for completion notifications — do NOT poll.

## 2. Determine Task Type

| Type | Example | Parallelization |
|------|---------|-----------------|
| Code review | `code review domain/services/*` | By package |
| Test review | `test review internal/pkg/*` | By package |
| Unit tests | `unit tests for domain/entities/*` | By package |
| Lint fix | `fix lint errors` | By file/package |
| Research | `find usage of X` | By directory |
| Custom | `--config file.yaml` | From config |

## 3. Analyze and Decompose

If `--config`: read YAML config file.
If text description:
1. Identify targets (packages, files, directories).
2. Build command for each target.
3. Batch size: if tasks ≤ max → all parallel. If tasks > max → batches of max.

## 4. Launch

Use Bash tool with run_in_background: true, timeout: 3600000.
ALL Bash calls in ONE message for parallel launch.
Each command writes to log: `.temp/<task-name>.log`.

## 5. Wait and Collect

1. Print table of launched tasks with log file paths.
2. Wait for completion notifications. Do NOT poll.
3. When all done: read log files, build results table.

## 6. Aggregate

1. For code/test review: launch report-aggregator agent.
2. For other tasks: print results summary.
3. Print metrics: task, duration, tokens, status, total wall time, time saved vs sequential.

## 7. Completion Checklist

- [ ] Task type determined.
- [ ] Targets identified and commands built.
- [ ] All tasks launched in parallel.
- [ ] Log file paths printed.
- [ ] All tasks completed.
- [ ] Results aggregated.
- [ ] Metrics printed.
