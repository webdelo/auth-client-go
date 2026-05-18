# README + godoc + CHANGELOG + tag v0.1.0

---

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](./REQ-001.md)

**Priority**: high
**Depends on**: REQ-001-TASK-006

---

## WHY

Без README SDK нельзя начать использовать. Без godoc — нельзя понять API в IDE. Без CHANGELOG — нельзя апгрейдить безопасно. Без тега `v0.1.0` — `go get` не работает. Это финальная задача umbrella REQ-001, превращающая working code в публикуемый артефакт.

---

## WHAT

1. **Создать `lib/doc.go`.** Package-level godoc: краткое описание SDK, ссылка на server (`github.com/webdelo/auth`), proto (`github.com/webdelo/auth-proto`), стандартный quickstart (5 строк кода). Без длинных эссе — это reference, не туториал.
2. **Добавить godoc-комментарии на все exported.** Пройти по каждому файлу `lib/*.go`: типы (`Client`, `Config`, `FailMode`, `Option`, `User`, `Role`, `LoginInput`, `LoginOutput`, `RefreshOutput`, `CookieOption`, `Logger`, `Cache`), sentinel errors, все exported функции/методы/опции. Каждый godoc начинается с имени объекта (`// Client is the ...`). Для security-чувствительных опций (`FailOpen`, `WithCookieSameSite`) — warning в комментарии.
3. **Написать `README.md`.** Содержит секции:
    - Installation (`go get github.com/webdelo/auth-client-go@v0.1.0`).
    - Quickstart (≤ 15 строк: New + HTTPMiddleware + handler читает UserFromContext).
    - Configuration table (все Config-поля + defaults + описание; все опции).
    - FailMode section с CAPSLOCK security warning про `FailOpen`.
    - Typed errors table (mapping gRPC code → SDK error, с примером `errors.Is`).
    - Adapter interfaces (Logger, Cache) с примером slog-обёртки.
    - Cookie helpers (defaults table, override patterns для multi-domain).
    - HTTP request helpers (IP/UserAgent — fingerprint binding rationale, spoofing caveat).
    - Examples (ссылки на `lib/examples/*`).
    - Local integration testing (см. TASK-001).
    - TLS setup (production: `WithTransportCredentials(credentials.NewTLS(...))`).
    - Troubleshooting:
        - debug 401 (включить Logger, посмотреть на `WWW-Authenticate` header в response);
        - fingerprint caveats (NAT, CGNAT, /24-binding);
        - keepalive auto-bump (Time < 10s);
        - distributed singleflight отсутствует (cross-replica BFF).
    - Known limitations (no distributed singleflight, no trusted-proxies validation, no Metrics interface, ValidateTokenResponse без email).
    - Versioning policy (semver, frozen API после v1.0.0).
4. **Обновить `CHANGELOG.md`.** Запись `## v0.1.0 — YYYY-MM-DD`:
    - Added: full feature list (Login/Validate/Refresh/Logout/LogoutAll/ChangePassword/GetCurrentUser, HTTPMiddleware, UnaryInterceptor, context/cookie/helpers, typed errors, Logger/Cache adapters).
    - Known limitations (см. README).
    - Acknowledgements (на основе чего сделано — auth-proto v0.1.0, OWASP refs).
5. **Сверить публичную поверхность с ADR.** `go doc github.com/webdelo/auth-client-go/lib | grep -E "^(type|func|var|const)"` — список должен соответствовать [REQ-001-ADR §API](../plans/REQ-001-ADR.md). Никаких лишних exported (`golint` обязан быть чистым), никаких missing.
6. **Зафиксировать тег `v0.1.0`.** Через `make release` (interactive) или `make release-dry-run` для preview. Проверить, что Go-toolchain видит тег:
   ```bash
   GOPROXY=direct go list -m github.com/webdelo/auth-client-go@v0.1.0
   ```
7. **Smoke-валидация на BFF-разработчике.** Один человек (не автор REQ) пытается через `go get github.com/webdelo/auth-client-go@v0.1.0` собрать рабочий middleware за < 30 минут, опираясь ТОЛЬКО на README. Время фиксируется в отчёте; если > 30 минут — fix README до релиза.

### Акценты

- _Триггерные правила: ES-0051, ES-0140._
- _README — не туториал, а reference. Quickstart короткий, остальное — таблицы и ссылки._
- _Godoc стиль: «Verb-first» (`// New creates a new Client...`), не `// This function does X...`._
- _Security warnings в godoc: `FailOpen`, `WithCookieSameSite`, `WithTrustedProxies` (когда добавится) — CAPSLOCK либо явный `// WARNING:`._
- _CHANGELOG формат: [Keep a Changelog](https://keepachangelog.com/) или совместимый — секции Added/Changed/Deprecated/Removed/Fixed/Security._
- _Тег формата `v0.1.0` (без префикса `go/` — это не submodule, репо целиком есть Go-модуль)._
- _BFF-валидация — мягкий критерий, но обязательный: если за 30 минут не собирается — README плох._

---

## DOD (Definition of Done)

- [ ] `lib/doc.go` существует, содержит package-level godoc.
- [ ] Все exported типы/функции/методы/константы/sentinel-ошибки имеют godoc-комментарии (проверка `go doc lib | grep -B1 "^func\|^type\|^var\|^const" | grep -v "^--" | grep -v "^//" | head` — нет пустых строк перед declarations).
- [ ] README.md содержит все секции из WHAT п. 3.
- [ ] README имеет CAPSLOCK security warning про `FailOpen`.
- [ ] README имеет таблицу typed errors и пример `errors.Is`.
- [ ] CHANGELOG.md содержит запись `## v0.1.0 — YYYY-MM-DD` с Added и Known limitations секциями.
- [ ] Публичная поверхность SDK (`go doc github.com/webdelo/auth-client-go/lib`) соответствует ADR §API (sanity check: 5 sentinels, 4 With*Option, 5 With*CookieOption, 9 методов на Client, 4 helpers).
- [ ] Тег `v0.1.0` опубликован в `webdelo/auth-client-go` (`git tag -l v0.1.0` + remote check).
- [ ] BFF-валидатор (другой разработчик) собрал working middleware через `go get github.com/webdelo/auth-client-go@v0.1.0` за < 30 минут, отчёт в `task_management/results/REQ-001-TASK-007-bff-validation.md`.
- [ ] `go mod why github.com/webdelo/auth-proto/go` подтверждает прямую зависимость v0.1.0.
- [ ] `make audit` + `make build-examples` + `make test-integration` зелёные.
- [ ] Umbrella DOD из [REQ-001 §DOD](./REQ-001.md) полностью закрыт (все 13 пунктов чек-листа).
