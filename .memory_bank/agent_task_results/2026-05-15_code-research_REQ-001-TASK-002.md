# Code Research Report — REQ-001 TASK-002: Core Client + Errors + Logger

**Date:** 2026-05-15
**Researcher:** code-researcher (claude-sonnet-4-6)
**Task:** REQ-001-TASK-002 — lib/client.go, lib/errors.go, lib/logger.go

---

## 1. Где смотрел

| Репо / путь | Что искал |
|-------------|-----------|
| `/Users/a.popov/Development/TRADING/bots/auth-client-go/task_management/tasks/REQ-001-TASK-002.md` | Требования TASK-002 |
| `/Users/a.popov/Development/TRADING/bots/auth-client-go/task_management/plans/REQ-001-ADR.md` | Псевдокод `lib/api.go` (стр. 544–699), §A1, §A6, §A7, §A11, §A15 |
| `/Users/a.popov/Development/TRADING/bots/auth-client-go/.memory_bank/agent_task_results/2026-05-15_code-research_codebase_analysis_REQ-001-ADR.yaml` | Результат предыдущего research (использован как отправная точка) |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping.go` | `statusFromWrapped` + `MapGRPCError` реализация |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping_test.go` | Table-driven тест для gRPC маппинга |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/handlers/grpc/signal/interceptors.go` | `NewRecoveryInterceptor` с named return + defer recover |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/signal_analyzer.go` | `grpc.NewClient` + `WithDefaultServiceConfig` + `WithChainUnaryInterceptor` |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/retry_config.go` | JSON retry policy (`maxAttempts`, `retryableStatusCodes`) |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/clients.go` | `grpc.NewClient` без keepalive (чистый minimal вариант) |
| `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/export/engine.go` | `sync.Once closeOnce` для идемпотентного Close |
| `/Users/a.popov/Development/GOLANG/fundingbot/internal/grpc/clients/grpc_funding_rates_screener_client/screener_client.go` | `sync.RWMutex` + `isActive bool` + keepalive + Close-через-channel |
| `/Users/a.popov/Development/GOLANG/fundingbot/internal/grpc/clients/grpc_balance_controller_client/client.go` | `atomic.Bool isActive` + `Swap(true)` guard |
| `/Users/a.popov/Development/TRADING/bots/auth/internal/handlers/grpc/auth/errors.go` | AUTH server gRPC error mapping (что приходит на вход SDK) |
| `/Users/a.popov/Development/TRADING/bots/auth/internal/handlers/grpc/auth/login.go` | `ResourceExhausted` без Retry-After в metadata |
| `/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go` | Поля `ValidateTokenResponse` |
| `/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go` | `AuthServiceClient` interface + метод-константы |
| `/Users/a.popov/Development/TRADING/bots/auth-client-go/go.mod` + `lib/lib.go` | Текущее состояние SDK (пустой) |

---

## 2. Что нашёл (по темам)

### 2.1 Конструктор `New(Config, ...Option) (*Client, error)`

Прямого аналога в sibling-репах нет (все они используют DI/FX и создают `conn` снаружи). Ближайший паттерн в **signal-analyzer** — `grpc.NewClient` с опциями прямо в FX-провайдере:

- `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/signal_analyzer.go:45-58` — `newBFFSAConn` создаёт conn с `grpc.NewClient` + `insecure` + retry config + chain interceptors, возвращает `(*grpc.ClientConn, error)`.
- `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/clients.go:44-60` — `newSTSAConn` — то же самое с `grpc_retry.UnaryClientInterceptor`.

Для SDK `New()` должен делать то, что FX-провайдеры делают в DI. Псевдокод ADR:

```
// ADR §A6 + §A1 (стр. 561-589)
type Config struct {
    Endpoint         string
    ValidateCacheTTL time.Duration   // default 30s
    FailMode         FailMode        // default FailClosed
    ValidateTimeout  time.Duration   // default 2s
    LoginTimeout     time.Duration   // default 5s
    RefreshTimeout   time.Duration   // default 5s
    DialOptions      []grpc.DialOption
    Logger           Logger
    Cache            Cache
}

func New(cfg Config, opts ...Option) (*Client, error) {
    if cfg.Endpoint == "" { return nil, errors.New("authclient: endpoint is required") }
    // apply defaults
    // apply opts
    // grpc.NewClient(cfg.Endpoint, defaultOpts..., cfg.DialOptions...)
    // NewAuthServiceClient(conn)
    // return &Client{...cancelBG...}, nil
}
```

Validation pattern (вернуть error из конструктора на missing required field) — `ES-0142`, явно указано в TASK-002 DOD.

### 2.2 Functional options (`WithXxx`)

ADR §A1 (стр. 579-584) полный список:

```go
type Option func(*Client)
func WithFailMode(mode FailMode) Option { return func(c *Client) { c.cfg.FailMode = mode } }
func WithLogger(l Logger) Option        { return func(c *Client) { c.cfg.Logger = l } }
func WithCache(c Cache) Option          { return func(c *Client) { c.cfg.Cache = c } }
func WithOpenMethods(methods ...string) Option
```

Прямые примеры в sibling-репах не используют этот паттерн (там DI), но паттерн `func(c *Config) {}` применяется в `grpc_retry.WithMax()` / `grpc_retry.WithBackoff()` — `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/clients.go:51-53`.

### 2.3 gRPC dial с keepalive + retry policy

**Лучший эталон: signal-analyzer.**

Retry policy (JSON):

```
/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/retry_config.go:4-15
```

Содержимое:
```json
{
  "methodConfig": [{"name": [{"service": ""}], "retryPolicy": {
    "maxAttempts": 3,
    "initialBackoff": "0.1s",
    "maxBackoff": "1s",
    "backoffMultiplier": 2,
    "retryableStatusCodes": ["UNAVAILABLE"]
  }}]
}
```

Применение через `grpc.WithDefaultServiceConfig(bffRetryServiceConfig)`:

```
/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/signal_analyzer.go:46-56
```

Keepalive используется в **fundingbot**:

```
/Users/a.popov/Development/GOLANG/fundingbot/internal/grpc/clients/grpc_balance_controller_client/client.go:235-241
  keepalive.ClientParameters{
      Time:                c.config.ConnectionPingDuration,
      Timeout:             c.config.ConnectionPingTimeout,
      PermitWithoutStream: c.config.ConnectionPermitWithoutStream,
  }
```

В SDK значения фиксированные (ADR §A6): `Time: 30s, Timeout: 10s, PermitWithoutStream: true`.

**Итоговый вариант для SDK** (комбинация signal-analyzer retry + fundingbot keepalive + `grpc.NewClient`):

```go
grpc.NewClient(
    cfg.Endpoint,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithKeepaliveParams(keepalive.ClientParameters{
        Time: 30 * time.Second, Timeout: 10 * time.Second, PermitWithoutStream: true,
    }),
    grpc.WithDefaultServiceConfig(retryPolicy),  // только для ValidateToken
)
```

**Важно:** ADR §A6 говорит — retry только на `ValidateToken`, не на `RefreshToken`. Но `WithDefaultServiceConfig` применяется ко всем методам, если `name: [{"service": ""}]`. Нужно ограничить `service` или `method`:

```json
{"name": [{"service": "auth.v1.AuthService", "method": "ValidateToken"}], "retryPolicy": {...}}
```

Метод-константы из proto:
```
/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go:26
  AuthService_ValidateToken_FullMethodName = "/auth.v1.AuthService/ValidateToken"
```

Сервис: `auth.v1.AuthService`.

### 2.4 `Close()` идемпотентный (sync.Once)

Лучший эталон найден в signal-analyzer export engine:

```
/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/export/engine.go:103-104
  closeOnce sync.Once

/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/export/engine.go:231-233
  e.closeOnce.Do(func() {
      // close conn, cancel background goroutine
  })
```

Fundingbot использует другой подход: `atomic.Bool` с `Swap(false)` в `Close()`:

```
/Users/a.popov/Development/GOLANG/fundingbot/internal/grpc/clients/grpc_balance_controller_client/client.go:66-81
  if !c.isActive.Swap(false) { return nil }
  if err := c.connection.Close(); err != nil { ... }
```

**Для SDK рекомендован `sync.Once`** (ADR DOD: "идемпотентен через `sync.Once`"):

```go
type Client struct {
    conn      *grpc.ClientConn
    pb        auth_v1.AuthServiceClient
    cancelBG  context.CancelFunc
    closeOnce sync.Once
    closeErr  error
}

func (c *Client) Close() error {
    c.closeOnce.Do(func() {
        c.cancelBG()
        c.closeErr = c.conn.Close()
    })
    return c.closeErr
}
```

Паттерн `closeErr` — стандартный для `sync.Once`-идемпотентного Close, совместимый с DOD: "повторный вызов не паникует и возвращает первую ошибку".

### 2.5 Mapping gRPC status → typed error

**Эталон: signal-analyzer `MapGRPCError` + `statusFromError`.**

```
/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping.go:19-36
```

Mapper возвращает `(httpCode int, message string)` — для SDK нужно адаптировать под `error`.

Таблица маппинга для SDK (из ADR §A7, стр. 206-216):

| gRPC code | SDK error |
|-----------|-----------|
| `Unauthenticated` | `ErrInvalidToken` |
| `PermissionDenied` | `ErrBlocked` |
| `ResourceExhausted` | `&AuthError{Err: ErrRateLimited, RetryAfter: ...}` |
| `Unavailable` / `DeadlineExceeded` | `ErrUnavailable` |
| любой другой | `ErrUnavailable` (с wrap) |

AUTH server подтверждён: возвращает именно эти коды (`errors.go:24-51`).

### 2.6 `statusFromWrapped` через `errors.Unwrap`

**Критический паттерн. Точный код:**

```
/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping.go:38-47
```

```go
func statusFromError(err error) (*status.Status, bool) {
    for e := err; e != nil; e = errors.Unwrap(e) {
        if st, ok := status.FromError(e); ok {
            return st, true
        }
    }
    return nil, false
}
```

В SDK называем `statusFromWrapped` (ADR использует это имя). Логика та же. Нужно потому что SDK может оборачивать ошибки через `fmt.Errorf("...: %w", grpcErr)`, а стандартный `status.FromError` проверяет только переданный аргумент без хождения по chain.

### 2.7 Извлечение `Retry-After` из metadata для `ResourceExhausted`

**AUTH server НЕ отправляет `Retry-After` в gRPC trailer/header.**

Подтверждено в:
```
/Users/a.popov/Development/TRADING/bots/auth/internal/handlers/grpc/auth/login.go:26-30
  allowed, retryAfter := h.deps.Limiter.AllowLogin(...)
  if !allowed {
      h.deps.Log.Info(ctx, "login rate limited", zap.Duration("retry_after", retryAfter))
      return nil, status.Error(codes.ResourceExhausted, msgRateLimited)
  }
```

`retryAfter` используется только для **логгирования**, не для gRPC metadata. То же в `errors.go:36` — plain `status.Error(codes.ResourceExhausted, msgRateLimited)`.

Поиск по `grpc.SetTrailer`, `grpc.SetHeader`, `metadata.New` в `/auth/internal/` — ни одного совпадения для rate-limit контекста.

**Вывод:** В ADR §A7 написано "с извлечением `Retry-After` из metadata, если есть" — это условное "если". Реально AUTH v0.1.0 не кладёт `Retry-After` в gRPC metadata. SDK должен:
1. Реализовать код извлечения через `status.Details()` / trailer metadata — для будущей совместимости.
2. Если поле отсутствует — `RetryAfter` будет `0`, `AuthError.RetryAfter = 0`.

Паттерн извлечения из gRPC trailer нет нигде в sibling-репах. Нужно реализовать с нуля, ориентируясь на gRPC spec. Стандартный подход:

```go
// При ResourceExhausted — попытка достать retry-after из trailer metadata
// grpc.Trailer(&trailer) передаётся как CallOption в ValidateToken RPC
// Но SDK не делает RPC — mapGRPCError получает уже готовый error.
// Поэтому Retry-After нужно доставать из status.Details() или из grpc.Header/Trailer
// переданных через grpc.Trailer(md) в вызывающем коде.
```

Простейшее решение: `mapGRPCError` возвращает `&AuthError{Err: ErrRateLimited, RetryAfter: 0}` всегда при `ResourceExhausted`. Когда AUTH начнёт слать metadata — добавится extraction. До тех пор `RetryAfter(err) == 0`, и middleware просто не добавляет `Retry-After` header в ответ (или ставит fallback 60s).

### 2.8 Logger interface + NoopLogger

ADR §A5 (стр. 157-166) даёт точную сигнатуру:

```go
type Logger interface {
    Info(ctx context.Context, msg string, fields ...any)
    Warn(ctx context.Context, msg string, fields ...any)
    Error(ctx context.Context, msg string, err error, fields ...any)
}

type NoopLogger struct{}
func (NoopLogger) Info(context.Context, string, ...any)         {}
func (NoopLogger) Warn(context.Context, string, ...any)         {}
func (NoopLogger) Error(context.Context, string, error, ...any) {}
```

ADR стр. 689-693 — дословный код для `NoopLogger`.

В signal-analyzer используется `ctxlogger.Logger` (конкретная зависимость):
```
/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/handlers/grpc/signal/interceptors.go:24
  func NewLoggingInterceptor(log ctxlogger.Logger) grpc.UnaryServerInterceptor
```

Для SDK — **адаптер-интерфейс**, не конкретный тип. `ctxlogger.Logger` несовместим (это prod зависимость), берём собственный.

Ключевое отличие от signal-analyzer: `Error` принимает `err error` отдельным аргументом — это ADR-специфичная сигнатура, совместимая со `slog`-парами через `...any`.

### 2.9 Тест-паттерны (table-driven + UT-0001)

**Эталон table-driven для error mapper:**

```
/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping_test.go:15-39
```

```go
func TestMapGRPCError(t *testing.T) {
    t.Parallel()
    tests := map[string]struct {
        err          error
        wantHTTPCode int
        wantMessage  string
    }{
        "happy_path_not_found":       {err: status.Error(codes.NotFound, "not found"), ...},
        "resource_exhausted":         {err: status.Error(codes.ResourceExhausted, "too many"), ...},
        "non_grpc_error_returns_500": {err: errors.New("some error"), ...},
    }
    for name, tc := range tests {
        tc := tc
        t.Run(name, func(t *testing.T) {
            t.Parallel()
            ...
        })
    }
}
```

Адаптация для SDK — заменить `(httpCode, message)` на `error`, добавить `errors.Is(got, tc.wantErr)`.

**Для `mapGRPCError` нужно покрыть:**
- `codes.Unauthenticated` → `errors.Is(err, ErrInvalidToken)`
- `codes.PermissionDenied` → `errors.Is(err, ErrBlocked)`
- `codes.ResourceExhausted` → `errors.Is(err, ErrRateLimited)`
- `codes.Unavailable` → `errors.Is(err, ErrUnavailable)`
- `codes.DeadlineExceeded` → `errors.Is(err, ErrUnavailable)`
- wrapped gRPC error (`fmt.Errorf("context: %w", status.Error(...))`) → корректный маппинг через `statusFromWrapped`
- non-gRPC error → `ErrUnavailable`

**Тест идемпотентного `Close()`:**

```go
func TestClientClose_Idempotent(t *testing.T) {
    c, err := New(Config{Endpoint: "localhost:50051"})  // нет реального сервера — OK для grpc.NewClient (lazy dial)
    require.NoError(t, err)
    err1 := c.Close()
    err2 := c.Close()
    // оба вызова не паникуют; ошибка (если есть) одинаковая
    assert.Equal(t, err1, err2)
}
```

`grpc.NewClient` не делает реальный dial → создание `*Client` без сервера безопасно в тестах.

---

## 3. Чего не нашёл

### 3.1 Извлечение `Retry-After` из gRPC metadata/trailer

Ни в signal-analyzer, ни в fundingbot, ни в auth server — нигде нет паттерна извлечения `retry-after` из gRPC trailer metadata в клиентском коде. AUTH server v0.1.0 не посылает это поле совсем. Нужно реализовать с нуля (либо заглушкой с `RetryAfter: 0`, либо через `grpc.Trailer` + парсинг).

### 3.2 `grpc.WithDefaultServiceConfig` для одного метода

Sibling-репы применяют retry policy на весь сервис (`"name": [{"service": ""}]`). Для SDK нужно применить только на `ValidateToken` (не на `RefreshToken`). Конфигурация через конкретный `service` + `method` в JSON — стандарт gRPC, но готового примера в репах нет. Нужно реализовать с нуля по спеке.

### 3.3 Logger adapter с `err error` как отдельным параметром в `Error()`

В signal-analyzer `Logger.Error(ctx, msg, ...fields)` — без выделенного `err`. ADR требует `Error(ctx, msg, err error, ...any)`. Это чуть другая сигнатура — нет прямого copy/paste. Реализовать по ADR §A5 стр. 163-165.

---

## 4. Рекомендации для PRP

### Что копировать напрямую:

1. **`statusFromWrapped`** (переименование `statusFromError`):
   - Источник: `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping.go:38-47`
   - Копировать как есть, переименовать в `statusFromWrapped`. Это 8 строк, полностью готово.

2. **gRPC retry policy JSON**:
   - Источник: `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/retry_config.go:4-15`
   - Адаптировать: `"name": [{"service": "auth.v1.AuthService", "method": "ValidateToken"}]` + `maxAttempts: 4` (ADR говорит max 4 attempts = 3 retries).

3. **`grpc.NewClient` с chain interceptors**:
   - Источник: `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/signal_analyzer.go:46-56`
   - Паттерн: `grpc.NewClient(addr, grpc.WithTransportCredentials(...), grpc.WithDefaultServiceConfig(...), grpc.WithChainUnaryInterceptor(...))`.

4. **Table-driven тест error mapper**:
   - Источник: `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping_test.go:15-39`
   - Адаптировать: `wantErr error` вместо `wantHTTPCode int`, проверка через `errors.Is`.

5. **Recovery interceptor (named return + defer recover)**:
   - Источник: `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/handlers/grpc/signal/interceptors.go:99-119`
   - Нужен в `TASK-005` (gRPC interceptor), но паттерн стоит зафиксировать сейчас.

6. **`sync.Once` для Close**:
   - Паттерн: `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/export/engine.go:103, 231`
   - Добавить `closeOnce sync.Once` + `closeErr error` в `Client` struct.

### Что реализовать с нуля:

- `mapGRPCError(err error) error` — по таблице ADR §A7.
- `mapErrorReason(reason string) error` — тривиально, switch по строкам.
- `AuthError` struct с `Err`/`Unwrap()`/`Error()`/`RetryAfter`.
- `New(cfg Config, opts ...Option) (*Client, error)` — применить дефолты, вызвать `grpc.NewClient`.
- `Logger` interface + `NoopLogger{}` — по ADR §A5 дословно.
- retry policy JSON — ограничить на `ValidateToken` метод.

### Критические особенности (не очевидны из кода):

1. **AUTH server НЕ кладёт `Retry-After` в gRPC metadata.** Реализовать `AuthError.RetryAfter = 0` по умолчанию. Extraction code написать как best-effort (попытка распарсить `status.Details()` или trailer — если нет, `RetryAfter = 0`).

2. **`grpc.NewClient` не делает реальный dial.** Соединение lazy → тест `New(Config{Endpoint: "localhost:50051"})` без сервера пройдёт без ошибки. Это ожидаемое поведение.

3. **ValidateToken НИКОГДА не возвращает gRPC error для invalid/expired токена** (подтверждено по proto docstring):
   ```
   /Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go:56-58
     // Никогда не возвращает gRPC ошибку — всегда возвращает ValidateTokenResponse
     // с is_valid=true/false. gRPC ошибка означает недоступность auth service.
   ```
   `mapGRPCError` применяется для Login/Refresh/etc. `mapErrorReason` применяется только для `ValidateToken` с `is_valid=false`.

4. **`ValidateTokenResponse` не содержит `Email`:**
   ```
   /Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go:510-513
     IsValid bool, UserUuid string, Role string, ErrorReason string
   ```
   `User.Email` будет `""` после `ValidateAccessToken`. Документировать в godoc.

5. **Retry policy для ValidateToken: `maxAttempts: 4`** (ADR §A6) — это 1 первый + 3 retry = 4 total. В JSON `maxAttempts` — это total (включая первую попытку).

6. **`cancelBG context.CancelFunc` в Client** — задел для будущего cache cleaner горутина (TASK-003). В TASK-002 создаётся и передаётся, `cancelBG()` вызывается в `Close()`. Горутина появится в TASK-003.

---

## Ключевые пути (для code-writer)

| Тема | Файл:строка | Что взять |
|------|-------------|-----------|
| `statusFromWrapped` готовый код | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping.go:38-47` | Копировать, переименовать |
| gRPC retry policy JSON | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/retry_config.go:4-15` | Адаптировать под ValidateToken |
| `grpc.NewClient` + chain + retry | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/fx/shows/fxs_grpc_clients/signal_analyzer.go:46-56` | Структура вызова |
| keepalive.ClientParameters | `/Users/a.popov/Development/GOLANG/fundingbot/internal/grpc/clients/grpc_balance_controller_client/client.go:235-241` | Параметры (подставить 30s/10s/true) |
| `sync.Once closeOnce` | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/infrastructure/export/engine.go:103, 231` | Паттерн поля + Do-блока |
| Table-driven error test | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/usecase/bff/error_mapping_test.go:15-39` | Структура теста |
| Recovery interceptor | `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/handlers/grpc/signal/interceptors.go:99-119` | Named return + defer (TASK-005) |
| AUTH gRPC error codes | `/Users/a.popov/Development/TRADING/bots/auth/internal/handlers/grpc/auth/errors.go:24-51` | Что реально приходит с сервера |
| ValidateTokenResponse fields | `/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go:508-574` | IsValid, UserUuid, Role, ErrorReason |
| AuthServiceClient interface | `/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go:40-100` | Сигнатуры всех 13 методов |
| Logger interface (ADR source) | `task_management/plans/REQ-001-ADR.md:157-165` | Дословный код |
| NoopLogger (ADR source) | `task_management/plans/REQ-001-ADR.md:689-693` | Дословный код |
| Config + Client API (ADR source) | `task_management/plans/REQ-001-ADR.md:561-589` | Полный mockup |
