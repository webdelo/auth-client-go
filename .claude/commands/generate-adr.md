---
description: Create Architecture Decision Record (ADR)
argument-hint: <task-file>
---

# Generate ADR

Arguments: $ARGUMENTS

## 1. Rules

- Role: technical lead for this task.
- ULTRATHINK before writing the ADR.
- STOP after ADR generation. Do NOT start execution.
- If ADR includes DB schema: load DBA rules from `.claude/guides/dba/rules_index.yaml`. CRITICAL: never use ON DELETE CASCADE / ON UPDATE CASCADE.

## 2. Preparation

1. Update tasks_list: `python3 .claude/scripts/req_refresh.py`
2. Read CLAUDE.md.
3. Read the feature file to understand requirements.

## 3. Codebase Analysis

1. Launch code-researcher agent: analyze architecture, find patterns, dependencies, integration points.
2. Analyze related documents in `guides/` and `patterns/`.
3. Search for similar features/patterns in the codebase.
4. Note existing conventions to follow.

## 4. API Design (if applicable)

Condition: feature includes API.

1. Launch api-designer agent: design API per requirements, follow existing patterns.
2. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`

## 5. External Research

1. Launch web-searcher agent: find documentation, architectural patterns, examples for relevant technologies.
2. Analyze agent report in `.memory_bank/agent_task_results/`.
3. Include found solutions in ADR.

## 6. Generate ADR

1. ULTRATHINK about the ADR.
2. Use template: `.claude/templates/adr_templates.md`.
3. Include critical context: documentation URLs, code examples from codebase, library gotchas, existing patterns.
4. Implementation blueprint: pseudocode, real file references, error handling, tasks in execution order.
5. If DB schema included: verify NO CASCADE constraints (DB-SCHEMA-001).
6. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`

## 7. Validation

1. All necessary context included.
2. Validation gates executable by AI.
3. References existing patterns.
4. Clear implementation path.
5. SQL schemas do NOT contain CASCADE.
6. Score ADR 1-10.

## 8. Registration

1. Save as: `task_management/plans/{REQ-XXX-ADR-YYY}.md`.
2. Add ADR to parent REQ file (TASKS section).
3. `python3 .claude/scripts/req_status.py add-file {TASK-ID} prp task_management/plans/{TASK-ID}-ADR.md`
4. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
5. STOP. Do NOT start execution.

## 9. Completion Checklist

- [ ] CLAUDE.md read.
- [ ] Feature file read and understood.
- [ ] code-researcher launched, results analyzed.
- [ ] API designed (if applicable).
- [ ] External research completed.
- [ ] ULTRATHINK performed.
- [ ] ADR written using template.
- [ ] No CASCADE in DB schemas.
- [ ] ADR scored >= 7.
- [ ] ADR saved and added to parent REQ.
- [ ] Lifecycle updated via req_status.py.
- [ ] STOPPED — did not start execution.
