---
description: Create DevOps problem resolving plan (PRP)
argument-hint: <devops-task-file>
---

# Generate DevOps PRP

Arguments: $ARGUMENTS

## 1. Rules

- ULTRATHINK about security, scalability, and observability before writing.
- STOP after PRP generation. Do NOT start execution.
- Load rules by keywords ONLY — do NOT read all guides (context overload).

## 2. Preparation

1. Read CLAUDE.md.
2. Read DevOps task file.
4. Keyword-based rule loading:
   - Extract keywords from task file.
   - Load `.claude/guides/devops/rules_index.yaml`.
   - Match keywords → load ONLY matched section files.
   - If task involves DB: also load `.claude/guides/dba/rules_index.yaml`.

## 3. Infrastructure Analysis

1. Launch code-researcher: analyze existing Dockerfiles, K8s manifests, CI/CD configs, deployment scripts.
2. Analyze current deployment strategy, security policies, monitoring/logging setup.

## 4. External Research

1. Launch web-searcher: find Docker/Kubernetes/CI-CD best practices, security hardening, production patterns.
2. Include found URLs and security guidelines in PRP.

## 5. Security and Compliance

1. Check: non-root users, secret management, image scanning.
2. Determine compliance requirements, resource quotas, network policies.

## 6. User Clarification (if needed)

Target environments? Scalability/HA requirements? Backup/DR needs? Security constraints?

## 7. DevOps Planning (optional)

Launch `devops-engineer` (virtual agent — use `general-purpose` with instructions from `.claude/agents/devops-engineer.md`):
- Read `.claude/agents/devops-engineer.md`, include contents in prompt.
- Task: design infrastructure solution per requirements.

## 8. Generate DevOps PRP

1. ULTRATHINK about security, scalability, observability.
2. Use template: `.claude/templates/devops_prp_template.md`. Schema: `.claude/templates/devops_prp.yaml`.
3. Include: infrastructure context, security requirements, resource specs, observability, deployment strategy, rollback procedure.
4. Score PRP 1-10 (criteria: security, scalability, observability, resilience, documentation).

## 9. Registration

1. Save as: `task_management/plans/{REQ-XXX-DEVOPS-TASK-YYY-PRP}.md`.
2. Add PRP to parent REQ file (if applicable).
3. STOP. Do NOT start execution.

## 10. Completion Checklist

- [ ] CLAUDE.md read.
- [ ] DevOps task file read and understood.
- [ ] Rules loaded by keywords (not all guides).
- [ ] code-researcher analyzed existing infrastructure.
- [ ] web-searcher found best practices.
- [ ] Security/compliance requirements documented.
- [ ] ULTRATHINK performed.
- [ ] PRP written using template.
- [ ] PRP scored >= 7.
- [ ] PRP saved and registered.
- [ ] devops-engineer agent used (if applicable).
- [ ] STOPPED — did not start execution.
