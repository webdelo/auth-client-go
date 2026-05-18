---
description: Create problem resolving plan (PRP) for a task
argument-hint: <task-file>
---

# Generate PRP

Arguments: $ARGUMENTS

## 1. Rules

### 1.1 Agent Rules
- All agents launched with run_in_background: true.
- Multiple Agent calls in ONE message for parallel execution.
- Mandatory agent: code-researcher.
- Conditional: web-searcher (new libs), api-designer (*.proto / API), task-planner (>5 files or >3 packages), security-auditor (auth/credentials).
- Agent selection happens AFTER code-researcher (based on real findings).

### 1.2 PRP Content Rules
- PRP contains ONLY the final chosen solution. NO alternatives, NO "could also do...".
- Alternatives analysis saved separately to `task_management/session_analysis/`.
- STOP after PRP generation. Do NOT start execution. Wait for user approval.

## 2. Task Analysis and Rules Loading

1. Read `task_management/tasks/{TASK-ID}.md`.
2. Extract scope: affected packages, files, layers.
3. Extract keywords: technologies, patterns, domain terms.
4. Load rules: verify `.temp/rules/{TASK_ID}/`. If not: `python3 .claude/scripts/extract_rules.py analyze {TASK_FILE} --output-dir .temp/rules/{TASK_ID}/`
5. Read scope files once (coding_scope.md, review_scope.md).

## 3. Research

1. Launch code-researcher (background: true): find patterns, conventions, test approaches, change scope. Output: `.memory_bank/agent_task_results/{DATE}_code-research_{feature}.md`.
2. If TASK mentions new libraries/technologies — launch web-searcher (background: true).
3. Read relevant guides from `.claude/guides/` and `.claude/patterns/`.

## 4. Agent Selection (After Research)

1. Read code-researcher report.
2. Select additional agents based on REAL findings:
   - api-designer: found *.proto OR need new gRPC/REST endpoints.
   - task-planner: complexity >5 files OR >3 packages.
   - security-auditor: auth/credentials/sensitive data.
3. Launch selected agents in parallel.
4. Aggregate results from all agents.

## 5. Clarification

Condition: if uncertainties or multiple approaches exist.

1. Identify unclear points from research.
2. If multiple approaches: save analysis to `task_management/session_analysis/{DATE}-{TASK-ID}-alternatives-analysis.md`. Format: Option 1/2/N with pros/cons — chosen solution + rationale.
3. AskUserQuestion if user input needed.
4. Finalize ONE solution for the PRP.

## 6. PRP Generation

1. ULTRATHINK before writing the PRP.
2. Use template: `.claude/templates/coding_prp_template.md`. Schema: `.claude/templates/coding_prp.yaml`.
3. Critical sections:
   - coding_standards: rules_index, loaded guides with critical_rules.
   - context: documentation URLs, current/desired structure, gotchas, research_findings.
   - implementation: ONLY the final solution. Data models, tasks in execution order, integration.
   - testing: unit_tests (happy/error/edge), integration_tests if needed, mocks and coverage.
4. Validate: all context included, coding_standards filled, references existing patterns, clear implementation path.
5. Score PRP 1-10. If <7 — add context / clarify requirements.

## 7. Registration

1. Save PRP: `task_management/plans/{TASK-ID}-PRP.md`.
2. Validate format: tables aligned, file links correct, code blocks have language specifier, no unfilled placeholders.
3. `python3 .claude/scripts/req_status.py add-file {TASK-ID} prp task_management/plans/{TASK-ID}-PRP.md`
4. `python3 .claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates`
5. STOP. Do NOT start PRP execution. Wait for user approval.

## 8. Completion Checklist

- [ ] TASK file read and analyzed.
- [ ] Keywords and scope extracted.
- [ ] Rules loaded.
- [ ] code-researcher launched and results analyzed.
- [ ] Additional agents selected based on real findings.
- [ ] ULTRATHINK performed before writing PRP.
- [ ] PRP contains ONLY the final solution (no alternatives).
- [ ] PRP contains links to ALL analysis files.
- [ ] PRP scored >= 7.
- [ ] PRP registered via req_status.py.
- [ ] STOPPED — did not start execution.
