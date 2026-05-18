---
description: Create requirement file (REQ) — feature description
argument-hint: <feature description> [--jira PROJ-XX]
---

# Generate REQ

Arguments: $ARGUMENTS

Do NOT use TodoWrite tool. Create file immediately without additional planning.

## 1. Parse Arguments

1. Main text: feature description.
2. Check for optional `--jira PROJ-XX` parameter. If present — extract Jira ID.

## 2. Determine Next REQ Number

1. Check existing files in `task_management/tasks/` matching `REQ-*.md`.
2. Determine next number (REQ-001, REQ-002, etc.).

## 3. Create REQ File

1. Use template: `.claude/templates/req_template.md`.
2. File path: `task_management/tasks/REQ-XXX.md`.
3. Fill template:
   - Title: feature description.
   - Jira ID: from `--jira` or placeholder.
   - WHY: initial business context (1-2 sentences).
   - WHAT: structure with placeholders.
   - SCOPE: basic placeholders.
   - TASKS: empty (added later via `/generate-task`).
   - DOD: basic criteria placeholders.

## 4. Output

1. Print created file path.
2. Remind user to fill details and create tasks via `/generate-task REQ-XXX <description>`.

## 5. Completion Checklist

- [ ] Arguments parsed (description + optional jira_id).
- [ ] Next REQ number determined.
- [ ] REQ file created using template.
- [ ] File path printed.
