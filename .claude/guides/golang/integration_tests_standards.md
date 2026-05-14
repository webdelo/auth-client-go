#AI Instructions: Writing Integration Tests with Gonkey

Each rule has a unique identifier (IT-XXXX) for reference in code reviews and discussions.

---

## Critical Rules (from checklist)

- [ ] **[IT-0001]** Test file location: `tests/*_test.go` (отдельный пакет tests)
- [ ] **[IT-0002]** YAML test case naming: `+happy_path.yaml` для позитивных, `-error_case.yaml` для негативных
- [ ] **[IT-0003]** Fixtures: ВСЕГДА начинай с `empty_db` для изоляции тестов
- [ ] **[IT-0004]** Mocks: ВСЕГДА указывай `calls: N` для проверки количества вызовов
- [ ] **[IT-0005]** dbQuery: Используй JSON агрегацию для читаемого сравнения результатов
- [ ] **[IT-0006]** `$matchRegexp()` для динамических полей (UUID, timestamps)
- [ ] **[IT-0007]** BDD naming: "КОГДА [условие] ТОГДА [результат]"
- [ ] **[IT-0008]** Environment: `RUN_INTEGRATION_TESTS=true` для запуска тестов
- [ ] **[IT-0009]** NO business logic in tests - только проверка интеграции компонентов
- [ ] **[IT-0010]** NO parallel execution - интеграционные тесты работают с общими ресурсами

---

## Test Preparation and Execution Order

### Required Infrastructure
```bash
# Перед запуском тестов должны быть доступны:
# - PostgreSQL/ClickHouse с тестовой БД
# - Redis/Valkey
# - Конфигурационный файл с переменными окружения

# Запуск инфраструктуры
docker-compose -f docker-compose.test.yml up -d

# Проверка готовности
docker-compose -f docker-compose.test.yml ps
```

### Running Tests
```bash
# Все интеграционные тесты
RUN_INTEGRATION_TESTS=true \
APP_CONF=${PWD}/dev/local.env \
go test -v ./tests/...

# Конкретный тест
RUN_INTEGRATION_TESTS=true \
APP_CONF=${PWD}/dev/local.env \
go test -v ./tests/... -run TestOrderConsumer

# Debug режим (вывод SQL запросов)
GONKEY_DEBUG=true \
RUN_INTEGRATION_TESTS=true \
APP_CONF=${PWD}/dev/local.env \
go test -v ./tests/...
```

---

## 1. Layer Strategy

### IT-0020: What to test at each layer

| Слой | Тестируем | НЕ тестируем |
|------|-----------|--------------|
| **HTTP API Handlers** | Роутинг, парсинг, валидация, статусы | Бизнес-логика (покрыта unit) |
| **Kafka Consumers** | Парсинг событий, side-effects в БД | Детали обработчиков |
| **External Services** | Корректность вызовов через mocks | Саму внешнюю логику |
| **Database** | Корректность записей после операций | Производительность |
| **Cron Jobs** | Запуск, результаты в БД/метриках | Внутреннюю логику |

---

## 2. Project Structure

### IT-0030: Mandatory project structure

```
tests/
├── cases/                          # YAML тест-кейсы
│   ├── api/                        # HTTP API тесты
│   │   └── orders/
│   │       ├── +create_order.yaml
│   │       ├── +get_order.yaml
│   │       └── -invalid_payload.yaml
│   ├── kafka/                      # Kafka consumer тесты
│   │   └── order_updates/
│   │       ├── +consume_new_order.yaml
│   │       └── -invalid_event.yaml
│   └── support/                    # Admin/support API
│       └── health/
│           └── +health_check.yaml
│
├── fixtures/                       # Database fixtures
│   ├── base_templates.yaml         # Общие шаблоны
│   ├── empty_db.yaml               # Очистка всех таблиц
│   ├── postgres/                   # PostgreSQL данные
│   │   └── orders/
│   │       └── active_orders.yaml
│   └── valkey/                     # Redis/Valkey данные
│       └── cache/
│           └── session_data.yaml
│
├── events/                         # JSON события для Kafka
│   └── order_updates/
│       ├── new_order.json
│       └── canceled_order.json
│
├── mocks_data/                     # Ответы mock-сервисов
│   └── payment_service/
│       └── charge/
│           ├── success.json
│           └── declined.json
│
├── runner/                         # Test runner infrastructure
│   ├── runner.go                   # Main runner
│   ├── env.go                      # Environment management
│   └── postgres/
│       ├── fixtures_loader.go
│       └── response_checker.go
│
├── handler/                        # Fake handlers
│   ├── fake_kafka_handler.go
│   └── metrics_handler.go
│
├── server/                         # Test HTTP server
│   └── server.go
│
├── api_test.go                     # HTTP API tests
├── kafka_test.go                   # Kafka consumer tests
└── support_test.go                 # Support endpoint tests
```

---

## 3. YAML Test Case Structure

### IT-0040: HTTP API Test structure

```yaml
# tests/cases/api/orders/+create_order.yaml
- name: КОГДА создается заказ с валидными данными ТОГДА возвращается 201 и заказ сохраняется в БД
  description: >-
    КОГДА пользователь отправляет POST /api/v1/orders с валидным payload
    И указан корректный Authorization header
    ТОГДА возвращается статус 201
    ТОГДА заказ создается в таблице orders
    ТОГДА вызывается payment service для резервирования

  method: POST
  path: /api/v1/orders

  fixtures:
    - empty_db
    - postgres/users/active_user

  headers:
    Content-Type: application/json
    Authorization: "Bearer {{ $auth_token }}"

  mocks:
    payment_service:
      calls: 1
      strategy: file
      filename: mocks_data/payment_service/reserve/success.json
      statusCode: 200
      requestConstraints:
        - kind: methodIsPOST
        - kind: pathMatches
          path: /api/v1/reserve
        - kind: bodyMatchesJSON
          body: |
            {
              "amount": 1000,
              "currency": "RUB"
            }

  request: |
    {
      "user_id": "{{ $user_id }}",
      "items": [
        {"sku": "ITEM001", "quantity": 2, "price": 500}
      ],
      "total": 1000
    }

  response:
    201: |
      {
        "id": "$matchRegexp([0-9a-f-]{36})",
        "status": "pending",
        "total": 1000
      }

  responseHeaders:
    201:
      Content-Type: application/json

  dbQuery: |
    SELECT json_build_object(
      'user_id', user_id,
      'status', status,
      'total', total
    )::text AS row
    FROM orders
    WHERE user_id = '{{ $user_id }}'

  dbResponse:
    - '{"user_id": "{{ $user_id }}", "status": "pending", "total": 1000}'
```

### IT-0041: Kafka Consumer Test structure

```yaml
# tests/cases/kafka/order_updates/+consume_new_order.yaml
- name: КОГДА приходит событие нового заказа ТОГДА создается запись в БД
  description: >-
    КОГДА из топика order.updates приходит событие типа ORDER_CREATED
    ТОГДА создается запись в таблице order_events
    ТОГДА НЕ вызывается notification service (тест проверяет calls: 0)

  method: POST
  path: /events.batch-produce

  fixtures:
    - empty_db

  mocks:
    notification_service:
      calls: 0
      strategy: nop

  request: |
    {
      "id": "test-request-001",
      "jsonrpc": "2.0",
      "method": "events.batch-produce",
      "params": {
        "topic": "order.updates",
        "events": {{ $new_order_event }}
      }
    }

  response:
    200: |
      {
        "id": "test-request-001",
        "jsonrpc": "2.0",
        "result": "ok"
      }

  dbQuery: |
    SELECT json_build_object(
      'order_id', order_id,
      'event_type', event_type,
      'status', status
    )::text AS row
    FROM order_events
    ORDER BY created_at

  dbResponse:
    - '{"order_id": "ORD001", "event_type": "created", "status": "processed"}'

  comparisonParams:
    ignoreDbOrdering: true
```

### IT-0042: Negative Test Case structure

```yaml
# tests/cases/api/orders/-invalid_payload.yaml
- name: КОГДА payload невалидный ТОГДА возвращается 400 и заказ НЕ создается
  description: >-
    КОГДА пользователь отправляет POST /api/v1/orders с невалидным JSON
    ТОГДА возвращается статус 400
    ТОГДА заказ НЕ создается в БД
    ТОГДА payment service НЕ вызывается

  method: POST
  path: /api/v1/orders

  fixtures:
    - empty_db

  headers:
    Content-Type: application/json
    Authorization: "Bearer {{ $auth_token }}"

  mocks:
    payment_service:
      calls: 0
      strategy: nop

  request: |
    {
      "user_id": "",
      "items": []
    }

  response:
    400: |
      {
        "error": "$matchRegexp(.+)",
        "code": "VALIDATION_ERROR"
      }

  dbQuery: |
    SELECT COUNT(*)::text AS count FROM orders

  dbResponse:
    - '0'
```

---

## 4. Fixtures

### IT-0050: Base Templates

```yaml
# tests/fixtures/base_templates.yaml
templates:
  base_order:
    status: pending
    currency: RUB
    created_at: $eval(NOW())
    updated_at: $eval(NOW())

  active_order:
    $extend: base_order
    status: active

  completed_order:
    $extend: base_order
    status: completed

  base_user:
    role: customer
    active: true
    created_at: $eval(NOW())
```

### IT-0003: Empty DB Fixture (Isolation)

```yaml
# tests/fixtures/empty_db.yaml
# ВАЖНО: Перечислить ВСЕ таблицы для полной изоляции
tables:
  orders: []
  order_items: []
  order_events: []
  users: []
  payments: []
  notifications: []
```

### Feature-Specific Fixtures

```yaml
# tests/fixtures/postgres/orders/active_orders.yaml
inherits:
  - base_templates

tables:
  users:
    - $name: test_user
      $extend: base_user
      id: user-001
      email: test@example.com

  orders:
    - $name: order_001
      $extend: active_order
      id: order-001
      user_id: $test_user.id
      total: 1000

    - $name: order_002
      $extend: active_order
      id: order-002
      user_id: $test_user.id
      total: 2000
```

### IT-0051: Redis/Valkey Fixtures

```yaml
# tests/fixtures/valkey/cache/session_data.yaml
databases:
  0:
    keys:
      values:
        session:user-001:
          value: '{"user_id": "user-001", "role": "customer"}'
          expiration: 1h

    sets:
      values:
        active_sessions:
          values:
            - value: "session:user-001"
            - value: "session:user-002"

    zsets:
      values:
        rate_limit:api:
          values:
            - value: "user-001"
              score: 100
```

---

## 5. Test Runner Structure

### IT-0060: Main Test File Pattern

```go
// tests/kafka_test.go
package tests

import (
    "testing"
    "gitlab.example.com/project/tests/runner"
)

func TestOrderUpdatesConsumer(t *testing.T) {
    runner.TestRunner{
        T:        t,
        Mocks:    []string{"notification_service"},
        CaseDirs: []runner.TestCase{{TestDir: "/kafka/order_updates"}},
        EnvVars: map[string]string{
            "new_order_event":      runner.PrepareEventsJSON("./events/order_updates/", "new_order"),
            "canceled_order_event": runner.PrepareEventsJSON("./events/order_updates/", "canceled_order"),
        },
    }.Run(t)
}

func TestAPIOrdersHandlers(t *testing.T) {
    runner.TestRunner{
        T:        t,
        Mocks:    []string{"payment_service"},
        CaseDirs: []runner.TestCase{{TestDir: "/api/orders"}},
        EnvVars: map[string]string{
            "auth_token": "test-jwt-token",
            "user_id":    "user-001",
        },
    }.Run(t)
}
```

### Runner Implementation

```go
// tests/runner/runner.go
package runner

import (
    "net/http/httptest"
    "os"
    "testing"

    "github.com/lamoda/gonkey/fixtures"
    "github.com/lamoda/gonkey/mocks"
    "github.com/lamoda/gonkey/runner"
    "github.com/stretchr/testify/require"
)

type TestCase struct {
    TestDir string
}

type TestRunner struct {
    *testing.T
    CaseDirs []TestCase
    Mocks    []string
    EnvVars  map[string]string
}

func (r TestRunner) Run(t *testing.T) {
    if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
        t.Skip("Skipped: set RUN_INTEGRATION_TESTS=true")
    }

    // Setup environment variables
    env := NewEnv(t.Logf)
    for key, val := range r.EnvVars {
        env.Set(key, val)
    }
    defer env.CleanUp()

    // Load configuration
    cfg, err := config.NewFromEnv()
    require.NoError(t, err)

    // Setup mocks
    var m *mocks.Mocks
    if len(r.Mocks) > 0 {
        m = mocks.NewNop(r.Mocks...)
        require.NoError(t, m.Start())
        defer m.Shutdown()

        // Configure mock addresses in config
        for _, name := range r.Mocks {
            addr := m.Service(name).ServerAddr()
            cfg.SetServiceURL(name, "http://"+addr)
        }
    }

    // Setup database connection
    db, err := OpenTestDB(cfg)
    require.NoError(t, err)

    // Setup application container
    container := app.NewContainer(cfg)

    // Setup test HTTP server
    testServer := httptest.NewServer(container.Router)
    defer testServer.Close()

    // Run gonkey tests for each case directory
    for _, caseDir := range r.CaseDirs {
        runner.RunWithTesting(t, &runner.RunWithTestingParams{
            TestsDir:      "cases" + caseDir.TestDir,
            Server:        testServer,
            Mocks:         m,
            FixturesDir:   "fixtures",
            DbType:        fixtures.Postgres,
            FixtureLoader: NewPostgresLoader(db, "fixtures"),
            Checkers:      []checker.CheckerInterface{NewPostgresChecker(db)},
        })
    }
}
```

---

## 6. Mock Configuration

### IT-0070: Mock Strategies

```yaml
# Проверка отсутствия вызовов (IT-0004: always specify calls)
mocks:
  service:
    calls: 0
    strategy: nop

# Статический ответ
mocks:
  service:
    calls: 1
    strategy: constant
    statusCode: 200
    body: '{"status": "ok"}'
    headers:
      Content-Type: application/json

# Ответ из файла
mocks:
  service:
    calls: 1
    strategy: file
    filename: mocks_data/service/success.json
    statusCode: 200

# Последовательность ответов (retry scenarios)
mocks:
  service:
    strategy: sequence
    sequence:
      - strategy: constant
        statusCode: 500
        body: '{"app_error": "temporary"}'
      - strategy: constant
        statusCode: 200
        body: '{"status": "ok"}'
```

### IT-0071: Request Validation with requestConstraints

```yaml
mocks:
  payment_service:
    calls: 1
    strategy: file
    filename: mocks_data/payment/success.json
    statusCode: 200

    requestConstraints:
      # HTTP method
      - kind: methodIsPOST

      # Path matching
      - kind: pathMatches
        path: /api/v1/charge

      # Header validation
      - kind: headerIs
        header: Authorization
        regexp: ^Bearer .+$

      # JSON body (subset matching)
      - kind: bodyMatchesJSON
        body: |
          {
            "amount": 1000,
            "currency": "RUB"
          }
```

---

## 7. Database Verification

### IT-0080: PostgreSQL with JSON aggregation

```yaml
# Простая проверка одной записи
dbQuery: |
  SELECT json_build_object(
    'order_id', order_id,
    'status', status,
    'total', total
  )::text AS row
  FROM orders
  WHERE order_id = 'ORD001'

dbResponse:
  - '{"order_id": "ORD001", "status": "active", "total": 1000}'

# Проверка нескольких записей
dbQuery: |
  SELECT json_build_object(
    'event_type', event_type,
    'processed', processed
  )::text AS row
  FROM order_events
  WHERE order_id = 'ORD001'
  ORDER BY created_at

dbResponse:
  - '{"event_type": "created", "processed": true}'
  - '{"event_type": "confirmed", "processed": true}'

comparisonParams:
  ignoreDbOrdering: false  # Порядок важен
```

### IT-0081: ClickHouse verification

```yaml
dbQuery: |
  SELECT toJSONString(
    map(
      'order_item_nr', order_item_nr,
      'status', status,
      'type', type
    )
  ) AS row
  FROM order_flow_events
  WHERE order_id = 'ORD001'

dbResponse:
  - '{"order_item_nr": "ORD001-001", "status": "active", "type": "fact"}'
```

### IT-0082: Count verification

```yaml
dbQuery: |
  SELECT COUNT(*)::text AS count FROM orders WHERE status = 'active'

dbResponse:
  - '5'
```

---

## 8. Response Validation

### IT-0090: Exact match

```yaml
response:
  200: |
    {
      "status": "ok",
      "count": 5
    }
```

### IT-0006: Regexp matching for dynamic fields

```yaml
response:
  201: |
    {
      "id": "$matchRegexp([0-9a-f-]{36})",
      "created_at": "$matchRegexp(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2})",
      "status": "pending"
    }
```

### IT-0091: Comparison parameters

```yaml
comparisonParams:
  # Игнорировать порядок элементов в массивах
  ignoreArraysOrdering: true

  # Проверять только структуру, не значения
  ignoreValues: false

  # Запретить лишние поля в ответе
  disallowExtraFields: true

  # Игнорировать порядок в результатах БД
  ignoreDbOrdering: true
```

---

## 9. Best Practices

### IT-0003: Test Isolation — always start with empty_db

```yaml
# ВСЕГДА начинай с empty_db
fixtures:
  - empty_db
  - specific_test_data
```

### IT-0007: BDD Naming

```yaml
# ХОРОШО - описывает условие и результат
- name: КОГДА пользователь создает заказ с нулевой суммой ТОГДА возвращается ошибка валидации

# ПЛОХО - не информативно
- name: Test create order validation
```

### IT-0004: Mock Call Verification

```yaml
# ХОРОШО - явно указано количество вызовов
mocks:
  service:
    calls: 1  # или calls: 0 для проверки отсутствия вызова

# ПЛОХО - не контролируем вызовы
mocks:
  service:
    strategy: constant
    body: '{}'
```

### IT-0005: Specific Assertions

```yaml
# ХОРОШО - проверяем конкретные поля
dbQuery: |
  SELECT json_build_object('status', status, 'total', total)::text
  FROM orders WHERE id = 'ORD001'

dbResponse:
  - '{"status": "active", "total": 1000}'

# ПЛОХО - SELECT * без явного указания полей
```

### Meaningful Event Files

```json
// events/order_updates/new_order.json
[
  {
    "occurred_at": "2024-01-15T10:00:00Z",
    "type": "ORDER_CREATED",
    "data": {
      "order_id": "ORD001",
      "user_id": "user-001",
      "items": [
        {"sku": "ITEM001", "quantity": 2}
      ]
    }
  }
]
```

---

## 10. Anti-patterns

### 1. Testing implementation details (violates IT-0009)

```yaml
# ПЛОХО - тестируем внутреннюю структуру
dbQuery: |
  SELECT internal_counter, cache_key FROM orders

# ХОРОШО - тестируем бизнес-результат
dbQuery: |
  SELECT status, total FROM orders WHERE order_id = 'ORD001'
```

### 2. Overly complex fixtures (violates IT-0003)

```yaml
# ПЛОХО - слишком много данных
fixtures:
  - all_users
  - all_orders
  - all_products

# ХОРОШО - минимум необходимого
fixtures:
  - empty_db
  - test_specific_user
```

### 3. Missing negative cases (violates IT-0002)

```yaml
# ПЛОХО - только happy path
tests/cases/api/orders/
  +create_order.yaml

# ХОРОШО - покрыты app_error cases
tests/cases/api/orders/
  +create_order.yaml
  -invalid_payload.yaml
  -unauthorized.yaml
  -duplicate_order.yaml
```

### 4. Hardcoded IDs in dbResponse (violates IT-0006)

```yaml
# ПЛОХО - захардкоженный автоинкремент ID
dbResponse:
  - '{"id": 1, "status": "active"}'

# ХОРОШО - используем regexp или бизнес-идентификатор
dbResponse:
  - '{"id": "$matchRegexp(\\d+)", "status": "active"}'
  # или
  - '{"order_id": "ORD001", "status": "active"}'
```

---

## CI/CD Integration

### Makefile

```makefile
.PHONY: test-integration

test-integration:
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for services..."
	sleep 5
	RUN_INTEGRATION_TESTS=true \
	APP_CONF=${PWD}/dev/test.env \
	go test -v ./tests/...
	docker-compose -f docker-compose.test.yml down

test-integration-coverage:
	docker-compose -f docker-compose.test.yml up -d
	sleep 5
	RUN_INTEGRATION_TESTS=true \
	APP_CONF=${PWD}/dev/test.env \
	go test -cover -coverprofile=coverage-integration.out \
	-coverpkg=./... -v ./tests/...
	docker-compose -f docker-compose.test.yml down
```

### Docker Compose

```yaml
# docker-compose.test.yml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: test
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U test"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
```

---

## Related Documents

- [Gonkey Pattern](../../guides/libs/gonkey.md) - полное описание gonkey
