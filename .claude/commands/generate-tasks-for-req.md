---
description: Decompose REQ into multiple TASKs with interactive clarification
argument-hint: <REQ-XXX or path/to/REQ-XXX.md>
---

# Generate Tasks for REQ

Arguments: $ARGUMENTS

## 1. Rules

- Role: technical lead, requirements analyst.
- ULTRATHINK about decomposition before creating any tasks.
- Each task must be autonomous with clear verification path.
- STOP after task generation. Do NOT start execution.
- Do NOT edit tasks_list.yaml manually — use ONLY req_refresh.py.
- Prefer MANY small tasks over FEW large ones.
- Every task MUST leave the codebase in a runnable state after completion.
- Every task MUST NOT reduce test coverage (or reduce minimally with justification).
- Ideally each task is verifiable by an integration test or a new case in an existing integration test.

## 2. Preparation

1. Read `CLAUDE.md`.
2. Parse argument: extract REQ-ID (e.g. `REQ-001`).
   - If full path given (`task_management/tasks/REQ-001.md`) — extract REQ-ID from filename.
   - If only REQ-ID given — construct path: `task_management/tasks/{REQ-ID}.md`.
3. Validate REQ file exists. If not — error, suggest `/generate-req`.
4. Read REQ file completely — understand WHY, WHAT, SCOPE sections.
5. Check existing tasks: list files matching `task_management/tasks/{REQ-ID}-TASK-*.md`.
   - Note which TASK numbers already exist.
   - Read existing tasks to understand what is already covered and avoid duplication.

## 3. Codebase Analysis

1. Launch code-researcher: analyze current codebase state relevant to REQ scope.
   - Identify existing modules, interfaces, patterns that tasks will touch.
   - Find integration points and dependencies.
   - Map affected layers (handlers, usecase, domain, repository).
2. Analyze related documents in `guides/` and `patterns/`.
3. Analyze key dependencies from REQ, check their documentation if needed.

## 4. Interactive Clarification (Уточнения)

**CRITICAL**: This step is MANDATORY. Do NOT skip even if REQ seems clear.

1. Analyze the REQ file for ambiguities, unclear scope boundaries, missing details.
2. Formulate 3-7 clarifying questions covering:
   - **Scope boundaries**: What is explicitly OUT of scope? Any grey areas?
   - **Priority**: Which functional areas are most critical?
   - **Dependencies**: Are there external blockers or prerequisites?
   - **Testing**: What level of test coverage is expected (unit / integration / e2e)?
   - **Ordering**: Are there preferred implementation phases or sequencing?
   - **Existing code**: Should any existing implementations be preserved, extended, or replaced?
   - **Integration points**: How should new code integrate with existing modules?
3. Ask ALL questions at once using a numbered list (one AskUserQuestion call).
4. Wait for user response.
5. If user response raises new questions — ask follow-up (max 2 rounds total).
6. Record ALL Q&A in the REQ file:
   - Append a `## Уточнения` section before the `## TASKS` section (or at end if no TASKS section).
   - Format:

```markdown
## Уточнения

**Дата**: YYYY-MM-DD

**В1**: [Question text]
**О1**: [Answer text]

**В2**: [Question text]
**О2**: [Answer text]
```

## 5. Task Decomposition Planning

1. Launch task-planner: create decomposition plan based on REQ + clarifications + codebase analysis.
2. Decomposition criteria — EVERY task MUST satisfy ALL of these:

| # | Criterion | Description |
|---|-----------|-------------|
| 1 | **Verifiability** | Task can be verified independently (specific checks, tests, observable behavior) |
| 2 | **Isolation** | Task is self-contained; no implicit dependencies on unfinished parallel tasks |
| 3 | **Completeness** | After task completion, code compiles and runs without errors |
| 4 | **Test coverage** | Task does not reduce existing test coverage; adds tests for new code |
| 5 | **Integration-testable** | Ideally verifiable by integration test (gonkey gRPC/HTTP) or a new case in existing test |
| 6 | **Size** | Estimated complexity XS-M (max ~half day of work); if L/XL — split further |

3. Create dependency graph between tasks (which must be done before which).
4. Group tasks into logical phases:
   - **Phase 1: Foundation** — types, interfaces, domain entities, migrations
   - **Phase 2: Core** — main business logic, repositories, services
   - **Phase 3: Integration** — handlers, DI wiring, API endpoints
   - **Phase 4: Quality** — tests, documentation, polish
5. Score each planned task 1-10 against decomposition criteria. If any task scores <7 — split or improve it.

## 6. Task Generation

For each task in the decomposition plan (in dependency order):

1. Determine task type: coding or bugfix.
2. Compose task description (1-2 sentences).
3. Call `/generate-task {REQ-ID} {task description}` — this handles:
   - Next TASK number determination
   - Template selection (coding/bugfix)
   - File creation with WHY/WHAT/DOD
   - Parent REQ TASKS table update
   - Registration via req_refresh.py
4. After `/generate-task` completes — verify the created TASK file:
   - DOD has >= 3 measurable criteria
   - WHAT has numbered action items starting with infinitive verbs
   - WHY connects to REQ context
   - Accents reference relevant rules/patterns/ADRs
5. If task file quality is insufficient — edit to improve before proceeding to next task.

**Task ordering**: Tasks appear in recommended execution order in the REQ TASKS table.

## 7. Registration

After ALL tasks are created, run final registration:

```bash
python3 .claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates
```

Verify `.memory_bank/tasks_list.yaml` contains all new tasks with correct statuses.

## 8. Completeness Validation

1. Re-read the REQ file (WHY, WHAT, SCOPE, Уточнения sections).
2. Create a coverage matrix: REQ requirement → TASK(s) that implement it.
3. Check for gaps:
   - Every functional requirement from WHAT must be covered by at least one TASK.
   - Every non-functional requirement must be addressed (possibly across multiple tasks).
   - Every clarification answer must be reflected in relevant tasks.
   - SCOPE items must all be covered.
4. If gaps found — create additional tasks via `/generate-task`.
5. Re-run `req_refresh.py` if additional tasks were created.

## 9. Output Summary

Print:
1. Total tasks created (count).
2. Task table:

| TASK-ID | Описание | Фаза | Сложность | Зависимости |
|---------|----------|------|-----------|-------------|
| TASK-XXX | ... | 1-Foundation | 3/10 | - |

3. Coverage matrix:

| Требование REQ | Покрыто TASK |
|----------------|--------------|
| [requirement] | TASK-XXX, TASK-YYY |

4. Warnings or recommendations (if any).

## 10. Completion Checklist

- [ ] CLAUDE.md read.
- [ ] REQ file read and understood.
- [ ] Existing tasks checked for duplication.
- [ ] Codebase analysis completed (code-researcher).
- [ ] Interactive clarification completed (questions asked and answered).
- [ ] Уточнения section added to REQ file with dated Q&A.
- [ ] Decomposition plan created with phases and dependencies.
- [ ] Each task scored >= 7 against decomposition criteria.
- [ ] All tasks created via `/generate-task`.
- [ ] Each task has quality DOD (>= 3 measurable criteria).
- [ ] tasks_list.yaml updated via req_refresh.py.
- [ ] Completeness validated — all REQ requirements covered by tasks.
- [ ] Coverage matrix printed.
- [ ] STOPPED — did not start execution.
