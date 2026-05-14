# Claude Scripts

This directory contains Python scripts that optimize common operations by reducing token usage and improving performance.

## Scripts

### req_status.py

**Purpose**: Generate status reports for requests (read-only analysis)

**Commands**:

```bash
# Generate status report (auto-saves to task_management/task_status/)
python3 req_status.py status <req_id> [--format=compact|markdown|table]

# Generate status report with custom save path
python3 req_status.py status <req_id> --output=<path>

# Refresh lifecycle fields for all tasks in request
python3 req_status.py refresh <req_id>

# Refresh lifecycle fields for a specific task
python3 req_status.py refresh-lifecycle <task_id>

# Add file reference to task (legacy, use req_refresh.py instead)
python3 req_status.py add-file <task_id> <file_type> <file_path>
# File types: code_review, bug_report, debug_report, prp, result, task
```

---

### req_refresh.py (v2.0)

**Purpose**: Full synchronization of task statuses from filesystem (write operations)

**Commands**:

```bash
# Refresh task statuses from filesystem (scans all files)
python3 req_refresh.py <req_id>
```

**What it does**:
- 📁 Scans `task_management/tasks/{REQ-ID}-TASK-*.md`
- 🔍 Finds all related files (PRP, result, code review, bug reports)
- ➕ Adds missing tasks to `tasks_list.yaml`
- 🔄 Updates existing tasks (status, files, metadata)
- 📊 Determines status from file evidence

**Key Features**:
- ✅ **Auto-saves** full report to `task_management/task_status/REQ-XXX-YYYY-MM-DD.md`
- 🔗 **Clickable links** in saved report - relative paths work in IDE markdown viewers
- 📊 **Compact console output** - simple task lists by status
- 🔍 **Finds all files**: task files, PRPs, results, code reviews, bug reports, debug reports
- 📋 **Compact lifecycle tracking** with 3 columns:
  - **Exec**: Task execution (✅ completed / ➖ not done)
  - **Review**: Status + Found/Fixed (✅, ⚠️ 3/0, ⚠️ 3/3, 🔴 2/0, ➖ not done)
  - **Debug**: Found/Fixed (5/0, 5/5, ⏳ pending data, ➖ no report)

**Examples**:

```bash
# Status report - compact tables + auto-save full report
python3 req_status.py status REQ-001
# Console: Summary + tasks tables
# Saves to: task_management/task_status/REQ-001-2026-01-27.md

# With custom output path
python3 req_status.py status REQ-001 --output=custom/path.md

# Refresh task statuses (discovers ALL tasks and files automatically)
python3 req_refresh.py REQ-001
# Output:
# ➕ Added 2 new tasks: REQ-001-TASK-004, REQ-001-TASK-011
# 🔄 Updated 3 tasks: status changes, file updates
# ✓ 10 tasks unchanged
# 📊 Total: 15 tasks processed

# Refresh lifecycle fields for all tasks (updates execution, code review, bug fixing statuses)
python3 req_status.py refresh REQ-001
# Output:
# ✅ Refresh completed for REQ-001
# 📝 Status Updated (2 tasks): ...
# 🔄 Lifecycle Updated (15 tasks): ...

# Refresh lifecycle for specific task (after creating code review or bug report)
python3 req_status.py refresh-lifecycle REQ-001-TASK-007
# ✅ Lifecycle refreshed for REQ-001-TASK-007

# Legacy: Manual file registration (not recommended, use req_refresh.py)
# For final aggregated code review reports:
python3 req_status.py add-file REQ-001-TASK-007 code_review task_management/reviews/REQ-001-TASK-007-CODE-REVIEW.yaml
python3 req_status.py add-file REQ-001-TASK-007 debug_report task_management/debug_reports/REQ-001-TASK-007-DEBUG-REPORT.yaml
```

**Lifecycle Format Examples**:

| Exec | Review | Debug | Meaning |
|------|--------|-------|---------|
| ✅ | ✅ | ➖ | Task done, review approved, no debug |
| ✅ | ⚠️ 3/0 | ➖ | Task done, 3 warnings found, 0 fixed |
| ✅ | ⚠️ 3/3 | ➖ | Task done, 3 warnings found, all fixed |
| ✅ | 🔴 2/0 | 5/0 | Task done, 2 critical issues, debug report with 5 bugs |
| ✅ | ⚠️ 3/3 | 5/5 | Task done, all issues fixed, all debug bugs fixed |
| ➖ | ➖ | ➖ | Task not executed yet |

**Output Formats**:
- `compact` (default) - Minimal console output. ~50-100 tokens. Full report auto-saved.
- `markdown` - Detailed console output (rarely needed, full report is auto-saved anyway)
- `table` - Plain text table format for scripting

**Input Formats**:
- `REQ-001` - Direct request ID
- `1` - Just the number (converted to REQ-001)
- `task_management/tasks/REQ-001.md` - File path

**How It Works**:

1. **req_status.py - Status Reports (read-only)**:
   - Reads `.memory_bank/tasks_list.yaml`
   - Filters tasks for the specified request
   - **Searches for all related files**:
     - First, reads file references from `tasks_list.yaml` (fast)
     - If not found, falls back to directory scanning (slower)
     - Task files: `task_management/tasks/`
     - PRPs: `task_management/plans/`
     - Results: `task_management/results/`
     - Code reviews (final): `task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`
     - Code reviews (interim): `.memory_bank/agent_task_results/{DATE}_code-review_*.yaml`
     - Bug reports: `task_management/bug_reports/{TASK-ID}-BUG-REPORT.yaml`
     - Session analysis: `task_management/session_analysis/` (temporary orchestrator analysis)
   - Generates two versions:
     - **Console**: Compact tables with task IDs only
     - **Saved report**: Full markdown with clickable links to all files
   - Auto-saves to: `task_management/task_status/REQ-XXX-YYYY-MM-DD.md`

2. **req_refresh.py - Full Sync (write operations, v2.0)**:
   - **Scans filesystem** for all task files: `{REQ-ID}-TASK-*.md`
   - For each task, **finds related files**:
     - PRP: `task_management/plans/{TASK-ID}-PRP.md`
     - Result: `task_management/results/{TASK-ID}-result-report.md`
     - Code Review: `**/*{TASK-ID}*code-review*.yaml`
     - Bug Report: `.memory_bank/agent_task_results/BUG-REPORT-{TASK-ID}*.yaml`
   - **Extracts metadata** from task files:
     - Description from first `# ` heading
     - Branch, Priority, Dependencies from file content
   - **Determines status** from file evidence:
     - Result report exists (with "COMPLETED" or "✅") → `completed`
     - PRP file exists → `prp_ready`
     - Only task file → `pending`
   - **Adds missing tasks** to `tasks_list.yaml`
   - **Updates existing tasks**:
     - Changes status if needed
     - Adds new file references
     - Updates metadata (if not set)
     - Moves between sections (pending/active/completed/blocked)
   - **Reports changes**:
     - Added tasks
     - Updated tasks (with change details)
     - Unchanged tasks count

3. **req_status.py refresh (lifecycle sync)**:
   - Updates `lifecycle` fields in `tasks_list.yaml` for all tasks in request
   - Extracts data from existing files:
     - **Execution**: Checks for `result_report` file → sets `lifecycle.execution.completed`
     - **Code Review**: Parses latest CODE-REVIEW.yaml → extracts verdict, issue counts
     - **Bug Fixing**: Parses latest BUG-REPORT.yaml → extracts bug counts, severity
     - **Debug Reports**: Checks for debug report files → updates bug fixing status
   - Automatically called by `req_status.py refresh <req_id>`
   - Can be called for single task: `req_status.py refresh-lifecycle <task_id>`

4. **req_status.py add-file (legacy)**:
   - Registers single file reference in `tasks_list.yaml`
   - Use only for edge cases (non-standard file names/locations)
   - **Recommended**: Use `req_refresh.py` for automatic registration

**Performance**:
- Runs in <1 second
- Saves 5,000-15,000 tokens per operation
- Can be used in automation/CI

**Integration**:
- Used by `/request-status` skill
- Used by `/refresh-req-status` skill
- Can be called directly from command line
- Can be integrated into git hooks or CI pipelines

## Why Scripts Instead of Claude Tools?

1. **Speed**: 10-20x faster than reading files with Claude tools
2. **Cost**: No token usage for file parsing
3. **Consistency**: Deterministic logic, same results every time
4. **Reusability**: Can be used outside of Claude sessions
5. **Automation**: Easy to integrate into workflows

## Claude Skills

These scripts are wrapped by Claude skills for better UX:

- `/request-status <req_id>` - Wrapper for `req_status.py status` (read-only reports)
- `/refresh-req-status <req_id>` - Wrapper for `req_refresh.py` (full sync)

The skills provide:
- Natural language interface
- Input validation
- Contextual suggestions
- Integration with other Claude commands
- Automatic clickable file links in IDE

## Development

To add new scripts:

1. Create Python script in this directory
2. Make it executable: `chmod +x script_name.py`
3. Document usage in this README
4. Create corresponding skill in `.claude/` if needed
5. Add to `dev_commands.yaml` for discoverability

## Dependencies

Current scripts require only Python 3 standard library:
- `pathlib` - File path handling
- `yaml` - YAML parsing (install: `pip install pyyaml`)
- `re` - Regular expressions
- `datetime` - Timestamps

Install dependencies:
```bash
pip install pyyaml
```

Or use project virtual environment if available.

## Testing

Test scripts before creating skills:

```bash
# Test status command (read-only, safe)
python3 req_status.py status REQ-001
python3 req_status.py status REQ-002 --format=table

# Test refresh command (writes to tasks_list.yaml - review changes!)
python3 req_refresh.py REQ-001

# Review changes before committing
git diff .memory_bank/tasks_list.yaml

# Undo if needed
git checkout .memory_bank/tasks_list.yaml
```

## Future Improvements

Potential additions:
- Task creation script
- PRP validation script
- Result report generator
- Task dependency analyzer
- Gantt chart generator for request timelines
- CI integration scripts
