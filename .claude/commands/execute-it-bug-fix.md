---
description: Fix bugs with verification on a running service
argument-hint: <bug-report>
---

# Execute IT Bug Fix

Arguments: $ARGUMENTS

Fix ALL bugs from Bug Report. EVERY bug is verified on a RUNNING service.

## 1. Rules

### 1.1 Mandatory Execution
- ALL bugs (CRITICAL, HIGH, MEDIUM, LOW) are mandatory to fix.
- Skipping — ONLY with explicit user consent (recorded in report).
- Exception flow: report problem → explain reason → ask "Skip bug {BUG-ID}?" → wait → record.

### 1.2 Orchestrator Role
- Does NOT write code. Coordination only.
- Allowed: read reports, launch agents, run build/test/lint, ask user.
- Forbidden: Edit/Write for .go files.

### 1.3 Agent Routing
- race/deadlock/concurrent/goroutine/flaky → debugger-complex (opus).
- nil pointer/compilation/assertion/mock → debugger (sonnet).
- New code → code-writer (sonnet).
- Tests → unit-test-writer (sonnet).
- Environment/verification → it-test-agent (mandatory).

## 2. Analyze Bug Report

1. Read Bug Report from $ARGUMENTS.
2. Find task in tasks_list.yaml — set status: `in_progress`.
3. Load rules: verify `.temp/rules/{TASK_ID}/`. Read ALL scope files once.
4. Prioritize: CRITICAL → HIGH → MEDIUM → LOW. ALL are mandatory.
4. Identify dependencies between bugs. Blocking bugs first.

## 3. Pre-validation

1. `go build ./...` && `make test` && `make lint` && `make test-race`.
2. Fix ONLY base errors (not bugs from report). Use appropriate agents.
3. Repeat until ALL PASS.

## 4. Start Environment

1. it-test-agent: start docker-compose, wait for dependencies.
2. it-test-agent: build and start service, health check.
3. Verify: service running and responding.

## 5. Bug Fix Cycle

For EACH bug:

1. Reproduce: it-test-agent verifies bug on running service.
2. Analyze: select agent by triggers (debugger or debugger-complex).
3. Fix: code-writer or debugger applies fix. If bug files are in repository/ or contain SQL/DB code — include DBA rules. If related to Docker/K8s/infra — include DevOps rules.
4. Rebuild: it-test-agent stops service → rebuild → restart → health check.
5. Verify: it-test-agent confirms fix on running service.
   - If NOT fixed → return to step 2 with new data.
   - If fixed → unit-test-writer writes regression test → next bug.
6. If unable to fix: ask user → record consent → skip.

## 6. Final Verification

1. Static checks: `go build ./...` && `make test` && `make lint` && `make test-race`.
2. it-test-agent: verify ALL fixed bugs on running service + check for regressions.
3. Code review: code-reviewer for changed packages → report-aggregator → CODE-REVIEW.yaml.
4. DOD validation: read TASK file → verify EVERY criterion. If NOT MET → CHANGES_REQUESTED.
5. Cross-check: all bugs either fixed or agreed upon.

## 7. Finalize

1. STOP environment: stop service, `make dev-down-v`. Verify: `docker ps` — empty.
2. Create debug report: `task_management/debug_reports/{TASK-ID}-DEBUG-REPORT.yaml`.
   - Sections: metadata, pre_validation, bugs_fixed, bugs_skipped (with consent records), final_verification, dod_validation, verdict.
3. `python3 .claude/scripts/req_status.py add-file {TASK-ID} debug_report ...`
4. `python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}`
5. Print summary: bug counts (total/fixed/skipped), checks status, DOD status, verdict (ALL_FIXED | PARTIAL).
6. Auto-commit: invoke `/generate-commit` with the task file path from arguments.

## 8. Completion Checklist

- [ ] Bug report read and analyzed.
- [ ] Rules loaded.
- [ ] Pre-validation passed.
- [ ] Environment started, service healthy.
- [ ] EACH bug reproduced on running service.
- [ ] EACH bug verified after fix on running service.
- [ ] Regression test written for each fixed bug.
- [ ] Skipped bugs have explicit user consent recorded.
- [ ] Static checks pass.
- [ ] All bugs verified on running service (final).
- [ ] Code review completed.
- [ ] DOD validation completed.
- [ ] Environment STOPPED (docker ps — empty).
- [ ] Debug report created.
- [ ] Lifecycle updated.
- [ ] Summary printed.
- [ ] Auto-commit completed (via /generate-commit).
