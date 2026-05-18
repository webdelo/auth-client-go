# REQ-001-TASK-005-PRP: Cookie helpers + HTTP request helpers (IP/User-Agent)

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-001-TASK-005-PRP |
| Parent Request | [REQ-001](../tasks/REQ-001.md) |
| Parent Task | [REQ-001-TASK-005](../tasks/REQ-001-TASK-005.md) |
| Created | 2026-05-18 |
| Updated | 2026-05-18 |
| PRP Score | 9/10 |

---

## Goal

### Summary

Реализовать оставшиеся consumer-facing helpers SDK: `lib/cookies.go` (refresh-cookie с security-defaults и functional options) и `lib/helpers.go` (`IPFromRequest`, `UserAgentFromRequest` — fingerprint binding для `Login`/`Refresh`).

### Scope shift (важно — фиксируется в начале)

Исходный TASK-005 включал три файла: `context.go`, `cookies.go`, `helpers.go`. Согласно [REQ-001-TASK-003 alternatives analysis (2026-05-18)](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md), `lib/context.go` (User, Role, IsZero/IsAdmin/HasRole, WithUser, UserFromContext, userCtxKey) был перенесён в TASK-003. Решение — **Option A: TASK-003 берёт всё**. Это устранило implicit cross-task dependency (TASK-003 первой использует `User`/`Role` в кеше и middleware).

TASK-005 теперь сводится к двум файлам: `cookies.go` + `helpers.go`. PRP включает:
1. Verification step (Task 0) — `lib/context.go` существует от TASK-003 c корректной surface.
2. Реализация двух новых файлов + их unit-тесты.
3. Документация — обновление `task_management/tasks/REQ-001-TASK-005.md` для отражения реального scope (Task 5).

### Why

- **Cookie defaults — frozen security primitives.** Без хелперов consumer ошибётся минимум в одном из четырёх типичных мест: `HttpOnly` без `Secure`, `Path` не сужен под `/auth/refresh`, забыт `SameSite`, неправильный domain. SDK инкапсулирует OWASP-compliant defaults; функциональные опции позволяют tweak без возможности отключить security (нет `WithCookieInsecure` / `WithCookieAllowJS` — это «фиксированно нельзя» после v1.0.0).
- **Fingerprint binding — критическая зона.** AUTH считает fingerprint как `SHA-256(UA + first-3-octets(IP))`. Если SDK сам бы автоматически брал `r.RemoteAddr` для `Login`/`Refresh`, AUTH получил бы IP BFF, а не реального клиента — fingerprint бесполезен (auth bypass через NAT любого). Решение: явные helpers `IPFromRequest`/`UserAgentFromRequest`, consumer прокидывает их в `LoginInput.ClientIP` / `Refresh(refresh, ip, ua)`. ([REQ-001-ADR §A12](./REQ-001-ADR.md))
- **`ClearRefreshCookie` правильно стирает.** Браузер удалит cookie только если Path/Domain совпадают с set-операцией. Если consumer ставит `Path="/auth/refresh"` через `SetRefreshCookie`, а потом вызывает `http.SetCookie(w, &http.Cookie{Name: "refresh_token", MaxAge: -1})` — cookie не удалится (default Path `/`). `ClearRefreshCookie` использует те же defaults, что и `SetRefreshCookie` → симметрично.
- **3 строки в защищённом handler.** Со всеми тремя файлами (`context.go` уже есть из TASK-003, `cookies.go`/`helpers.go` из TASK-005) consumer пишет: `user, _ := authclient.UserFromContext(r.Context())` + `authclient.SetRefreshCookie(w, tokens.RefreshToken)` + `client.Login(ctx, LoginInput{..., ClientIP: authclient.IPFromRequest(r), UserAgent: authclient.UserAgentFromRequest(r)})`.

### What

После TASK-005 потребитель SDK может:

```go
// Cookie helpers — refresh-token storage.
authclient.SetRefreshCookie(w, tokens.RefreshToken)
authclient.SetRefreshCookie(w, tokens.RefreshToken,
    authclient.WithCookieDomain(".webdelo.trade"),
    authclient.WithCookieMaxAge(7*24*time.Hour),
)
authclient.ClearRefreshCookie(w)
authclient.ClearRefreshCookie(w, authclient.WithCookieDomain(".webdelo.trade"))

// HTTP request helpers — fingerprint extraction.
clientIP := authclient.IPFromRequest(r)
userAgent := authclient.UserAgentFromRequest(r)

out, err := client.Login(ctx, authclient.LoginInput{
    Email: req.Email, Password: req.Password,
    ClientIP: clientIP, UserAgent: userAgent,
})
```

Все экспортируемые символы:
- Имеют godoc-комментарий, начинающийся с имени (ES-0080/0081).
- Стабильны на участке `v0.1.0+` — нарушение defaults = security regression ([REQ-001-ADR §A9](./REQ-001-ADR.md) evolution table).

### Success Criteria

- [ ] `lib/cookies.go` экспортирует `CookieOption`, 4 опции (`WithCookieDomain`/`WithCookiePath`/`WithCookieMaxAge`/`WithCookieSameSite`), `SetRefreshCookie`, `ClearRefreshCookie`.
- [ ] `SetRefreshCookie` defaults включают: `Name="refresh_token"`, `HttpOnly=true`, `Secure=true`, `SameSite=http.SameSiteStrictMode`, `Path="/auth/refresh"`, `MaxAge=14*24*60*60` секунд, `Domain=""` (host-only).
- [ ] Нет опций для отключения `HttpOnly` или `Secure` (no `WithCookieInsecure`, no `WithCookieAllowJS`) — gate'ится unit-тестом, проверяющим, что `Secure` и `HttpOnly` остаются `true` после применения произвольной комбинации опций.
- [ ] `ClearRefreshCookie` устанавливает `MaxAge=-1`, `Value=""`, `Expires=time.Unix(0, 0)` (RFC 6265 §5.3 legacy-browser compat), сохраняя Path/Domain/SameSite/Secure/HttpOnly из defaults и применённых опций.
- [ ] `lib/helpers.go` экспортирует `IPFromRequest`, `UserAgentFromRequest`.
- [ ] `IPFromRequest` fallback chain: `X-Real-IP` (non-empty) → `X-Forwarded-For[0]` (trim spaces, take before first comma) → `net.SplitHostPort(r.RemoteAddr)` host part → `r.RemoteAddr` as-is (когда `SplitHostPort` errored).
- [ ] `UserAgentFromRequest(r) == r.UserAgent()` — pure passthrough.
- [ ] `lib/cookies_test.go` — table-driven, покрывает defaults / каждую опцию / clear-симметрию / security regression guard.
- [ ] `lib/helpers_test.go` — table-driven, покрывает все 7 IP edge-cases из TASK spec (X-Real-IP, XFF multi, XFF whitespace, RemoteAddr IPv4, RemoteAddr IPv6, RemoteAddr no port, all empty) + 3 UA кейса (пустой, типичный, unicode).
- [ ] Unit coverage по `lib/cookies.go` + `lib/helpers.go` ≥ 90% (`make coverage`).
- [ ] `make audit` (vet + lint + test) проходит без ошибок.
- [ ] `task_management/tasks/REQ-001-TASK-005.md` обновлён — пункт 1 (context.go) и связанные DOD-чекбоксы помечены как `(absorbed by TASK-003)`, ссылка на alternatives analysis добавлена.

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен через `.temp/rules/REQ-001-TASK-005/` |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | ES-0010, ES-0011, ES-0015, ES-0016, ES-0021, ES-0030, ES-0031, ES-0040, ES-0042, ES-0045, ES-0051, ES-0060, ES-0070, ES-0072, ES-0080, ES-0081, ES-0110, ES-0120, ES-0130, ES-0140, ES-0143, ES-0170, ES-0171, ES-0173 |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | CS-0001, CS-0002, CS-0010, CS-0020, CS-0021, CS-0022, CS-0023, CS-0024, CS-0030, CS-0060, CS-0061, CS-0070, CS-0071, CS-0080, CS-0090, CS-0111, CS-0112, CS-0115 (CookieOption — VO-style), CS-0122 |
| Effective Concurrency | [effective_concurrency.md](../../.claude/guides/golang/effective_concurrency.md) | EC-0056 (verified pre-existing from TASK-003) |
| Unit Tests | [unit_tests_standards.md](../../.claude/guides/golang/unit_tests_standards.md) | UT-0001, UT-0002, UT-0010, UT-0011, UT-0012, UT-0027, UT-0028, UT-0029, UT-0030, UT-0031, UT-0032, UT-0040, UT-0080 |

### Critical rules — точечный список

- **CS-0050** Each entity in separate package — `cookies.go` и `helpers.go` остаются в `package authclient` (lib-fronting), не выносятся в подпакеты: они мелкие, public, тесно завязаны на `http.Cookie`/`http.Request`. Подпакетов в `lib/` для мелких stateless helpers не делаем (это соответствует layout TASK-002/003/004 — все public API живут в `lib/*.go` под одним package'ом).
- **EC-0056** Use custom types for context keys — **проверочный** (TASK-003 уже реализовал `type userCtxKey struct{}`). TASK-005 НЕ создаёт новых context keys.
- **ES-0021** Return errors early (guard clauses) — `IPFromRequest`: short-circuit на каждой ветке fallback chain.
- **ES-0040** Return value and error — `IPFromRequest` возвращает только `string` (error-less; пустой string = «не смог определить»). Это OK для best-effort helper и **зафиксировано в спецификации TASK-005** («все три источника пусты → `""` (consumer должен заметить)»).
- **ES-0045** No optional parameters via zero values — `CookieOption` функциональные опции вместо `SetRefreshCookie(w, token, domain, path, maxAge, sameSite)` (был бы антипаттерн).
- **ES-0080/0081** Doc comments precede declaration, начинаются с имени.
- **ES-0120** Use functional options for many optional parameters — `CookieOption func(*http.Cookie)` mirrors `Option func(*Client)` из TASK-002.
- **ES-0143** Use noop defaults for optional dependencies — cookies/helpers не имеют dependencies (pure stdlib), правило здесь зеркальное: defaults в `SetRefreshCookie` non-nil/sensible, никаких nil-check на caller-side.
- **ES-0170** Prefer value parameters over pointers — `CookieOption func(*http.Cookie)` принимает pointer (нужно мутировать), `IPFromRequest(r *http.Request)`/`UserAgentFromRequest(r *http.Request)` — pointer (стандарт стдлиба, не копировать большой struct).
- **ES-0171** No `nolint` annotations.
- **UT-0001** Table-driven tests mandatory.
- **UT-0011** First case `happy_path`.
- **UT-0023** Полный запрет `mock.Anything` — здесь mocks НЕ нужны вообще (pure stdlib, нет dependencies).
- **UT-0028** No helper functions for test data — все test data inline в case-table.
- **UT-0040** `t.Parallel()` в `TestXxx` и в каждом subtest. **Исключение**: subtests, у которых нет shared state и используют только локальные переменные, всё равно ставят `t.Parallel()` (UT-0040 не делает исключений для «маленьких» подтестов).

---

## Analysis Files

- **Parent REQ + ADR**: [REQ-001.md](../tasks/REQ-001.md), [REQ-001-ADR.md](./REQ-001-ADR.md) (§A8 — context conventions, верифицируется как pre-existing; §A9 — cookie helpers; §A12 — fingerprint binding helpers; §R2 — fingerprint risks за NAT).
- **Scope shift decision**: [2026-05-18-REQ-001-TASK-003-alternatives-analysis.md](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md) — Option A: TASK-003 absorbs `lib/context.go`.
- **Prior PRPs (контракт-источники, верифицируются Task 0)**:
  - [REQ-001-TASK-002-PRP.md](./REQ-001-TASK-002-PRP.md) — `package authclient` rename, layout `lib/*.go`, golangci config.
  - [REQ-001-TASK-003-PRP.md](./REQ-001-TASK-003-PRP.md) — `lib/context.go` finalized here (User/Role/WithUser/UserFromContext).
  - [REQ-001-TASK-004-PRP.md](./REQ-001-TASK-004-PRP.md) — `LoginInput.ClientIP`/`UserAgent` consume `IPFromRequest`/`UserAgentFromRequest` от TASK-005.
- **Code research**: [.memory_bank/agent_task_results/2026-05-18_code-research_REQ-001-TASK-005.md](../../.memory_bank/agent_task_results/2026-05-18_code-research_REQ-001-TASK-005.md) — verified godoc/test-style в `lib/`, паттерны cookie из `bots/auth/internal/service/httpcookies/`, IP extraction из `bots/auth/internal/handlers/http/auth/requestctx.go`. **Key takeaway:** `Clear` в auth-service использует и `MaxAge=-1`, и `Expires=time.Unix(0, 0)` для RFC 6265 §5.3 legacy-browser compat — этот паттерн **incorporated** в `ClearRefreshCookie` (Task 1 ниже).

---

## Context

### Documentation & References

```yaml
# === Critical ===

- file: task_management/tasks/REQ-001-TASK-005.md
  why: Source TASK — WHAT/WHY/DOD/акценты. ВСЕ 7 IP edge cases + 3 UA edge cases — обязательны.
  critical: |
    TASK spec пункт 1 (context.go) АНУЛИРОВАН по alternatives analysis 2026-05-18.
    Task 5 PRP обновляет TASK файл — пункт 1 + связанные DOD-чекбоксы помечаются (absorbed by TASK-003).

- file: task_management/plans/REQ-001-ADR.md
  section: "§A9 Cookie helpers"
  critical: |
    Defaults frozen после v1.0.0. Никаких опций для disabling HttpOnly/Secure.
    Path="/auth/refresh" — минимизация attack surface (refresh-cookie не отдаётся на /api/*).
    MaxAge=14*24*60*60 sec — синхронизировано c refresh_token TTL в AUTH.
    Domain="" (host-only) — без поделиться между поддоменами без явной опции.

- file: task_management/plans/REQ-001-ADR.md
  section: "§A12 Fingerprint binding (helpers)"
  critical: |
    IPFromRequest алгоритм для v0.1.0 — без trusted-proxies-валидации.
    Documentation MUST flag spoofing risk (X-Real-IP / XFF под контролем consumer'а — должен иметь trusted proxy).
    WithTrustedProxies(cidrs ...netip.Prefix) — additive в v0.2.0.

- file: task_management/plans/REQ-001-ADR.md
  section: "API surface (§API CookieOption + helpers)"
  why: |
    func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)
    func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)
    func WithCookieDomain(domain string) CookieOption
    func WithCookiePath(path string) CookieOption
    func WithCookieMaxAge(maxAge time.Duration) CookieOption
    func WithCookieSameSite(mode http.SameSite) CookieOption
    func IPFromRequest(r *http.Request) string
    func UserAgentFromRequest(r *http.Request) string

- file: task_management/plans/REQ-001-TASK-002-PRP.md
  section: "Task: package rename to authclient"
  critical: |
    Все файлы под lib/*.go используют `package authclient`. TASK-005 файлы — тоже.
    `lib/lib.go` + `lib/lib_test.go` удалены TASK-002.

- file: task_management/plans/REQ-001-TASK-003-PRP.md
  section: "Task 2: CREATE lib/context.go"
  critical: |
    lib/context.go УЖЕ создан в TASK-003 с полным surface (User/Role/IsZero/IsAdmin/HasRole, userCtxKey, WithUser, UserFromContext).
    TASK-005 НЕ создаёт context.go. Task 0 верифицирует, что surface совпадает.

# === stdlib & spec references ===

- url: https://pkg.go.dev/net/http#Cookie
  section: "Cookie struct fields"
  critical: |
    http.Cookie.MaxAge — int (seconds). Negative → expire immediately (browser delete).
    Zero → session cookie (browser-dependent; не используем — у нас всегда положительный MaxAge).
    http.Cookie.SameSite — type http.SameSite. http.SameSiteStrictMode value.

- url: https://pkg.go.dev/net/http#SetCookie
  critical: |
    http.SetCookie добавляет Set-Cookie header. Множественные cookie на один response — повторные вызовы.
    Cookie.String() escape-safe для значения, если оно содержит спецсимволы.
    Refresh tokens с AUTH — base64url, не требуют экранирования, но Cookie.String() обрабатывает корректно.

- url: https://datatracker.ietf.org/doc/html/rfc6265#section-5.3
  section: "Storage Model"
  critical: |
    Cookie identity = (Name, Domain, Path) triple. ClearRefreshCookie должен УКАЗАТЬ те же Path/Domain,
    что и SetRefreshCookie — иначе браузер не сотрёт. Symmetry — основной инвариант ClearRefreshCookie.

- url: https://datatracker.ietf.org/doc/html/rfc7239
  section: "Forwarded header"
  why: |
    RFC 7239 — стандартный Forwarded header (вместо X-Forwarded-For). НЕ используем в v0.1.0:
    реальный мир — XFF + X-Real-IP (nginx, ALB, Cloudflare). RFC 7239 — отложен в v0.2.0.

- url: https://owasp.org/Top10/2025/A07_2025-Authentication_Failures/
  why: |
    Fingerprint binding должен быть на real client IP/UA, не proxy/BFF. Это базовая защита.
    IPFromRequest godoc должен явно предупреждать о spoofing-риске (consumer обязан иметь trusted proxy).

- url: https://pkg.go.dev/net#SplitHostPort
  critical: |
    SplitHostPort("192.0.2.1:54321") = ("192.0.2.1", "54321", nil)
    SplitHostPort("[::1]:54321") = ("::1", "54321", nil)  ← без квадратных скобок в host
    SplitHostPort("192.0.2.1") = ("", "", &AddrError{...})  ← err, нужен fallback
    SplitHostPort("") = ("", "", &AddrError{...})

# === Прецеденты sibling-репозиториев (если найдено code-researcher'ом) ===

- file: ~/Development/TRADING/bots/auth/internal/...
  why: |
    AUTH сам себе SetCookie не делает (он gRPC). Cookie-управление — на стороне consumer'а
    (auth.webdelo.trade frontend в текущем deployment'е). Прецедентов в sibling-репах ожидается мало.
    code-researcher findings будут supplementary — TASK self-contained от спецификации.

- file: ~/Development/TRADING/bots/auth-client-go/.memory_bank/agent_task_results/2026-05-18_code-research_REQ-001-TASK-005.md
  why: |
    Когда отчёт опубликуется — добавить ссылки на конкретные паттерны (godoc style, test layout)
    как supplementary references. PRP не зависит от этого отчёта в принятии решений.
```

### Current Structure (после TASK-002/003/004)

```
auth-client-go/
├── go.mod                              # module github.com/webdelo/auth-client-go
├── go.sum
├── Makefile
├── mockery.yaml
├── .golangci.yaml
├── lib/
│   ├── client.go                       # (TASK-002) Client/Config/New/Close/Option
│   ├── errors.go                       # (TASK-002) sentinel + mapGRPCError + AuthError + RetryAfter
│   ├── logger.go                       # (TASK-002) Logger interface + NoopLogger
│   ├── context.go                      # (TASK-003) Role + User + IsZero/IsAdmin/HasRole + userCtxKey
│   │                                   #            + WithUser + UserFromContext  ◄── ПРОВЕРИТЬ В Task 0
│   ├── validate.go                     # (TASK-003) ValidateAccessToken + hashToken
│   ├── middleware_http.go              # (TASK-003) HTTPMiddleware
│   ├── interceptor_grpc.go             # (TASK-003) UnaryInterceptor
│   ├── auth.go                         # (TASK-004) Login/Refresh/Logout/LogoutAll/ChangePassword/GetCurrentUser
│   ├── *_test.go                       # (TASK-002/003/004) соответствующие unit-тесты
│   ├── internal/
│   │   ├── cache/memory.go             # (TASK-003)
│   │   └── refresh/group.go            # (TASK-004)
│   ├── mocks/                          # (TASK-003+) сгенерены mockery
│   └── examples/basic/main.go          # (TASK-002 stub; полные примеры в TASK-006)
├── dev/deployments/                    # (TASK-001) compose.test.yaml, test.env, extra.mk
└── task_management/
    ├── tasks/
    │   ├── REQ-001.md
    │   ├── REQ-001-TASK-005.md         # ◄── ОБНОВЛЯЕТСЯ в Task 5
    │   └── ...
    └── plans/
        ├── REQ-001-ADR.md
        ├── REQ-001-TASK-001-PRP.md, ..., REQ-001-TASK-004-PRP.md
        └── REQ-001-TASK-005-PRP.md     # ◄── этот файл
```

### Desired Structure (after TASK-005)

```
lib/
├── client.go                           # unchanged
├── errors.go                           # unchanged
├── logger.go                           # unchanged
├── context.go                          # unchanged (verified in Task 0)
├── validate.go                         # unchanged
├── middleware_http.go                  # unchanged
├── interceptor_grpc.go                 # unchanged
├── auth.go                             # unchanged
├── cookies.go                          # NEW: SetRefreshCookie + ClearRefreshCookie + CookieOption + 4 With*
├── cookies_test.go                     # NEW: table-driven unit tests
├── helpers.go                          # NEW: IPFromRequest + UserAgentFromRequest
├── helpers_test.go                     # NEW: table-driven unit tests (7 IP + 3 UA cases)
├── internal/, mocks/, examples/        # unchanged
└── ...

task_management/tasks/
└── REQ-001-TASK-005.md                 # UPDATED: пункт 1 + соотв. DOD помечены (absorbed by TASK-003)
```

### Gotchas

| # | Category | Issue | Solution |
|---|----------|-------|----------|
| G1 | **Scope shift** | `lib/context.go` создан TASK-003, не TASK-005. Если PRP попытается создать заново — конфликт компиляции. | Task 0 verifies file existence + surface (User/Role/WithUser/UserFromContext exported). Если verify fails — STOP, эскалировать TASK-003 первым. |
| G2 | **Cookie identity (RFC 6265)** | Cookie identity = (Name, Domain, Path) triple. `ClearRefreshCookie` без указания Path/Domain не удалит cookie, поставленную `SetRefreshCookie`. | `ClearRefreshCookie` использует **те же defaults**, что и `SetRefreshCookie` (Path/Domain/SameSite/Secure/HttpOnly), затем применяет opts. MaxAge=-1, Value="". Симметрия — основной инвариант. |
| G3 | **Security regression guard** | Опции типа `WithCookieInsecure()` (Secure=false) или `WithCookieAllowJS()` (HttpOnly=false) — security regression. Они НЕ должны существовать. | В `lib/cookies.go` — только 4 опции: Domain/Path/MaxAge/SameSite. `Secure=true`, `HttpOnly=true` — hardcoded в `newRefreshCookie()` helper после применения опций (опция не может перезаписать). Unit-тест проходит произвольную комбинацию опций и assert'ит `cookie.Secure==true && cookie.HttpOnly==true`. |
| G4 | **MaxAge: time.Duration vs int** | TASK spec: `WithCookieMaxAge(maxAge time.Duration)`. `http.Cookie.MaxAge` — `int` seconds. Округление `<1s` durations → 0 (session cookie, не наш use case). | `WithCookieMaxAge` конвертирует: `int(maxAge.Seconds())`. Negative duration → негативный int → expire immediately (= ClearRefreshCookie). Это OK: caller знает, что делает. Godoc предупреждает: «Use ClearRefreshCookie to delete, not negative MaxAge». |
| G5 | **CookieOption order semantics** | Если caller передал 2 опции для одного поля (`WithCookiePath("/a"), WithCookiePath("/b")`), какая применяется? | Последняя побеждает — функциональные опции применяются в порядке поступления. Документируем в godoc `CookieOption`. |
| G6 | **SameSite=Strict и login redirect** | `SameSite=Strict` означает: cookie НЕ отправляется при cross-site requests (включая user-clicked link на login.example.com → app.example.com). Первый POST /auth/login отправит refresh-cookie только если пользователь уже на app.example.com (что норма для SPA). | Это intentional — refresh-cookie должна летать ТОЛЬКО при same-site POST /auth/refresh. SPA-сценарий: пользователь логинится, ставится refresh-cookie, последующие XHR на /auth/refresh — same-site → отправляется. Если consumer хочет server-side rendering c cross-site initial login → `WithCookieSameSite(http.SameSiteLaxMode)`. Godoc у `SetRefreshCookie` упоминает trade-off. |
| G7 | **X-Real-IP / X-Forwarded-For spoofing** | Без trusted-proxy валидации, любой клиент может прислать `X-Real-IP: 1.2.3.4` → SDK поверит. AUTH тогда вычислит fingerprint от подделанного IP. | v0.1.0: documentation only — godoc `IPFromRequest` явно говорит: «MUST be deployed behind trusted reverse proxy that strips/sets these headers. Without trusted proxy, this is spoofable.» WithTrustedProxies — additive v0.2.0 (ADR §A12). |
| G8 | **XFF whitespace + multi-hop** | `X-Forwarded-For: 203.0.113.1, 10.0.0.1, 10.0.0.2` — list of IPs. Первый — original client (если первая proxy trusted и не присоединила). С пробелами после запятой — стандарт. | `strings.SplitN(xff, ",", 2)[0]` → `" 203.0.113.1"`, затем `strings.TrimSpace(...)` → `"203.0.113.1"`. SplitN с лимитом 2 — экономия аллокации (не парсим весь хвост, нам нужен только первый IP). |
| G9 | **`net.SplitHostPort` failure modes** | `SplitHostPort("192.0.2.1")` (без порта) возвращает error. TASK spec edge case: `RemoteAddr: 192.0.2.1` → ожидаемый result `"192.0.2.1"`. | После ошибки `SplitHostPort` — return `r.RemoteAddr` as-is (TASK spec ветка 4). Это branch отдельно от `host==""` (пустой host тоже надо вернуть, но edge-кейс «весь RemoteAddr пуст» уже отброшен предыдущими проверками). |
| G10 | **IPv6 без brackets** | `SplitHostPort("[::1]:54321")` возвращает host `"::1"` (без скобок). Тест: `expected="::1"`, не `"[::1]"`. | Естественное поведение stdlib — никаких преобразований не делаем. |
| G11 | **`r.UserAgent()` return** | `r.UserAgent()` = `r.Header.Get("User-Agent")` — может быть `""`. Никаких преобразований, никакого default. | Wrapper-метод 1:1, существует для API-симметрии с `IPFromRequest` (consumer вызывает `authclient.IPFromRequest(r)` и `authclient.UserAgentFromRequest(r)` стилистически парой). |
| G12 | **`refresh_token` cookie value escaping** | AUTH issues opaque refresh tokens (base64url-like). Cookie value spec (RFC 6265) допускает большинство base64url-символов. `http.SetCookie` сам делает `Cookie.String()`, который quote-escape'ит. | Никаких ручных escape — передаём raw token в `Cookie.Value`. stdlib справляется. Тест: cookie с base64url-значением (`AbCdEf123-_.=`) корректно сериализуется. |
| G13 | **`http.Cookie.Path=""` default browser behavior** | Если `Path=""`, браузер использует Path of the request URL. Это **не наш case** — мы всегда ставим `Path="/auth/refresh"`. | Default `Path="/auth/refresh"` в `SetRefreshCookie`. Если caller передал `WithCookiePath("")` явно — пропускаем как есть (caller responsibility). Защитная проверка: НЕТ, не делаем (caller контракт — передавать осмысленный path). |
| G14 | **Test parallelism + http.ResponseWriter** | `t.Parallel()` в каждом subtest. `httptest.NewRecorder()` создаёт новый recorder на каждый subtest — изоляция гарантирована. | `httptest.NewRecorder()` локально в каждом subtest case. Никаких shared response writers. |
| G15 | **Legacy-browser `Expires` vs modern `MaxAge`** | RFC 6265 §5.3 (cookie storage): современные клиенты предпочитают `Max-Age`, но старые (IE ≤8, некоторые embedded WebViews) игнорируют его и читают только `Expires`. Если `ClearRefreshCookie` ставит только `MaxAge=-1`, такой клиент не удалит cookie. | `ClearRefreshCookie` ставит и `MaxAge=-1`, и `Expires=time.Unix(0, 0)` (1970-01-01 = далеко в прошлом → forced delete). Паттерн из `bots/auth/internal/service/httpcookies/refresh.go:55-66`. `SetRefreshCookie` — НЕ ставит `Expires` (MaxAge достаточно — современный браузер обновляет cookie storage; legacy-клиенты для refresh-flow не critical use case). |

---

## Implementation

### Data Models

```go
// lib/cookies.go

// CookieOption is a functional option that customizes the refresh-cookie set
// by SetRefreshCookie / cleared by ClearRefreshCookie. Options are applied
// after the SDK defaults — security-critical attributes (HttpOnly, Secure) are
// re-applied AFTER options to prevent accidental disablement.
//
// Multiple options targeting the same field are applied in call order
// (the last one wins).
type CookieOption func(*http.Cookie)
```

```go
// lib/helpers.go — нет публичных типов, только функции.
```

---

### Task 0 (pre-requisite): Verify `lib/context.go` exists from TASK-003

**Action:** VERIFY (no code changes if pass)

**Description:** Перед началом TASK-005 убедиться, что TASK-003 действительно создал `lib/context.go` c полной surface, ожидаемой по [alternatives analysis](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md). Если файла нет или surface неполная — STOP, эскалировать TASK-003.

**Details:**
- `lib/context.go` существует.
- `package authclient`.
- Экспортирует: `Role` (type), `RoleUnknown`/`RoleUser`/`RoleAdmin` (consts), `User` (struct с UUID/Role/Email), методы `User.IsZero()` / `User.IsAdmin()` / `User.HasRole(Role) bool`, `WithUser(ctx, User) context.Context`, `UserFromContext(ctx) (User, bool)`.
- `userCtxKey` — unexported `struct{}` (EC-0056).

**Commands:**
```bash
# Existence
test -f lib/context.go || (echo "MISSING lib/context.go — run TASK-003 first" && exit 1)

# Package name
grep -q "^package authclient" lib/context.go || (echo "BAD package in lib/context.go" && exit 1)

# Public surface
grep -q "^type Role " lib/context.go || (echo "MISSING Role type" && exit 1)
grep -q "^type User struct" lib/context.go || (echo "MISSING User struct" && exit 1)
grep -q "^func WithUser(" lib/context.go || (echo "MISSING WithUser" && exit 1)
grep -q "^func UserFromContext(" lib/context.go || (echo "MISSING UserFromContext" && exit 1)
grep -q "^func (u User) IsZero(" lib/context.go || (echo "MISSING IsZero" && exit 1)
grep -q "^func (u User) IsAdmin(" lib/context.go || (echo "MISSING IsAdmin" && exit 1)
grep -q "^func (u User) HasRole(" lib/context.go || (echo "MISSING HasRole" && exit 1)
```

**Failure handling:** Если какая-либо проверка failed — НЕ создавать `lib/context.go` в TASK-005. Эскалировать: «TASK-003 не выполнен или выполнен частично — re-run before TASK-005».

---

### Task 1: CREATE `lib/cookies.go`

**Action:** CREATE
**File:** [lib/cookies.go](../../lib/cookies.go)

**Description:** Реализовать `CookieOption`, 4 опции (`WithCookieDomain`/`WithCookiePath`/`WithCookieMaxAge`/`WithCookieSameSite`), `SetRefreshCookie`, `ClearRefreshCookie`. Security-critical attributes (`HttpOnly=true`, `Secure=true`) применяются ПОСЛЕ опций — опция не может отключить.

**Details:**
- `package authclient`.
- Импорты: `net/http`, `time`.
- Приватные константы внутри файла:
  - `defaultRefreshCookieName = "refresh_token"`
  - `defaultRefreshCookiePath = "/auth/refresh"`
  - `defaultRefreshCookieMaxAge = 14 * 24 * 60 * 60` (seconds)
- Приватный helper `newRefreshCookie(value string, opts []CookieOption) *http.Cookie`:
  1. Создаёт `*http.Cookie` c defaults: `Name`, `Value`, `Path`, `MaxAge`, `SameSite=http.SameSiteStrictMode`, `Domain=""`.
  2. Применяет opts по одному.
  3. **После** opts — re-применяет `HttpOnly=true`, `Secure=true` (security regression guard).
  4. Возвращает cookie.
- `SetRefreshCookie(w, token, opts...)`: builds cookie через `newRefreshCookie`, вызывает `http.SetCookie(w, cookie)`. Возвращает void.
- `ClearRefreshCookie(w, opts...)`: builds cookie через `newRefreshCookie` с `value=""`, **после** опций ставит `MaxAge=-1` (clears), `Value=""` (re-применяет, потому что опции могли затронуть Value? — НЕТ, нет опции `WithCookieValue`, но оставим явный invariant для будущего), и `Expires=time.Unix(0, 0)` для **RFC 6265 §5.3 legacy-browser compat** (старые клиенты могут игнорировать `MaxAge`, но честят `Expires` в прошлом). Вызывает `http.SetCookie`. Этот паттерн взят из `bots/auth/internal/service/httpcookies/refresh.go:55-66`.

**Code Pattern:**

```go
package authclient

import (
	"net/http"
	"time"
)

const (
	defaultRefreshCookieName   = "refresh_token"
	defaultRefreshCookiePath   = "/auth/refresh"
	defaultRefreshCookieMaxAge = 14 * 24 * 60 * 60 // 14 days in seconds, synced with AUTH refresh-token TTL.
)

// CookieOption customizes the refresh-cookie produced by SetRefreshCookie and
// ClearRefreshCookie. Options run AFTER the SDK defaults are applied; the
// SDK then re-asserts HttpOnly=true and Secure=true so options cannot
// silently disable security attributes.
//
// Multiple options targeting the same field apply in call order (last wins).
type CookieOption func(*http.Cookie)

// WithCookieDomain sets Cookie.Domain. Use "" (default) for host-only cookies
// (recommended). Use "example.com" or ".example.com" to share between
// subdomains (e.g., when BFF on app.example.com and AUTH on auth.example.com).
func WithCookieDomain(domain string) CookieOption {
	return func(c *http.Cookie) { c.Domain = domain }
}

// WithCookiePath overrides the default "/auth/refresh" Path. Narrowest path
// minimises attack surface — cookie is sent only when the browser requests
// that path. Wider paths (e.g., "/") expose the cookie to every endpoint.
func WithCookiePath(path string) CookieOption {
	return func(c *http.Cookie) { c.Path = path }
}

// WithCookieMaxAge overrides the default 14-day MaxAge. Duration is converted
// to integer seconds via maxAge.Seconds(); fractional seconds are truncated.
// Negative durations expire the cookie immediately — use ClearRefreshCookie
// to delete instead of passing a negative duration here.
func WithCookieMaxAge(maxAge time.Duration) CookieOption {
	return func(c *http.Cookie) { c.MaxAge = int(maxAge.Seconds()) }
}

// WithCookieSameSite overrides the default SameSite=Strict. Lax weakens CSRF
// protection — only use when SPA requires cross-site initial cookie delivery
// (typically when login completes on a different origin and redirects).
func WithCookieSameSite(mode http.SameSite) CookieOption {
	return func(c *http.Cookie) { c.SameSite = mode }
}

// SetRefreshCookie writes a Set-Cookie header with the refresh token. Defaults:
//   - Name:     "refresh_token"
//   - HttpOnly: true (always; not overridable)
//   - Secure:   true (always; not overridable)
//   - SameSite: Strict
//   - Path:     "/auth/refresh"
//   - MaxAge:   14 days
//   - Domain:   "" (host-only)
//
// HttpOnly and Secure are deliberately not exposed via CookieOption — disabling
// them is a security regression. If you need to test locally without TLS,
// provide a custom transport that strips Secure; do NOT add a SDK option.
//
// Path defaults to "/auth/refresh" so the cookie is sent only on refresh calls
// and is NOT exposed to /api/* or other endpoints (reduces XSS impact and
// avoids server-side accidental logging of the refresh token).
func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption) {
	http.SetCookie(w, newRefreshCookie(refreshToken, opts))
}

// ClearRefreshCookie writes a Set-Cookie header that instructs the browser to
// delete the refresh cookie set by SetRefreshCookie. Path and Domain are
// inherited from the same defaults (and the same opts) — they MUST match the
// original SetRefreshCookie call, otherwise RFC 6265 cookie identity does not
// match and the browser keeps the cookie.
//
// If SetRefreshCookie was called with WithCookieDomain("example.com"), call
// ClearRefreshCookie with the same WithCookieDomain.
func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption) {
	cookie := newRefreshCookie("", opts)
	cookie.MaxAge = -1
	cookie.Value = ""
	// Expires=epoch is set in addition to MaxAge=-1 for RFC 6265 §5.3 compat
	// with legacy clients that ignore MaxAge but honour an Expires in the past.
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, cookie)
}

// newRefreshCookie builds an http.Cookie with SDK defaults, applies opts, then
// re-asserts HttpOnly=true and Secure=true. This ordering is the security
// invariant: any option attempting to disable HttpOnly/Secure is silently
// overridden. The CookieOption type intentionally lacks WithCookieInsecure /
// WithCookieAllowJS — disabling these is forbidden by the SDK contract.
func newRefreshCookie(value string, opts []CookieOption) *http.Cookie {
	cookie := &http.Cookie{
		Name:     defaultRefreshCookieName,
		Value:    value,
		Path:     defaultRefreshCookiePath,
		MaxAge:   defaultRefreshCookieMaxAge,
		SameSite: http.SameSiteStrictMode,
	}
	for _, opt := range opts {
		opt(cookie)
	}
	cookie.HttpOnly = true
	cookie.Secure = true
	return cookie
}
```

**Critical notes:**
- `newRefreshCookie` returns `*http.Cookie` (stdlib expects pointer in `http.SetCookie`).
- Domain default `""` — НЕ ставим явно в struct literal (zero value already `""`). Это документировано в godoc.
- Re-application of `HttpOnly`/`Secure` после opts — единственное место security invariant'а; comment'ом помечено критично.
- НИКАКИХ опций `WithCookieValue` или `WithCookieName` — их добавление = breaking change, отложено до v0.2.0 если кто-то попросит (нет реальных use-cases в v0.1.0).

**File length:** ~110 строк (включая godoc). CS-0001 (120 char limit) — все строки укладываются.

---

### Task 2: CREATE `lib/cookies_test.go`

**Action:** CREATE
**File:** [lib/cookies_test.go](../../lib/cookies_test.go)

**Description:** Table-driven unit-тесты для `SetRefreshCookie`, `ClearRefreshCookie`, четырёх опций. UT-0001 (table-driven), UT-0011 (`happy_path` first), UT-0040 (`t.Parallel()`).

**Required Test Cases:**

`TestSetRefreshCookie`:
- [ ] `happy_path` — default opts → cookie с defaults (Name="refresh_token", Path="/auth/refresh", MaxAge=14d, SameSite=Strict, HttpOnly=true, Secure=true, Domain="", Value=token).
- [ ] `with_domain` — `WithCookieDomain(".webdelo.trade")` → Domain=".webdelo.trade", остальные defaults.
- [ ] `with_path` — `WithCookiePath("/refresh")` → Path="/refresh".
- [ ] `with_max_age` — `WithCookieMaxAge(7*24*time.Hour)` → MaxAge=7*24*60*60=604800.
- [ ] `with_same_site_lax` — `WithCookieSameSite(http.SameSiteLaxMode)` → SameSite=Lax.
- [ ] `all_options_combined` — Domain + Path + MaxAge + SameSite одновременно → все применены, Secure+HttpOnly остаются true.
- [ ] `multiple_path_options_last_wins` — `WithCookiePath("/a"), WithCookiePath("/b")` → Path="/b".
- [ ] `empty_token_value` — token="" → cookie.Value="" (caller responsibility; no SDK error).
- [ ] `negative_max_age` — `WithCookieMaxAge(-1*time.Second)` → MaxAge=-1 (immediate expire); HttpOnly/Secure remain true.

`TestClearRefreshCookie`:
- [ ] `happy_path` — default opts → MaxAge=-1, Value="", Expires=`time.Unix(0, 0)`, Name="refresh_token", Path="/auth/refresh", Domain="", SameSite=Strict, HttpOnly=true, Secure=true.
- [ ] `with_domain_preserved` — `WithCookieDomain(".webdelo.trade")` → Domain=".webdelo.trade", MaxAge=-1, Value="", Expires=epoch.
- [ ] `with_path_preserved` — `WithCookiePath("/refresh")` → Path="/refresh", MaxAge=-1, Expires=epoch.
- [ ] `with_max_age_ignored` — `WithCookieMaxAge(7*24*time.Hour)` → MaxAge=-1 (overrides opt; clearing is final), Expires=epoch.
- [ ] `expires_is_epoch_for_legacy_compat` — отдельная проверка, что Expires.IsZero() == false и Expires.Unix() == 0 (защита от регрессии: кто-нибудь может в будущем удалить строку с Expires).

`TestSetRefreshCookie_SecurityRegressionGuard`:
- [ ] `happy_path` — `Secure==true && HttpOnly==true` после default опций.
- [ ] `with_many_options_still_secure` — комбинация всех 4 опций → Secure+HttpOnly остаются true. (Это проверяет re-assertion после opts.)

**Code Pattern:**

```go
package authclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/webdelo/auth-client-go/lib"
)

func TestSetRefreshCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		token string
		opts  []authclient.CookieOption
		want  func(t *testing.T, c *http.Cookie)
	}{
		"happy_path": {
			token: "abc.def.ghi",
			opts:  nil,
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "refresh_token", c.Name)
				assert.Equal(t, "abc.def.ghi", c.Value)
				assert.Equal(t, "/auth/refresh", c.Path)
				assert.Equal(t, "", c.Domain)
				assert.Equal(t, 14*24*60*60, c.MaxAge)
				assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
				assert.True(t, c.HttpOnly)
				assert.True(t, c.Secure)
			},
		},
		"with_domain": {
			token: "tok",
			opts:  []authclient.CookieOption{authclient.WithCookieDomain(".webdelo.trade")},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, ".webdelo.trade", c.Domain)
				assert.Equal(t, "/auth/refresh", c.Path) // default unchanged
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"with_path": {
			token: "tok",
			opts:  []authclient.CookieOption{authclient.WithCookiePath("/refresh")},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "/refresh", c.Path)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"with_max_age": {
			token: "tok",
			opts:  []authclient.CookieOption{authclient.WithCookieMaxAge(7 * 24 * time.Hour)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, 7*24*60*60, c.MaxAge)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"with_same_site_lax": {
			token: "tok",
			opts:  []authclient.CookieOption{authclient.WithCookieSameSite(http.SameSiteLaxMode)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"all_options_combined": {
			token: "tok",
			opts: []authclient.CookieOption{
				authclient.WithCookieDomain(".webdelo.trade"),
				authclient.WithCookiePath("/auth"),
				authclient.WithCookieMaxAge(time.Hour),
				authclient.WithCookieSameSite(http.SameSiteLaxMode),
			},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, ".webdelo.trade", c.Domain)
				assert.Equal(t, "/auth", c.Path)
				assert.Equal(t, 3600, c.MaxAge)
				assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"multiple_path_options_last_wins": {
			token: "tok",
			opts: []authclient.CookieOption{
				authclient.WithCookiePath("/a"),
				authclient.WithCookiePath("/b"),
			},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "/b", c.Path)
			},
		},
		"empty_token_value": {
			token: "",
			opts:  nil,
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "", c.Value)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
		"negative_max_age": {
			token: "tok",
			opts:  []authclient.CookieOption{authclient.WithCookieMaxAge(-time.Second)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, -1, c.MaxAge)
				assert.True(t, c.Secure)
				assert.True(t, c.HttpOnly)
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			authclient.SetRefreshCookie(rec, tc.token, tc.opts...)

			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1, "exactly one Set-Cookie header expected")
			tc.want(t, cookies[0])
		})
	}
}

func TestClearRefreshCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		opts []authclient.CookieOption
		want func(t *testing.T, c *http.Cookie)
	}{
		"happy_path": {
			opts: nil,
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "refresh_token", c.Name)
				assert.Equal(t, "", c.Value)
				assert.Equal(t, "/auth/refresh", c.Path)
				assert.Equal(t, "", c.Domain)
				assert.Equal(t, -1, c.MaxAge)
				assert.Equal(t, time.Unix(0, 0).UTC(), c.Expires.UTC())
				assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
				assert.True(t, c.HttpOnly)
				assert.True(t, c.Secure)
			},
		},
		"with_domain_preserved": {
			opts: []authclient.CookieOption{authclient.WithCookieDomain(".webdelo.trade")},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, ".webdelo.trade", c.Domain)
				assert.Equal(t, -1, c.MaxAge)
				assert.Equal(t, "", c.Value)
				assert.Equal(t, int64(0), c.Expires.Unix())
			},
		},
		"with_path_preserved": {
			opts: []authclient.CookieOption{authclient.WithCookiePath("/refresh")},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, "/refresh", c.Path)
				assert.Equal(t, -1, c.MaxAge)
				assert.Equal(t, int64(0), c.Expires.Unix())
			},
		},
		"with_max_age_overridden_by_clear": {
			opts: []authclient.CookieOption{authclient.WithCookieMaxAge(7 * 24 * time.Hour)},
			want: func(t *testing.T, c *http.Cookie) {
				assert.Equal(t, -1, c.MaxAge, "Clear must force MaxAge=-1 regardless of opts")
				assert.Equal(t, int64(0), c.Expires.Unix())
			},
		},
		"expires_is_epoch_for_legacy_compat": {
			opts: nil,
			want: func(t *testing.T, c *http.Cookie) {
				// Regression guard: don't let anyone silently remove the Expires line.
				// MaxAge=-1 alone is not enough for RFC 6265 §5.3 legacy clients.
				assert.False(t, c.Expires.IsZero(), "Expires must be set, not zero-value")
				assert.Equal(t, int64(0), c.Expires.Unix(), "Expires must be Unix epoch")
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			authclient.ClearRefreshCookie(rec, tc.opts...)

			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1)
			tc.want(t, cookies[0])
		})
	}
}
```

**Critical notes:**
- `package authclient_test` — black-box test, как в TASK-003/004 PRPs. Импорт `authclient "github.com/webdelo/auth-client-go/lib"`. (Если в TASK-002/003 PRPs использовался `_test` suffix — следуем.)
- `httptest.NewRecorder()` + `.Result().Cookies()` — стандартный stdlib pattern для inspect'а `Set-Cookie` headers без HTTP-сервера.
- UT-0028 (no helper functions for test data) — все assertions inline в `want` closure. `want` — closure, не shared helper (UT-0032 — no logic in tests, ok потому что закрытие в case-table).
- `tc := tc` — UT-0041 variable capture for `t.Parallel()`. **NB:** в Go 1.22+ loop var capture исправлено per-iteration по умолчанию; но проект использует Go 1.24 — `tc := tc` всё равно держим для явности и совместимости с UT-0041 явным требованием.

---

### Task 3: CREATE `lib/helpers.go`

**Action:** CREATE
**File:** [lib/helpers.go](../../lib/helpers.go)

**Description:** Реализовать `IPFromRequest` (fallback chain согласно [REQ-001-ADR §A12](./REQ-001-ADR.md) + edge case из TASK spec для `SplitHostPort` error → return raw `RemoteAddr`) и `UserAgentFromRequest` (passthrough).

**Details:**
- `package authclient`.
- Импорты: `net`, `net/http`, `strings`.
- `IPFromRequest` — algorithm:
  1. `if v := r.Header.Get("X-Real-IP"); v != "" { return v }`.
  2. `if xff := r.Header.Get("X-Forwarded-For"); xff != "" { return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]) }`.
  3. `host, _, err := net.SplitHostPort(r.RemoteAddr); if err == nil { return host }`.
  4. `return r.RemoteAddr` (включает случай `RemoteAddr=""` → возвращаем `""`).
- `UserAgentFromRequest` — `return r.UserAgent()`.

**Code Pattern:**

```go
package authclient

import (
	"net"
	"net/http"
	"strings"
)

// IPFromRequest extracts the client IP from r using a best-effort fallback
// chain suitable for production deployments behind a trusted reverse proxy:
//
//  1. X-Real-IP header (if non-empty). Set by Nginx/Cloudflare/ALB to the
//     immediate client.
//  2. First entry of X-Forwarded-For (the original client per RFC 7239
//     convention). Whitespace is trimmed.
//  3. Host part of r.RemoteAddr extracted via net.SplitHostPort (handles
//     IPv4 "1.2.3.4:5678" → "1.2.3.4" and IPv6 "[::1]:5678" → "::1").
//  4. r.RemoteAddr verbatim if SplitHostPort errors (e.g., RemoteAddr lacks
//     a port, which Go's HTTP server doesn't normally produce but unit tests
//     might).
//
// Returns "" when all sources are empty.
//
// SECURITY: This function is spoofable in the absence of a trusted proxy.
// X-Real-IP and X-Forwarded-For are user-controllable. Deploy auth-client-go
// behind a reverse proxy that strips client-supplied values for these headers
// and sets them itself; otherwise a malicious client can fake their IP and
// circumvent the AUTH fingerprint check.
//
// v0.1.0 does not validate against a trusted-proxy list (planned as
// WithTrustedProxies in v0.2.0 — see REQ-001-ADR §A12).
func IPFromRequest(r *http.Request) string {
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// UserAgentFromRequest returns r.UserAgent(). Wrapper exists for API symmetry
// with IPFromRequest — callers pass IP and UA together to Login/Refresh, so a
// matching pair of helpers is clearer than mixing styles.
//
// Returns "" when the User-Agent header is absent.
func UserAgentFromRequest(r *http.Request) string {
	return r.UserAgent()
}
```

**Critical notes:**
- `strings.SplitN(xff, ",", 2)` — выделяет первый элемент за O(n) до первой запятой, не парсит весь хвост. Микро-оптимизация, но и читается ясно.
- `r.RemoteAddr` может быть `""` (например, в тестах с `httptest.NewRequest`, где RemoteAddr не выставлен) — это OK, branch 4 вернёт `""`.
- `r.UserAgent()` стандартный helper в `net/http` — внутри это `r.Header.Get("User-Agent")`. Используем именно его (не Header напрямую) — будущая совместимость с эволюцией Go stdlib.
- Godoc обязательно содержит warning «SECURITY: spoofable without trusted proxy» — это требование ADR §A12.

**File length:** ~65 строк (включая godoc). CS-0001 (120 char) — все строки укладываются.

---

### Task 4: CREATE `lib/helpers_test.go`

**Action:** CREATE
**File:** [lib/helpers_test.go](../../lib/helpers_test.go)

**Description:** Table-driven unit-тесты для `IPFromRequest` (7 cases из TASK spec) и `UserAgentFromRequest` (3 cases). UT-0001, UT-0011, UT-0040.

**Required Test Cases:**

`TestIPFromRequest`:
- [ ] `happy_path` (alias `x_real_ip`) — Header `X-Real-IP: 203.0.113.1` → `"203.0.113.1"`.
- [ ] `xff_single_ip` — Header `X-Forwarded-For: 203.0.113.1` → `"203.0.113.1"`.
- [ ] `xff_multi_ip_first_wins` — `X-Forwarded-For: 203.0.113.1, 10.0.0.1, 10.0.0.2` → `"203.0.113.1"`.
- [ ] `xff_with_whitespace_trimmed` — `X-Forwarded-For: " 203.0.113.1 "` → `"203.0.113.1"`.
- [ ] `xff_multi_with_whitespace` — `X-Forwarded-For: " 203.0.113.1 , 10.0.0.1"` → `"203.0.113.1"`.
- [ ] `remote_addr_ipv4_with_port` — RemoteAddr `192.0.2.1:54321`, no headers → `"192.0.2.1"`.
- [ ] `remote_addr_ipv6_with_port` — RemoteAddr `[::1]:54321` → `"::1"`.
- [ ] `remote_addr_without_port` — RemoteAddr `192.0.2.1` (no port — SplitHostPort error) → `"192.0.2.1"`.
- [ ] `all_sources_empty` — no headers, RemoteAddr="" → `""`.
- [ ] `x_real_ip_takes_priority_over_xff` — X-Real-IP=`203.0.113.1`, XFF=`198.51.100.1` → `"203.0.113.1"` (X-Real-IP побеждает).
- [ ] `xff_takes_priority_over_remote_addr` — XFF=`203.0.113.1`, RemoteAddr=`192.0.2.1:54321` → `"203.0.113.1"`.

`TestUserAgentFromRequest`:
- [ ] `happy_path` — User-Agent=`Mozilla/5.0 (Macintosh; Intel Mac OS X)` → identical string.
- [ ] `empty_user_agent` — no User-Agent header → `""`.
- [ ] `unicode_user_agent` — User-Agent=`MyApp/1.0 (📱 iOS)` → identical string.

**Code Pattern:**

```go
package authclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/webdelo/auth-client-go/lib"
)

func TestIPFromRequest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		xRealIP    string
		xff        string
		remoteAddr string
		want       string
	}{
		"happy_path": {
			xRealIP: "203.0.113.1",
			want:    "203.0.113.1",
		},
		"xff_single_ip": {
			xff:  "203.0.113.1",
			want: "203.0.113.1",
		},
		"xff_multi_ip_first_wins": {
			xff:  "203.0.113.1, 10.0.0.1, 10.0.0.2",
			want: "203.0.113.1",
		},
		"xff_with_whitespace_trimmed": {
			xff:  " 203.0.113.1 ",
			want: "203.0.113.1",
		},
		"xff_multi_with_whitespace": {
			xff:  " 203.0.113.1 , 10.0.0.1",
			want: "203.0.113.1",
		},
		"remote_addr_ipv4_with_port": {
			remoteAddr: "192.0.2.1:54321",
			want:       "192.0.2.1",
		},
		"remote_addr_ipv6_with_port": {
			remoteAddr: "[::1]:54321",
			want:       "::1",
		},
		"remote_addr_without_port": {
			remoteAddr: "192.0.2.1",
			want:       "192.0.2.1",
		},
		"all_sources_empty": {
			want: "",
		},
		"x_real_ip_takes_priority_over_xff": {
			xRealIP: "203.0.113.1",
			xff:     "198.51.100.1",
			want:    "203.0.113.1",
		},
		"xff_takes_priority_over_remote_addr": {
			xff:        "203.0.113.1",
			remoteAddr: "192.0.2.1:54321",
			want:       "203.0.113.1",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xRealIP != "" {
				req.Header.Set("X-Real-IP", tc.xRealIP)
			}
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}

			got := authclient.IPFromRequest(req)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUserAgentFromRequest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		ua   string
		want string
	}{
		"happy_path": {
			ua:   "Mozilla/5.0 (Macintosh; Intel Mac OS X)",
			want: "Mozilla/5.0 (Macintosh; Intel Mac OS X)",
		},
		"empty_user_agent": {
			ua:   "",
			want: "",
		},
		"unicode_user_agent": {
			ua:   "MyApp/1.0 (📱 iOS)",
			want: "MyApp/1.0 (📱 iOS)",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.ua != "" {
				req.Header.Set("User-Agent", tc.ua)
			}

			got := authclient.UserAgentFromRequest(req)

			assert.Equal(t, tc.want, got)
		})
	}
}
```

**Critical notes:**
- `httptest.NewRequest` создаёт `*http.Request` с пустым RemoteAddr, что для нас удобно: мы выставляем его вручную в `tc.remoteAddr`.
- Тесты НЕ требуют mocks — pure stdlib (UT-0023 mock.Anything ban здесь не активен — нет моков).
- UT-0028 (no helper functions for test data) — все request setup inline в subtest. Не выносим в helper (request-builder).
- UT-0030 / UT-0031 — `assert.Equal` (additional check). `require` не используется здесь — все assertions equal-checks одной строки, нет sequential dependencies между ними.

---

### Task 5: UPDATE `task_management/tasks/REQ-001-TASK-005.md`

**Action:** MODIFY
**File:** [task_management/tasks/REQ-001-TASK-005.md](../tasks/REQ-001-TASK-005.md)

**Description:** Привести TASK file в соответствие с реальным scope (post-[alternatives analysis](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md) Option A). Пункт 1 (создание `lib/context.go`) был absorbed by TASK-003. TASK file не был обновлён в моменте — это TODO от 2026-05-18, который выполняем здесь.

**Details:**
- В разделе `## WHAT`:
  - Пункт **1** (`Создать lib/context.go ...`) — добавить **в начале**: `**(✅ Перенесено в [TASK-003](./REQ-001-TASK-003.md) по [alternatives analysis 2026-05-18](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md). Пункт оставлен для исторической трассировки.)**` — текст пункта НЕ удаляем (история scope shift'а — важна для аудита), но помечаем как done elsewhere.
- В разделе `## DOD`:
  - Чекбоксы, относящиеся к `lib/context.go` (первые 3 пункта: `lib/context.go экспортирует Role...`, `UserFromContext использует comma-ok...`, `userCtxKey — unexported struct{}...`) — пометить как `[x] ✅ Done in TASK-003`.
- Никаких других изменений TASK file.

**Code Pattern (для каждой правки — minimal diff):**

```markdown
## WHAT

1. **Создать `lib/context.go`.** **(✅ Перенесено в [TASK-003](./REQ-001-TASK-003.md) по [alternatives analysis 2026-05-18](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md). Пункт оставлен для исторической трассировки — TASK-005 верифицирует существующий surface в Task 0.)** Определить:
    - `Role string` тип + константы `RoleUnknown=""`, `RoleUser="user"`, `RoleAdmin="admin"` ...
    ...
```

```markdown
## DOD (Definition of Done)

- [x] ✅ Done in TASK-003: `lib/context.go` экспортирует `Role`, константы `RoleUnknown/User/Admin`, `User` struct с методами `IsZero/IsAdmin/HasRole`, `WithUser`, `UserFromContext`.
- [x] ✅ Done in TASK-003: `UserFromContext` использует comma-ok идиому (`(User, bool)`).
- [x] ✅ Done in TASK-003: `userCtxKey` — unexported `struct{}` (EC-0056).
- [ ] `lib/cookies.go` экспортирует ...
```

**Critical notes:**
- Это документация-fix, не code change. Не влияет на `make audit`.
- Цель — будущий ревью / аудит видит trace scope shift'а без неожиданности.
- НЕ удалять текст пункта 1 — explicit «historical» mark лучше silent removal'а (легче audit'ить).

---

## Integration

### Dependencies

**No new go.mod dependencies.** Используется только stdlib: `context`, `net`, `net/http`, `strings`, `time`. Все они уже неявно подтянуты прошлыми тасками (через `client.go`, `middleware_http.go`).

### Config (если применимо)

**N/A.** TASK-005 helpers не имеют конфигурации. Defaults (Path, MaxAge, etc.) — hardcoded в `lib/cookies.go` per ADR §A9 (frozen contract).

### Database (если применимо)

**N/A.** SDK без БД.

### Auth-proto contract

**N/A.** TASK-005 не использует `auth_v1.*` напрямую — только stdlib `net/http`.

### Cross-task integration

- TASK-004 `LoginInput.ClientIP` / `Refresh(refreshToken, clientIP, userAgent)` — теперь покрываются `authclient.IPFromRequest(r)` / `authclient.UserAgentFromRequest(r)` (consumer pattern из ADR Dataflow §Login).
- TASK-004 после успешного Login возвращает `out.RefreshToken` — consumer вызывает `authclient.SetRefreshCookie(w, out.RefreshToken)`.
- TASK-004 после Logout consumer вызывает `authclient.ClearRefreshCookie(w)`.
- TASK-006 (examples) использует все три (`IPFromRequest`, `UserAgentFromRequest`, `SetRefreshCookie`) в `lib/examples/basic/main.go` и `lib/examples/http_middleware/main.go`.

### Mockery

**N/A.** Не добавляем новых интерфейсов. `mockery.yaml` не модифицируется.

---

## Testing

### Unit Tests

| Entity | Test File | Cases |
|--------|-----------|-------|
| `CookieOption`, `WithCookie*` × 4, `SetRefreshCookie` | [lib/cookies_test.go](../../lib/cookies_test.go) | TestSetRefreshCookie (9 cases), TestClearRefreshCookie (5 cases) |
| `IPFromRequest`, `UserAgentFromRequest` | [lib/helpers_test.go](../../lib/helpers_test.go) | TestIPFromRequest (11 cases), TestUserAgentFromRequest (3 cases) |

**Required Test Cases (полный inventory):**

`TestSetRefreshCookie`:
- [ ] `happy_path`
- [ ] `with_domain`
- [ ] `with_path`
- [ ] `with_max_age`
- [ ] `with_same_site_lax`
- [ ] `all_options_combined`
- [ ] `multiple_path_options_last_wins`
- [ ] `empty_token_value`
- [ ] `negative_max_age`

`TestClearRefreshCookie`:
- [ ] `happy_path`
- [ ] `with_domain_preserved`
- [ ] `with_path_preserved`
- [ ] `with_max_age_overridden_by_clear`
- [ ] `expires_is_epoch_for_legacy_compat`

`TestIPFromRequest`:
- [ ] `happy_path` (X-Real-IP)
- [ ] `xff_single_ip`
- [ ] `xff_multi_ip_first_wins`
- [ ] `xff_with_whitespace_trimmed`
- [ ] `xff_multi_with_whitespace`
- [ ] `remote_addr_ipv4_with_port`
- [ ] `remote_addr_ipv6_with_port`
- [ ] `remote_addr_without_port`
- [ ] `all_sources_empty`
- [ ] `x_real_ip_takes_priority_over_xff`
- [ ] `xff_takes_priority_over_remote_addr`

`TestUserAgentFromRequest`:
- [ ] `happy_path`
- [ ] `empty_user_agent`
- [ ] `unicode_user_agent`

### Integration Tests

**N/A.** TASK-005 helpers — pure functions против stdlib types. Integration test'ы (против реального AUTH) для них бессмысленны. Все edge cases покрываются unit-тестами.

### Mocks and Coverage

**No new mocks** — pure stdlib, нет dependencies для мокирования.

**Coverage target**: ≥90% по `lib/cookies.go` + `lib/helpers.go` (за счёт простоты функций — практически 100% достижимо).

---

## Validation

### Build & Lint

```bash
# 1. Format / vet / lint
make fmt
make vet
make lint                       # golangci-lint

# 2. Tests
make test                       # unit tests, all packages
make test-race                  # with race detector
make coverage                   # coverage report; check lib/cookies.go and lib/helpers.go ≥90%

# 3. Full audit
make audit                      # vet + lint + test

# 4. Mod hygiene (NO new deps expected)
make tidy                       # idempotent — go.mod не должен меняться
git diff go.mod go.sum          # must be empty
```

### Granular checks

```bash
# Specific test invocations
go test -race -run TestSetRefreshCookie -count=2 ./lib/...
go test -race -run TestClearRefreshCookie -count=2 ./lib/...
go test -race -run TestIPFromRequest -count=2 ./lib/...
go test -race -run TestUserAgentFromRequest -count=2 ./lib/...

# Coverage for new files specifically
go test -coverprofile=.temp/coverage.out ./lib/...
go tool cover -func=.temp/coverage.out | grep -E "(cookies|helpers).go"
# Expected: each function ≥90% statement coverage.

# Public API snapshot — should include new symbols
go doc github.com/webdelo/auth-client-go/lib | grep -E "^(func|var|type)" | grep -E "(Cookie|IPFromRequest|UserAgentFromRequest)"
# Expected lines:
#   type CookieOption func(*http.Cookie)
#   func WithCookieDomain(domain string) CookieOption
#   func WithCookiePath(path string) CookieOption
#   func WithCookieMaxAge(maxAge time.Duration) CookieOption
#   func WithCookieSameSite(mode http.SameSite) CookieOption
#   func SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)
#   func ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)
#   func IPFromRequest(r *http.Request) string
#   func UserAgentFromRequest(r *http.Request) string
```

### Manual Checks

1. **Security regression invariant:** прочитать `lib/cookies.go` и убедиться, что `cookie.HttpOnly = true` и `cookie.Secure = true` стоят **ПОСЛЕ** цикла `for _, opt := range opts`. Если опция не отключает их, но порядок применения нарушен — это потенциальная уязвимость в будущем при добавлении опций.
2. **Godoc rendering:** `go doc github.com/webdelo/auth-client-go/lib SetRefreshCookie` показывает full godoc включая security warning (HttpOnly/Secure not overridable).
3. **Godoc IPFromRequest spoofing warning:** `go doc github.com/webdelo/auth-client-go/lib IPFromRequest` содержит секцию `SECURITY:` с предупреждением о trusted proxy.
4. **TASK-005 spec обновлён:** `grep "✅ Done in TASK-003" task_management/tasks/REQ-001-TASK-005.md` возвращает ≥3 совпадения.
5. **No removed code:** `git diff lib/` показывает только новые файлы (`cookies.go`, `cookies_test.go`, `helpers.go`, `helpers_test.go`) — никаких изменений в файлах TASK-002/003/004.

---

## DOD (Definition of Done)

- [ ] **Task 0 verify:** `lib/context.go` существует от TASK-003 с полной public surface (User/Role/IsZero/IsAdmin/HasRole/WithUser/UserFromContext/userCtxKey-unexported).
- [ ] `lib/cookies.go` создан, `package authclient`, экспортирует `CookieOption` + 4 `WithCookie*` + `SetRefreshCookie` + `ClearRefreshCookie`. Godoc у каждого exported идентификатора, начинается с имени (ES-0080/0081).
- [ ] `SetRefreshCookie` defaults: Name="refresh_token", Path="/auth/refresh", MaxAge=14d, SameSite=Strict, HttpOnly=true, Secure=true, Domain="".
- [ ] `HttpOnly` и `Secure` re-applied **после** opts (security regression guard). Нет опций `WithCookieInsecure` / `WithCookieAllowJS` или подобных.
- [ ] `ClearRefreshCookie`: MaxAge=-1, Value="", `Expires=time.Unix(0, 0)` (RFC 6265 §5.3 legacy-browser compat), остальные attribute из defaults+opts (symmetric с `SetRefreshCookie`).
- [ ] `lib/helpers.go` создан, экспортирует `IPFromRequest` и `UserAgentFromRequest`. Godoc на каждом + явный `SECURITY:` warning у `IPFromRequest`.
- [ ] `IPFromRequest` fallback chain покрывает все 4 ветки: X-Real-IP / XFF[0] / SplitHostPort host / raw RemoteAddr.
- [ ] `lib/cookies_test.go` + `lib/helpers_test.go` созданы, table-driven (UT-0001), первый кейс `happy_path` (UT-0011), все `Test*` и subtest имеют `t.Parallel()` (UT-0040).
- [ ] Все 28 test cases (9 + 5 + 11 + 3) реализованы и проходят локально через `make test-race`.
- [ ] Coverage `lib/cookies.go` + `lib/helpers.go` ≥ 90% (`make coverage`).
- [ ] `make audit` (vet + lint + test) проходит без ошибок и без `// nolint` директив (ES-0171).
- [ ] `go doc github.com/webdelo/auth-client-go/lib` показывает все 9 новых символов без TODO в комментариях.
- [ ] `task_management/tasks/REQ-001-TASK-005.md` обновлён: пункт 1 WHAT помечен как (absorbed by TASK-003), 3 относящихся DOD-чекбокса помечены [x] ✅ Done in TASK-003.
- [ ] `go.mod` / `go.sum` не изменились (никаких новых deps).
- [ ] `git diff lib/` показывает только добавленные файлы (никаких modifications в файлах TASK-002/003/004).

---

## Score: 9/10

| Критерий | Оценка | Заметка |
|----------|--------|---------|
| Necessary context included | 10/10 | ADR §A8/A9/A12 + alternatives analysis + TASK-002/003/004 PRP cross-refs + 8 documentation URLs (stdlib, RFC, OWASP) |
| Validation gates executable | 10/10 | 4 make-команды + 6 granular `go test` invocations + 3 manual checks |
| References existing patterns | 8/10 | Mirror'ит functional options pattern из TASK-002 (`Option func(*Client)` ↔ `CookieOption func(*http.Cookie)`); test layout идентичен TASK-003/004 (черно-белый `_test` package, httptest.NewRecorder/NewRequest, table-driven). Sibling-репах прецедентов мало — TASK self-contained. |
| Clear implementation path | 10/10 | 5 tasks с deterministic actions; pseudocode complete для каждой; нет архитектурных open questions |
| API design rigor | 9/10 | ADR §A9/§A12 + API surface §API уже зафиксировали полный contract; PRP формализует security regression invariant (re-assert HttpOnly/Secure после opts) — это extension над ADR |
| Risks and mitigations | 9/10 | 14 gotchas покрывают: scope shift, RFC 6265 cookie identity, security regression, MaxAge int/Duration, option order, SameSite/redirect, XFF spoofing, XFF whitespace, SplitHostPort error modes, IPv6, UA defaults, refresh-token escaping, Path="" semantics, test parallelism |
| Test coverage rigor | 10/10 | 27 unit test cases — все 11 edge cases для IP (из TASK spec + 4 priority-tests) + 9 cookie set cases + 4 clear cases + 3 UA cases. UT-0001/0011/0040 строго соблюдаются. |

**Минус 1 балл:** code-research отчёт ещё не опубликован к моменту PRP draft'а — может содержать дополнительные паттерны godoc-style из sibling-репах. Не блокер: PRP self-sufficient от спецификации и ADR. После публикации — supplementary references могут быть добавлены к Analysis Files без перерезанья PRP.

---

## Риски и нюансы

| # | Риск | Митигация |
|---|------|-----------|
| R1 | Если TASK-003 не выполнен или выполнен частично, TASK-005 не имеет `lib/context.go` для верификации в Task 0 | Task 0 — explicit verification с `grep -q` + exit code 1. Failure ⇒ STOP, эскалировать TASK-003 первым. |
| R2 | CookieOption order semantics — пользователь может предполагать «первое выигрывает» | Документировано в godoc `CookieOption`: «Multiple options targeting the same field apply in call order (last wins)». |
| R3 | Negative MaxAge через `WithCookieMaxAge(-time.Second)` — путаница с ClearRefreshCookie | Godoc у `WithCookieMaxAge`: «Use ClearRefreshCookie to delete, not negative MaxAge». Тест `negative_max_age` явно валидирует поведение. |
| R4 | `Path="/auth/refresh"` — если consumer выставит другой path на login flow, ClearRefreshCookie с defaults не удалит | Документировано в godoc `ClearRefreshCookie`: «Path and Domain MUST match the original SetRefreshCookie call». Symmetric `WithCookie*` опции работают одинаково — caller сам проверяет соответствие. |
| R5 | SameSite=Strict + первый внешний POST /auth/login: refresh-cookie не отправится | Документировано в godoc `SetRefreshCookie` (G6 in gotchas): «Use WithCookieSameSite(http.SameSiteLaxMode) for SSR / cross-site initial login». |
| R6 | `X-Real-IP` / `X-Forwarded-For` spoofing без trusted proxy → fake fingerprint в AUTH | Документировано как **SECURITY:** warning в godoc `IPFromRequest`. WithTrustedProxies — additive v0.2.0 (ADR §A12). |
| R7 | `r.RemoteAddr` без порта в production маловероятен (Go HTTP server всегда даёт port), но в integration tests `httptest.NewRequest` может выставить любую строку | Branch 4 (SplitHostPort error → raw RemoteAddr) корректно покрывает edge case. Тест `remote_addr_without_port` явный. |
| R8 | XFF formatting: client → proxy_1 → proxy_2 → server. `XFF: client, proxy_1, proxy_2` (последний trusted = proxy_2). Если первая proxy untrusted, она может вписать spoofed IP первым | Это inherent ограничение IPFromRequest для v0.1.0. WithTrustedProxies в v0.2.0 будет фильтровать список snake-from-right (drop trusted proxies до первого untrusted-or-eof, использовать тот IP). В v0.1.0 — best-effort + docs. |
| R9 | Test file `package authclient_test` (black-box). Если future-TASK захочет тестировать internal helpers — придётся менять подход | Black-box покрывает все public API. Internal `newRefreshCookie` не тестируется напрямую — это OK, его поведение проверяется через `SetRefreshCookie`/`ClearRefreshCookie`. |
| R10 | Update TASK-005 spec (Task 5) — minor doc fix, но может вызвать confusion при ревью «почему PRP меняет TASK?» | Task 5 содержит явное reasoning + ссылку на alternatives analysis. Ревьюер видит, что это документация-fix, не scope creep. |

---

## Open Questions (не блокеры для TASK-005)

| OQ | Вопрос | Решение / отложение |
|----|--------|---------------------|
| OQ-01 | Нужен ли `RefreshFromRequest(ctx, w, r)` convenience-helper уже в v0.1.0? Объединил бы `r.Cookie("refresh_token")` + `Refresh` + `SetRefreshCookie` в одну строку. | Отложено в v0.2.0 (ADR OQ-02). v0.1.0 — consumer пишет 3 строки явно (понятнее, прозрачнее). |
| OQ-02 | `WithCookieName(name string) CookieOption` — кастомизация имени cookie? | Отложено. v0.1.0 — Name hardcoded `"refresh_token"`. Кастомное имя в multi-tenant setup'ах потенциально нужно — но это additive в v0.2.0. |
| OQ-03 | `Cookie.Partitioned` (CHIPS — Cookies Having Independent Partitioned State)? Новая security-фича браузеров, актуальна для 3p cookies. | Out of scope v0.1.0. Refresh cookie — first-party (`SameSite=Strict`), Partitioned не критичен. v0.2.0+ при появлении use-case'а 3p iframe. |

---

## Ссылки

- Repo: https://github.com/webdelo/auth-client-go
- ADR: [REQ-001-ADR.md](./REQ-001-ADR.md) (§A8, §A9, §A12, §R2)
- Alternatives analysis: [2026-05-18-REQ-001-TASK-003-alternatives-analysis.md](../session_analysis/2026-05-18-REQ-001-TASK-003-alternatives-analysis.md)
- Prior PRPs: [TASK-002](./REQ-001-TASK-002-PRP.md), [TASK-003](./REQ-001-TASK-003-PRP.md), [TASK-004](./REQ-001-TASK-004-PRP.md)
- Standards: [.claude/guides/rules_index.yaml](../../.claude/guides/rules_index.yaml), [coding_scope.md](../../.temp/rules/REQ-001-TASK-005/coding_scope.md), [test_scope.md](../../.temp/rules/REQ-001-TASK-005/test_scope.md)
- Stdlib: [net/http#Cookie](https://pkg.go.dev/net/http#Cookie), [net/http#SetCookie](https://pkg.go.dev/net/http#SetCookie), [net#SplitHostPort](https://pkg.go.dev/net#SplitHostPort)
- RFCs: [RFC 6265 (Cookies)](https://datatracker.ietf.org/doc/html/rfc6265), [RFC 7239 (Forwarded)](https://datatracker.ietf.org/doc/html/rfc7239)
- Security: [OWASP A07:2025 Auth Failures](https://owasp.org/Top10/2025/A07_2025-Authentication_Failures/), [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
