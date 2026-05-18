# Core Client + типизированные ошибки + Logger adapter

---

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](./REQ-001.md)

**Priority**: high
**Depends on**: REQ-001-TASK-001

---

## WHY

Базовый каркас SDK, который импортируют все остальные таски (validate, auth ops, middleware, interceptor). Закладывает обязательные части публичного API: конструктор с Config + functional options, lifecycle (`Close`), маппинг gRPC-ошибок в sentinel-ошибки, adapter-интерфейс логгера. Без этого каркаса последующие задачи не могут начать.

---

## WHAT

1. **Создать `lib/client.go`.** Определить `Config` struct (Endpoint, ValidateCacheTTL, FailMode, ValidateTimeout, LoginTimeout, RefreshTimeout, DialOptions, Logger, Cache), `FailMode` enum (`FailClosed` default, `FailOpen`), `Option func(*Client)` тип и опции `WithFailMode`, `WithLogger`, `WithCache`, `WithOpenMethods`. Реализовать `New(cfg Config, opts ...Option) (*Client, error)` и `(*Client).Close() error` (идемпотентный через `sync.Once`).
2. **Применить дефолты и dial-опции внутри `New`.** Если `Endpoint==""` → вернуть ошибку. Применить дефолты для таймаутов и TTL (см. [REQ-001-ADR §A6](../plans/REQ-001-ADR.md)). Использовать `grpc.NewClient` (НЕ deprecated `grpc.Dial`) с keepalive `{Time: 30s, Timeout: 10s, PermitWithoutStream: true}` и `WithDefaultServiceConfig` retry policy для UNAVAILABLE на `ValidateToken` (max 4 attempts, exp backoff), но НЕ ретраить `RefreshToken` (family detection risk, см. [REQ-001-ADR §R11](../plans/REQ-001-ADR.md)). Дефолтный transport credentials — `insecure.NewCredentials()`, переопределяется через `Config.DialOptions`.
3. **Создать `lib/errors.go`.** Определить 5 sentinel-ошибок: `ErrInvalidToken`, `ErrExpired`, `ErrBlocked`, `ErrRateLimited`, `ErrUnavailable`. Реализовать `AuthError` wrapper с полем `RetryAfter time.Duration` и методами `Error()`/`Unwrap()`. Реализовать helper `RetryAfter(err error) time.Duration`.
4. **Реализовать `mapGRPCError(err error) error`.** Приватный mapper gRPC status → sentinel согласно таблице в [REQ-001-ADR §A7](../plans/REQ-001-ADR.md): `Unauthenticated`→`ErrInvalidToken`, `PermissionDenied`→`ErrBlocked`, `ResourceExhausted`→`ErrRateLimited` (с извлечением `Retry-After` из metadata), `Unavailable`/`DeadlineExceeded`/network→`ErrUnavailable`. Реализовать приватный `statusFromWrapped(err error) (*status.Status, bool)` — итерирует через `errors.Unwrap`, потому что стандартный `status.FromError` НЕ ходит по chain.
5. **Реализовать `mapErrorReason(reason string) error`.** Mapping `ValidateTokenResponse.error_reason` строк: `"expired"`→`ErrExpired`, `"blocked"`→`ErrBlocked`, `"invalid"` / пусто / неизвестное → `ErrInvalidToken`. Используется в TASK-003 для negative результатов `ValidateToken`.
6. **Создать `lib/logger.go`.** Объявить интерфейс `Logger` с методами `Info(ctx, msg, fields...)`, `Warn(ctx, msg, fields...)`, `Error(ctx, msg, err, fields...)` (variadic `...any` совместим со slog/zap парами). Реализовать `NoopLogger struct{}` со всеми no-op методами (per ES-0143). Дефолт `Config.Logger == nil` → `NoopLogger{}`.
7. **Покрыть unit-тестами.** Table-driven для `mapGRPCError` (все gRPC коды), `RetryAfter`, `New` (валидация Config), `Close` (идемпотентность). Mockery моки для `grpc.ClientConn` не нужны — `Close` тестируется на реальном клиенте без подключения.

### Акценты

- _ES-0051 (interface на стороне consumer), ES-0120 (functional options), ES-0140 (`New()` not `NewClient`), ES-0143 (NoopLogger default), ES-0171 (без `nolint:staticcheck`)._
- _EC-0010 lifecycle, EC-0012 no leak — `Close` обязан останавливать любые фоновые goroutines (в этом TASK их ещё нет; задел: `cancelBG context.CancelFunc` для будущего cache cleaner)._
- _Паттерн mapping gRPC-status через `errors.Unwrap`-итерацию — взять из signal-analyzer (см. [REQ-001-ADR §A7](../plans/REQ-001-ADR.md))._
- _`grpc.NewClient` (deprecated `grpc.Dial` запрещён); никаких `nolint`._
- _Имена sentinel-ошибок и FailMode-констант — frozen после v1.0.0 (см. evolution table в [REQ-001-ADR §API](../plans/REQ-001-ADR.md))._
- _Pitfall: keepalive `Time < 10s` grpc-go молча подменяет на 10s — в SDK ставим 30s (см. [REQ-001-ADR §R9](../plans/REQ-001-ADR.md))._

---

## DOD (Definition of Done)

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
