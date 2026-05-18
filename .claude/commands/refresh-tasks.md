---
description: Sync task statuses based on task_management/ files
argument-hint: "[req-id]"
---

# Refresh Tasks

Arguments: $ARGUMENTS

Sync debug reports, update lifecycle, rebuild tasks_list.yaml and tasks_list.md.

## 1. Determine REQ IDs

If $ARGUMENTS provided — use as REQ-ID.
Otherwise — scan `task_management/tasks/` and collect unique REQ IDs:
```bash
ls task_management/tasks/REQ-*-TASK-*.md 2>/dev/null | sed 's|.*/\(REQ-[0-9]*\)-TASK.*|\1|' | sort -u
```

## 2. Sync Debug Reports

For each TASK ID under each REQ:

### 2.1 Refresh code review lifecycle (run FIRST)
Scan `task_management/reviews/{REQ-ID}-TASK-*-CODE-REVIEW.yaml`. For each found task:
```bash
python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}
```

### 2.2 Register debug reports and refresh lifecycle
1. Check for `task_management/debug_reports/{TASK-ID}-DEBUG-REPORT.yaml`.
2. If exists and NOT registered in tasks_list.yaml:
   ```bash
   python3 .claude/scripts/req_status.py add-file {TASK-ID} debug_report task_management/debug_reports/{TASK-ID}-DEBUG-REPORT.yaml
   ```
3. Refresh lifecycle for tasks with debug reports:
   ```bash
   python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}
   ```

## 3. Update tasks_list

For each REQ ID:
```bash
python3 .claude/scripts/req_refresh.py {REQ-ID}
```
If YAML_VALIDATION_FAILED: read error output, fix YAML syntax in the reported file, re-run.

## 4. Report

Print: REQs processed, newly registered debug reports, refreshed tasks, req_refresh output.

## 5. Completion Checklist

- [ ] REQ IDs determined.
- [ ] Debug reports scanned and registered.
- [ ] Lifecycle refreshed for all tasks with reviews/reports.
- [ ] req_refresh.py run for each REQ.
- [ ] YAML validation errors fixed (if any).
- [ ] Report printed.
