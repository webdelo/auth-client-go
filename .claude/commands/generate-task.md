---
description: Create task file (TASK) linked to a requirement
argument-hint: <REQ-XXX> <task description> [--jira PROJ-XX]
---

# Generate TASK

Arguments: $ARGUMENTS

Do NOT use TodoWrite tool. Create file immediately without additional planning.

## 1. Parse Arguments

1. First argument: REQ identifier (e.g. `REQ-001`). If missing — ask user.
2. Remaining text: task description.
3. Check for optional `--jira PROJ-XX`. If not present — check parent REQ file for `**Jira**:` section.

## 2. Validate REQ

1. Check `task_management/tasks/REQ-XXX.md` exists. If not — warn, suggest `/generate-req`.
2. Read REQ file — get title.

## 3. Determine Next TASK Number

1. Check existing files matching `REQ-XXX-TASK-*.md`.
2. Determine next number.

## 4. Create TASK File

1. Choose template based on task type:
   - Bug fix / error correction → `.claude/templates/bugfix_task_template.md`
   - Everything else → `.claude/templates/coding_task_template.md`
2. File path: `task_management/tasks/REQ-XXX-TASK-YYY.md`.
3. Fill template following these principles:

### Content principles

**CRITICAL: Task says WHAT to do, not HOW to do it. Implementation details belong in PRP.**

- **WHY**: 1-3 sentences connecting task to REQ context. Why this work is needed.
- **WHAT**: Numbered list of actions. Each item starts with infinitive verb (Создать, Добавить, Реализовать, Удалить, Перенести).
  - Describe WHAT to do, not HOW to implement.
  - No code examples, no project structure diagrams, no step-by-step implementation.
  - **Акценты** subsection: trigger words for rules (e.g. "согласно CS-0112"), patterns and technologies to use, links to similar libraries/projects/articles, ADR/spec references.
- **DOD**: Checklist (`- [ ]`) with measurable completion criteria. **This is the most important section — elaborate thoroughly.**
  - Functional criteria: what must work, what must not break.
  - Verification criteria: how to confirm the task is done (commands, checks, observable behavior).
  - Quality criteria: lint, tests, build.

### Forbidden in WHAT section

- Code examples or snippets
- File/directory structure diagrams
- Step-by-step implementation instructions
- Dependency lists
- "Структура проекта после изменений"

### Bugfix-specific: Evidence section

For bugfix tasks, fill the **Evidence** section with:
- Error messages from logs
- Code snippets showing invalid behavior
- Stack traces
- Affected modules and reproduction frequency

## 5. Update Parent REQ

1. Add new TASK to the TASKS section in parent REQ file.

## 6. Register in tasks_list.yaml

1. If jira_id available: include `jira_id: PROJ-XX`, branch = `PROJ-XX`.
2. If no jira_id: standard branch format.
3. Use: `python3 .claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates`

## 7. Output

1. Print created file path.
2. Remind user to review DOD section.

## 8. Completion Checklist

- [ ] Arguments parsed (REQ-ID + description + optional jira_id).
- [ ] Parent REQ exists and read.
- [ ] Next TASK number determined.
- [ ] TASK file created using template (WHY/WHAT/DOD only, no implementation details).
- [ ] Parent REQ updated with new TASK reference.
- [ ] tasks_list.yaml updated via req_refresh.py.
- [ ] File path printed.
