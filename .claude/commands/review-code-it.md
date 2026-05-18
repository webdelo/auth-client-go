---
description: Integration testing review on a running service. Result — bug report.
argument-hint: <task-file>
---

# Integration Testing Review

Arguments: $ARGUMENTS

Verify implemented functionality on a running service. Result: BUG-REPORT.yaml.

## 1. Rules

### 1.1 Forbidden Actions
- Skipping pre-validation — service may not start.
- Skipping DOD items without explicit user consent.
- Hiding bugs or classifying as "unimportant".
- Forgetting to stop environment — causes port conflicts.

### 1.2 Required Actions
- ALL DOD items verified on running service.
- ALL found bugs documented with evidence.
- If unable to verify: ask user "Skip item {X}?", wait for consent, record in report.
- Environment MUST be stopped after completion: `make dev-down-v`.

### 1.3 Bug Categories
CRITICAL_CRASH | FUNCTIONALITY | VALIDATION | ERROR_HANDLING | PERFORMANCE | DATA_INTEGRITY | INTEGRATION | EDGE_CASES | SECURITY | API_CONTRACT

## 2. Load Context

1. Read task from $ARGUMENTS — determine TASK-ID.
2. Load rules: verify `.temp/rules/{TASK_ID}/` exists. If not: `python3 .claude/scripts/extract_rules.py analyze {TASK_FILE} --output-dir .temp/rules/{TASK_ID}/`
3. Read ALL scope files once.
4. Extract DOD criteria, build testing checklist (happy path + edge cases + error handling).

## 3. Pre-validation (GATE)

Proceed to section 4 ONLY after ALL checks pass.

1. `go build ./...` — on fail: debugger → fix → retry.
2. `make test` — on fail: debugger (or debugger-complex for flaky) → fix → retry.
3. `make lint` — on fail: refactorer or debugger → fix → retry.
4. `make test-race` — on fail: debugger-complex → fix → retry.

## 4. Start Environment

1. Launch it-test-agent:
   - Start docker-compose / dependencies.
   - Health check dependencies.
   - Build and start service.
   - Health check service.
2. Verify: all dependencies + service are running and responding.

## 5. Testing

1. Happy path: verify each DOD item on running service. For each: perform action → record result → compare with expected.
2. Edge cases: empty/invalid data, boundary values, missing required fields, incorrect types.
3. Error handling: dependency unavailability, correct error messages, HTTP status codes, logging.

## 6. DOD Validation

1. For each DOD criterion: verify on running service. Status: PASS / FAIL / PARTIAL.
2. If FAIL → add to bug list with evidence.
3. Build dod_checklist section for bug report.

## 7. Bug Report

1. Collect all bugs with evidence (logs, response, reproduction steps).
2. Categorize and prioritize.
3. Create bug report: `task_management/bug_reports/{TASK-ID}-BUG-REPORT.yaml`.
4. Validate YAML: `python3 -c "import yaml; yaml.safe_load(open('{path}'))"`.
5. `python3 .claude/scripts/req_status.py add-file {TASK-ID} bug_report {path}`
6. `python3 .claude/scripts/req_refresh.py {REQ-ID} --fix-duplicates`

## 8. Finalization

1. STOP environment: `make dev-down-v`. Fallback: `docker compose down -v && docker network prune -f && docker volume prune -f`. Verify: `docker ps` — empty list.
2. Determine verdict: ALL_PASSED / BUGS_FOUND / BLOCKED.
3. Print summary: TASK-ID, date, verdict, bug count by priority, DOD passed/total, path to bug report.
4. If BUGS_FOUND: next step — `/execute-it-bug-fix {bug_report_path}`.

## 9. Completion Checklist

- [ ] Task file read, DOD criteria extracted.
- [ ] Rules loaded.
- [ ] Pre-validation passed (build, test, lint, race).
- [ ] Environment started, service healthy.
- [ ] Happy path tested.
- [ ] Edge cases tested.
- [ ] Error handling tested.
- [ ] DOD validation completed on running service.
- [ ] Bug report created (or confirmed no bugs).
- [ ] YAML validation passed.
- [ ] Bug report registered via req_status.py.
- [ ] Environment STOPPED (docker ps — empty).
- [ ] Summary printed.
