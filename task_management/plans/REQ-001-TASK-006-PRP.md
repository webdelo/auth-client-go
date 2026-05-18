# REQ-001-TASK-006-PRP: Examples — basic, http_middleware, grpc_interceptor

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-001-TASK-006-PRP |
| Parent Request | [REQ-001](../tasks/REQ-001.md) |
| Parent Task | [REQ-001-TASK-006](../tasks/REQ-001-TASK-006.md) |
| Parent ADR | [REQ-001-ADR](./REQ-001-ADR.md) |
| Created | 2026-05-18 |
| Updated | 2026-05-18 |
| Score | 8/10 — −1 за API-drift риск (TASK-003/004/005 не выполнены — examples пишутся против PRP-документов, not against merged code; mitigated Task 0 pre-flight gate), −1 за compile-only валидацию examples (main-пакеты в Go не имеют unit-тестов; `make build-examples` это единственная автоматизированная проверка). Чтобы поднять до 9: после merge TASK-003/004/005 — добавить smoke-test в CI который запускает `.temp/examples/<name>` против реального AUTH (вне scope TASK-006). |

---

## Goal

### Summary

Создать три рабочих `main`-пакета в `lib/examples/` (`basic/`, `http_middleware/`, `grpc_interceptor/`), каждый — минималистичная демонстрация одной фичи SDK, плюс README по 50 строк на каждый. Заменить placeholder `lib/examples/basic/main.go`, обновить Makefile-таргет `build-examples` на сборку в `.temp/examples/<name>`. Examples используют только публичный API SDK (`authclient.*`) и stdlib (`net/http`, `encoding/json`, `log/slog`, `os`, `flag`) + уже-импортированный `google.golang.org/grpc` (включая бесплатные subpackages `health` и `health/grpc_health_v1`).

### Why

- **Examples — это документация в коде.** TASK-007 DOD требует, чтобы BFF-разработчик собрал рабочий middleware за < 30 минут после `go get` SDK. Три рабочих примера с командами запуска покрывают ровно три типичных сценария: auth-flow (basic), HTTP-сервер (http_middleware), gRPC-сервер (grpc_interceptor).
- **Доказательство работоспособности публичного API.** `make build-examples` в CI = compile-test для каждого consumer-сценария. Если SDK сломал public surface — examples не соберутся, и это видно сразу, без интеграционного теста.
- **Бесплатная защита от dep-bloat.** DOD: `go.sum` после `build-examples` идентичен `go.sum` до. Examples — формальный «канарейка»-тест на случайное добавление сторонних библиотек (zap, cobra, viper, etc.).
- **Слой выше unit/integration тестов.** Тесты проверяют единицы кода; examples проверяют, что SDK **складывается** в полезный сценарий из публичных вызовов.

### What

После выполнения TASK'а:

```bash
# basic flow
export AUTH_ENDPOINT=localhost:50051
export AUTH_TEST_EMAIL=test@example.com
export AUTH_TEST_PASSWORD=secret123
go run ./lib/examples/basic
# → step1 New ok / step2 Login ok access=abcd1234... / step3 Validate ok user=... / step4 Logout ok / step5 Close ok

# protected HTTP server
go run ./lib/examples/http_middleware
# listens on :8080; curl -X POST :8080/auth/login -d '{"email":"...","password":"..."}'
# returns {access_token, expires_in, user} + sets HttpOnly refresh cookie
# curl -H "Authorization: Bearer <access>" :8080/api/profile → {uuid, role}

# protected gRPC server
go run ./lib/examples/grpc_interceptor
# bound to :50061; built-in demo client fires 3 RPCs and exits 0 on success.
# /grpc.health.v1.Health/Check  no auth → SERVING (in default openMethods, skipped)
# /grpc.health.v1.Health/List   with valid Bearer in metadata → ok
# /grpc.health.v1.Health/List   no metadata → codes.Unauthenticated

# build all
make build-examples
# → .temp/examples/basic, .temp/examples/http_middleware, .temp/examples/grpc_interceptor
```

### Success Criteria

- [ ] `lib/examples/basic/main.go` собирается, демонстрирует `New → Login → ValidateAccessToken → Logout → Close`, ENV-driven (`AUTH_ENDPOINT`, `AUTH_TEST_EMAIL`, `AUTH_TEST_PASSWORD`), печатает шаги в stdout с первыми 8 символами токенов.
- [ ] `lib/examples/http_middleware/main.go` собирается, поднимает HTTP-сервер на `:8080` с тремя эндпоинтами: `/api/profile` (под `client.HTTPMiddleware`), `/auth/login`, `/auth/logout`. Login использует `IPFromRequest`/`UserAgentFromRequest`, ставит refresh-cookie через `SetRefreshCookie`. Logout читает refresh-cookie, вызывает `client.Logout`, чистит через `ClearRefreshCookie`. Логирование через inline `slogAdapter`-обёртку над `slog.Default()`.
- [ ] `lib/examples/grpc_interceptor/main.go` собирается, поднимает gRPC-сервер на `:50061` с `grpc.UnaryInterceptor(client.UnaryInterceptor())`, регистрирует `health.NewServer()`. Demo-client в том же `main.go` отправляет 3 RPC и проверяет ожидаемые исходы.
- [ ] `lib/examples/basic/README.md`, `lib/examples/http_middleware/README.md`, `lib/examples/grpc_interceptor/README.md` существуют, < 50 строк каждый, формат «What / Run / Expected output».
- [ ] `make build-examples` собирает все три бинарника в `.temp/examples/<name>/` без warnings, exit code 0.
- [ ] `go list -deps ./lib/examples/... | grep -F "/lib/internal" | wc -l` → `0`.
- [ ] `git diff go.sum` после `make build-examples` пуст.
- [ ] `make audit` (vet + lint + test) проходит чисто.
- [ ] Запуск любого example-бинарника без аргументов и без ENV → печатает usage в stderr и завершается с **exit code 0** (CI: build success ≠ run success, см. Gotcha G3).

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | ES-0010 (package name), ES-0011 (exported), ES-0021 (guard clauses), ES-0040 (return error), ES-0051 (Logger interface — consumer-side), ES-0060 (error handling), ES-0061 (wrap with context), ES-0063 (no panic), ES-0080/0081 (godoc), ES-0140 (`New`), ES-0143 (noop defaults — adapter обёртки над `slog`), ES-0171 (no nolint) |
| Effective Concurrency | [effective_concurrency.md](../../.claude/guides/golang/effective_concurrency.md) | EC-0050 (ctx first), EC-0053 (timeouts через `context.WithTimeout`), EC-0080 (signal shutdown через `signal.NotifyContext`) — для http и grpc серверов с graceful shutdown |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | CS-0001 (line length: 140 per `.golangci.yaml`), CS-0010 (empty line after blocks), CS-0020 (top-to-bottom storytelling), CS-0021 (clear names), CS-0050 (один main-пакет на пример), CS-0060 (godoc на public — но в `main`-пакетах ничего не экспортируется, кроме package-doc), CS-0070 (English), CS-0111 (minimal inline comments) |
| Coding Scope (этот TASK) | [.temp/rules/REQ-001-TASK-006/coding_scope.md](../../.temp/rules/REQ-001-TASK-006/coding_scope.md) | 10 загруженных правил (CS-0050, ES-0040, ES-0044, ES-0051, ES-0055, ES-0056, ES-0060, ES-0080, ES-0140, ES-0143) |
| Review Scope | [.temp/rules/REQ-001-TASK-006/review_scope.md](../../.temp/rules/REQ-001-TASK-006/review_scope.md) | По умолчанию весь набор (review этого TASK покроет examples и Makefile-патч) |

### Concurrency Patterns

- [x] **Graceful shutdown через `signal.NotifyContext`** — для `http_middleware` (`http.Server.Shutdown`) и `grpc_interceptor` (`grpc.Server.GracefulStop`). Один-`select` на SIGINT/SIGTERM, нет goroutine leaks.
- [ ] Singleflight / Worker pools / errgroup — НЕ нужны в этом TASK'е (examples не имеют конкурентности уровня SDK).

---

## Analysis Files

- **Code Research**: [2026-05-18_code-research_REQ-001-TASK-006.md](../../.memory_bank/agent_task_results/2026-05-18_code-research_REQ-001-TASK-006.md) — полный inventory public API после TASK-002/003/004/005, подтверждение transitive-доступности `grpc/health`, slog-adapter pattern, Makefile-расхождение, lint config.
- **Alternatives Analysis**: [2026-05-18-REQ-001-TASK-006-alternatives-analysis.md](../session_analysis/2026-05-18-REQ-001-TASK-006-alternatives-analysis.md) — выбор `Health/List` как protected-метода в gRPC примере (Option A) vs hand-crafted `ServiceDesc` / custom proto / unknown-handler tricks.
- **Predecessor PRPs** (содержат полные API-сигнатуры, которые examples используют):
  - [REQ-001-TASK-003-PRP.md](./REQ-001-TASK-003-PRP.md) — `User`/`Role`/`WithUser`/`UserFromContext`/`ValidateAccessToken`/`HTTPMiddleware`/`UnaryInterceptor`
  - [REQ-001-TASK-004-PRP.md](./REQ-001-TASK-004-PRP.md) — `LoginInput`/`LoginOutput`/`RefreshOutput`/`Login`/`Refresh`/`Logout`/`LogoutAll`/`ChangePassword`/`GetCurrentUser`
  - [REQ-001-TASK-005-PRP.md](./REQ-001-TASK-005-PRP.md) — `SetRefreshCookie`/`ClearRefreshCookie`/`CookieOption`/`WithCookie*`/`IPFromRequest`/`UserAgentFromRequest`
- **ADR**: [REQ-001-ADR.md](./REQ-001-ADR.md) §A9 (cookie defaults), §A10 (HTTP middleware), §A11 (gRPC interceptor), §A12 (fingerprint helpers).

---

## Context

### Current Structure

```
lib/
├── client.go                # Config, FailMode, Option, Client, New, Close, Cache interface, WithOpenMethods
├── errors.go                # 5 sentinels, AuthError, RetryAfter, mapGRPCError
├── logger.go                # Logger, NoopLogger
├── client_test.go
├── client_integration_test.go
├── errors_test.go
├── logger_test.go
├── examples/
│   └── basic/
│       └── main.go          # PLACEHOLDER — only prints version line
└── internal/
    └── tools/tools.go
```

After TASK-003/004/005 (PRP-ready, will run before TASK-006), the lib/ tree adds: `context.go`, `validate.go`, `middleware_http.go`, `interceptor_grpc.go`, `auth.go`, `cookies.go`, `helpers.go`, `internal/cache/`, `internal/refresh/`, `mocks/`.

Makefile current `build-examples` (lines 80–88):

```makefile
build-examples:
	@mkdir -p bin
	@for dir in lib/examples/*/; do \
		name=$$(basename $$dir); \
		echo "  Building example-$$name..."; \
		go build -o bin/example-$$name ./$$dir; \
	done
	@echo "✅ Examples built in bin/"
```

`.gitignore` already contains `.temp/` (line confirmed). `bin/` also in `.gitignore`.

### Desired Structure

```
lib/examples/
├── basic/
│   ├── main.go              # REPLACED — Login → Validate → Logout demo
│   └── README.md            # NEW — < 50 lines, What/Run/Expected output
├── http_middleware/         # NEW directory
│   ├── main.go              # NEW — HTTP server with /api/profile, /auth/login, /auth/logout
│   └── README.md            # NEW
└── grpc_interceptor/        # NEW directory
    ├── main.go              # NEW — gRPC server + demo-client in one file
    └── README.md            # NEW

Makefile  # UPDATED — build-examples → .temp/examples/<name>, clean → rm .temp/examples/
```

### Gotchas

| Category | Issue | Solution |
|----------|-------|----------|
| API drift | TASK-003/004/005 не выполнены к моменту планирования; PRP пишет код против их публичного API из PRP-документов. | Examples должны соответствовать сигнатурам из соответствующих PRP. После выполнения TASK-003/004/005 (но до старта TASK-006) — выполнить sanity-check: `grep -E "func \(c \*Client\) (Login|Refresh|Logout|...)" lib/auth.go` и сравнить с PRP. Если API расходится — STOP и фикс. См. Task 0. |
| go.sum invariant | Любой случайный импорт сторонней библиотеки в examples → новый entry в `go.sum` → DOD breach. | Каждый файл в `lib/examples/*/main.go` использует **строго** stdlib (`context`, `errors`, `encoding/json`, `flag`, `fmt`, `log/slog`, `net`, `net/http`, `os`, `os/signal`, `syscall`, `time`) **+** `github.com/webdelo/auth-client-go/lib` (это сам SDK, локальный модуль — не идёт в go.sum) **+** `google.golang.org/grpc` (+ его subpackages: `credentials/insecure`, `metadata`, `health`, `health/grpc_health_v1`). Никаких `github.com/...` логгеров, никаких `cobra`/`viper`. Validation: `git diff go.sum` после `make build-examples`. |
| Build-but-not-run в CI | `make build-examples` в CI запускается БЕЗ AUTH-инфраструктуры; если бинарник пытается `client.New(...)` сразу → ошибка dial при запуске. | Examples с required-ENV: при отсутствии ENV → печатают usage в stderr и **exit 0**. CI запускает `./.temp/examples/basic` без ENV — должно вернуть 0. См. реализацию `mustEnv` ниже. NB: `grpc.NewClient` НЕ блокирует на dial (lazy connect), так что сам `New` не упадёт — но `Login` упадёт; и без ENV `Login` не вызывается. |
| slog ↔ Logger | SDK `Logger.Error(ctx, msg, err error, fields ...any)` имеет dedicated `err` param; `slog.ErrorContext(ctx, msg, args ...any)` — нет. | Adapter в `http_middleware/main.go` (inline): `Error` prepend'ит `"error", err` в args и зовёт `slog.ErrorContext`. См. реализацию `slogAdapter` ниже. Pure stdlib — no new deps. |
| Default openMethods полностью покрывает Health/{Check,Watch} | `Health/Check` И `Health/Watch` оба в default openMethods (`lib/client.go:248-253`). Подсказка из task spec («`Health/Check` пропускается, другой метод требует auth») неоднозначна — какой "другой метод"? | Использовать `Health/List` — это unary-метод стандартного `grpc.health.v1.Health` service'а, **не входит** в default openMethods, проходит через `UnaryServerInterceptor` нормально. Verified: `health.Server.List` реализован в grpc v1.79.3 ([health/server.go:73-87](file:///Users/a.popov/go/pkg/mod/google.golang.org/grpc@v1.79.3/health/server.go) — возвращает `HealthListResponse{Statuses: map[string]*HealthCheckResponse}` из внутреннего `statusMap`). См. alternatives-analysis Option A. |
| Go 1.22+ `http.ServeMux` pattern syntax | Examples используют `mux.HandleFunc("POST /auth/login", ...)` — метод-prefix в pattern (Go 1.22+). go.mod = `go 1.24.0` ⇒ безопасно. | Не использовать `mux.HandleFunc("/auth/login", ...)` (без метода) — `GET /auth/login` тогда тоже сматчится. Тулчейн ниже Go 1.22 не примет — но проект на 1.24, это безопасно. |
| `http_middleware/main.go` `/auth/logout` cookie path mismatch | Браузер сотрёт cookie только если `(Name, Path, Domain)` matchится. `SetRefreshCookie` ставит `Path=/auth/refresh` по умолчанию; `ClearRefreshCookie` ДОЛЖЕН использовать те же defaults. | Использовать `SetRefreshCookie(w, token)` без custom options, и `ClearRefreshCookie(w)` без custom options — defaults симметричны. (Если consumer перешёл бы на свой Path, ему пришлось бы повторять `WithCookiePath(...)` в обоих вызовах — это документируется в README, но для example достаточно defaults.) |
| `IPFromRequest` под `httptest` / `localhost` | В example LocalHost: `r.RemoteAddr = "127.0.0.1:PORT"`. `IPFromRequest` вернёт `127.0.0.1`. Реалистичный для demo. Нет XFF/X-Real-IP без proxy. | Не нужно фейкать headers — demo прозрачно показывает реальное поведение. README одной строкой говорит, что для production за nginx/cloudflare нужен trusted proxy + XFF. |
| `Login.ClientIP` в `basic/main.go` (нет `*http.Request`) | `basic` — CLI, нет HTTP-запроса, нечего передать в `IPFromRequest`. Передавать пустую строку нельзя (AUTH fingerprint станет `SHA-256(UA + "")` — слабо). | Литерал `"127.0.0.1"` + UA `"authclient-basic-example/0.1"` + комментарий «в production используйте `authclient.IPFromRequest(r)` из реального `*http.Request`». |
| `gosec` warnings про token в логах | `gosec G101` (hard-coded credentials) реагирует на любые literal "password"-подобные strings. Examples используют ENV → false positive риск минимален. | Никаких hardcoded паролей. Если `gosec` поднимет noise — добавить short-form комментарий в одной строке `// not a credential — env var name only`. |
| `revive` без godoc на main-package | `revive` требует package-doc. Каждый example начинается с `// Package main demonstrates ...`. | Package-doc в каждом `main.go`. Никаких exported идентификаторов в main-пакетах (все handler-функции — приватные). |
| `lll` 140 chars | Long type assertions / function signatures с auth_v1 типами могут вылезти. | Если пересечение неизбежно — переименовать локальные variable или разбить на 2 строки. NO `//nolint`. |
| Demo gRPC client в том же `main.go` | Поднять gRPC-сервер + сразу же сделать 3 client-call'а из одного процесса. | После `lis, _ := net.Listen("tcp", ":0"); go grpcSrv.Serve(lis)` — взять `lis.Addr().String()` и подключаться к нему `grpc.NewClient`. Используем ephemeral-port (`":0"`), чтобы не было конфликтов в CI. `defer grpcSrv.GracefulStop()` для cleanup. |
| `health.NewServer()` SetServingStatus | Если не вызвать `SetServingStatus("", SERVING)` → `Health/Check` вернёт `NOT_SERVING`. | Сразу после `health.NewServer()` — `srv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)`. |
| Build dependency: TASK-006 must run AFTER TASK-003/004/005 are merged | Без них examples не компилируются (импорты на несуществующие функции). | Task 0 верифицирует наличие `lib/auth.go`, `lib/validate.go`, `lib/middleware_http.go`, `lib/interceptor_grpc.go`, `lib/cookies.go`, `lib/helpers.go`, `lib/context.go`. Если чего-то нет — STOP. |

---

## Implementation

### Reused public API (no new types introduced)

Examples используют ТОЛЬКО следующие публичные идентификаторы из `github.com/webdelo/auth-client-go/lib` (псевдоним `authclient`):

```go
// From client.go (TASK-002, уже в коде)
authclient.New(cfg Config, opts ...Option) (*Client, error)
authclient.Config{Endpoint, Logger, FailMode, ValidateCacheTTL, ...}
authclient.WithFailMode(mode FailMode) Option
authclient.FailClosed, authclient.FailOpen  // FailMode constants
(c *Client).Close() error

// From logger.go (TASK-002, уже в коде)
authclient.Logger        // interface — Info/Warn/Error
// SDK NoopLogger как default — explicitly не используется в examples (мы передаём slogAdapter)

// From errors.go (TASK-002, уже в коде)
authclient.ErrInvalidToken, ErrExpired, ErrBlocked, ErrRateLimited, ErrUnavailable
authclient.RetryAfter(err error) time.Duration

// From context.go (TASK-003)
authclient.User{UUID, Role, Email}
authclient.UserFromContext(ctx) (User, bool)

// From validate.go (TASK-003)
(c *Client).ValidateAccessToken(ctx, accessToken string) (User, error)

// From middleware_http.go (TASK-003)
(c *Client).HTTPMiddleware(next http.Handler) http.Handler

// From interceptor_grpc.go (TASK-003)
(c *Client).UnaryInterceptor() grpc.UnaryServerInterceptor

// From auth.go (TASK-004)
authclient.LoginInput{Email, Password, ClientIP, UserAgent}
authclient.LoginOutput{AccessToken, RefreshToken, AccessExpiresIn, RefreshExpiresIn, User}
(c *Client).Login(ctx, LoginInput) (LoginOutput, error)
(c *Client).Logout(ctx, refreshToken string) error
// Refresh / LogoutAll / ChangePassword / GetCurrentUser НЕ используются в TASK-006 examples.

// From cookies.go (TASK-005)
authclient.SetRefreshCookie(w http.ResponseWriter, refreshToken string, opts ...CookieOption)
authclient.ClearRefreshCookie(w http.ResponseWriter, opts ...CookieOption)
// CookieOptions НЕ используются (defaults достаточны для demo).

// From helpers.go (TASK-005)
authclient.IPFromRequest(r *http.Request) string
authclient.UserAgentFromRequest(r *http.Request) string
```

Никаких internal-импортов (`lib/internal/cache`, `lib/internal/refresh`) — это проверяется `go list -deps`.

---

### Task 0 (pre-flight): Verify predecessor APIs are merged

**Action:** VERIFY
**File:** — (no code change, gate only)

**Description:** TASK-006 не может быть начат, пока TASK-003/004/005 не подняли публичный API. Перед старотом — проверить наличие файлов и ключевых функций.

**Commands:**

```bash
# 1. File existence:
for f in lib/context.go lib/validate.go lib/middleware_http.go lib/interceptor_grpc.go \
         lib/auth.go lib/cookies.go lib/helpers.go; do
    test -f "$f" || { echo "STOP: missing $f — TASK-003/004/005 not merged"; exit 1; }
done

# 2. Method signatures (matches code that examples generate):
grep -q "func (c \*Client) Login("                  lib/auth.go             || { echo "API: Login"; exit 1; }
grep -q "func (c \*Client) Logout("                 lib/auth.go             || { echo "API: Logout"; exit 1; }
grep -q "func (c \*Client) ValidateAccessToken("    lib/validate.go         || { echo "API: ValidateAccessToken"; exit 1; }
grep -q "func (c \*Client) HTTPMiddleware("         lib/middleware_http.go  || { echo "API: HTTPMiddleware"; exit 1; }
grep -q "func (c \*Client) UnaryInterceptor()"      lib/interceptor_grpc.go || { echo "API: UnaryInterceptor"; exit 1; }
grep -q "func SetRefreshCookie("                    lib/cookies.go          || { echo "API: SetRefreshCookie"; exit 1; }
grep -q "func ClearRefreshCookie("                  lib/cookies.go          || { echo "API: ClearRefreshCookie"; exit 1; }
grep -q "func IPFromRequest("                       lib/helpers.go          || { echo "API: IPFromRequest"; exit 1; }
grep -q "func UserAgentFromRequest("                lib/helpers.go          || { echo "API: UserAgentFromRequest"; exit 1; }
grep -q "func UserFromContext("                     lib/context.go          || { echo "API: UserFromContext"; exit 1; }

# 3. Struct field names that examples reference (BREAKS BUILD if renamed in TASK-004):
grep -Eq "Email\s+string"        lib/auth.go || { echo "API: LoginInput.Email"; exit 1; }
grep -Eq "Password\s+string"     lib/auth.go || { echo "API: LoginInput.Password"; exit 1; }
grep -Eq "ClientIP\s+string"     lib/auth.go || { echo "API: LoginInput.ClientIP"; exit 1; }
grep -Eq "UserAgent\s+string"    lib/auth.go || { echo "API: LoginInput.UserAgent"; exit 1; }
grep -Eq "AccessToken\s+string"  lib/auth.go || { echo "API: LoginOutput.AccessToken"; exit 1; }
grep -Eq "RefreshToken\s+string" lib/auth.go || { echo "API: LoginOutput.RefreshToken"; exit 1; }
grep -Eq "AccessExpiresIn\s+"    lib/auth.go || { echo "API: LoginOutput.AccessExpiresIn"; exit 1; }

# 4. User type fields that examples reference:
grep -Eq "UUID\s+string"  lib/context.go || { echo "API: User.UUID"; exit 1; }
grep -Eq "Role\s+Role"    lib/context.go || { echo "API: User.Role"; exit 1; }
```

Если что-то не сходится — STOP, эскалировать на TASK-003/004/005. Не подгонять examples под расхождение — сначала фикс upstream.

---

### Task 1: REPLACE `lib/examples/basic/main.go`

**Action:** REPLACE (overwrite existing 8-line placeholder)
**File:** [lib/examples/basic/main.go](../../lib/examples/basic/main.go)

**Description:** Минимальная CLI-демонстрация happy-path: `New → Login → ValidateAccessToken → Logout → Close`. ENV-driven config. Печатает каждый шаг и safe-truncated токены (первые 8 символов + `…`).

**Details:**

- Package: `main`. Один файл `main.go`.
- Package-doc:

  ```go
  // Package main is the basic authclient SDK example: it logs in with credentials
  // from environment variables, validates the issued access token, and logs out.
  // Run without any environment variables to see a usage line.
  package main
  ```

- Imports (всё stdlib + сам SDK):

  ```go
  import (
      "context"
      "errors"
      "fmt"
      "os"
      "time"

      authclient "github.com/webdelo/auth-client-go/lib"
  )
  ```

- Структура `main()`:

  1. `endpoint, email, password, ok := readEnv()`. Если `!ok` → `usage()` в stderr → `os.Exit(0)`.
  2. `ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()`.
  3. `client, err := authclient.New(authclient.Config{Endpoint: endpoint})`. Ошибка → stderr, `os.Exit(1)`.
  4. `defer client.Close()`.
  5. Print `"step 1: New ok (endpoint=%s)"`.
  6. `out, err := client.Login(ctx, authclient.LoginInput{Email: email, Password: password, ClientIP: "127.0.0.1", UserAgent: "authclient-basic-example/0.1"})`.
     - Ошибка → определить тип через `errors.Is(err, authclient.ErrInvalidToken)` / `ErrRateLimited` (для retry-after) / fallthrough → stderr + exit 1.
     - Успех → print `"step 2: Login ok user=%s role=%s access=%s…"` где `access = out.AccessToken[:8]`.
  7. `user, err := client.ValidateAccessToken(ctx, out.AccessToken)`.
     - Ошибка → stderr + exit 1.
     - Print `"step 3: Validate ok user.uuid=%s user.role=%s"` (note: `user.Email == ""` после ValidateAccessToken — это OK, документировано в proto v0.1.0).
  8. `if err := client.Logout(ctx, out.RefreshToken); err != nil { ... }`.
     - Print `"step 4: Logout ok"`.
  9. Print `"step 5: Close ok (deferred)"`.

- `readEnv() (endpoint, email, password string, ok bool)`:

  ```go
  func readEnv() (endpoint, email, password string, ok bool) {
      endpoint = os.Getenv("AUTH_ENDPOINT")
      email    = os.Getenv("AUTH_TEST_EMAIL")
      password = os.Getenv("AUTH_TEST_PASSWORD")
      ok = endpoint != "" && email != "" && password != ""
      return
  }
  ```

- `usage()`:

  ```go
  func usage() {
      fmt.Fprintln(os.Stderr, "usage: AUTH_ENDPOINT=host:port AUTH_TEST_EMAIL=... AUTH_TEST_PASSWORD=... basic")
      fmt.Fprintln(os.Stderr, "see lib/examples/basic/README.md")
  }
  ```

- `safeTrunc(s string, n int) string` — local helper, returns `s[:min(n,len(s))] + "…"`. Используется для безопасного логирования токенов.

- **Errors handling pattern:**

  ```go
  if errors.Is(err, authclient.ErrRateLimited) {
      fmt.Fprintf(os.Stderr, "rate limited, retry after %s\n", authclient.RetryAfter(err))
      os.Exit(1)
  }
  if err != nil {
      fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
      os.Exit(1)
  }
  ```

**Code skeleton:**

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "os"
    "time"

    authclient "github.com/webdelo/auth-client-go/lib"
)

func main() {
    endpoint, email, password, ok := readEnv()
    if !ok {
        usage()
        os.Exit(0)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    client, err := authclient.New(authclient.Config{Endpoint: endpoint})
    if err != nil {
        fmt.Fprintf(os.Stderr, "authclient.New: %v\n", err)
        os.Exit(1)
    }
    defer client.Close()
    fmt.Printf("step 1: New ok (endpoint=%s)\n", endpoint)

    out, err := client.Login(ctx, authclient.LoginInput{
        Email:     email,
        Password:  password,
        ClientIP:  "127.0.0.1",                       // demo only; use authclient.IPFromRequest(r) in real HTTP handler
        UserAgent: "authclient-basic-example/0.1",
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "Login: %v (retry-after=%s)\n", err, authclient.RetryAfter(err))
        if errors.Is(err, authclient.ErrInvalidToken) {
            fmt.Fprintln(os.Stderr, "wrong email or password")
        }
        os.Exit(1)
    }
    fmt.Printf("step 2: Login ok user=%s role=%s access=%s\n",
        out.User.UUID, out.User.Role, safeTrunc(out.AccessToken, 8))

    user, err := client.ValidateAccessToken(ctx, out.AccessToken)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ValidateAccessToken: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("step 3: Validate ok user.uuid=%s user.role=%s\n", user.UUID, user.Role)

    if err := client.Logout(ctx, out.RefreshToken); err != nil {
        fmt.Fprintf(os.Stderr, "Logout: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("step 4: Logout ok")
    fmt.Println("step 5: Close ok (deferred)")
}

func readEnv() (endpoint, email, password string, ok bool) { /* ... */ }
func usage() { /* ... */ }
func safeTrunc(s string, n int) string {
    if n > len(s) {
        n = len(s)
    }
    return s[:n] + "…"
}
```

---

### Task 2: CREATE `lib/examples/basic/README.md`

**Action:** CREATE
**File:** [lib/examples/basic/README.md](../../lib/examples/basic/README.md)

**Description:** Менее 50 строк. Формат `What / Run / Expected output`.

**Content shape:**

```markdown
# Example: basic

## What

Minimal happy-path: `New → Login → ValidateAccessToken → Logout → Close`.
Demonstrates the four core operations every SDK consumer eventually calls.
ENV-driven; no flags.

## Prerequisites

- A running AUTH gRPC server (see [webdelo/auth](https://github.com/webdelo/auth)
  or run `make env-up` in this repo to spin one up via docker-compose).
- A test user account in that AUTH instance.

## Run

    export AUTH_ENDPOINT=localhost:50051
    export AUTH_TEST_EMAIL=test@example.com
    export AUTH_TEST_PASSWORD=secret123
    go run ./lib/examples/basic

Without environment variables, the binary prints a usage line and exits 0.

## Expected output

    step 1: New ok (endpoint=localhost:50051)
    step 2: Login ok user=<uuid> role=user access=eyJhbGci…
    step 3: Validate ok user.uuid=<uuid> user.role=user
    step 4: Logout ok
    step 5: Close ok (deferred)

Tokens are truncated to 8 chars + ellipsis for safety.

## Notes

- The example passes `ClientIP="127.0.0.1"` and a literal User-Agent. In a
  real HTTP-facing service, use `authclient.IPFromRequest(r)` and
  `authclient.UserAgentFromRequest(r)` from a `*http.Request`.
- `User.Email` is empty after `ValidateAccessToken` (proto v0.1.0 limitation).
```

---

### Task 3: CREATE `lib/examples/http_middleware/main.go`

**Action:** CREATE
**File:** [lib/examples/http_middleware/main.go](../../lib/examples/http_middleware/main.go)

**Description:** Один-`main.go` HTTP-сервер на `net/http`. Три эндпоинта:
- `POST /auth/login` — без middleware, читает JSON `{email, password}`, вызывает `client.Login`, ставит refresh-cookie через `SetRefreshCookie`, возвращает JSON `{access_token, expires_in_seconds, user: {uuid, role, email}}`.
- `POST /auth/logout` — без middleware, читает cookie `refresh_token`, вызывает `client.Logout`, `ClearRefreshCookie`, возвращает `204 No Content`.
- `GET /api/profile` — обёрнут `client.HTTPMiddleware`, читает `UserFromContext(r.Context())`, возвращает JSON `{uuid, role}`.

Логирование через inline `slogAdapter`-обёртку над `slog.Default()`. Graceful shutdown через `signal.NotifyContext`.

**Details:**

- Package-doc:

  ```go
  // Package main is the http_middleware authclient SDK example: it wires
  // client.HTTPMiddleware onto a protected /api/profile endpoint, exposes
  // /auth/login (sets HttpOnly refresh cookie) and /auth/logout (clears it).
  // Demonstrates slog → authclient.Logger adapter and IPFromRequest /
  // UserAgentFromRequest fingerprint extraction.
  package main
  ```

- Imports (всё stdlib + SDK):

  ```go
  import (
      "context"
      "encoding/json"
      "errors"
      "fmt"
      "log/slog"
      "net/http"
      "os"
      "os/signal"
      "syscall"
      "time"

      authclient "github.com/webdelo/auth-client-go/lib"
  )
  ```

- `slogAdapter` (inline, реализует `authclient.Logger`):

  ```go
  type slogAdapter struct{ l *slog.Logger }

  func (a slogAdapter) Info(ctx context.Context, msg string, fields ...any) {
      a.l.InfoContext(ctx, msg, fields...)
  }
  func (a slogAdapter) Warn(ctx context.Context, msg string, fields ...any) {
      a.l.WarnContext(ctx, msg, fields...)
  }
  func (a slogAdapter) Error(ctx context.Context, msg string, err error, fields ...any) {
      args := make([]any, 0, len(fields)+2)
      if err != nil {
          args = append(args, "error", err)
      }
      args = append(args, fields...)
      a.l.ErrorContext(ctx, msg, args...)
  }
  ```

  Compile-time check: `var _ authclient.Logger = slogAdapter{}`.

- `main()` структура:

  1. `endpoint := os.Getenv("AUTH_ENDPOINT")`. Empty → usage stderr → `os.Exit(0)`.
  2. `addr := getenvOr("HTTP_ADDR", ":8080")`.
  3. `logger := slog.New(slog.NewTextHandler(os.Stderr, nil))`.
  4. `client, err := authclient.New(authclient.Config{Endpoint: endpoint, Logger: slogAdapter{logger}})`. Ошибка → stderr + exit 1.
  5. `defer client.Close()`.
  6. `mux := http.NewServeMux()`.
  7. `mux.HandleFunc("POST /auth/login", loginHandler(client))`.
  8. `mux.HandleFunc("POST /auth/logout", logoutHandler(client))`.
  9. `mux.Handle("GET /api/profile", client.HTTPMiddleware(http.HandlerFunc(profileHandler)))`.
  10. `srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}`.
  11. `ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM); defer stop()`.
  12. Запустить `srv.ListenAndServe()` в goroutine; ждать `<-ctx.Done()`; `srv.Shutdown(shutCtx с 5s timeout)`.

- `loginHandler(c *authclient.Client) http.HandlerFunc`:

  ```go
  return func(w http.ResponseWriter, r *http.Request) {
      var req struct {
          Email    string `json:"email"`
          Password string `json:"password"`
      }
      if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
          http.Error(w, "bad json", http.StatusBadRequest)
          return
      }
      out, err := c.Login(r.Context(), authclient.LoginInput{
          Email:     req.Email,
          Password:  req.Password,
          ClientIP:  authclient.IPFromRequest(r),
          UserAgent: authclient.UserAgentFromRequest(r),
      })
      switch {
      case errors.Is(err, authclient.ErrInvalidToken):
          http.Error(w, "invalid credentials", http.StatusUnauthorized)
          return
      case errors.Is(err, authclient.ErrRateLimited):
          if d := authclient.RetryAfter(err); d > 0 {
              w.Header().Set("Retry-After", fmt.Sprintf("%d", int(d.Seconds())))
          }
          http.Error(w, "rate limited", http.StatusTooManyRequests)
          return
      case errors.Is(err, authclient.ErrUnavailable):
          http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
          return
      case err != nil:
          http.Error(w, err.Error(), http.StatusInternalServerError)
          return
      }
      authclient.SetRefreshCookie(w, out.RefreshToken)
      writeJSON(w, http.StatusOK, map[string]any{
          "access_token":       out.AccessToken,
          "expires_in_seconds": int(out.AccessExpiresIn.Seconds()),
          "user": map[string]string{
              "uuid":  out.User.UUID,
              "role":  string(out.User.Role),
              "email": out.User.Email,
          },
      })
  }
  ```

- `logoutHandler(c *authclient.Client) http.HandlerFunc`:

  ```go
  return func(w http.ResponseWriter, r *http.Request) {
      cookie, err := r.Cookie("refresh_token")
      if err != nil {
          http.Error(w, "no refresh cookie", http.StatusBadRequest)
          return
      }
      if err := c.Logout(r.Context(), cookie.Value); err != nil &&
          !errors.Is(err, authclient.ErrInvalidToken) { // already-invalidated cookie — idempotent
          http.Error(w, err.Error(), http.StatusInternalServerError)
          return
      }
      authclient.ClearRefreshCookie(w)
      w.WriteHeader(http.StatusNoContent)
  }
  ```

- `profileHandler(w, r)`:

  ```go
  user, ok := authclient.UserFromContext(r.Context())
  if !ok {
      http.Error(w, "no user", http.StatusInternalServerError) // unreachable when middleware is wired
      return
  }
  writeJSON(w, http.StatusOK, map[string]string{
      "uuid": user.UUID,
      "role": string(user.Role),
  })
  ```

- `writeJSON(w http.ResponseWriter, status int, v any)` — local helper sets `Content-Type: application/json` and encodes.

- `getenvOr(key, def string) string` — `if v := os.Getenv(key); v != "" { return v } return def`.

- `usage()` идентичен паттерну из basic.

---

### Task 4: CREATE `lib/examples/http_middleware/README.md`

**Action:** CREATE
**File:** [lib/examples/http_middleware/README.md](../../lib/examples/http_middleware/README.md)

**Content shape:**

```markdown
# Example: http_middleware

## What

Single-file `net/http` server with:
- `POST /auth/login` — JSON in, sets HttpOnly refresh cookie, returns access token.
- `POST /auth/logout` — clears the refresh cookie via `client.Logout`.
- `GET  /api/profile` — protected by `client.HTTPMiddleware`; reads `UserFromContext`.

Shows the `slog` → `authclient.Logger` adapter pattern and the
`IPFromRequest` / `UserAgentFromRequest` fingerprint helpers.

## Prerequisites

A reachable AUTH endpoint (`AUTH_ENDPOINT=host:port`).

## Run

    export AUTH_ENDPOINT=localhost:50051
    go run ./lib/examples/http_middleware           # listens on :8080

    # Login (replace with a real test account):
    curl -sS -X POST http://localhost:8080/auth/login \
        -H "Content-Type: application/json" \
        -c cookies.txt \
        -d '{"email":"test@example.com","password":"secret123"}'

    # Access protected route:
    ACCESS=$(jq -r .access_token < login-response.json)
    curl -sS http://localhost:8080/api/profile -H "Authorization: Bearer $ACCESS"

    # Logout:
    curl -sS -X POST http://localhost:8080/auth/logout -b cookies.txt

Without `AUTH_ENDPOINT`, the binary prints a usage line and exits 0.

## Expected output

`/auth/login` returns:
    {"access_token":"...", "expires_in_seconds": 900, "user":{"uuid":"...","role":"user","email":"..."}}

`/api/profile` returns:
    {"uuid":"...","role":"user"}

`/auth/logout` returns `204 No Content` and sends `Set-Cookie` with `MaxAge=-1`.

## Notes

- `IPFromRequest` may be spoofed without a trusted proxy. In production, put
  this service behind nginx/Cloudflare and configure XFF correctly.
- `SetRefreshCookie` sets `Path=/auth/refresh`. The browser scopes the cookie
  to that path — so the cookie is sent to `/auth/logout` only if you keep
  defaults (this example does).
```

---

### Task 5: CREATE `lib/examples/grpc_interceptor/main.go`

**Action:** CREATE
**File:** [lib/examples/grpc_interceptor/main.go](../../lib/examples/grpc_interceptor/main.go)

**Description:** Один-`main.go`: поднимает gRPC-сервер с `client.UnaryInterceptor()`, регистрирует стандартный `health.NewServer()`, а затем встроенный demo-client делает 3 RPC и завершает процесс с exit code 0 (или ≠0 если что-то не так).

- Skipped (default openMethods): `/grpc.health.v1.Health/Check` → SERVING.
- Protected: `/grpc.health.v1.Health/List`.

**Details:**

- Package-doc:

  ```go
  // Package main is the grpc_interceptor authclient SDK example: it starts a
  // gRPC server protected by client.UnaryInterceptor() and exercises three
  // RPCs from an in-process client:
  //   1. /grpc.health.v1.Health/Check without metadata    — skipped (default openMethods) → SERVING
  //   2. /grpc.health.v1.Health/List with a valid Bearer  — interceptor accepts → SERVING
  //   3. /grpc.health.v1.Health/List without metadata     — interceptor returns codes.Unauthenticated
  // Run without AUTH_ENDPOINT/AUTH_TEST_EMAIL/AUTH_TEST_PASSWORD to see a usage
  // line and exit 0.
  package main
  ```

- Imports:

  ```go
  import (
      "context"
      "fmt"
      "net"
      "os"
      "time"

      authclient "github.com/webdelo/auth-client-go/lib"
      "google.golang.org/grpc"
      "google.golang.org/grpc/codes"
      "google.golang.org/grpc/credentials/insecure"
      "google.golang.org/grpc/health"
      "google.golang.org/grpc/health/grpc_health_v1"
      "google.golang.org/grpc/metadata"
      "google.golang.org/grpc/status"
  )
  ```

  Все imports — внутри grpc module v1.79.3, который уже в go.mod. **0 новых go.sum entries.**

- `main()` структура:

  1. `endpoint, email, password, ok := readEnv()`. Empty → usage stderr → `os.Exit(0)`.
  2. Init SDK: `client, err := authclient.New(authclient.Config{Endpoint: endpoint}); defer client.Close()`.
  3. Login для получения valid access-токена:

     ```go
     loginCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
     defer cancel()
     out, err := client.Login(loginCtx, authclient.LoginInput{
         Email: email, Password: password,
         ClientIP: "127.0.0.1", UserAgent: "authclient-grpc-example/0.1",
     })
     // exit on error
     ```

  4. Start gRPC server:

     ```go
     lis, err := net.Listen("tcp", "127.0.0.1:0")   // ephemeral port, CI-friendly
     // exit on error

     grpcSrv := grpc.NewServer(grpc.UnaryInterceptor(client.UnaryInterceptor()))
     healthSrv := health.NewServer()
     healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
     grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)

     go func() { _ = grpcSrv.Serve(lis) }()
     defer grpcSrv.GracefulStop()
     ```

  5. Connect demo client:

     ```go
     conn, err := grpc.NewClient(lis.Addr().String(),
         grpc.WithTransportCredentials(insecure.NewCredentials()))
     // exit on error
     defer conn.Close()
     hc := grpc_health_v1.NewHealthClient(conn)
     ```

  6. **RPC #1** — `Check`, no auth → должно вернуть SERVING:

     ```go
     rpcCtx, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
     defer cancel1()
     resp, err := hc.Check(rpcCtx, &grpc_health_v1.HealthCheckRequest{})
     if err != nil {
         fmt.Fprintf(os.Stderr, "RPC1 Health/Check failed (expected success): %v\n", err)
         os.Exit(1)
     }
     fmt.Printf("RPC1 Health/Check (no auth, default openMethod) → %s\n", resp.GetStatus())
     ```

  7. **RPC #2** — `List`, valid token → ok:

     ```go
     md := metadata.Pairs("authorization", "Bearer "+out.AccessToken)
     ctx2 := metadata.NewOutgoingContext(rpcCtx, md)
     resp2, err := hc.List(ctx2, &grpc_health_v1.HealthListRequest{})
     if err != nil {
         fmt.Fprintf(os.Stderr, "RPC2 Health/List with token failed (expected success): %v\n", err)
         os.Exit(1)
     }
     fmt.Printf("RPC2 Health/List (with Bearer) → %d statuses\n", len(resp2.GetStatuses()))
     ```

  8. **RPC #3** — `List`, no metadata → expect `codes.Unauthenticated`:

     ```go
     _, err = hc.List(rpcCtx, &grpc_health_v1.HealthListRequest{})
     st, ok := status.FromError(err)
     if !ok || st.Code() != codes.Unauthenticated {
         fmt.Fprintf(os.Stderr, "RPC3 Health/List without token: expected Unauthenticated, got %v\n", err)
         os.Exit(1)
     }
     fmt.Printf("RPC3 Health/List (no metadata) → %s (as expected)\n", st.Code())
     ```

  9. Logout for cleanup: `_ = client.Logout(ctx, out.RefreshToken)`. Не критично если ошибка.
  10. `fmt.Println("all 3 RPCs passed")`.

- `readEnv()` / `usage()` — копии паттернов из basic.

**Note on `Health/List` shape:** The standard `grpc_health_v1.HealthListResponse` has a `map<string, HealthCheckResponse> statuses` field. `health.NewServer()` populates it for all services registered via `SetServingStatus`. Default has `""` (empty service name) → so `Statuses[""]` returns SERVING. The example just logs `len(resp.GetStatuses())` to confirm we got a non-error response.

---

### Task 6: CREATE `lib/examples/grpc_interceptor/README.md`

**Action:** CREATE
**File:** [lib/examples/grpc_interceptor/README.md](../../lib/examples/grpc_interceptor/README.md)

**Content shape:**

```markdown
# Example: grpc_interceptor

## What

Single-process gRPC server + client demo of `client.UnaryInterceptor()`:

- Server registers `health.NewServer()` (stdlib of `google.golang.org/grpc`)
  and wires the SDK's unary interceptor.
- An in-process client fires three RPCs and the program exits 0 on success.

`/grpc.health.v1.Health/Check` and `/Watch` are in the SDK's default open-methods
registry (no auth needed). `/grpc.health.v1.Health/List` is **not** in that
registry, so the interceptor enforces a valid Bearer token in gRPC metadata.

## Prerequisites

- A reachable AUTH endpoint and a test user (the example calls `client.Login`
  to obtain a real access token, then uses it in metadata).

## Run

    export AUTH_ENDPOINT=localhost:50051
    export AUTH_TEST_EMAIL=test@example.com
    export AUTH_TEST_PASSWORD=secret123
    go run ./lib/examples/grpc_interceptor

Without env vars, the binary prints a usage line and exits 0.

## Expected output

    RPC1 Health/Check (no auth, default openMethod) → SERVING
    RPC2 Health/List (with Bearer) → 1 statuses
    RPC3 Health/List (no metadata) → Unauthenticated (as expected)
    all 3 RPCs passed

## Notes

- `Health/List` is used as the demo "protected" method because `Check` and
  `Watch` are unconditionally open in the SDK's default registry. To add your
  own open methods, pass `authclient.WithOpenMethods("/your.Svc/Method")` to
  `New`.
- The example listens on an ephemeral port (`127.0.0.1:0`) — multiple copies
  can run in parallel without conflicts.
```

---

### Task 7: UPDATE `Makefile` — `build-examples` and `clean`

**Action:** UPDATE
**File:** [Makefile](../../Makefile)

**Description:** Перенаправить вывод `build-examples` в `.temp/examples/<name>/` (не `bin/example-<name>`). Обновить `clean` чтобы тоже подчищал `.temp/examples/`.

**Diff target:**

```makefile
# BEFORE (lines 80-88):
build-examples: ## Build example programs
	@echo "Building examples..."
	@mkdir -p bin
	@for dir in lib/examples/*/; do \
		name=$$(basename $$dir); \
		echo "  Building example-$$name..."; \
		go build -o bin/example-$$name ./$$dir; \
	done
	@echo "✅ Examples built in bin/"

# AFTER:
build-examples: ## Build example programs
	@echo "Building examples..."
	@mkdir -p .temp/examples
	@for dir in lib/examples/*/; do \
		name=$$(basename $$dir); \
		echo "  Building $$name..."; \
		go build -o .temp/examples/$$name ./$$dir; \
	done
	@echo "✅ Examples built in .temp/examples/"
```

```makefile
# BEFORE (clean):
clean: ## Clean build artifacts
	rm -rf $(COVERAGE_DIR) bin/

# AFTER:
clean: ## Clean build artifacts
	rm -rf $(COVERAGE_DIR) bin/ .temp/examples/
```

(`.temp/` already in `.gitignore` — no change there.)

**Sanity check:** `make build-examples && ls .temp/examples/` should print `basic  grpc_interceptor  http_middleware`.

---

## Integration

### Database
N/A — examples не используют БД.

### Config
N/A — examples управляются ENV-переменными (`AUTH_ENDPOINT`, `AUTH_TEST_EMAIL`, `AUTH_TEST_PASSWORD`, `HTTP_ADDR`).

### Dependencies

| Package | Purpose | New in go.sum? |
|---|---|---|
| `context`, `encoding/json`, `errors`, `flag`, `fmt`, `log/slog`, `net`, `net/http`, `os`, `os/signal`, `syscall`, `time` | stdlib — example code | No (stdlib) |
| `github.com/webdelo/auth-client-go/lib` (= `authclient`) | сам SDK | No (own module) |
| `google.golang.org/grpc` | gRPC server/client in `grpc_interceptor` | Already in `go.sum` |
| `google.golang.org/grpc/credentials/insecure` | insecure dial для in-process demo client | Subpackage — no new entry |
| `google.golang.org/grpc/codes`, `.../status` | RPC status в expectation checks | Subpackage — no new entry |
| `google.golang.org/grpc/metadata` | client-side `authorization` header | Subpackage — no new entry |
| `google.golang.org/grpc/health` | `health.NewServer()` | Subpackage — no new entry |
| `google.golang.org/grpc/health/grpc_health_v1` | `HealthCheckRequest`/`Response`/`HealthListRequest`/`Response`, `RegisterHealthServer`, `NewHealthClient` | Subpackage — no new entry |

**Invariant**: `git diff go.sum` after `make tidy && make build-examples` must be empty. CI должен проверять.

---

## Testing

### Unit Tests

`main`-пакеты в Go не покрываются unit-тестами стандартными способом — нет экспортируемых идентификаторов. Тесты для `lib/examples/*` НЕ требуются и НЕ создаются.

**Compile coverage** в роли «теста»: `make build-examples` — если код не компилируется, build падает. Этого достаточно.

### Integration Tests

| Endpoint / Scenario | Test File | Cases |
|---|---|---|
| `/auth/login` + `/api/profile` E2E через HTTP | — (handled by manual run + future CI smoke) | Не пишем для v0.1.0 — TASK-006 не требует, см. DOD |
| `Health/Check` (open) + `Health/List` (protected) | — | Демонстрация — встроена в `grpc_interceptor/main.go` (self-test) |

Examples сами выполняют end-to-end self-test при запуске (basic делает Login+Validate+Logout, grpc_interceptor делает 3 RPC и проверяет codes). Это de-facto smoke-test, который CI может запустить с реальным AUTH-инстансом.

**CI smoke test (рекомендация, не обязательная DOD item):** в будущем `make test-integration` может запустить bin'ы из `.temp/examples/` против реального AUTH через `make env-up`. Сейчас вне scope TASK-006.

### Mocks

Не используются. Examples ходят к реальному (для consumer) сервису или показывают сам факт wiring.

---

## Validation

### Build & Lint

```bash
# 1. Predecessor APIs are merged (Task 0 gate):
bash -c '
test -f lib/auth.go &&
test -f lib/cookies.go &&
test -f lib/helpers.go &&
test -f lib/context.go &&
test -f lib/middleware_http.go &&
test -f lib/interceptor_grpc.go &&
test -f lib/validate.go
' || echo "missing predecessor file — STOP"

# 2. Build all examples:
make build-examples
ls -la .temp/examples/   # → basic  grpc_interceptor  http_middleware

# 3. Sanity-run each binary without ENV (should print usage, exit 0):
.temp/examples/basic            ; echo "exit=$?"   # exit=0
.temp/examples/http_middleware  ; echo "exit=$?"   # exit=0
.temp/examples/grpc_interceptor ; echo "exit=$?"   # exit=0

# 4. go.sum invariant:
git diff --exit-code go.sum                       # zero diff

# 5. No internal/ imports in examples:
go list -deps ./lib/examples/... | grep -F "/lib/internal" | wc -l   # == 0

# 6. Full audit:
make audit                                          # vet + lint + test pass
```

### Manual Checks

1. **README sanity**: open each README in a markdown viewer; confirm < 50 lines, sections What / Prerequisites / Run / Expected output / Notes are present, code blocks have language specifier.
2. **basic против real AUTH** (если есть локальный AUTH): `make env-up` → set ENV → `go run ./lib/examples/basic` → see 5 lines.
3. **http_middleware вручную**: `go run ./lib/examples/http_middleware` → curl `POST /auth/login` → see `Set-Cookie: refresh_token=...; Path=/auth/refresh; HttpOnly; Secure; SameSite=Strict` header.
4. **grpc_interceptor self-test**: `go run ./lib/examples/grpc_interceptor` → see 4 lines of output ending with `all 3 RPCs passed`.

---

## DOD (Definition of Done)

Соответствует TASK-006 «Definition of Done» 1:1:

- [ ] `lib/examples/basic/main.go` собирается, демонстрирует Login → Validate → Logout flow.
- [ ] `lib/examples/http_middleware/main.go` собирается, имеет 3 endpoint'а (`/api/profile` защищённый, `/auth/login`, `/auth/logout`).
- [ ] `lib/examples/grpc_interceptor/main.go` собирается, использует `grpc.UnaryInterceptor(client.UnaryInterceptor())`, демонстрирует skip `Health/Check`, авторизацию `Health/List`.
- [ ] Каждый пример имеет README.md (< 50 строк, What/Run/Expected output).
- [ ] `make build-examples` собирает все три в `.temp/examples/<name>` без warnings.
- [ ] Старый placeholder `lib/examples/basic/main.go` заменён.
- [ ] Examples используют только публичный API SDK (`go list -deps ./lib/examples/... | grep internal | wc -l == 0`).
- [ ] Examples не добавляют сторонних deps (`git diff --exit-code go.sum` пуст).
- [ ] `make audit` (vet + lint + test) проходит.
- [ ] Каждый бинарник без аргументов / без ENV → печатает usage в stderr и exit 0.

---

## Notes for the executor (post-PRP)

- **STOP after PRP generation. Do NOT start execution.** Wait for user approval.
- After approval and execution, run `python3 .claude/scripts/req_refresh.py REQ-001 --fix-duplicates` to sync statuses.
- When predecessor tasks (TASK-003/004/005) are merged but their public API drifts from what's documented in this PRP (e.g., they rename `LoginInput` → `LoginParams`), update Task 1/3/5 accordingly **before** writing code — don't make examples match a stale PRP.
