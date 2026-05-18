---
description: Generate tasks from Architecture Decision Record
argument-hint: <adr-file>
---

# Generate Tasks for ADR

Arguments: $ARGUMENTS

## 1. Rules

- Role: technical lead.
- ULTRATHINK about each task before writing.
- Each task must be autonomous with clear implementation path.
- STOP after task generation. Do NOT start execution.
- Do NOT edit tasks_list.yaml manually — use ONLY req_refresh.py or req_status.py.

## 2. Preparation

1. Read CLAUDE.md.
2. Read ADR file to understand the technical solution.

## 3. Codebase Analysis

1. Launch code-researcher (if needed): find existing patterns for the task types.
2. Analyze related documents in `guides/` and `patterns/`.
3. Analyze key dependencies from ADR, check their documentation.

## 4. External Research

1. Launch web-searcher (if needed): find dependency documentation, API docs, gotchas.
2. Include documentation links in tasks.

## 5. Task Generation

1. Launch task-planner: decompose ADR into tasks with dependencies and execution order.
2. For each task, use template: `.claude/templates/coding_task_template.md`.
3. Validate each task: all context included, validation gates executable, references patterns, clear implementation path.
4. Score each task 1-10. If <7 — improve.
5. Save each task: `task_management/tasks/{REQ-XXX-TASK-YYY}.md`.

## 6. Registration

1. Add each task to parent REQ file.
2. Register: `python3 .claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates`

## 7. Completeness Validation

1. Launch task-planner: verify ADR fully covered by tasks.
2. Check: all requirements covered, no missing components, dependencies correct.
3. If gaps found — create additional tasks.
4. STOP. Do NOT start execution.

## 8. Completion Checklist

- [ ] CLAUDE.md read.
- [ ] ADR read and understood.
- [ ] Code research completed (if needed).
- [ ] External research completed (if needed).
- [ ] Tasks decomposed with dependencies.
- [ ] Each task uses template and scored >= 7.
- [ ] Tasks saved and added to parent REQ.
- [ ] tasks_list updated via req_refresh.py.
- [ ] Completeness validated — ADR fully covered.
- [ ] STOPPED — did not start execution.
