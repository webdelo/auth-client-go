# REQ-001-ADR: Auth Client Go SDK v0.1.0 — Architecture Decision Record

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](../tasks/REQ-001.md)

| Field | Value |
|-------|-------|
| ADR ID | REQ-001-ADR-001 |
| Author | a.popov |
| Date | 2026-05-15 |
| Status | Proposed |
| Score | 9/10 (см. секцию Validation) |
| Supersedes | — (первый ADR репозитория) |

---

## Контекст

`webdelo/auth-client-go` — Go SDK v0.1.0 для микросервиса аутентификации `webdelo/auth`. Тонкая обёртка над gRPC-stub'ами из `github.com/webdelo/auth-proto/go@v0.1.0`. Consumer'ы (BFF, simple-trader, arbitrage-trader, в будущем другие Go-сервисы платформы Webdelo Trading) импортируют SDK через `import authclient "github.com/webdelo/auth-client-go/lib"` и получают типизированные методы Login/Validate/Refresh, готовые HTTP middleware и gRPC unary interceptor.

**Проблема, которую решаем.** Без SDK каждый consumer вынужден сам разбираться в нюансах AUTH-протокола: TTL-кэш для ValidateToken (без него AUTH захлёбывается), singleflight для refresh-race (без него family detection убивает легитимные сессии), правильные cookie-параметры (HttpOnly+Secure+SameSite=Strict+Path), fingerprint binding (IP+UA реального клиента, не proxy), мапинг gRPC-кодов в типизированные ошибки, fail-closed-по-умолчанию (без него — auth bypass). Каждое из решений — типовая ошибка интеграции, которую SDK закрывает в одном месте.

**Текущее состояние репо.** scratch lib bootstrap, добавлена AUTH-style инфраструктура: `task_management/`, `.claude/`, `.memory_bank/`. В `lib/` пусто кроме placeholder `lib.go` с константой `Version = "0.0.1"`. CI базовый (vet+lint+test без Docker). Ни одной публичной функции SDK ещё нет.

---

## Решение (TL;DR)

Реализуем SDK в одном Go-пакете `lib/` с минимальной публичной поверхностью (12 функций/методов + 5 sentinel-ошибок + 2 adapter-интерфейса). Архитектурные решения:

1. **Public API** — гибрид: `Config` struct для обязательных параметров (Endpoint) + functional options (`WithFailMode`, `WithLogger`, `WithCache`, `WithOpenMethods`) для опциональных toggles.
2. **ValidateToken cache** — positive-only, TTL 30s, ключ = `sha256(access_token)` (а не сам токен — для безопасности логов), внутренняя реализация в `lib/internal/cache/`, swappable через `Cache` интерфейс.
3. **Refresh race** — `golang.org/x/sync/singleflight.Group` по ключу `sha256(refresh_token)`, `Forget` сразу после успеха (см. подводный камень в pitfalls).
4. **Fail-mode** — `FailClosed` по умолчанию (OWASP A07:2025), `FailOpen` — явный opt-in с security warning в godoc.
5. **Adapter interfaces** — `Logger` (slog-style variadic fields, `NoopLogger{}` default per ES-0143), `Cache` (Get/Set/Delete с context), `Metrics` отложен до v0.2.0.
6. **gRPC connection** — один `*grpc.ClientConn` на `*Client`, через `grpc.NewClient` (не deprecated `grpc.Dial`), keepalive 30s/10s/true, retry policy через `WithDefaultServiceConfig` для `UNAVAILABLE`.
7. **Typed errors** — 5 sentinel-ошибок + `AuthError` wrapper для контекста + `RetryAfter(err)` helper. Mapping gRPC status → SDK error в приватном `mapGRPCError`.
8. **Context conventions** — unexported `type userCtxKey struct{}`, `WithUser`/`UserFromContext` (comma-ok), per EC-0056.
9. **Cookie defaults** — HttpOnly+Secure+SameSite=Strict+Path=`/auth/refresh`+MaxAge=14d, override через `CookieOption` functional options.
10. **HTTP middleware** — извлекает `Authorization: Bearer <token>`, 401+`WWW-Authenticate: Bearer` на missing/invalid, 503 на FailClosed-unavailable, pass-through (с пустым User в ctx) на FailOpen.
11. **gRPC interceptor** — `UnaryServerInterceptor`, метаданные `authorization`, `WithOpenMethods` registry для пропуска health/login methods, default whitelist включает `/grpc.health.v1.Health/Check`.
12. **Fingerprint binding** — SDK обязывает consumer'а передавать IP/UA через `IPFromRequest(r)` / `UserAgentFromRequest(r)`. Алгоритм IP: `X-Real-IP` → `X-Forwarded-For[0]` → `RemoteAddr`. **Без** trusted-proxies-валидации в v0.1.0 (отложено до v0.2.0 как additive feature).
13. **Integration testing** — отдельный target `make test-integration`, build tag `//go:build integration`, AUTH поднимается через `dev/deployments/compose.test.yaml`. В CI — опционально (требует доступа к docker).
14. **Без JWT-парсинга на стороне SDK** — каждая валидация идёт в AUTH (с кэшем). Нет state ключей, нет риска рассинхрона при ротации (auth REQ-001 SEC-10).
15. **Без deprecated API** — `grpc.NewClient`, `golang.org/x/sync/singleflight` (не самописный), `errors.New + Unwrap` (стандарт). Никаких `nolint:staticcheck` (ES-0171).

---

## Описание

### Концепт

**Цель.** Дать Go-сервисам платформы один импорт, после которого правильная работа с AUTH «получается сама собой». Не заменить AUTH, а инкапсулировать неочевидные детали интеграции.

**Dataflow на уровне SDK.**

```
┌─────────────────────────────────────────────────┐
│                    Consumer                      │
│            (BFF / arbitrage / etc.)              │
│  ┌─────────────────────────────────────────┐    │
│  │   import authclient "github.com/        │    │
│  │              webdelo/auth-client-go/lib"│    │
│  └─────────────────────────────────────────┘    │
│                       │                          │
│   ┌───────────┬───────┴──────┬────────────┐     │
│   │HTTPMidlwre│UnaryIntcptr │auth methods│     │
│   └───────────┴──────────────┴────────────┘     │
│                       │                          │
│   ┌───────────┐  ┌────┴──────┐  ┌────────────┐  │
│   │  TTL Cache│  │singleflght│  │error mapper│  │
│   │ (positive)│  │  (refresh)│  │(gRPC→typed)│  │
│   └───────────┘  └───────────┘  └────────────┘  │
│                       │                          │
│            ┌──────────┴───────────┐              │
│            │  *grpc.ClientConn    │              │
│            │  (single, keepalive) │              │
│            └──────────┬───────────┘              │
└───────────────────────┼──────────────────────────┘
                        │ gRPC (auth.v1)
                        ▼
              ┌──────────────────┐
              │   webdelo/auth   │
              │   (server)       │
              └──────────────────┘
```

**Ключевые потоки.**

- **Validate (hot path):** `client.ValidateAccessToken(ctx, token)` → cache lookup по `sha256(token)` → hit: return User; miss: gRPC `ValidateToken` → если `is_valid=true`: положить в cache + return; если `is_valid=false`: НЕ кэшировать, return `ErrInvalidToken|ErrExpired|ErrBlocked` в зависимости от `error_reason`.
- **Login:** `client.Login(ctx, LoginInput{Email, Password, ClientIP, UserAgent})` → gRPC `Login` → `LoginOutput{AccessToken, RefreshToken, AccessExpiresIn, RefreshExpiresIn, User}`.
- **Refresh (race-protected):** `client.Refresh(ctx, refreshToken, ip, ua)` → `singleflight.Do(sha256(refreshToken), gRPC RefreshToken)` → `Forget(sha256(refreshToken))` после успеха → `RefreshOutput{новый AccessToken, новый RefreshToken (rotated), expirations}`.
- **HTTP middleware:** `r → Authorization: Bearer X → ValidateAccessToken(X) → user → WithUser(ctx, user) → next.ServeHTTP(w, r.WithContext(ctx))`. Ошибки: missing/invalid → 401 + `WWW-Authenticate: Bearer realm="api"`; ErrUnavailable + FailClosed → 503; ErrUnavailable + FailOpen → pass-through с `User{}` в ctx.

### Архитектура

#### A1. Public API surface (Config + Options гибрид)

**Решение:** `Config` struct содержит обязательные/часто-меняемые параметры (Endpoint, ValidateCacheTTL, FailMode, timeouts, DialOptions). Функциональные `Option`-функции (`WithFailMode`, `WithLogger`, `WithCache`, `WithOpenMethods`) применяются после Config и переопределяют поля.

**Обоснование:**
- Endpoint — обязателен, без него `New` обязан вернуть ошибку → struct field удобнее (zero-value-проверка).
- Опциональные toggles типа FailMode и регистры (OpenMethods) — orthogonal concerns, добавление новых не должно ломать сигнатуру `New` → functional options.
- Прецеденты: `aws-sdk-go-v2`, `mongo-go-driver`, `grpc-ecosystem/go-grpc-middleware/auth`.
- Соответствует `ES-0120` (functional options) и `ES-0140` (`New()`, не `NewClient`).

**Mockup финальной поверхности:** см. секцию «API» ниже (полный `lib/api.go`).

#### A2. ValidateToken cache

**Решение:**
- **Только положительные результаты.** `is_valid=true` идут в кэш, `is_valid=false` — никогда. Заблокированный/разлогиненный пользователь должен мгновенно перестать ходить.
- **Ключ = `hex(sha256(access_token))`** — сам токен в логах кэша/метриках видеть нельзя (security).
- **Значение = `(User, expiresAt time.Time)`** — `expiresAt = min(cached_at + TTL, access_token_expires_at)`. Кэш не переживает истечение самого токена.
- **TTL по умолчанию 30s, диапазон [0, 5m].** `ValidateCacheTTL=0` отключает кэш (каждый запрос → AUTH). Минимум для production — 5s, не enforce'им (только в README).
- **Реализация в `lib/internal/cache/`:** `sync.Map` + lazy-expire при `Get` + background goroutine, чистящий tombstones каждые `TTL/2`. Goroutine завязан на `context.Context`, который закрывается в `Client.Close()` (EC-0010 lifecycle, EC-0012 no leak).
- **Swappable через `Cache` интерфейс.** Дефолт — `internal/cache.MemoryCache`. Consumer может передать Redis-implementation через `WithCache(myCache)`.
- **TTL-инвалидация по token_version.** Когда AUTH инкрементирует `token_version` (LogoutAll, BlockUser, ChangePassword), все access-tokens начинают возвращать `is_valid=false`. Кэш SDK подхватит при следующем lookup'е после истечения TTL. Худший случай — 30s жизни старого токена в кэше. Документируется в Risk #4.

**Что НЕ делаем:**
- Не парсим JWT — нет необходимости знать ключ, нет risk рассинхрона при ротации (auth SEC-10).
- Не подписываемся на push-уведомления от AUTH — overengineering для v0.1.0.
- Не кэшируем negative results — security trade-off (см. выше).

#### A3. Singleflight для Refresh

**Решение:**
- `golang.org/x/sync/singleflight.Group` в `lib/internal/refresh/`.
- Ключ: `hex(sha256(refresh_token))`.
- API: `singleflight.Do(key, func() (interface{}, error) { return c.refreshTokenRPC(ctx, ...) })`.
- **`Forget(key)` сразу после успеха** — иначе следующий вызов дождётся завершённой (и уже не актуальной) операции. Подводный камень из web-research.
- Ошибка одного вызова шарится со всеми ожидающими — это OK (если refresh упал — все ожидающие должны это узнать).
- Cancellation: singleflight НЕ отменяет `fn`, когда caller'ский context отменяется. Это OK — refresh должен довыполниться, новый refresh_token выдан, иначе family detection в AUTH сработает на следующем валидном refresh.

**Ограничение:** работает только в пределах одного процесса SDK. Если consumer запускает несколько replicas BFF — singleflight в одной replica не видит refresh в другой. **Accepted limitation для v0.1.0**, фиксируется в README. Distributed-singleflight через Redis — потенциальная фича для v0.2.0+ (additive).

**Альтернатива, отклонённая:** singleflight по `userID`. Web-search рекомендовал это, но user_id извлекается только после refresh (refresh_token opaque). По `sha256(refresh_token)` мы покрываем основной кейс — две вкладки шлют один и тот же refresh_token в окне миллисекунд.

#### A4. Fail-mode

**Решение:**
- `FailClosed` (default) — при `ErrUnavailable` (gRPC error, timeout, network) middleware возвращает 503 + `Retry-After: 5`.
- `FailOpen` — явный opt-in. При unavailable: middleware пропускает запрос с `User{}` в ctx, логгирует Warn. Использовать только для health checks под аутентификацией или эндпоинтов с собственной авторизацией.

**Обоснование:** OWASP Top 10 2025 A07 Authentication Failures — fail-open — намеренно эксплуатируемая уязвимость. Default fail-closed устраняет class of bugs «забыли проверить, что AUTH доступен».

**API:**
```go
authclient.New(authclient.Config{Endpoint: "auth:50051"})            // FailClosed
authclient.New(authclient.Config{...}, authclient.WithFailMode(authclient.FailOpen))
```

В godoc на `WithFailMode(FailOpen)` — security warning capslock-style.

#### A5. Adapter interfaces

**Logger:**
```go
type Logger interface {
    Info(ctx context.Context, msg string, fields ...any)
    Warn(ctx context.Context, msg string, fields ...any)
    Error(ctx context.Context, msg string, err error, fields ...any)
}
```
Variadic `...any` совместим с `slog.Attr`-pairs / `zap.Field` после простого адаптера. Дефолт — `NoopLogger{}` (per ES-0143). Consumer может передать обёртку над `webdelo/go-ctx-logger`, но SDK об этом не знает.

**Cache:**
```go
type Cache interface {
    Get(ctx context.Context, key string) (user User, ok bool)
    Set(ctx context.Context, key string, user User, ttl time.Duration)
    Delete(ctx context.Context, key string)
}
```
Context обязателен — Redis-implementation делает network call. Без `Stats()` / `Size()` — implementation detail. Дефолт — `internal/cache.MemoryCache`.

**Metrics:** **отложено до v0.2.0.** Counter/histogram/labels — нетривиальный design, лучше дозреть. Добавление будет additive (новый `WithMetrics(m)` + новый интерфейс).

#### A6. gRPC connection management

**Решение:**
- Один `*grpc.ClientConn` на `*authclient.Client`. gRPC сам мультиплексирует по HTTP/2.
- `grpc.NewClient(endpoint, opts...)` — **не** deprecated `grpc.Dial`. Никаких `nolint:staticcheck` (ES-0171).
- Дефолтные опции (зашиты в SDK):
  - `grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30s, Timeout: 10s, PermitWithoutStream: true})` — поддержание соединения через файрволы.
  - `grpc.WithDefaultServiceConfig(...)` с retry policy для `UNAVAILABLE` (max 3 retries, exponential backoff 100ms→1s).
  - **TLS не зашит:** дефолт — `grpc.WithTransportCredentials(insecure.NewCredentials())` (dev). Production: consumer передаёт `WithTransportCredentials(credentials.NewTLS(...))` через `Config.DialOptions`. README про это явно говорит.
- `Client.Close()` обязателен — graceful shutdown (закрывает conn, останавливает cache cleanup goroutine). Идемпотентен.

**Pitfall (документируется в README):** `keepalive.ClientParameters{Time}` < 10s grpc-go молча подменяет на 10s без ошибки.

#### A7. Typed errors

**Решение:**
```go
var (
    ErrInvalidToken = errors.New("authclient: invalid token")
    ErrExpired      = errors.New("authclient: token expired")
    ErrBlocked      = errors.New("authclient: user blocked")
    ErrRateLimited  = errors.New("authclient: rate limited")
    ErrUnavailable  = errors.New("authclient: service unavailable")
)
```

**Mapping gRPC status → SDK error** (приватный `func mapGRPCError(err error) error`):

| gRPC code | error_reason (если есть) | SDK error |
|-----------|--------------------------|-----------|
| `OK` (но `is_valid=false`) | `"expired"` | `ErrExpired` |
| `OK` (но `is_valid=false`) | `"blocked"` | `ErrBlocked` |
| `OK` (но `is_valid=false`) | `"invalid"`/`""` | `ErrInvalidToken` |
| `Unauthenticated` | — | `ErrInvalidToken` |
| `PermissionDenied` | — | `ErrBlocked` (для нашего scope == blocked user) |
| `ResourceExhausted` | — | `ErrRateLimited` (с `RetryAfter` из metadata, если есть) |
| `Unavailable` / `DeadlineExceeded` / network | — | `ErrUnavailable` |
| любой другой | — | `ErrUnavailable` (с wrap для godebug) |

**RetryAfter wrapper:**
```go
type AuthError struct {
    Err        error
    RetryAfter time.Duration  // только для ErrRateLimited
}
func (e *AuthError) Error() string { return e.Err.Error() }
func (e *AuthError) Unwrap() error { return e.Err }

func RetryAfter(err error) time.Duration {
    var ae *AuthError
    if errors.As(err, &ae) { return ae.RetryAfter }
    return 0
}
```

Caller branching: `if errors.Is(err, authclient.ErrRateLimited) { time.Sleep(authclient.RetryAfter(err)); ... }`.

**Status extraction из обёрнутых ошибок:** стандартный `status.FromError` НЕ ходит по `errors.Unwrap` chain. Используем приватный `statusFromWrapped(err error) (*status.Status, bool)`, который итерирует через `errors.Unwrap` и пытается `status.FromError` на каждом уровне (паттерн из signal-analyzer per code-research).

#### A8. Context conventions

```go
type userCtxKey struct{}  // unexported — EC-0056

func WithUser(ctx context.Context, user User) context.Context {
    return context.WithValue(ctx, userCtxKey{}, user)
}

func UserFromContext(ctx context.Context) (User, bool) {
    user, ok := ctx.Value(userCtxKey{}).(User)
    return user, ok
}
```

`comma-ok` идиома — стандарт Go для optional значений. Caller: `user, ok := authclient.UserFromContext(r.Context()); if !ok { ... }`.

#### A9. Cookie helpers

**Дефолтные параметры refresh-cookie:**
- `Name: "refresh_token"`
- `HttpOnly: true` — XSS защита.
- `Secure: true` — только HTTPS.
- `SameSite: http.SameSiteStrictMode` — CSRF защита.
- `Path: "/auth/refresh"` — отдаётся ТОЛЬКО на этот эндпоинт (минимизация attack surface).
- `MaxAge: 14*24*60*60` — 14 дней (синхронизировано с refresh_token TTL в AUTH).
- `Domain: ""` — host-only (не делиться между поддоменами без явной опции).

**API:**
```go
func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)
func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)

type CookieOption func(*http.Cookie)

func WithCookieDomain(domain string) CookieOption
func WithCookiePath(path string) CookieOption          // override default "/auth/refresh"
func WithCookieMaxAge(maxAge time.Duration) CookieOption
func WithCookieSameSite(mode http.SameSite) CookieOption
```

Override через `CookieOption` для multi-domain setup'ов (когда BFF и AUTH на разных поддоменах одного домена и нужен `Domain: ".webdelo.trade"`).

**Frozen после v1.0.0** — изменение defaults = security regression.

#### A10. HTTP middleware

**Алгоритм `client.HTTPMiddleware(next).ServeHTTP(w, r)`:**

1. `authHeader := r.Header.Get("Authorization")`
2. Если пусто или не начинается с `"Bearer "`: `w.Header().Set("WWW-Authenticate", "Bearer realm=\"api\"")`; `http.Error(w, "missing token", 401)`; return.
3. `token := strings.TrimPrefix(authHeader, "Bearer ")`.
4. `user, err := c.ValidateAccessToken(r.Context(), token)`.
5. Если `errors.Is(err, ErrUnavailable)`:
   - FailClosed: `w.Header().Set("Retry-After", "5")`; `http.Error(w, "auth unavailable", 503)`; return.
   - FailOpen: log Warn; `next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), User{})))`; return.
6. Если `errors.Is(err, ErrInvalidToken|ErrExpired|ErrBlocked)`: `w.Header().Set("WWW-Authenticate", "Bearer realm=\"api\", error=\"invalid_token\"")`; `http.Error(w, "invalid token", 401)`; return.
7. Если `errors.Is(err, ErrRateLimited)`: `w.Header().Set("Retry-After", strconv.Itoa(int(RetryAfter(err).Seconds())))`; `http.Error(w, "rate limited", 429)`; return.
8. Success: `next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))`.

**Не сериализуем в JSON body** — оставляем `text/plain`, чтобы не навязывать формат ошибок consumer'у. Consumer может обернуть middleware своим, который перепакует 401/403/429/503 в свой error format.

#### A11. gRPC unary server interceptor

**Алгоритм `client.UnaryInterceptor()`:**

1. Извлечь `info.FullMethod` (e.g., `"/myapp.v1.UserService/GetMe"`).
2. Если method в `openMethods` registry → `return handler(ctx, req)` (skip auth).
3. Извлечь metadata: `md, ok := metadata.FromIncomingContext(ctx)`. Если no metadata: return `status.Error(codes.Unauthenticated, "missing metadata")`.
4. `authValues := md.Get("authorization")` (lower-case стандарт). Если пусто или не Bearer: return `codes.Unauthenticated`.
5. `user, err := c.ValidateAccessToken(ctx, token)` → mapping:
   - `ErrUnavailable` + FailClosed: `codes.Unavailable`.
   - `ErrUnavailable` + FailOpen: `handler(WithUser(ctx, User{}), req)`.
   - `ErrInvalidToken|ErrExpired|ErrBlocked`: `codes.Unauthenticated`.
   - `ErrRateLimited`: `codes.ResourceExhausted`.
6. Success: `handler(WithUser(ctx, user), req)`.
7. **Защита от паники:** named return `(resp interface{}, err error)` + `defer func() { if r := recover(); r != nil { err = status.Errorf(codes.Internal, "panic: %v", r) } }()` (паттерн из signal-analyzer/handlers/grpc/signal/interceptors.go).

**`WithOpenMethods(methods ...string)`** — функциональная опция, наполняет registry. Default: `[]string{"/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/Watch"}`.

#### A12. Fingerprint binding (helpers)

```go
func IPFromRequest(r *http.Request) string
func UserAgentFromRequest(r *http.Request) string
```

**`IPFromRequest` алгоритм (для v0.1.0 — без trusted-proxies):**
1. `r.Header.Get("X-Real-IP")` — если непусто, return.
2. `r.Header.Get("X-Forwarded-For")` — если непусто, взять первый IP до запятой, return.
3. `host, _, _ := net.SplitHostPort(r.RemoteAddr)`; return host.

**Зачем заставляем consumer вызывать вручную, а не делать автоматически?**
- Если SDK сам берёт `r.RemoteAddr` для `Login`/`Refresh` — AUTH получит IP BFF, а не реального клиента → fingerprint бесполезен (OWASP).
- Consumer знает свою proxy-конфигурацию (Cloudflare/nginx/k8s ingress кладут IP в разные хедеры).
- В v0.1.0 — best-effort + честная док про spoofing-риски.
- В v0.2.0 — `WithTrustedProxies(cidrs ...netip.Prefix)` (additive).

**Что AUTH делает с IP:** `fingerprint = SHA-256(UserAgent + first-3-octets(IP))`. Refresh, выданный для `203.0.113.55`, не сработает на `198.51.100.1` (другая /24), но сработает на `203.0.113.99` (та же /24). За корпоративным NAT работает.

#### A13. Integration testing infrastructure

**Что добавляем (в TASK-001):**
- `dev/deployments/compose.test.yaml` — поднимает AUTH (image из локально-собранного `webdelo/auth` или GHCR) + PostgreSQL.
- `dev/deployments/test.env` — `AUTH_ENDPOINT=localhost:50051`, секреты для AUTH.
- Makefile targets: `env-up`, `env-down`, `test-integration`.
  - `test-integration` → `env-up` → ждёт healthcheck → `go test -tags=integration ./lib/...` → `env-down` (через trap).
- Build tag `//go:build integration` на интеграционных тестах (`lib/client_integration_test.go`).
- Go-нативные тесты (не gonkey) — для SDK удобнее писать на Go.

**CI:** в `.github/workflows/ci.yaml` — добавить опциональный job `integration` (matrix entry с `if: ${{ inputs.run_integration }}`). Обязательный — только локально. Caveat: AUTH-image сейчас не публикуется → отдельный devops-REQ (не блокер для v0.1.0).

#### A14. Без JWT-парсинга на стороне SDK

**Решение:** SDK НИКОГДА не парсит access_token локально. Каждая валидация (без cache hit) идёт через gRPC `ValidateToken`. Обоснование:
- Локальная JWT-валидация требует знать публичный ключ AUTH и обновлять его при ротации (auth REQ-001 SEC-10).
- Добавляет state в SDK, риск рассинхрона ключей.
- Не даёт ничего сверх ValidateToken-кэша (тот же 30s window для invalidation).

#### A15. Никакого deprecated API

- `grpc.NewClient` (не `grpc.Dial`).
- `golang.org/x/sync/singleflight` (не самописный mutex+map).
- `errors.New` + `errors.Is` + `errors.Unwrap` (стандарт, не самописный errors-пакет).
- Никаких `nolint:staticcheck` (ES-0171).

---

### Dataflow

**Login flow (BFF → AUTH через SDK):**

```
HTTP POST /auth/login
   { email, password }
        │
        ▼
BFF handler:
  in := authclient.LoginInput{
    Email:     req.Email,
    Password:  req.Password,
    ClientIP:  authclient.IPFromRequest(r),
    UserAgent: authclient.UserAgentFromRequest(r),
  }
  out, err := client.Login(ctx, in)
        │
        ▼
SDK Login:
  gRPC Call → AuthService.Login(LoginRequest{
    email: in.Email, password: in.Password,
    ip: in.ClientIP, user_agent: in.UserAgent,
  })
        │
        ▼
AUTH server:
  - validate creds
  - issue access+refresh tokens
  - store session (with fingerprint)
  - return LoginResponse
        │
        ▼
SDK:
  - mapGRPCError(err) if err
  - convert protobuf → LoginOutput{
      AccessToken, RefreshToken,
      AccessExpiresIn:  time.Until(time.Unix(resp.AccessTokenExpiresAt, 0)),
      RefreshExpiresIn: time.Until(time.Unix(resp.RefreshTokenExpiresAt, 0)),
      User: User{UUID, Email, Role: Role(resp.User.Role)},
    }
        │
        ▼
BFF:
  authclient.SetRefreshCookie(w, out.RefreshToken)
  json.NewEncoder(w).Encode(map{
    "access_token": out.AccessToken,
    "expires_in":   out.AccessExpiresIn.Seconds(),
    "user":         out.User,
  })
```

**Validate flow (hot path — каждый запрос):**

```
HTTP GET /api/something
   Authorization: Bearer ABC...
        │
        ▼
HTTPMiddleware:
  token := "ABC..."
  user, err := client.ValidateAccessToken(ctx, token)
        │
        ▼
SDK ValidateAccessToken:
  key := hex(sha256(token))
  if user, ok := cache.Get(ctx, key); ok {
      return user, nil   ◄── HOT PATH (>95% requests)
  }
  resp, err := authClient.ValidateToken(ctx, &ValidateTokenRequest{AccessToken: token})
  if err != nil {
      return User{}, mapGRPCError(err)   // ErrUnavailable
  }
  if !resp.IsValid {
      return User{}, mapErrorReason(resp.ErrorReason)   // ErrExpired/ErrBlocked/ErrInvalidToken (NOT cached)
  }
  user := User{UUID: resp.UserUuid, Role: Role(resp.Role)}
  cache.Set(ctx, key, user, c.cfg.ValidateCacheTTL)
  return user, nil
        │
        ▼
HTTPMiddleware:
  ctx := authclient.WithUser(r.Context(), user)
  next.ServeHTTP(w, r.WithContext(ctx))
        │
        ▼
Handler:
  user, _ := authclient.UserFromContext(r.Context())
  // ... business logic
```

**Refresh flow (race-protected):**

```
Tab A and Tab B simultaneously:
  POST /auth/refresh
  Cookie: refresh_token=XYZ...
        │
        ▼
BFF:
  refreshToken, _ := r.Cookie("refresh_token")
  out, err := client.Refresh(ctx, refreshToken.Value,
                             authclient.IPFromRequest(r),
                             authclient.UserAgentFromRequest(r))
        │
        ▼
SDK Refresh:
  key := hex(sha256(refreshToken))
  v, err, _ := sf.Do(key, func() (interface{}, error) {
      return c.refreshTokenRPC(ctx, refreshToken, ip, ua)
  })
  defer sf.Forget(key)         // ОБЯЗАТЕЛЬНО — иначе следующий refresh ждёт stale result
        │
        ▼
gRPC call goes ONCE (Tab B waits for Tab A's response):
  AuthService.RefreshToken(...)
        │
        ▼
AUTH:
  - validate fingerprint (IP/24 + UA)
  - rotate refresh token (issue new, invalidate old)
  - return RefreshTokenResponse
        │
        ▼
Both Tab A and Tab B get same RefreshOutput.
BFF sets new refresh_token cookie for both.
```

### Влияние на существующую функциональность

**N/A.** REQ-001 — первый функциональный REQ репозитория. Текущий код (`lib/lib.go` с константой `Version`) будет заменён полным SDK. Никаких consumers ещё нет (BFF будет интегрирован в отдельном REQ в репозитории BFF после релиза `v0.1.0`).

### Нагрузка

**Тип SDK:** клиентская библиотека, нагрузка определяется consumer'ом.

**Ожидаемый профиль (из Webdelo Trading контекста):**
- BFF: ~50–500 RPS на ValidateAccessToken (every authenticated request). Cache hit ratio target: >95% (TTL 30s + типичный пользователь делает >10 запросов/мин).
- Login: <1 RPS (login event редкий).
- Refresh: <5 RPS (раз в ~15 мин на пользователя).

**SLA:**
- ValidateAccessToken cache hit: <1ms (in-process map lookup).
- ValidateAccessToken cache miss: <50ms (gRPC roundtrip + AUTH JWT verify).
- Login/Refresh: <200ms (gRPC + AUTH bcrypt verify ~100ms).
- Negative cache miss flood (если кто-то DDoS'ит фейк-токенами): SDK не кэширует negative → нагрузка идёт в AUTH. **Митигация на стороне AUTH** (rate limit per IP, см. auth REQ).

**Требования по железу:** SDK добавляет к consumer: ~10MB heap (cache + grpc connection state) + 1 background goroutine (cache cleaner). Negligible.

**Деградация:** при недоступности AUTH — FailClosed возвращает 503 (consumer показывает «сервис временно недоступен»). FailOpen — пропускает с пустым User (только для not-critical эндпоинтов).

### Логирование и мониторинг

**Через adapter-интерфейсы.** SDK не зависит от конкретного логгера/метрик-системы.

**Что SDK логирует (через `Logger` adapter):**
- `Info`: Login/Logout/LogoutAll/ChangePassword success (с user UUID, без email/IP).
- `Info`: Refresh success (с user UUID).
- `Warn`: cache miss → AUTH unavailable (FailOpen pass-through).
- `Warn`: rate limited (с retry-after).
- `Error`: gRPC connection failures, panics in interceptor.

**Метрики (отложено до v0.2.0):**
- `authclient_validate_total{result="hit|miss|invalid|expired|blocked|error"}`
- `authclient_validate_duration_seconds{result="..."}` histogram
- `authclient_login_total{result="success|invalid_creds|rate_limited|error"}`
- `authclient_refresh_total{result="success|invalid|rate_limited|error"}`
- `authclient_grpc_unavailable_total`
- `authclient_singleflight_dedupes_total`

**Алерты:** consumer-side, не SDK. Рекомендации в README:
- `authclient_grpc_unavailable_total[5m] > 0` → AUTH down → page oncall.
- cache hit rate < 80% → возможно expired tokens / TTL слишком маленький.

**Диагностика проблем:** SDK логи идут в consumer-logger → consumer-side observability (ELK/Loki/etc.).

### API (публичная Go-поверхность)

Полная сигнатура `lib/api.go` (mockup, godoc сокращён — полные комментарии в реализации):

```go
// Package authclient is the official Go SDK for the webdelo/auth service.
package authclient

import (
    "context"
    "errors"
    "net/http"
    "time"

    "google.golang.org/grpc"
)

// ─── Constructor & Config ─────────────────────────────────────────────────

type Config struct {
    Endpoint         string             // required: "host:port"
    ValidateCacheTTL time.Duration      // default 30s; 0 disables cache
    FailMode         FailMode           // default FailClosed
    ValidateTimeout  time.Duration      // default 2s
    LoginTimeout     time.Duration      // default 5s
    RefreshTimeout   time.Duration      // default 5s
    DialOptions      []grpc.DialOption  // appended after SDK defaults
    Logger           Logger             // default NoopLogger{}
    Cache            Cache              // default in-memory TTL cache
}

type FailMode uint8
const (
    FailClosed FailMode = iota
    FailOpen
)

type Option func(*Client)

func WithFailMode(mode FailMode) Option
func WithLogger(l Logger) Option
func WithCache(c Cache) Option
func WithOpenMethods(methods ...string) Option   // gRPC method whitelist

func New(cfg Config, opts ...Option) (*Client, error)

type Client struct{ /* unexported */ }
func (c *Client) Close() error

// ─── Validate ────────────────────────────────────────────────────────────

func (c *Client) ValidateAccessToken(ctx context.Context, accessToken string) (User, error)

// ─── Auth flow ───────────────────────────────────────────────────────────

type LoginInput struct {
    Email     string
    Password  string
    ClientIP  string
    UserAgent string
}

type LoginOutput struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresIn  time.Duration
    RefreshExpiresIn time.Duration
    User             User
}

type RefreshOutput struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresIn  time.Duration
    RefreshExpiresIn time.Duration
}

func (c *Client) Login(ctx context.Context, in LoginInput) (LoginOutput, error)
func (c *Client) Refresh(ctx context.Context, refreshToken, clientIP, userAgent string) (RefreshOutput, error)
func (c *Client) Logout(ctx context.Context, refreshToken string) error
func (c *Client) LogoutAll(ctx context.Context, accessToken string) error
func (c *Client) ChangePassword(ctx context.Context, accessToken, currentPassword, newPassword string) error
func (c *Client) GetCurrentUser(ctx context.Context, accessToken string) (User, error)

// ─── Middleware ──────────────────────────────────────────────────────────

func (c *Client) HTTPMiddleware(next http.Handler) http.Handler
func (c *Client) UnaryInterceptor() grpc.UnaryServerInterceptor

// ─── User & Context ──────────────────────────────────────────────────────

type Role string
const (
    RoleUnknown Role = ""
    RoleUser    Role = "user"
    RoleAdmin   Role = "admin"
)

type User struct {
    UUID  string
    Role  Role
    Email string  // populated by Login/GetCurrentUser; "" after ValidateAccessToken (proto limit)
}

func (u User) IsZero() bool
func (u User) IsAdmin() bool
func (u User) HasRole(role Role) bool

func WithUser(ctx context.Context, user User) context.Context
func UserFromContext(ctx context.Context) (User, bool)

// ─── Cookie helpers ──────────────────────────────────────────────────────

type CookieOption func(*http.Cookie)
func WithCookieDomain(domain string) CookieOption
func WithCookiePath(path string) CookieOption
func WithCookieMaxAge(maxAge time.Duration) CookieOption
func WithCookieSameSite(mode http.SameSite) CookieOption

func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)
func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)

// ─── HTTP request helpers ────────────────────────────────────────────────

func IPFromRequest(r *http.Request) string
func UserAgentFromRequest(r *http.Request) string

// ─── Errors ──────────────────────────────────────────────────────────────

var (
    ErrInvalidToken = errors.New("authclient: invalid token")
    ErrExpired      = errors.New("authclient: token expired")
    ErrBlocked      = errors.New("authclient: user blocked")
    ErrRateLimited  = errors.New("authclient: rate limited")
    ErrUnavailable  = errors.New("authclient: service unavailable")
)

func RetryAfter(err error) time.Duration

// ─── Adapter interfaces ──────────────────────────────────────────────────

type Logger interface {
    Info(ctx context.Context, msg string, fields ...any)
    Warn(ctx context.Context, msg string, fields ...any)
    Error(ctx context.Context, msg string, err error, fields ...any)
}

type NoopLogger struct{}
func (NoopLogger) Info(context.Context, string, ...any)         {}
func (NoopLogger) Warn(context.Context, string, ...any)         {}
func (NoopLogger) Error(context.Context, string, error, ...any) {}

type Cache interface {
    Get(ctx context.Context, key string) (user User, ok bool)
    Set(ctx context.Context, key string, user User, ttl time.Duration)
    Delete(ctx context.Context, key string)
}
```

**Эволюция API:**

| Запрещено после v1.0.0 (frozen) | Разрешено после v1.0.0 (additive) |
|----------------------------------|-----------------------------------|
| Сигнатуры всех `func` на `*Client` | Новые методы (`StreamInterceptor`, `RefreshFromRequest`) |
| Имена/значения констант (Role*, FailMode*) | Новые роли (`RoleModerator` etc.) |
| Имена sentinel-ошибок | Новые sentinel-ошибки |
| Имена/типы exported полей в struct | Новые поля в struct (zero-value-compatible) |
| Сигнатуры методов в Logger/Cache | Новые `WithXxx` опции |
| Cookie defaults (security regression) | `Metrics` interface + `WithMetrics` |
|  | `WithTrustedProxies` для IPFromRequest |

### Разработка и деплой

**SDK не деплоится** — это библиотека. Распространение через Go modules:
- Тег `v0.1.0` в `webdelo/auth-client-go`.
- Consumer: `go get github.com/webdelo/auth-client-go@v0.1.0`.

**Тестовое окружение:**
- Unit-тесты: `make test` / `make test-race` (без зависимостей).
- Integration-тесты: `make test-integration` (поднимает AUTH через `dev/deployments/compose.test.yaml`).

**Сетевые доступы для consumer (production):**
- gRPC до AUTH (TLS, порт 50051 по умолчанию).
- Никаких других — SDK self-contained.

**CI:**
- `vet`, `lint`, `test`, `build-examples` — обязательно.
- `test-integration` — опционально (gated by Docker availability в CI runner).

---

## All Needed Context

### Documentation & References

```yaml
# === Critical context ===

- file: ~/Development/TRADING/bots/auth-client-go/task_management/tasks/REQ-001.md
  why: Source REQ — все архитектурные решения, риски, DOD, декомпозиция на TASK-001..TASK-007.

- file: ~/Development/TRADING/bots/auth-proto/proto/auth_service.proto
  why: gRPC контракт. 13 RPC, 7 в scope SDK, 6 admin/sessions — out of scope.
  critical: |
    - ValidateToken НИКОГДА не возвращает gRPC error для invalid token —
      возвращает ValidateTokenResponse{is_valid: false, error_reason: "..."}.
      gRPC error означает только недоступность сервиса.
    - ValidateTokenResponse содержит только user_uuid + role + error_reason.
      Email НЕ возвращается → User{Email} пустой после ValidateAccessToken.
    - Все timestamps в Unix seconds UTC.

- file: ~/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go
  why: Сгенерированные Go bindings — что мы оборачиваем.

- file: .memory_bank/agent_task_results/2026-05-15_code-research_codebase_analysis_REQ-001-ADR.yaml
  why: Готовые паттерны из fundingbot/signal-analyzer для cache+singleflight, error mapping, gRPC interceptor с recover.

- file: .memory_bank/agent_task_results/web_research_REQ-001-ADR.yaml
  why: External validation — keepalive params, singleflight gotchas, OWASP fail-closed.

- file: .memory_bank/agent_task_results/api_design_REQ-001-ADR.yaml
  why: Полный API mockup, evolution guidelines, 13 design decisions.

# === gRPC ===

- url: https://github.com/grpc/grpc-go/blob/master/Documentation/concurrency.md
  section: ClientConn
  critical: |
    grpc.ClientConn safe for concurrent use by multiple goroutines.
    Один conn на process, не пул.

- url: https://pkg.go.dev/google.golang.org/grpc#NewClient
  why: |
    grpc.NewClient — replacement for deprecated grpc.Dial.
    NewClient НЕ блокирует на dial, соединение lazy.

- url: https://pkg.go.dev/google.golang.org/grpc/keepalive#ClientParameters
  critical: Time < 10s silently bumped to 10s by grpc-go.

- url: https://pkg.go.dev/google.golang.org/grpc/examples/features/retry
  why: Retry policy через WithDefaultServiceConfig (JSON).

# === Singleflight ===

- url: https://pkg.go.dev/golang.org/x/sync/singleflight
  critical: |
    singleflight НЕ отменяет fn при отмене caller'ского context.
    Это OK для Refresh (хотим довыполнить), но проверять ctx.Err() перед RPC.
    После успеха — Forget(key), иначе следующий вызов ждёт stale result.

# === Security ===

- url: https://owasp.org/Top10/2025/A07_2025-Authentication_Failures/
  why: Fail-open — namedly exploitable vulnerability. SDK default = FailClosed.

- url: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
  section: Cookie attributes
  critical: HttpOnly + Secure + SameSite + Path scoping.

- url: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
  section: Session ID binding
  critical: |
    "Binding session IDs to the client IP address can be bypassed by reusing
    the same IP address assigned to the victim user by sharing the same
    network (NAT)." → IP+UA binding — detection hint, не hard block.

- url: https://datatracker.ietf.org/doc/html/rfc6750
  why: Bearer token (Authorization header). 401 + WWW-Authenticate: Bearer realm.

# === Go SDK design ===

- url: https://pkg.go.dev/golang.org/x/net/context
  why: Comma-ok idiom for context.Value extraction.

- file: .claude/guides/golang/effective_standards.md
  rules: ES-0051, ES-0120, ES-0140, ES-0143, ES-0171
  why: Conv-ы для конструктора, опций, noop defaults, без nolint.

- file: .claude/guides/golang/effective_concurrency.md
  rules: EC-0010, EC-0012, EC-0034, EC-0056
  why: Goroutine lifecycle (cache cleaner), no leak (Close()), unexported context key.

- file: .claude/guides/golang/coding_style.md
  rules: CS-0050, CS-0100, CS-0117
  why: Один package для public surface, без env-tags в Config, exported types layout.
```

### Current Codebase Structure

```
auth-client-go/
├── CLAUDE.md
├── CHANGELOG.md
├── Makefile                    ← test/test-race/coverage/lint/build-examples (no integration target yet)
├── README.md                   ← placeholder
├── go.mod                      ← module github.com/webdelo/auth-client-go (Go 1.26.2)
├── scratch.yaml                ← project_type: lib, integration_tests: true
├── mockery.yaml
├── .golangci.yaml
├── .github/workflows/ci.yaml   ← basic CI (no integration job)
├── lib/
│   ├── lib.go                  ← const Version = "0.0.1" (placeholder)
│   ├── lib_test.go             ← placeholder
│   ├── internal/.gitkeep
│   └── examples/
│       └── basic/main.go       ← prints version (placeholder)
├── dev/tools/                  ← release.sh, tools.mk, test-runner-main.go
├── .memory_bank/
│   ├── tech_stack.yaml
│   ├── project_map.yaml
│   ├── external_libs.yaml
│   ├── dev_commands.yaml
│   └── tasks_list.yaml
└── task_management/
    ├── tasks/REQ-001.md
    ├── plans/                  ← (этот ADR)
    ├── results/, reviews/, specs/, debug_reports/, bug_reports/, session_analysis/
```

### Desired Codebase Structure (после полной реализации REQ-001)

```
auth-client-go/
├── CHANGELOG.md                ← запись v0.1.0 — YYYY-MM-DD (TASK-007)
├── README.md                   ← инсталляция, quickstart, конфиг, errors, examples (TASK-007)
├── Makefile                    ← + env-up, env-down, test-integration (TASK-001)
├── go.mod                      ← + auth-proto/go v0.1.0, grpc, singleflight (TASK-001)
├── go.sum
├── .github/workflows/ci.yaml   ← + опциональный integration job (TASK-001)
├── lib/
│   ├── doc.go                  ← package godoc (TASK-007)
│   ├── client.go               ← New, Config, Options, Close (TASK-002)
│   ├── errors.go               ← sentinel + AuthError + RetryAfter + mapGRPCError (TASK-002)
│   ├── logger.go               ← Logger interface + NoopLogger (TASK-002)
│   ├── validate.go             ← ValidateAccessToken (TASK-003)
│   ├── middleware_http.go      ← HTTPMiddleware (TASK-003)
│   ├── interceptor_grpc.go     ← UnaryInterceptor (TASK-003)
│   ├── auth.go                 ← Login, Refresh, Logout, LogoutAll, ChangePassword,
│   │                              GetCurrentUser (TASK-004)
│   ├── context.go              ← User, Role, WithUser, UserFromContext (TASK-005)
│   ├── cookies.go              ← SetRefreshCookie, ClearRefreshCookie, CookieOption (TASK-005)
│   ├── helpers.go              ← IPFromRequest, UserAgentFromRequest (TASK-005)
│   ├── client_test.go          ← unit tests for client.go (TASK-002)
│   ├── errors_test.go          ← (TASK-002)
│   ├── validate_test.go        ← (TASK-003)
│   ├── middleware_http_test.go ← (TASK-003)
│   ├── interceptor_grpc_test.go← (TASK-003)
│   ├── auth_test.go            ← (TASK-004)
│   ├── context_test.go         ← (TASK-005)
│   ├── cookies_test.go         ← (TASK-005)
│   ├── helpers_test.go         ← (TASK-005)
│   ├── client_integration_test.go    ← //go:build integration (TASK-001..004)
│   ├── internal/
│   │   ├── cache/
│   │   │   ├── memory.go       ← MemoryCache (TASK-003)
│   │   │   └── memory_test.go
│   │   └── refresh/
│   │       ├── group.go        ← singleflight wrapper (TASK-004)
│   │       └── group_test.go
│   ├── examples/
│   │   ├── basic/main.go            ← Login → Validate → Logout (TASK-006)
│   │   ├── http_middleware/main.go  ← protected HTTP server (TASK-006)
│   │   └── grpc_interceptor/main.go ← protected gRPC server (TASK-006)
│   └── mocks/                  ← generated mockery mocks (TASK-002+)
├── dev/deployments/
│   ├── compose.test.yaml       ← AUTH + Postgres for integration tests (TASK-001)
│   └── test.env                ← AUTH_ENDPOINT=localhost:50051, etc. (TASK-001)
└── task_management/plans/REQ-001-ADR.md    ← (этот файл)
```

### List of tasks to be completed to fulfill the ADR in order

**Все 7 задач уже описаны в `task_management/tasks/REQ-001.md` (раздел TASKS).** ADR не дублирует, а уточняет:

```markdown
# TASK-001: Bootstrap deps + integration test infra
## WHY
До любого кода SDK нужно:
- зафиксировать `auth-proto/go@v0.1.0` в go.mod;
- иметь рабочий способ запустить SDK против реального AUTH (без mock'ов на этом уровне).

## WHAT
- [ ] `go get github.com/webdelo/auth-proto/go@v0.1.0`, `go get google.golang.org/grpc`,
      `go get golang.org/x/sync`, `go mod tidy`.
- [ ] Создать `dev/deployments/compose.test.yaml` (AUTH + PostgreSQL,
      AUTH-image из локально-собранного `webdelo/auth` или GHCR-tag).
- [ ] Создать `dev/deployments/test.env` с переменными для AUTH и тестов.
- [ ] Добавить в Makefile: `env-up`, `env-down`, `test-integration` (с trap для cleanup).
- [ ] Добавить в `.github/workflows/ci.yaml` опциональный `integration` job
      (matrix entry, не блокирующий PR).
- [ ] Документировать в README раздел «Local integration testing».

## DOD
- [ ] `make env-up` поднимает AUTH, healthcheck отвечает.
- [ ] `make test-integration` пробрасывает env-vars, прогоняет
      `go test -tags=integration ./lib/...`, делает `env-down`.
- [ ] `go.mod` содержит auth-proto/go, grpc, singleflight; `go mod tidy` чисто.

# TASK-002: Core Client + Errors + Logger
## WHY
Базовый каркас, который импортируют все остальные таски.

## WHAT
- [ ] `lib/client.go`: `Config`, `FailMode`, `Option`, `New`, `(*Client).Close`.
- [ ] `lib/errors.go`: 5 sentinel-ошибок, `AuthError` wrapper, `RetryAfter`,
      приватные `mapGRPCError(err) error` и `statusFromWrapped(err) (*status.Status, bool)`.
- [ ] `lib/logger.go`: `Logger` interface, `NoopLogger`.
- [ ] Unit-тесты для каждого файла (table-driven, UT-0001).
- [ ] gRPC dial с keepalive(30s, 10s, true) + retry policy для UNAVAILABLE.

## DOD
- [ ] `New(Config{Endpoint: "auth:50051"})` возвращает рабочий `*Client`.
- [ ] `Close()` идемпотентен.
- [ ] `errors.Is(mapGRPCError(...), ErrXxx)` корректно для всех маппингов.
- [ ] Coverage ≥ 85% по `client.go`, `errors.go`.

# TASK-003: ValidateToken + Cache + Middleware/Interceptor
## WHY
Hot path SDK. Самая критичная логика по производительности и безопасности.

## WHAT
- [ ] `lib/internal/cache/memory.go`: TTL cache на `sync.Map` + lazy-expire +
      background goroutine (контекст из New, Close = cancel).
- [ ] `lib/validate.go`: `ValidateAccessToken` с cache + sha256 ключ +
      positive-only кэширование.
- [ ] `lib/middleware_http.go`: `HTTPMiddleware` (Authorization: Bearer,
      WWW-Authenticate, FailMode handling).
- [ ] `lib/interceptor_grpc.go`: `UnaryInterceptor` (metadata authorization,
      WithOpenMethods registry, recover).
- [ ] Unit-тесты с mockery-моками `AuthServiceClient`.
- [ ] Integration-тесты против реального AUTH: validate happy path,
      validate expired, validate blocked, validate invalid.

## DOD
- [ ] Cache hit < 1ms (benchmark).
- [ ] Negative result не попадает в кэш (test).
- [ ] Cleanup goroutine завершается при Close (test с goroutine count).
- [ ] HTTPMiddleware возвращает 401/503/429/200 согласно А10.
- [ ] UnaryInterceptor пропускает методы из WithOpenMethods (test).

# TASK-004: Auth operations + Singleflight refresh
## WHY
Login flow для consumer'а. Refresh — самая токсичная race-condition зона.

## WHAT
- [ ] `lib/internal/refresh/group.go`: обёртка над `singleflight.Group`
      с автоматическим Forget после успеха.
- [ ] `lib/auth.go`: Login, Refresh (с singleflight), Logout, LogoutAll,
      ChangePassword, GetCurrentUser.
- [ ] LoginInput/Output, RefreshOutput.
- [ ] Маппинг proto Unix-timestamps → time.Duration для ExpiresIn полей.
- [ ] Unit-тесты с моками + специальный test:
      «два одновременных Refresh с одинаковым refresh_token →
      gRPC RefreshToken вызван 1 раз».
- [ ] Integration-тесты: login, refresh (включая race через goroutine),
      logout, change_password.

## DOD
- [ ] Race-test проходит (2 goroutine → 1 RPC call в моке).
- [ ] errors.Is для всех ошибочных сценариев.
- [ ] Coverage ≥ 80% по auth.go.

# TASK-005: Context + Cookies + HTTP helpers
## WHY
Convenience для consumer-кода.

## WHAT
- [ ] `lib/context.go`: User, Role, WithUser, UserFromContext (unexported key).
- [ ] `lib/cookies.go`: SetRefreshCookie, ClearRefreshCookie, CookieOption,
      4 With*-опции.
- [ ] `lib/helpers.go`: IPFromRequest (X-Real-IP / XFF[0] / RemoteAddr),
      UserAgentFromRequest.
- [ ] Unit-тесты — table-driven для всех граничных кейсов
      (XFF с пробелами, multi-IP в XFF, IPv6, etc.).

## DOD
- [ ] Cookie defaults включают HttpOnly+Secure+SameSite=Strict+Path=/auth/refresh+MaxAge=14d.
- [ ] CookieOption корректно переопределяет defaults.
- [ ] IPFromRequest fallback chain работает для всех веток.

# TASK-006: Examples
## WHY
Отдельные main.go доказывают, что SDK реально работает в типовых сценариях,
и служат документацией.

## WHAT
- [ ] `lib/examples/basic/main.go`: Login → ValidateAccessToken → Logout.
- [ ] `lib/examples/http_middleware/main.go`: HTTP-сервер с
      `client.HTTPMiddleware`, тестовый handler читает `UserFromContext`.
- [ ] `lib/examples/grpc_interceptor/main.go`: gRPC-сервер с
      `client.UnaryInterceptor`, отдельный «открытый» метод через WithOpenMethods.
- [ ] Каждый пример имеет README с командой запуска.
- [ ] `make build-examples` собирает все три без warnings.

## DOD
- [ ] Все три примера собираются.
- [ ] Каждый пример демонстрирует ровно одну фичу (минимализм).

# TASK-007: README + godoc + CHANGELOG + tag v0.1.0
## WHY
Без README SDK нельзя начать использовать.

## WHAT
- [ ] `lib/doc.go`: package-level godoc.
- [ ] godoc-комментарии на все exported (включая sentinels, options, types).
- [ ] README.md: установка, quickstart (5 строк),
      конфиг (все Config-поля + опции),
      описание FailMode (с warning про FailOpen),
      описание typed errors (таблица),
      описание адаптеров (Logger, Cache),
      ссылка на examples,
      troubleshooting (debug 401, TLS setup, fingerprint каверзы).
- [ ] CHANGELOG.md: запись `## v0.1.0 — YYYY-MM-DD` с фичами и known limitations
      (no distributed singleflight, no trusted-proxies, no metrics).
- [ ] `make release` создаёт тег v0.1.0.

## DOD
- [ ] godoc.org/github.com/webdelo/auth-client-go/lib (или go doc локально)
      рендерится без TODO.
- [ ] README прогоняется на BFF-разработчике (ему нужно собрать middleware за <30 мин).
- [ ] Тег v0.1.0 опубликован.
```

---

## Implementation Blueprint (псевдокод критичных компонентов)

### `lib/client.go` (упрощённо)

```go
type Client struct {
    cfg          Config
    grpcConn     *grpc.ClientConn
    authClient   auth_v1.AuthServiceClient
    cache        Cache
    sf           *refresh.Group
    logger       Logger
    failMode     FailMode
    openMethods  map[string]struct{}

    cancelBG     context.CancelFunc  // для cache cleanup goroutine
    closeOnce    sync.Once
    closeErr     error
}

func New(cfg Config, opts ...Option) (*Client, error) {
    if cfg.Endpoint == "" {
        return nil, errors.New("authclient: Config.Endpoint required")
    }

    cfg = applyDefaults(cfg)  // заполнить ValidateCacheTTL=30s, timeouts, etc.

    dialOpts := append([]grpc.DialOption{
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time: 30*time.Second, Timeout: 10*time.Second, PermitWithoutStream: true,
        }),
        grpc.WithDefaultServiceConfig(`{"methodConfig":[{"name":[{"service":"auth.v1.AuthService"}],"retryPolicy":{"maxAttempts":4,"initialBackoff":"0.1s","maxBackoff":"1s","backoffMultiplier":2,"retryableStatusCodes":["UNAVAILABLE"]}}]}`),
    }, cfg.DialOptions...)
    if !hasTransportCreds(dialOpts) {
        dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
    }

    conn, err := grpc.NewClient(cfg.Endpoint, dialOpts...)
    if err != nil { return nil, fmt.Errorf("authclient: dial: %w", err) }

    bgCtx, cancel := context.WithCancel(context.Background())
    cache := cfg.Cache
    if cache == nil {
        cache = cachepkg.NewMemoryCache(bgCtx, cfg.ValidateCacheTTL/2)
    }

    c := &Client{
        cfg:         cfg,
        grpcConn:    conn,
        authClient:  auth_v1.NewAuthServiceClient(conn),
        cache:       cache,
        sf:          refresh.New(),
        logger:      cfg.Logger,
        failMode:    cfg.FailMode,
        openMethods: defaultOpenMethods(),
        cancelBG:    cancel,
    }
    for _, opt := range opts { opt(c) }
    return c, nil
}

func (c *Client) Close() error {
    c.closeOnce.Do(func() {
        c.cancelBG()                 // останавливает cache cleaner
        c.closeErr = c.grpcConn.Close()
    })
    return c.closeErr
}
```

### `lib/validate.go`

```go
func (c *Client) ValidateAccessToken(ctx context.Context, accessToken string) (User, error) {
    if accessToken == "" { return User{}, ErrInvalidToken }

    key := hashToken(accessToken)  // hex(sha256(accessToken))
    if user, ok := c.cache.Get(ctx, key); ok {
        return user, nil
    }

    callCtx, cancel := context.WithTimeout(ctx, c.cfg.ValidateTimeout)
    defer cancel()

    resp, err := c.authClient.ValidateToken(callCtx, &auth_v1.ValidateTokenRequest{
        AccessToken: accessToken,
    })
    if err != nil {
        return User{}, mapGRPCError(err)  // ErrUnavailable
    }
    if !resp.IsValid {
        return User{}, mapErrorReason(resp.ErrorReason)  // NOT cached
    }

    user := User{
        UUID: resp.UserUuid,
        Role: Role(resp.Role),
        // Email пустой — proto не возвращает в ValidateTokenResponse
    }
    c.cache.Set(ctx, key, user, c.cfg.ValidateCacheTTL)
    return user, nil
}
```

### `lib/auth.go` (Refresh — самое токсичное)

```go
func (c *Client) Refresh(ctx context.Context, refreshToken, clientIP, userAgent string) (RefreshOutput, error) {
    if refreshToken == "" { return RefreshOutput{}, ErrInvalidToken }

    key := hashToken(refreshToken)
    v, err, _ := c.sf.Do(key, func() (interface{}, error) {
        callCtx, cancel := context.WithTimeout(ctx, c.cfg.RefreshTimeout)
        defer cancel()
        return c.authClient.RefreshToken(callCtx, &auth_v1.RefreshTokenRequest{
            RefreshToken: refreshToken,
            Ip:           clientIP,
            UserAgent:    userAgent,
        })
    })
    c.sf.Forget(key)  // ОБЯЗАТЕЛЬНО — иначе следующий refresh ждёт stale
    if err != nil {
        return RefreshOutput{}, mapGRPCError(err)
    }
    resp := v.(*auth_v1.RefreshTokenResponse)
    return RefreshOutput{
        AccessToken:      resp.AccessToken,
        RefreshToken:     resp.RefreshToken,
        AccessExpiresIn:  time.Until(time.Unix(resp.AccessTokenExpiresAt, 0)),
        RefreshExpiresIn: time.Until(time.Unix(resp.RefreshTokenExpiresAt, 0)),
    }, nil
}
```

### `lib/middleware_http.go`

```go
func (c *Client) HTTPMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token, ok := bearerToken(r.Header.Get("Authorization"))
        if !ok {
            w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
            http.Error(w, "missing or malformed token", http.StatusUnauthorized)
            return
        }

        user, err := c.ValidateAccessToken(r.Context(), token)
        switch {
        case errors.Is(err, ErrUnavailable):
            if c.failMode == FailOpen {
                c.logger.Warn(r.Context(), "auth unavailable, fail-open pass-through")
                next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), User{})))
                return
            }
            w.Header().Set("Retry-After", "5")
            http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
            return
        case errors.Is(err, ErrRateLimited):
            if d := RetryAfter(err); d > 0 {
                w.Header().Set("Retry-After", strconv.Itoa(int(d.Seconds())))
            }
            http.Error(w, "rate limited", http.StatusTooManyRequests)
            return
        case err != nil:  // ErrInvalidToken / ErrExpired / ErrBlocked
            w.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
            http.Error(w, err.Error(), http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
    })
}
```

### `lib/internal/cache/memory.go`

```go
type entry struct {
    user      User
    expiresAt time.Time
}

type MemoryCache struct {
    m sync.Map  // key string → entry
}

func NewMemoryCache(bgCtx context.Context, cleanupInterval time.Duration) *MemoryCache {
    c := &MemoryCache{}
    if cleanupInterval > 0 {
        go c.cleanupLoop(bgCtx, cleanupInterval)  // EC-0010 lifecycle via bgCtx
    }
    return c
}

func (c *MemoryCache) Get(_ context.Context, key string) (User, bool) {
    v, ok := c.m.Load(key)
    if !ok { return User{}, false }
    e := v.(entry)
    if time.Now().After(e.expiresAt) {
        c.m.Delete(key)  // lazy expire
        return User{}, false
    }
    return e.user, true
}

func (c *MemoryCache) Set(_ context.Context, key string, user User, ttl time.Duration) {
    if ttl <= 0 { return }  // ValidateCacheTTL=0 disables cache
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
            return  // EC-0012 no leak
        case <-t.C:
            now := time.Now()
            c.m.Range(func(k, v interface{}) bool {
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

## Validation Gates (все исполнимы AI)

```bash
# 1. Базовое качество кода (после каждой задачи)
make audit                    # vet + lint + test (включая race)

# 2. Покрытие
make coverage                 # должно показать >= 75% для lib/

# 3. Сборка примеров
make build-examples           # все три должны собираться без warnings

# 4. Integration (после TASK-001..004)
make test-integration         # поднимает AUTH в Docker, прогоняет integration suite

# 5. Гранулярные проверки
go test -tags=integration -race -run TestRefresh_Race -count=10 ./lib/...
                              # 10 прогонов race-теста для Refresh singleflight
go test -bench=BenchmarkValidateCache -benchmem ./lib/internal/cache/
                              # cache hit < 1µs/op

# 6. API stability (TASK-007)
go doc github.com/webdelo/auth-client-go/lib | grep -E "^(type|func|var)"
                              # проверка, что вся публичная поверхность задокументирована

# 7. Module hygiene
go mod why github.com/webdelo/auth-proto/go    # подтверждает прямую зависимость
go mod why google.golang.org/grpc              # подтверждает прямую зависимость
go mod why github.com/sirupsen/logrus          # ДОЛЖНО вернуть error (нет логгеров)
go mod why go.uber.org/fx                      # ДОЛЖНО вернуть error (нет DI)

# 8. Релиз
make release-dry-run          # preview без изменений
make release                  # interactive, создаёт tag v0.1.0
```

---

## Score: 9/10

| Критерий | Оценка | Заметка |
|----------|--------|---------|
| Necessary context included | 10/10 | REQ + 3 agent reports + .proto + 13 docs URL |
| Validation gates executable | 10/10 | 8 конкретных команд, все автоматизируемые |
| References existing patterns | 9/10 | code-research дал ссылки на fundingbot/signal-analyzer |
| Clear implementation path | 9/10 | 7 задач с зависимостями + псевдокод 4 ключевых компонентов |
| SQL schemas — no CASCADE | N/A | SDK без БД |
| API design rigor | 9/10 | api-designer дал mockup, 13 design decisions, evolution guide |
| Risks and mitigations | 9/10 | 7 рисков из REQ + 3 новых (singleflight cancellation, keepalive auto-bump, no Email in Validate response) |

**Минус 1 балл:** integration test infra зависит от внешнего фактора (публикация AUTH-image). В TASK-001 это adressed как accepted risk, но не закрыто полностью.

---

## Риски и нюансы

| # | Риск | Митигация | Источник |
|---|------|-----------|----------|
| R1 | Версия grpc/protobuf не совпала с auth-proto | Не пинить grpc/protobuf вручную, MVS выбирает максимум | REQ Risk #1 |
| R2 | Fingerprint binding ломается за NAT/CGNAT | Документировать в README; consumer передаёт IP/UA вручную | REQ Risk #2 + OWASP |
| R3 | Singleflight не покрывает кросс-инстанс BFF | Accepted в v0.1.0; додать Redis-impl в v0.2.0 если станет проблемой | REQ Risk #3 |
| R4 | Кэш не подхватит мгновенный logout (до 30s lag) | Документировать; ValidateCacheTTL=0 отключает кэш для критичных consumer'ов | REQ Risk #4 |
| R5 | Logger interface заставит писать adapter даже для silent | NoopLogger{} default — ничего настраивать не надо | REQ Risk #5 |
| R6 | Integration-тесты требуют AUTH-image | Отдельный devops-REQ на публикацию в GHCR; пока local-only | REQ Risk #6 |
| R7 | Случайное добавление тяжёлых deps | Code review каждого TASK включает `go mod why` для новых deps | REQ Risk #7 |
| R8 | Singleflight + ctx-cancel: первый caller отменил, вторые ждут | Singleflight НЕ отменит fn — все ждущие получат результат, fn доходит до AUTH. OK для refresh | web-research |
| R9 | grpc-go silently bumps Keepalive.Time<10s | Документировать; SDK ставит 30s, не разрешать override меньше 10s в v0.1.0 | web-research |
| R10 | ValidateTokenResponse не содержит email — User.Email пустой после Validate | Документировать в godoc на User; Login/GetCurrentUser возвращают email; либо убрать User.Email, либо принять nullability (выбрано — принять) | proto + api-design OQ-01 |
| R11 | retry policy для UNAVAILABLE может усугубить нагрузку при storm | maxAttempts=4 + exp.backoff гарантируют не больше 4 RPC за ~3s; идемпотентны только Validate/Login (Refresh unsafe для retry — можно убить family) | web-research |

**Дополнение к R11:** retry policy ставится по-методно через JSON ServiceConfig. Для `RefreshToken` — НЕ ретраить (`retryableStatusCodes` пустой). Для `ValidateToken` — ретраить только UNAVAILABLE.

---

## DOD (umbrella) — обновлённый из REQ-001

- [ ] `webdelo/auth-client-go` импортирует `github.com/webdelo/auth-proto/go v0.1.0`
- [ ] `lib/` содержит все файлы из секции «Desired Codebase Structure»
- [ ] `lib/examples/` содержит три рабочих примера (basic, http_middleware, grpc_interceptor)
- [ ] Все public API задокументированы godoc-комментариями (`go doc lib` без TODO)
- [ ] Unit-тесты покрывают ≥ 75% (`make coverage`)
- [ ] Integration-тесты против реального AUTH покрывают: login, validate (positive/negative — expired/blocked/invalid), refresh (включая race через goroutine + singleflight), logout, logout_all, change_password, get_current_user
- [ ] `make audit` (vet + lint + test) проходит без ошибок
- [ ] `make test-integration` поднимает AUTH через docker-compose, прогоняет тесты, останавливает
- [ ] README.md содержит: установку, quickstart, конфигурацию (все опции), описание FailMode (с warning про FailOpen), описание typed errors (таблица mapping), описание адаптеров (Logger, Cache), ссылку на examples, troubleshooting (debug 401, TLS setup, fingerprint caveats)
- [ ] CHANGELOG.md содержит запись `## v0.1.0 — YYYY-MM-DD` с known limitations (no distributed singleflight, no trusted-proxies validation, no Metrics interface)
- [ ] Tag `v0.1.0` опубликован
- [ ] BFF-разработчик через `go get github.com/webdelo/auth-client-go@v0.1.0` собирает рабочий middleware за < 30 минут (валидируется на 1 reviewer'е)
- [ ] `go mod why` для каждой прямой зависимости подтверждает обоснованность (см. Validation Gate #7)

---

## Open Questions (не блокеры для v0.1.0)

| OQ | Вопрос | Кому/Когда решать |
|----|--------|-------------------|
| OQ-01 | Стоит ли расширить `ValidateTokenResponse` в auth-proto, чтобы возвращался email? Сейчас `User.Email` пустой после `ValidateAccessToken`. | Решить до v1.0.0 SDK. Если AUTH добавит — User.Email консистентен; если нет — пометить поле deprecated в Validate-контексте. |
| OQ-02 | Нужен ли `RefreshFromRequest(ctx, w, r)` convenience-helper уже в v0.1.0? | Отложено в v0.2.0 (additive). Пока consumer вызывает Refresh + SetRefreshCookie вручную. |
| OQ-03 | Streaming gRPC interceptor (`StreamInterceptor`) — нужен в v0.1.0? | Отложено. Webdelo Trading сейчас не использует streaming RPC между Go-сервисами. |
| OQ-04 | Multi-role в User? | Отложено до AUTH REQ-XXX. Сейчас User.Role — единственная роль (string из proto). |
| OQ-05 | Distributed singleflight через Redis для cross-instance BFF? | v0.2.0+, если возникнет реальная проблема (multi-replica BFF + частые family-detection срабатывания). |

---

## Ссылки

### Этот репо
- REQ: [task_management/tasks/REQ-001.md](../tasks/REQ-001.md)
- Memory bank: [.memory_bank/](../../.memory_bank/)
- Agent reports:
  - [code-research](../../.memory_bank/agent_task_results/2026-05-15_code-research_codebase_analysis_REQ-001-ADR.yaml)
  - [web-research](../../.memory_bank/agent_task_results/web_research_REQ-001-ADR.yaml)
  - [api-design](../../.memory_bank/agent_task_results/api_design_REQ-001-ADR.yaml)

### Связанные репо
- Contract: https://github.com/webdelo/auth-proto (tag `go/v0.1.0`)
- Server: https://github.com/webdelo/auth
- AUTH proto: `~/Development/TRADING/bots/auth-proto/proto/auth_service.proto`

### External
- gRPC concurrency: https://github.com/grpc/grpc-go/blob/master/Documentation/concurrency.md
- gRPC keepalive: https://pkg.go.dev/google.golang.org/grpc/keepalive
- gRPC retry: https://pkg.go.dev/google.golang.org/grpc/examples/features/retry
- singleflight: https://pkg.go.dev/golang.org/x/sync/singleflight
- OWASP Session Management: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- OWASP Top 10 2025 A07: https://owasp.org/Top10/2025/A07_2025-Authentication_Failures/
- RFC 6750 (Bearer): https://datatracker.ietf.org/doc/html/rfc6750
