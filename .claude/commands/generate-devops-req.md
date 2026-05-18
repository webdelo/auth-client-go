---
description: Create DevOps requirement file (DEVOPS-REQ)
argument-hint: <DevOps task description>
---

# Generate DevOps REQ

Arguments: $ARGUMENTS

Do NOT use TodoWrite tool. Create file immediately.

## 1. Determine Next Number

1. Check existing files in `task_management/tasks/` matching `DEVOPS-REQ-*.md`.
2. Determine next number.

## 2. Create DEVOPS-REQ File

1. Use template: `.claude/templates/devops_req_template.md`.
2. File path: `task_management/tasks/DEVOPS-REQ-XXX.md`.
3. Fill template:
   - Title: DevOps task description.
   - WHY: initial infrastructure context (1-2 sentences).
   - WHAT: infrastructure components based on description.
   - Sections: Containerization, Kubernetes, CI/CD, Monitoring (mark applicable).
   - Target Environment: mark applicable (Development/Staging/Production).
   - DOD: default criteria + task-specific ones.

## 3. Output

1. Print created file path.
2. Remind user to: fill infrastructure details, verify environments, create PRP via `/generate-devops-prp`.

## 4. Completion Checklist

- [ ] Next DEVOPS-REQ number determined.
- [ ] File created using template.
- [ ] Infrastructure components filled.
- [ ] File path printed.
