# Auth operations: Login / Refresh (singleflight) / Logout / LogoutAll / ChangePassword / GetCurrentUser

---

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](./REQ-001.md)

**Priority**: high
**Depends on**: REQ-001-TASK-002

---

## WHY

Login-flow для consumer-сервисов (BFF). Refresh — самая токсичная race-condition зона: две вкладки шлют один refresh_token, AUTH детектит reuse и убивает family — пользователь выкидывается. SDK обязан сериализовать одинаковые refresh-запросы через `singleflight` так, чтобы в AUTH улетел ровно один RPC. Остальные операции (Logout, LogoutAll, ChangePassword, GetCurrentUser) — тонкие обёртки над gRPC с маппингом ошибок.

---

## WHAT

1. **Создать `lib/internal/refresh/group.go`.** Обёртка над `singleflight.Group` из `golang.org/x/sync/singleflight`. Метод `Do(key string, fn func() (any, error)) (any, error)` запускает `sf.Do(key, fn)` и **обязательно** вызывает `sf.Forget(key)` сразу после возврата (defer внутри Do) — иначе следующий `Do` с тем же ключом дождётся stale-результата завершённой операции (см. [REQ-001-ADR §A3 + R8](../plans/REQ-001-ADR.md)). Singleflight НЕ отменяет fn при отмене caller'ского context — это OK для refresh.
2. **Подключить singleflight Group в `Client`.** Поле `sf *refresh.Group`, инициализация в `New`.
3. **Создать `lib/auth.go`.** Определить публичные типы `LoginInput`, `LoginOutput`, `RefreshOutput` согласно [REQ-001-ADR §API](../plans/REQ-001-ADR.md). Реализовать методы:
    - `Login(ctx, in LoginInput) (LoginOutput, error)` — gRPC `Login`, mapping ошибок через `mapGRPCError`, конвертация Unix-timestamps → `time.Duration` для `AccessExpiresIn`/`RefreshExpiresIn`.
    - `Refresh(ctx, refreshToken, clientIP, userAgent string) (RefreshOutput, error)` — через `sf.Do(hashToken(refreshToken), ...)`, gRPC `RefreshToken`, новый refresh_token (rotated AUTH) возвращается в Output.
    - `Logout(ctx, refreshToken string) error` — gRPC `Logout`, mapping ошибок.
    - `LogoutAll(ctx, accessToken string) error` — gRPC `LogoutAll`.
    - `ChangePassword(ctx, accessToken, currentPassword, newPassword string) error` — gRPC `ChangePassword`.
    - `GetCurrentUser(ctx, accessToken string) (User, error)` — gRPC `GetCurrentUser`, возвращает `User{UUID, Email, Role}` (в отличие от `ValidateAccessToken`, здесь Email есть).
4. **Использовать таймауты из Config.** Каждая операция оборачивает `ctx` в `context.WithTimeout` со своим таймаутом (`LoginTimeout`, `RefreshTimeout`, default `ValidateTimeout` для остальных, если в Config не выделять отдельные — оставить общий `ValidateTimeout` как fallback).
5. **Логировать через `Logger` adapter.** На success Login/Logout/LogoutAll/ChangePassword/Refresh — `Info` с user UUID (без email/IP/токена). На error UNAVAILABLE — `Warn`. На panic в `Refresh fn` — `Error`.
6. **Покрыть unit-тестами.** Mockery моки для `auth_v1.AuthServiceClient`:
    - Login: happy, invalid creds → `ErrInvalidToken`, rate limited → `ErrRateLimited` (с RetryAfter), unavailable.
    - Refresh: happy, rotated refresh_token в Output, race-тест — 2 goroutine с одним `refreshToken` → `auth_v1.AuthServiceClient.RefreshToken` вызван 1 раз (проверка через mock call counter).
    - Logout/LogoutAll/ChangePassword/GetCurrentUser: happy + основные ошибки.
    - Конвертация timestamps: `AccessExpiresIn` корректно вычисляется через `time.Until(time.Unix(...))`.
7. **Покрыть integration-тестами против реального AUTH.** Сценарии:
    - `TestLogin_Happy` — создать пользователя через admin API AUTH, login, проверка токенов.
    - `TestRefresh_HappyAndRotation` — login → refresh → старый refresh_token больше не работает.
    - `TestRefresh_Race` — 10 goroutine с одним refresh_token → ровно один новый refresh_token у всех, family не убита.
    - `TestLogout` — login → logout → старый refresh_token invalid.
    - `TestChangePassword` — login → change password → старый access_token invalid (eventually, после TTL кэша).
    - `TestGetCurrentUser_HasEmail` — после login, GetCurrentUser возвращает email.
8. **Race-тест с `go test -race -count=10`.** `TestRefresh_Race` прогоняется 10 раз подряд — отсутствие flakiness.

### Акценты

- _Триггерные правила: EC-0010, EC-0012, ES-0051, ES-0120._
- _Singleflight гарантия: `Forget` сразу после `Do` — без этого следующий refresh ждёт stale (см. [REQ-001-ADR §A3 + R8](../plans/REQ-001-ADR.md))._
- _Cross-instance limitation: singleflight работает только в пределах одного процесса SDK. Multi-replica BFF — accepted limitation v0.1.0, документировать (см. [REQ-001-ADR §R3](../plans/REQ-001-ADR.md))._
- _Refresh НЕ ретраить (retry policy для UNAVAILABLE в gRPC service config исключает RefreshToken — установлено в TASK-002, см. [REQ-001-ADR §R11](../plans/REQ-001-ADR.md))._
- _Fingerprint binding: `ip`/`user_agent` параметры передаются явно. SDK не извлекает их сам из `http.Request` — это делает consumer через хелперы из TASK-005 (см. [REQ-001-ADR §A12](../plans/REQ-001-ADR.md))._
- _Логи не содержат токены/пароли/email — только UUID (фиксируется в unit-тесте на содержимое лог-вызовов через NoopLogger replacement)._

---

## DOD (Definition of Done)

- [ ] `lib/internal/refresh/group.go` экспортирует `Group` с `Do(key, fn)` (Forget внутри).
- [ ] `lib/auth.go` содержит публичные типы `LoginInput`, `LoginOutput`, `RefreshOutput` и методы `Login`/`Refresh`/`Logout`/`LogoutAll`/`ChangePassword`/`GetCurrentUser` на `*Client`.
- [ ] Unit-test `TestRefresh_SingleFlight`: 2 goroutine с одним refresh_token → mock `RefreshToken` вызван **ровно 1 раз** (проверка через `mock.AssertNumberOfCalls(t, "RefreshToken", 1)`).
- [ ] `errors.Is(err, ErrInvalidToken|ErrRateLimited|ErrUnavailable|...)` корректно для всех ошибочных сценариев.
- [ ] `AccessExpiresIn`/`RefreshExpiresIn` корректно вычисляются (`time.Until(time.Unix(seconds, 0))`).
- [ ] Integration-тесты `TestLogin_Happy`, `TestRefresh_HappyAndRotation`, `TestRefresh_Race` (10 goroutine), `TestLogout`, `TestChangePassword`, `TestGetCurrentUser_HasEmail` проходят через `make test-integration`.
- [ ] `go test -race -run TestRefresh_Race -count=10 ./lib/...` — все 10 прогонов зелёные.
- [ ] Unit coverage по `auth.go` + `internal/refresh/` ≥ 80%.
- [ ] Логи не содержат токены/пароли (test inspects log fields, fields list MUST NOT contain values matching token-pattern).
- [ ] `make audit` проходит.
