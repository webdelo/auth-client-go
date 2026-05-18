# REQ-001-TASK-002-PRP: Core Client + типизированные ошибки + Logger adapter

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-001-TASK-002-PRP |
| Parent Request | [REQ-001](../tasks/REQ-001.md) |
| Parent Task | [REQ-001-TASK-002](../tasks/REQ-001-TASK-002.md) |
| Parent ADR | [REQ-001-ADR](./REQ-001-ADR.md) |
| Created | 2026-05-15 |
| Updated | 2026-05-15 |
| Score | 9/10 |

---

## Goal

### Summary

Реализовать базовый каркас SDK: публичный тип `Client` с конструктором `New(Config, ...Option)` и идемпотентным `Close()`, 5 sentinel-ошибок с маппером `mapGRPCError` (gRPC status → typed error через `errors.Unwrap`-итерацию), adapter-интерфейс `Logger` с `NoopLogger{}` по умолчанию. Без какой-либо бизнес-логики (Validate/Login/Refresh — следующие TASK'и).

### Why

- **Все последующие TASK'и (003–006) импортируют этот каркас.** Без него `validate.go`, `auth.go`, `middleware_http.go`, `interceptor_grpc.go` физически не могут существовать.
- **Зафиксировать публичную API-поверхность.** Имена `Config`, `FailMode`, `ErrInvalidToken`, `Logger`, сигнатуры `New`/`Close` — frozen после `v1.0.0` ([REQ-001-ADR §API](./REQ-001-ADR.md) evolution table). Ошибки в API после релиза = breaking change.
- **Закрыть mapping gRPC ↔ typed errors в одном месте.** Иначе каждый последующий метод будет ветвиться по `codes.X` — type mess.
- **Заложить lifecycle для фоновых goroutines.** `cancelBG context.CancelFunc` + `sync.Once`-Close (EC-0010/EC-0012). В TASK-002 фоновых горутин нет, но задел готов под cache cleaner (TASK-003).

### What

После выполнения TASK'а consumer может выполнить:

```go
import authclient "github.com/webdelo/auth-client-go/lib"

c, err := authclient.New(authclient.Config{Endpoint: "auth:50051"})
if err != nil { return err }
defer c.Close()
```

Без реальных RPC-методов — только конструктор, options, ошибки, Logger. Все остальные методы (`ValidateAccessToken`, `Login`, etc.) — отдельные TASK'и.

### Success Criteria

- [ ] `New(Config{Endpoint: "auth:50051"})` возвращает рабочий `*Client` без ошибки.
- [ ] `New(Config{})` возвращает ошибку про обязательный Endpoint.
- [ ] `Close()` идемпотентен (повторный вызов не паникует, возвращает первую ошибку).
- [ ] `errors.Is(mapGRPCError(grpcErr), ErrXxx)` корректно для всех gRPC-кодов из таблицы маппинга (включая обёрнутые через `fmt.Errorf("...: %w", ...)`).
- [ ] `NoopLogger{}.Info/Warn/Error` не паникует с nil-аргументами.
- [ ] gRPC dial использует `grpc.NewClient` (не deprecated `grpc.Dial`) с keepalive `{30s, 10s, true}` и retry policy для UNAVAILABLE **только на `/auth.v1.AuthService/ValidateToken`** (`RefreshToken` НЕ ретраится).
- [ ] Unit-тесты покрывают ≥ 85% по `lib/client.go` и `lib/errors.go` (`make coverage`).
- [ ] `make audit` (vet + lint + test) проходит чисто.
- [ ] `go doc github.com/webdelo/auth-client-go/lib` показывает все exported типы без TODO.

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | ES-0010, ES-0011, ES-0021, ES-0040, ES-0050, ES-0051, ES-0056, ES-0060, ES-0061, ES-0062, ES-0063, ES-0080, ES-0081, ES-0090, ES-0120, ES-0140, ES-0142, ES-0143, ES-0171 |
| Effective Concurrency | [effective_concurrency.md](../../.claude/guides/golang/effective_concurrency.md) | EC-0010 (lifecycle), EC-0011 (cancel), EC-0012 (no leak), EC-0050 (ctx first), EC-0051 (no ctx in struct), EC-0053 (timeouts), EC-0055 (call cancel), EC-0062 (atomic Swap не нужен — используем sync.Once) |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | CS-0001 (120 chars), CS-0021 (clear names), CS-0050 (each entity package — SDK исключение, см. ниже), CS-0060 (godoc public), CS-0070 (English) |
| Unit Tests | [unit_tests.md](../../.claude/guides/golang/unit_tests.md) | UT-0001 (table-driven), UT-0002 (one method = one test), UT-0010 (TestType_Method), UT-0011 (happy_path first), UT-0012 (snake_case), UT-0027 (no decorative separators), UT-0028 (no helper funcs), UT-0030/0031 (require/assert), UT-0040/0041 (t.Parallel), UT-0100 (one test file per source file) |

### Исключение из CS-0050 (single primary struct per package)

SDK по архитектурному решению ADR — **один Go-пакет `authclient`**, в котором живут все exported типы (`Client`, `Config`, `User`, etc.). Это стандарт Go SDK-библиотек (`gorm.io/gorm`, `github.com/redis/go-redis/v9`) и осознанный выбор в [REQ-001-ADR §A1](./REQ-001-ADR.md). Внутреннее разбиение — через `lib/internal/cache/`, `lib/internal/refresh/` (это уже отдельные пакеты по CS-0050).

### Concurrency Patterns

- [x] **sync.Once для идемпотентного Close** (паттерн из `signal-analyzer/internal/infrastructure/export/engine.go:103`)
- [x] **context.CancelFunc + Close()** для будущей фоновой goroutine cache cleaner (задел под TASK-003)

---

## Analysis Files

- **Code Research**: [2026-05-15_code-research_REQ-001-TASK-002.md](../../.memory_bank/agent_task_results/2026-05-15_code-research_REQ-001-TASK-002.md) — паттерны `statusFromWrapped`, retry policy, `grpc.NewClient`, `sync.Once`-Close
- **ADR**: [REQ-001-ADR.md](./REQ-001-ADR.md) — секции §A1 (API surface), §A6 (gRPC connection), §A7 (typed errors + mapping table), §A14 (без JWT-парсинга), §A15 (без deprecated API)
- **ADR psevdocode**: [REQ-001-ADR.md](./REQ-001-ADR.md) — `Client` struct и `New()` (стр. 1071–1134), `Logger` (стр. 689–693), `Config` (стр. 561–571), Mapping table (§A7 стр. 200–230)
- **Previous research**:
  - [Codebase analysis for ADR](../../.memory_bank/agent_task_results/2026-05-15_code-research_codebase_analysis_REQ-001-ADR.yaml)
  - [API design for ADR](../../.memory_bank/agent_task_results/api_design_REQ-001-ADR.yaml)
  - [Web research for ADR](../../.memory_bank/agent_task_results/web_research_REQ-001-ADR.yaml)

---

## Context

### Current Structure

```
auth-client-go/
├── go.mod                          # module github.com/webdelo/auth-client-go (Go 1.26.2)
├── Makefile                        # make test/test-race/coverage/lint/audit
├── .golangci.yaml
├── mockery.yaml
└── lib/
    ├── lib.go                      # package lib, const Version = "0.0.1" (placeholder)
    ├── lib_test.go                 # TestVersion (placeholder)
    ├── examples/basic/main.go      # импортирует lib.Version
    └── internal/.gitkeep
```

### Desired Structure

```
auth-client-go/
├── go.mod                          # UPDATE: + auth-proto/go v0.1.0, grpc (если TASK-001 не выполнен)
├── go.sum                          # UPDATE: соответственно
└── lib/
    ├── client.go                   # NEW: Config, FailMode, Option, Client, New, Close, WithXxx
    ├── errors.go                   # NEW: 5 sentinel + AuthError + RetryAfter + mapGRPCError +
    │                               #      mapErrorReason + statusFromWrapped
    ├── logger.go                   # NEW: Logger interface + NoopLogger
    ├── client_test.go              # NEW: TestNew, TestClient_Close
    ├── errors_test.go              # NEW: TestMapGRPCError, TestMapErrorReason, TestRetryAfter,
    │                               #      TestStatusFromWrapped, TestAuthError_Unwrap, TestAuthError_Error
    ├── logger_test.go              # NEW: TestNoopLogger
    ├── lib.go                      # DELETE (placeholder Version)
    ├── lib_test.go                 # DELETE (placeholder TestVersion)
    ├── examples/basic/main.go      # UPDATE: stub print без Version (полная реализация — TASK-006)
    └── internal/.gitkeep
```

### Gotchas

| # | Category | Issue | Solution |
|---|----------|-------|----------|
| G1 | package name | `lib/lib.go` сейчас `package lib`. ADR §API требует `package authclient`. | Все новые файлы — `package authclient`. Удалить `lib/lib.go` + `lib/lib_test.go`. |
| G2 | grpc.NewClient lazy dial | `grpc.NewClient("localhost:50051", ...)` НЕ делает реальный TCP-connect. Возвращает `*ClientConn` даже без сервера. | Конструктор может создать `*Client` без живого AUTH — это by-design. Реальный коннект — при первом RPC. Тестируется без поднятия сервера. |
| G3 | keepalive.Time silently bumped | `grpc-go` при `Time < 10s` молча подменяет на 10s без ошибки. | SDK ставит фиксированные 30s ([ADR §R9](./REQ-001-ADR.md)). Не делать Time настраиваемым через Config в v0.1.0. |
| G4 | status.FromError не ходит по chain | Стандартный `status.FromError(err)` возвращает `(Unknown, false)` для `fmt.Errorf("ctx: %w", grpcErr)`. | Реализовать приватный `statusFromWrapped` с итерацией через `errors.Unwrap` (паттерн из `signal-analyzer/internal/usecase/bff/error_mapping.go:38-47`). |
| G5 | ValidateToken не возвращает gRPC error для invalid token | По proto-контракту: `is_valid: false` приходит как **успешный** `ValidateTokenResponse` с `error_reason`. gRPC error = только недоступность сервиса. | Два разных mapper'а: `mapGRPCError(err)` для RPC-уровневых ошибок, `mapErrorReason(reason)` для `is_valid=false`. Оба определены в `errors.go`. |
| G6 | AUTH server не кладёт Retry-After в metadata | AUTH v0.1.0 шлёт `status.Error(codes.ResourceExhausted, msg)` без trailer-metadata ([auth/internal/handlers/grpc/auth/login.go:26-30](file path)). | `mapGRPCError` для `ResourceExhausted` возвращает `&AuthError{Err: ErrRateLimited, RetryAfter: 0}`. Best-effort парсинг через `st.Details()` — если ничего не нашли, `0`. Будущая совместимость когда AUTH начнёт слать metadata. |
| G7 | retry policy только на один метод | `grpc.WithDefaultServiceConfig` применяет JSON ко всем методам, если `"name": [{"service": ""}]`. RefreshToken ретраить НЕЛЬЗЯ (family detection). | В JSON указать `"name": [{"service": "auth.v1.AuthService", "method": "ValidateToken"}]`. |
| G8 | mapGRPCError для unknown code | gRPC может вернуть код, которого нет в таблице маппинга (например `Internal`, `InvalidArgument`). | Default branch → `ErrUnavailable` с wrap. Не паниковать, не возвращать nil. |
| G9 | nil ошибка на входе mapGRPCError | Если caller вызовет `mapGRPCError(nil)` — должен вернуть nil. | Явный guard: `if err == nil { return nil }`. |
| G10 | `Config.ValidateCacheTTL < 0` | Отрицательный TTL — потенциальный bug (бесконечный кэш / зацикливание). | `applyDefaults` нормализует: `< 0 → default 30s`. `= 0 → keep` (отключает кэш в TASK-003). |
| G11 | lib/examples/basic зависит от Version | `lib/examples/basic/main.go` импортирует `lib.Version`. После переименования package и удаления Version — examples не компилируется. | Обновить `lib/examples/basic/main.go` на stub (`fmt.Println("authclient SDK")`). Полная реализация — в TASK-006. |
| G12 | ES-0143: NoopLogger как default | Если `Config.Logger == nil` — присвоить `NoopLogger{}` в конструкторе. Никаких `if logger != nil` в местах вызова. | Внутри `applyDefaults` или `New` — гарантия что `c.logger != nil`. |
| G13 | TASK-001 не выполнен | `go.mod` сейчас не содержит `auth-proto/go`, `grpc` — TASK-001 ещё не закрыт. | Перед началом TASK-002 убедиться: `go get github.com/webdelo/auth-proto/go@v0.1.0`, `go get google.golang.org/grpc`. Если deps уже есть (TASK-001 закрыт) — пропустить. |

---

## Implementation

### Data Models

```go
// Config — параметры конструктора. Endpoint обязателен.
type Config struct {
    Endpoint         string             // required: "host:port"
    ValidateCacheTTL time.Duration      // default 30s; 0 disables cache (used by TASK-003)
    FailMode         FailMode           // default FailClosed
    ValidateTimeout  time.Duration      // default 2s
    LoginTimeout     time.Duration      // default 5s
    RefreshTimeout   time.Duration      // default 5s
    DialOptions      []grpc.DialOption  // appended after SDK defaults
    Logger           Logger             // default NoopLogger{}
    Cache            Cache              // default nil (created by TASK-003)
}

// FailMode определяет поведение HTTPMiddleware/UnaryInterceptor (TASK-003) при недоступности AUTH.
type FailMode uint8

const (
    FailClosed FailMode = iota  // 0 — default: 503 on unavailable
    FailOpen                    // 1 — pass-through with empty User
)

// Option — функциональная опция для New.
type Option func(*Client)

// Client — публичный SDK-клиент. Создаётся через New, закрывается через Close.
type Client struct {
    cfg         Config
    grpcConn    *grpc.ClientConn
    authClient  auth_v1.AuthServiceClient
    logger      Logger
    failMode    FailMode
    openMethods map[string]struct{}     // used by TASK-003 UnaryInterceptor

    cancelBG    context.CancelFunc      // cancels future background goroutines (cache cleaner — TASK-003)
    closeOnce   sync.Once
    closeErr    error
}

// AuthError — wrapper для дополнительного контекста (Retry-After). Реализует Unwrap для errors.Is.
type AuthError struct {
    Err        error
    RetryAfter time.Duration
}

// Logger — adapter-интерфейс. SDK не зависит от конкретного логгера (slog/zap/logrus).
type Logger interface {
    Info(ctx context.Context, msg string, fields ...any)
    Warn(ctx context.Context, msg string, fields ...any)
    Error(ctx context.Context, msg string, err error, fields ...any)
}

// NoopLogger — silent default. Все методы no-op (ES-0143).
type NoopLogger struct{}
```

### Sentinel Errors

```go
// Все 5 sentinel'ов. Frozen после v1.0.0 (ADR API evolution table).
var (
    ErrInvalidToken = errors.New("authclient: invalid token")
    ErrExpired      = errors.New("authclient: token expired")
    ErrBlocked      = errors.New("authclient: user blocked")
    ErrRateLimited  = errors.New("authclient: rate limited")
    ErrUnavailable  = errors.New("authclient: service unavailable")
)
```

### Mapping Table (`mapGRPCError`)

| gRPC code | SDK error | Notes |
|-----------|-----------|-------|
| `Unauthenticated` | `ErrInvalidToken` | — |
| `PermissionDenied` | `ErrBlocked` | — |
| `ResourceExhausted` | `&AuthError{Err: ErrRateLimited, RetryAfter: 0}` | RetryAfter из `st.Details()` best-effort; в v0.1.0 AUTH не шлёт. |
| `Unavailable` | `ErrUnavailable` | retry policy на ValidateToken уже отработал, дошёл error. |
| `DeadlineExceeded` | `ErrUnavailable` | trance с timeout. |
| любой другой gRPC code | `fmt.Errorf("authclient: unexpected grpc code %s: %w", code, ErrUnavailable)` | wrap для godebug. `errors.Is(_, ErrUnavailable)` true. |
| non-gRPC error (включая network errors не обёрнутые в status) | `fmt.Errorf("authclient: %w", ErrUnavailable)` | catch-all. |
| `nil` | `nil` | guard. |

### Mapping Table (`mapErrorReason`)

Принимает `ValidateTokenResponse.error_reason` (строка). Используется TASK-003 для negative `is_valid=false`.

| `error_reason` | SDK error |
|----------------|-----------|
| `"expired"` | `ErrExpired` |
| `"blocked"` | `ErrBlocked` |
| `"invalid"`, `""`, любое неизвестное | `ErrInvalidToken` |

---

### Task 0 (pre-requisite): Проверить зависимости от TASK-001

**Action**: VERIFY
**File**: [go.mod](../../go.mod)

**Description**: TASK-002 импортирует `github.com/webdelo/auth-proto/go/auth_v1` и `google.golang.org/grpc`. Согласно зависимостям, TASK-001 ставит эти deps. Перед началом TASK-002 убедиться.

**Details**:
- Прочитать `go.mod`.
- Если `github.com/webdelo/auth-proto/go` отсутствует → выполнить `go get github.com/webdelo/auth-proto/go@v0.1.0`.
- Если `google.golang.org/grpc` отсутствует → выполнить `go get google.golang.org/grpc`.
- Выполнить `go mod tidy`.

**Commands**:
```bash
grep -E "auth-proto|google.golang.org/grpc" go.mod || \
    (go get github.com/webdelo/auth-proto/go@v0.1.0 && go get google.golang.org/grpc && go mod tidy)
```

---

### Task 1: Удалить placeholder lib.go + lib_test.go

**Action**: DELETE
**File**: [lib/lib.go](../../lib/lib.go), [lib/lib_test.go](../../lib/lib_test.go)

**Description**: Удалить старые placeholder файлы (содержат `package lib` + `const Version`), чтобы избежать конфликта с новым `package authclient`.

**Details**:
- Удалить `lib/lib.go`.
- Удалить `lib/lib_test.go`.

**Commands**:
```bash
rm lib/lib.go lib/lib_test.go
```

---

### Task 2: Обновить lib/examples/basic/main.go (stub)

**Action**: MODIFY
**File**: [lib/examples/basic/main.go](../../lib/examples/basic/main.go)

**Description**: Заменить `lib.Version` на stub. Полная реализация (Login → Validate → Logout) — в TASK-006.

**Code Pattern**:

```go
// Package main is a placeholder. The full demo lives under TASK-006.
package main

import "fmt"

func main() {
    fmt.Println("authclient SDK — see examples/basic/main.go in TASK-006")
}
```

**Note**: `lib/examples/` пока не обязан компилироваться по `make build-examples` — этот таргет наполняется в TASK-006. Достаточно того, что файл валиден синтаксически.

---

### Task 3: CREATE lib/errors.go

**Action**: CREATE
**File**: [lib/errors.go](../../lib/errors.go)

**Description**: 5 sentinel-ошибок, `AuthError` wrapper, `RetryAfter` helper, приватные `mapGRPCError`, `mapErrorReason`, `statusFromWrapped`.

**Details**:
- `package authclient`.
- 5 `var` sentinel'ов с префиксом `Err`, тексты `"authclient: ..."`.
- `AuthError struct { Err error; RetryAfter time.Duration }` с методами `Error() string` (делегирует `Err.Error()`) и `Unwrap() error` (возвращает `Err`).
- `func RetryAfter(err error) time.Duration` — через `errors.As(err, &ae)`.
- `func mapGRPCError(err error) error` — guard nil, использовать `statusFromWrapped`, switch по `status.Code()`, default → wrapped `ErrUnavailable`.
- `func mapErrorReason(reason string) error` — switch по строке.
- `func statusFromWrapped(err error) (*status.Status, bool)` — итерация через `errors.Unwrap`.

**Code Pattern**:

```go
// Package authclient is the official Go SDK for the webdelo/auth service.
package authclient

import (
    "errors"
    "fmt"
    "time"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// ─── Sentinel errors ───────────────────────────────────────────────────────

// ErrInvalidToken indicates the access token is malformed, unknown, or signed
// by a different key. The caller should re-authenticate.
var ErrInvalidToken = errors.New("authclient: invalid token")

// ErrExpired indicates the token signature is valid but its expiration has passed.
// The caller should refresh.
var ErrExpired = errors.New("authclient: token expired")

// ErrBlocked indicates the user account is blocked at the AUTH server.
// The caller should NOT retry — block is persistent until admin action.
var ErrBlocked = errors.New("authclient: user blocked")

// ErrRateLimited indicates AUTH rate-limited this caller. Inspect RetryAfter
// via the RetryAfter helper to know when to retry.
var ErrRateLimited = errors.New("authclient: rate limited")

// ErrUnavailable indicates AUTH was unreachable (network, timeout, server down).
// HTTPMiddleware behavior depends on Config.FailMode.
var ErrUnavailable = errors.New("authclient: service unavailable")

// ─── AuthError wrapper ─────────────────────────────────────────────────────

// AuthError carries the sentinel error plus optional context (Retry-After for
// rate-limited responses). Use errors.Is(err, ErrXxx) for sentinel detection
// and RetryAfter(err) for the retry hint.
type AuthError struct {
    Err        error
    RetryAfter time.Duration
}

func (e *AuthError) Error() string {
    if e == nil || e.Err == nil {
        return ""
    }
    return e.Err.Error()
}

func (e *AuthError) Unwrap() error {
    if e == nil {
        return nil
    }
    return e.Err
}

// RetryAfter returns the Retry-After hint for ErrRateLimited errors. Returns 0
// if err is nil, not an *AuthError, or the server did not include the hint.
func RetryAfter(err error) time.Duration {
    var ae *AuthError
    if errors.As(err, &ae) {
        return ae.RetryAfter
    }
    return 0
}

// ─── gRPC status → typed error mapper ──────────────────────────────────────

// mapGRPCError converts a gRPC error into one of the SDK sentinel errors.
// Returns nil if err is nil. Unwraps through fmt.Errorf("...: %w", ...) chains
// because status.FromError does NOT follow errors.Unwrap.
func mapGRPCError(err error) error {
    if err == nil {
        return nil
    }
    st, ok := statusFromWrapped(err)
    if !ok {
        return fmt.Errorf("authclient: %w", ErrUnavailable)
    }
    switch st.Code() {
    case codes.Unauthenticated:
        return ErrInvalidToken
    case codes.PermissionDenied:
        return ErrBlocked
    case codes.ResourceExhausted:
        return &AuthError{Err: ErrRateLimited, RetryAfter: retryAfterFromStatus(st)}
    case codes.Unavailable, codes.DeadlineExceeded:
        return ErrUnavailable
    default:
        return fmt.Errorf("authclient: unexpected grpc code %s: %w", st.Code(), ErrUnavailable)
    }
}

// retryAfterFromStatus best-effort extracts Retry-After from status details.
// AUTH v0.1.0 does NOT include this field — returns 0. Future-compat placeholder.
func retryAfterFromStatus(st *status.Status) time.Duration {
    // TODO (when AUTH starts sending google.rpc.RetryInfo in details):
    // for _, d := range st.Details() {
    //     if ri, ok := d.(*errdetails.RetryInfo); ok && ri.RetryDelay != nil {
    //         return ri.RetryDelay.AsDuration()
    //     }
    // }
    _ = st
    return 0
}

// statusFromWrapped extracts gRPC *Status from an error chain, walking through
// errors.Unwrap. Standard status.FromError does not unwrap.
//
// Pattern adapted from signal-analyzer:
// /Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping.go:38-47
func statusFromWrapped(err error) (*status.Status, bool) {
    for e := err; e != nil; e = errors.Unwrap(e) {
        if st, ok := status.FromError(e); ok && st.Code() != codes.Unknown {
            return st, true
        }
    }
    return nil, false
}

// ─── ValidateTokenResponse.error_reason → typed error mapper ──────────────

// mapErrorReason converts ValidateTokenResponse.error_reason (returned when
// is_valid=false) into a sentinel. Unknown values map to ErrInvalidToken
// (conservative default — caller should re-authenticate).
func mapErrorReason(reason string) error {
    switch reason {
    case "expired":
        return ErrExpired
    case "blocked":
        return ErrBlocked
    default: // "invalid", "", any unknown
        return ErrInvalidToken
    }
}
```

**Critical notes**:
- `statusFromWrapped` фильтрует `codes.Unknown` — это default из `status.FromError` для не-gRPC ошибок. Без фильтрации первая итерация на не-gRPC ошибке вернёт `(Unknown, true)` и мы не дойдём до wrap'нутого gRPC error.
- `ResourceExhausted` всегда возвращает `*AuthError` (даже если `RetryAfter == 0`) — это позволяет caller'у через `errors.Is(err, ErrRateLimited)` понять что это rate-limit, а через `RetryAfter(err)` получить hint.

---

### Task 4: CREATE lib/logger.go

**Action**: CREATE
**File**: [lib/logger.go](../../lib/logger.go)

**Description**: Adapter-интерфейс `Logger` со slog-совместимой variadic-сигнатурой и `NoopLogger{}` (ES-0143).

**Code Pattern**:

```go
package authclient

import "context"

// Logger is the adapter interface used by the SDK for diagnostic logging.
//
// Methods follow slog-style variadic fields: callers pass key-value pairs
// (e.g., []any{"user_uuid", uuid, "method", "Validate"}). The SDK does not
// depend on any concrete logging library (slog, zap, logrus, etc.).
//
// To plug in a concrete logger, wrap it in a small adapter that satisfies
// this interface and pass via Config.Logger or WithLogger.
type Logger interface {
    Info(ctx context.Context, msg string, fields ...any)
    Warn(ctx context.Context, msg string, fields ...any)
    Error(ctx context.Context, msg string, err error, fields ...any)
}

// NoopLogger is the silent default Logger. Used when Config.Logger is nil
// (ES-0143 noop defaults instead of nil-checks at every call site).
type NoopLogger struct{}

// Info implements Logger. No-op.
func (NoopLogger) Info(context.Context, string, ...any) {}

// Warn implements Logger. No-op.
func (NoopLogger) Warn(context.Context, string, ...any) {}

// Error implements Logger. No-op.
func (NoopLogger) Error(context.Context, string, error, ...any) {}

// Compile-time check.
var _ Logger = NoopLogger{}
```

**Critical notes**:
- Value receiver на `NoopLogger` (нет состояния — ES-0143 явно требует value-receiver для noop'ов).
- `Error` принимает `err error` отдельным параметром — это ADR §A5 решение. Совместимо со slog через адаптер: `slog.With("err", err).Error(msg, fields...)`.
- `Cache` interface (ADR §A5) — НЕ в TASK-002. Появится в TASK-003 вместе с реализацией.

---

### Task 5: CREATE lib/client.go

**Action**: CREATE
**File**: [lib/client.go](../../lib/client.go)

**Description**: Главный файл SDK — `Config`, `FailMode`, `Option`, `Client`, `New`, `Close`, `applyDefaults`, 4 функциональные опции.

**Details**:
- `package authclient`.
- `Config` struct из Data Models.
- `FailMode` enum (iota — `FailClosed=0`, `FailOpen=1`).
- `Option func(*Client)`.
- `WithFailMode`, `WithLogger`, `WithCache`, `WithOpenMethods` — функциональные опции.
- `Client` struct из Data Models (включая `closeOnce sync.Once`, `closeErr error`, `cancelBG context.CancelFunc`).
- `New(cfg Config, opts ...Option) (*Client, error)`:
  1. Валидация: `Endpoint != ""`.
  2. `cfg = applyDefaults(cfg)`.
  3. Собрать dial options (keepalive, retry-policy, transport-creds).
  4. `grpc.NewClient(cfg.Endpoint, dialOpts...)`.
  5. Создать `auth_v1.AuthServiceClient(conn)`.
  6. `bgCtx, cancel := context.WithCancel(context.Background())` — задел для TASK-003.
  7. Создать `*Client` + применить `opts`.
- `Close() error` — `closeOnce.Do(...)`, cancel BG context + close gRPC conn.
- `defaultOpenMethods()` returns `map[string]struct{}` с `/grpc.health.v1.Health/Check` и `/Watch`.
- `hasTransportCreds(opts []grpc.DialOption) bool` — helper для определения, нужен ли default `insecure.NewCredentials()`. Реализация — пройтись по opts, искать `*grpcTransportCredentialsOption`. Проблема — grpc-go не экспортирует tag типа. **Решение**: вместо детекта — задокументировать что **первой** в default dialOpts ставим `insecure.NewCredentials()`, а `cfg.DialOptions` идут после → пользовательский `WithTransportCredentials` override'нет. gRPC поддерживает множественные WithTransportCredentials — побеждает последний. Это стандартный паттерн.

**Code Pattern**:

```go
package authclient

import (
    "context"
    "errors"
    "fmt"
    "sync"
    "time"

    auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/keepalive"
)

// ─── Defaults ──────────────────────────────────────────────────────────────

const (
    defaultValidateCacheTTL = 30 * time.Second
    defaultValidateTimeout  = 2 * time.Second
    defaultLoginTimeout     = 5 * time.Second
    defaultRefreshTimeout   = 5 * time.Second

    keepaliveTime    = 30 * time.Second
    keepaliveTimeout = 10 * time.Second
)

// retryServiceConfig configures gRPC retry policy ONLY for ValidateToken.
// RefreshToken is intentionally excluded — retrying a refresh that succeeded
// at AUTH but lost the response would trigger family detection and burn the
// user's refresh chain (REQ-001-ADR §R11).
//
// maxAttempts=4 means 1 initial + 3 retries, exponential backoff 100ms→1s.
const retryServiceConfig = `{
  "methodConfig": [{
    "name": [{"service": "auth.v1.AuthService", "method": "ValidateToken"}],
    "retryPolicy": {
      "maxAttempts": 4,
      "initialBackoff": "0.1s",
      "maxBackoff": "1s",
      "backoffMultiplier": 2,
      "retryableStatusCodes": ["UNAVAILABLE"]
    }
  }]
}`

// ─── Types ─────────────────────────────────────────────────────────────────

// FailMode controls how HTTPMiddleware/UnaryInterceptor (TASK-003) react when
// the AUTH service is unavailable.
type FailMode uint8

const (
    // FailClosed (default) blocks the request with 503 if AUTH is unreachable.
    // This is the OWASP-recommended behavior (A07 Authentication Failures).
    FailClosed FailMode = iota

    // FailOpen passes the request through with an empty User in context.
    // Use ONLY for non-critical endpoints (health checks, public probes).
    // SECURITY WARNING: misuse leads to auth bypass.
    FailOpen
)

// Config bundles the constructor parameters.
//
// Endpoint is the only required field. All others have sensible defaults
// (see New).
type Config struct {
    Endpoint         string
    ValidateCacheTTL time.Duration
    FailMode         FailMode
    ValidateTimeout  time.Duration
    LoginTimeout     time.Duration
    RefreshTimeout   time.Duration
    DialOptions      []grpc.DialOption
    Logger           Logger
    Cache            Cache // populated by TASK-003 (typed here as interface placeholder)
}

// Cache is the interface for ValidateToken result caching. The actual default
// implementation lives in lib/internal/cache (TASK-003). For TASK-002 the
// type is declared here so Config compiles; the field defaults to nil.
type Cache interface {
    // Get returns the cached User and ok=true on hit.
    Get(ctx context.Context, key string) (user any, ok bool)
    // Set stores user with the given TTL.
    Set(ctx context.Context, key string, user any, ttl time.Duration)
    // Delete removes the key.
    Delete(ctx context.Context, key string)
}

// Option is a functional option for New.
type Option func(*Client)

// WithFailMode overrides Config.FailMode.
//
// SECURITY WARNING: WithFailMode(FailOpen) disables auth on AUTH-unavailable.
// Use only for endpoints where this is acceptable.
func WithFailMode(mode FailMode) Option {
    return func(c *Client) { c.failMode = mode }
}

// WithLogger overrides Config.Logger. nil collapses to NoopLogger{}.
func WithLogger(l Logger) Option {
    return func(c *Client) {
        if l == nil {
            l = NoopLogger{}
        }
        c.logger = l
    }
}

// WithCache overrides Config.Cache. Used in TASK-003.
func WithCache(cache Cache) Option {
    return func(c *Client) { c.cfg.Cache = cache }
}

// WithOpenMethods registers gRPC methods (FullMethod, e.g.
// "/myapp.v1.UserService/Login") that bypass the UnaryInterceptor (TASK-003).
//
// Default whitelist includes /grpc.health.v1.Health/Check and /Watch.
func WithOpenMethods(methods ...string) Option {
    return func(c *Client) {
        for _, m := range methods {
            c.openMethods[m] = struct{}{}
        }
    }
}

// Client is the SDK entry point. Create with New, dispose with Close.
//
// Safe for concurrent use by multiple goroutines (the underlying
// *grpc.ClientConn is goroutine-safe per gRPC concurrency guarantees).
type Client struct {
    cfg         Config
    grpcConn    *grpc.ClientConn
    authClient  auth_v1.AuthServiceClient
    logger      Logger
    failMode    FailMode
    openMethods map[string]struct{}

    cancelBG  context.CancelFunc
    closeOnce sync.Once
    closeErr  error
}

// ─── Constructor ───────────────────────────────────────────────────────────

// New creates a Client. Validates Endpoint, applies defaults, sets up the
// gRPC connection (lazy — does NOT block on dial), and registers default
// open methods for the interceptor.
//
// The returned Client must be closed with Close to release the gRPC
// connection and stop background goroutines.
func New(cfg Config, opts ...Option) (*Client, error) {
    if cfg.Endpoint == "" {
        return nil, errors.New("authclient: Config.Endpoint is required")
    }

    cfg = applyDefaults(cfg)

    // Default dial options come FIRST, user opts SECOND — user can override
    // any (e.g. WithTransportCredentials for TLS).
    dialOpts := []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:                keepaliveTime,
            Timeout:             keepaliveTimeout,
            PermitWithoutStream: true,
        }),
        grpc.WithDefaultServiceConfig(retryServiceConfig),
    }
    dialOpts = append(dialOpts, cfg.DialOptions...)

    conn, err := grpc.NewClient(cfg.Endpoint, dialOpts...)
    if err != nil {
        return nil, fmt.Errorf("authclient: grpc dial %q: %w", cfg.Endpoint, err)
    }

    bgCtx, cancel := context.WithCancel(context.Background())
    _ = bgCtx // reserved for cache cleaner goroutine — TASK-003

    c := &Client{
        cfg:         cfg,
        grpcConn:    conn,
        authClient:  auth_v1.NewAuthServiceClient(conn),
        logger:      cfg.Logger,
        failMode:    cfg.FailMode,
        openMethods: defaultOpenMethods(),
        cancelBG:    cancel,
    }

    for _, opt := range opts {
        opt(c)
    }

    return c, nil
}

// applyDefaults fills in zero-value Config fields with SDK defaults.
// Negative timeouts/TTLs are treated as zero (i.e. defaulted), since
// negative values have no sensible interpretation.
func applyDefaults(cfg Config) Config {
    if cfg.ValidateCacheTTL < 0 {
        cfg.ValidateCacheTTL = 0 // treat as "use default"
    }
    if cfg.ValidateCacheTTL == 0 {
        cfg.ValidateCacheTTL = defaultValidateCacheTTL
    }
    if cfg.ValidateTimeout <= 0 {
        cfg.ValidateTimeout = defaultValidateTimeout
    }
    if cfg.LoginTimeout <= 0 {
        cfg.LoginTimeout = defaultLoginTimeout
    }
    if cfg.RefreshTimeout <= 0 {
        cfg.RefreshTimeout = defaultRefreshTimeout
    }
    if cfg.Logger == nil {
        cfg.Logger = NoopLogger{}
    }
    // FailMode zero-value is FailClosed — intentional, no override needed.
    // Cache zero-value (nil) is correct — TASK-003 wires the default.
    return cfg
}

// defaultOpenMethods is the always-open registry: health checks bypass auth.
func defaultOpenMethods() map[string]struct{} {
    return map[string]struct{}{
        "/grpc.health.v1.Health/Check": {},
        "/grpc.health.v1.Health/Watch": {},
    }
}

// ─── Close ─────────────────────────────────────────────────────────────────

// Close shuts down background goroutines and closes the underlying gRPC
// connection. Idempotent — subsequent calls return the same error from the
// first call.
//
// Should be called via `defer c.Close()` after a successful New.
func (c *Client) Close() error {
    c.closeOnce.Do(func() {
        c.cancelBG()
        c.closeErr = c.grpcConn.Close()
    })
    return c.closeErr
}
```

**Critical notes**:
- `Cache` interface объявлен ЗДЕСЬ (с `user any`), а не в TASK-003. Причина: `Config.Cache Cache` нужен компилируемому коду в TASK-002. В TASK-003 интерфейс будет уточнён (заменит `any` на `User` — но `User` тоже не существует в TASK-002). Альтернатива — отложить поле Cache до TASK-003 (вырезать из Config), но это breaking change в API. **Решение**: оставить с `any`, в TASK-003 при добавлении `User` — refine signature (это будет minor evolution before v0.1.0 tag, не breaking change).
- `_ = bgCtx` — линтер может ругаться. Если ругается — добавить в TASK-002 простую заглушку: `go func() { <-bgCtx.Done() }()` (no-op горутина, завершается при Close). Это валидирует EC-0010/EC-0012 уже в TASK-002. **Лучше**: НЕ запускать заглушку. Линтер `unused` обычно не ругается на присваивания. Если ругается — в `applyDefaults` или `New` сохранять bgCtx в поле Client'а: `c.bgCtx = bgCtx`. Это пригодится TASK-003 — там goroutine возьмёт `c.bgCtx`.

  **Принимаем**: добавить поле `bgCtx context.Context` в `Client` struct + сохранять в `New`. Тогда `_ = bgCtx` не нужен.

**Update Client struct**: добавить поле `bgCtx context.Context`.

---

### Task 6: CREATE lib/errors_test.go

**Action**: CREATE
**File**: [lib/errors_test.go](../../lib/errors_test.go)

**Description**: Table-driven тесты для `mapGRPCError`, `mapErrorReason`, `RetryAfter`, `AuthError.Unwrap/Error`, `statusFromWrapped`. UT-0001/UT-0011/UT-0040.

**Required Test Cases**:

**`TestMapGRPCError`**:
- [ ] `happy_path` (Unauthenticated → ErrInvalidToken)
- [ ] `permission_denied_returns_blocked`
- [ ] `resource_exhausted_returns_rate_limited_with_zero_retry_after`
- [ ] `unavailable_returns_unavailable`
- [ ] `deadline_exceeded_returns_unavailable`
- [ ] `unknown_grpc_code_returns_wrapped_unavailable` (e.g. `codes.Internal`)
- [ ] `non_grpc_error_returns_wrapped_unavailable`
- [ ] `wrapped_grpc_error_unwraps_correctly` (`fmt.Errorf("ctx: %w", status.Error(...))`)
- [ ] `nil_error_returns_nil`

**`TestMapErrorReason`**:
- [ ] `happy_path` (expired → ErrExpired)
- [ ] `reason_blocked_returns_blocked`
- [ ] `reason_invalid_returns_invalid_token`
- [ ] `reason_empty_returns_invalid_token`
- [ ] `reason_unknown_returns_invalid_token`

**`TestRetryAfter`**:
- [ ] `happy_path` (AuthError with RetryAfter=5s → 5s)
- [ ] `non_auth_error_returns_zero`
- [ ] `nil_returns_zero`
- [ ] `wrapped_auth_error_returns_value` (`fmt.Errorf("ctx: %w", &AuthError{...})`)

**`TestAuthError_Unwrap`**:
- [ ] `happy_path` (errors.Is для ErrRateLimited)
- [ ] `nil_receiver_returns_nil`

**`TestAuthError_Error`**:
- [ ] `happy_path` (Error() возвращает Err.Error())
- [ ] `nil_receiver_returns_empty_string`
- [ ] `nil_inner_err_returns_empty_string`

**`TestStatusFromWrapped`**:
- [ ] `happy_path` (`status.Error(codes.Unauthenticated, ...)` → Status, true)
- [ ] `wrapped_grpc_error` (`fmt.Errorf("ctx: %w", grpcErr)` → Status, true)
- [ ] `double_wrapped` (`fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", grpcErr))` → Status, true)
- [ ] `non_grpc_error` (`errors.New("plain")` → nil, false)
- [ ] `nil_returns_nil_false`

**Code Pattern** (фрагмент):

```go
package authclient

import (
    "errors"
    "fmt"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func TestMapGRPCError(t *testing.T) {
    t.Parallel()

    tests := map[string]struct {
        in            error
        wantSentinel  error // for errors.Is check; nil means "input is nil"
        wantNil       bool
        wantTypedAuth bool // true if result should be *AuthError
    }{
        "happy_path": {
            in:           status.Error(codes.Unauthenticated, "no token"),
            wantSentinel: ErrInvalidToken,
        },
        "permission_denied_returns_blocked": {
            in:           status.Error(codes.PermissionDenied, "blocked"),
            wantSentinel: ErrBlocked,
        },
        "resource_exhausted_returns_rate_limited_with_zero_retry_after": {
            in:            status.Error(codes.ResourceExhausted, "slow down"),
            wantSentinel:  ErrRateLimited,
            wantTypedAuth: true,
        },
        "unavailable_returns_unavailable": {
            in:           status.Error(codes.Unavailable, "down"),
            wantSentinel: ErrUnavailable,
        },
        "deadline_exceeded_returns_unavailable": {
            in:           status.Error(codes.DeadlineExceeded, "timeout"),
            wantSentinel: ErrUnavailable,
        },
        "unknown_grpc_code_returns_wrapped_unavailable": {
            in:           status.Error(codes.Internal, "boom"),
            wantSentinel: ErrUnavailable,
        },
        "non_grpc_error_returns_wrapped_unavailable": {
            in:           errors.New("plain error"),
            wantSentinel: ErrUnavailable,
        },
        "wrapped_grpc_error_unwraps_correctly": {
            in:           fmt.Errorf("context: %w", status.Error(codes.Unauthenticated, "bad")),
            wantSentinel: ErrInvalidToken,
        },
        "nil_error_returns_nil": {
            in:      nil,
            wantNil: true,
        },
    }

    for name, tc := range tests {
        tc := tc
        t.Run(name, func(t *testing.T) {
            t.Parallel()

            got := mapGRPCError(tc.in)

            if tc.wantNil {
                require.NoError(t, got)
                return
            }
            require.Error(t, got)
            require.True(t, errors.Is(got, tc.wantSentinel),
                "expected errors.Is(_, %v) true, got %v", tc.wantSentinel, got)

            if tc.wantTypedAuth {
                var ae *AuthError
                assert.True(t, errors.As(got, &ae), "expected *AuthError")
                assert.Equal(t, time.Duration(0), RetryAfter(got))
            }
        })
    }
}
```

(Аналогично остальные тесты — структура та же, table-driven с t.Parallel().)

**Note on UT-0028/UT-0029**: тесты НЕ используют helper-функции для построения данных. Все ошибки конструируются прямо в табличных entry'ях через `status.Error(...)` и `fmt.Errorf("...: %w", ...)`.

**Note on UT-0020**: моки в этом тесте НЕ нужны — все функции чистые (in → out без зависимостей).

---

### Task 7: CREATE lib/logger_test.go

**Action**: CREATE
**File**: [lib/logger_test.go](../../lib/logger_test.go)

**Description**: Smoke-тест на `NoopLogger` (no-panic с граничными вход­ными).

**Required Test Cases**:

**`TestNoopLogger`** (одна функция, table-driven по методам):
- [ ] `info_no_panic_with_nil_ctx`
- [ ] `info_no_panic_with_empty_fields`
- [ ] `warn_no_panic_with_nil_ctx`
- [ ] `error_no_panic_with_nil_err`
- [ ] `error_no_panic_with_nil_ctx_and_nil_err`
- [ ] `compile_time_interface_check` (можно ассертом `var _ Logger = NoopLogger{}` в production-коде, тест проверяет инстанцирование)

**Code Pattern**:

```go
package authclient

import (
    "context"
    "errors"
    "testing"
)

func TestNoopLogger(t *testing.T) {
    t.Parallel()

    tests := map[string]struct {
        run func(l Logger)
    }{
        "happy_path_info_with_ctx_and_fields": {
            run: func(l Logger) { l.Info(context.Background(), "msg", "k", "v") },
        },
        "info_no_panic_with_nil_ctx": {
            run: func(l Logger) { l.Info(nil, "msg") }, //nolint:staticcheck // testing nil-ctx tolerance
        },
        "warn_no_panic_with_empty_fields": {
            run: func(l Logger) { l.Warn(context.Background(), "msg") },
        },
        "error_no_panic_with_nil_err": {
            run: func(l Logger) { l.Error(context.Background(), "msg", nil) },
        },
        "error_no_panic_with_nil_ctx_and_nil_err": {
            run: func(l Logger) { l.Error(nil, "msg", nil) }, //nolint:staticcheck
        },
        "error_with_real_err_no_panic": {
            run: func(l Logger) { l.Error(context.Background(), "boom", errors.New("x"), "k", "v") },
        },
    }

    for name, tc := range tests {
        tc := tc
        t.Run(name, func(t *testing.T) {
            t.Parallel()
            // The test PASSES iff the call does not panic.
            // No assertions needed — NoopLogger is silent by design.
            tc.run(NoopLogger{})
        })
    }
}
```

**Critical**:
- ES-0171 запрещает `//nolint`. Но `staticcheck` поднимет ругань на `Info(nil, "msg")` (passing nil context). Альтернатива — использовать `var nilCtx context.Context` и передавать. Это валидно по типу и не вызывает staticcheck warning:

```go
"info_no_panic_with_nil_ctx": {
    run: func(l Logger) {
        var nilCtx context.Context
        l.Info(nilCtx, "msg")
    },
},
```

**Применить эту замену** — никаких `//nolint`.

---

### Task 8: CREATE lib/client_test.go

**Action**: CREATE
**File**: [lib/client_test.go](../../lib/client_test.go)

**Description**: Тесты `New` (валидация Config + дефолты + опции) и `Close` (идемпотентность).

**Required Test Cases**:

**`TestNew`** (table-driven):
- [ ] `happy_path` (`Config{Endpoint: "localhost:50051"}` → no error, client.cfg.ValidateCacheTTL == 30s)
- [ ] `missing_endpoint_returns_error` (`Config{}` → error содержит "Endpoint")
- [ ] `defaults_applied` (`Config{Endpoint: "x:1"}` → все default timeouts)
- [ ] `negative_ttl_normalized_to_default`
- [ ] `with_fail_mode_open` (`New(cfg, WithFailMode(FailOpen))` → c.failMode == FailOpen)
- [ ] `with_logger_nil_uses_noop` (`WithLogger(nil)` → не паника, logger == NoopLogger{})
- [ ] `with_logger_custom` (`WithLogger(custom)` → logger == custom)
- [ ] `with_open_methods_appends_to_default`
- [ ] `endpoint_invalid_returns_dial_error_or_lazy` — endpoint `"bad host with spaces"`. NB: `grpc.NewClient` иногда валидирует target, иногда лениво. Если возвращает err → ассерт; если nil → пропустить.

**`TestClient_Close`**:
- [ ] `happy_path` (Close один раз → no error для valid endpoint)
- [ ] `close_is_idempotent` (Close дважды → один и тот же error, без паники)
- [ ] `close_cancels_background_context` (после Close → c.bgCtx.Done() закрыт)

**Code Pattern**:

```go
package authclient

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

type recordingLogger struct {
    infos int
}

func (r *recordingLogger) Info(context.Context, string, ...any)         { r.infos++ }
func (r *recordingLogger) Warn(context.Context, string, ...any)         {}
func (r *recordingLogger) Error(context.Context, string, error, ...any) {}

func TestNew(t *testing.T) {
    t.Parallel()

    tests := map[string]struct {
        cfg     Config
        opts    []Option
        wantErr string // substring; empty → no error
        check   func(t *testing.T, c *Client)
    }{
        "happy_path": {
            cfg: Config{Endpoint: "localhost:50051"},
            check: func(t *testing.T, c *Client) {
                require.NotNil(t, c)
                assert.Equal(t, 30*time.Second, c.cfg.ValidateCacheTTL)
                assert.Equal(t, FailClosed, c.failMode)
                assert.NotNil(t, c.logger)
                _, ok := c.logger.(NoopLogger)
                assert.True(t, ok, "default logger should be NoopLogger")
            },
        },
        "missing_endpoint_returns_error": {
            cfg:     Config{},
            wantErr: "Endpoint",
        },
        "negative_ttl_normalized_to_default": {
            cfg: Config{Endpoint: "x:1", ValidateCacheTTL: -1 * time.Second},
            check: func(t *testing.T, c *Client) {
                assert.Equal(t, 30*time.Second, c.cfg.ValidateCacheTTL)
            },
        },
        "with_fail_mode_open": {
            cfg:  Config{Endpoint: "x:1"},
            opts: []Option{WithFailMode(FailOpen)},
            check: func(t *testing.T, c *Client) {
                assert.Equal(t, FailOpen, c.failMode)
            },
        },
        "with_logger_nil_uses_noop": {
            cfg:  Config{Endpoint: "x:1"},
            opts: []Option{WithLogger(nil)},
            check: func(t *testing.T, c *Client) {
                _, ok := c.logger.(NoopLogger)
                assert.True(t, ok)
            },
        },
        "with_logger_custom": {
            cfg:  Config{Endpoint: "x:1"},
            opts: []Option{WithLogger(&recordingLogger{})},
            check: func(t *testing.T, c *Client) {
                _, ok := c.logger.(*recordingLogger)
                assert.True(t, ok)
            },
        },
        "with_open_methods_appends": {
            cfg:  Config{Endpoint: "x:1"},
            opts: []Option{WithOpenMethods("/x.v1.X/Login", "/x.v1.X/Register")},
            check: func(t *testing.T, c *Client) {
                _, ok := c.openMethods["/x.v1.X/Login"]
                assert.True(t, ok)
                _, ok = c.openMethods["/grpc.health.v1.Health/Check"]
                assert.True(t, ok, "default methods preserved")
            },
        },
    }

    for name, tc := range tests {
        tc := tc
        t.Run(name, func(t *testing.T) {
            t.Parallel()

            c, err := New(tc.cfg, tc.opts...)

            if tc.wantErr != "" {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tc.wantErr)
                assert.Nil(t, c)
                return
            }
            require.NoError(t, err)
            require.NotNil(t, c)
            defer func() { _ = c.Close() }()

            if tc.check != nil {
                tc.check(t, c)
            }
        })
    }
}

func TestClient_Close(t *testing.T) {
    t.Parallel()

    tests := map[string]struct {
        run func(t *testing.T)
    }{
        "happy_path": {
            run: func(t *testing.T) {
                c, err := New(Config{Endpoint: "localhost:50051"})
                require.NoError(t, err)
                require.NoError(t, c.Close())
            },
        },
        "close_is_idempotent": {
            run: func(t *testing.T) {
                c, err := New(Config{Endpoint: "localhost:50051"})
                require.NoError(t, err)
                err1 := c.Close()
                err2 := c.Close()
                err3 := c.Close()
                assert.Equal(t, err1, err2)
                assert.Equal(t, err2, err3)
            },
        },
        "close_cancels_background_context": {
            run: func(t *testing.T) {
                c, err := New(Config{Endpoint: "localhost:50051"})
                require.NoError(t, err)
                require.NoError(t, c.Close())
                select {
                case <-c.bgCtx.Done():
                    // ok
                case <-time.After(100 * time.Millisecond):
                    t.Fatal("bgCtx not cancelled after Close")
                }
            },
        },
    }

    for name, tc := range tests {
        tc := tc
        t.Run(name, func(t *testing.T) {
            t.Parallel()
            tc.run(t)
        })
    }
}
```

**Critical**:
- `recordingLogger` — это локальный stub для проверки `WithLogger`. По UT-0028 helper'ы для test data запрещены. Но stub-структура — это **не helper-функция**, это просто тип. Это allowed. Стоит зафиксировать в комментарии теста.
- `_ = c.Close()` использовать defer — иначе ES-0060 (no `_ =` для error). **Альтернатива (правильнее)**: `t.Cleanup(func() { _ = c.Close() })`. Это идиоматично для `*testing.T`. Но `_ = c.Close()` для cleanup-action — допустимо, если `errors.Join` неуместен (мы в тесте, и ошибка Close в тестах нас не интересует). Лучше использовать `t.Cleanup`:

```go
t.Cleanup(func() {
    if err := c.Close(); err != nil {
        t.Logf("close error: %v", err)
    }
})
```

Применить этот паттерн.

---

## Integration

### Database

N/A — SDK без БД.

### Config

N/A — SDK не имеет env-config (Consumer-side).

### Dependencies

**Add to `go.mod`** (если TASK-001 не закрыт):

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/webdelo/auth-proto/go` | `v0.1.0` | gRPC контракт `auth.v1` (типы AuthServiceClient, ValidateTokenRequest, etc.) |
| `google.golang.org/grpc` | latest stable | gRPC framework (NewClient, ClientConn, codes, status, keepalive, credentials/insecure) |
| `github.com/stretchr/testify` | latest stable | assert/require (для тестов — обычно уже в проекте scratch-default) |

**Команды**:

```bash
go get github.com/webdelo/auth-proto/go@v0.1.0
go get google.golang.org/grpc
go get github.com/stretchr/testify
go mod tidy
```

**Запрещено добавлять в TASK-002**: `golang.org/x/sync` (для singleflight — TASK-004), `slog`/`zap`/`logrus` (adapter pattern, consumer's choice), `go.uber.org/fx` (no DI), `prometheus/client_golang` (no metrics).

---

## Testing

### Unit Tests

| Entity | Test File | Cases |
|--------|-----------|-------|
| `mapGRPCError` | [errors_test.go](../../lib/errors_test.go) | 9 cases (см. Task 6) |
| `mapErrorReason` | [errors_test.go](../../lib/errors_test.go) | 5 cases |
| `RetryAfter` | [errors_test.go](../../lib/errors_test.go) | 4 cases |
| `AuthError.Unwrap`/`Error` | [errors_test.go](../../lib/errors_test.go) | 5 cases |
| `statusFromWrapped` | [errors_test.go](../../lib/errors_test.go) | 5 cases |
| `NoopLogger` | [logger_test.go](../../lib/logger_test.go) | 6 cases (one test, table-driven по методам) |
| `New` | [client_test.go](../../lib/client_test.go) | 7+ cases |
| `Client.Close` | [client_test.go](../../lib/client_test.go) | 3 cases |

**Coverage target**: ≥ 85% для `lib/client.go` и `lib/errors.go` (per TASK-002 DOD).

**Mocks**: НЕ требуются. Все функции в TASK-002 чистые (errors.go) или используют lazy gRPC dial (client.go). UT-0020 (mocks ONLY via mockery) не применим.

### Integration Tests

**N/A для TASK-002.** Integration-тесты против реального AUTH server покрывают `ValidateAccessToken`, `Login`, etc. — это TASK-003/TASK-004. В TASK-002 нет ни одного RPC-метода.

---

## Validation

### Build & Lint

```bash
# 1. Компиляция
go build ./...

# 2. Линтер (использует .golangci.yaml — НЕ менять config, ES-0171/CS-0030)
make lint

# 3. Тесты с race detector (UT-0040, EC-0090)
make test
make test-race

# 4. Покрытие (DOD требует ≥ 85% по client.go и errors.go)
make coverage

# 5. Полный audit (vet + lint + test)
make audit

# 6. Godoc — public API без TODO
go doc github.com/webdelo/auth-client-go/lib | head -80

# 7. Идемпотентность Close под -race -count
go test -race -run TestClient_Close -count=20 ./lib/...
```

**Все 7 шагов должны пройти зелёным.**

### Manual Checks

1. **Package name**: после задач 1–8 — `head -1 lib/*.go | grep "package"` → все `package authclient`. Нет ни одного `package lib`.
2. **API surface**: `go doc github.com/webdelo/auth-client-go/lib | grep -E "^(type|func|var)"` показывает:
   - `var ErrInvalidToken/ErrExpired/ErrBlocked/ErrRateLimited/ErrUnavailable`
   - `type AuthError struct { Err error; RetryAfter time.Duration }`
   - `type Cache interface`
   - `type Client struct`
   - `type Config struct`
   - `type FailMode uint8`
   - `type Logger interface`
   - `type NoopLogger struct`
   - `type Option func(*Client)`
   - `func New(cfg Config, opts ...Option) (*Client, error)`
   - `func RetryAfter(err error) time.Duration`
   - `func WithFailMode/WithLogger/WithCache/WithOpenMethods`
   - `const FailClosed/FailOpen`
3. **No deprecated grpc.Dial**: `grep -r "grpc.Dial" lib/` → пусто. Только `grpc.NewClient`.
4. **No `nolint`**: `grep -rn "nolint" lib/` → пусто (ES-0171).
5. **Retry config has method-level scope**: `grep -A5 "retryServiceConfig" lib/client.go | grep "ValidateToken"` → найдено.

---

## DOD (Definition of Done)

Копия из [REQ-001-TASK-002.md](../tasks/REQ-001-TASK-002.md) — все пункты должны быть выполнены:

- [ ] `lib/client.go` содержит публичные типы `Config`, `FailMode`, `Option`, `Client`, функции `New`, `WithFailMode`, `WithLogger`, `WithCache`, `WithOpenMethods` и метод `(*Client).Close`.
- [ ] `New(Config{Endpoint: "auth:50051"})` возвращает рабочий `*Client` без ошибки; `New(Config{})` возвращает ошибку про обязательный Endpoint.
- [ ] `(*Client).Close()` идемпотентен (повторный вызов не паникует и возвращает первую ошибку).
- [ ] `lib/errors.go` экспортирует 5 sentinel-ошибок + `AuthError` + `RetryAfter`. `errors.Is(mapGRPCError(grpcErr), ErrXxx)` корректно для всех gRPC-кодов из таблицы маппинга.
- [ ] `statusFromWrapped` извлекает gRPC status из ошибки, обёрнутой в `fmt.Errorf("...: %w", grpcErr)`.
- [ ] `lib/logger.go` содержит `Logger` interface и `NoopLogger`. `NoopLogger{}.Info/Warn/Error` не паникует с nil arguments.
- [ ] gRPC dial использует `grpc.NewClient` с keepalive(30s, 10s, true) и retry policy для UNAVAILABLE на `ValidateToken`; `RefreshToken` НЕ ретраится.
- [ ] Unit-тесты покрывают ≥ 85% по `client.go` и `errors.go` (`make coverage`).
- [ ] `make audit` (vet + lint + test) проходит чисто.
- [ ] `go doc github.com/webdelo/auth-client-go/lib` показывает все exported типы без TODO в комментариях.

Дополнительно (контроль качества PRP):

- [ ] Package name: все файлы `package authclient`.
- [ ] Старые placeholder `lib/lib.go` + `lib/lib_test.go` удалены.
- [ ] `lib/examples/basic/main.go` обновлён на stub (полный пример — TASK-006).
- [ ] Нет deprecated `grpc.Dial` в коде (ADR §A15).
- [ ] Нет `//nolint` директив (ES-0171).
- [ ] Все exported идентификаторы имеют godoc-комментарии (CS-0060).

---

## Score Self-Assessment: 9/10

| Критерий | Оценка | Заметка |
|----------|--------|---------|
| Necessary context included | 10/10 | TASK + ADR (псевдокод, §A1/A6/A7) + code-research отчёт (`statusFromWrapped`, retry config, sync.Once) + 19 правил ES/EC/CS/UT |
| Validation gates executable | 10/10 | 7 команд (build, lint, test, test-race, coverage, audit, godoc) + 5 manual checks |
| References existing patterns | 10/10 | Прямые ссылки `file:line` на signal-analyzer для `statusFromWrapped`, retry policy, sync.Once-Close. Псевдокод адаптирован дословно. |
| Clear implementation path | 9/10 | 9 tasks (включая pre-requisite Task 0). Каждый task имеет CREATE/DELETE/MODIFY + детальный code pattern. Минус 1 балл: `_ = bgCtx` обсуждение в Task 5 могло быть короче. |
| Risks and mitigations | 10/10 | 13 gotcha'ев с конкретными решениями (lazy dial, keepalive bump, status.Unwrap, retry scope, package rename, etc.) |
| Test coverage plan | 9/10 | Полная разбивка по case'ам (43+ case'а на 7 функций). Минус 1: integration-тесты невозможны в TASK-002 (нет RPC методов). |

**Score: 9/10.** Минус 1 балл — `Cache` interface в Config зависит от `any` placeholder, который будет refined в TASK-003 (потенциальный API noise перед v0.1.0 tag). Принято осознанно — альтернатива (вырезать `Cache` из Config до TASK-003) хуже, потому что v0.1.0 release content без Cache в Config — крупное breaking change в момент TASK-003.

---

## Ссылки

- **Parent ADR**: [REQ-001-ADR.md](./REQ-001-ADR.md) — §A1 (API surface), §A6 (gRPC connection), §A7 (typed errors + mapping table), §A15 (no deprecated)
- **Code Research**: [2026-05-15_code-research_REQ-001-TASK-002.md](../../.memory_bank/agent_task_results/2026-05-15_code-research_REQ-001-TASK-002.md)
- **Rules**: [.temp/rules/REQ-001-TASK-002/cheatsheet.md](../../.temp/rules/REQ-001-TASK-002/cheatsheet.md), [coding_scope.md](../../.temp/rules/REQ-001-TASK-002/coding_scope.md), [test_scope.md](../../.temp/rules/REQ-001-TASK-002/test_scope.md)
- **Source patterns** (для code-writer):
  - `statusFromWrapped` — `signal-analyzer/internal/usecase/bff/error_mapping.go:38-47`
  - Retry config JSON — `signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/retry_config.go:4-15`
  - `grpc.NewClient` пример — `signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/signal_analyzer.go:46-56`
  - `sync.Once`-Close — `signal-analyzer/internal/infrastructure/export/engine.go:103, 231`
  - Table-driven mapper test — `signal-analyzer/internal/usecase/bff/error_mapping_test.go:15-39`
- **Proto**: `~/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go:26` — `AuthService_ValidateToken_FullMethodName`
