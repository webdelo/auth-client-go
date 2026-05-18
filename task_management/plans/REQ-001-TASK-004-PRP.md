# REQ-001-TASK-004-PRP: Auth operations — Login / Refresh (singleflight) / Logout / LogoutAll / ChangePassword / GetCurrentUser

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-001-TASK-004-PRP |
| Parent Request | [REQ-001](../tasks/REQ-001.md) |
| Parent Task | [REQ-001-TASK-004](../tasks/REQ-001-TASK-004.md) |
| Created | 2026-05-18 |
| Updated | 2026-05-18 |
| PRP Score | 9/10 |

---

## Goal

### Summary

Реализовать auth-flow методы SDK (`Login`, `Refresh`, `Logout`, `LogoutAll`, `ChangePassword`, `GetCurrentUser`) поверх готового каркаса из TASK-002 (Client, errors, Logger). Центральный нетривиальный кусок — `Refresh` с защитой от refresh-race через `golang.org/x/sync/singleflight`: две параллельные вкладки с одним `refresh_token` отправят в AUTH ровно один RPC (см. [REQ-001-ADR §A3 + §R8](./REQ-001-ADR.md)). Остальные методы — типизированные обёртки над gRPC с маппингом ошибок через готовый `mapGRPCError`.

### Why

- **Login-flow для consumer-сервисов (BFF).** Без `Login`/`Refresh`/`Logout` SDK не закрывает первичный сценарий — авторизация пользователя в BFF.
- **Refresh race — токсичная зона.** Если SDK просто проксирует параллельные refresh'ы, AUTH детектит reuse → убивает family → пользователь выкидывается из всех сессий. Singleflight в одном процессе SDK покрывает ~80% реальных race'ов (одна replica BFF). Multi-replica — accepted limitation v0.1.0 ([REQ-001-ADR §R3](./REQ-001-ADR.md)).
- **Email появляется только через Login / GetCurrentUser.** `ValidateAccessToken` (TASK-003) не возвращает email из proto-контракта (ADR §R10). `GetCurrentUser` нужен для UI «my account» страниц.
- **ChangePassword + LogoutAll** — критичные user-actions, входят в minimal viable login-flow.

### What

После TASK-004 потребитель SDK может:

```go
// 1) Логин — обмен email+password на токены.
out, err := client.Login(ctx, authclient.LoginInput{
    Email:     "user@example.com",
    Password:  "secret",
    ClientIP:  authclient.IPFromRequest(r), // TASK-005
    UserAgent: authclient.UserAgentFromRequest(r),
})
// out.AccessToken, out.RefreshToken, out.AccessExpiresIn, out.RefreshExpiresIn, out.User

// 2) Refresh — обновление пары токенов (rotation). Race-safe.
out, err := client.Refresh(ctx, refreshTokenFromCookie, clientIP, userAgent)
// Две параллельные goroutine с одним refreshTokenFromCookie → ровно один RPC в AUTH.

// 3) Logout — инвалидация одной сессии.
err := client.Logout(ctx, refreshTokenFromCookie)

// 4) LogoutAll — invalidate все сессии (bump token_version).
err := client.LogoutAll(ctx, accessToken)

// 5) ChangePassword — поменять пароль, AUTH сам invalidate'ит family.
err := client.ChangePassword(ctx, accessToken, currentPassword, newPassword)

// 6) GetCurrentUser — текущий пользователь с email.
user, err := client.GetCurrentUser(ctx, accessToken)
// user.UUID, user.Email, user.Role — все три поля заполнены.
```

Все методы:
- Принимают `context.Context` первым параметром (EC-0050).
- Оборачивают ctx в `context.WithTimeout` со специфичным таймаутом (`cfg.LoginTimeout` / `cfg.RefreshTimeout` / `cfg.ValidateTimeout`).
- Возвращают типизированные ошибки через `mapGRPCError` (TASK-002).
- Логируют success через `Logger.Info` с `user_uuid` (без email/IP/токенов).

### Success Criteria

- [ ] `lib/internal/refresh/group.go` экспортирует `Group.Do(key, fn)` с автоматическим `Forget(key)` после Do — никаких stale-результатов на повторных вызовах.
- [ ] `*Client` имеет приватное поле `sf *refresh.Group`, проинициализированное в `New`.
- [ ] `lib/auth.go` экспортирует `LoginInput`, `LoginOutput`, `RefreshOutput` и методы `Login`/`Refresh`/`Logout`/`LogoutAll`/`ChangePassword`/`GetCurrentUser` на `*Client`.
- [ ] Unit-test `TestClient_Refresh_SingleFlight`: 2 goroutine с одним `refresh_token` → mock `AuthServiceClient.RefreshToken` вызван **ровно 1 раз** (через `Once()` ограничение mockery).
- [ ] `errors.Is(err, ErrInvalidToken|ErrRateLimited|ErrUnavailable|...)` корректно для каждого error-path всех 6 методов.
- [ ] `AccessExpiresIn`/`RefreshExpiresIn` вычислены через `time.Until(time.Unix(seconds, 0))`; обнулённые timestamps (`0`) → `0 * time.Second`.
- [ ] Integration-тесты `TestLogin_Happy`, `TestRefresh_HappyAndRotation`, `TestRefresh_Race` (10 goroutine), `TestLogout`, `TestChangePassword`, `TestGetCurrentUser_HasEmail` проходят через `make test-integration` против реального AUTH.
- [ ] `go test -race -run TestClient_Refresh_SingleFlight -count=10 ./lib/...` — все 10 прогонов зелёные.
- [ ] Unit coverage по `lib/auth.go` + `lib/internal/refresh/` ≥ 80% (`make coverage`).
- [ ] Логи не содержат raw-токенов/паролей/email — mockery `Logger.EXPECT().Info(...)` с явными fields проверяет это.
- [ ] `make audit` (vet + lint + test) проходит без ошибок.

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | ES-0050, ES-0060, ES-0061, ES-0062, ES-0063, ES-0064, ES-0080, ES-0081, ES-0120, ES-0140, ES-0171, ES-0172, ES-0173 |
| Effective Concurrency | [effective_concurrency.md](../../.claude/guides/golang/effective_concurrency.md) | EC-0010, EC-0012, EC-0013, EC-0050, EC-0053, EC-0055, EC-0090, EC-0091, EC-0092, EC-0100, EC-0101, EC-0102, EC-0120 (singleflight pattern) |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | CS-0001, CS-0002, CS-0010, CS-0021, CS-0040, CS-0060, CS-0070, CS-0080, CS-0090, CS-0111, CS-0116 (LoginInput/Output — DTO) |
| Unit Tests | [unit_tests_standards.md](../../.claude/guides/golang/unit_tests_standards.md) | UT-0001, UT-0010, UT-0011, UT-0012, UT-0020, UT-0021, UT-0022, UT-0023, UT-0024, UT-0026, UT-0027, UT-0028, UT-0029, UT-0030, UT-0040, UT-0041 |
| Integration Tests Standards | [integration_tests_standards.md](../../.claude/guides/golang/integration_tests_standards.md) | Build tag `//go:build integration`, gate через `RUN_INTEGRATION_TESTS=true`, изоляция через docker-compose (TASK-001) |

### Concurrency Patterns

- [x] **Singleflight** — `golang.org/x/sync/singleflight.Group` для дедупликации Refresh-вызовов по `hex(sha256(refresh_token))`.
- [ ] Worker Pool
- [ ] Semaphore
- [ ] errgroup
- [ ] Fan-Out/Fan-In

### Critical Rules — точечный список

- **ES-0060** Handle errors explicitly: каждый возврат `mapGRPCError(err)` обязателен; nil-error не игнорируется.
- **ES-0061** Wrap errors with context: для нестандартных кейсов (nil User в LoginResponse, type-assert miss singleflight return value) — `fmt.Errorf("authclient: <ctx>: %w", ErrUnavailable)`.
- **ES-0062** Sentinel errors: все error-path возвращают `errors.Is`-совместимые ошибки из `lib/errors.go` (TASK-002).
- **ES-0063** No panic in production: не паникуем сами. Recover'имся от паники В callback'е singleflight (защита от паники mock'а или транзитивной зависимости).
- **ES-0064** Recover only for crash prevention: `defer recover()` ТОЛЬКО внутри `Refresh` fn для singleflight — паника одной goroutine не должна валить процесс или ронять всех ожидающих refresh'а.
- **ES-0080/0081** Doc comments precede each exported identifier, начинаются с имени.
- **ES-0171** No `nolint` annotations — никаких exception'ов.
- **ES-0172** Use full object names: log fields `"user_uuid"`, `"method"`, не `"id"`/`"m"`.
- **EC-0050** Context первым параметром во всех 6 публичных методах.
- **EC-0053** `context.WithTimeout` с типовым лимитом из `cfg`.
- **EC-0055** Всегда `defer cancel()` после `context.WithTimeout`.
- **EC-0090** Race-test через `make test-race` обязательно зелёный.
- **EC-0091** Capture loop vars в horoutines (`i := i` внутри `for`).
- **EC-0120** Singleflight pattern с обязательным `Forget` после `Do` (отдельная гарантия — внутри `refresh.Group.Do`).
- **UT-0021** Expecter API mockery (`.EXPECT()`), не `.On()`.
- **UT-0022** Explicit call count: `.Once()` / `.Times(n)` на каждом mock-вызове.
- **UT-0023** ПОЛНЫЙ ЗАПРЕТ `mock.Anything` / `mock.AnythingOfType` — используем `mock.MatchedBy(func)` для ctx и proto-сообщений.
- **UT-0024** Свежие mock'и в каждом subtest (через `testMocks` struct).
- **UT-0040** `t.Parallel()` на каждом `TestXxx` и subtest.

---

## Analysis Files

- **Parent REQ + ADR**: [REQ-001.md](../tasks/REQ-001.md), [REQ-001-ADR.md](./REQ-001-ADR.md) (§API, §A3, §A7, §A12, §R3, §R8, §R10, §R11)
- **Prior PRPs (контракт-источники для TASK-004)**:
  - [REQ-001-TASK-002-PRP.md](./REQ-001-TASK-002-PRP.md) — `Client`, `Config`, `errors.go`, `mapGRPCError`, `Logger`, `AuthError`, `RetryAfter`
  - [REQ-001-TASK-003-PRP.md](./REQ-001-TASK-003-PRP.md) — `User`, `Role`, `hashToken`, `cacheAdapter`, `mockery.yaml` для `auth_v1.AuthServiceClient` + `Cache` + `Logger`
  - [REQ-001-TASK-001-PRP.md](./REQ-001-TASK-001-PRP.md) — integration test infra (`make env-up`, `RUN_INTEGRATION_TESTS=true`, build tag `//go:build integration`)
- **Proto-контракт**: `~/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go` (типы `LoginRequest/Response`, `RefreshTokenRequest/Response`, `LogoutRequest`, `LogoutAllRequest`, `ChangePasswordRequest`, `GetCurrentUserRequest/Response`, `User`)
- **Code research (supplementary, in progress)**: `.memory_bank/agent_task_results/2026-05-18_code-research_REQ-001-TASK-004.md` — паттерны singleflight в sibling-репах + race-test patterns. Включить ссылку после публикации; не блокер для PRP.

---

## Context

### Documentation & References

```yaml
# === Critical ===

- file: task_management/tasks/REQ-001-TASK-004.md
  why: Source TASK — WHAT/WHY/DOD/акценты. Все 7 шагов и 6 интеграционных кейсов — обязательны.

- file: task_management/plans/REQ-001-ADR.md
  section: "§A3 Singleflight для Refresh"
  critical: |
    Forget(key) ОБЯЗАТЕЛЕН после Do — иначе следующий Do с тем же ключом дождётся stale result.
    Singleflight НЕ отменяет fn при отмене caller'ского ctx — это OK для Refresh (хотим довыполнить).
    Cross-instance limitation: работает в пределах одного процесса. Multi-replica BFF — accepted v0.1.0.

- file: task_management/plans/REQ-001-ADR.md
  section: "§A7 Typed errors"
  critical: |
    mapGRPCError (TASK-002) ходит по errors.Unwrap для извлечения gRPC status (status.FromError не unwraps).
    ResourceExhausted → *AuthError{Err: ErrRateLimited, RetryAfter}. errors.Is(err, ErrRateLimited) == true.

- file: task_management/plans/REQ-001-ADR.md
  section: "§A12 Fingerprint binding"
  critical: |
    ClientIP / UserAgent — это IP/UA РЕАЛЬНОГО клиента (через IPFromRequest), а НЕ BFF.
    AUTH fingerprint = SHA-256(UA + first 3 octets IP). Refresh, выданный для 1.2.3.55, сработает на 1.2.3.99 (та же /24) — за NAT это OK.
    SDK НЕ извлекает IP сам — это делает consumer через TASK-005 helpers.

- file: task_management/plans/REQ-001-TASK-002-PRP.md
  section: "Task 5: lib/client.go"
  why: Client struct fields, Config fields (LoginTimeout=5s, RefreshTimeout=5s, ValidateTimeout=2s по defaults).
  critical: Client struct уже содержит cfg/grpcConn/authClient/logger/failMode/closeOnce/bgCtx. TASK-004 ДОБАВЛЯЕТ только `sf *refresh.Group`.

- file: task_management/plans/REQ-001-TASK-002-PRP.md
  section: "Task 3: lib/errors.go"
  why: mapGRPCError, AuthError, RetryAfter, ErrInvalidToken, ErrExpired, ErrBlocked, ErrRateLimited, ErrUnavailable.

- file: task_management/plans/REQ-001-TASK-003-PRP.md
  section: "Task 4: lib/validate.go (hashToken)"
  critical: |
    hashToken(token string) string — приватная функция, hex.EncodeToString(sha256.Sum256([]byte(token))).
    Уже создана в lib/validate.go (TASK-003). TASK-004 переиспользует — НИЧЕГО НЕ ДУБЛИРУЕТ.

- file: task_management/plans/REQ-001-TASK-003-PRP.md
  section: "Task 12: mockery.yaml"
  critical: |
    mockery.yaml в TASK-003 уже зарегистрирован для auth_v1.AuthServiceClient + Cache + Logger.
    TASK-004 НЕ модифицирует mockery.yaml. Достаточно `make generate-mocks` для пересборки моков (если новых интерфейсов не добавляли — мок AuthServiceClient уже покрывает RefreshToken/Login/Logout/LogoutAll/ChangePassword/GetCurrentUser).

# === Proto contract ===

- file: ~/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go
  section: "LoginRequest (lines 24-31), LoginResponse (lines 92-101)"
  critical: |
    LoginRequest fields: Email, Password, Ip, UserAgent.
    LoginResponse fields: AccessToken, RefreshToken, AccessTokenExpiresAt (int64 unix-seconds), RefreshTokenExpiresAt (int64), User (*User).
    User fields: Uuid, Email, Role (string), IsBlocked, CreatedAt, LastLoginAt.

- file: ~/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go
  section: "RefreshTokenRequest (lines 336-342), RefreshTokenResponse (lines 396-404)"
  critical: |
    RefreshTokenRequest: RefreshToken, Ip, UserAgent.
    RefreshTokenResponse: AccessToken, RefreshToken (ROTATED — новый), AccessTokenExpiresAt, RefreshTokenExpiresAt.
    Заметь: RefreshTokenResponse НЕ возвращает User. Логировать без user_uuid.

- file: ~/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service.pb.go
  section: "LogoutRequest (168-173), LogoutResponse (212), LogoutAllRequest (248-254), LogoutAllResponse (292), ChangePasswordRequest (576-582), ChangePasswordResponse (636), GetCurrentUserRequest (848-853), GetCurrentUserResponse (892-897)"
  critical: |
    Logout/LogoutAll/ChangePassword возвращают Empty-like response (только state/unknownFields). _, err := ... достаточно.
    GetCurrentUserResponse.User — pointer. Может быть nil → guard'и.

- file: ~/Development/TRADING/bots/auth-proto/go/auth_v1/auth_service_grpc.pb.go
  section: "AuthServiceClient interface (line 40)"
  why: |
    Это интерфейс для mock'ирования. Methods: Login, Logout, LogoutAll, RefreshToken, ValidateToken, ChangePassword, GetCurrentUser, ListUsers, CreateUser, UpdateUserRole, BlockUser, ListSessions, RevokeSession.
    TASK-004 использует первые 6 (без Validate — TASK-003).

# === Singleflight ===

- url: https://pkg.go.dev/golang.org/x/sync/singleflight
  section: "Group.Do"
  critical: |
    Signature: Do(key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool)
    shared=true если ХОТЯ БЫ один другой caller дождался результата (хорошо для метрик).
    fn запускается ТОЛЬКО для leader'а; followers блокируются до возврата fn и получают тот же (v, err).
    КРИТИЧНО: НЕТ context-respect — Do(key, fn) НЕ отменит fn при отмене ctx caller'а. Это by design.

- url: https://www.openmymind.net/Forgotten-Method-On-Sync-Singleflight-Group/
  why: |
    Известный pitfall: без Forget(key) сразу после Do — followers, попавшие в Do ПОСЛЕ завершения fn, всё равно
    получат stale result (потому что singleflight хранит результат до явного Forget или до момента, когда
    leader-call вернёт управление followers'ам). С Forget сразу после Do это окно закрыто:
      - while fn runs: followers block on the SAME call
      - fn returns: leader unblocks first, ALL waiters unblock with same result, key is forgotten
      - next Do with the same key: starts a fresh fn (новый RPC)
    Если Forget не вызвать — следующий Do увидит "уже есть result" и вернёт его БЕЗ запуска fn.

# === gRPC / proto-gen + mockery ===

- url: https://pkg.go.dev/google.golang.org/protobuf/proto#Equal
  why: |
    proto-сообщения имеют unexported state (protoimpl.MessageState) — struct equality (== / reflect.DeepEqual)
    может вести себя непредсказуемо. Для матчинга proto-аргументов в моках использовать proto.Equal или getter-comparison.

- url: https://vektra.github.io/mockery/v2/installation/
  section: "Expecter API"
  why: |
    mockery v3.7.0 (см. Makefile) — generates EXPECT()-style fluent API.
    Pattern: mockAuth.EXPECT().Login(matchCtx, matchReq).Return(resp, nil).Once()
    Для context — `mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })`.
    Для proto-req — `mock.MatchedBy(func(req *auth_v1.LoginRequest) bool { return req.GetEmail() == ... })`.
    ЗАПРЕТ mock.Anything / mock.AnythingOfType (UT-0023) — обходим через MatchedBy.
```

### Current Structure

```
lib/
├── client.go                        # (TASK-002) Client/Config/New/Close; будет МОДИФИЦИРОВАН (+sf field)
├── errors.go                        # (TASK-002) sentinel + mapGRPCError + AuthError
├── logger.go                        # (TASK-002) Logger interface + NoopLogger
├── context.go                       # (TASK-003) User/Role/WithUser/UserFromContext
├── validate.go                      # (TASK-003) ValidateAccessToken + hashToken (переиспользуется TASK-004)
├── middleware_http.go               # (TASK-003)
├── interceptor_grpc.go              # (TASK-003)
├── internal/
│   └── cache/
│       └── memory.go                # (TASK-003) MemoryCache
├── mocks/
│   ├── auth_service_client_mock.go  # (TASK-003) сгенерён mockery — содержит EXPECT().Login/RefreshToken/...
│   ├── cache_mock.go                # (TASK-003)
│   └── logger_mock.go               # (TASK-003)
└── examples/basic/main.go           # (TASK-002 stub)

dev/deployments/
├── compose.test.yaml                # (TASK-001) AUTH + Postgres
├── test.env                         # (TASK-001) RUN_INTEGRATION_TESTS, AUTH_ENDPOINT
└── extra.mk                         # (TASK-001) env-up, env-down, test-integration

mockery.yaml                          # (TASK-003) NB: содержит auth_v1.AuthServiceClient + Cache + Logger
```

### Desired Structure (after TASK-004)

```
lib/
├── client.go                                # MODIFY: +sf *refresh.Group field + init in New
├── auth.go                                  # NEW: LoginInput/Output, RefreshOutput, 6 методов на *Client
├── auth_test.go                             # NEW: unit-тесты 6 методов + expiresIn + log-content guard
├── auth_integration_test.go                 # NEW: build tag integration; TestLogin/Refresh/Race/Logout/...
├── errors.go                                # unchanged
├── logger.go                                # unchanged
├── context.go                               # unchanged
├── validate.go                              # unchanged (hashToken переиспользуется)
├── middleware_http.go                       # unchanged
├── interceptor_grpc.go                      # unchanged
├── internal/
│   ├── cache/
│   │   └── memory.go                        # unchanged
│   └── refresh/
│       ├── group.go                         # NEW: Group + Do(key, fn) с Forget
│       └── group_test.go                    # NEW: unit-тесты Group
├── mocks/
│   ├── auth_service_client_mock.go          # REGENERATE (через make generate-mocks) — нет API-изменений
│   ├── cache_mock.go                        # unchanged
│   └── logger_mock.go                       # unchanged
└── examples/basic/main.go                   # unchanged

tests/
└── integration_helpers_test.go              # NEW (optional): helpers seedTestUser/loginAndGetTokens
                                              # ИЛИ inline в auth_integration_test.go — выбор в Task 7.
```

### Gotchas

| Category | Issue | Solution |
|----------|-------|----------|
| **Singleflight** | `Forget` placement — без него next Do получает stale | `refresh.Group.Do` ВСЕГДА вызывает `sf.Forget(key)` сразу после `sf.Do`. Тест `TestGroup_Do_KeyForgotten` явно проверяет, что повторный Do с тем же ключом перевызывает fn. |
| **Singleflight** | `Do` не отменяет fn при cancellation caller'ского ctx | Принимаем (ADR §A3). Refresh должен довыполниться: иначе AUTH видит «refresh использован, но новый не выдан» → на следующий refresh family detection убьёт сессию. Документируется в godoc Refresh. |
| **Singleflight** | Тип возврата `Do(key, fn) (any, error)` против `(any, error, bool)` | TASK-004 spec item 1 фиксирует **`(any, error)`** 2-return. `shared` bool из `singleflight.Group.Do` игнорируется (`_`). Если в v0.2.0+ потребуется shared для метрик — добавим `Stats()` метод. Тестируемость dedup'а — через mock call counter (`.Once()`), не через shared. |
| **Singleflight key** | Использование raw refresh-токена как ключа — попадание в heap dump | Ключ — `hashToken(refreshToken)` = `hex(sha256(token))` (TASK-003 helper). Raw-токен в singleflight не уходит. |
| **Panic guard** | Паника в fn (например, в mock или в transitive lib) валит leader-goroutine, followers зависают | `defer recover()` ВНУТРИ fn в `Refresh` методе. Recover → wrapped `ErrUnavailable` + `Logger.Error`. Это явный пункт TASK item 5: «На panic в Refresh fn — Error». |
| **Proto getter nil-safety** | `LoginResponse.User` может прийти nil от AUTH (теоретически) → `nil.GetUuid()` паника? | Getter генерирует с nil-check'ом (см. pb.go line 162): `func (x *LoginResponse) GetUser() *User { if x != nil { return x.User }; return nil }`. Но дальше `userPb.GetUuid()` на nil — getter тоже nil-safe (возвращает ""). Логически: если AUTH вернул LoginResponse без User — это инвариант-нарушение. Возвращаем `out.User = User{}` без error (consumer увидит пустой UUID и поймёт). Альтернатива — fmt.Errorf wrap. **Решение:** не возвращать error, просто `out.User = User{}` для устойчивости. |
| **Proto getter nil-safety** | `GetCurrentUserResponse.User` тоже может быть nil | В отличие от Login, для GetCurrentUser empty user в ответе — баг сервера (метод обязан вернуть пользователя или error). Здесь возвращаем `fmt.Errorf("authclient: get_current_user: empty user: %w", ErrUnavailable)`. |
| **Timestamps** | Unix-seconds могут быть 0 (defaults) или в прошлом | Helper `expiresIn(seconds int64) time.Duration`: `if seconds <= 0 { return 0 }`; иначе `time.Until(time.Unix(seconds, 0))`. Negative duration (просроченные timestamps) пропускаем как есть — caller решит. |
| **Timeout choice** | Какой таймаут для Logout/LogoutAll/ChangePassword/GetCurrentUser? | По TASK item 4 — fallback на `ValidateTimeout`. **Уточнение:** ChangePassword делает bcrypt-verify текущего пароля (~100ms+ на стороне AUTH) → используем `LoginTimeout` (5s по умолчанию). Login → `LoginTimeout`. Refresh / Logout → `RefreshTimeout` (refresh-token-операции, симметричны). LogoutAll / GetCurrentUser → `ValidateTimeout` (быстрые ops). |
| **Mock arg-matching** | `mock.Anything` / `mock.AnythingOfType` ЗАПРЕЩЕНЫ (UT-0023) | `mock.MatchedBy(fn)` для ctx и proto-сообщений. ctx-matcher: `mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil })`. proto-matcher: getter-сравнение. |
| **Mock proto equality** | `reflect.DeepEqual(*pb.LoginRequest, ...)` может сломаться из-за unexported state | Используем getter-сравнение (`req.GetEmail() == "..."`). НЕ структурное равенство. |
| **Race-test flakiness** | Два goroutine должны попасть в `sf.Do` параллельно, иначе тест проверяет sequential | Pattern: `chan` rendezvous внутри mock fn. Mock получает signal "fn started" → тест ждёт другую goroutine на barrier → закрывает release-chan. `.Once()` на mock гарантирует, что fn вызвана ровно один раз. См. Task 6 — TestClient_Refresh_SingleFlight. |
| **Race-test и `Once()`** | mockery `.Once()` падает с panic при втором вызове — стандартное поведение, тест не должен полагаться на эту panic как на assertion | После `wg.Wait()` явно вызываем `mockAuth.AssertExpectations(t)` ИЛИ полагаемся на t.Cleanup mockery (он сам проверит Once). Mockery v3 это делает автоматически через testify-mock `AssertExpectations` в Cleanup, если mock создан через `mocks.NewAuthServiceClient(t)` (передан *testing.T). |
| **Integration test: User seeding** | TestLogin требует существующего пользователя в AUTH. AUTH не имеет seed-механизма из коробки. | Создаём helper `seedTestUser(t, email, password)` в `lib/auth_integration_test.go`. Реализация — прямой INSERT в AUTH Postgres через `database/sql` + `pgx` ИЛИ через AUTH `CreateUser` admin RPC (требует admin user, который тоже надо seed'ить). **Решение PRP:** прямой SQL insert с bcrypt-хэшированием пароля (golang.org/x/crypto/bcrypt). Это minimum viable; вынесение в test-utility — после v0.1.0. Бcrypt-cost = 4 (test cost — fast). См. Task 7. |
| **Integration test: cross-test isolation** | Tests могут оставлять users/sessions в AUTH БД | Каждый test создаёт пользователя с UNIQUE email через `t.Name()` + `uuid.NewString()` суффикс. После теста — best-effort cleanup через `t.Cleanup` (DELETE FROM users WHERE email LIKE 'test_%'). Не блокер для DOD — параллельные тесты не конфликтуют благодаря уникальности. |
| **Logger no-tokens guard** | Как утверждать, что fields НЕ содержат token-pattern строк? | Используем mockery `mocks.Logger.EXPECT().Info(matchCtx, "authclient: login success", "user_uuid", "test-uuid-123").Once()` — fields в expectation указаны явно. Если в продакшен-коде кто-то добавит лишний field (например refresh_token) — mock-вызов не совпадёт с expectation, тест упадёт. Это структурная гарантия, не runtime-проверка содержимого. |
| **Logger context** | TASK-002 Logger: `Info(ctx context.Context, msg string, fields ...any)`. fields — variadic, key-value pairs (slog-style). | В коде: `c.logger.Info(ctx, "authclient: login success", "user_uuid", out.User.UUID)`. Один pair. |
| **Refresh context lifecycle** | `callCtx, cancel := context.WithTimeout(ctx, ...)`; `cancel()` через defer; но fn выполняется в leader goroutine — что, если leader's ctx отменился? | callCtx — derived от ctx leader'а. Если leader cancel'ится — callCtx cancel'ится — fn вернёт `context.Canceled` через gRPC → `mapGRPCError` вернёт `ErrUnavailable`. Followers получат тот же error. Это OK — symmetric behavior. |
| **Singleflight ctx miss between leader/followers** | Leader получает `ctx_leader.WithTimeout(LoginTimeout)`; follower тоже создаёт свой callCtx, но в fn попадает только leader's callCtx | Followers НЕ запускают fn — они ждут результата leader'а. callCtx follower'а просто defer cancel'ится при выходе из Refresh. Никакой утечки. |
| **Default Logger usage** | После `New(...)` `c.logger` гарантированно non-nil (ES-0143 — TASK-002 ставит NoopLogger дефолтом). | В TASK-004 коде вызываем `c.logger.Info(...)` без nil-check. |
| **gRPC retry vs Refresh** | TASK-002 в `retryServiceConfig` указал retry ТОЛЬКО для `ValidateToken`. RefreshToken НЕ ретраится grpc-go. | OK — TASK-004 не дополнительно ничего не делает. Документируем в godoc Refresh: «caller MUST handle ErrUnavailable explicitly — SDK does NOT retry refresh» (ADR §R11). |

---

## Implementation

### Data Models

```go
// lib/internal/refresh/group.go

// Group is a thin wrapper over singleflight.Group with mandatory key-forget
// after Do. The Forget step is what distinguishes correct refresh-singleflight
// from the naive form: without Forget, callers arriving AFTER the underlying
// fn returned would get the stale (already-rotated) result for the same key.
type Group struct {
    sf singleflight.Group
}

// Do invokes fn at most once for concurrent calls with the same key.
// Returns the leader's (v, err). The underlying singleflight `shared` bool
// is intentionally NOT exposed in v0.1.0 — caller does not need it; the
// task contract specifies `(any, error)` 2-return (TASK-004 spec item 1).
func (g *Group) Do(key string, fn func() (any, error)) (any, error)
```

```go
// lib/auth.go

// LoginInput is the input for Login. ClientIP/UserAgent forward to AUTH for
// fingerprint binding — they MUST come from the actual end-user HTTP request,
// not the BFF process metadata. Use IPFromRequest / UserAgentFromRequest
// helpers (TASK-005).
type LoginInput struct {
    Email     string
    Password  string
    ClientIP  string
    UserAgent string
}

// LoginOutput is the result of Login. Expiries are converted from absolute
// Unix-seconds (proto) into durations relative to time.Now().
type LoginOutput struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresIn  time.Duration
    RefreshExpiresIn time.Duration
    User             User
}

// RefreshOutput is the result of Refresh. Note: RefreshToken here is the
// ROTATED token (AUTH issues a new refresh token on every successful refresh).
// The OLD refresh token is invalidated server-side.
type RefreshOutput struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresIn  time.Duration
    RefreshExpiresIn time.Duration
}
```

```go
// lib/client.go — UPDATED Client struct

type Client struct {
    cfg         Config
    grpcConn    *grpc.ClientConn
    authClient  auth_v1.AuthServiceClient
    logger      Logger
    failMode    FailMode
    openMethods map[string]struct{}
    cache       Cache             // wired in TASK-003
    sf          *refresh.Group    // NEW in TASK-004
    bgCtx       context.Context
    cancelBG    context.CancelFunc
    closeOnce   sync.Once
    closeErr    error
}
```

---

### Task 0 (pre-requisite): Verify dependencies and prior-task state

**Action:** VERIFY

**Description:** TASK-004 импортирует `golang.org/x/sync/singleflight`. Согласно TASK-001 PRP, `golang.org/x/sync` уже добавлен как direct dependency (blank import в integration test). TASK-002/003 уже подняли каркас Client+errors+Logger+validate+mocks.

**Details:**
- Прочитать `go.mod`. Проверить наличие `golang.org/x/sync` в `require` block (direct, без `// indirect`).
- Проверить существование файлов: `lib/client.go`, `lib/errors.go`, `lib/logger.go`, `lib/validate.go`, `lib/mocks/auth_service_client_mock.go`, `lib/mocks/logger_mock.go`.
- Если TASK-001/002/003 не выполнены — STOP, эскалировать (TASK-004 не должен компилироваться без них). Запустить их сначала.

**Commands:**
```bash
grep -E "golang.org/x/sync" go.mod || (echo "MISSING golang.org/x/sync — run TASK-001 first" && exit 1)
test -f lib/client.go || (echo "TASK-002 not done" && exit 1)
test -f lib/validate.go || (echo "TASK-003 not done" && exit 1)
test -f lib/mocks/auth_service_client_mock.go || (echo "TASK-003 mocks missing" && exit 1)
```

---

### Task 1: CREATE `lib/internal/refresh/group.go`

**Action:** CREATE
**File:** [lib/internal/refresh/group.go](../../lib/internal/refresh/group.go)

**Description:** Тонкая обёртка над `golang.org/x/sync/singleflight.Group`. Метод `Do(key, fn)` вызывает `sf.Do(key, fn)` и **обязательно** `sf.Forget(key)` сразу после — это закрывает окно stale-result'а (ADR §A3/§R8).

**Details:**
- Пакет `package refresh` (lower-case, single word — ES-0010).
- Импорт только `golang.org/x/sync/singleflight`.
- `type Group struct { sf singleflight.Group }` — `sf` value-receiver, zero-value ready (EC-0030).
- Метод `Do(key string, fn func() (any, error)) (any, error)`:
  - `v, err, _ := g.sf.Do(key, fn)` — `shared` bool из singleflight игнорируется (TASK-004 spec item 1 явно требует `(any, error)` 2-return).
  - `g.sf.Forget(key)` — НЕ через defer, потому что Forget'нуть надо ПОСЛЕ Do (когда followers уже разблокированы и получили результат). defer тоже отработает после return — это equivalent, но синтаксически читаемее без defer.
  - return `v, err`.
- Godoc-комментарий precede declaration, начинается с `Group` / `Do` (ES-0080/0081).
- Compile-time check: НЕ нужен (нет interface implementation).

**Code Pattern:**

```go
// Package refresh provides a thin wrapper over singleflight.Group with
// mandatory key-forget after each Do call.
//
// Why a wrapper? Plain singleflight.Group keeps the last result indefinitely
// until the caller invokes Forget. For refresh-token rotation this is wrong:
// after fn returns, the result is single-use (the refresh token has been
// rotated — calling Do again with the SAME key must hit AUTH for a fresh
// rotation). With Forget right after Do, that invariant holds.
//
// See REQ-001-ADR.md §A3 + §R8.
package refresh

import "golang.org/x/sync/singleflight"

// Group deduplicates concurrent calls with the same key. After fn completes
// (whether successfully or with error), the key is forgotten so subsequent
// callers run a fresh fn.
//
// Zero value is ready to use.
type Group struct {
    sf singleflight.Group
}

// Do invokes fn at most once per concurrent key. Concurrent callers with
// the same key block on the leader's call; all waiters receive the leader's
// (value, error) pair when fn returns.
//
// Note: singleflight does NOT respect ctx cancellation — if a follower's
// context cancels, the follower still waits until the leader's fn returns.
// For Refresh this is intentional (the rotation must complete to avoid AUTH
// family detection on the next attempt).
//
// The underlying singleflight.Group.Do returns a `shared` bool indicating
// whether more than one caller received the same result. v0.1.0 does not
// surface it (TASK-004 contract is `(any, error)`); if needed for metrics,
// expose via a future Stats() method.
func (g *Group) Do(key string, fn func() (any, error)) (any, error) {
    v, err, _ := g.sf.Do(key, fn)
    g.sf.Forget(key)
    return v, err
}
```

**Critical notes:**
- `Group` value type, не pointer-required. Embedding в `Client` через `*refresh.Group` (для ясности владения) — приемлемо, но и поле-значение тоже работает. **Выбор PRP:** `sf *refresh.Group` для consistency с другим полями pointer-кода (логично — caller владеет lifecycle).
- Forget стоит ПОСЛЕ Do, не в defer — defer работает после Do return, но синтаксически плотнее в одном виде; defer добавляет 1 extra func-call в frame. Microscopic perf не важно, но ясно — оставляем без defer.

---

### Task 2: CREATE `lib/internal/refresh/group_test.go`

**Action:** CREATE
**File:** [lib/internal/refresh/group_test.go](../../lib/internal/refresh/group_test.go)

**Description:** Unit-тесты для `refresh.Group`. Table-driven где применимо (UT-0001), параллельные (UT-0040).

**Required Test Cases:**

**`TestGroup_Do`:**
- [ ] `happy_path` — Do вызывает fn один раз, возвращает (v, nil).
- [ ] `concurrent_dedup_single_call` — N=10 goroutines с одним key → fn вызвана 1 раз, все 10 получили тот же результат. Используем `atomic.Int32` счётчик в fn.
- [ ] `different_keys_independent` — 2 goroutines с разными keys → fn вызвана 2 раза.
- [ ] `key_forgotten_next_call_reruns` — Sequential Do(key, fn) → Do(key, fn) → fn вызвана 2 раза (доказательство Forget работает; критично для §A3/§R8).
- [ ] `fn_error_propagates` — fn возвращает error → Do возвращает тот же error для всех waiters.
- [ ] `fn_panic_propagates` — fn panic'ует. singleflight НЕ recover'ится сам — panic пройдёт через Do. Это намеренное поведение: recover делает caller (`Client.Refresh`) ровно в нужном месте.

**Code Pattern (фрагмент):**

```go
package refresh

import (
    "errors"
    "sync"
    "sync/atomic"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestGroup_Do_HappyPath(t *testing.T) {
    t.Parallel()

    g := &Group{}
    v, err := g.Do("key", func() (any, error) {
        return "result", nil
    })

    require.NoError(t, err)
    assert.Equal(t, "result", v)
}

func TestGroup_Do_ConcurrentDedupSingleCall(t *testing.T) {
    t.Parallel()

    g := &Group{}
    var calls atomic.Int32
    release := make(chan struct{})
    started := make(chan struct{}, 1)

    fn := func() (any, error) {
        calls.Add(1)
        // Signal leader started — but only once (buffered=1 + select default).
        select {
        case started <- struct{}{}:
        default:
        }
        <-release
        return "result", nil
    }

    const N = 10
    var wg sync.WaitGroup
    results := make([]any, N)
    errs := make([]error, N)

    for i := 0; i < N; i++ {
        i := i
        wg.Add(1)
        go func() {
            defer wg.Done()
            v, err := g.Do("same-key", fn)
            results[i] = v
            errs[i] = err
        }()
    }

    // Wait for leader to enter fn — give followers time to converge.
    <-started
    // Slight pause for followers to reach sf.Do. Singleflight rendezvous on
    // map lookup; in practice <1ms. We give 20ms for slow CI.
    // (No deterministic barrier in singleflight — best-effort.)
    // Using runtime.Gosched in a loop is brittle; small sleep is the standard
    // pattern in golang.org/x/sync own tests.
    // We compensate via -count=10 in CI (DOD requirement).
    time.Sleep(20 * time.Millisecond)

    close(release)
    wg.Wait()

    assert.Equal(t, int32(1), calls.Load(), "fn must run exactly once")
    for i := 0; i < N; i++ {
        require.NoError(t, errs[i])
        assert.Equal(t, "result", results[i])
    }
}

func TestGroup_Do_KeyForgottenNextCallReruns(t *testing.T) {
    t.Parallel()

    g := &Group{}
    var calls atomic.Int32
    fn := func() (any, error) {
        calls.Add(1)
        return "result", nil
    }

    _, _ = g.Do("key", fn)
    _, _ = g.Do("key", fn)

    assert.Equal(t, int32(2), calls.Load(), "Forget must rerun fn on next Do")
}

func TestGroup_Do_FnErrorPropagates(t *testing.T) {
    t.Parallel()

    g := &Group{}
    sentinel := errors.New("boom")
    fn := func() (any, error) {
        return nil, sentinel
    }

    _, err := g.Do("key", fn)

    require.Error(t, err)
    assert.True(t, errors.Is(err, sentinel))
}
```

**Critical notes:**
- `time.Sleep` для rendezvous — best-effort, не deterministic. Альтернатива: `synctest.Wait()` (Go 1.24+, UT-0071) — но singleflight использует mutex'ы, не time-based primitives, так что synctest здесь не помогает. Пакет `golang.org/x/sync/singleflight` сам использует small sleep в своих тестах. DOD требует `-count=10` — это статистическая защита от flake.
- `TestGroup_Do_FnPanicPropagates` — можно реализовать через `assert.Panics` или `defer recover()` в тесте. **Решение:** упоминаем в test cases, но из-за того, что singleflight в panic'е может leak'ать (зависит от внутренней реализации), маркируем тест как `t.Skip("singleflight panic-handling — caller responsibility")`. Достаточно того, что `Client.Refresh` recover'ится сам.

---

### Task 3: MODIFY `lib/client.go` — add `sf *refresh.Group` field

**Action:** MODIFY
**File:** [lib/client.go](../../lib/client.go)

**Description:** Добавить приватное поле `sf *refresh.Group` в `Client` struct + инициализировать в `New(...)`.

**Details:**
- Импорт: `"github.com/webdelo/auth-client-go/lib/internal/refresh"`.
- Поле `sf *refresh.Group` после `cache Cache`, перед `bgCtx context.Context` (логически — runtime-state блок).
- В `New(...)` после `c := &Client{...}` (или внутри struct literal) выставить `sf: &refresh.Group{}`.
- НЕ создавать `refresh.Group` в `applyDefaults` — это поле не зависит от Config.

**Code Pattern (diff-фрагмент):**

```go
// lib/client.go (после блока импортов TASK-002/003)

import (
    // ... existing imports ...
    "github.com/webdelo/auth-client-go/lib/internal/refresh"
)

type Client struct {
    cfg         Config
    grpcConn    *grpc.ClientConn
    authClient  auth_v1.AuthServiceClient
    logger      Logger
    failMode    FailMode
    openMethods map[string]struct{}
    cache       Cache
    sf          *refresh.Group       // NEW: singleflight for Refresh dedup

    bgCtx     context.Context
    cancelBG  context.CancelFunc
    closeOnce sync.Once
    closeErr  error
}

func New(cfg Config, opts ...Option) (*Client, error) {
    // ... existing validation, applyDefaults, dial ...

    c := &Client{
        cfg:         cfg,
        grpcConn:    conn,
        authClient:  auth_v1.NewAuthServiceClient(conn),
        logger:      cfg.Logger,
        failMode:    cfg.FailMode,
        openMethods: defaultOpenMethods(),
        sf:          &refresh.Group{},  // NEW
        cancelBG:    cancel,
        bgCtx:       bgCtx,
    }

    for _, opt := range opts {
        opt(c)
    }

    // ... cache wiring (TASK-003) ...

    return c, nil
}
```

**Critical notes:**
- `&refresh.Group{}` — zero-value ready. Не требует cleanup. `Close()` ничего не делает с `sf`.
- Импортируется `github.com/webdelo/auth-client-go/lib/internal/refresh` — `internal` обеспечивает, что только подпакеты `lib/` могут импортировать. Consumer'ы (BFF) НЕ имеют доступа.

---

### Task 4: CREATE `lib/auth.go`

**Action:** CREATE
**File:** [lib/auth.go](../../lib/auth.go)

**Description:** Публичные типы `LoginInput`/`LoginOutput`/`RefreshOutput` + 6 методов на `*Client`. Каждый метод: ctx-timeout, gRPC-вызов, mapGRPCError, лог success с `user_uuid` (если есть). Refresh — через `c.sf.Do(hashToken(refreshToken), ...)` с recover в fn.

**Details:**
- `package authclient`.
- Импорты: `context`, `fmt`, `time`, `auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"`.
- Хелпер `expiresIn(unix int64) time.Duration` — приватный.
- Все методы: receiver `*Client`, ctx первым параметром (EC-0050).
- Доки godoc на каждом exported (ES-0080/0081).

**Code Pattern:**

```go
package authclient

import (
    "context"
    "fmt"
    "time"

    auth_v1 "github.com/webdelo/auth-proto/go/auth_v1"
)

// LoginInput contains the credentials and fingerprint metadata for Login.
//
// ClientIP and UserAgent are forwarded to AUTH for refresh-token fingerprint
// binding (AUTH computes fingerprint = SHA-256(UserAgent + first 3 octets of
// IP)). The caller MUST extract these from the actual end-user HTTP request
// (use IPFromRequest / UserAgentFromRequest helpers), NOT from BFF's own
// connection metadata — otherwise AUTH binds the refresh to BFF's IP and the
// user gets logged out on the next refresh.
type LoginInput struct {
    Email     string
    Password  string
    ClientIP  string
    UserAgent string
}

// LoginOutput is the result of Login. Expiries are converted from absolute
// Unix-seconds (proto contract) to durations relative to time.Now().
//
// User.Email is populated by Login (unlike ValidateAccessToken which leaves
// it empty due to proto contract).
type LoginOutput struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresIn  time.Duration
    RefreshExpiresIn time.Duration
    User             User
}

// RefreshOutput is the result of Refresh. RefreshToken here is the ROTATED
// token — the OLD token passed to Refresh is server-side invalidated.
// Caller MUST replace the stored refresh token with this new one (and the
// refresh cookie via SetRefreshCookie from TASK-005).
type RefreshOutput struct {
    AccessToken      string
    RefreshToken     string
    AccessExpiresIn  time.Duration
    RefreshExpiresIn time.Duration
}

// expiresIn converts an absolute Unix-seconds timestamp into a duration
// relative to time.Now. Returns 0 for the proto-default 0 (missing field).
// May return negative duration for timestamps in the past — caller's choice
// whether to treat as error.
func expiresIn(unixSeconds int64) time.Duration {
    if unixSeconds <= 0 {
        return 0
    }
    return time.Until(time.Unix(unixSeconds, 0))
}

// Login authenticates the user with email+password and returns access+refresh
// tokens plus user info. Forwards ClientIP/UserAgent to AUTH for fingerprint
// binding.
//
// Wraps ctx with cfg.LoginTimeout (default 5s) before the gRPC call.
//
// Returns:
//   - ErrInvalidToken for invalid credentials.
//   - ErrRateLimited when AUTH rate-limits the caller (call RetryAfter to
//     inspect the hint).
//   - ErrUnavailable when AUTH is unreachable or returns an unexpected code.
//
// Logs success at Info level with user_uuid (no email/IP/tokens).
func (c *Client) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
    callCtx, cancel := context.WithTimeout(ctx, c.cfg.LoginTimeout)
    defer cancel()

    resp, err := c.authClient.Login(callCtx, &auth_v1.LoginRequest{
        Email:     in.Email,
        Password:  in.Password,
        Ip:        in.ClientIP,
        UserAgent: in.UserAgent,
    })
    if err != nil {
        mapped := mapGRPCError(err)
        c.logUnavailableIfNeeded(ctx, "login", mapped)
        return LoginOutput{}, mapped
    }

    out := LoginOutput{
        AccessToken:      resp.GetAccessToken(),
        RefreshToken:     resp.GetRefreshToken(),
        AccessExpiresIn:  expiresIn(resp.GetAccessTokenExpiresAt()),
        RefreshExpiresIn: expiresIn(resp.GetRefreshTokenExpiresAt()),
    }
    if u := resp.GetUser(); u != nil {
        out.User = User{
            UUID:  u.GetUuid(),
            Role:  Role(u.GetRole()),
            Email: u.GetEmail(),
        }
    }

    c.logger.Info(ctx, "authclient: login success", "user_uuid", out.User.UUID)
    return out, nil
}

// Refresh exchanges the given refresh token for a new pair of tokens. Refresh
// is the rotation point: AUTH issues a new refresh_token and invalidates the
// old one. RefreshOutput.RefreshToken is the NEW token — store it and discard
// the previous.
//
// Concurrent Refresh calls with the same refreshToken within this Client are
// deduplicated via internal singleflight: ONLY ONE RefreshToken RPC reaches
// AUTH, the other waiters share the result. This prevents AUTH family
// detection from killing the session when two browser tabs simultaneously
// refresh.
//
// LIMITATION: dedup is per-process. Multi-replica BFF deployments have no
// cross-instance protection in v0.1.0 (REQ-001-ADR §R3). If you operate
// multi-replica and observe family-detection logouts under load, consider
// sticky sessions or a v0.2.0+ distributed lock.
//
// clientIP / userAgent MUST come from the end-user request (see IPFromRequest
// in TASK-005). AUTH binds the refresh to fingerprint(UA, /24 IP) — passing
// BFF's own IP will break the next refresh after IP/UA changes.
//
// Wraps ctx with cfg.RefreshTimeout (default 5s). gRPC retry policy excludes
// RefreshToken (REQ-001-ADR §R11) — failures are returned to caller without
// retry to avoid burning the refresh chain.
//
// Returns:
//   - ErrInvalidToken when the refresh token is unknown, malformed, or
//     reused (AUTH family detection).
//   - ErrUnavailable when AUTH is unreachable.
//   - ErrRateLimited when rate-limited.
func (c *Client) Refresh(ctx context.Context, refreshToken, clientIP, userAgent string) (RefreshOutput, error) {
    callCtx, cancel := context.WithTimeout(ctx, c.cfg.RefreshTimeout)
    defer cancel()

    key := hashToken(refreshToken)
    v, err := c.sf.Do(key, func() (result any, fnErr error) {
        // Defensive recover (ES-0064 — recover only for crash prevention).
        // singleflight does NOT recover panics — without this, a panic from
        // the gRPC client (or a buggy interceptor) would kill the leader
        // goroutine and leave followers blocked.
        defer func() {
            if r := recover(); r != nil {
                fnErr = fmt.Errorf("authclient: refresh fn panic: %v: %w", r, ErrUnavailable)
                c.logger.Error(ctx, "authclient: refresh fn panic", fnErr, "recover", fmt.Sprint(r))
            }
        }()

        return c.authClient.RefreshToken(callCtx, &auth_v1.RefreshTokenRequest{
            RefreshToken: refreshToken,
            Ip:           clientIP,
            UserAgent:    userAgent,
        })
    })
    if err != nil {
        mapped := mapGRPCError(err)
        c.logUnavailableIfNeeded(ctx, "refresh", mapped)
        return RefreshOutput{}, mapped
    }

    resp, ok := v.(*auth_v1.RefreshTokenResponse)
    if !ok {
        return RefreshOutput{}, fmt.Errorf(
            "authclient: refresh unexpected response type %T: %w",
            v, ErrUnavailable,
        )
    }

    out := RefreshOutput{
        AccessToken:      resp.GetAccessToken(),
        RefreshToken:     resp.GetRefreshToken(),
        AccessExpiresIn:  expiresIn(resp.GetAccessTokenExpiresAt()),
        RefreshExpiresIn: expiresIn(resp.GetRefreshTokenExpiresAt()),
    }
    c.logger.Info(ctx, "authclient: refresh success")
    return out, nil
}

// Logout invalidates the given refresh token at AUTH. The corresponding
// access token remains valid until its natural expiry (ValidateAccessToken
// cache lag — up to cfg.ValidateCacheTTL after server-side invalidation).
// To invalidate ALL of a user's sessions, use LogoutAll.
//
// Wraps ctx with cfg.RefreshTimeout (logout operates on refresh tokens).
//
// Returns ErrInvalidToken if refreshToken is unknown, ErrUnavailable if AUTH
// is unreachable. Logout is idempotent at AUTH — a second logout of an
// already-invalid token returns success or ErrInvalidToken depending on
// server impl; do NOT rely on idempotence.
func (c *Client) Logout(ctx context.Context, refreshToken string) error {
    callCtx, cancel := context.WithTimeout(ctx, c.cfg.RefreshTimeout)
    defer cancel()

    _, err := c.authClient.Logout(callCtx, &auth_v1.LogoutRequest{
        RefreshToken: refreshToken,
    })
    if err != nil {
        mapped := mapGRPCError(err)
        c.logUnavailableIfNeeded(ctx, "logout", mapped)
        return mapped
    }

    c.logger.Info(ctx, "authclient: logout success")
    return nil
}

// LogoutAll invalidates ALL of the user's sessions by bumping the
// token_version at AUTH. All access tokens issued before the bump start
// returning is_valid=false from ValidateToken; the SDK cache picks this up
// after cfg.ValidateCacheTTL (default 30s).
//
// Wraps ctx with cfg.ValidateTimeout (this is a fast DB write at AUTH).
//
// Returns ErrInvalidToken if accessToken is invalid, ErrUnavailable if AUTH
// is down.
func (c *Client) LogoutAll(ctx context.Context, accessToken string) error {
    callCtx, cancel := context.WithTimeout(ctx, c.cfg.ValidateTimeout)
    defer cancel()

    _, err := c.authClient.LogoutAll(callCtx, &auth_v1.LogoutAllRequest{
        AccessToken: accessToken,
    })
    if err != nil {
        mapped := mapGRPCError(err)
        c.logUnavailableIfNeeded(ctx, "logout_all", mapped)
        return mapped
    }

    c.logger.Info(ctx, "authclient: logout_all success")
    return nil
}

// ChangePassword changes the user's password at AUTH. AUTH verifies
// currentPassword (bcrypt — ~100ms) and, on success, bumps token_version,
// invalidating all sessions. The caller MUST re-Login after a successful
// ChangePassword.
//
// Wraps ctx with cfg.LoginTimeout (bcrypt verification — same order as
// Login).
//
// Returns:
//   - ErrInvalidToken if accessToken is invalid (AUTH returns Unauthenticated).
//   - ErrUnavailable (wrapped) for wrong-current-password / weak-new-password
//     (AUTH returns InvalidArgument; v0.1.0 mapGRPCError does NOT distinguish
//     this from real Unavailable — caller must inspect err.Error() to tell
//     apart). This is documented limitation; v0.2.0 will add an explicit
//     ErrInvalidPassword sentinel. See REQ-001 risks.
//   - ErrUnavailable if AUTH is unreachable.
func (c *Client) ChangePassword(ctx context.Context, accessToken, currentPassword, newPassword string) error {
    callCtx, cancel := context.WithTimeout(ctx, c.cfg.LoginTimeout)
    defer cancel()

    _, err := c.authClient.ChangePassword(callCtx, &auth_v1.ChangePasswordRequest{
        AccessToken:     accessToken,
        CurrentPassword: currentPassword,
        NewPassword:     newPassword,
    })
    if err != nil {
        mapped := mapGRPCError(err)
        c.logUnavailableIfNeeded(ctx, "change_password", mapped)
        return mapped
    }

    c.logger.Info(ctx, "authclient: change_password success")
    return nil
}

// GetCurrentUser returns the user identified by accessToken, INCLUDING email
// (unlike ValidateAccessToken which leaves email empty due to the
// ValidateTokenResponse proto contract — see REQ-001-ADR §R10).
//
// Use this for UI "my account" pages where email display is needed.
//
// Wraps ctx with cfg.ValidateTimeout (simple lookup at AUTH).
func (c *Client) GetCurrentUser(ctx context.Context, accessToken string) (User, error) {
    callCtx, cancel := context.WithTimeout(ctx, c.cfg.ValidateTimeout)
    defer cancel()

    resp, err := c.authClient.GetCurrentUser(callCtx, &auth_v1.GetCurrentUserRequest{
        AccessToken: accessToken,
    })
    if err != nil {
        mapped := mapGRPCError(err)
        c.logUnavailableIfNeeded(ctx, "get_current_user", mapped)
        return User{}, mapped
    }

    u := resp.GetUser()
    if u == nil {
        return User{}, fmt.Errorf("authclient: get_current_user empty user: %w", ErrUnavailable)
    }

    user := User{
        UUID:  u.GetUuid(),
        Role:  Role(u.GetRole()),
        Email: u.GetEmail(),
    }
    c.logger.Info(ctx, "authclient: get_current_user success", "user_uuid", user.UUID)
    return user, nil
}

// logUnavailableIfNeeded emits a Warn log when the mapped error is
// ErrUnavailable. Other errors (invalid/expired/blocked/rate_limited) are
// not logged at SDK level — they are business outcomes for the caller.
//
// This is a private helper to avoid copy-pasting `if errors.Is(... ) { Warn }`
// in every method.
func (c *Client) logUnavailableIfNeeded(ctx context.Context, method string, err error) {
    if errors.Is(err, ErrUnavailable) {
        c.logger.Warn(ctx, "authclient: AUTH unavailable", "method", method)
    }
}
```

**Note on imports:** the snippet above uses `errors.Is` in `logUnavailableIfNeeded` — add `"errors"` to imports.

**Critical notes:**
- `LoginResponse.User == nil` после успешного gRPC-вызова — это серверная аномалия. Возвращаем `User{}` без error (LoginOutput valid, просто без user-info). Это снисходительно, но безопасно: caller увидит `out.User.UUID == ""` и поймёт.
- `GetCurrentUserResponse.User == nil` — это контрактное нарушение, error-возврат уместен (этот метод именно за user-info вызывают).
- `logUnavailableIfNeeded` логирует Warn только для UNAVAILABLE — остальные ошибки (invalid token, rate limited) — это business outcome, caller сам решает логировать или нет. Это соответствует пункту 5 TASK: «На error UNAVAILABLE — Warn».
- Refresh fn использует `callCtx` (с RefreshTimeout). Если waiter'ский ctx отменился — он всё равно ждёт результат leader'а (singleflight design). Документировано в godoc.
- `mapped := mapGRPCError(err)` — храним в локальной переменной, чтобы потом передать в `logUnavailableIfNeeded` (избегаем второго вызова mapGRPCError, который дороже).
- **`ChangePassword` wrong-current-password gotcha** — proto-документ `ChangePassword` возвращает `codes.InvalidArgument` для wrong-current-password или слабого new-password. Текущая таблица `mapGRPCError` (TASK-002) НЕ имеет ветки для `InvalidArgument` → попадает в default → `fmt.Errorf("authclient: unexpected grpc code InvalidArgument: %w", ErrUnavailable)`. Consumer видит `errors.Is(err, ErrUnavailable) == true` для wrong-password — семантически некорректно (это не недоступность сервиса, а business error). **Принято для v0.1.0:** документировать в godoc `ChangePassword` как known limitation. Резолюция в v0.2.0: расширить `mapGRPCError` маппингом `codes.InvalidArgument` → `ErrInvalidToken` (additive). PRP item: добавить в Open Questions / Risks ниже.

---

### Task 5: CREATE `lib/auth_test.go`

**Action:** CREATE
**File:** [lib/auth_test.go](../../lib/auth_test.go)

**Description:** Unit-тесты для всех 6 публичных методов + `expiresIn`. Mockery-based (UT-0020/0021), table-driven где применимо (UT-0001), параллельные (UT-0040), explicit call counts (UT-0022).

**Pre-requisite:** `make generate-mocks` после TASK-003 уже сгенерил `lib/mocks/AuthServiceClient` и `lib/mocks/Logger`. TASK-004 не модифицирует `mockery.yaml` — структура mock'ов покрывает все необходимые методы.

**Required Test Cases:**

**`TestExpiresIn`** (table-driven):
- [ ] `happy_path` — `expiresIn(time.Now().Add(time.Hour).Unix()) ≈ 1h ± slack`.
- [ ] `zero_returns_zero`.
- [ ] `negative_returns_zero`.
- [ ] `past_timestamp_returns_negative`.

**`TestClient_Login`** (table-driven):
- [ ] `happy_path` — mock возвращает полный `LoginResponse{AccessToken, RefreshToken, AccessTokenExpiresAt, RefreshTokenExpiresAt, User{Uuid, Email, Role}}` → out корректно заполнен, logger.Info вызван с user_uuid.
- [ ] `invalid_credentials_returns_invalid_token` — mock возвращает `status.Error(codes.Unauthenticated, "bad creds")` → ErrInvalidToken.
- [ ] `rate_limited_returns_rate_limited` — mock возвращает `status.Error(codes.ResourceExhausted, ...)` → ErrRateLimited (AuthError), logger.Warn НЕ вызван (rate limit ≠ unavailable).
- [ ] `unavailable_returns_unavailable` — mock возвращает `status.Error(codes.Unavailable, ...)` → ErrUnavailable, logger.Warn вызван с `"method"="login"`.
- [ ] `nil_user_in_response_returns_zero_user` — mock возвращает `LoginResponse{User: nil}` → out.User == User{} без error.

**`TestClient_Refresh`** (table-driven):
- [ ] `happy_path_returns_rotated_token` — mock RefreshToken возвращает `RefreshTokenResponse{AccessToken: "new-a", RefreshToken: "new-r"}` → out.AccessToken=="new-a", out.RefreshToken=="new-r".
- [ ] `invalid_refresh_token_returns_invalid` — mock возвращает Unauthenticated → ErrInvalidToken.
- [ ] `unavailable_returns_unavailable` — mock возвращает Unavailable → ErrUnavailable + Warn.
- [ ] `ip_and_user_agent_propagated_to_grpc` — проверяем через `mock.MatchedBy`, что `req.GetIp() == "1.2.3.4"` и `req.GetUserAgent() == "test-agent"`.

**`TestClient_Refresh_SingleFlight`** — отдельный тест (не table-driven, специальный rendezvous):
- [ ] 2 goroutine с одним refresh_token → mock.RefreshToken called `.Once()` (mockery контракт), оба goroutine получили одинаковый RefreshOutput.

**`TestClient_Refresh_FnPanicRecovered`**:
- [ ] mock RefreshToken — `RunAndReturn(func(...) { panic("boom") })` → Refresh возвращает error с `errors.Is(err, ErrUnavailable)` true, logger.Error вызван.

**`TestClient_Logout`** (table-driven):
- [ ] `happy_path` — mock returns nil → no error, logger.Info "logout success".
- [ ] `invalid_token_returns_invalid` — mock Unauthenticated → ErrInvalidToken.
- [ ] `unavailable_returns_unavailable` — mock Unavailable → ErrUnavailable + Warn.

**`TestClient_LogoutAll`** (table-driven):
- [ ] `happy_path` + 2 error scenarios.

**`TestClient_ChangePassword`** (table-driven):
- [ ] `happy_path`.
- [ ] `invalid_current_password_returns_invalid_token`.
- [ ] `unavailable_returns_unavailable`.

**`TestClient_GetCurrentUser`** (table-driven):
- [ ] `happy_path_returns_user_with_email` — mock возвращает User{Uuid, Email, Role} → returned User имеет все поля заполнены.
- [ ] `empty_user_in_response_returns_unavailable` — mock возвращает `GetCurrentUserResponse{User: nil}` → error wrapped ErrUnavailable.
- [ ] `invalid_token_returns_invalid` — Unauthenticated → ErrInvalidToken.

**`TestClient_Login_LogsNoTokens`** — структурная гарантия:
- [ ] Mock'ируем Logger c expected call `Info(matchCtx, "authclient: login success", "user_uuid", "test-uuid")` `.Once()`. Любой extra field вызовет mismatch.

**testMocks pattern (UT-0026):**

```go
type testMocks struct {
    authClient *mocks.AuthServiceClient
    logger     *mocks.Logger
}

func newClientForTest(t *testing.T) (*Client, testMocks) {
    t.Helper()
    m := testMocks{
        authClient: mocks.NewAuthServiceClient(t),
        logger:     mocks.NewLogger(t),
    }
    c := &Client{
        cfg: Config{
            LoginTimeout:    5 * time.Second,
            RefreshTimeout:  5 * time.Second,
            ValidateTimeout: 2 * time.Second,
        },
        authClient: m.authClient,
        logger:     m.logger,
        sf:         &refresh.Group{},
    }
    return c, m
}
```

> **Note on UT-0028 "No helper functions for test data, mocks, or assertions":**
> The rule prohibits helpers for **test data** and **assertions**. Constructing the test SUT (`newClientForTest`) is sometimes considered acceptable infrastructure (per UT-0026 testMocks). If lint flags this, inline the Client construction at each test. **Decision:** start with inline construction (no `newClientForTest`), promote to helper only if duplication becomes painful.

**Code Pattern (TestClient_Refresh_SingleFlight):**

```go
func TestClient_Refresh_SingleFlight(t *testing.T) {
    t.Parallel()

    const refreshToken = "rt-shared"
    const newAccess = "new-access-token"
    const newRefresh = "new-refresh-token-rotated"

    mockAuth := mocks.NewAuthServiceClient(t)
    mockLogger := mocks.NewLogger(t)

    release := make(chan struct{})
    started := make(chan struct{}, 1)

    mockAuth.EXPECT().
        RefreshToken(
            mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
            mock.MatchedBy(func(req *auth_v1.RefreshTokenRequest) bool {
                return req.GetRefreshToken() == refreshToken &&
                    req.GetIp() == "1.2.3.4" &&
                    req.GetUserAgent() == "test-agent"
            }),
        ).
        RunAndReturn(func(ctx context.Context, req *auth_v1.RefreshTokenRequest, opts ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
            select {
            case started <- struct{}{}:
            default:
            }
            <-release
            return &auth_v1.RefreshTokenResponse{
                AccessToken:           newAccess,
                RefreshToken:          newRefresh,
                AccessTokenExpiresAt:  time.Now().Add(15 * time.Minute).Unix(),
                RefreshTokenExpiresAt: time.Now().Add(14 * 24 * time.Hour).Unix(),
            }, nil
        }).Once()

    // logger.Info will be called twice (one per Refresh return path);
    // mockery .Times(2) enforces exact count.
    mockLogger.EXPECT().
        Info(
            mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
            "authclient: refresh success",
        ).Times(2)

    c := &Client{
        cfg:        Config{RefreshTimeout: 5 * time.Second},
        authClient: mockAuth,
        logger:     mockLogger,
        sf:         &refresh.Group{},
    }

    const N = 2
    var wg sync.WaitGroup
    outs := make([]RefreshOutput, N)
    errs := make([]error, N)

    for i := 0; i < N; i++ {
        i := i // EC-0091 capture
        wg.Add(1)
        go func() {
            defer wg.Done()
            outs[i], errs[i] = c.Refresh(context.Background(), refreshToken, "1.2.3.4", "test-agent")
        }()
    }

    // Wait for leader to enter mock fn — signal that singleflight winner
    // is committed.
    <-started

    // Give the follower goroutine time to converge on sf.Do.
    // Best-effort sync; DOD compensates via -count=10.
    time.Sleep(50 * time.Millisecond)

    close(release)
    wg.Wait()

    for i := 0; i < N; i++ {
        require.NoError(t, errs[i])
        assert.Equal(t, newAccess, outs[i].AccessToken)
        assert.Equal(t, newRefresh, outs[i].RefreshToken)
    }
    // mockAuth.AssertExpectations is auto-invoked by mockery via t.Cleanup
    // when constructed with `mocks.NewAuthServiceClient(t)` — Once() enforced.
}
```

**Code Pattern (TestClient_Refresh_FnPanicRecovered):**

```go
func TestClient_Refresh_FnPanicRecovered(t *testing.T) {
    t.Parallel()

    mockAuth := mocks.NewAuthServiceClient(t)
    mockLogger := mocks.NewLogger(t)

    mockAuth.EXPECT().
        RefreshToken(
            mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
            mock.MatchedBy(func(req *auth_v1.RefreshTokenRequest) bool { return req.GetRefreshToken() == "rt" }),
        ).
        RunAndReturn(func(context.Context, *auth_v1.RefreshTokenRequest, ...grpc.CallOption) (*auth_v1.RefreshTokenResponse, error) {
            panic("boom")
        }).Once()

    mockLogger.EXPECT().
        Error(
            mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
            "authclient: refresh fn panic",
            mock.MatchedBy(func(err error) bool { return errors.Is(err, ErrUnavailable) }),
            "recover", "boom",
        ).Once()

    mockLogger.EXPECT().
        Warn(
            mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
            "authclient: AUTH unavailable",
            "method", "refresh",
        ).Once()

    c := &Client{
        cfg:        Config{RefreshTimeout: 5 * time.Second},
        authClient: mockAuth,
        logger:     mockLogger,
        sf:         &refresh.Group{},
    }

    _, err := c.Refresh(context.Background(), "rt", "1.2.3.4", "test-agent")
    require.Error(t, err)
    assert.True(t, errors.Is(err, ErrUnavailable))
}
```

**Critical notes:**
- Mock'и создаются через `mocks.NewAuthServiceClient(t)` — это auto-registers `AssertExpectations` в t.Cleanup. Это значит `.Once()` / `.Times(n)` ENFORCE'ятся при завершении теста (UT-0022).
- `mock.MatchedBy(func(...) bool)` — единственный разрешённый matcher для ctx и proto-сообщений (UT-0023 запрещает mock.Anything).
- Для proto-аргументов matcher использует **getter-сравнение**, не reflect.DeepEqual (proto unexported state ломает equality).
- `RunAndReturn(func(...) (resp, err))` — mockery API для динамической логики (rendezvous, panic). Подходит для race-теста.
- Тесты не используют `helper-функции для test data` (UT-0028) — конструкция inline в test'е. testMocks struct — единственная общая абстракция (UT-0026).

---

### Task 6: CREATE `lib/auth_integration_test.go`

**Action:** CREATE
**File:** [lib/auth_integration_test.go](../../lib/auth_integration_test.go)

**Description:** Интеграционные тесты против реального AUTH (поднятого через `make env-up`). Build tag `//go:build integration`. Каждый тест: создаёт уникального пользователя в AUTH (direct SQL helper), Login через SDK, выполняет операцию, проверяет результат.

**Pre-requisites:**
- `make env-up` поднял AUTH + Postgres.
- ENV `RUN_INTEGRATION_TESTS=true`, `AUTH_ENDPOINT=localhost:50051`, `AUTH_DB_DSN=postgres://...` (из `dev/deployments/test.env`).

**Details:**

- Build tag `//go:build integration` (первая строка файла).
- Package `authclient_test` (external — TASK-001 уже использует `lib_test`).
- `TestMain` НЕ нужен — `lib/client_integration_test.go` (TASK-001) уже gate'ит `RUN_INTEGRATION_TESTS`.
- Helper `seedTestUser(t *testing.T, email, password string)`:
  - Подключается к AUTH Postgres (`AUTH_DB_DSN` ENV).
  - Bcrypt-хэширует пароль (`golang.org/x/crypto/bcrypt`, cost=4 для теста — fast).
  - INSERT INTO users (uuid, email, password_hash, role, is_blocked, token_version) VALUES (gen_random_uuid(), $1, $2, 'user', false, 0).
  - `t.Cleanup` — DELETE WHERE email=$1.

**Required Test Cases:**

```go
//go:build integration

package authclient_test

import (
    "context"
    "database/sql"
    "fmt"
    "os"
    "sync"
    "testing"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
    authclient "github.com/webdelo/auth-client-go/lib"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "golang.org/x/crypto/bcrypt"
)

const (
    testIP        = "203.0.113.55"
    testUserAgent = "auth-client-go-integration-test/0.1.0"
)

func newIntegrationClient(t *testing.T) *authclient.Client {
    t.Helper()
    endpoint := os.Getenv("AUTH_ENDPOINT")
    if endpoint == "" {
        endpoint = "localhost:50051"
    }
    c, err := authclient.New(authclient.Config{Endpoint: endpoint})
    require.NoError(t, err)
    t.Cleanup(func() { _ = c.Close() })
    return c
}

func seedTestUser(t *testing.T, email, password string) {
    t.Helper()
    dsn := os.Getenv("AUTH_DB_DSN")
    require.NotEmpty(t, dsn, "AUTH_DB_DSN must be set in dev/deployments/test.env")

    db, err := sql.Open("pgx", dsn)
    require.NoError(t, err)
    t.Cleanup(func() { _ = db.Close() })

    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
    require.NoError(t, err)

    _, err = db.ExecContext(context.Background(), `
        INSERT INTO users (uuid, email, password_hash, role, is_blocked, token_version, created_at)
        VALUES (gen_random_uuid(), $1, $2, 'user', false, 0, NOW())
    `, email, string(hash))
    require.NoError(t, err, "INSERT user")

    t.Cleanup(func() {
        _, _ = db.ExecContext(context.Background(),
            `DELETE FROM users WHERE email = $1`, email)
    })
}

func uniqueEmail(t *testing.T) string {
    t.Helper()
    return fmt.Sprintf("test_%s_%d@example.com",
        strings.ReplaceAll(t.Name(), "/", "_"),
        time.Now().UnixNano())
}

func TestLogin_Happy(t *testing.T) {
    t.Parallel()
    email := uniqueEmail(t)
    const password = "Str0ng!Passw0rd"
    seedTestUser(t, email, password)
    c := newIntegrationClient(t)

    out, err := c.Login(context.Background(), authclient.LoginInput{
        Email:     email,
        Password:  password,
        ClientIP:  testIP,
        UserAgent: testUserAgent,
    })
    require.NoError(t, err)
    assert.NotEmpty(t, out.AccessToken)
    assert.NotEmpty(t, out.RefreshToken)
    assert.Greater(t, out.AccessExpiresIn, time.Duration(0))
    assert.Greater(t, out.RefreshExpiresIn, time.Duration(0))
    assert.Equal(t, email, out.User.Email)
    assert.NotEmpty(t, out.User.UUID)
}

func TestRefresh_HappyAndRotation(t *testing.T) {
    t.Parallel()
    email := uniqueEmail(t)
    const password = "Str0ng!Passw0rd"
    seedTestUser(t, email, password)
    c := newIntegrationClient(t)

    initial, err := c.Login(context.Background(), authclient.LoginInput{
        Email: email, Password: password, ClientIP: testIP, UserAgent: testUserAgent,
    })
    require.NoError(t, err)

    refreshed, err := c.Refresh(context.Background(), initial.RefreshToken, testIP, testUserAgent)
    require.NoError(t, err)
    assert.NotEqual(t, initial.RefreshToken, refreshed.RefreshToken, "refresh token must rotate")
    assert.NotEqual(t, initial.AccessToken, refreshed.AccessToken)

    // Old refresh token must no longer work.
    _, err = c.Refresh(context.Background(), initial.RefreshToken, testIP, testUserAgent)
    require.Error(t, err, "old refresh token should be invalid after rotation")
    assert.True(t, errors.Is(err, authclient.ErrInvalidToken))
}

func TestRefresh_Race(t *testing.T) {
    // NB: no t.Parallel — race test stresses singleflight, isolation matters
    email := uniqueEmail(t)
    const password = "Str0ng!Passw0rd"
    seedTestUser(t, email, password)
    c := newIntegrationClient(t)

    initial, err := c.Login(context.Background(), authclient.LoginInput{
        Email: email, Password: password, ClientIP: testIP, UserAgent: testUserAgent,
    })
    require.NoError(t, err)

    const N = 10
    var wg sync.WaitGroup
    outs := make([]authclient.RefreshOutput, N)
    errs := make([]error, N)
    start := make(chan struct{})

    for i := 0; i < N; i++ {
        i := i
        wg.Add(1)
        go func() {
            defer wg.Done()
            <-start
            outs[i], errs[i] = c.Refresh(context.Background(), initial.RefreshToken, testIP, testUserAgent)
        }()
    }
    close(start)
    wg.Wait()

    // All N goroutines must succeed and share the SAME rotated token —
    // singleflight collapsed them into one RPC.
    for i := 0; i < N; i++ {
        require.NoError(t, errs[i], "goroutine %d", i)
        assert.Equal(t, outs[0].AccessToken, outs[i].AccessToken, "goroutine %d", i)
        assert.Equal(t, outs[0].RefreshToken, outs[i].RefreshToken, "goroutine %d", i)
    }

    // Family still alive — the rotated token works for one more refresh.
    _, err = c.Refresh(context.Background(), outs[0].RefreshToken, testIP, testUserAgent)
    require.NoError(t, err, "family detection should NOT trigger after singleflight dedup")
}

func TestLogout(t *testing.T) {
    t.Parallel()
    // ... seed + Login + Logout + Refresh-with-old-token-must-fail
}

func TestChangePassword(t *testing.T) {
    t.Parallel()
    // ... seed + Login + ChangePassword + Login-with-old-pw-must-fail + Login-with-new-pw-works
}

func TestGetCurrentUser_HasEmail(t *testing.T) {
    t.Parallel()
    email := uniqueEmail(t)
    const password = "Str0ng!Passw0rd"
    seedTestUser(t, email, password)
    c := newIntegrationClient(t)

    out, err := c.Login(context.Background(), authclient.LoginInput{
        Email: email, Password: password, ClientIP: testIP, UserAgent: testUserAgent,
    })
    require.NoError(t, err)

    user, err := c.GetCurrentUser(context.Background(), out.AccessToken)
    require.NoError(t, err)
    assert.Equal(t, email, user.Email, "GetCurrentUser must return email")
    assert.NotEmpty(t, user.UUID)
}
```

**Critical notes:**
- `TestRefresh_Race` намеренно БЕЗ `t.Parallel()` — два параллельных race-теста с разными пользователями технически безопасны, но один источник flake'ов меньше. (UT-0042: NOT use t.Parallel() with shared state — здесь shared state ноль, но keep simple.) Опционально включить t.Parallel() позже.
- `seedTestUser` импорт `database/sql` + `github.com/jackc/pgx/v5/stdlib` — это **integration-test-only** dep, под build tag `integration`. `go mod tidy` после добавления.
- `bcrypt` с `bcrypt.MinCost` (=4) — быстрый для тестов. Production AUTH использует cost=12.
- Уникальный email через `t.Name() + UnixNano` — параллельные тесты не конфликтуют. Cleanup через `t.Cleanup` — DELETE WHERE email=$1.
- Имена констант `testIP`/`testUserAgent` — на уровне файла (CS-0003 no magic numbers, ES-0173 no abbrev).
- Все error-проверки через `require.NoError` (тест fail-fast) ИЛИ `errors.Is` (для error-path scenarios).

---

### Task 7: REGENERATE mocks + run `make audit`

**Action:** EXECUTE
**Files:** none direct; orchestration

**Description:** После добавления `lib/auth.go` ничего нового в mockery.yaml — `auth_v1.AuthServiceClient` интерфейс уже зарегистрирован в TASK-003 PRP. Но генератор должен пересобрать (на случай, если TASK-003 не сгенерил все методы — теоретически impossible, но проверяем).

**Commands:**
```bash
# 1. Регенерация моков
make generate-mocks

# 2. Проверка, что моки содержат все методы из AuthServiceClient
grep -E "func \(_m \*AuthServiceClient\) (Login|RefreshToken|Logout|LogoutAll|ChangePassword|GetCurrentUser)\(" lib/mocks/auth_service_client_mock.go || \
    (echo "FAIL: mocks missing methods" && exit 1)

# 3. Запуск unit-тестов
make test
make test-race

# 4. Целенаправленный race-test
go test -race -run TestClient_Refresh_SingleFlight -count=10 ./lib/...

# 5. Группа Group тест
go test -race -count=10 ./lib/internal/refresh/...

# 6. Coverage
make coverage
# Проверить >= 80% для lib/auth.go + lib/internal/refresh/

# 7. Lint + vet
make lint
make vet

# 8. Full audit
make audit

# 9. Integration tests (опционально, если AUTH доступен локально)
make env-up
make test-integration
make env-down
```

**Critical notes:**
- `make generate-mocks` идемпотентен — если моки уже актуальны, ничего не перегенерится. Если перегенерируется — diff в `lib/mocks/auth_service_client_mock.go` минимальный.
- DOD требует `-count=10` зелёным — это компенсирует best-effort rendezvous в unit-тесте singleflight.
- Integration-тесты требуют АНКОРНЕНГО pgx-stdlib + bcrypt в go.mod как direct deps (integration-only). `go mod tidy` после добавления.

---

## Integration

### Dependencies

| Package | Purpose |
|---------|---------|
| `golang.org/x/sync/singleflight` | Refresh dedup (already direct via TASK-001 blank import) |
| `github.com/webdelo/auth-proto/go` | gRPC types (already from TASK-002) |
| `google.golang.org/grpc` | Variadic `grpc.CallOption` type in mock signatures (already) |
| `golang.org/x/crypto/bcrypt` | **NEW (test-only)** — integration test password hashing |
| `github.com/jackc/pgx/v5/stdlib` | **NEW (test-only)** — integration test Postgres driver for direct user seeding |

**`go mod tidy`** после Task 6 — добавит две новые indirect/direct (помеченные `// indirect` если используются ТОЛЬКО под build tag integration).

### Config

No config changes. `Config.LoginTimeout` / `Config.RefreshTimeout` / `Config.ValidateTimeout` уже определены в TASK-002.

### Mockery

**No mockery.yaml change.** TASK-003 уже зарегистрировал:
- `github.com/webdelo/auth-proto/go/auth_v1` → `AuthServiceClient` → `lib/mocks/auth_service_client_mock.go`
- `github.com/webdelo/auth-client-go/lib` → `Logger`, `Cache` → `lib/mocks/logger_mock.go`, `lib/mocks/cache_mock.go`

`make generate-mocks` для регенерации (идемпотентен).

---

## Testing

### Unit Tests

| Entity | Test File | Cases |
|--------|-----------|-------|
| `refresh.Group.Do` | [lib/internal/refresh/group_test.go](../../lib/internal/refresh/group_test.go) | 6 cases (см. Task 2) |
| `expiresIn` | [lib/auth_test.go](../../lib/auth_test.go) | 4 cases |
| `*Client.Login` | [lib/auth_test.go](../../lib/auth_test.go) | 5 cases |
| `*Client.Refresh` | [lib/auth_test.go](../../lib/auth_test.go) | 4 happy/error cases + SingleFlight + FnPanicRecovered |
| `*Client.Logout` | [lib/auth_test.go](../../lib/auth_test.go) | 3 cases |
| `*Client.LogoutAll` | [lib/auth_test.go](../../lib/auth_test.go) | 3 cases |
| `*Client.ChangePassword` | [lib/auth_test.go](../../lib/auth_test.go) | 3 cases |
| `*Client.GetCurrentUser` | [lib/auth_test.go](../../lib/auth_test.go) | 3 cases |

**Required Test Cases (summary):**

- [ ] `TestExpiresIn` table-driven (4 cases)
- [ ] `TestClient_Login` (5 cases)
- [ ] `TestClient_Refresh` (4 cases happy/error)
- [ ] `TestClient_Refresh_SingleFlight` (race, mockery `.Once()`)
- [ ] `TestClient_Refresh_FnPanicRecovered`
- [ ] `TestClient_Logout` (3 cases)
- [ ] `TestClient_LogoutAll` (3 cases)
- [ ] `TestClient_ChangePassword` (3 cases)
- [ ] `TestClient_GetCurrentUser` (3 cases — happy, nil_user, invalid_token)
- [ ] `TestClient_Login_LogsNoTokens` — структурная гарантия отсутствия токенов в лог-полях
- [ ] `TestGroup_Do_HappyPath`
- [ ] `TestGroup_Do_ConcurrentDedupSingleCall`
- [ ] `TestGroup_Do_DifferentKeysIndependent`
- [ ] `TestGroup_Do_KeyForgottenNextCallReruns` — критичный для §A3/§R8 гарантии
- [ ] `TestGroup_Do_FnErrorPropagates`

### Integration Tests

| Endpoint / Scenario | Test File | Cases |
|---------------------|-----------|-------|
| Login (full credential flow) | [lib/auth_integration_test.go](../../lib/auth_integration_test.go) | TestLogin_Happy |
| Refresh + token rotation | same | TestRefresh_HappyAndRotation |
| Refresh race (10 goroutines) | same | TestRefresh_Race |
| Logout invalidates refresh | same | TestLogout |
| ChangePassword bumps token_version | same | TestChangePassword |
| GetCurrentUser returns email | same | TestGetCurrentUser_HasEmail |

**Required Integration Cases:**

- [ ] `TestLogin_Happy` — Login возвращает токены и user.Email.
- [ ] `TestRefresh_HappyAndRotation` — Refresh обменивает токен на новый; старый больше не работает.
- [ ] `TestRefresh_Race` — 10 goroutine с одним refresh_token: все 10 успешны с одинаковым новым токеном; family НЕ убита.
- [ ] `TestLogout` — Logout → Refresh со старым refresh_token → ErrInvalidToken.
- [ ] `TestChangePassword` — ChangePassword → Login со старым паролем → ErrInvalidToken; со новым — OK.
- [ ] `TestGetCurrentUser_HasEmail` — email присутствует.

---

## Validation

### Build & Lint

```bash
# Компиляция (включая integration build)
go build ./...
go build -tags integration ./...

# Линтер (все правила из .golangci.yaml)
make lint

# Vet
make vet

# Unit тесты
make test
make test-race

# Race-stress (DOD)
go test -race -run TestClient_Refresh_SingleFlight -count=10 ./lib/...
go test -race -count=10 ./lib/internal/refresh/...

# Coverage
make coverage
go tool cover -func coverage/coverage.out | grep -E "auth\.go|refresh/group\.go" | awk '{print $1, $3}'

# Integration (требует AUTH)
make env-up
make test-integration
make env-down

# Полный аудит
make audit
```

### Manual Checks

1. Открыть `lib/auth.go` — убедиться, что:
   - Все 6 публичных методов имеют godoc.
   - `expiresIn` приватный (lowercase).
   - Импорт `"github.com/webdelo/auth-client-go/lib/internal/refresh"` отсутствует — это для `client.go`. В `auth.go` `c.sf` используется через `Client`.
2. Открыть `lib/client.go` — проверить, что `sf *refresh.Group` инициализирован в `New`.
3. `grep -rE "(access_token|refresh_token|password)" lib/auth.go | grep -v "/auth_v1" | grep -v "//"` — НЕ должно быть raw-tokens / passwords как field-values в Logger-вызовах. Логи содержат только `"user_uuid"`.
4. `grep -rE "mock\.Anything" lib/auth_test.go` — должно быть пусто (UT-0023).
5. `grep -rE "nolint" lib/` — должно быть пусто (ES-0171).
6. `go test -race -run TestClient_Refresh_SingleFlight -count=10 ./lib/...` — все 10 прогонов зелёные.

---

## DOD (Definition of Done)

- [ ] `lib/internal/refresh/group.go` создан; `Group.Do` вызывает `Forget` после `Do`.
- [ ] `lib/internal/refresh/group_test.go` создан; `TestGroup_Do_KeyForgottenNextCallReruns` явно проверяет Forget.
- [ ] `lib/client.go` модифицирован: добавлено поле `sf *refresh.Group` + инициализация в `New`.
- [ ] `lib/auth.go` создан; экспортирует `LoginInput`, `LoginOutput`, `RefreshOutput`; реализует 6 методов на `*Client`.
- [ ] `lib/auth_test.go` создан; все required test cases проходят.
- [ ] `lib/auth_integration_test.go` создан; build tag `//go:build integration`; покрывает 6 сценариев.
- [ ] `TestClient_Refresh_SingleFlight` — mock `RefreshToken` вызван **ровно 1 раз** (mockery `.Once()` enforces).
- [ ] `errors.Is(err, ErrXxx)` корректно для всех error-path во всех 6 методах.
- [ ] `AccessExpiresIn` / `RefreshExpiresIn` корректно вычислены через `expiresIn`.
- [ ] Integration-тесты `TestLogin_Happy`, `TestRefresh_HappyAndRotation`, `TestRefresh_Race`, `TestLogout`, `TestChangePassword`, `TestGetCurrentUser_HasEmail` проходят через `make test-integration`.
- [ ] `go test -race -run TestClient_Refresh_SingleFlight -count=10 ./lib/...` — все 10 прогонов зелёные.
- [ ] `go test -race -count=10 ./lib/internal/refresh/...` — все 10 прогонов зелёные.
- [ ] Unit coverage по `lib/auth.go` + `lib/internal/refresh/` ≥ 80% (`make coverage`).
- [ ] Логи не содержат токены/пароли/email — проверено через mockery-Logger expectations с явными fields.
- [ ] `make lint`, `make vet`, `make test`, `make test-race`, `make audit` — без ошибок.
- [ ] `go mod tidy` — без diff'а в `go.mod` direct deps кроме integration-test-only (bcrypt, pgx/v5).
- [ ] PRP-инвариант: НЕТ `mock.Anything` / `mock.AnythingOfType` (UT-0023), НЕТ `nolint` (ES-0171), нет дублирования `hashToken` (берётся из TASK-003 `lib/validate.go`).

---

## Open Questions / Risks

1. **Integration test user seeding** — direct SQL INSERT в `users` таблицу AUTH через pgx. Это связывает SDK тестовую инфру с внутренней схемой AUTH. Если AUTH изменит схему (например переименует `password_hash` в `password`) — все integration-тесты сломаются. **Митигация:** изолировать seeding в `tests/integration_helpers_test.go` (single source of truth), документировать зависимость в README. Альтернатива на будущее (v0.2.0+) — AUTH публикует `CreateUser` admin RPC + bootstrap admin через AUTH-side seeder.

2. **`TestClient_Refresh_SingleFlight` flakiness** — best-effort rendezvous через `time.Sleep(50ms)` после `<-started`. На slow CI (e.g. macOS GitHub runners под load) follower может не успеть дойти до `sf.Do`. **Митигация:** DOD требует `-count=10` — статистическая защита. Если в практике увидим flake'и, можно увеличить sleep до 100ms или использовать `runtime.Gosched()`-loop с timeout.

3. **Refresh ctx cancellation semantics** — singleflight НЕ уважает caller ctx. Если две вкладки имеют разные ctx и одна cancel'нулась мгновенно — она всё равно ждёт результат leader'а. Это документировано в godoc и в ADR §A3, но может быть surprise'ом для consumer'ов. **Митигация:** README в TASK-007 должен иметь явный пункт «Refresh blocks until the underlying RPC completes, regardless of caller ctx cancellation».

4. **Multi-replica BFF** — singleflight per-process. Если BFF запущен в 3+ replicas, и пользовательский refresh пришёл в две разные replica'ы, singleflight НЕ сработает. AUTH детектит reuse → family убита. **Принято** для v0.1.0; документируется в README + ADR §R3.

5. **`logUnavailableIfNeeded` через `errors.Is`** — лишний lookup при каждом error-path. Negligible cost для error path (<0.001% общего трафика), но всё же. Альтернатива — флаг в `mapGRPCError` возвращать. **Решение:** keep current — оптимизация преждевременная.

6. **`ChangePassword` — wrong-current-password выходит как `ErrUnavailable`** — AUTH возвращает `codes.InvalidArgument`; текущая `mapGRPCError` (TASK-002) не имеет ветки для InvalidArgument → wrap в `ErrUnavailable`. Это семантическое несоответствие (это business error, не недоступность). **Принято для v0.1.0:** documented в godoc на `ChangePassword`. **Резолюция в v0.2.0** — расширить `mapGRPCError` (additive change): `case codes.InvalidArgument: return ErrInvalidToken` ИЛИ ввести новый sentinel `ErrInvalidPassword`. PRP TASK-004 НЕ затрагивает `lib/errors.go` (out of scope). Альтернатива (если решим включить в TASK-004): дополнительные 3 строки в `lib/errors.go` mapGRPCError switch — может быть scope-shift'нуто в TASK-004 после согласования с пользователем. **По умолчанию: НЕ включать, документировать.**

7. **Refresh не логирует user_uuid** — `RefreshTokenResponse` не содержит User. SDK не парсит JWT (ADR §A14). Лог success — только `"authclient: refresh success"` без user_uuid field. Альтернатива — после Refresh неявно вызвать ValidateAccessToken для нового access_token, чтобы получить UUID и залогировать. Это double-RPC overhead. **Решение:** log без UUID; consumer'у можно сделать ValidateAccessToken explicitly если нужен audit log с user_uuid.

---

## Score: 9/10

**Сильные стороны:**
- Все 6 методов имеют детальные code patterns с правильным error mapping.
- Singleflight design покрыт от первого Forget до race-теста с .Once() constraint.
- Все edge cases (nil User в LoginResponse, panic в fn, type-assert failure, proto unexported state) описаны.
- Тестовая стратегия чётко делит unit (mockery) и integration (real AUTH + Postgres seeding).
- Coding standards mapping явный (ES-0060/0061/0062/0064, EC-0050/0053/0091, UT-0001/0021/0023).

**Что бы улучшилось до 10/10:**
- Code-research отчёт от sibling-репов (signal-analyzer/fundingbot) с готовыми singleflight + race-test паттернами (в процессе background-агента — добавим cross-ссылку при поступлении).

**PRP готов к выполнению. STOP — ожидаем подтверждение пользователя.**
