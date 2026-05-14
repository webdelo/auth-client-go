# Task Management Guide

## Unified Task List

**Primary file**: `.memory_bank/tasks_list.md` — auto-generated overview of all tasks.

## Commands

```bash
# View tasks
cat .memory_bank/tasks_list.md

# Regenerate tasks_list.md
python3 .claude/scripts/req_status.py generate-md

# Refresh statuses + regenerate
/refresh-req-status REQ-001
python3 .claude/scripts/req_refresh.py REQ-001

# Register file in tasks_list.yaml
python3 .claude/scripts/req_status.py add-file <TASK-ID> <TYPE> <FILE-PATH>
# Types: prp, result, code_review, bug_report, debug_report, task

# Refresh lifecycle fields
python3 .claude/scripts/req_status.py refresh-lifecycle <TASK-ID>
```

## Task Status Flow

| Status | Icon | Condition |
|--------|------|-----------|
| pending | ⏳ | No PRP file |
| prp_ready | 📋 | PRP exists, awaiting execution |
| in_progress | 🔄 | Actively being worked on |
| pending_review | 👀 | Execution completed, awaiting code review |
| review_fixes | 🔧 | Code review done, requires fixes |
| completed | ✅ | Code review passed (APPROVED/ALL_FIXED) |

## Critical Rules

> **Code Review обязателен для completed статуса**

- **ЗАПРЕЩЕНО** ставить `status: completed` без файла CODE-REVIEW.yaml
- **ЗАПРЕЩЕНО** писать "Code Review: APPROVED" в result report — это НЕ code review
- После `/execute-prp` задача → `pending_review`, НЕ `completed`

## Lifecycle Tracking

Tasks track: Execution → Code Review → Bug Fixing

```bash
# After result report
python3 .claude/scripts/req_status.py refresh-lifecycle REQ-001-TASK-007
# Sets: lifecycle.execution.completed = true

# After code review
python3 .claude/scripts/req_status.py refresh-lifecycle REQ-001-TASK-007
# Sets: lifecycle.code_review.verdict = "APPROVED_WITH_COMMENTS"
```

## Benefits of Scripts

- ⚡ 10-20x faster than Claude tools
- 💰 Saves 5,000-15,000 tokens per operation
- 🎯 Consistent, deterministic logic
- 📄 Auto-generates unified tasks_list.md
