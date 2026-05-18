# Context helpers + Cookie helpers + HTTP request helpers (IP/User-Agent)

---

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](./REQ-001.md)

**Priority**: medium
**Depends on**: REQ-001-TASK-002

---

## WHY

Удобные хелперы, которые позволяют consumer'у написать защищённый handler в 3 строки: получить `User` из context, поставить refresh cookie с правильными security-параметрами, передать в `Login`/`Refresh` корректный IP/UA реального клиента (а не BFF). Без этих helpers consumer ошибётся в одном из четырёх типичных мест (HttpOnly без Secure, Path не сужен, IP взят из `RemoteAddr` вместо `X-Forwarded-For`, etc.).

---

## WHAT

1. **Создать `lib/context.go`.** Определить:
    - `Role string` тип + константы `RoleUnknown=""`, `RoleUser="user"`, `RoleAdmin="admin"` (values синхронизированы с `auth.v1.proto`, мапятся в TASK-003 через `Role(resp.Role)`).
    - `User struct{UUID, Role Role, Email string}`.
    - Методы: `(u User).IsZero() bool` (UUID==""), `(u User).IsAdmin() bool`, `(u User).HasRole(role Role) bool`.
    - `type userCtxKey struct{}` (unexported, EC-0056).
    - `WithUser(ctx, user) context.Context` и `UserFromContext(ctx) (User, bool)` (comma-ok идиома).
2. **Создать `lib/cookies.go`.** Определить:
    - Cookie defaults: `Name="refresh_token"`, `HttpOnly=true`, `Secure=true`, `SameSite=http.SameSiteStrictMode`, `Path="/auth/refresh"`, `MaxAge=14*24*60*60`, `Domain=""` (host-only).
    - Тип `CookieOption func(*http.Cookie)`.
    - Опции: `WithCookieDomain(domain)`, `WithCookiePath(path)`, `WithCookieMaxAge(maxAge time.Duration)`, `WithCookieSameSite(mode http.SameSite)`.
    - `SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)` — собирает `http.Cookie` с defaults, применяет opts, `http.SetCookie(w, &cookie)`.
    - `ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)` — `MaxAge=-1`, value="", остальные параметры из defaults+opts (Path/Domain обязаны совпадать, иначе браузер не сотрёт).
3. **Создать `lib/helpers.go`.** Определить:
    - `IPFromRequest(r *http.Request) string` — алгоритм согласно [REQ-001-ADR §A12](../plans/REQ-001-ADR.md): `X-Real-IP` (если непусто) → `X-Forwarded-For[0]` (первый IP до запятой, trim spaces) → `host` из `net.SplitHostPort(r.RemoteAddr)` → если `SplitHostPort` ошибся (IP без порта), вернуть `r.RemoteAddr` как есть.
    - `UserAgentFromRequest(r *http.Request) string` — обёртка `r.UserAgent()` (для симметрии API; reserve для будущей XFF-сложной логики).
4. **Покрыть unit-тестами (table-driven).**
    - `UserFromContext`: `(User{}, false)` если в ctx нет user; `(filled User, true)` после `WithUser`.
    - `User.IsAdmin`/`IsZero`/`HasRole`: все ветки.
    - `SetRefreshCookie`: defaults применяются; каждая опция переопределяет соответствующий field; `Secure=true` и `HttpOnly=true` всегда (нельзя выключить — это security regression).
    - `ClearRefreshCookie`: `MaxAge=-1`, Path/Domain из defaults+opts.
    - `IPFromRequest`: 
        - `X-Real-IP: 203.0.113.1` → `"203.0.113.1"`.
        - `X-Forwarded-For: 203.0.113.1, 10.0.0.1, 10.0.0.2` → `"203.0.113.1"`.
        - `X-Forwarded-For: " 203.0.113.1 "` (с пробелами) → `"203.0.113.1"` (trim).
        - `RemoteAddr: 192.0.2.1:54321` → `"192.0.2.1"`.
        - `RemoteAddr: [::1]:54321` (IPv6) → `"::1"`.
        - `RemoteAddr: 192.0.2.1` (без порта) → `"192.0.2.1"`.
        - Все три источника пусты → `""` (consumer должен заметить).
    - `UserAgentFromRequest`: пустой UA, типичный UA, UA с unicode.

### Акценты

- _Триггерные правила: EC-0056 (unexported context key), ES-0051, ES-0120._
- _OWASP Session Management: HttpOnly+Secure+SameSite+Path scoping (см. [REQ-001-ADR §A9](../plans/REQ-001-ADR.md))._
- _Cookie defaults — **frozen после v1.0.0**, изменение = security regression (см. evolution table в [REQ-001-ADR §API](../plans/REQ-001-ADR.md))._
- _`IPFromRequest` в v0.1.0 — без trusted-proxies-валидации (`WithTrustedProxies` отложено в v0.2.0). Документировать spoofing-риски в godoc-комментарии на функции (см. [REQ-001-ADR §A12](../plans/REQ-001-ADR.md))._
- _AUTH считает fingerprint как `SHA-256(UA + first-3-octets(IP))` — IP `203.0.113.55` и `203.0.113.99` дадут одинаковый fingerprint (та же /24). Это intentional behavior AUTH, в SDK ничего адаптировать не надо._
- _`Role` type-alias не использовать — отдельный type для type safety._

---

## DOD (Definition of Done)

- [ ] `lib/context.go` экспортирует `Role`, константы `RoleUnknown/User/Admin`, `User` struct с методами `IsZero/IsAdmin/HasRole`, `WithUser`, `UserFromContext`.
- [ ] `UserFromContext` использует comma-ok идиому (`(User, bool)`).
- [ ] `userCtxKey` — unexported `struct{}` (EC-0056).
- [ ] `lib/cookies.go` экспортирует `CookieOption`, 4 опции (`WithCookieDomain/Path/MaxAge/SameSite`), `SetRefreshCookie`, `ClearRefreshCookie`.
- [ ] Defaults `SetRefreshCookie` ВКЛЮЧАЮТ: `HttpOnly=true`, `Secure=true`, `SameSite=http.SameSiteStrictMode`, `Path="/auth/refresh"`, `MaxAge=14*24*60*60`.
- [ ] `HttpOnly` и `Secure` НЕ имеют опций для отключения (table-driven test проверяет — нет `WithCookieInsecure`/`WithCookieAllowJS`).
- [ ] `ClearRefreshCookie` устанавливает `MaxAge=-1` и пустое значение; сохраняет Path/Domain (иначе браузер не сотрёт).
- [ ] `lib/helpers.go` экспортирует `IPFromRequest`, `UserAgentFromRequest`.
- [ ] `IPFromRequest` table-driven test покрывает все ветки fallback chain и edge cases (multi-IP в XFF, IPv6, пробелы, пустые источники).
- [ ] Unit coverage по `context.go`/`cookies.go`/`helpers.go` ≥ 90% (простой код, легко покрыть).
- [ ] `make audit` проходит.
