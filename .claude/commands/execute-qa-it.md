---
description: QA integration testing of a running service by test case
argument-hint: <test-case-file>
---

# Execute QA Integration Testing

Arguments: $ARGUMENTS

Perform full QA integration testing of a running service based on a test case description. Orchestrator acts as a QA engineer.

## 1. Rules

### 1.1 Orchestrator Role — QA Engineer
- Orchestrator acts as a **QA engineer**: plans tests, executes them, documents results.
- Does NOT write application code. Coordination and verification only.
- Allowed: read test cases, launch agents, run build/test/lint, execute API calls via agents, ask user.
- Forbidden: Edit/Write for .go files (except test files via unit-test-writer agent).

### 1.2 Mandatory Execution
- ALL test scenarios from the test case are MANDATORY to execute.
- ALL found bugs are documented with full evidence.
- Skipping a scenario — ONLY with explicit user consent (recorded in report).
- Exception flow: report problem → explain reason → ask "Skip scenario {ID}?" → wait → record.

### 1.3 Bug Severity Levels
- BLOCKER — service crash, data loss, security vulnerability.
- CRITICAL — core functionality broken, no workaround.
- MAJOR — functionality broken, workaround exists.
- MINOR — cosmetic, UX, non-critical deviation from spec.

### 1.4 Agent Routing
- Environment management → it-test-agent (mandatory).
- API calls and verification on service → it-test-agent.
- Bug analysis (if needed) → code-researcher.
- Bug fix (if requested by user) → code-writer / debugger.
- Test writing (if requested) → unit-test-writer.

## 2. Load Context

1. Read test case from $ARGUMENTS — determine test scope, target service, scenarios.
2. Identify target service (signal-analyzer, strategy-tester, bff, data-loader).
4. Extract all test scenarios with expected results.
5. Build a testing checklist: scenario ID, description, preconditions, steps, expected result.

## 3. Pre-validation (GATE)

Proceed to section 4 ONLY after ALL checks pass.

1. `go build ./...` — on fail: debugger → fix → retry.
2. `make test` — on fail: debugger (or debugger-complex for flaky) → fix → retry.
3. `make lint` — on fail: refactorer or debugger → fix → retry.
4. `make test-race` — on fail: debugger-complex → fix → retry.

## 4. Start Environment

1. Launch it-test-agent:
   - Start docker-compose / dependencies.
   - Health check dependencies (DB, message brokers, etc.).
   - Build and start target service.
   - Health check service endpoints.
2. Verify: all dependencies + service are running and responding.

## 5. Test Execution

For EACH scenario from the test case:

### 5.1 Precondition Setup
- Prepare test data (DB fixtures, seed data) if specified in scenario.
- Verify preconditions are met before executing test steps.

### 5.2 Execute Test Steps
- Execute each step from the scenario via it-test-agent.
- Record: request sent, response received, service logs, timing.
- Compare actual result with expected result.

### 5.3 Classify Result
- **PASS** — actual matches expected.
- **FAIL** — deviation found → create bug entry with:
  - Scenario ID and step number.
  - Expected vs actual result.
  - Full request/response.
  - Relevant service logs.
  - Severity level (BLOCKER/CRITICAL/MAJOR/MINOR).
  - Reproduction steps.

### 5.4 Cleanup
- Reset test data if needed for next scenario isolation.

## 6. Edge Case & Negative Testing

After all scenarios from the test case:

1. **Invalid input**: empty fields, wrong types, boundary values, oversized payloads.
2. **Error handling**: correct error codes, meaningful error messages, no stack traces in response.
3. **Concurrency**: parallel requests to same endpoint (if applicable).
4. **Idempotency**: repeated requests produce consistent results (if applicable).

## 7. QA Report

1. Collect all results with evidence.
2. Derive report path from the test case file path:
   - Extract service directory from `$ARGUMENTS`: path like `qa/test_data/{service}/test_plans/cases/{TC-ID}_...`
   - Service name = directory after `qa/test_data/` (e.g., `data_loader`, `signal_analyzer`, `strategy_tester`, `bff`)
   - QA report YAML → `qa/test_data/{service}/test_results/{TEST-CASE-ID}-QA-REPORT.yaml`
   - Bug report files → `qa/test_data/{service}/qa_bug_reports/BUG-{NNN}.md`
3. Report structure:
   ```yaml
   metadata:
     test_case: <path>
     service: <name>
     date: <date>
     environment: local|remote

   summary:
     total_scenarios: N
     passed: N
     failed: N
     skipped: N
     bugs_found: N

   scenarios:
     - id: SC-001
       name: <name>
       status: PASS|FAIL|SKIPPED
       steps_executed: N
       bugs: [BUG-001]
       evidence: <logs, responses>

   bugs:
     - id: BUG-001
       scenario: SC-001
       severity: BLOCKER|CRITICAL|MAJOR|MINOR
       title: <title>
       expected: <expected>
       actual: <actual>
       reproduction_steps: [...]
       evidence:
         request: <request>
         response: <response>
         logs: <logs>

   edge_case_results:
     - category: invalid_input
       tests_run: N
       bugs_found: N

   verdict: ALL_PASSED|BUGS_FOUND|BLOCKED
   ```
4. Validate YAML: `python3 -c "import yaml; yaml.safe_load(open('{path}'))"`.

## 8. Finalization

1. STOP environment: stop service, `make dev-down-v`. Fallback: `docker compose down -v && docker network prune -f && docker volume prune -f`. Verify: `docker ps` — empty list.
2. Determine verdict: ALL_PASSED / BUGS_FOUND / BLOCKED.
3. Print summary:

```
## QA Integration Testing Result

**Test Case**: {filename}
**Service**: {service_name}
**Date**: {date}

### Scenarios
| ID | Name | Status |
|----|------|--------|
| SC-001 | ... | PASS |
| SC-002 | ... | FAIL (BUG-001) |

### Bugs Found
| BUG-ID | Severity | Title | Scenario |
|--------|----------|-------|----------|
| BUG-001 | CRITICAL | ... | SC-002 |

### Checks
- Pre-validation: PASS
- Environment: started/stopped OK
- Scenarios executed: {total}
- Edge case testing: DONE

**Verdict**: {ALL_PASSED | BUGS_FOUND | BLOCKED}
**QA Report**: {path_to_report}
```

4. If BUGS_FOUND and user wants fixes: suggest `/execute-it-bug-fix {qa_report_path}`.

## 9. Completion Checklist

- [ ] Test case file read and analyzed.
- [ ] All scenarios extracted and checklist built.
- [ ] Pre-validation passed (build, test, lint, race).
- [ ] Environment started, service healthy.
- [ ] ALL scenarios executed on running service.
- [ ] Edge case & negative testing completed.
- [ ] Each result documented with evidence.
- [ ] QA report created and YAML validated.
- [ ] Environment STOPPED (docker ps — empty).
- [ ] Summary printed with verdict.
