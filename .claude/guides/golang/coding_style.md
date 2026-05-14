# Go Coding Style Guide

This document contains coding style rules for Go projects. Each rule has a unique identifier (CS-XXXX) for reference in code reviews and discussions.

> **Related**: For effective programming standards, see [effective_standards.md](./effective_standards.md)
> **Related**: For concurrency rules, see [effective_concurrency.md](./effective_concurrency.md)

---

## 1. General Formatting Rules

### CS-0001: Line length limit - 120 characters
Maximum line length is 120 characters. Break long lines at logical points.

### CS-0002: Explicit returns only
Avoid naked returns. Always specify what you're returning.

```go
// CS-0002: CORRECT - explicit return
func process() (result Result, err error) {
    return Result{}, nil
}

// CS-0002: AVOID - naked return
func process() (result Result, err error) {
    result = Result{}
    err = nil
    return  // less clear
}
```

### CS-0003: No magic numbers
Use named constants instead of magic numbers.

**Exceptions**: `0`, `1`, `2`, `0.5` are allowed.

```go
// CS-0003: GOOD
const minEventCount = 0

// CS-0003: BAD
if count > 42 {  // what is 42?
}
```

### CS-0004: No global state unless absolutely necessary
Avoid global variables. Use dependency injection instead.

---

## 2. Block Spacing Rules

### CS-0010: Empty line after if/switch/select blocks
After blocks `if`, `switch`, `select` there must be an empty line.

**Exception**: Between error initialization and `if err != nil {}` there should be NO empty line.

```go
// CS-0010: GOOD - empty line after if block
if condition {
    doSomething()
}

nextOperation()
```

```go
// CS-0010: GOOD - no empty line before app_error check
result, err := someOperation()
if err != nil {
    return err
}

// Continue with result...
```

```go
// CS-0010: BAD - missing empty line after if block
if condition {
    doSomething()
}
nextOperation()
```

```go
// CS-0010: BAD - unnecessary empty line before app_error check
result, err := someOperation()

if err != nil {  // Unnecessary empty line!
    return err
}
```

---

## 3. Code Quality Principles

### CS-0020: Code should read top-to-bottom like a story
Function code should be self-explanatory and read naturally.

### CS-0021: Use clear, meaningful names
Names for functions and variables should be descriptive.

### CS-0022: Extract meaningful actions into functions
```go
// CS-0022: BAD
for _, u := range users {
    if u.IsBanned {
        bannedUsersCount++
    }
}

// CS-0022: GOOD
bannedUsersCount := countBannedUsers(users)
```

### CS-0023: Keep cyclomatic complexity low
- Use **guard clauses** to reduce deep nesting
- Prefer **early returns** to simplify control flow

### CS-0024: Avoid over-engineering
- Prefer simple, explicit solutions over complex abstractions
- Don't optimize prematurely
- Avoid unnecessary interfaces, decorators, or factories

---

## 4. Linting Policy

### CS-0030: Never modify linter configuration
**FORBIDDEN to modify `.golangci.yaml`!**

When receiving linter warnings, **ALWAYS fix the code**, not the linter settings.

**Forbidden actions:**
- Adding exceptions to `exclude-rules`
- Disabling linters in `linters.disable`
- Changing thresholds (e.g., `dupl.threshold`, `lll.line-length`)
- Adding files/directories to `exclude-dirs` or `exclude-files`

### CS-0031: //nolint directives require justification
`//nolint` directives are allowed ONLY:
- With specific linter: `//nolint:errcheck`
- With comment explaining why: `//nolint:unused // Reserved for future use`
- In exceptional cases when fix is impossible

**Principle**: If linter points to a problem - it's a problem in code that needs to be fixed.

---

## 5. Anonymous Functions

### CS-0040: Anonymous functions in function calls - max 5 lines
Anonymous functions can be passed directly in function parameters ONLY if length does not exceed 5 lines.

**Rules:**
- **<= 5 lines**: can pass anonymous function directly, all parameters on separate lines
- **> 5 lines**: MUST extract function to named variable before call

```go
// CS-0040: GOOD (≤ 5 lines)
c.addHandler(
    topic,
    func(ctx context.Context, msg Message) error {
        return processMessage(ctx, msg)
    },
)
```

```go
// CS-0040: BAD (> 5 lines, not extracted)
c.addHandler(topic, func(ctx context.Context, msg Message) error {
    if err := validate(msg); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    result := transform(msg)
    if err := save(result); err != nil {
        return fmt.Errorf("save failed: %w", err)
    }
    return nil
})
```

```go
// CS-0040: GOOD (extracted to named variable)
messageHandler := func(ctx context.Context, msg Message) error {
    if err := validate(msg); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    result := transform(msg)
    if err := save(result); err != nil {
        return fmt.Errorf("save failed: %w", err)
    }

    return nil
}

c.addHandler(topic, messageHandler)
```

**Rationale:**
- Long anonymous functions in parameters hurt readability
- Hard to understand what function does when "hidden" inside call
- Named variables give function a clear name (`messageHandler`, `errorHandler`)
- Simplifies testing and reuse

---

## 6. Package Structure

### CS-0050: Each entity in separate package
Each entity (entity/service/repository) must be in its own package.

**Scope**: This rule applies to ALL packages: services, repositories, infrastructure components, AND domain entity packages in `entity/`, `vo/`, `dto/`. Domain entity structs should be organized so each package has a single primary exported struct where possible.

This ensures:
- Code readability
- Isolation of changes
- Clean dependencies through interfaces
- Easy code navigation

```
// CS-0050: GOOD
lib/
├── consumer/           # separate package for Consumer
│   ├── consumer.go     # struct, interfaces, constructor
│   ├── start.go        # Start() method
│   ├── stop.go         # Stop() method
│   └── retry.go        # Retry() method
├── producer/           # separate package for Producer
│   ├── producer.go
│   ├── publish.go
│   └── batch.go
└── dispatcher/         # separate package for Dispatcher
    ├── dispatcher.go
    └── dispatch.go
```

```
// CS-0050: BAD
lib/
└── core/               # "mega-package" with many entities
    ├── consumer.go
    ├── producer.go
    ├── dispatcher.go
    ├── handler.go
    └── ... (many files, hard to navigate)
```

### CS-0051: File structure within package

**Main file** (name matches package, e.g., `consumer.go`):
1. Used types and dependency interfaces (defined at consumer!)
2. Entity struct
3. Constructor `New()`
4. Private validation methods

**Separate files for public methods**:
- Each public method in separate file
- Filename = method name in snake_case (e.g., `Retry()` → `retry.go`)

### CS-0052: Element order in main file

```go
package mypackage

// 1. Imports
import (...)

// 2. Interfaces (dependencies)
type Repository interface { ... }
type Logger interface { ... }

// 3. Types - auxiliary types, configs
type Config struct { ... }
type Options struct { ... }

// 4. Constants and package-level variables
const defaultTimeout = 30 * time.Second
var ErrNotFound = errors.New("not found")

// 5. Entity struct
type Service struct { ... }

// 6. Constructor
func New(...) (*Service, error) { ... }

// 7. Private helper functions
func validateConfig(...) error { ... }
func validateDependencies(...) error { ... }
```

### CS-0053: Forbidden file patterns
- DO NOT place public methods in main file (except constructor)
- DO NOT define interfaces in separate files from entity that uses them
- DO NOT create "God-files" with 500+ lines

### CS-0119: One primary exported struct per file in entity/vo/dto packages

In `entity/`, `vo/`, and `dto/` directories, each `.go` file must contain at most **one primary exported struct** with its constructor and methods.

**Filename** = `snake_case` of the primary struct name (e.g., `UploadSession` → `upload_session.go`).

**Allowed in the same file:**
- Private helper structs used exclusively by the primary struct
- Constants and type aliases directly related to the primary struct
- Interfaces consumed by the primary struct

**Forbidden:**
- Multiple exported structs in one file (split into separate files)
- File name that doesn't match the primary struct

```
// CS-0119: GOOD
entity/warehouse/
├── material.go          # Material struct + methods
├── material_category.go # MaterialCategory struct + methods
├── unit_conversion.go   # UnitConversion struct + methods
└── receipt.go           # Receipt struct + methods
```

```
// CS-0119: BAD - multiple entities in one file
entity/warehouse/
├── material.go    # Material + MaterialCategory + UnitConversion
└── stock.go       # StockEvent + StockProjection + StockSnapshot
```

**Rationale**: One struct per file ensures clear ownership, simplifies navigation, and prevents "God-files". Combined with CS-0050 (each entity in separate package), this keeps the codebase modular.

---

## 7. Documentation

### CS-0060: Godoc comments for public methods
Every public method/function must have short but meaningful `// godoc` comments unless trivial.

### CS-0061: Use gofmt and golangci-lint
Format with `gofmt`, run `golangci-lint`.

---

## 8. Naming Style

### CS-0070: English-only naming
Use **English-only** for all identifiers.

### CS-0071: Variable names - meaningful
Make variable names **meaningful** and proportional to scope:
- **Wide scope → Long, descriptive names**
- **Narrow scope → Shorter names** (e.g., loop vars: `i`, `v`, `u`)

### CS-0072: Use idiomatic casing
- **`lowerCamelCase`** for variables, functions, and methods (unexported)
- **`UpperCamelCase`** for exported functions, types, and constants

---

## 9. Types and Function Limits

### CS-0080: Distinct types for meaningful values
```go
// CS-0080: BAD
func myFunc(usersCount int, userID int, descriptor int)

// CS-0080: GOOD
func myFunc(usersCount int, userID UserID, descriptor Descriptor)
```

### CS-0081: Functions returning >3 values need refactoring
Consider using a struct or splitting the function.

### CS-0082: Function length rules
- **40+ lines:** Requires review
- **80+ lines:** Generally considered bad

---

## 10. Nil Pointer Safety

### CS-0090: Always check pointers for nil
Unless:
1. They were validated earlier in the function
2. The pointer is local and guaranteed to be non-nil

```go
// CS-0090: GOOD - total check
func anyPointerHandler(p any) error {
    if p == nil || (reflect.ValueOf(p).Kind() == reflect.Ptr && reflect.ValueOf(p).IsNil()) {
        return errors.New("pointer is nil")
    }
    // ...
}

// CS-0090: BAD - incomplete check
func anyPointerHandler(p any) error {
    if p == nil {  // doesn't catch typed nil pointers!
        return errors.New("pointer is nil")
    }
    // ...
}
```

---

## 11. Configuration Standards

### CS-0100: No envDefault in production configs
`envDefault` tag is forbidden in production configuration structures.

**Rationale**: According to 12-factor app methodology, configuration should come from environment variables. Default values hide missing configuration and make debugging difficult.

**Where defaults belong**:
- `dev/deployments/examples/*.env.example` - documented examples
- `dev/deployments/dev.env` - development environment

**No exceptions** - all configuration values must be explicit.

```go
// CS-0100: BAD
type Config struct {
    Timeout int `env:"TIMEOUT" envDefault:"30"`  // Hidden default
}

// CS-0100: GOOD
type Config struct {
    Timeout int `env:"TIMEOUT,required"`  // Fails fast if missing
}
// Default documented in dev/deployments/examples/service.env.example:
// TIMEOUT=30  # Request timeout in seconds
```

**This rule enforces explicit configuration and easier debugging.**

---

## 13. Data Structure Design

### CS-0113: Entity = identity + lifecycle + closed mutable state

Use `Entity` for domain objects with stable identity and lifecycle (`User`, `Order`, `Session`, `Token` aggregate).

**Requirements:**
- Must have explicit identity (`ID`, `UUID`, etc.)
- Mutable fields MUST be unexported
- State changes only through methods that enforce invariants
- Equality is by identity, not by full field comparison
- Constructors/factory functions must validate required invariants
- Exported methods must represent domain behavior, not raw field mutation (`SetX` allowed only when it enforces a business rule)

**Field visibility policy:**
- Closed mutable fields are mandatory
- Read access via methods is allowed
- Sensitive state (token value, secret hash, internal status flags) MUST never be directly mutable from outside

```go
// CS-0113: GOOD - entity with closed state and controlled mutation
type Token struct {
    id        string
    valueHash string
    revokedAt *time.Time
}

func (t *Token) Revoke(now time.Time) error {
    if t.revokedAt != nil {
        return errors.New("token already revoked")
    }

    t.revokedAt = &now
    return nil
}
```

```go
// CS-0113: BAD - entity with open mutable fields
type Token struct {
    ID        string
    ValueHash string
    RevokedAt *time.Time
}
```

**Rationale**: For entities, open mutable fields break invariants and allow invalid state transitions from outside.

### CS-0115: Value Object = value semantics + immutability

Use `Value Object` when identity is irrelevant and only value matters (`Money`, `Email`, `Address`).

**Requirements:**
- No identity used for equality
- Compare by all significant fields
- Must be immutable after construction
- Exposed fields are allowed only when effectively read-only
- Validation occurs at creation time (`NewXxx`, parser, or constructor)
- No methods that mutate receiver state

```go
// CS-0115: GOOD - immutable value object
type Money struct {
    amount   int64
    currency string
}

func NewMoney(amount int64, currency string) (Money, error) {
    if amount < 0 {
        return Money{}, errors.New("amount cannot be negative")
    }

    if currency == "" {
        return Money{}, errors.New("currency cannot be empty")
    }

    return Money{amount: amount, currency: currency}, nil
}

func (m Money) Amount() int64 { return m.amount }
func (m Money) Currency() string { return m.currency }
```

```go
// CS-0115: BAD - mutable value object
type Money struct {
    Amount   int64
    Currency string
}
```

**Rationale**: Value objects must be stable and predictable; mutability makes equality and invariants fragile.

### CS-0116: DTO = transport structure + open fields + no domain invariants

Use `DTO` only at system boundaries (HTTP, gRPC, queues, repository mapping).

**Requirements:**
- Fields are exported for serialization and mapping
- No business behavior that changes domain state
- Validation only for transport contract shape (required fields, format)
- DTO must not be reused as `Entity` or `Value Object`
- DTO methods may exist only for mechanical concerns (mapping helpers, normalization without business decisions)

**Field visibility policy:**
- Open exported fields are the default and preferred style
- DTO with mixed visibility (part exported, part unexported) is forbidden
- Closed fields in DTO are allowed only for technical metadata that is not part of the contract and is not exported outside package

```go
// CS-0116: GOOD - boundary DTO with open fields
type CreateUserRequestDTO struct {
    Email string `json:"email"`
    Name  string `json:"name"`
}
```

```go
// CS-0116: BAD - mixed DTO + domain behavior
type CreateUserRequestDTO struct {
    Email string `json:"email"`
    Name  string `json:"name"`
}

func (d *CreateUserRequestDTO) ActivatePremium() {
    // domain rule in transport model - forbidden
}
```

**Rationale**: In Clean Code terms, DTO is a data structure (open data), while domain objects are behavior-centric objects (closed state + methods). Mixing these models creates leaky boundaries and fragile code.

### CS-0117: Exported Entity/VO/DTO structs must live in entity/vo/dto packages

Every exported data struct that crosses package boundaries must be classified as exactly one of:
`Entity`, `Value Object`, or `DTO`.

**Requirements:**
- Exported `Entity` structs MUST be in `entity` package
- Exported `Value Object` structs MUST be in `vo` package
- Exported `DTO` structs MUST be in `dto` package
- A struct cannot belong to multiple categories simultaneously
- Crossing package boundaries with a data struct requires explicit category choice (`entity`/`vo`/`dto`)

**Allowed exceptions:**
- Technical structs that are not data models (`Service`, `Repository`, `Client`, `Config`) follow CS-0050/CS-0051
- Test-only helper structs in `_test.go`

```go
// CS-0117: GOOD - exported DTO in dedicated dto package
package dto

type UserResponse struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

```go
// CS-0117: BAD - exported DTO in non-dto package
package userhandler

type UserResponse struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

**Rationale**: Explicit package/category binding prevents model drift and makes rule applicability machine-detectable.

### CS-0118: Layer-local structs outside entity/vo/dto must be unexported and package-local

Layer-specific structs are allowed outside `entity`/`vo`/`dto` only when they are implementation details.

**Requirements:**
- Struct name must be unexported (starts with lowercase)
- Usage scope must be limited to a single package
- Such structs must not leak through exported API (no exported return types, fields, or interface contracts)

```go
// CS-0118: GOOD - package-local payload
package userhandler

type createUserPayload struct {
    Email string `json:"email"`
    Name  string `json:"name"`
}
```

```go
// CS-0118: BAD - package-local struct leaked via exported API
package userhandler

type createUserPayload struct {
    Email string `json:"email"`
}

func (h *Handler) Parse() createUserPayload { // leaked through exported method
    return createUserPayload{}
}
```

**Rationale**: Keeps layer internals local while preserving clear external domain/transport boundaries.

### CS-0120: Struct field visibility must be uniform

All fields of a struct must have the **same visibility** — either all exported or all unexported. Mixing exported and unexported fields in the same struct is **forbidden**.

**Direction by struct category:**
- **Entity** (CS-0113): all fields **unexported** — access via getter methods
- **Value Object** (CS-0115): all fields **unexported** — access via getter methods
- **DTO** (CS-0116): all fields **exported** — open for serialization

```go
// CS-0120: GOOD - entity with uniform unexported fields
type UploadSession struct {
	id            uuid.UUID
	ownerID       uuid.UUID
	fileName      string
	fileSize      int64
	status        UploadSessionStatus
	createdAt     time.Time
}

func (s *UploadSession) ID() uuid.UUID { return s.id }
func (s *UploadSession) OwnerID() uuid.UUID { return s.ownerID }
```

```go
// CS-0120: BAD - mixed visibility
type UploadSession struct {
	ID      uuid.UUID              // exported
	OwnerID uuid.UUID              // exported
	status  UploadSessionStatus    // unexported
}
```

**Rationale**: Mixed visibility signals confused design — either the struct protects its invariants (entity/VO) or exposes data openly (DTO). A half-measure breaks both contracts.

### CS-0121: Single constructor per entity struct

Each entity struct must have **exactly one constructor**. Constructor naming follows package scope:

- Entity is the **sole exported struct** in its package → `New()`
- Package contains **multiple exported structs** (transitional) → `New<StructName>()`
- There must be exactly **one** constructor per struct
- For reconstruction from storage, use `Restore()` or `Reconstruct()` pattern on repository side

```go
// CS-0121: GOOD - single entity, single constructor
package token

func New(id string, valueHash string, createdAt time.Time) (*Token, error) {
	// validate invariants
	return &Token{id: id, valueHash: valueHash, createdAt: createdAt}, nil
}
```

```go
// CS-0121: BAD - multiple constructors
package token

func New(id string, hash string) *Token { ... }
func NewFromDB(row *sql.Row) *Token { ... }     // should be in repository
func NewDefault() *Token { ... }                 // use options pattern (CS-0122)
```

**Rationale**: Single constructor enforces one validated entry point. Multiple constructors scatter validation and may allow invalid states.

### CS-0122: Options pattern for multiple initialization variants

When a constructor needs **optional parameters** or **multiple initialization variants**, use the functional options pattern instead of creating multiple constructors.

```go
// CS-0122: GOOD - functional options
type Option func(*config)

func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

func WithRetries(n int) Option {
	return func(c *config) { c.maxRetries = n }
}

func New(required string, opts ...Option) (*Service, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Service{cfg: cfg}, nil
}
```

```go
// CS-0122: BAD - multiple constructors for variants
func New(name string) *Service { ... }
func NewWithTimeout(name string, timeout time.Duration) *Service { ... }
func NewWithRetries(name string, retries int) *Service { ... }
```

**Rationale**: Options pattern provides a single entry point while allowing flexible configuration. This follows the Go standard library pattern (`http.Server`, `grpc.NewServer`).

---

## 14. Error Handling Style

### CS-0114: No error storage in struct fields for deferred retrieval
FORBIDDEN: Storing an error in a struct field in one method and returning it from another method. Each method that can produce an error MUST return it directly to the caller.

```go
// CS-0114: BAD - error stored in struct, retrieved later
type Parser struct {
    current    Record
    currentErr error  // anti-pattern: deferred error
}

func (p *Parser) Next() bool {
    p.current, p.currentErr = p.parse()
    return p.currentErr == nil
}

func (p *Parser) Value() (Record, error) {
    return p.current, p.currentErr  // error from a different method call
}
```

```go
// CS-0114: GOOD - error returned directly from the method that produces it
type Parser struct {
    // no error field
}

// Next returns the parsed record and any error directly.
// Returns io.EOF when no more data is available.
func (p *Parser) Next(ctx context.Context) (Record, error) {
    record, err := p.parse(ctx)
    if err != nil {
        return Record{}, err
    }
    return record, nil
}
```

**Rationale**: Storing errors in struct fields creates implicit state coupling between methods, makes error flow harder to trace, and violates the principle that errors should be handled at the point of occurrence. For stream-like APIs, use `io.EOF` as the end-of-data signal — this is the standard Go pattern (`io.Reader`, `json.Decoder`).

---

## Summary - Style Principles

1. **Readability** - code should be easy to read and understand
2. **Consistency** - follow the same patterns throughout codebase
3. **Simplicity** - prefer simple solutions over complex ones
4. **Explicitness** - be explicit about what code does
5. **Self-documenting** - names and structure should explain intent

---

## 12. Comment Hygiene

### CS-0111: Minimal inline comments - code must be self-explanatory
Inline comments that restate what the code does are forbidden. Code should be readable without comments. Non-obvious design decisions and important assumptions belong in godoc format at the function/method level.

```go
// CS-0111: BAD - restates the obvious
mu.Lock()        // lock the mutex
defer mu.Unlock() // unlock when done

// CS-0111: BAD - describes what, not why
if conn != nil { // check if connection exists
    return conn, nil
}

// CS-0111: GOOD - godoc explains non-obvious design decision
// buildDescriptorSource returns a grpcurl.DescriptorSource for the given proto source.
// Reflection clients are not cached because the service schema may evolve between calls.
func (t *GrpcTransport) buildDescriptorSource(...) (grpcurl.DescriptorSource, error) {
```

### CS-0112: No decorative comment separators
FORBIDDEN: decorative banners or section dividers made of repeated characters used to separate code blocks.

```go
// CS-0112: FORBIDDEN
// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// CS-0112: FORBIDDEN
// ===========================
// Validation
// ===========================

// CS-0112: FORBIDDEN
// --- helpers ---
```

If code requires decorative separators to be readable, it signals a structural problem: the file is too large, logic is mixed in one place, or methods belong in separate files per CS-0051.

**Fix**: split into separate files or extract into well-named functions — not add visual noise.

### CS-0110: No internal standard references in code comments
FORBIDDEN to include references to internal rules, standards, or guide identifiers in code comments.

Examples of FORBIDDEN patterns: `// TR-001:`, `// CR-S002:`, `// UT-0011 requires...`, `// DEVIATION from TASK DOD`.

Comments should explain WHAT and WHY, not trace back to review findings.

---

## References

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [golangci-lint](https://golangci-lint.run/)
