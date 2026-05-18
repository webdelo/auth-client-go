# ValidateAccessToken + TTL cache + HTTPMiddleware + UnaryInterceptor (server-side gRPC) + fail-mode

---

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](./REQ-001.md)

**Priority**: high
**Depends on**: REQ-001-TASK-002

---

## WHY

Hot path SDK — каждый аутентифицированный запрос consumer'а проходит через `ValidateAccessToken`. Самая критичная зона по производительности (cache hit < 1ms) и безопасности (positive-only кэш, fail-closed default, корректная инвалидация после `token_version` bump). Готовые `HTTPMiddleware` и `UnaryInterceptor` — главная ценность SDK для consumer'а, ради них и пишется обёртка.

---

## WHAT

1. **Создать `lib/internal/cache/memory.go`.** Реализовать `MemoryCache` на `sync.Map` (key `string` = `hex(sha256(token))`, value = `entry{user User, expiresAt time.Time}`). `Get` делает lazy-expire при истёкшей записи. `Set` no-op если `ttl<=0`. Реализовать `Delete` (для будущего использования). Конструктор `NewMemoryCache(bgCtx context.Context, cleanupInterval time.Duration) *MemoryCache` запускает background goroutine `cleanupLoop`, который раз в `cleanupInterval` сканирует map и удаляет истёкшие записи. Goroutine завершается по `bgCtx.Done()` (EC-0012).
2. **Создать `lib/validate.go`.** Реализовать `(*Client).ValidateAccessToken(ctx context.Context, accessToken string) (User, error)`:
    - Пустой токен → `ErrInvalidToken`.
    - Cache lookup по `hashToken(accessToken)` (приватный helper, hex sha256).
    - Cache miss → gRPC `ValidateToken` с таймаутом `cfg.ValidateTimeout`.
    - gRPC error → `mapGRPCError` → `ErrUnavailable`.
    - `resp.IsValid == false` → `mapErrorReason(resp.ErrorReason)` (НЕ кэшировать).
    - Success → собрать `User{UUID, Role}` (Email пустой — proto не возвращает, см. [REQ-001-ADR §R10](../plans/REQ-001-ADR.md)), положить в кэш с `ttl = min(cfg.ValidateCacheTTL, time-to-token-expiry)`, вернуть.
3. **Подключить кэш в `Client`.** В `New` создавать `bgCtx, cancelBG := context.WithCancel(context.Background())` и инициализировать дефолтный `cache.MemoryCache` если `cfg.Cache == nil`. `Close()` вызывает `cancelBG()` чтобы остановить cleanup goroutine.
4. **Создать `lib/middleware_http.go`.** Реализовать `(*Client).HTTPMiddleware(next http.Handler) http.Handler` согласно алгоритму [REQ-001-ADR §A10](../plans/REQ-001-ADR.md): извлечь Bearer token, вызвать `ValidateAccessToken`, обработать ошибки (`ErrUnavailable`/FailMode, `ErrRateLimited`+RetryAfter, `ErrInvalidToken|Expired|Blocked` → 401, success → `WithUser` в context). Использовать `WWW-Authenticate: Bearer realm="api"` на missing/invalid, `Retry-After` header на 503/429.
5. **Создать `lib/interceptor_grpc.go`.** Реализовать `(*Client).UnaryInterceptor() grpc.UnaryServerInterceptor` согласно алгоритму [REQ-001-ADR §A11](../plans/REQ-001-ADR.md): проверить `info.FullMethod` против `openMethods` registry, извлечь `authorization` из metadata, вызвать `ValidateAccessToken`, mapping ошибок в gRPC коды (`codes.Unauthenticated`/`Unavailable`/`ResourceExhausted`). Защита от паники через named return + `defer recover()` → `codes.Internal` (паттерн из signal-analyzer). Дефолтный openMethods registry: `["/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/Watch"]`.
6. **Подключить `WithOpenMethods` опцию.** Дополнить map в `Client.openMethods` (метод-set), используется в interceptor для skip auth.
7. **Покрыть unit-тестами.** Mockery моки для `auth_v1.AuthServiceClient`. Сценарии:
    - Cache: hit, miss, expired, positive-only (negative результат НЕ в кэше), cleanup goroutine завершается по Close (проверка через `runtime.NumGoroutine` до/после).
    - ValidateAccessToken: happy, expired, blocked, invalid, unavailable, empty token.
    - HTTPMiddleware: 401 missing/invalid, 503 FailClosed unavailable, pass-through FailOpen с пустым User, 429 + Retry-After, 200 success с User в ctx.
    - UnaryInterceptor: skip openMethods, missing metadata, invalid token, success, panic recovery.
8. **Покрыть integration-тестами против реального AUTH.** Build tag `//go:build integration`. Сценарии: `ValidateAccessToken` happy path (свежий токен после Login), expired (ждать TTL), blocked (после BlockUser admin-call → но это admin API, может быть `LogoutAll` instead), invalid (рандомная строка).
9. **Бенчмарки кэша.** `BenchmarkValidateCache_Hit` — Get на 100% hit, должен быть < 1µs/op (`go test -bench=BenchmarkValidateCache -benchmem ./lib/internal/cache/`).

### Акценты

- _Триггерные правила: EC-0010 lifecycle, EC-0012 no goroutine leak, EC-0034 (sync.Map использование), EC-0056 (unexported context keys)._
- _ES-0051, ES-0120, ES-0140._
- _Positive-only caching — заблокированный/разлогиненный пользователь должен мгновенно перестать ходить (см. [REQ-001-ADR §A2](../plans/REQ-001-ADR.md))._
- _Ключ кэша = `hex(sha256(token))`, никогда не сам токен — security (логи/метрики/дампы)._
- _`ValidateCacheTTL=0` отключает кэш полностью (для критичных consumer'ов)._
- _Negative cache miss flood mitigation — на стороне AUTH (rate limit per IP); SDK просто не кэширует negative._
- _Panic recovery в interceptor — паттерн из signal-analyzer (`handlers/grpc/signal/interceptors.go`, см. [REQ-001-ADR §A11](../plans/REQ-001-ADR.md))._
- _ValidateTokenResponse не содержит email → `User.Email == ""` после `ValidateAccessToken` (документировать в godoc, см. [REQ-001-ADR §R10](../plans/REQ-001-ADR.md))._

---

## DOD (Definition of Done)

- [ ] `lib/internal/cache/memory.go` содержит `MemoryCache` с методами `Get/Set/Delete` (реализует `lib.Cache` интерфейс).
- [ ] `lib/validate.go` экспортирует `(*Client).ValidateAccessToken`. Пустой токен возвращает `ErrInvalidToken` без gRPC-вызова.
- [ ] Positive результат кэшируется; negative (invalid/expired/blocked) НЕ кэшируется (unit-тест с моком + проверка повторного gRPC-вызова).
- [ ] Cleanup goroutine завершается при `Close()` (тест с `runtime.NumGoroutine`: число goroutines до `New` == после `Close`).
- [ ] `BenchmarkValidateCache_Hit` показывает < 1µs/op на cache hit.
- [ ] `lib/middleware_http.go` возвращает 401/503/429/200 согласно [REQ-001-ADR §A10](../plans/REQ-001-ADR.md). FailOpen режим помещает пустой `User{}` в context и вызывает `next.ServeHTTP`.
- [ ] `lib/interceptor_grpc.go` пропускает методы из `WithOpenMethods` (unit-тест). Защищён от паники: panic в handler → `codes.Internal` (unit-тест с handler, который `panic("boom")`).
- [ ] `WithOpenMethods("/foo/Bar")` корректно дополняет registry — `Bar` пропускается, остальные требуют auth.
- [ ] Integration-тесты: `TestValidateAccessToken_Happy`, `TestValidateAccessToken_Expired`, `TestValidateAccessToken_Invalid` проходят против реального AUTH через `make test-integration`.
- [ ] Unit + integration coverage по `validate.go`/`middleware_http.go`/`interceptor_grpc.go`/`internal/cache/` ≥ 80%.
- [ ] `make audit` проходит, `go test -race ./lib/...` проходит без race detection errors.
