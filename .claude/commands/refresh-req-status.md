---
description: Refresh task statuses for a requirement based on file evidence
argument-hint: <req-id>
---

# Refresh REQ Status

Arguments: $ARGUMENTS

## 1. Parse Request ID

Support formats: `REQ-001`, `1`, `task_management/tasks/REQ-001.md`.

## 2. Run Refresh Script

```bash
python3 .claude/scripts/req_refresh.py <req_id>
```

The script:
- Scans filesystem for all task files matching `{req_id}-TASK-*.md`.
- Finds related files (PRP, result, code review, bug report).
- Adds missing tasks, updates existing (status, files, metadata).
- Detects status based on file evidence + lifecycle data.
- Auto-regenerates `.memory_bank/tasks_list.md`.

### Status Logic

| Status | Condition |
|--------|-----------|
| pending | No PRP file |
| prp_ready | PRP exists, awaiting execution |
| in_progress | Actively being worked on |
| pending_review | Execution completed, awaiting code review |
| review_fixes | Code review done, requires fixes |
| completed | Code review passed (APPROVED / ALL_FIXED) |

## 3. Analyze Output

Identify: tasks added, tasks updated (old → new status), unchanged tasks, errors.

## 4. Report

Print summary: added/updated/unchanged counts, status transitions, `tasks_list.md regenerated`.

## 5. Suggest Next Actions

- pending_review tasks → `/review-code`
- review_fixes tasks → `/execute-bug-fix`
- pending tasks → `/generate-prp`

## 6. Duplicate Handling

If DUPLICATE_TASK_IDS_FOUND error:
```bash
python3 .claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates
```

## 7. Completion Checklist

- [ ] REQ ID parsed.
- [ ] req_refresh.py executed.
- [ ] Output analyzed.
- [ ] Summary printed.
- [ ] Next actions suggested.
