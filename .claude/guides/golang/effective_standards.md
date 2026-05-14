# Effective Go Standards

This document contains comprehensive standards for effective Go programming. Each rule has a unique identifier (ES-XXXX) for reference in code reviews and discussions.

> **Sources**: [go.dev/doc/effective_go](https://go.dev/doc/effective_go), [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)

> **Related**: For concurrency-specific rules, see [effective_concurrency.md](./effective_concurrency.md)

---

## 1. Code Formatting

### ES-0001: Use gofmt for automatic formatting
- **Use `gofmt`** for automatic code formatting
- The tool ensures consistent style across the entire project
- Don't work around formatting - refactor your code if necessary

### ES-0002: Use tabs for indentation
- **Use tabs** for indentation (gofmt default)
- No strict line length limit, but prefer max 120 characters
- Break long lines into multiple lines with additional tab indentation

### ES-0003: Brace placement on same line
```go
// ES-0003: CORRECT - brace on same line
if x > 0 {
    return true
}

// ES-0003: WRONG - brace on new line triggers automatic semicolon insertion
if x > 0
{
    return true
}
```

---

## 2. Naming Conventions

### ES-0010: Package names - lowercase, short, no underscores
```go
// ES-0010: CORRECT
package time
package fmt
package bufio
package orderprocessor

// ES-0010: WRONG - underscores
package order_processor

// ES-0010: WRONG - MixedCaps
package orderProcessor

// ES-0010: WRONG - plural form
package users

// ES-0010: WRONG - generic names
package util
package common
package helpers
```

**Rules:**
- Use **only lowercase** letters (no capitals, no underscores)
- Use **singular form**: `user`, not `users`
- Avoid **generic terms**: `util`, `common`, `shared`, `lib`, `helpers`
- Name should be **short and concise**

### ES-0011: Exported vs Unexported identifiers
- **Capital first letter** = exported (visible outside package)
- **Lowercase first letter** = unexported (private to package)

```go
// ES-0011: Exported
type Reader interface { ... }
func (r *Reader) Read(p []byte) (n int, err error) { ... }

// ES-0011: Unexported
type reader struct { ... }
func (r *reader) read(p []byte) (n int, err error) { ... }
```

### ES-0012: Getter naming - omit "Get" prefix
```go
// ES-0012: CORRECT
func (o *Owner) Owner() string { ... }

// ES-0012: WRONG
func (o *Owner) GetOwner() string { ... }
```

### ES-0013: Setter naming - use "Set" prefix
```go
// ES-0013: CORRECT
func (o *Owner) SetOwner(name string) { ... }
```

### ES-0014: One-method interfaces - use method name + "-er" suffix
```go
// ES-0014: CORRECT
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}
```

### ES-0015: Use MixedCaps for multi-word identifiers
```go
// ES-0015: CORRECT
var httpServerPort int
func isNumeric(c byte) bool { ... }

// ES-0015: WRONG - underscores
var http_server_port int
func is_numeric(c byte) bool { ... }
```

### ES-0016: Function naming - use MixedCaps
```go
// ES-0016: CORRECT - MixedCaps
func ProcessOrder() {}
func getUserByID() {}

// ES-0016: WRONG - snake_case
func process_order() {}
```

**Exception:** Test functions may contain underscores for grouping:
```go
func TestMyFunction_WhatIsBeingTested(t *testing.T) {}
```

### ES-0017: Boolean naming - use is/has/can/should prefixes
```go
// ES-0017: CORRECT
var isEnabled bool
var hasError bool
var canProcess bool
var shouldRetry bool
```

### ES-0018: Avoid generic function names
**Bad function names**: `process`, `handle`, `do`, `perform`, `change`, `filter`, `check`, `validate`

Be specific about what the function does.

---

## 3. Control Structures

### ES-0020: Initialization in conditions
```go
// ES-0020: GOOD - initialization in condition
if err := os.Open("file.txt"); err != nil {
    return err
}

if x := compute(); x > 0 {
    return x
}
```

### ES-0021: Return errors early (guard clauses)
```go
// ES-0021: CORRECT - early return on app_error
func process() error {
    f, err := os.Open("file.txt")
    if err != nil {
        return err
    }
    defer f.Close()

    data, err := ioutil.ReadAll(f)
    if err != nil {
        return err
    }

    // successful logic here
    return nil
}

// ES-0021: WRONG - nested conditions
func process() error {
    f, err := os.Open("file.txt")
    if err == nil {
        defer f.Close()
        data, err := ioutil.ReadAll(f)
        if err == nil {
            // successful logic here
            return nil
        } else {
            return err
        }
    } else {
        return err
    }
}
```

### ES-0022: Minimize scope of variables
```go
// ES-0022: CORRECT - narrow scope
if err := doSomething(); err != nil {
    return err
}

// ES-0022: LESS GOOD - wider scope
var err error
err = doSomething()
if err != nil {
    return err
}
```

---

## 4. Data Structures

### ES-0030: Prefer slices over arrays
- Arrays have **fixed size** in the type: `[10]int` and `[20]int` are different types
- Arrays are **copied entirely** on assignment
- **Prefer slices** for most sequence operations

```go
// ES-0030: Prefer this
var s []int
s = append(s, 1)

// ES-0030: Rarely needed
var a [10]int
```

### ES-0031: Design zero values to be immediately usable
```go
// ES-0031: GOOD - zero value is ready to use
var m sync.Mutex
m.Lock()  // works without initialization

var buf bytes.Buffer
fmt.Fprintf(&buf, "hello")  // works directly
```

### ES-0032: Use composite literals with field names
```go
// ES-0032: CORRECT - explicit field names
p := Point{x: 0, y: 0}
p := Point{y: 1, x: 2}  // can be in any order

// ES-0032: LESS GOOD - no names (brittle to struct changes)
p := Point{0, 0}
```

### ES-0033: Pre-allocate slices when size is known
```go
// ES-0033: GOOD - pre-allocate if you know size
items := make([]string, 0, 100)
for i := 0; i < 100; i++ {
    items = append(items, getItem(i))
}

// ES-0033: LESS EFFICIENT - grows dynamically
var items []string
for i := 0; i < 100; i++ {
    items = append(items, getItem(i))
}
```

### ES-0034: Pre-allocate maps when size is known
```go
// ES-0034: GOOD - pre-allocate if you know size
items := make(map[int]Item, 100)

// ES-0034: LESS EFFICIENT - grows dynamically
var items map[int]Item
```

### ES-0035: Defensive copying for slices/maps at boundaries

**When receiving slice/map as argument:**
```go
// ES-0035: WRONG - storing reference to external slice
func (s *Store) SetItems(items []Item) {
    s.items = items  // caller can modify!
}

// ES-0035: CORRECT - create a copy
func (s *Store) SetItems(items []Item) {
    s.items = make([]Item, len(items))
    copy(s.items, items)
}
```

**When returning internal slice/map:**
```go
// ES-0035: WRONG - returning internal slice
func (s *Store) Items() []Item {
    return s.items  // caller can modify!
}

// ES-0035: CORRECT - return a copy
func (s *Store) Items() []Item {
    result := make([]Item, len(s.items))
    copy(result, s.items)
    return result
}
```

---

## 5. Function Design

### ES-0040: Return value and error
```go
// ES-0040: CORRECT - value and app_error
func Read(b []byte) (n int, err error) { ... }

// Usage
n, err := Read(buf)
if err != nil {
    return err
}
```

### ES-0041: Named return parameters for documentation
```go
// ES-0041: GOOD - named return parameters
func Read(b []byte) (n int, err error) {
    n = len(b)
    err = nil
    return
}
```

**Advantages**:
- Documents what is returned
- Return values initialize to zero values
- Allows `bare return` statement

### ES-0042: Use defer for guaranteed resource cleanup
```go
// ES-0042: CORRECT - guaranteed closing
func process(filename string) error {
    f, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer f.Close()

    // file operations
    return nil
}
```

### ES-0043: Avoid defer inside loops
```go
// ES-0043: WRONG - defers accumulate
for _, file := range files {
    f, _ := os.Open(file)
    defer f.Close()  // not released until function returns!
}

// ES-0043: CORRECT - wrap in helper function
for _, file := range files {
    processFile(file)
}

func processFile(file string) {
    f, _ := os.Open(file)
    defer f.Close()
    // ...
}
```

### ES-0044: Functions returning more than 3 values need refactoring
Consider using a struct or splitting the function.

### ES-0045: No optional parameters via zero values — use separate methods
Do not use zero/empty values (`""`, `0`, `nil`) as "no filter" or "use default" markers. This creates implicit contracts that are error-prone and hard to read.

Instead, create **separate methods** with explicit names.

```go
// ES-0045: BAD — empty string means "all", non-empty means "filter"
func (c *Catalog) Symbols(marketCode string) []Symbol {
    if marketCode == "" {
        return c.allSymbols()
    }
    return c.symbolsByMarket(marketCode)
}

// ES-0045: BAD — nil pointer as optional flag
func (c *Catalog) Symbols(marketCode *string) []Symbol { ... }
```

```go
// ES-0045: GOOD — two explicit methods
func (c *Catalog) AllSymbols() []Symbol { ... }
func (c *Catalog) SymbolsByMarket(marketCode string) []Symbol { ... }
```

**Why:**
- Caller intent is unambiguous — `AllSymbols()` vs `SymbolsByMarket("binance")`
- No hidden contract ("pass empty string to get all")
- Each method has a single responsibility
- Easier to test, mock, and document

**Exception:** Functional options pattern (ES-0120) is allowed for constructor configuration, not for query/filter methods.

> **Note**: For function length limits (40+/80+ lines), see [coding_style.md](./coding_style.md#cs-0082-function-length-rules)

---

## 6. Interface Design

### ES-0050: Keep interfaces small and focused
```go
// ES-0050: GOOD - minimal interface
type Reader interface {
    Read(p []byte) (n int, err error)
}

// ES-0050: BAD - interface too large
type Everything interface {
    Read(p []byte) (int, error)
    Write(p []byte) (int, error)
    Close() error
    Seek(offset int64, whence int) (int64, error)
    // ... many more methods
}
```

### ES-0051: Define interfaces at consumer, not producer
```go
// ES-0051: GOOD - interface defined where used (consumer)
package orderprocessor

type Repository interface {
    GetOrder(ctx context.Context, id OrderID) (*Order, error)
}

type Processor struct {
    repo Repository
}

// ES-0051: BAD - interface at producer (repository package)
package repository

type OrderRepository interface {  // Don't do this!
    GetOrder(ctx context.Context, id OrderID) (*Order, error)
    CreateOrder(ctx context.Context, order *Order) error
    // ... many methods
}
```

### ES-0052: Use type assertions with comma-ok
```go
// ES-0052: CORRECT - comma ok idiom
value, ok := interfaceVal.(Type)
if !ok {
    return errors.New("expected string")
}

// ES-0052: WRONG - panics if type is wrong
value := interfaceVal.(Type)  // panics if wrong type!
```

### ES-0053: Verify interface implementation at compile time
```go
// ES-0053: Compile-time check
var _ io.Reader = (*MyReader)(nil)
var _ io.Writer = (*MyWriter)(nil)
```

### ES-0054: Don't use pointers to interfaces
```go
// ES-0054: WRONG - pointer to interface
func process(r *io.Reader) { }

// ES-0054: CORRECT - interface by value
func process(r io.Reader) { }
```

### ES-0055: No generalized interfaces in provider packages

**🔴 АБСОЛЮТНЫЙ ЗАПРЕТ: Обобщённые интерфейсы в provider packages ЗАПРЕЩЕНЫ.**

Это правило НЕ имеет альтернатив и обходных путей. Единственное допустимое решение - удаление интерфейсов из provider package и определение их в каждом consumer.

Interfaces must be defined ONLY in the package that USES them (consumer), not in the package that IMPLEMENTS them (provider).

```go
// ES-0055: BAD - generalized interfaces in provider package
package transport

// DON'T DO THIS - these interfaces should be defined at each consumer!
type Publisher interface {
    Publish(ctx context.Context, topic string, body []byte) error
}

type Consumer interface {
    Subscribe(topic, channel string) error
    Messages() <-chan *Message
}

type Transport interface {
    Publisher() Publisher
    Consumer() Consumer
}
```

```go
// ES-0055: GOOD - interface defined in consumer package
package producer

// Consumer defines only the methods THIS package needs
type transport interface {
    Publish(ctx context.Context, topic string, body []byte) error
}

type Producer struct {
    transport transport  // uses local interface
}
```

```go
// ES-0055: GOOD - another consumer defines its own interface
package consumer

// Consumer defines only the methods IT needs
type transport interface {
    Subscribe(topic, channel string) error
    Messages() <-chan *Message
}

type Consumer struct {
    transport transport
}
```

**Why this matters:**
- Generalized interfaces couple consumers to implementation details
- Each consumer should define minimal interface with only methods it needs
- Makes testing easier (smaller mock interfaces)
- Follows Interface Segregation Principle
- Allows independent evolution of consumers

**🚫 НЕДОПУСТИМЫЕ "АЛЬТЕРНАТИВЫ" (НЕ предлагать в code review):**
- ❌ "Переименовать пакет в `*interfaces`" - это НЕ решает проблему
- ❌ "Переместить в `internal/`" - это НЕ решает проблему
- ❌ "Добавить комментарий о назначении" - это НЕ решает проблему
- ❌ "Оставить интерфейсы, они нужны для..." - НЕТ исключений (кроме factory pattern, см. ниже)

**✅ ЕДИНСТВЕННОЕ ДОПУСТИМОЕ РЕШЕНИЕ:**
1. Удалить обобщённые интерфейсы из provider package
2. Каждый consumer определяет свой минимальный интерфейс
3. Provider package экспортирует только конкретные типы (structs)

**Допустимые исключения из правила:**
- `io.Reader`, `io.Writer` — использование интерфейсов стандартной библиотеки допустимо
- **Factory pattern** — интерфейс, возвращаемый фабричным методом, допустим при выполнении ВСЕХ условий:
  1. Пакет содержит фабрику (constructor/factory), которая выбирает реализацию по параметрам
  2. Существует (или планируется) более одной реализации интерфейса в этом пакете
  3. Интерфейс минимален (1–3 метода) и описывает контракт возвращаемого значения
  4. Consumer не импортирует интерфейс для объявления полей — он получает его как возвращаемое значение фабрики

```go
// ES-0055: VALID — factory returns interface for polymorphic dispatch
package csv

type TradeParser interface {
    Next(ctx context.Context) (entity.RawTrade, error)
    Close() error
}

type ParserFactory struct { ... }

func (f *ParserFactory) Create(r io.Reader, symbol primitives.Symbol) (TradeParser, error) {
    switch symbol.Market.Code {
    case MarketBinance:
        return NewBinanceSpotTradeParser(r, symbol, f.log)
    default:
        return nil, fmt.Errorf("unsupported market %q", symbol.Market.Code)
    }
}
```

### ES-0056: Prefer unexported interfaces

**🔴 Экспортированный интерфейс — это публичный контракт, который нарушает ES-0051.**

Consumer сам определяет интерфейс для своих нужд — чаще всего он unexported и минимален.

```go
// ES-0056: BAD - exported interface in consumer package
package orderprocessor

type Repository interface {           // Exported — навязывает контракт другим consumers!
    GetOrder(ctx context.Context, id OrderID) (*Order, error)
}

type Processor struct {
    repo Repository
}
```

```go
// ES-0056: GOOD - unexported interface in consumer package
package orderprocessor

type repository interface {           // unexported — контракт только для этого пакета
    GetOrder(ctx context.Context, id OrderID) (*Order, error)
}

type Processor struct {
    repo repository
}
```

**Когда экспорт интерфейса допустим:**
- Интерфейс является частью публичного API пакета (например, `http.Handler`, `io.Reader`)
- Пакет предоставляет абстракцию для внешних потребителей (библиотека, SDK)
- Интерфейс используется как параметр публичной функции

**Правила:**
- Если интерфейс используется только внутри пакета — он **обязан** быть unexported
- Exported interface требует явного обоснования в комментарии

### ES-0057: Interface placement within a package

Интерфейс объявляется в **одном из двух допустимых мест** внутри пакета:

**Вариант 1: Непосредственно перед конструктором** (предпочтительный — если конструктор принимает **не более 3** интерфейсных параметров)

```go
// ES-0057: GOOD — interface directly before the constructor that takes it

// repository defines the data access methods needed by this package.
type repository interface {
    GetOrder(ctx context.Context, id OrderID) (*Order, error)
}

func New(repo repository) *Processor {
    return &Processor{repo: repo}
}
```

**Вариант 2: Файл `interfaces.go`** (если конструктор принимает **4 и более** интерфейсных параметров)

```go
// interfaces.go
package orderprocessor

type repository interface {
    GetOrder(ctx context.Context, id OrderID) (*Order, error)
}

type eventPublisher interface {
    Publish(ctx context.Context, event Event) error
}

type cache interface {
    Get(ctx context.Context, key string) (*Order, bool)
    Set(ctx context.Context, key string, order *Order)
}
```

**Запрещено:**
- Определять интерфейс в середине файла, вдали от конструктора
- Определять интерфейс в отдельном файле с произвольным именем (например, `deps.go`, `types.go`)
- Смешивать оба варианта в одном пакете

---

## 7. Error Handling

### ES-0060: Handle errors explicitly
- Never ignore errors silently
- No empty lines before `if err != nil`
- **NEVER** use blank identifier to discard errors from function/method calls
- `_ = expr` where `expr` returns an error is **FORBIDDEN** — no exceptions. Every error must be handled: returned, logged, or joined with the primary error via `errors.Join`. In `defer` cleanup, use `errors.Join` to combine cleanup errors with the primary error. The `//nolint:errcheck` exception is also removed per ES-0171 (nolint annotations are forbidden).

```go
// ES-0060: CORRECT
result, err := operation()
if err != nil {
    return err
}

// ES-0060: CORRECT - handle app_error from function call
if err := queue.Enqueue(job); err != nil {
    return fmt.Errorf("enqueue job: %w", err)
}

// ES-0060: CORRECT - log app_error if can't return
if err := cleanup(); err != nil {
    logger.Error(ctx, "cleanup failed", "app_error", err)
}
```

```go
// ES-0060: BAD - assigning app_error to blank identifier
err := doSomething()
_ = err  // silently ignoring!

// ES-0060: BAD - discarding app_error from function call
_ = doSomething()
_ = queue.Enqueue(job)
_ = obj.Method()

// ES-0060: BAD - discarding app_error in closure
_ = queue.Enqueue(func() error {
    return someOperation()  // app_error from Enqueue itself is lost!
})
```

### ES-0061: Wrap errors with context
```go
// ES-0061: CORRECT - includes context
return fmt.Errorf("loading config from %s: %w", path, err)

// ES-0061: BAD - redundant words
return fmt.Errorf("failed to load config: %w", err)
```

### ES-0061a: Context-aware errors via WrapError / UnwrapErrorCtx

**Precondition:** applies only if the project uses `github.com/webdelo/go-ctx-logger`. Check for `ctxlogger` import in `go.mod` before applying.

When ctxlogger is used, context is enriched with structured fields as it passes through layers (operation, symbol, file, line, etc.). To propagate these fields to the logging point, every returned error **must** be wrapped with `ctxlogger.WrapError(ctx, err)`.

**Two rules:**
1. **Returning errors**: if `ctx` is available — ALWAYS use `ctxlogger.WrapError(ctx, fmt.Errorf("...: %w", err))`. No exceptions.
2. **Logging errors**: at the top level, enrich context via `ctxlogger.UnwrapErrorCtx(ctx, err)` and pass enriched context to the logger. Signature: `UnwrapErrorCtx(ctx context.Context, err error) context.Context`.

```go
// ES-0061a: CORRECT — return error with context
if err != nil {
    return ctxlogger.WrapError(ctx, fmt.Errorf("read csv line: %w", err))
}

// ES-0061a: CORRECT — add value-specific fields before wrapping
if err != nil {
    return entity.RawTrade{}, ctxlogger.WrapError(
        ctxlogger.WithField(ctx, zap.String("value", record[colPrice])),
        fmt.Errorf("invalid price: %w", err),
    )
}

// ES-0061a: CORRECT — log error with enriched context (fields extracted from error chain)
logger.Warn(ctxlogger.UnwrapErrorCtx(ctx, err), "skipping invalid trade row", zap.Error(err))

// ES-0061a: WRONG — plain fmt.Errorf without wrapping (context lost)
return fmt.Errorf("invalid price: %w", err)

// ES-0061a: WRONG — logging with original ctx instead of enriched (wrapped fields lost)
logger.Error(ctx, "operation failed", zap.Error(err))

// ES-0061a: WRONG — log.Warn + plain fmt.Errorf (data not attached to error)
p.log.Warn(ctx, "invalid price", zap.String("value", val), zap.Error(err))
return fmt.Errorf("invalid price: %w", err)

// ES-0061a: WRONG — embedding values into error string via %d/%q
return fmt.Errorf("invalid price %q at line %d: %w", val, line, err)
```

### ES-0062: Use sentinel errors for detection
```go
// ES-0062: CORRECT - package-level var for sentinel errors
var ErrNotFound = errors.New("not found")
var ErrInvalidInput = errors.New("invalid input")

// Usage
if errors.Is(err, ErrNotFound) {
    // handle not found
}
```

### ES-0063: No panic in production code

This includes factory functions and constructors. If initialization can fail, return `(T, error)` — never `panic`. The caller decides whether the error is fatal (e.g., `log.Fatal`).

```go
// ES-0063: WRONG
func processInput(input string) {
    if input == "" {
        panic("empty input")
    }
}

// ES-0063: CORRECT
func processInput(input string) error {
    if input == "" {
        return errors.New("input is empty")
    }
    // ...
}
```

### ES-0064: Use recover only for server crash prevention
```go
// ES-0064: CORRECT - recover in handler
func handleRequest(w http.ResponseWriter, r *http.Request) {
    defer func() {
        if err := recover(); err != nil {
            log.Printf("panic: %v", err)
            http.Error(w, "Internal Error", 500)
        }
    }()
    // request handling
}
```

### ES-0065: Return empty result + nil for "not found"
For "not found" conditions, return **empty result + nil error** instead of `NotFound` error.

---

## 8. Methods and Receivers

### ES-0070: Consistent receiver types for all methods
**ALL methods of one type must have the same receiver type.**

```go
// ES-0070: GOOD - all methods with pointer receiver
func (s *Service) Start() error { }
func (s *Service) Stop() error { }
func (s *Service) Status() string { }

// ES-0070: BAD - mixed receivers
func (s *Service) Start() error { }
func (s Service) Status() string { }  // inconsistent!
```

### ES-0071: Pointer receiver for modifications
```go
// ES-0071: Use pointer receiver if method modifies receiver
func (p *Point) Move(dx, dy int) {
    p.x += dx
    p.y += dy
}
```

### ES-0072: Value receiver for read-only
```go
// ES-0072: Value receiver for immutable operations
func (p Point) Add(q Point) Point {
    return Point{p.x + q.x, p.y + q.y}
}
```

### ES-0073: Receiver type choice guidelines

| Scenario | Receiver |
|----------|----------|
| Method modifies receiver | `*T` (pointer) |
| Large struct (>= 3-5 fields) | `*T` (pointer) |
| Method only reads data | `T` (value) |
| Small immutable type | `T` (value) |

---

## 9. Documentation and Comments

### ES-0080: Doc comments precede declaration
```go
// ES-0080: CORRECT - doc comment for function
// NewReader creates a new reader for the given file.
func NewReader(filename string) (*Reader, error) { ... }

// ES-0080: WRONG - blank line before comment breaks the association
type MyType struct { ... }

// this is not a doc comment

func SomeFunc() {}
```

### ES-0081: Start doc comments with declaration name
```go
// ES-0081: CORRECT
// Reader implements the io.Reader interface.
type Reader struct { ... }

// NewReader creates a new Reader.
func NewReader() *Reader { ... }
```

---

## 10. Enums

### ES-0090: Start enums with 1, not 0
```go
// ES-0090: GOOD - start with 1
type Status int

const (
    StatusPending Status = iota + 1  // 1
    StatusActive                     // 2
    StatusDone                       // 3
)

// ES-0090: BAD - 0 can be confused with uninitialized value
type Status int

const (
    StatusPending Status = iota  // 0 - confuses with zero value
    StatusActive                 // 1
)
```

**Exception**: When 0 has a semantic meaning (e.g., `StatusUnknown`).

---

## 11. Context Usage

> **Moved**: Context rules are now in [effective_concurrency.md](./effective_concurrency.md#6-context-propagation) (Section 6: EC-0050 - EC-0054).
>
> Context is primarily a concurrency mechanism for cancellation propagation and deadline management.

---

## 12. Performance

### ES-0110: Use strconv instead of fmt for conversions
```go
// ES-0110: GOOD - faster
s := strconv.Itoa(123)
n, _ := strconv.Atoi("123")

// ES-0110: SLOWER
s := fmt.Sprintf("%d", 123)
```

### ES-0111: Avoid repeated string <-> []byte conversion
```go
// ES-0111: BAD - conversion on each iteration
for i := 0; i < b.N; i++ {
    _ = string(data)
}

// ES-0111: GOOD - convert once
s := string(data)
for i := 0; i < b.N; i++ {
    _ = s
}
```

### ES-0112: Use decimal for financial calculations
```go
import "github.com/shopspring/decimal"

// ES-0112: CORRECT - for precise monetary calculations
amount := decimal.NewFromString("0.1")
price := decimal.NewFromString("0.2")
total := amount.Add(price)

// ES-0112: WRONG - floats have rounding errors
amount := 0.1
price := 0.2
total := amount + price  // may be 0.30000000000000004
```

---

## 13. Functional Options Pattern

### ES-0120: Use functional options for many optional parameters
```go
// ES-0120: Functional options pattern
type Option func(*Server)

func WithPort(port int) Option {
    return func(s *Server) {
        s.port = port
    }
}

func WithTimeout(timeout time.Duration) Option {
    return func(s *Server) {
        s.timeout = timeout
    }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        port:    8080,            // default
        timeout: 30 * time.Second, // default
    }

    for _, opt := range opts {
        opt(s)
    }

    return s
}

// Usage
server := NewServer(
    WithPort(9000),
    WithTimeout(time.Minute),
)
```

---

## 14. Import Organization

### ES-0130: Import order - standard, external, internal
```go
// ES-0130: CORRECT grouping
import (
    "fmt"
    "os"

    "github.com/pkg/errors"

    "myproject/internal/config"
)
```

### ES-0131: Use import aliases only when necessary
```go
// ES-0131: GOOD - alias needed (name differs from path)
import (
    client "example.com/client-go"
)

// ES-0131: BAD - unnecessary alias
import (
    ctx "context"  // unnecessary!
)
```

---

## 15. Constructors and Dependencies

### ES-0140: Constructor name is "New"
```go
// ES-0140: CORRECT
type Handler struct {}

func New() (*Handler, error)

// ES-0140: WRONG
func NewHandler() (*Handler, error)
```

### ES-0141: Use Dependencies struct for 3+ dependencies
```go
// ES-0141: CORRECT
type Dependencies struct {
    Handler Handler
    Logger  Logger
}

func New(cfg Config, deps Dependencies) (*Service, error) {
    if err := validateConfig(cfg); err != nil {
        return nil, err
    }
    if err := validateDependencies(deps); err != nil {
        return nil, err
    }
    return &Service{config: cfg, deps: deps}, nil
}
```

### ES-0142: Validate dependencies in constructor
All dependencies and configuration must be validated in constructor. Checking in methods is prohibited.

### ES-0143: Use noop defaults for optional dependencies instead of nil-checks
Optional dependencies must be replaced with a noop implementation in the constructor. Nil-checks at every call site are prohibited.

```go
// ES-0143: CORRECT - noop default assigned in constructor
type defaultMetricsRecorder struct{}

func (defaultMetricsRecorder) RecordEvent(name string) {}

func New(deps Dependencies) *Service {
    if deps.Metrics == nil {
        deps.Metrics = defaultMetricsRecorder{}
    }
    return &Service{deps: deps}
}

// Call sites are clean - no nil-checks needed
func (s *Service) Process() {
    s.deps.Metrics.RecordEvent("processed")
}
```

```go
// ES-0143: WRONG - nil-checks scattered across call sites
func (s *Service) Process() {
    if s.deps.Metrics != nil {
        s.deps.Metrics.RecordEvent("processed")
    }
}

func (s *Service) Complete() {
    if s.deps.Metrics != nil {
        s.deps.Metrics.RecordEvent("completed")
    }
}
```

**Rules:**
- Noop type must be **unexported** (`defaultMetricsRecorder`, not `DefaultMetricsRecorder`)
- Use **value receiver** on noop type (no state to mutate)
- Assignment happens in constructor, **after** `checkDependencies()` validation
- DI tags (e.g. Fx `optional:"true"`) remain unchanged — nil is allowed at DI level, noop is assigned in domain constructor
- Tests that construct structs directly (bypassing constructor) must set the noop default explicitly

---

## 16. Standard Library Collections

### ES-0150: Use maps and slices packages to simplify collection operations

Prefer using the `golang.org/x/exp/maps`, `golang.org/x/exp/slices`, and native `maps`/`slices` packages (Go 1.21+) over manual loops for common collection operations.

**Rules:**
- Use `maps.Keys(m)` to extract map keys instead of a manual loop + append
- Use `maps.Values(m)` to extract map values instead of a manual loop + append
- Use `maps.Clear(m)` to clear a map instead of re-assigning `make`
- Use `slices.Contains(s, v)` instead of a manual search loop
- Use `slices.Index(s, v)` to find element index
- **Do not pre-allocate a map** with the count of source elements when the expected result is much smaller (e.g., deduplicating markets from thousands of symbols — use `make(map[K]V)` with no hint)

**Examples:**

```go
// ES-0150: CORRECT — use maps.Values() for extraction
func uniqueMarkets(symbols []primitives.Symbol) []primitives.Market {
    markets := make(map[primitives.MarketCode]primitives.Market)
    for i := range symbols {
        markets[symbols[i].Market.Code] = symbols[i].Market
    }
    return maps.Values(markets) // golang.org/x/exp/maps
}

// ES-0150: CORRECT — use maps.Keys() for key extraction
func uniqueSections(symbols []primitives.Symbol) []primitives.Section {
    seen := make(map[primitives.Section]struct{})
    for i := range symbols {
        seen[symbols[i].Market.Section] = struct{}{}
    }
    return maps.Keys(seen) // golang.org/x/exp/maps
}

// ES-0150: WRONG — manual accumulation with parallel seen+result structures
func uniqueMarketsBad(symbols []primitives.Symbol) []primitives.Market {
    seen := make(map[primitives.MarketCode]struct{}, len(symbols)) // wrong: over-allocates
    markets := make([]primitives.Market, 0, len(symbols))          // parallel slice — not needed
    for i := range symbols {
        if _, exists := seen[symbols[i].Market.Code]; !exists {
            seen[symbols[i].Market.Code] = struct{}{}
            markets = append(markets, symbols[i].Market)
        }
    }
    return markets
}
```

**Import:** For Go < 1.21 compatibility use `golang.org/x/exp/maps` and `golang.org/x/exp/slices`. For Go 1.21+ projects the native `maps` and `slices` packages are available.

---

## 17. Domain Entity и Database Model — разделение ответственности

### ES-0160: Domain entity не повторяет структуру таблицы БД

Entity отражает бизнес-логику, а не схему хранения. Поля entity — это доменные типы и value objects (`primitives.Symbol`, `assetquantity.AssetQuantity`), а не колонки БД (`string`, `int64`, `uint8`). Entity **не содержит** тегов маппинга (`ch:`, `db:`, `json:`).

Правило распространяется на все базы данных: ClickHouse, PostgreSQL, Redis, MongoDB и др.

```go
// ES-0160: WRONG — entity повторяет структуру таблицы, содержит db-теги
type RawTrade struct {
    TradeID           int64     `ch:"trade_id"`
    Market            string    `ch:"market"`
    Symbol            string    `ch:"symbol"`
    Section           string    `ch:"section"`
    Price             int64     `ch:"price"`
    Quantity          int64     `ch:"quantity"`
    PricePrecision    uint8     `ch:"price_precision"`
    QuantityPrecision uint8     `ch:"quantity_precision"`
    TransactTime      time.Time `ch:"transact_time"`
    IsBuyerMaker      bool      `ch:"is_buyer_maker"`
}
```

```go
// ES-0160: CORRECT — entity использует доменные типы и value objects
type RawTrade struct {
    TradeID      int64
    Symbol       primitives.Symbol              // содержит Market, Section, Pair
    Price        assetquantity.AssetQuantity     // содержит coins, precision, asset
    Quantity     assetquantity.AssetQuantity     // содержит coins, precision, asset
    TransactTime time.Time
    IsBuyerMaker bool
}
```

**Почему:**
- Entity — ядро приложения, не зависит от инфраструктуры
- Value objects инкапсулируют бизнес-правила (precision, asset, market)
- Смена БД или схемы хранения не затрагивает доменный слой
- `Market`, `Section`, `PricePrecision` — это не самостоятельные поля, а атрибуты value objects

### ES-0161: Database row struct хранится в пакете `models/` внутри репозитория

Row struct — это implementation detail репозитория. Он содержит теги маппинга БД (`ch:`, `db:`) и повторяет структуру таблицы. Row structs размещаются в отдельном пакете `models/` внутри соответствующего репозитория.

Правило распространяется на все базы данных.

```
# ES-0161: CORRECT — row structs в пакете models/ внутри репозитория
internal/repository/clickhouse/
├── models/
│   └── trade_row.go          # TradeRow с ch: тегами
├── builders/
│   └── trade_builder.go      # маппинг entity ↔ row
└── trade_data_repo.go        # использует models + builders

internal/repository/postgres/strategy/
├── models/
│   └── backtest_result_row.go  # BacktestResultRow с db: тегами
├── builders/
│   └── backtest_result_builder.go
└── backtest_result_repo.go
```

```go
// ES-0161: CORRECT — row struct в пакете models, экспортированный, с db-тегами
package models

type TradeRow struct {
    TradeID           int64     `ch:"trade_id"`
    Market            string    `ch:"market"`
    Symbol            string    `ch:"symbol"`
    Section           string    `ch:"section"`
    BaseAsset         string    `ch:"base_asset"`
    QuoteAsset        string    `ch:"quote_asset"`
    Price             int64     `ch:"price"`
    Quantity          int64     `ch:"quantity"`
    PricePrecision    uint8     `ch:"price_precision"`
    QuantityPrecision uint8     `ch:"quantity_precision"`
    TransactTime      time.Time `ch:"transact_time"`
    IsBuyerMaker      bool      `ch:"is_buyer_maker"`
}
```

```go
// ES-0161: WRONG — row struct внутри entity или в общем models/ пакете
package entity

type RawTrade struct {
    TradeID int64 `ch:"trade_id"` // db-теги в доменном слое!
}
```

```go
// ES-0161: WRONG — row struct приватный в файле репозитория
package clickhouse

type tradeRow struct { ... } // сложно переиспользовать в builders/
```

**Правила:**
- Row struct **экспортированный** (в отдельном пакете, используется из `builders/`)
- Суффикс `Row`: `TradeRow`, `BacktestResultRow`, `KlineRow`
- Один файл — одна row struct
- Row struct **не импортируется** за пределами `repository/`

### ES-0162: Маппинг entity ↔ row реализуется как builder в пакете `builders/`

Функции преобразования между domain entity и database row struct размещаются в отдельном пакете `builders/` внутри репозитория. Это обеспечивает тестируемость маппинга и чёткое разделение ответственности.

Правило распространяется на все базы данных.

```go
// ES-0162: CORRECT — builder-функции в отдельном пакете builders/
package builders

import (
    "github.com/webdelo/trade-signal-analyzer/internal/domain/entity"
    "github.com/webdelo/trade-signal-analyzer/internal/repository/clickhouse/models"
)

// FromRawTrade конвертирует domain entity → DB row для INSERT.
func FromRawTrade(trade entity.RawTrade) models.TradeRow {
    return models.TradeRow{
        TradeID:           trade.TradeID,
        Market:            string(trade.Symbol.Market.Code),
        Symbol:            trade.Symbol.Pair,
        Section:           string(trade.Symbol.Market.Section),
        BaseAsset:         trade.Quantity.Asset().Name(),
        QuoteAsset:        trade.Price.Asset().Name(),
        Price:             trade.Price.Coins(),
        Quantity:          trade.Quantity.Coins(),
        PricePrecision:    trade.Price.Precision(),
        QuantityPrecision: trade.Quantity.Precision(),
        TransactTime:      trade.TransactTime,
        IsBuyerMaker:      trade.IsBuyerMaker,
    }
}

// ToRawTrade конвертирует DB row → domain entity для SELECT.
func ToRawTrade(row models.TradeRow, symbol primitives.Symbol) entity.RawTrade {
    priceAsset := assetquantity.NewAsset("", row.QuoteAsset, row.PricePrecision)
    qtyAsset := assetquantity.NewAsset("", row.BaseAsset, row.QuantityPrecision)

    return entity.RawTrade{
        TradeID:      row.TradeID,
        Symbol:       symbol,
        Price:        assetquantity.FromCoins(row.Price, priceAsset),
        Quantity:     assetquantity.FromCoins(row.Quantity, qtyAsset),
        TransactTime: row.TransactTime,
        IsBuyerMaker: row.IsBuyerMaker,
    }
}
```

```go
// ES-0162: CORRECT — репозиторий использует builders для маппинга
package clickhouse

func (r *TradeDataRepository) InsertBatch(ctx context.Context, trades []entity.RawTrade) error {
    batch, err := r.conn.PrepareBatch(ctx, `INSERT INTO raw_trades (...)`)
    if err != nil {
        return err
    }

    for i := range trades {
        row := builders.FromRawTrade(trades[i])
        if err := batch.AppendStruct(&row); err != nil {
            return err
        }
    }

    return batch.Send()
}
```

```go
// ES-0162: WRONG — маппинг размазан по репозиторию
package clickhouse

func (r *TradeDataRepository) InsertBatch(ctx context.Context, trades []entity.RawTrade) error {
    for i := range trades {
        // inline маппинг — нетестируем, дублируется
        batch.Append(
            trades[i].TradeID,
            string(trades[i].Symbol.Market.Code),
            trades[i].Symbol.Pair,
            // ...
        )
    }
}
```

**Правила:**
- Функции экспортированные: `FromRawTrade`, `ToRawTrade` (не `fromRawTrade`)
- Именование: `From<Entity>` (entity → row), `To<Entity>` (row → entity)
- Один файл на пару entity/row: `trade_builder.go`, `backtest_result_builder.go`
- Builder-функции тестируются отдельно от репозитория
- Builder **не импортируется** за пределами `repository/`

---

## 18. Parameter Passing

### ES-0170: Prefer value parameters over pointers

**Default:** pass structs and primitives by value. Copy semantics are explicit and prevent aliasing bugs.

**Pointer parameters are justified ONLY when:**
1. **Long-lived service object** — DI-injected dependency that lives for the entire service lifecycle (e.g., `*Repository`, `*Server`)
2. **Callee mutates state** — exceptional case; prefer return-copy pattern (like `append`) for clarity

**Return-copy over mutation:** When a function modifies a struct, return the modified copy instead of mutating through a pointer. This makes data flow explicit (like `append` for slices).

**Examples:**

```go
// ✅ CORRECT — value parameter (Symbol is a domain value object)
func CheckExistingRange(ctx context.Context, sym primitives.Symbol, fromID, toID int64) (int64, error)

// ❌ INCORRECT — unnecessary pointer
func CheckExistingRange(ctx context.Context, sym *primitives.Symbol, fromID, toID int64) (int64, error)

// ✅ CORRECT — return modified copy
func WithTimeout(cfg Config, timeout time.Duration) Config {
    cfg.Timeout = timeout
    return cfg
}

// ❌ INCORRECT — mutating through pointer
func SetTimeout(cfg *Config, timeout time.Duration) {
    cfg.Timeout = timeout
}
```

**Reference:** [Go Code Review Comments — Pass Values](https://go.dev/wiki/CodeReviewComments#pass-values), [Effective Go — Pointers vs. Values](https://go.dev/doc/effective_go#pointers_vs_values)

### ES-0171: nolint annotations are FORBIDDEN

**CRITICAL:** Using `//nolint:...` annotations in code is strictly prohibited. These annotations mask real issues and reduce code quality visibility.

If the linter reports a legitimate warning:
1. **Fix the code** to satisfy the linter
2. **Configure the linter** (`.golangci.yaml`) to adjust thresholds or exclude specific patterns globally
3. **Never** suppress warnings at the call site with `//nolint`

```go
// ES-0171: FORBIDDEN
func Process(sym Symbol) error { //nolint:gocritic
    // ...
}

// ES-0171: CORRECT — fix the code or configure linter globally
func Process(sym Symbol) error {
    // ...
}
```

### ES-0172: Use full object names in log messages

Log messages must use full, descriptive object names — never abbreviations. A reader should understand the log entry without knowing the code context.

```go
// ES-0172: CORRECT
log.Info(ctx, "loading raw trades from file",
    zap.String("symbol", symbol.Code),
    zap.String("file_name", fileName),
)

// ES-0172: WRONG — abbreviated names
log.Info(ctx, "loading rt from f",
    zap.String("sym", sym.Code),
)
```

### ES-0173: Variable naming — no unclear abbreviations

Variable names must be either:
- **Single letter** (if unique in scope, e.g., `s` for a single string parameter)
- **Full descriptive name** (e.g., `symbol`, `decompressor`, `configuration`)

Partial abbreviations are acceptable ONLY for widely recognized Go idioms (`ctx`, `err`, `cfg`, `repo`, `tx`, `db`). All other names must be spelled out fully.

```go
// ES-0173: CORRECT — well-known Go idioms
ctx context.Context
err error
cfg Config
repo *Repository
tx pgx.Tx

// ES-0173: CORRECT — full descriptive names
symbol primitives.Symbol
decompressor *csv.Decompressor
configuration SignalConfiguration

// ES-0173: WRONG — unclear abbreviations
sym primitives.Symbol     // use "symbol"
decomp *csv.Decompressor  // use "decompressor"
pf *ParserFactory         // use "parserFactory"
```

---

## Summary - Key Principles

1. **Simplicity** - code should be simple and clear
2. **Clarity** - prefer obvious code over clever code
3. **Consistency** - consistent code is easier to maintain
4. **Readability** - easy to write, hard to read
5. **Minimalism** - don't add without necessity

---

## References

- [Effective Go](https://go.dev/doc/effective_go)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
