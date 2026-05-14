---
prefix: GNK
name: Gonkey
packages:
  - github.com/Issengaard/gonkey_grpc
  - github.com/lamoda/gonkey
triggers:
  - gonkey
  - yaml test
  - integration test
  - fixture
description: "YAML-based integration testing for gRPC/REST APIs; DB fixtures (PG, CH, Redis), HTTP mocks, response verification"
---

# GNK-001: Integration Testing with Gonkey

## GNK-002: Overview

Gonkey is an automation tool for integration testing REST/JSON APIs from Lamoda. It supports YAML-based test descriptions, database fixtures for PostgreSQL, MySQL, ClickHouse, Redis/Valkey, and mocking of external HTTP services.

**Key advantages:**
- Declarative YAML tests without writing Go code for each test
- Fixtures for preparing database state with inheritance and templates
- HTTP mocks for simulating external services with request validation
- Database state verification after test execution (dbQuery/dbResponse)
- Allure reports with TMS integration
- Can be used as CLI or as a Go library

**Documentation:**
- GitHub: https://github.com/lamoda/gonkey
- pkg.go.dev: https://pkg.go.dev/github.com/lamoda/gonkey
- README-ru: https://github.com/lamoda/gonkey/blob/master/README-ru.md

---

## GNK-003: Test Project Structure

```
tests/
├── cases/                          # YAML test cases
│   ├── kafka/                      # Kafka consumer tests
│   │   └── topic_name/
│   │       ├── +happy_path.yaml    # Positive tests (+ prefix)
│   │       └── -error_case.yaml    # Negative tests (- prefix)
│   └── api/                        # HTTP API tests
│       └── endpoint_name/
├── fixtures/                       # Database fixtures
│   ├── base_templates.yaml         # Base templates
│   ├── empty_db.yaml               # Empty database
│   ├── postgres/                   # PostgreSQL fixtures
│   │   └── feature/
│   └── valkey/                     # Redis/Valkey fixtures
│       └── feature/
├── events/                         # JSON events for Kafka tests
│   └── topic_name/
│       └── event_name.json
├── mocks_data/                     # Mock service responses
│   └── service_name/
│       └── endpoint/
│           └── success.json
├── runner/                         # Test runner code
│   ├── runner.go                   # Main runner
│   ├── env.go                      # Environment variable management
│   └── database/                   # Custom loaders and checkers
│       ├── fixtures_loader.go
│       └── response_checker.go
├── handler/                        # Fake handlers for testing
│   └── fake_kafka_handler.go
├── server/                         # Test HTTP server
│   └── server.go
└── *_test.go                       # Go test files
```

### GNK-004: File Naming Conventions

| Pattern | Description | Example |
|---------|-------------|---------|
| `+name.yaml` | Positive test (happy path) | `+consume_order.yaml` |
| `-name.yaml` | Negative test (error case) | `-invalid_payload.yaml` |
| `*.yaml` / `*.yml` | Both extensions supported | |

---

## GNK-005: YAML Test Case Format

### GNK-006: Basic Structure

```yaml
- name: WHEN valid order arrives THEN record is created in database
  description: >-
    Full test description in BDD format.
    WHEN [condition]
    THEN [expected behavior]

  method: POST
  path: /api/v1/orders
  status: ""  # "", "broken", "skipped", "focus"

  fixtures:
    - empty_db
    - feature/base_order

  headers:
    Content-Type: application/json
    Authorization: "Bearer {{ $token }}"

  mocks:
    external_service:
      calls: 1
      strategy: file
      filename: mocks_data/service/success.json
      statusCode: 200
      requestConstraints:
        - kind: methodIsPOST
        - kind: pathMatches
          path: /api/endpoint
        - kind: bodyMatchesJSON
          body: '{"order_id": "ORD001"}'

  request: |
    {
      "order_id": "ORD001",
      "amount": 1000
    }

  response:
    200: |
      {
        "id": "$matchRegexp([0-9a-f-]{36})",
        "status": "created"
      }

  responseHeaders:
    200:
      Content-Type: application/json

  dbQuery: |
    SELECT id, order_id, status
    FROM orders
    WHERE order_id = 'ORD001'

  dbResponse:
    - '{"id": "$matchRegexp(\\d+)", "order_id": "ORD001", "status": "created"}'

  comparisonParams:
    ignoreArraysOrdering: true
    ignoreDbOrdering: true
```

### GNK-007: Test Statuses

| Status | Description |
|--------|-------------|
| `""` | Normal test (executed) |
| `broken` | Test is broken, not executed |
| `skipped` | Test is skipped |
| `focus` | ONLY this test is executed |

### GNK-008: Variables and Templating

```yaml
# Using environment variables
request: |
  {
    "events": {{ $kafka_event }}
  }

# Saving variables from response
variables_to_set:
  200:
    createdOrderId: "id"
    userName: "user.name"

# Using in next test
- name: Get created order
  path: /api/orders/{{ $createdOrderId }}
```

### GNK-009: Parameterized Tests (Cases)

```yaml
- name: Get order by status
  method: GET
  path: /api/orders?status={{ .status }}

  response:
    200: |
      {
        "count": {{ .expectedCount }}
      }

  cases:
    - requestArgs:
        status: "active"
      responseArgs:
        200:
          expectedCount: 5
    - requestArgs:
        status: "completed"
      responseArgs:
        200:
          expectedCount: 10
```

---

## GNK-010: Fixtures (Database Preparation)

### GNK-011: PostgreSQL/MySQL/ClickHouse

```yaml
# fixtures/base_templates.yaml
templates:
  base_order:
    status: pending
    created_at: $eval(NOW())
    updated_at: $eval(NOW())

  active_order:
    $extend: base_order
    status: active

# fixtures/feature/orders.yaml
inherits:
  - base_templates

tables:
  orders:
    - $name: order_001
      $extend: active_order
      order_id: ORD001
      amount: 1000
      user_id: $user_john.id  # Reference to another record

    - $extend: base_order
      order_id: ORD002
      amount: 500

  users:
    - $name: user_john
      name: John Doe
      email: john@example.com
```

### GNK-012: Table Cleanup

```yaml
# fixtures/empty_db.yaml
tables:
  orders: []
  users: []
  order_items: []
```

### GNK-013: Redis/Valkey Fixtures

```yaml
# fixtures/valkey/feature/promises.yaml
databases:
  0:
    sets:
      values:
        tracking_items:
          values:
            - value: ITEM001
            - value: ITEM002

    zsets:
      values:
        promises:exportable:
          values:
            - value: ITEM001
              score: 1715012009
            - value: ITEM002
              score: 1715012010

    hashes:
      values:
        item:ITEM001:
          values:
            - key: status
              value: active
            - key: deadline
              value: "1715012009"

    keys:
      values:
        simple_key:
          value: "string_value"
          expiration: 1h
```

### GNK-014: Key Fixture Constructs

| Construct | Description |
|-----------|-------------|
| `$name: <name>` | Named record for references |
| `$extend: <template>` | Inheritance from template |
| `$refName.field` | Reference to record field |
| `$eval(...)` | SQL expression (NOW(), UUID(), etc.) |
| `inherits: [...]` | File inheritance |
| `tables: { name: [] }` | Table cleanup |

---

## GNK-015: Mocking External Services

### GNK-016: Mock Initialization (Go)

```go
// Creating mocks
m := mocks.NewNop("payment", "delivery", "inventory")

if err := m.Start(); err != nil {
    t.Fatal(err)
}
defer m.Shutdown()

// Getting mock address
paymentAddr := m.Service("payment").ServerAddr()

// Also available via env: GONKEY_MOCK_PAYMENT
```

### GNK-017: YAML Configuration

```yaml
mocks:
  payment_service:
    # Expected number of calls
    calls: 1

    # Response strategy
    strategy: file  # constant, file, template, sequence, etc.
    filename: mocks_data/payment/success.json
    statusCode: 200

    headers:
      Content-Type: application/json

    # Request validation
    requestConstraints:
      - kind: methodIsPOST
      - kind: pathMatches
        path: /api/v1/charge
      - kind: headerIs
        header: Authorization
        regexp: ^Bearer .+$
      - kind: bodyMatchesJSON
        body: |
          {
            "amount": 1000,
            "currency": "RUB"
          }
```

### GNK-018: Request Constraints

| Constraint | Description |
|------------|-------------|
| `nop` | Always matches |
| `methodIs` / `methodIsPOST` / `methodIsGET` | HTTP method verification |
| `pathMatches` | Path (path or regexp) |
| `queryMatches` | Query parameters |
| `headerIs` | Header verification |
| `bodyMatchesJSON` | JSON body (subset matching) |
| `bodyMatchesText` | Text body |
| `bodyMatchesXML` | XML body |

### GNK-019: Response Strategies

| Strategy | Description |
|----------|-------------|
| `nop` | No response |
| `constant` | Static response |
| `file` | Response from file |
| `template` | Template with request data |
| `sequence` | Sequence of responses |
| `uriVary` | Different responses by URI |
| `methodVary` | Different responses by method |
| `basedOnRequest` | Conditional responses |
| `dropRequest` | Connection drop |

---

## GNK-020: Response Verification

### GNK-021: JSON Response with Regexps

```yaml
response:
  200: |
    {
      "id": "$matchRegexp([0-9a-f-]{36})",
      "status": "$matchRegexp(active|pending|completed)",
      "code": "$matchRegexp(^[A-Z]{2}\\d{4}$)",
      "items": [
        "$matchRegexp(ITEM\\d{3})",
        "$matchRegexp(ITEM\\d{3})"
      ]
    }
```

### GNK-022: Comparison Parameters

```yaml
comparisonParams:
  ignoreValues: false           # Check only structure
  ignoreArraysOrdering: true    # Ignore array order
  disallowExtraFields: false    # Forbid extra fields
  ignoreDbOrdering: true        # Ignore database order
```

### GNK-023: Database Verification

```yaml
# Simple verification
dbQuery: |
  SELECT id, status FROM orders WHERE order_id = 'ORD001'

dbResponse:
  - '{"id": 1, "status": "active"}'

# Multiple verifications
dbChecks:
  - dbQuery: SELECT COUNT(*) as count FROM orders
    dbResponse:
      - '{"count": 5}'

  - dbQuery: SELECT status FROM orders WHERE id = 1
    dbResponse:
      - '{"status": "active"}'
```

---

## GNK-024: Test Runner Configuration

### GNK-025: Main Runner

```go
type TestRunner struct {
    *testing.T
    CaseDirs []TestCase
    Mocks    []string
    EnvVars  map[string]string
}

func (r TestRunner) Run(t *testing.T) {
    if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
        t.Skip("Set RUN_INTEGRATION_TESTS=true")
    }

    // Setup environment
    env := NewEnv(t.Logf)
    for key, val := range r.EnvVars {
        env.Set(key, val)
    }
    defer env.CleanUp()

    // Setup mocks
    m := mocks.NewNop(r.Mocks...)
    m.Start()
    defer m.Shutdown()

    // Setup test server
    testServer := httptest.NewServer(yourHandler)
    defer testServer.Close()

    // Run gonkey
    runner.RunWithTesting(t, &runner.RunWithTestingParams{
        TestsDir:      "cases/" + caseDir.TestDir,
        Server:        testServer,
        Mocks:         m,
        FixturesDir:   "fixtures",
        DbType:        fixtures.Postgres,
        FixtureLoader: customLoader,
        Checkers:      []checker.CheckerInterface{customChecker},
    })
}
```

### GNK-026: Loading Events as Variables

```go
func TestKafkaConsumer(t *testing.T) {
    runner.TestRunner{
        T:        t,
        Mocks:    []string{"external_service"},
        CaseDirs: []runner.TestCase{{TestDir: "/kafka/topic"}},
        EnvVars: map[string]string{
            "order_event": PrepareEventsJSON("./events/topic/", "order"),
        },
    }.Run(t)
}

func PrepareEventsJSON(path, filename string) string {
    rawEvents, _ := os.ReadFile(filepath.Join(path, filename+".json"))
    buffer := new(bytes.Buffer)
    json.Compact(buffer, rawEvents)
    return buffer.String()
}
```

---

## GNK-027: Custom Fixture Loaders and Checkers

### GNK-028: Custom Fixture Loader

```go
type Loader struct {
    db       *sql.DB
    location string
}

func (l *Loader) Load(names []string) error {
    tx, _ := l.db.Begin()
    defer tx.Rollback()

    // 1. Truncate tables
    for _, table := range tables {
        tx.Exec("TRUNCATE TABLE " + table)
    }

    // 2. Insert data
    for _, row := range rows {
        // Build and execute INSERT
    }

    return tx.Commit()
}
```

### GNK-029: Custom Database Checker

```go
type ResponseDbChecker struct {
    db *sql.DB
}

func (c *ResponseDbChecker) Check(t models.TestInterface, result *models.Result) ([]error, error) {
    // Execute dbQuery
    actualResponse, _ := c.executeQuery(t.DbQueryString())

    // Compare with expected
    errs := compare.Compare(
        t.DbResponseJson(),
        actualResponse,
        compare.Params{IgnoreArraysOrdering: t.IgnoreDbOrdering()},
    )

    return errs, nil
}
```

---

## GNK-030: Running Tests

### GNK-031: Environment Variables

```bash
# Required for running
RUN_INTEGRATION_TESTS=true

# Configuration
APP_CONF=/path/to/config.env

# Debug mode
GONKEY_DEBUG=true
```

### GNK-032: Makefile

```makefile
test-integration:
	RUN_INTEGRATION_TESTS=true \
	APP_CONF=${PWD}/dev/local.env \
	go test -v ./tests/...

test-integration-debug:
	GONKEY_DEBUG=true \
	RUN_INTEGRATION_TESTS=true \
	APP_CONF=${PWD}/dev/local.env \
	go test -v ./tests/... -run TestSpecificTest
```

---

## GNK-033: Best Practices

### GNK-034: Test Naming (BDD Style)

```yaml
- name: "WHEN order arrives in active status THEN record is created with deadline"
- name: "WHEN user is not authorized THEN 401 is returned"
```

### GNK-035: Test Isolation

```yaml
# Each test starts with clean state
fixtures:
  - empty_db
  - specific_test_data
```

### GNK-036: Fixture Organization

```yaml
# Base templates at root
# fixtures/base_templates.yaml
templates:
  base_order:
    status: pending

# Feature-specific inherit base
# fixtures/feature/orders.yaml
inherits:
  - base_templates
```

### GNK-037: Mock Call Count Verification

```yaml
mocks:
  service:
    calls: 1  # Ensures service is called exactly once
```

### GNK-038: Using $matchRegexp for Dynamic Fields

```yaml
response:
  200: |
    {
      "id": "$matchRegexp([0-9a-f-]{36})",
      "created_at": "$matchRegexp(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2})"
    }
```

---

## GNK-039: Common Mistakes

| Error | Solution |
|-------|----------|
| Template field references | Use `$name` for named records |
| Mocks not working | Mocks work only in library mode, not CLI |
| Fixture order | Account for order in `inherits` |
| Escape in regexp | Use `\\d` instead of `\d` in YAML |
| Database cleanup | `-db_dsn` cleans database, use separate test DB |

---

## GNK-040: Related Documentation

- [Integration Test Standards](../guides/golang/integration_tests_standards.md)
- [Unit Test Standards](../guides/golang/unit_tests_standards.md)
- [Testify Patterns](testify.md)
