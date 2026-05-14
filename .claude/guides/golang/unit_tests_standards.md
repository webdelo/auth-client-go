# AI Instructions: Writing Tests in Go

Each rule has a unique identifier (UT-XXXX) for reference in code reviews and discussions.

---

## Critical Rules (from checklist)
- [ ] **[UT-0010]** Function name: `Test<Type>_<Method>` (e.g., `TestProcessor_NewFeature`)
- [ ] **[UT-0011]** First case: `"happy_path"`
- [ ] **[UT-0022]** All mocks with `.Once()`, `.Times(0)` or `.Times(n)`
- [ ] **[UT-0023]** **`mock.Anything` и `mock.AnythingOfType` ЗАПРЕЩЕНЫ ПОЛНОСТЬЮ** — для ЛЮБЫХ параметров (включая ctx, logger, SQL строки). Используй точные значения или `mock.MatchedBy()` с типизированной валидацией
- [ ] **[UT-0001]** Follow table-driven test patterns for all unit tests.
- [ ] Use [testify](../../guides/libs/testify.md) lib for all tests.
- [ ] Handle errors from ALL dependencies (repo, service, processor)
- [ ] **[UT-0030]** `require` for critical checks (stops the test)
- [ ] **[UT-0031]** `assert` for additional checks (continues the test)
- [ ] **[UT-0032]** No logic (if/switch) in tests - use closures in `want`
- [ ] **[UT-0051]** No Magic Numbers - use named constants
- [ ] **[UT-0050]** **Complex objects extracted to private functions** (newTestOrder, newTestData)
- [ ] **[UT-0040]** `t.Parallel()` at the beginning of the main test and each subtest
- [ ] **[UT-0041]** `tc := tc` when using t.Parallel()
- [ ] **[UT-0024]** New mocks for each subtest (not shared between tests)

---
## Test Preparation and Execution Order

### Required Actions Before Writing Tests
- [ ] Code validation completed (code compiles, written without critical errors that prevent project build).
- [ ] Mocks generated for test execution.

### Code Validation (correct project build)
```bash
# Run these FIRST - fix any errors before proceeding
go fmt ./...                    # Format code
go vet ./...                   # Static analysis
golangci-lint run ./...        # Comprehensive linting (if configured)
go build ./...                 # Ensure everything compiles

# Expected: No errors. If errors, READ the app_error and fix.
```

### Mock Preparation
```bash
# All mocks should be generated using mockery
# When writing tests, TELL ME which mocks are needed:
# - Interface name
# - Path to the interface file
# - Which methods will be mocked
# I will configure mockery appropriately for generation
```

---

## 1. 🎯 Layer Strategy

| Layer | Coverage | What We Test | What We DON'T Test |
|------|----------|---------------|------------------|
| **Domain Layer** (processors) | 100% | All business logic, branching, errors | — |
| **Service Layer** | High | Builders, parsing, API clients | Real HTTP calls |
| **Application Layer** (handlers) | Basic | Routing, parsing, basic errors | Business logic |
| **Infrastructure Layer** (repositories) | High | Go-логика: маппинг, ошибки, nullable, edge cases | Реальные SQL запросы (покрыты gonkey) |
| **Entry Point** (cmd) | None | — | Covered by Gonkey tests |

---

## 2. 📝 Naming

### UT-0010: Function naming format Test\<Type\>\_\<Method\>

**Functions:**
```go
func Test<Type>_<Method>(t *testing.T)

// ✅ Correct
func TestProcessor_DeliveryIntentCreate(t *testing.T)
func TestBuilder_BuildParams(t *testing.T)

// ❌ Incorrect
func TestProcess(t *testing.T)
func TestHandler(t *testing.T)
```

### UT-0011: First case must be "happy\_path"

### UT-0012: Test case naming in snake\_case

**Test cases:**
```go
// ✅ Correct: verb + short description
"happy_path"
"return_error_when_repo_fails"
"skip_processing_when_flag_disabled"

// ❌ Incorrect
"order with flag should be skipped"
"should return app_error for repo"
```

---

## 3. 🏗️ Unit Test Structure

### UT-0001: Table-driven tests mandatory
All unit tests MUST use the table-driven test pattern. One method = one test function with a case table.

### UT-0002: One method = one test function
FORBIDDEN to create multiple test functions for one method (e.g., `TestLoadFromPath_Success`, `TestLoadFromPath_Error`).
All cases MUST be in one table inside one `Test<Type>_<Method>` function.

### Simple functions (without dependencies)
```go
tests := map[string]struct {
    input    string
    expected bool
}{
    "happy_path": {
        input:    "valid",
        expected: true,
    },
}
```

### Complex functions (with dependencies)
```go
tests := map[string]struct {
    args         struct { orderNr string }
    prepareMocks func(repo *mock.MockRepo, service *mock.MockService)
    want         func(t *testing.T, result Result, err error)
}{
    "happy path": {
        args: struct{ orderNr string }{orderNr: "ORD123"},
        prepareMocks: func(repo *mock.MockRepo, service *mock.MockService) {
            expectedOrder := &Order{}
            repo.EXPECT().Find("ORD123").Return(expectedOrder, nil).Once()
            service.EXPECT().Create(expectedOrder).Return(nil).Once()
        },
        want: func(t *testing.T, result Result, err error) {
            require.NoError(t, err)
            assert.Equal(t, "SENT", result.Status)
        },
    },
}
```

---

## 4. 🎭 Working with Mocks

### UT-0020: Mocks ONLY via mockery (never manual mock structs)
FORBIDDEN to write mock structs manually (`type MockXxx struct { mock.Mock }`).
REQUIRED: use mocks generated by mockery from `mocks/` directory.

For gRPC server interfaces, use the custom gRPC template
(see [libs/mockery.md](../../guides/libs/mockery.md) § gRPC Server Mocks).

### UT-0021: Use Expecter API, not .On()
REQUIRED: `.EXPECT().Method().Return().Once()`
FORBIDDEN: `.On("Method", ...)`

### UT-0022: All mocks with explicit call count
```go
// ✅ ALWAYS specify call count
repo.EXPECT().Find(orderNr).Return(order, nil).Once()

// UT-0023: mock.Anything ЗАПРЕЩЁН ПОЛНОСТЬЮ
// ✅ Handle all app_error types
"handle_repo.Find_error_and_return_err"
"handle_service.Create_error_and_return_err"
"handle_processor.Process_error_and_return_err"

// UT-0024: New mocks for each subtest (not shared)
// ✅ Explicitly verify absence of calls
tx.EXPECT().Commit().Times(0)
tx.EXPECT().Rollback().Return(nil).Once()
```

### UT-0023: ПОЛНЫЙ ЗАПРЕТ на mock.Anything и mock.AnythingOfType
`mock.Anything` и `mock.AnythingOfType` **ЗАПРЕЩЕНЫ ПОЛНОСТЬЮ** — для ЛЮБЫХ параметров, включая `context.Context`, logger, SQL строки.

**Вместо mock.Anything / mock.AnythingOfType используй:**
- Для `context.Context` — передавай ту же переменную `ctx`, что используется в тесте
- Для бизнес-данных — передавай точное ожидаемое значение
- Для сложных объектов — используй `mock.MatchedBy(func)` с конкретной проверкой
- Для nullable полей (`*string`, `*bool`, `*time.Time`) — используй `mock.MatchedBy()` с типизированной проверкой

```go
// ✅ ПРАВИЛЬНО — точные значения
ctx := context.Background()
repo.EXPECT().Find(ctx, orderNr).Return(order, nil).Once()
service.EXPECT().Create(ctx, expectedOrder).Return(nil).Once()

// ✅ ПРАВИЛЬНО — mock.MatchedBy для сложной валидации
repo.EXPECT().Save(ctx, mock.MatchedBy(func(o *Order) bool {
    return o.Status == "NEW" && o.Nr == "ORD123"
})).Return(nil).Once()

// ❌ ЗАПРЕЩЕНО — mock.Anything для любого параметра
repo.EXPECT().Find(mock.Anything, mock.Anything).Return(order, nil).Once()
service.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

// ❌ ЗАПРЕЩЕНО — mock.AnythingOfType для любого параметра
conn.EXPECT().Query(mock.AnythingOfType("*context.Background"), mock.AnythingOfType("string"), id).Return(rows, nil).Once()

// ✅ ПРАВИЛЬНО — точные значения или mock.MatchedBy
conn.EXPECT().Query(ctx, expectedSQL, id).Return(rows, nil).Once()
```

### UT-0024: New mocks for each subtest
Each subtest MUST create its own mock instances. Sharing mocks between subtests leads to flaky tests.

### UT-0025: Mocking DB Transactions
```go
prepareMocks: func(db *mock.MockDB, tx *mock.MockTx) {
    ctx := context.Background()
    db.EXPECT().BeginTx(ctx).Return(tx, nil).Once()

    // Success scenario
    tx.EXPECT().Commit().Return(nil).Once()
    tx.EXPECT().Rollback().Times(0)

    // Or app_error scenario
    tx.EXPECT().Commit().Times(0)
    tx.EXPECT().Rollback().Return(nil).Once()
}
```

### UT-0026: testMocks struct — always group mocks in a struct

For any handler/service under test with mock dependencies (even a single mock):
- **FORBIDDEN:** helper function returning multiple mocks as separate return values
- **REQUIRED:** define a `testMocks` struct grouping all mocks; create it per-subtest inside `t.Run`

Rationale: the struct is future-proof — adding a new dependency requires only adding a field to the struct, not changing every call site.

### UT-0028: No helper functions for test data, mocks, or assertions

**CRITICAL: Any helper function in test files that creates, configures, or returns test data, mocks, or assertions is STRICTLY FORBIDDEN.**

This includes ALL of the following patterns:
```go
// ❌ ЗАПРЕЩЕНО — data construction helpers
func makeTestSignal() Signal { ... }
func makeTestSignalForSave() Signal { ... }
func newTestSymbol() primitives.Symbol { ... }
func buildTestRow(s Signal) []any { ... }

// ❌ ЗАПРЕЩЕНО — assertion helpers
func assertValidResult(t *testing.T, result Result) { ... }
func signalCopyRow(s Signal) ([]any, error) { ... }
func signalScanValues(s Signal) []any { ... }
func signalTimeframeCode(s Signal) string { ... }

// ❌ ЗАПРЕЩЕНО — mock setup helpers (except testMocks struct per UT-0026)
func setupMocksForHappyPath(m *testMocks) { ... }
```

Each test case MUST construct its own data and mocks inline within the test case definition. No exceptions.

**Exception:** `testMocks` struct + `newTestMocks(t)` for grouping mocks within a single test function is allowed (see UT-0026).

### UT-0029: No package-level shared variables for test data

**CRITICAL: Package-level shared variables, constants, or structures for test data are STRICTLY FORBIDDEN.**

```go
// ❌ ЗАПРЕЩЕНО — shared test data variables
var signalColumnNames = []string{"id", "symbol", "price"}
var testSymbol = primitives.Symbol{Code: "BTCUSDT"}
var defaultTradeID int64 = 42

// ✅ ПРАВИЛЬНО — each test case defines its own values inline
"happy_path": {
    symbol: primitives.Symbol{Code: "BTCUSDT"},
    columnNames: []string{"id", "symbol", "price"},
    tradeID: int64(42),
},
```

Each test must define its own expected values inline to be fully self-contained.

```go
// ✅ CORRECT — testMocks struct
type testMocks struct {
    client  *mocks.MockdeliveryDatesClient
    metrics *mocks.MockmetricsCollector
}

func newTestMocks(t *testing.T) *testMocks {
    t.Helper()
    return &testMocks{
        client:  mocks.NewMockdeliveryDatesClient(t),
        metrics: mocks.NewMockmetricsCollector(t),
    }
}

func newHandlerFromMocks(t *testing.T, m *testMocks) *Handler {
    t.Helper()
    h, err := New(Dependencies{Client: m.client, Metrics: m.metrics})
    require.NoError(t, err)
    return h
}

tests := map[string]struct {
    prepareMocks func(t *testing.T, m *testMocks)
    want         func(t *testing.T, resp *pb.Response, err error)
}{
    "happy_path": {
        prepareMocks: func(t *testing.T, m *testMocks) {
            m.client.EXPECT().Fetch(ctx, req).Return(resp, nil).Once()
            m.metrics.EXPECT().Collect("api", nil, expectedDuration).Once()
        },
    },
}

for name, tc := range tests {
    tc := tc
    t.Run(name, func(t *testing.T) {
        t.Parallel()
        m := newTestMocks(t)
        if tc.prepareMocks != nil {
            tc.prepareMocks(t, m)
        }
        h := newHandlerFromMocks(t, m)
        // ... call + assert
    })
}

// ❌ FORBIDDEN — helper returning multiple mocks
func newTestHandler(t *testing.T) (*Handler, *mocks.MockA, *mocks.MockB, ...) { ... }
```

### UT-0027: No decorative section separators in test files

**FORBIDDEN** — decorative `// ─────` bordered comments as section dividers:
```go
// ─────────────────────────────────────────────────────────────────────────────
// TestHandler_GetSignals
// ─────────────────────────────────────────────────────────────────────────────
```

**ALLOWED** — standard single-line Go doc comment above a non-obvious helper:
```go
// newTestSignal returns a minimal valid Signal for test assertions.
func newTestSignal(...) signal.Signal { ... }
```

Rationale: test functions are self-documenting through their names. Decorative borders add visual noise, duplicate the function name, and create maintenance overhead.

---

## 5. ✅ Assertions

### UT-0030: require for critical, assert for additional

```go
// ✅ require — for critical conditions (stops the test)
require.NoError(t, err)
require.NotNil(t, result)

// ✅ assert — for additional checks (continues the test)
assert.Equal(t, "SENT", result.Status)
assert.True(t, result.IsProcessed)
```

### UT-0031: assert for additional checks (continues test)
Use `assert` when the test can continue after a failed check (non-blocking).

---

## 6. 🎨 Assert Optimization

### UT-0032: No logic in tests — use closures in want

```go
// ✅ One complete test + focused checks
"happy_path": {
    want: func(t *testing.T, result Result, err error) {
        require.NoError(t, err)
        assert.Equal(t, "SENT", result.Status)
        assert.True(t, result.IsProcessed)
        assert.NotEmpty(t, result.ID)
    },
},
"return_error": {
    want: func(t *testing.T, result Result, err error) {
        require.Error(t, err)  // Only app_error check
    },
},
```

### UT-0033: JSON validation with assert.Contains
```go
// ✅ Use assert.Contains for JSON
payloadStr := string(payload)
assert.Contains(t, payloadStr, `"country":"RU"`)
assert.Contains(t, payloadStr, `"city":"Moscow"`)

// ❌ Don't use string values directly
assert.Equal(t, "RU", data.Country)
```

---

## 7. 🚫 What NOT to Do (Anti-patterns)

### Запрет на helper-функции для тестовых данных

**ЗАПРЕЩЕНО** создавать вспомогательные функции для формирования тестовых значений (типа `newTestSymbol()`, `newTestMarkets()`, `makeTestInput()`).
Это плохая практика: затрудняет чтение тестов и может привести к разрастанию вспомогательного кода при изменении данных.

**Исключение:** функции `t.Helper()` для assertions и helper-функции для *операций над результатами* (например, `protoMarketCodes` для конвертации proto) — допустимы.

**ПРАВИЛЬНО:** все входные данные определены непосредственно внутри каждого `test case` (в поле структуры `prepareMocks`):
```go
"happy_path": {
    prepareMocks: func(m *mocks.SymbolListerMock) {
        binanceSpot := primitives.NewMarket("Binance", "binance", primitives.SectionSpot, time.UTC)
        m.AllMock.Times(1).Return([]primitives.Symbol{
            {Market: binanceSpot, Code: "BTCUSDT", Status: primitives.SymbolStatusActive},
        })
    },
    ...
}
```

**НЕПРАВИЛЬНО:**
```go
func newTestSymbolsThreeInTwoMarkets() []primitives.Symbol { ... }  // ЗАПРЕЩЕНО

"happy_path": {
    prepareMocks: func(m *mocks.SymbolListerMock) {
        m.AllMock.Times(1).Return(newTestSymbolsThreeInTwoMarkets())  // ЗАПРЕЩЕНО
    },
}
```

```go
// ❌ Logic in tests (violates UT-0032)
if tc.hasError {
    assert.Error(t, err)
} else {
    assert.NoError(t, err)
}

// ✅ Use closures in want
want: func(t *testing.T, err error) {
    require.Error(t, err)
}

// ❌ Magic Numbers (violates UT-0051)
assert.Equal(t, 3600, timeout)


// ✅ ALWAYS specify expected value in mock (UT-0022)
repo.EXPECT().Find(orderNr).Return(order, nil).Once()

// ❌ ЗАПРЕЩЕНО — mock.Anything для любого параметра (violates UT-0023)
repo.EXPECT().Find(mock.Anything).Return(order, nil).Once()

// ✅ Named constants
const expectedTimeout = 3600
assert.Equal(t, expectedTimeout, timeout)

// ❌ Multiple unrelated asserts
assert.Equal(t, 1, result.A)
assert.Equal(t, 2, result.B)
assert.Equal(t, 3, result.C)

// ✅ Use go-cmp for structs (UT-0070)
if diff := cmp.Diff(expected, actual); diff != "" {
    t.Errorf("Result mismatch (-want +got):\n%s", diff)
}

// ❌ Testing implementation details
assert.Equal(t, 2, len(processor.cache))  // Internal field

// ✅ Test public behavior
result := processor.Get(key)
assert.NotNil(t, result)
```

---

## 8. ⚡ Parallelism

### UT-0040: t.Parallel() at start of main test and each subtest

```go
func TestProcessor_Process(t *testing.T) {
    t.Parallel()  // ✅ For the main test

    for name, tc := range tests {
        tc := tc  // ✅ MUST capture variable (UT-0041)

        t.Run(name, func(t *testing.T) {
            t.Parallel()  // ✅ At the beginning of subtest

            // ✅ New mocks for each test (UT-0024)
            mockRepo := mock.NewMockRepo(t)
            tc.prepareMocks(mockRepo)

            // test...
        })
    }
}
```

### UT-0041: tc := tc variable capture when using t.Parallel()
REQUIRED when iterating over test cases with `t.Parallel()` to prevent data races.

### UT-0042: When NOT to use t.Parallel()
- Tests with shared global variables
- Integration tests with DB
- Tests with file system

---

## 9. 📦 Test Placement (Internal vs External)

### UT-0060: Test placement rules

**Selection Algorithm:**
1. **Unit tests?** → placed in the same package as the tested code
2. **Integration tests (use gonkey package, require database deployment, etc.)** → placed in [tests](../../../tests) package

### UT-0100: One test file per tested struct/function source file

Test files should mirror source file structure. FORBIDDEN to combine multiple unrelated test functions in one file.

**Pattern:**
```go
// Source files in builders/
builders.go               // Contains BuildFoo(), BuildBar()
build_foo_test.go        // Tests for BuildFoo()
build_bar_test.go        // Tests for BuildBar()

// ❌ WRONG
builders_test.go         // Contains TestBuildFoo + TestBuildBar together
```

**Rationale:**
- Improves discoverability (test location is predictable from source)
- Reduces file complexity (fewer tests per file)
- Makes diffs clearer (changes to one builder don't span multiple test functions)
- Aligns with Go conventions (source.go → source_test.go)

---

## 10. 🔧 Advanced Techniques

### UT-0070: go-cmp for complex structures
```go
import "github.com/google/go-cmp/cmp"

want := &Result{Status: "SENT", ID: "123"}
if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(Result{}, "Timestamp")); diff != "" {
    t.Errorf("Result mismatch (-want +got):\n%s", diff)
}
```

### UT-0071: synctest for time-dependent code (Go 1.24+)
```go
import "testing/synctest"

synctest.Run(func() {
    synctest.Wait()  // Wait for goroutines to finish
    result := processor.ProcessWithTimeout(ctx, 5*time.Second)
    assert.NotNil(t, result)
})
```

### Fixture Builders (only for repositories)
```go
// ✅ Only for infrastructure tests with real DB
order := fixtures.NewOrderBuilder().
    WithStatus("NEW").
    WithItems(3).
    Build()
```

---

## 11. 📊 Map vs Slice for Tests

### UT-0080: Use Map for simple scenarios

**Use Map:**
- Simple scenarios
- Readable keys are more important than order
- Quick case lookup

```go
tests := map[string]struct {
    input    string
    expected bool
}{
    "happy path":         {input: "valid", expected: true},
    "empty input":        {input: "", expected: false},
    "invalid format":     {input: "123", expected: false},
}
```

### UT-0081: Use Slice when order matters

**Use Slice:**
- Execution order matters
- Complex data structures
- Many fields in test case

```go
tests := []struct {
    name         string
    args         Args
    prepareMocks func(*mock.MockRepo)
    want         func(*testing.T, Result, error)
}{
    {
        name: "happy path",
        args: Args{orderNr: "ORD123"},
        // ...
    },
}
```

---

## Complete Test Structure Example

```go
// CREATE internal/processors/new_feature_test.go

func TestProcessor_NewFeature(t *testing.T) {
    t.Parallel()  // ✅ UT-0040: Always at the beginning for unit tests

    tests := map[string]struct {
        args         struct {
            orderNr string
            data    *RequestData
        }
        prepareMocks func(*mock.MockRepo, *mock.MockService)
        want         func(*testing.T, *Result, error)
    }{
        "happy_path": {  // ✅ UT-0011: First case is always happy path
            args: struct {
                orderNr string
                data    *RequestData
            }{
                orderNr: "ORD123",
                data:    newTestRequestData(),  // ✅ UT-0050: Private function for complex data
            },
            prepareMocks: func(repo *mock.MockRepo, service *mock.MockService) {
                // ✅ UT-0022: MUST specify .Once(), .Times(0), etc.
                expectedOrder := newTestOrder()
                repo.EXPECT().FindByNR("ORD123").Return(expectedOrder, nil).Once()

                // ✅ UT-0023: Точные значения, mock.Anything ЗАПРЕЩЁН
                service.EXPECT().Create(expectedOrder).Return(nil).Once()
            },
            want: func(t *testing.T, result *Result, err error) {
                require.NoError(t, err)  // ✅ UT-0030: require for critical checks
                assert.Equal(t, "SENT", result.Status)
                assert.NotEmpty(t, result.ID)
            },
        },
        "handle_repo.FindByNR_error_and_return_err": {  // ✅ UT-0012: Standard app_error naming
            args: struct {
                orderNr string
                data    *RequestData
            }{
                orderNr: "ORD999",
                data:    newTestRequestData(),
            },
            prepareMocks: func(repo *mock.MockRepo, service *mock.MockService) {
                repo.EXPECT().FindByNR("ORD999").Return(nil, errors.New("db app_error")).Once()
                // service.Create не вызывается — mockery автоматически зафейлит unexpected call
            },
            want: func(t *testing.T, result *Result, err error) {
                require.Error(t, err)  // ✅ UT-0032: Focused check only for app_error
                assert.Contains(t, err.Error(), "db app_error")
            },
        },
        "handle_service.Create_error_and_return_err": {
            args: struct {
                orderNr string
                data    *RequestData
            }{
                orderNr: "ORD456",
                data:    newTestRequestData(),
            },
            prepareMocks: func(repo *mock.MockRepo, service *mock.MockService) {
                expectedOrder := newTestOrder()
                repo.EXPECT().FindByNR("ORD456").Return(expectedOrder, nil).Once()
                service.EXPECT().Create(expectedOrder).Return(errors.New("service app_error")).Once()
            },
            want: func(t *testing.T, result *Result, err error) {
                require.Error(t, err)
                assert.Contains(t, err.Error(), "service app_error")
            },
        },
    }

    for name, tc := range tests {
        tc := tc  // ✅ UT-0041: MUST capture variable for t.Parallel()

        t.Run(name, func(t *testing.T) {
            t.Parallel()  // ✅ UT-0040: At the beginning of each subtest

            // ✅ UT-0024: New mocks for each test (not shared!)
            mockRepo := mock.NewMockRepo(t)
            mockService := mock.NewMockService(t)
            tc.prepareMocks(mockRepo, mockService)

            processor := NewProcessor(mockRepo, mockService)
            result, err := processor.NewFeature(context.Background(), tc.args.orderNr, tc.args.data)

            tc.want(t, result, err)
        })
    }
}

// ✅ UT-0050: Private functions for creating test data
func newTestRequestData() *RequestData {
    return &RequestData{
        Field1: "value1",
        Field2: 123,
        Items: []Item{
            {SKU: "ITEM1", Quantity: 2},
        },
    }
}

func newTestOrder() *Order {
    return &Order{
        NR:     "ORD123",
        Status: "NEW",
        Items:  []Item{{SKU: "ITEM1", Quantity: 2}},
    }
}
```

**For working with DB transactions:**
```go
prepareMocks: func(db *mock.MockDB, tx *mock.MockTx, repo *mock.MockRepo) {
    // ✅ UT-0025: Transaction mocking pattern
    db.EXPECT().BeginTx(ctx).Return(tx, nil).Once()  // ✅ передаём конкретный ctx

    // Success scenario
    repo.EXPECT().Save(gomock.AssignableToTypeOf(&Order{})).Return(nil).Once()
    tx.EXPECT().Commit().Return(nil).Once()
    tx.EXPECT().Rollback().Times(0)  // ✅ Explicitly verify absence

    // Or app_error scenario
    // tx.EXPECT().Commit().Times(0)
    // tx.EXPECT().Rollback().Return(nil).Once()
}
```

**JSON validation in tests:**
```go
want: func(t *testing.T, payload []byte, err error) {
    require.NoError(t, err)

    // ✅ UT-0033: Use assert.Contains for JSON strings
    payloadStr := string(payload)
    assert.Contains(t, payloadStr, `"country":"RU"`)
    assert.Contains(t, payloadStr, `"city":"Moscow"`)
    assert.Contains(t, payloadStr, `"status":"active"`)
}
```


---

## 12. 🗄️ Repository Layer Tests

### UT-0110: Repository unit tests ОБЯЗАТЕЛЬНЫ

Unit-тесты для repository слоя **ОБЯЗАТЕЛЬНЫ**. Gonkey-интеграционные тесты НЕ заменяют unit-тесты —
они покрывают разные аспекты:

| Аспект | Unit-тесты (mock pool) | Gonkey (интеграция через gRPC) |
|--------|----------------------|-------------------------------|
| **Что тестируют** | Go-код репозитория: маппинг entity ↔ SQL, обработку ошибок, nullable-поля | Полный путь: gRPC handler → usecase → repo → реальная БД |
| **Ошибки БД** | Specific PG errors → domain errors | Не покрыто (фикстуры не провоцируют constraint violations) |
| **Edge cases** | Пустой slice, nil deps, scan ошибки | Только happy path + базовые ошибки |
| **Скорость** | Мгновенно, без инфраструктуры | Нужен запущенный сервис + PG + CH |
| **Гранулярность** | Каждый метод репо изолированно | Тестирует endpoint целиком |

### UT-0111: Мок пула БД через unexported interface

Репозиторий определяет **unexported** интерфейс пула (consumer-side, ES-0051).
Мок генерируется mockery для этого интерфейса.

```go
// В репозитории — unexported interface
type backtestPool interface {
    CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// В тесте — testMocks struct (UT-0026)
type testBacktestMocks struct {
    pool *mocks.MockbacktestPool
}

func newTestBacktestMocks(t *testing.T) *testBacktestMocks {
    t.Helper()
    return &testBacktestMocks{
        pool: mocks.NewMockbacktestPool(t),
    }
}
```

### UT-0112: Обязательные кейсы для repository тестов

Каждый метод репозитория ДОЛЖЕН иметь следующие категории тестов:

**1. happy_path** — успешное выполнение:
```go
// ✅ ПРАВИЛЬНО — проверяем маппинг и возврат
"happy_path": {
    prepareMocks: func(m *testBacktestMocks) {
        m.pool.EXPECT().
            CopyFrom(ctx,
                pgx.Identifier{"backtest_results"},
                []string{"id", "signal_id", "strategy_id", "outcome"},
                mock.MatchedBy(func(src pgx.CopyFromSource) bool { return src != nil })).
            Return(int64(1), nil).
            Once()
    },
    want: func(t *testing.T, err error) {
        require.NoError(t, err)
    },
},
```

**2. edge cases** — граничные условия (пустой вход, nil deps):
```go
// ✅ ПРАВИЛЬНО — пустой slice не вызывает CopyFrom
"return_nil_when_empty_slice": {
    results: func() []strategy.BacktestResult { return []strategy.BacktestResult{} },
    prepareMocks: func(m *testBacktestMocks) {
        // CopyFrom must not be called for empty input
    },
    want: func(t *testing.T, err error) {
        require.NoError(t, err)
    },
},

// ✅ ПРАВИЛЬНО — nil pool в конструкторе
"return_error_when_pool_is_nil": {
    deps: func(m *testBacktestMocks) strategy.BacktestResultDeps {
        return strategy.BacktestResultDeps{Pool: nil}
    },
    want: func(t *testing.T, repo *strategy.BacktestResultRepo, err error) {
        require.Error(t, err)
        assert.Nil(t, repo)
        assert.Contains(t, err.Error(), "pool is required")
    },
},
```

**3. PG-ошибки → domain errors** — маппинг ошибок БД в доменные:
```go
// ✅ ПРАВИЛЬНО — unique violation → ErrBacktestFailed
"return_ErrBacktestFailed_on_unique_violation": {
    prepareMocks: func(m *testBacktestMocks) {
        m.pool.EXPECT().
            CopyFrom(ctx, pgx.Identifier{"backtest_results"},
                []string{"id", "signal_id", "strategy_id", "outcome"},
                mock.MatchedBy(func(src pgx.CopyFromSource) bool { return src != nil })).
            Return(int64(0), &pgconn.PgError{Code: "23505"}).
            Once()
    },
    want: func(t *testing.T, err error) {
        require.Error(t, err)
        assert.ErrorIs(t, err, apperrors.ErrBacktestFailed)
    },
},
```

**4. ошибки инфраструктуры** — сбой Query/Exec/CopyFrom:
```go
// ✅ ПРАВИЛЬНО — ошибка копирования
"return_error_when_copy_fails": {
    prepareMocks: func(m *testBacktestMocks) {
        m.pool.EXPECT().
            CopyFrom(ctx, pgx.Identifier{"backtest_results"},
                []string{"id", "signal_id", "strategy_id", "outcome"},
                mock.MatchedBy(func(src pgx.CopyFromSource) bool { return src != nil })).
            Return(int64(0), errors.New("copy failed")).
            Once()
    },
    want: func(t *testing.T, err error) {
        require.Error(t, err)
        assert.Contains(t, err.Error(), "save backtest results")
    },
},
```

### UT-0113: Анти-паттерны в repository тестах

```go
// ❌ ЗАПРЕЩЕНО — тест без проверки конкретных колонок (теряется маппинг-контроль)
m.pool.EXPECT().
    CopyFrom(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
    Return(int64(1), nil).Once()

// ❌ ЗАПРЕЩЕНО — не тестировать конструктор
// Если конструктор валидирует deps — для него ОБЯЗАТЕЛЕН отдельный TestNew<Repo>

// ❌ ЗАПРЕЩЕНО — пропускать тест на пустой вход
// Если метод принимает slice — ОБЯЗАТЕЛЕН кейс с пустым slice

// ❌ ЗАПРЕЩЕНО — не тестировать маппинг PG-ошибок
// Если repo оборачивает PgError в domain error — ОБЯЗАТЕЛЕН кейс с &pgconn.PgError{Code: "XXXXX"}
```

### UT-0114: fakeRows/fakeRow для Query/QueryRow тестов

Для методов, использующих `Query` (возвращает `pgx.Rows`), необходимо реализовать `fakeRows`
в `shared_test.go` пакета. Для `QueryRow` — `fakeRow`, реализующий `pgx.Row`.

```go
// shared_test.go — переиспользуется всеми repo тестами в пакете
type fakeRows struct {
    rows    [][]any
    current int
    closed  bool
}

func (f *fakeRows) Next() bool { ... }
func (f *fakeRows) Scan(dest ...any) error { ... }
func (f *fakeRows) Close() { f.closed = true }
func (f *fakeRows) Err() error { return nil }
// ... остальные методы pgx.Rows

type fakeRow struct {
    values []any
    err    error
}

func (f *fakeRow) Scan(dest ...any) error { ... }
```

---

**🎯 Main rule:** Tests should be clear, isolated, and fast. One test = one check = one intent.
