# REQ-001-TASK-003-PRP: ValidateAccessToken + TTL cache + HTTPMiddleware + UnaryInterceptor

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-001-TASK-003-PRP |
| Parent Request | [REQ-001](../tasks/REQ-001.md) |
| Parent Task | [REQ-001-TASK-003](../tasks/REQ-001-TASK-003.md) |
| Parent ADR | [REQ-001-ADR](./REQ-001-ADR.md) |
| Created | 2026-05-18 |
| Updated | 2026-05-18 |
| Score | 9/10 |

---

## Goal

### Summary

Реализовать hot-path SDK: in-memory TTL-кэш для ValidateToken-ответов, метод `(*Client).ValidateAccessToken`, готовые `HTTPMiddleware` и `UnaryServerInterceptor` для защиты consumer-эндпоинтов. Положительные результаты кэшируются на `cfg.ValidateCacheTTL` (default 30s), negative — никогда. Fail-mode определяет поведение middleware при недоступности AUTH (default `FailClosed` → 503; opt-in `FailOpen` → pass-through с пустым `User`). gRPC interceptor защищён от паники через named-return + `defer recover()` (паттерн из signal-analyzer).

### Why

- **Hot path SDK.** Каждый аутентифицированный запрос consumer'а проходит через `ValidateAccessToken`. Cache hit должен быть `< 1µs` (in-process `sync.Map`), miss — единичный gRPC roundtrip с timeout `cfg.ValidateTimeout`.
- **Security-critical поведение.** Positive-only caching гарантирует, что заблокированный (`BlockUser`) / разлогиненный (`LogoutAll`) пользователь перестаёт ходить максимум через 30s — за счёт TTL-инвалидации `token_version` в AUTH. Cache-key = `hex(sha256(token))` — сам токен в кэше/логах/метриках видеть нельзя.
- **Главная ценность SDK.** Без `HTTPMiddleware` и `UnaryInterceptor` consumer пишет middleware вручную и регулярно ошибается: бесконечный кэш, fail-open by default, забыл `WWW-Authenticate`, неверный код на 401/429/503. SDK закрывает class of bugs централизованно.
- **Lifecycle goroutine.** Background cleanup goroutine `cacheCleaner` подписан на `bgCtx` из `Client.New`. `Close()` через `cancelBG()` гасит её детерминированно (EC-0010 lifecycle, EC-0012 no leak).
- **Защита от паники в gRPC handler.** Без recovery — паника в `next` handler ломает соединение и не оборачивается в `codes.Internal`. SDK делает это за consumer'а.

### What

После выполнения TASK'а consumer пишет:

```go
client, _ := authclient.New(authclient.Config{Endpoint: "auth:50051"})
defer client.Close()

// HTTP — защищённый handler
mux.Handle("/api/me", client.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    user, _ := authclient.UserFromContext(r.Context())
    fmt.Fprintf(w, "hello %s (role=%s)", user.UUID, user.Role)
})))

// gRPC — защищённый сервер
grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(client.UnaryInterceptor()))

// Прямой вызов
user, err := client.ValidateAccessToken(ctx, accessToken)
switch {
case errors.Is(err, authclient.ErrInvalidToken): // 401
case errors.Is(err, authclient.ErrExpired):      // 401
case errors.Is(err, authclient.ErrBlocked):      // 401
case errors.Is(err, authclient.ErrUnavailable):  // 503 / FailOpen
}
```

### Success Criteria

- [ ] `(*Client).ValidateAccessToken(ctx, "")` → `ErrInvalidToken` без gRPC-вызова.
- [ ] Cache hit на `(*Client).ValidateAccessToken` укладывается в `< 1µs/op` (`BenchmarkValidateCache_Hit`).
- [ ] Positive result кэшируется; negative (`is_valid=false`, `ErrUnavailable`, ratelimited) НЕ кэшируется — повторный вызов идёт в gRPC.
- [ ] Cleanup goroutine завершается при `Close()`: `runtime.NumGoroutine()` до `New` == после `Close()` (на стабильной точке после `runtime.Gosched`).
- [ ] `HTTPMiddleware` возвращает: 401 при missing/invalid Bearer, 503 + `Retry-After: 5` при `ErrUnavailable` (FailClosed), pass-through с `User{}` при `ErrUnavailable` (FailOpen), 429 + `Retry-After: N` при `ErrRateLimited`, 200 при success — `WithUser(ctx, user)` в `r.Context()`.
- [ ] `UnaryInterceptor` пропускает методы из `WithOpenMethods` (e.g. `/grpc.health.v1.Health/Check`), возвращает `codes.Unauthenticated` для отсутствия/невалидного токена, `codes.Unavailable`/`codes.ResourceExhausted`/`codes.Internal` для остальных кейсов, защищён от паники handler'а через named return.
- [ ] Cache key — `hex(sha256(token))`, никогда не сам токен (статически — `grep -E "cache.+Set.+token|Get.+token" lib/` не должен возвращать сам raw-токен).
- [ ] Integration-тесты с тегом `//go:build integration`: `TestValidateAccessToken_Happy`, `TestValidateAccessToken_Invalid` проходят против реального AUTH (`make test-integration`). `_Expired` помечен `t.Skip` (требует TTL waiting > 1m — в TASK-007 добавить).
- [ ] Unit coverage по `lib/validate.go` / `lib/middleware_http.go` / `lib/interceptor_grpc.go` / `lib/context.go` / `lib/internal/cache/` ≥ 80% (`make coverage`).
- [ ] `make audit` (vet + lint + test) проходит чисто.
- [ ] `go test -race ./lib/...` проходит без race detection errors.

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | ES-0010, ES-0011, ES-0021, ES-0040, ES-0051, ES-0056, ES-0060, ES-0061, ES-0063, ES-0064, ES-0080, ES-0081, ES-0090, ES-0120, ES-0140, ES-0142, ES-0143, ES-0171 |
| Effective Concurrency | [effective_concurrency.md](../../.claude/guides/golang/effective_concurrency.md) | EC-0010 (goroutine lifecycle), EC-0011 (cancel), EC-0012 (no leak), EC-0013 (ctx.Done in long-running), EC-0050 (ctx first), EC-0051 (no ctx in struct — намеренное исключение для `bgCtx`, см. Gotchas), EC-0053 (timeouts), EC-0055 (call cancel), EC-0056 (unexported ctx keys), EC-0080 (signal shutdown via ctx), EC-0090 (-race) |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | CS-0001 (120 chars), CS-0010 (empty line after blocks), CS-0021 (clear names), CS-0050 (each entity package — SDK исключение, см. TASK-002 PRP), CS-0060 (godoc public), CS-0070 (English), CS-0111 (minimal inline comments) |
| Unit Tests | [unit_tests_standards.md](../../.claude/guides/golang/unit_tests_standards.md) | UT-0001 (table-driven), UT-0002 (one method = one test), UT-0010 (TestType_Method), UT-0011 (happy_path first), UT-0012 (snake_case), UT-0020/0021 (mockery + Expecter), UT-0022 (explicit call count), UT-0023 (no `mock.Anything` for business data), UT-0026 (testMocks struct), UT-0027/0028/0029 (no separators / helpers / globals), UT-0030/0031 (require/assert), UT-0040/0041 (t.Parallel), UT-0070 (go-cmp для структур), UT-0071 (synctest для time-dependent — для cleanup goroutine), UT-0100 (one test file per source file) |
| Integration Tests | [integration_tests_standards.md](../../.claude/guides/golang/integration_tests_standards.md) | Build tag `//go:build integration`, `TestMain` гард `RUN_INTEGRATION_TESTS=true` (наследуется из TASK-001) |

### Concurrency Patterns

- [x] **sync.Map TTL cache** (новый; per ADR §A2)
- [x] **Background goroutine с context cancellation** (cleanupLoop в `internal/cache`)
- [x] **sync.Once-Close** (наследуется из TASK-002)
- [x] **Named return + defer recover** в gRPC interceptor (паттерн из signal-analyzer)

NB: TASK spec ссылается на **EC-0034 (RWMutex для read-heavy)**, но ADR §A2 и тело TASK предписывают **`sync.Map`**. `sync.Map` — корректный выбор для write-once-per-key / read-many паттерна (что точно соответствует кэшу `hash(token) → User`). Citation в TASK — артефакт; следуем ADR. См. Gotcha G2.

---

## Analysis Files

- **Code Research**: [2026-05-18_code-research_REQ-001-TASK-003.md](../../.memory_bank/agent_task_results/2026-05-18_code-research_REQ-001-TASK-003.md) — proto contract specifics, recovery-interceptor pattern, Bearer extraction, ErrorReason values, benchmark conventions
- **Alternatives Analysis (scope shift)**: [2026-05-18-REQ-001-TASK-003-alternatives-analysis.md](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md) — TASK-003 absorbs `lib/context.go` (User/Role/WithUser/UserFromContext) из TASK-005; выбран Option A
- **ADR**: [REQ-001-ADR.md](./REQ-001-ADR.md) §A2 (cache), §A4 (FailMode), §A10 (HTTP middleware), §A11 (gRPC interceptor), Implementation Blueprint (validate.go, middleware_http.go, internal/cache/memory.go)
- **TASK-002 PRP** (предшественник): [REQ-001-TASK-002-PRP.md](./REQ-001-TASK-002-PRP.md) — `Cache` interface с `user any` (line 638–645), `bgCtx context.Context` field decision (line 808–812), `Client` struct без `cache` field, `WithOpenMethods` уже объявлен (line 673–683)
- **Prior research**: [2026-05-15_code-research_REQ-001-TASK-002.md](../../.memory_bank/agent_task_results/2026-05-15_code-research_REQ-001-TASK-002.md) — `mapGRPCError`, `statusFromWrapped`, `grpc.NewClient`, `sync.Once`-Close

---

## Context

### Current Structure

После завершения TASK-001 + TASK-002 ожидаемая структура:

```
auth-client-go/
├── go.mod                          # github.com/webdelo/auth-proto/go, grpc, x/sync, testify
├── Makefile                        # + env-up/env-down/test-integration (TASK-001)
├── mockery.yaml                    # пустой stub (нет интерфейсов для генерации)
└── lib/
    ├── client.go                   # Config, FailMode, Option, Client, New, Close, Cache interface (user any)
    ├── errors.go                   # 5 sentinel + AuthError + RetryAfter + mapGRPCError + mapErrorReason + statusFromWrapped
    ├── logger.go                   # Logger interface + NoopLogger
    ├── client_test.go              # TestNew, TestClient_Close
    ├── errors_test.go              # mapping tables
    ├── logger_test.go              # TestNoopLogger
    ├── client_integration_test.go  # //go:build integration; TestMain gate (TASK-001)
    ├── examples/basic/main.go      # stub print
    └── internal/.gitkeep
```

### Desired Structure

```
auth-client-go/
├── mockery.yaml                                # UPDATE: +auth_v1.AuthServiceClient mock, +authclient.Cache mock
└── lib/
    ├── client.go                               # UPDATE: Cache.Get/Set → User (was any); +cache Cache field on Client;
    │                                           #          wire NewMemoryCache(c.bgCtx, ttl/2) when cfg.Cache==nil;
    │                                           #          ensure bgCtx is stored as field
    ├── context.go                              # NEW: Role, User, IsZero/IsAdmin/HasRole, userCtxKey, WithUser, UserFromContext
    ├── context_test.go                         # NEW: tests for User methods, WithUser/UserFromContext
    ├── validate.go                             # NEW: hashToken, ValidateAccessToken
    ├── validate_test.go                        # NEW: cache hit/miss/expired/positive-only, gRPC error mapping, empty token
    ├── middleware_http.go                      # NEW: bearerToken, HTTPMiddleware
    ├── middleware_http_test.go                 # NEW: 401/503/429/200/FailOpen cases + ctx propagation
    ├── interceptor_grpc.go                     # NEW: UnaryInterceptor (named return + defer recover)
    ├── interceptor_grpc_test.go                # NEW: open-method skip, missing metadata, panic recovery, mapping
    ├── validate_integration_test.go            # NEW: //go:build integration; happy + invalid (expired/blocked → t.Skip TODO)
    └── internal/
        └── cache/
            ├── memory.go                       # NEW: MemoryCache (sync.Map) + cleanupLoop
            ├── memory_test.go                  # NEW: Get/Set/Delete/lazy-expire/cleanup-goroutine-stops/concurrency
            └── memory_bench_test.go            # NEW: BenchmarkMemoryCache_Get (< 1µs/op target)
```

### Gotchas

| # | Category | Issue | Solution |
|---|----------|-------|----------|
| G1 | TASK-002 carry-over | `Cache` interface объявлен с `user any` (TASK-002 PRP line 638–645). | UPDATE: заменить `any` → `User` для `Get`/`Set`. Это compile-time change; `User` теперь определён в `lib/context.go`. `WithCache` и `Config.Cache` автоматически подхватывают новый тип. |
| G2 | Rule citation mismatch | TASK триггерит **EC-0034** (RWMutex для read-heavy), но ADR §A2 и тело TASK предписывают **sync.Map**. | Используем `sync.Map` — корректно для write-once-per-key + read-many (см. official doc: «entries are only written once but read many times»). EC-0034 — рекомендация для общих read-heavy map'ов; здесь специализированный паттерн. |
| G3 | `time.NewTicker(0)` panic | Если `cfg.ValidateCacheTTL == 0` (cache disabled), `cleanupInterval = 0` приходит в `NewMemoryCache`. `time.NewTicker(0)` паникует. | Guard: `if cleanupInterval <= 0 { return &MemoryCache{} }` без запуска cleanup goroutine. Lazy-expire в `Get` всё равно работает. |
| G4 | Proto не возвращает ExpiresAt | `ValidateTokenResponse` имеет только `IsValid/UserUuid/Role/ErrorReason`. ADR Blueprint упоминает `min(TTL, time-to-token-expiry)`. | TTL = просто `cfg.ValidateCacheTTL`. Документировать в godoc на `ValidateAccessToken`. Это accepted limitation v0.1.0 — ADR §A2 ставит TTL ≤ 30s ровно для этой причины. |
| G5 | Proto не возвращает Email | `User.Email == ""` после `ValidateAccessToken`. Email заполняется только через `Login`/`GetCurrentUser` (TASK-004). | Документировать в godoc на `User.Email` и на `ValidateAccessToken`. Соответствует ADR §R10. |
| G6 | `ValidateToken` никогда не возвращает gRPC error | По proto-комментарию (`auth_service_grpc.pb.go:56`): `is_valid=false` приходит как **успешный** response. gRPC error = только network/transport. | В `validate.go`: после `gRPC err != nil` сразу `return User{}, mapGRPCError(err)` (→ `ErrUnavailable`). Negative result обрабатывается через `mapErrorReason(resp.ErrorReason)`. |
| G7 | `mapErrorReason` value set | AUTH server возвращает строки: `expired`, `invalid_signature`, `invalid_token_version`, `blocked`, `user_not_found`, `internal_error`, `empty_token` (из `auth/usecase/auth/dto.go:48`). | Switch:<br>`expired` / `invalid_token_version` → `ErrExpired`<br>`blocked` → `ErrBlocked`<br>default (включая `invalid_signature`, `user_not_found`, `internal_error`, `empty_token`, `""`) → `ErrInvalidToken` |
| G8 | Bearer extraction case-sensitivity | ADR pseudocode использует `strings.TrimPrefix(authz, "Bearer ")` — case-sensitive. RFC 7235 — scheme case-insensitive. | Реализуем robust helper `bearerToken(authz string) (string, bool)` по образцу `auth/internal/service/httpauthmw/middleware.go:209-232`: case-insensitive `EqualFold` на префикс, `TrimSpace` value, проверка `ContainsAny(" \t\r\n")`, `len <= 8192`. |
| G9 | gRPC metadata keys lowercase | `metadata.MD.Get("authorization")` (lowercase) — gRPC нормализует на wire и в MD. | Использовать `md.Get("authorization")` (нижний регистр), как в ADR §A11. `md.Get("Authorization")` тоже работает (Get нормализует), но lowercase — convention. |
| G10 | EC-0051 vs `bgCtx` field | EC-0051 говорит «никогда не храни ctx в struct». Но TASK-002 PRP добавил `bgCtx context.Context` в `Client` (line 812) — single source for cancellation. | Это намеренное исключение per ADR §A2 (cache cleaner подписан на bgCtx). Документировать в godoc на поле: `bgCtx is the background context cancelled by Close; intentional exception to EC-0051`. |
| G11 | `next.ServeHTTP` после `http.Error` | После `http.Error(w, ...)` нельзя писать в `w`. Если забыть `return` — handler продолжится. | Каждая ветка switch завершается explicit `return`. Тест на это: после 401 не должен сработать `next`. |
| G12 | `WithOpenMethods` уже объявлен в TASK-002 | Не повторно объявлять. Default registry в TASK-002 уже даёт `/grpc.health.v1.Health/Check`+`Watch`. | В UnaryInterceptor просто читаем `c.openMethods` (lookup `if _, ok := c.openMethods[info.FullMethod]; ok { return handler(ctx, req) }`). |
| G13 | Race на cleanupLoop через cancel | Если `Close()` вызвать **до** `New` завершился (нереально через публичный API, но в тестах возможно), goroutine может ещё не успеть запуститься. | Запускаем cleanupLoop **внутри** `NewMemoryCache` после получения bgCtx — `Close()` уже после `New` отрабатывает корректно. Test через `t.Cleanup(c.Close); runtime.Gosched(); ...`. |
| G14 | Mock для `auth_v1.AuthServiceClient` | Mock'и через mockery (UT-0020). Интерфейс `auth_v1.AuthServiceClient` — внешний пакет. | В `mockery.yaml` добавить запись с `srcdir: ./vendor/github.com/webdelo/auth-proto/go/auth_v1` либо использовать встроенный go-list-discovery mockery v2. Реализация — mock в `lib/mocks/auth_service_client.go`. |
| G15 | Mock для `Cache` interface | Тесты `validate.go` мокают cache, чтобы изолировать поведение от реальной MemoryCache. | В `mockery.yaml` добавить `github.com/webdelo/auth-client-go/lib` → `Cache`. Mock — `lib/mocks/cache.go`. |
| G16 | t.Parallel + shared `time.Now` | Cache lazy-expire зависит от `time.Now()`. Тесты на expiration в Parallel могут флакать на slow runner. | Использовать `testing/synctest` (Go 1.24+, UT-0071) для time-controlled тестов. Альтернатива: инъекция `now func() time.Time` в `MemoryCache` — но это меняет API. Выбор: `synctest.Run(...)` для тестов с reliance на TTL boundary. |
| G17 | `http.Error` content-type | `http.Error` пишет `text/plain; charset=utf-8` + `"\n"`. ADR §A10: «не сериализуем в JSON». | OK — это и есть желаемое поведение. Documented в ADR. |
| G18 | golangci-lint `contextcheck` | Если `cleanupLoop` использует ctx первого аргумента, но создаётся с `context.Background()` под капотом — лантер может ругаться на mismatched ctx propagation. | `New` явно создаёт `bgCtx, cancel := context.WithCancel(context.Background())`, передаёт `bgCtx` в `NewMemoryCache`. Cleanup в это поле — НЕ ctx из request. Это by-design (background lifecycle); если lint ругается — `//nolint:contextcheck` запрещён (ES-0171) → перепроверить, что мы НЕ передаём request-ctx. |

---

## Implementation

### Data Models

```go
// lib/context.go

// Role is the user authorization role.
//
// Values mirror auth.v1 proto contract. New roles are additive after v1.0.0;
// adding a new constant must NOT renumber the existing ones.
type Role string

const (
    RoleUnknown Role = ""      // populated when AUTH returns an unrecognized role
    RoleUser    Role = "user"
    RoleAdmin   Role = "admin"
)

// User is the authenticated principal extracted from a valid access token.
//
// Email is populated only by Login and GetCurrentUser. After ValidateAccessToken
// the Email is always empty because the gRPC ValidateTokenResponse does not
// carry it (proto v0.1.0 limitation — see REQ-001-ADR §R10).
type User struct {
    UUID  string
    Role  Role
    Email string
}

// IsZero reports whether u is the zero value (no UUID).
func (u User) IsZero() bool { return u.UUID == "" }

// IsAdmin reports whether u has the admin role.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// HasRole reports whether u has the exact given role.
func (u User) HasRole(role Role) bool { return u.Role == role }

// userCtxKey is the unexported context key for User (EC-0056).
type userCtxKey struct{}

// WithUser returns a copy of ctx with user attached.
func WithUser(ctx context.Context, user User) context.Context {
    return context.WithValue(ctx, userCtxKey{}, user)
}

// UserFromContext extracts the User stored by WithUser. The bool is false if
// no User is attached.
func UserFromContext(ctx context.Context) (User, bool) {
    user, ok := ctx.Value(userCtxKey{}).(User)
    return user, ok
}
```

```go
// lib/internal/cache/memory.go

// MemoryCache is an in-process TTL cache for ValidateToken results.
//
// Keys are caller-supplied (the SDK uses hex(sha256(access_token))). Values are
// authclient.User. Access is lock-free (sync.Map); a background goroutine
// drains tombstones on `cleanupInterval`. The goroutine exits when bgCtx
// passed to NewMemoryCache is cancelled (EC-0010 lifecycle).
type MemoryCache struct {
    m sync.Map // key string -> entry
}

type entry struct {
    user      authclient.User // ← actually `Cache.Set` value type per public interface
    expiresAt time.Time
}
```

**Cache interface (updated in `lib/client.go`):**

```go
type Cache interface {
    Get(ctx context.Context, key string) (user User, ok bool)
    Set(ctx context.Context, key string, user User, ttl time.Duration)
    Delete(ctx context.Context, key string)
}
```

> **Note (import cycle):** `internal/cache` стоит лежать в отдельном пакете и НЕ импортировать `authclient` (чтобы избежать import cycle через `Cache` интерфейс). Решение: `cache.MemoryCache` хранит `entry{user any, expiresAt time.Time}` внутри; `Get`/`Set` принимают/возвращают `any`, а `Client.cache` обёрнут адаптером `cacheAdapter{*cache.MemoryCache}` в `lib/client.go`, который type-asserts `any → User`. См. Task 7 ниже.

---

### Task 1: UPDATE lib/client.go

**Action:** UPDATE
**File:** [lib/client.go](../../lib/client.go)

**Description:** Финализировать `Cache` интерфейс (`any` → `User`), добавить `cache Cache` поле на `Client`, добавить хранение `bgCtx context.Context` в struct (если ещё не добавлено в TASK-002 execution), wire default `MemoryCache` в `New` через адаптер.

**Details:**

- Заменить `Cache` интерфейс:

```go
type Cache interface {
    Get(ctx context.Context, key string) (user User, ok bool)
    Set(ctx context.Context, key string, user User, ttl time.Duration)
    Delete(ctx context.Context, key string)
}
```

- Добавить поля в `Client`:

```go
type Client struct {
    cfg         Config
    grpcConn    *grpc.ClientConn
    authClient  auth_v1.AuthServiceClient
    cache       Cache               // ← NEW
    logger      Logger
    failMode    FailMode
    openMethods map[string]struct{}

    bgCtx     context.Context     // ← NEW (intentional EC-0051 exception, see Gotcha G10)
    cancelBG  context.CancelFunc
    closeOnce sync.Once
    closeErr  error
}
```

- В `New(cfg, opts...)`:

```go
bgCtx, cancel := context.WithCancel(context.Background())

c := &Client{
    cfg:         cfg,
    grpcConn:    conn,
    authClient:  auth_v1.NewAuthServiceClient(conn),
    logger:      cfg.Logger,
    failMode:    cfg.FailMode,
    openMethods: defaultOpenMethods(),
    bgCtx:       bgCtx,
    cancelBG:    cancel,
}

// Cache: pre-fill via Config.Cache or WithCache (after opts apply);
// if still nil after opts, use default MemoryCache wrapped in cacheAdapter.
for _, opt := range opts {
    opt(c)
}

if c.cache == nil && cfg.Cache != nil {
    c.cache = cfg.Cache
}
if c.cache == nil {
    cleanupInterval := cfg.ValidateCacheTTL / 2
    c.cache = newCacheAdapter(cachepkg.NewMemoryCache(bgCtx, cleanupInterval))
}
```

> **Important:** `WithCache(Cache)` option currently sets `c.cfg.Cache` (TASK-002 PRP line 670). Поменять на set `c.cache` напрямую — иначе нужны два места проверки. Обновить:
>
> ```go
> func WithCache(cache Cache) Option {
>     return func(c *Client) { c.cache = cache }
> }
> ```

- Добавить адаптер `cacheAdapter` (приватный) для конверсии между `Cache` (типизированный `User`) и `cache.MemoryCache` (нетипизированный `any`):

```go
// cacheAdapter wraps *cache.MemoryCache to satisfy the Cache interface
// without forcing internal/cache to depend on authclient.User (import cycle).
type cacheAdapter struct {
    inner *cachepkg.MemoryCache
}

func newCacheAdapter(inner *cachepkg.MemoryCache) *cacheAdapter {
    return &cacheAdapter{inner: inner}
}

func (a *cacheAdapter) Get(ctx context.Context, key string) (User, bool) {
    v, ok := a.inner.Get(ctx, key)
    if !ok {
        return User{}, false
    }
    user, ok := v.(User)
    if !ok {
        return User{}, false
    }
    return user, true
}

func (a *cacheAdapter) Set(ctx context.Context, key string, user User, ttl time.Duration) {
    a.inner.Set(ctx, key, user, ttl)
}

func (a *cacheAdapter) Delete(ctx context.Context, key string) {
    a.inner.Delete(ctx, key)
}
```

- Import: `cachepkg "github.com/webdelo/auth-client-go/lib/internal/cache"`.

**Code Pattern Diff (illustrative):**

```diff
 type Cache interface {
-    Get(ctx context.Context, key string) (user any, ok bool)
-    Set(ctx context.Context, key string, user any, ttl time.Duration)
+    Get(ctx context.Context, key string) (user User, ok bool)
+    Set(ctx context.Context, key string, user User, ttl time.Duration)
     Delete(ctx context.Context, key string)
 }
```

---

### Task 2: CREATE lib/context.go

**Action:** CREATE
**File:** [lib/context.go](../../lib/context.go)

**Description:** Опубликовать `Role`, `User` (+ методы `IsZero/IsAdmin/HasRole`), `userCtxKey`, `WithUser`, `UserFromContext`. Финализировать context-payload контракт SDK.

**Details:**

- `package authclient`.
- Все типы и функции согласно Data Models выше.
- `WithUser` использует `context.WithValue` с unexported key `userCtxKey{}` (EC-0056).
- `UserFromContext` использует comma-ok type assertion: `user, ok := ctx.Value(userCtxKey{}).(User); return user, ok`.

**Acceptance:**

- `var _ context.Context = WithUser(context.Background(), User{UUID: "u"})` — компилируется.
- `u, ok := UserFromContext(ctx)` — возвращает comma-ok пару.
- `User{}.IsZero() == true`, `User{UUID: "u"}.IsZero() == false`.

---

### Task 3: CREATE lib/internal/cache/memory.go

**Action:** CREATE
**File:** [lib/internal/cache/memory.go](../../lib/internal/cache/memory.go)

**Description:** TTL cache на `sync.Map` с lazy-expire в `Get` и background cleanup goroutine, подписанной на `bgCtx`.

**Details:**

- `package cache`.
- Тип `MemoryCache` хранит `entry{user any, expiresAt time.Time}` (нетипизированный, чтобы избежать import cycle на `authclient.User`).
- Конструктор `NewMemoryCache(bgCtx context.Context, cleanupInterval time.Duration) *MemoryCache`:
    - Создаёт `*MemoryCache`.
    - Если `cleanupInterval > 0` — запускает `go c.cleanupLoop(bgCtx, cleanupInterval)`. Иначе — НЕ запускает (EC-0010 — без goroutine нет лайфсайкла).
- `Get(_ context.Context, key string) (any, bool)`:
    - `v, ok := m.Load(key)`; если нет — `return nil, false`.
    - Тип-каст `e := v.(entry)`; если `time.Now().After(e.expiresAt)` — `m.Delete(key)` (lazy) + `return nil, false`.
    - Иначе — `return e.user, true`.
- `Set(_ context.Context, key string, user any, ttl time.Duration)`:
    - Если `ttl <= 0` — no-op (отключение кэша для одиночной записи).
    - `m.Store(key, entry{user: user, expiresAt: time.Now().Add(ttl)})`.
- `Delete(_ context.Context, key string)` — `m.Delete(key)`.
- `cleanupLoop(ctx context.Context, interval time.Duration)`:
    - `t := time.NewTicker(interval); defer t.Stop()`.
    - В цикле `select { case <-ctx.Done(): return; case <-t.C: scan-and-delete }`.
    - `m.Range(func(k, v any) bool { if time.Now().After(v.(entry).expiresAt) { m.Delete(k) }; return true })`.

**Code Pattern:**

```go
package cache

import (
    "context"
    "sync"
    "time"
)

type entry struct {
    user      any
    expiresAt time.Time
}

// MemoryCache is the default TTL-cache implementation backed by sync.Map.
//
// Lifecycle: a cleanup goroutine is started by NewMemoryCache when
// cleanupInterval > 0 and exits when bgCtx is cancelled (EC-0010, EC-0012).
type MemoryCache struct {
    m sync.Map
}

// NewMemoryCache constructs a MemoryCache and (when cleanupInterval > 0)
// launches a background cleanup goroutine.
//
// bgCtx controls the cleanup goroutine lifecycle. Cancelling bgCtx stops
// the goroutine. Pass context.Background() if the cache is permanent;
// pass the SDK Client's background context to bind cleanup lifetime to
// Client.Close.
func NewMemoryCache(bgCtx context.Context, cleanupInterval time.Duration) *MemoryCache {
    c := &MemoryCache{}
    if cleanupInterval > 0 {
        go c.cleanupLoop(bgCtx, cleanupInterval)
    }
    return c
}

func (c *MemoryCache) Get(_ context.Context, key string) (any, bool) {
    v, ok := c.m.Load(key)
    if !ok {
        return nil, false
    }
    e := v.(entry)
    if time.Now().After(e.expiresAt) {
        c.m.Delete(key)
        return nil, false
    }
    return e.user, true
}

func (c *MemoryCache) Set(_ context.Context, key string, user any, ttl time.Duration) {
    if ttl <= 0 {
        return
    }
    c.m.Store(key, entry{user: user, expiresAt: time.Now().Add(ttl)})
}

func (c *MemoryCache) Delete(_ context.Context, key string) {
    c.m.Delete(key)
}

func (c *MemoryCache) cleanupLoop(ctx context.Context, interval time.Duration) {
    t := time.NewTicker(interval)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-t.C:
            now := time.Now()
            c.m.Range(func(k, v any) bool {
                if now.After(v.(entry).expiresAt) {
                    c.m.Delete(k)
                }
                return true
            })
        }
    }
}
```

---

### Task 4: CREATE lib/validate.go

**Action:** CREATE
**File:** [lib/validate.go](../../lib/validate.go)

**Description:** Реализовать `(*Client).ValidateAccessToken` + приватный helper `hashToken`. Положительный результат кэшируется; negative — нет.

**Details:**

- `package authclient`.
- Приватный helper:

```go
// hashToken returns hex(sha256(token)) — used as the cache key so the raw
// token never appears in logs / metrics / heap dumps.
func hashToken(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}
```

- Метод `ValidateAccessToken`:

```go
// ValidateAccessToken validates accessToken against AUTH and returns the
// authenticated User on success. On cache hit returns the cached User; on
// cache miss issues a gRPC ValidateToken call with cfg.ValidateTimeout.
//
// Positive results are cached for cfg.ValidateCacheTTL; negative results
// (ErrInvalidToken / ErrExpired / ErrBlocked) and transport errors
// (ErrUnavailable, ErrRateLimited) are NOT cached.
//
// Returned errors: ErrInvalidToken (empty token, invalid signature, unknown
// user), ErrExpired (token TTL elapsed or token_version bumped), ErrBlocked
// (user blocked), ErrUnavailable (AUTH unreachable / timeout), ErrRateLimited
// (AUTH rate-limited the SDK).
//
// User.Email is always empty after ValidateAccessToken (proto v0.1.0 does not
// carry email in ValidateTokenResponse — see REQ-001-ADR §R10). Use
// GetCurrentUser to fetch email.
func (c *Client) ValidateAccessToken(ctx context.Context, accessToken string) (User, error) {
    if accessToken == "" {
        return User{}, ErrInvalidToken
    }

    key := hashToken(accessToken)
    if user, ok := c.cache.Get(ctx, key); ok {
        return user, nil
    }

    callCtx, cancel := context.WithTimeout(ctx, c.cfg.ValidateTimeout)
    defer cancel()

    resp, err := c.authClient.ValidateToken(callCtx, &auth_v1.ValidateTokenRequest{
        AccessToken: accessToken,
    })
    if err != nil {
        return User{}, mapGRPCError(err)
    }
    if !resp.IsValid {
        return User{}, mapErrorReason(resp.ErrorReason)
    }

    user := User{
        UUID: resp.UserUuid,
        Role: Role(resp.Role),
        // Email is intentionally empty — see godoc above.
    }
    c.cache.Set(ctx, key, user, c.cfg.ValidateCacheTTL)
    return user, nil
}
```

- Import: `crypto/sha256`, `encoding/hex`, `context`, `auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"`.
- `mapGRPCError` и `mapErrorReason` — определены в TASK-002 `lib/errors.go`. Проверить, что `mapErrorReason` покрывает `"invalid_token_version"` → `ErrExpired` (per Gotcha G7). Если в TASK-002 это не сделано — добавить в `lib/errors.go` строку в switch.

---

### Task 5: CREATE lib/middleware_http.go

**Action:** CREATE
**File:** [lib/middleware_http.go](../../lib/middleware_http.go)

**Description:** HTTP middleware с Bearer extraction, маппингом в коды 401/503/429/200 и FailMode-обработкой. Алгоритм по REQ-001-ADR §A10.

**Details:**

- Приватные константы:

```go
const (
    bearerPrefix   = "Bearer "
    maxBearerLen   = 8192
    retryAfterUnav = "5"

    wwwAuthMissing = `Bearer realm="api"`
    wwwAuthInvalid = `Bearer realm="api", error="invalid_token"`
)
```

- Приватный helper `bearerToken(authz string) (token string, ok bool)` — case-insensitive `EqualFold` на префикс, trim leading whitespace + value-side `TrimSpace`, reject inner whitespace, length-cap. Образец: `auth/internal/service/httpauthmw/middleware.go:209-232`.

```go
func bearerToken(authz string) (string, bool) {
    authz = strings.TrimLeft(authz, " \t")
    if len(authz) < len(bearerPrefix) ||
        !strings.EqualFold(authz[:len(bearerPrefix)], bearerPrefix) {
        return "", false
    }
    token := strings.TrimSpace(authz[len(bearerPrefix):])
    if token == "" ||
        strings.ContainsAny(token, " \t\r\n") ||
        len(token) > maxBearerLen {
        return "", false
    }
    return token, true
}
```

- Метод `(*Client).HTTPMiddleware(next http.Handler) http.Handler`:

```go
func (c *Client) HTTPMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token, ok := bearerToken(r.Header.Get("Authorization"))
        if !ok {
            w.Header().Set("WWW-Authenticate", wwwAuthMissing)
            http.Error(w, "missing or malformed token", http.StatusUnauthorized)
            return
        }

        user, err := c.ValidateAccessToken(r.Context(), token)
        switch {
        case errors.Is(err, ErrUnavailable):
            if c.failMode == FailOpen {
                c.logger.Warn(r.Context(), "authclient: AUTH unavailable, fail-open pass-through")
                next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), User{})))
                return
            }
            w.Header().Set("Retry-After", retryAfterUnav)
            http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
            return

        case errors.Is(err, ErrRateLimited):
            if d := RetryAfter(err); d > 0 {
                w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())))
            }
            http.Error(w, "rate limited", http.StatusTooManyRequests)
            return

        case err != nil: // ErrInvalidToken / ErrExpired / ErrBlocked
            w.Header().Set("WWW-Authenticate", wwwAuthInvalid)
            http.Error(w, err.Error(), http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
    })
}
```

- Import: `errors`, `net/http`, `strconv`, `strings`.
- Каждая ветка switch ОБЯЗАНА завершаться `return` (Gotcha G11).
- `c.logger` — гарантированно non-nil после `applyDefaults` (NoopLogger fallback, ES-0143).

---

### Task 6: CREATE lib/interceptor_grpc.go

**Action:** CREATE
**File:** [lib/interceptor_grpc.go](../../lib/interceptor_grpc.go)

**Description:** gRPC unary server interceptor с защитой от паники и openMethods skip. Алгоритм по REQ-001-ADR §A11.

**Details:**

- `package authclient`.
- Метод `(*Client).UnaryInterceptor() grpc.UnaryServerInterceptor`:

```go
func (c *Client) UnaryInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (resp interface{}, err error) {
        // Panic recovery (paттерн из signal-analyzer/internal/handlers/grpc/signal/interceptors.go:99-119).
        // Named return is REQUIRED — defer assignment to `err` propagates to caller
        // only when the variable is named.
        defer func() {
            if r := recover(); r != nil {
                c.logger.Error(ctx, "authclient: grpc handler panic", nil, "panic", r)
                err = status.Errorf(codes.Internal, "panic: %v", r)
                resp = nil
            }
        }()

        if _, open := c.openMethods[info.FullMethod]; open {
            return handler(ctx, req)
        }

        md, mdOK := metadata.FromIncomingContext(ctx)
        if !mdOK {
            return nil, status.Error(codes.Unauthenticated, "missing metadata")
        }
        authValues := md.Get("authorization")
        if len(authValues) == 0 {
            return nil, status.Error(codes.Unauthenticated, "missing authorization")
        }
        token, tokOK := bearerToken(authValues[0])
        if !tokOK {
            return nil, status.Error(codes.Unauthenticated, "invalid authorization header")
        }

        user, vErr := c.ValidateAccessToken(ctx, token)
        switch {
        case errors.Is(vErr, ErrUnavailable):
            if c.failMode == FailOpen {
                c.logger.Warn(ctx, "authclient: AUTH unavailable, fail-open pass-through")
                return handler(WithUser(ctx, User{}), req)
            }
            return nil, status.Error(codes.Unavailable, "auth unavailable")

        case errors.Is(vErr, ErrRateLimited):
            return nil, status.Error(codes.ResourceExhausted, "rate limited")

        case vErr != nil: // ErrInvalidToken / ErrExpired / ErrBlocked
            return nil, status.Error(codes.Unauthenticated, vErr.Error())
        }

        return handler(WithUser(ctx, user), req)
    }
}
```

- Import: `errors`, `context`, `google.golang.org/grpc`, `google.golang.org/grpc/codes`, `google.golang.org/grpc/metadata`, `google.golang.org/grpc/status`.
- `c.openMethods` lookup — O(1) на map.
- panic-recovery ОБЯЗАН быть на defer **до** auth-check, чтобы покрывать и handler-panic, и (теоретически) panic из ValidateAccessToken. Логирование паники через `c.logger.Error` — non-blocking (NoopLogger как default).

---

### Task 7: CREATE lib/internal/cache/memory_test.go + memory_bench_test.go

**Action:** CREATE
**Files:** [lib/internal/cache/memory_test.go](../../lib/internal/cache/memory_test.go), [lib/internal/cache/memory_bench_test.go](../../lib/internal/cache/memory_bench_test.go)

**Description:** Unit-тесты для `MemoryCache` (включая EC-0012 cleanup-goroutine-stops) + бенчмарк `BenchmarkMemoryCache_Get` (target `< 1µs/op`).

**Required Test Cases (memory_test.go):**

`TestMemoryCache_Get`:
- [ ] `happy_path` — Set, затем Get → hit.
- [ ] `miss_returns_false` — Get несуществующего ключа.
- [ ] `expired_returns_false` — Set с ttl=10ms, Sleep 20ms (или `synctest`), Get → miss + key removed (lazy-expire).
- [ ] `ttl_zero_set_is_noop` — Set с ttl=0; Get → false (запись не положена).
- [ ] `concurrent_get_set_race_clean` (`go test -race`) — N goroutines Set/Get на пересекающихся ключах.

`TestMemoryCache_Delete`:
- [ ] `happy_path` — Set → Delete → Get → false.
- [ ] `delete_missing_key_is_noop` — Delete несуществующего ключа без паники.

`TestMemoryCache_Cleanup`:
- [ ] `happy_path` — `synctest.Run(...)` (UT-0071): Set с ttl=100ms, прошёл cleanup tick (200ms), Range показывает пустой map.
- [ ] `goroutine_exits_when_ctx_cancelled` — `NewMemoryCache(ctx, 10*time.Millisecond)`, `runtime.NumGoroutine` до `cancel`, после `cancel` + `runtime.Gosched`-цикл (max 100ms) число goroutines = base. Это **DOD-критерий**.
- [ ] `zero_interval_no_goroutine_started` — `NewMemoryCache(ctx, 0)`: `runtime.NumGoroutine` не меняется.

`TestNewMemoryCache`:
- [ ] `with_positive_interval_starts_cleanup` (косвенно через TestMemoryCache_Cleanup happy_path).
- [ ] `with_zero_interval_skips_cleanup`.

**Required Test Cases (memory_bench_test.go):**

```go
func BenchmarkMemoryCache_Get(b *testing.B) {
    c := NewMemoryCache(context.Background(), time.Minute)
    c.Set(context.Background(), "k", "user-payload", time.Minute) // any payload
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = c.Get(context.Background(), "k")
    }
}

func BenchmarkMemoryCache_Set(b *testing.B) {
    c := NewMemoryCache(context.Background(), time.Minute)
    payload := "user-payload"
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        c.Set(context.Background(), "k", payload, time.Minute)
    }
}
```

**Acceptance:** `go test -bench=BenchmarkMemoryCache_Get -benchmem ./lib/internal/cache/` показывает **< 1µs/op** на cache hit (sync.Map.Load ≈ 50–100 ns).

---

### Task 8: CREATE lib/context_test.go

**Action:** CREATE
**File:** [lib/context_test.go](../../lib/context_test.go)

**Description:** Table-driven тесты для `User` методов, `WithUser`, `UserFromContext`. UT-0001, UT-0011, UT-0040.

**Required Test Cases:**

`TestUser_IsZero` (table-driven):
- [ ] `happy_path` — `User{}` → `true`.
- [ ] `with_uuid_returns_false` — `User{UUID:"u"}` → `false`.

`TestUser_IsAdmin`:
- [ ] `happy_path` — `User{Role: RoleAdmin}` → `true`.
- [ ] `user_role_returns_false` — `User{Role: RoleUser}` → `false`.
- [ ] `unknown_role_returns_false` — `User{}` → `false`.

`TestUser_HasRole`:
- [ ] `happy_path` — `User{Role: RoleAdmin}.HasRole(RoleAdmin)` → `true`.
- [ ] `mismatch_returns_false`.

`TestUserFromContext`:
- [ ] `happy_path` — `WithUser(ctx, u)` → `UserFromContext` возвращает `(u, true)`.
- [ ] `empty_ctx_returns_zero_and_false` — пустой ctx → `(User{}, false)`.

NB: `t.Parallel()` ОБЯЗАТЕЛЕН в main + subtest (UT-0040 + UT-0041).

---

### Task 9: CREATE lib/validate_test.go

**Action:** CREATE
**File:** [lib/validate_test.go](../../lib/validate_test.go)

**Description:** Unit-тесты для `ValidateAccessToken` с моком `auth_v1.AuthServiceClient` + моком `Cache`. UT-0020 (mockery), UT-0023 (no mock.Anything для business data).

**Pre-requisite:** `make generate-mocks` сгенерил `lib/mocks/AuthServiceClient.go` + `lib/mocks/Cache.go` (см. Task 12).

**Required Test Cases:**

`TestClient_ValidateAccessToken`:
- [ ] `happy_path` — cache miss → gRPC OK + IsValid=true → user возвращается + Cache.Set вызван 1 раз с ttl=cfg.ValidateCacheTTL.
- [ ] `cache_hit_returns_without_grpc` — Cache.Get → (User, true) → User возвращается, AuthServiceClient.ValidateToken **не вызывается** (Times(0)).
- [ ] `empty_token_returns_invalid_token` — `ValidateAccessToken(ctx, "")` → `ErrInvalidToken`; ни Cache.Get, ни ValidateToken не вызываются.
- [ ] `is_valid_false_expired_returns_err_expired` — gRPC OK + IsValid=false + ErrorReason="expired" → `ErrExpired`; Cache.Set Times(0).
- [ ] `is_valid_false_invalid_token_version_returns_err_expired` — ErrorReason="invalid_token_version" → `ErrExpired`.
- [ ] `is_valid_false_blocked_returns_err_blocked` — ErrorReason="blocked" → `ErrBlocked`.
- [ ] `is_valid_false_unknown_reason_returns_err_invalid_token` — ErrorReason="user_not_found" → `ErrInvalidToken`.
- [ ] `is_valid_false_empty_reason_returns_err_invalid_token` — ErrorReason="" → `ErrInvalidToken`.
- [ ] `grpc_unavailable_returns_err_unavailable` — gRPC error `codes.Unavailable` → `ErrUnavailable`; Cache.Set Times(0).
- [ ] `grpc_deadline_exceeded_returns_err_unavailable`.
- [ ] `grpc_resource_exhausted_returns_err_rate_limited` — `ErrRateLimited` + `RetryAfter(err) == 0` (proto не передаёт metadata).
- [ ] `cache_key_is_hex_sha256` — Cache.Set вызван с key, проверенным через локально вычисленный `hex(sha256("token"))` (длина 64, hex-only).

NB:
- `testMocks` struct (UT-0026): `mocks := testMocks{authClient: mocks.NewAuthServiceClient(t), cache: mocks.NewCache(t)}`.
- Mocks через `Expecter` API (UT-0021): `mocks.cache.EXPECT().Get(mock.Anything, "...").Return(User{}, false).Times(1)`.
- `mock.Anything` ТОЛЬКО для `ctx`/`opts` (per UT-0023 — business data запрещена).

---

### Task 10: CREATE lib/middleware_http_test.go

**Action:** CREATE
**File:** [lib/middleware_http_test.go](../../lib/middleware_http_test.go)

**Description:** Table-driven тесты для `HTTPMiddleware`. Используют `httptest.NewRecorder` + ручной `http.HandlerFunc` как `next`. UT-0040.

**Required Test Cases:**

`TestBearerToken` (приватная функция):
- [ ] `happy_path` — `"Bearer eyJ..."` → `("eyJ...", true)`.
- [ ] `case_insensitive_scheme` — `"bearer eyJ..."` → ok.
- [ ] `empty_authz_returns_false`.
- [ ] `no_bearer_prefix_returns_false` — `"Basic dXNlcjpwYXNz"` → ok=false.
- [ ] `empty_token_after_prefix_returns_false` — `"Bearer "` → false.
- [ ] `token_with_inner_whitespace_returns_false` — `"Bearer a b"` → false.
- [ ] `token_too_long_returns_false` — len > 8192 → false.
- [ ] `leading_whitespace_trimmed` — `" \tBearer eyJ..."` → ok.

`TestClient_HTTPMiddleware`:
- [ ] `happy_path` — valid token → 200 + `User` в ctx видна next handler'у.
- [ ] `missing_authorization_returns_401` — нет header'а → 401 + `WWW-Authenticate: Bearer realm="api"`; next НЕ вызван.
- [ ] `malformed_bearer_returns_401`.
- [ ] `invalid_token_returns_401_with_error_attribute` — ValidateAccessToken → ErrInvalidToken → 401 + `WWW-Authenticate: Bearer realm="api", error="invalid_token"`.
- [ ] `expired_token_returns_401` — ErrExpired → 401.
- [ ] `blocked_user_returns_401` — ErrBlocked → 401.
- [ ] `unavailable_failclosed_returns_503` — ErrUnavailable + default FailClosed → 503 + `Retry-After: 5`; next НЕ вызван.
- [ ] `unavailable_failopen_passes_through_with_empty_user` — ErrUnavailable + FailOpen → 200 + `UserFromContext` возвращает `(User{}, true)`.
- [ ] `rate_limited_returns_429_with_retry_after` — `ErrRateLimited` + `&AuthError{RetryAfter: 7*time.Second}` → 429 + `Retry-After: 7`.
- [ ] `rate_limited_without_retry_after_returns_429_no_header` — `RetryAfter == 0` → 429, no `Retry-After` header.
- [ ] `next_handler_not_called_on_error` — для каждого error-кейса проверить, что `next.called == false` (через флаг в next).

NB: моки `Client.cache` + `Client.authClient` через `Expecter`; либо использовать stub Client (build с DI'ом приватных полей через test-helpers — но UT-0028 запрещает helpers, поэтому inline сборка `&Client{cache: mockCache, authClient: mockAuth, cfg: ..., logger: NoopLogger{}, failMode: ...}`).

---

### Task 11: CREATE lib/interceptor_grpc_test.go

**Action:** CREATE
**File:** [lib/interceptor_grpc_test.go](../../lib/interceptor_grpc_test.go)

**Description:** Table-driven тесты для `UnaryInterceptor`. Используем `grpc.UnaryServerInfo{FullMethod: "..."}` + `metadata.NewIncomingContext(ctx, md)`.

**Required Test Cases:**

`TestClient_UnaryInterceptor`:
- [ ] `happy_path` — valid token в metadata → handler вызван с `UserFromContext` возвращающим user.
- [ ] `open_method_skips_auth` — `info.FullMethod = "/grpc.health.v1.Health/Check"` → handler вызван **без** обращения к cache/authClient.
- [ ] `with_open_methods_custom_method_skips_auth` — `WithOpenMethods("/myapp.v1.Public/Method")` через `New(..., WithOpenMethods(...))` → `info.FullMethod=...` → skip.
- [ ] `missing_metadata_returns_unauthenticated` — handler НЕ вызван; err = `codes.Unauthenticated`.
- [ ] `missing_authorization_returns_unauthenticated` — metadata без `authorization` key.
- [ ] `malformed_bearer_returns_unauthenticated`.
- [ ] `invalid_token_returns_unauthenticated` — ValidateAccessToken → ErrInvalidToken → `codes.Unauthenticated`; handler НЕ вызван.
- [ ] `expired_token_returns_unauthenticated`.
- [ ] `blocked_returns_unauthenticated`.
- [ ] `unavailable_failclosed_returns_unavailable` — `codes.Unavailable`.
- [ ] `unavailable_failopen_calls_handler_with_empty_user` — `codes.OK` (no err) + handler видит `User{}`.
- [ ] `rate_limited_returns_resource_exhausted` — `codes.ResourceExhausted`.
- [ ] `panic_in_handler_returns_internal` — handler делает `panic("boom")` → err = `status.Error(codes.Internal, "panic: boom")`; resp = nil. Это **DOD-критерий**.

NB:
- `status.Code(err)` для извлечения кода (testify `assert.Equal(t, codes.Unauthenticated, status.Code(err))`).
- panic-test ОБЯЗАТЕЛЬНО проверяет, что err не nil + resp == nil + код Internal.

---

### Task 12: UPDATE mockery.yaml + CREATE lib/mocks/

**Action:** UPDATE + CREATE (generated)
**Files:** [mockery.yaml](../../mockery.yaml), [lib/mocks/](../../lib/mocks/)

**Description:** Зарегистрировать интерфейсы для генерации mock'ов: `auth_v1.AuthServiceClient` (внешний) и `authclient.Cache` (внутренний). Запустить `make generate-mocks`.

**Details:**

- В `mockery.yaml` (вариант для mockery v2):

```yaml
with-expecter: true
packages:
  github.com/webdelo/auth-proto/go/auth_v1:
    interfaces:
      AuthServiceClient:
        config:
          dir: lib/mocks
          outpkg: mocks
          filename: auth_service_client_mock.go
          mockname: AuthServiceClient
  github.com/webdelo/auth-client-go/lib:
    interfaces:
      Cache:
        config:
          dir: lib/mocks
          outpkg: mocks
          filename: cache_mock.go
          mockname: Cache
      Logger:
        config:
          dir: lib/mocks
          outpkg: mocks
          filename: logger_mock.go
          mockname: Logger
```

- Выполнить `make generate-mocks` (или `mockery` напрямую).
- Сгенерированные файлы коммитим (стандарт sibling-репов — generated mocks под VCS).
- Проверить, что `lib/mocks/auth_service_client_mock.go` имеет метод `EXPECT().ValidateToken(...)`.

NB: если на момент TASK-003 mockery.yaml в TASK-002 уже содержит секцию для Logger — не дублировать, только добавить недостающее.

---

### Task 13: CREATE lib/validate_integration_test.go

**Action:** CREATE
**File:** [lib/validate_integration_test.go](../../lib/validate_integration_test.go)

**Description:** Integration-тесты `ValidateAccessToken` против реального AUTH gRPC-сервера. Build tag `//go:build integration`.

**Test Cases:**

```go
//go:build integration

package authclient_test

import (
    "context"
    "os"
    "testing"
    "time"

    authclient "github.com/webdelo/auth-client-go/lib"
    "github.com/stretchr/testify/require"
)

func TestValidateAccessToken_Happy(t *testing.T) {
    endpoint := requireAuthEndpoint(t)
    c, err := authclient.New(authclient.Config{Endpoint: endpoint})
    require.NoError(t, err)
    t.Cleanup(func() { _ = c.Close() })

    accessToken := loginAndGetAccessToken(t, c) // helper, использует Login (TASK-004) — если TASK-004 не сделан, см. TODO ниже
    user, err := c.ValidateAccessToken(context.Background(), accessToken)
    require.NoError(t, err)
    require.NotEmpty(t, user.UUID)
    require.Empty(t, user.Email) // proto не возвращает email
}

func TestValidateAccessToken_Invalid(t *testing.T) {
    endpoint := requireAuthEndpoint(t)
    c, err := authclient.New(authclient.Config{Endpoint: endpoint})
    require.NoError(t, err)
    t.Cleanup(func() { _ = c.Close() })

    _, err = c.ValidateAccessToken(context.Background(), "definitely-not-a-real-token")
    require.ErrorIs(t, err, authclient.ErrInvalidToken)
}

func TestValidateAccessToken_Expired(t *testing.T) {
    t.Skip("requires waiting >1m for access_token TTL; deferred to TASK-007 release-readiness")
}
```

**Pre-requisites:**
- `RUN_INTEGRATION_TESTS=true` env var (gate из TASK-001 `TestMain`).
- AUTH server up (Makefile `env-up` поднимает docker-compose).
- `AUTH_ENDPOINT` env var (default `localhost:50051`, выставлен в `dev/deployments/test.env`).

**TODO comments:**
- `loginAndGetAccessToken` опирается на `(*Client).Login`, который реализуется в TASK-004. Если TASK-004 ещё не выполнен на момент запуска тестов — `t.Skip("requires TASK-004 Login")`. Альтернатива: использовать прямой gRPC-вызов к AUTH через `auth_v1.AuthServiceClient` напрямую в test-setup; но это duplicate API, не нужно. Лучше: `TestValidateAccessToken_Happy` тоже `t.Skip`-ается до TASK-004. **DOD-критерий** TASK-003 требует прохождение `_Happy` и `_Invalid` — допустить `_Happy` через `t.Skipf("blocked by TASK-004 Login API")` если TASK-004 не зафиксирован, иначе использовать его.
- `requireAuthEndpoint` (приватный helper) — читает env `AUTH_ENDPOINT`, fallback `"localhost:50051"`.

**Note on UT-0028 (no helper functions):** UT-0028 запрещает helpers для test data, mocks, assertions. `requireAuthEndpoint` — environment lookup, не test data — допустим. Альтернатива: inline вызов `os.Getenv` в каждом тесте. Inline предпочтительнее, чтобы строго соответствовать UT-0028. Удалить helper, заменить inline вызовом.

---

## Integration

### Database

N/A — SDK не работает с БД напрямую. AUTH-сервер использует PostgreSQL (поднимается через `dev/deployments/compose.test.yaml`, см. TASK-001).

### Config

Новые поля Config не добавляются — `ValidateCacheTTL` и `FailMode` уже в TASK-002. Проверить, что `applyDefaults` корректно ставит:
- `ValidateCacheTTL == 0` → default 30s (line 762 TASK-002 PRP).
- `ValidateCacheTTL < 0` → 0 → default 30s (line 759 TASK-002 PRP).

**Особый кейс:** consumer хочет ОТКЛЮЧИТЬ кэш. Сейчас по TASK-002 `applyDefaults` принудительно ставит default при ttl=0. Это в конфликте с ADR §A2 («ValidateCacheTTL=0 disables cache»). Resolution в TASK-003 не нужен — но **отметить как Open Question для TASK-007**: разрешить ли `ttl=0` означать «отключить кэш», или оставить «применить default». ADR говорит «disables». Compatible fix: трактовать только `ttl < 0` как «use default», а `ttl == 0` как «disable». **Не делаем в TASK-003**, иначе TASK-002 PRP меняется задним числом. Записать в Open Questions.

### Dependencies

| Package | Purpose | Status |
|---------|---------|--------|
| `github.com/webdelo/auth-proto/go/auth_v1` | gRPC stub `AuthServiceClient.ValidateToken` | Из TASK-001 |
| `google.golang.org/grpc/codes` | `codes.Unauthenticated/Unavailable/ResourceExhausted/Internal` | Из TASK-001 |
| `google.golang.org/grpc/metadata` | `metadata.FromIncomingContext` в interceptor | Из TASK-001 |
| `google.golang.org/grpc/status` | `status.Error`/`status.Errorf` | Из TASK-001 |
| `crypto/sha256` | hashToken | stdlib |
| `encoding/hex` | hashToken | stdlib |
| `sync` | sync.Map | stdlib |
| `net/http` | HTTPMiddleware | stdlib |
| `testing/synctest` | UT-0071 time-controlled tests | Go 1.24+ stdlib (project requires Go 1.26) |
| `github.com/stretchr/testify/mock` | Mockery Expecter API | Уже в TASK-002 |

**`go mod tidy` after this TASK:** не должна добавить новых direct require'ов (всё уже в TASK-001).

### Cross-task follow-ups (post-PRP)

**TASK-005 scope update** (consequence of Option A, см. [alternatives-analysis](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md)):

- Удалить пункт 1 WHAT (`Создать lib/context.go ...`) — это сделано в TASK-003.
- DOD пункты 1–3 (`lib/context.go ...`, `UserFromContext`, `userCtxKey`) удалить — они выполняются в TASK-003.
- TASK-005 сохраняет только пункты 2–3 WHAT (`cookies.go`, `helpers.go`) и соответствующие DOD пункты.
- Сложность TASK-005 пересчитать: с 3/10 → 2/10.

Эта правка TASK-005 spec — **post-PRP step**, выполняется отдельным mini-commit перед запуском TASK-005 execution.

---

## Testing

### Unit Tests

| Entity | Test File | Cases |
|--------|-----------|-------|
| `MemoryCache` (cache pkg) | [lib/internal/cache/memory_test.go](../../lib/internal/cache/memory_test.go) | Get/Set/Delete, lazy-expire, cleanup-goroutine-stops, zero-interval-no-goroutine, race |
| `MemoryCache.Get` | [lib/internal/cache/memory_bench_test.go](../../lib/internal/cache/memory_bench_test.go) | `BenchmarkMemoryCache_Get` (< 1µs/op) |
| `User` methods + ctx helpers | [lib/context_test.go](../../lib/context_test.go) | IsZero/IsAdmin/HasRole, WithUser, UserFromContext |
| `(*Client).ValidateAccessToken` | [lib/validate_test.go](../../lib/validate_test.go) | cache hit/miss/empty, IsValid=true→cached, IsValid=false→not cached, mapErrorReason ветки, gRPC error mapping, hashToken correctness |
| `(*Client).HTTPMiddleware` + `bearerToken` | [lib/middleware_http_test.go](../../lib/middleware_http_test.go) | 401/503/429/200 ветки, FailClosed/FailOpen, next-not-called assertion |
| `(*Client).UnaryInterceptor` | [lib/interceptor_grpc_test.go](../../lib/interceptor_grpc_test.go) | open-methods skip, code mapping, panic recovery, FailMode |

**Required Test Cases (общие):**
- [ ] Happy path для каждого метода.
- [ ] Все ветки error mapping.
- [ ] EC-0012: cleanup goroutine завершается по `cancel`.
- [ ] EC-0090: `go test -race ./lib/...` без warnings.
- [ ] UT-0040: `t.Parallel()` в каждом test/subtest, кроме panic-recovery (там shared logger state — допустимо без parallel).

### Integration Tests

| Test | File | Cases |
|------|------|-------|
| `TestValidateAccessToken_Happy` | [lib/validate_integration_test.go](../../lib/validate_integration_test.go) | Login → validate → User.UUID != "" (либо t.Skip до TASK-004) |
| `TestValidateAccessToken_Invalid` | same | random token → `ErrInvalidToken` |
| `TestValidateAccessToken_Expired` | same | `t.Skip("deferred to TASK-007")` |

`make test-integration` поднимает AUTH через docker-compose (TASK-001), запускает тесты, гасит env.

---

## Validation

### Build & Lint

```bash
# Компиляция всех файлов lib/
go build ./lib/...

# Линтер (включая bgCtx field — может потребовать //nolint только если действительно неизбежно;
# обычно настройки .golangci.yaml уже допускают контекст-поля в SDK lib-packages)
make lint

# Unit-тесты (включая race)
make test
make test-race

# Coverage по новым файлам ≥ 80%
make coverage
go tool cover -func=coverage.out | grep -E "validate|middleware|interceptor|context|internal/cache"

# Бенчмарк
go test -bench=BenchmarkMemoryCache_Get -benchmem ./lib/internal/cache/
# Expect:
# BenchmarkMemoryCache_Get-N    20000000   ~50-100 ns/op    0 allocs/op

# Mocks
make generate-mocks
ls lib/mocks/

# Integration (требует docker)
make test-integration
```

### Manual Checks

1. **Cache key contains no raw token:** `grep -E "c\.cache\.(Set|Get|Delete).*accessToken" lib/` — должен возвращать только вызовы через `hashToken(...)`-результат.
2. **Goroutine count неизменен после Close:** добавить `TestClient_NoGoroutineLeak` в `client_test.go` (если не сделан в TASK-002) — `runtime.NumGoroutine()` до `New`, `runtime.GC(); runtime.Gosched()` после `Close`, проверить что не выросло.
3. **godoc проверка:**
   ```bash
   go doc github.com/webdelo/auth-client-go/lib | grep -E "ValidateAccessToken|HTTPMiddleware|UnaryInterceptor|User|Role|WithUser|UserFromContext"
   ```
   Должны все присутствовать с комментариями.
4. **Race-stress test:** `go test -race -count=20 -run TestClient_HTTPMiddleware ./lib/` — 20 повторов, без race detect.
5. **Panic-recovery smoke:** запустить `interceptor_grpc_test.go` `panic_in_handler_returns_internal` отдельно — лог должен показать запись от `logger.Error("authclient: grpc handler panic")`.

---

## DOD (Definition of Done)

- [ ] `lib/internal/cache/memory.go` содержит `MemoryCache` с методами `Get/Set/Delete`. `NewMemoryCache(ctx, interval)` запускает cleanup goroutine только при `interval > 0`.
- [ ] `lib/context.go` экспортирует `Role`, `RoleUnknown/User/Admin`, `User struct`, методы `IsZero/IsAdmin/HasRole`, `userCtxKey` unexported, `WithUser`, `UserFromContext` (comma-ok).
- [ ] `lib/validate.go` экспортирует `(*Client).ValidateAccessToken`. Пустой токен → `ErrInvalidToken` без gRPC-вызова. Cache key — `hex(sha256(token))`.
- [ ] Positive result кэшируется в `cfg.ValidateCacheTTL`; negative (`is_valid=false`, `mapGRPCError → ErrUnavailable/ErrRateLimited`) — нет (unit-тест через моки с проверкой `Cache.Set` Times(0)).
- [ ] Cleanup goroutine завершается при `Close()`: тест `TestMemoryCache_Cleanup_goroutine_exits_when_ctx_cancelled` зелёный.
- [ ] `BenchmarkMemoryCache_Get` показывает < 1µs/op.
- [ ] `lib/middleware_http.go` возвращает 401/503/429/200 согласно ADR §A10. FailOpen режим помещает `User{}` в context и вызывает `next.ServeHTTP`.
- [ ] `lib/interceptor_grpc.go` пропускает методы из `WithOpenMethods` (unit-тест). Panic в handler → `codes.Internal` (unit-тест с `handler` который `panic("boom")`).
- [ ] `WithOpenMethods("/foo/Bar")` корректно дополняет registry — `/foo/Bar` пропускается, остальные требуют auth (unit-тест).
- [ ] `Cache` интерфейс в `lib/client.go` обновлён: `any` → `User`. `WithCache(c Cache) Option` устанавливает `c.cache` напрямую.
- [ ] `lib/client.go` имеет поле `cache Cache` на `*Client`, инициализируется default `cacheAdapter{*cache.MemoryCache}` если `cfg.Cache == nil` и `WithCache` не вызывался.
- [ ] `lib/client.go` имеет поле `bgCtx context.Context` (если не было в TASK-002 execution); инициализируется в `New` через `context.WithCancel(context.Background())`.
- [ ] `mockery.yaml` сгенерил `lib/mocks/AuthServiceClient.go`, `lib/mocks/Cache.go`, `lib/mocks/Logger.go` (если ещё не было). `make generate-mocks` идемпотентен.
- [ ] Integration-тесты: `TestValidateAccessToken_Happy` и `TestValidateAccessToken_Invalid` проходят через `make test-integration` (либо помечены `t.Skip` до TASK-004 для `_Happy`).
- [ ] Unit coverage по `validate.go` / `middleware_http.go` / `interceptor_grpc.go` / `context.go` / `internal/cache/` ≥ 80%.
- [ ] `make audit` проходит без warnings/errors.
- [ ] `go test -race ./lib/...` проходит без race detection errors.

---

## Open Questions

| # | Question | Where to resolve |
|---|----------|------------------|
| OQ-1 | `ValidateCacheTTL == 0` сейчас в `applyDefaults` (TASK-002) трактуется как «применить default». ADR §A2 говорит «disable cache». Кто прав? | TASK-007 readme + потенциально fix в `applyDefaults`. Не блокер TASK-003. |
| OQ-2 | `ErrRateLimited` без `RetryAfter` (AUTH v0.1.0 не передаёт metadata). Что в `Retry-After` header? | Сейчас — не ставим header при `d == 0`. Документировать. Можно перейти на default `1s` в будущем (additive). |

---

## Score Self-Assessment: 9/10

**Сильные стороны:**
- Полный context из ADR + research + TASK-002 PRP. Все типы, файлы, поля Client'а, интерфейсы согласованы.
- Решена implicit cross-task зависимость (TASK-005 → TASK-003) через explicit alternatives-analysis + AskUserQuestion approval.
- Каждый таск имеет concrete code patterns, references на конкретные file:line в sibling-репах.
- 18 gotchas покрывают известные подводные камни (panic ticker, lazy-expire race, EC-0034 mismatch, proto contract gaps, http.Error newline, etc.).
- DOD пункты измеримые: benchmark target, race-test, coverage, integration test names.
- Тестовая стратегия следует UT-0001/0020/0023/0026/0040 строго; mock'и через mockery + Expecter.

**Слабые стороны / Снижение на −1:**
- TASK-005 spec update (как post-PRP follow-up) — нужно отдельное действие пользователя. Идеально было бы автоматизировать через скрипт, но scope PRP не такой.
- `cacheAdapter` добавляет один уровень indirection (`any` → `User`). Можно было перенести `User` в `internal/cache` или дублировать определение — но это хуже архитектурно. Сейчас — accept небольшое overhead за clean separation. Микро-аллокации замерены в benchmark.

---

## Ссылки

- TASK-003 spec: [REQ-001-TASK-003.md](../tasks/REQ-001-TASK-003.md)
- ADR: [REQ-001-ADR.md](./REQ-001-ADR.md) (§A2, §A4, §A10, §A11, Implementation Blueprint)
- TASK-002 PRP: [REQ-001-TASK-002-PRP.md](./REQ-001-TASK-002-PRP.md)
- Code Research: [2026-05-18_code-research_REQ-001-TASK-003.md](../../.memory_bank/agent_task_results/2026-05-18_code-research_REQ-001-TASK-003.md)
- Alternatives Analysis: [2026-05-18-REQ-001-TASK-003-alternatives-analysis.md](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md)
- Sibling pattern — recovery interceptor: `/Users/a.popov/Development/TRADING/bots/signal-analyzer/internal/handlers/grpc/signal/interceptors.go:99-119`
- Sibling pattern — Bearer extraction: `/Users/a.popov/Development/TRADING/bots/auth/internal/service/httpauthmw/middleware.go:209-232`
- Sibling pattern — sha256 hashing: `/Users/a.popov/Development/TRADING/bots/auth/internal/service/tokenmanager/generate_refresh_token.go:20-21`
- Proto contract: `/Users/a.popov/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go:464-574`
- Rules cheatsheet: [.temp/rules/REQ-001-TASK-003/cheatsheet.md](../../.temp/rules/REQ-001-TASK-003/cheatsheet.md)
- Rules full text: [.temp/rules/REQ-001-TASK-003/full_rules.md](../../.temp/rules/REQ-001-TASK-003/full_rules.md)
