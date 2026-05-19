# REQ-001-TASK-006 — Result Report

## Metadata

| Field | Value |
|-------|-------|
| Task ID | REQ-001-TASK-006 |
| Title | Examples — basic, http_middleware, grpc_interceptor |
| Parent REQ | [REQ-001](../tasks/REQ-001.md) |
| PRP | [REQ-001-TASK-006-PRP.md](../plans/REQ-001-TASK-006-PRP.md) |
| Code Review | [REQ-001-TASK-006-CODE-REVIEW.yaml](../reviews/REQ-001-TASK-006-CODE-REVIEW.yaml) |
| Branch | `feature/req-001-task-006` |
| Date | 2026-05-19 |
| Verdict | ALL_FIXED (orig APPROVED_WITH_COMMENTS — 0 critical, 2 warning, 1 suggestion; все исправлены) |

---

## Summary

Созданы три рабочих `main`-пакета в `lib/examples/`, каждый — минималистичная демонстрация одной фичи SDK через публичный API. `basic/` — CLI happy-path: `New → Login → ValidateAccessToken → Logout → Close`, ENV-driven (`AUTH_ENDPOINT`, `AUTH_TEST_EMAIL`, `AUTH_TEST_PASSWORD`), safe-truncated токены. `http_middleware/` — `net/http` сервер на `:8080` с тремя эндпоинтами (`POST /auth/login` — `SetRefreshCookie` + JSON `{access_token,...}`, `POST /auth/logout` — `ClearRefreshCookie`, `GET /api/profile` — обёрнут `client.HTTPMiddleware`, читает `UserFromContext`), inline `slogAdapter` (compile-time `var _ authclient.Logger = slogAdapter{}`), graceful shutdown через `signal.NotifyContext` + `srv.Shutdown` с 5s timeout. `grpc_interceptor/` — gRPC сервер на ephemeral port с `grpc.UnaryInterceptor(client.UnaryInterceptor())` + `health.NewServer()`, embedded demo-client отправляет 3 RPC: `Health/Check` без auth → SERVING (default openMethod), `Health/List` с Bearer → success, `Health/List` без metadata → `codes.Unauthenticated`. Каждый бинарник без ENV печатает usage в stderr и завершается с exit 0 (CI: build success ≠ run success). Makefile `build-examples` обновлён на вывод в `.temp/examples/<name>/`, `clean` дополнен `.temp/examples/`. `go.sum` invariant: examples используют только stdlib + сам SDK + grpc subpackages (`codes`, `credentials/insecure`, `health`, `health/grpc_health_v1`, `metadata`, `status`) — все уже в go.mod, ноль новых записей в go.sum. Прямые internal-импорты отсутствуют (`grep` lib/examples/*/main.go = 0). После code-review исправлены 2 WARNING (CS-0010 missing blank line после if-guard в 3 файлах, CS-0002 naked return в `readEnv` × 2) и 1 SUGGESTION (CS-0010 в `safeTrunc`); `make audit` чист: vet 0, lint 0, 214/214 tests pass с race detector.

---

## Files Changed

### New (examples)

| Path | Lines | Purpose |
|------|-------|---------|
| `lib/examples/basic/main.go` | 99 | CLI happy-path: New → Login → ValidateAccessToken → Logout → Close; ENV-driven; `safeTrunc` для безопасного логирования токенов; `main()` делает только `os.Exit(run())` для корректной работы defer (gocritic exitAfterDefer) |
| `lib/examples/basic/README.md` | 43 | What / Prerequisites / Run / Expected output / Notes |
| `lib/examples/http_middleware/main.go` | 204 | HTTP сервер с 3 endpoints; inline `slogAdapter` (Logger interface); graceful shutdown через `signal.NotifyContext` + buffered serverErr channel; idempotent logout (swallows ErrInvalidToken) |
| `lib/examples/http_middleware/README.md` | 46 | curl-snippets для login/profile/logout, cookie path-scoping caveat |
| `lib/examples/grpc_interceptor/main.go` | 146 | gRPC server + in-process client; `health.NewServer()` + `SetServingStatus("", SERVING)`; 3 self-test RPCs; ephemeral port `127.0.0.1:0` для CI-friendly |
| `lib/examples/grpc_interceptor/README.md` | 46 | Run + expected output (4 строки), Notes про default openMethods и `WithOpenMethods` |

### Updated

| Path | Изменение |
|------|-----------|
| `Makefile` | `build-examples`: `bin/example-<name>` → `.temp/examples/<name>` (полный путь без `example-` префикса); `clean`: добавлен `.temp/examples/` к списку удаления |

### Replaced

| Path | До → После |
|------|-----------|
| `lib/examples/basic/main.go` | 8-строчный placeholder (`fmt.Println("authclient SDK — see examples/basic/main.go in TASK-006")`) → полноценный example |

### Auto-formatted (out of scope, gofmt/goimports fallout)

| Path | Изменение |
|------|-----------|
| `lib/auth_test.go` | Import reorder (3rd-party блок) |
| `lib/internal/refresh/group_test.go` | Whitespace в struct field |

---

## API Surface Used

Examples используют ТОЛЬКО следующий публичный API SDK через прямой импорт `authclient "github.com/webdelo/auth-client-go/lib"`:

```
authclient.New + Config{Endpoint, Logger}
(*Client).Close() error
(*Client).Login(ctx, LoginInput) (LoginOutput, error)
(*Client).Logout(ctx, refreshToken) error
(*Client).ValidateAccessToken(ctx, accessToken) (User, error)
(*Client).HTTPMiddleware(next http.Handler) http.Handler
(*Client).UnaryInterceptor() grpc.UnaryServerInterceptor
authclient.User{UUID, Role, Email}, authclient.Role
authclient.UserFromContext(ctx) (User, bool)
authclient.LoginInput{Email, Password, ClientIP, UserAgent}
authclient.LoginOutput{AccessToken, RefreshToken, AccessExpiresIn, RefreshExpiresIn, User}
authclient.SetRefreshCookie(w, refreshToken)
authclient.ClearRefreshCookie(w)
authclient.IPFromRequest(r) / UserAgentFromRequest(r)
authclient.Logger (interface — реализуется slogAdapter в http_middleware)
authclient.ErrInvalidToken, ErrRateLimited, ErrUnavailable; RetryAfter(err)
```

Прямые internal-импорты: 0 (проверено `grep -E '"github.com/webdelo/auth-client-go/lib/internal' lib/examples/*/main.go`).

---

## Tests

`main`-пакеты в Go не покрываются unit-тестами стандартными способами — нет экспортируемых идентификаторов. Тесты для `lib/examples/*` намеренно не создаются. Compile coverage и self-test execution выполняют роль теста:

- `make build-examples` — все 3 бинарника собираются в `.temp/examples/`.
- Каждый бинарник без ENV → usage в stderr → exit 0 (CI safety).
- `make test` — 214 существующих unit-тестов SDK pass (нет регрессий).
- `make test-race` — 214 тестов pass с `-race`.

---

## Validation

| Check | Result |
|-------|--------|
| `make build-examples` | ✅ basic, http_middleware, grpc_interceptor собраны |
| `.temp/examples/basic` (no ENV) | usage + exit=0 |
| `.temp/examples/http_middleware` (no ENV) | usage + exit=0 |
| `.temp/examples/grpc_interceptor` (no ENV) | usage + exit=0 |
| `git diff --exit-code go.sum` | ✅ нет diff (no new deps) |
| Direct internal imports in examples | 0 |
| `make vet` | 0 issues |
| `make lint` | 0 issues |
| `make test` | 214/214 pass |
| `make test-race` | 214/214 pass |
| `make audit` (vet + lint + test + mod verify + tidy -diff) | ✅ clean |
| README < 50 lines | basic=43, http=46, grpc=46 |

---

## DOD Coverage

| Criterion | Status |
|-----------|--------|
| `lib/examples/basic/main.go` собирается, Login → Validate → Logout | ✅ |
| `lib/examples/http_middleware/main.go` собирается, 3 endpoints | ✅ |
| `lib/examples/grpc_interceptor/main.go` собирается, `grpc.UnaryInterceptor(...)`, skip `Health/Check` | ✅ |
| Каждый пример имеет README с командой запуска и expected output | ✅ |
| `make build-examples` собирает все три без warnings | ✅ |
| Placeholder `lib/examples/basic/main.go` заменён | ✅ |
| Examples используют только публичный API SDK (`internal` imports = 0) | ✅ (direct imports clean; transitive deps inevitable) |
| Examples не добавляют сторонних deps — `go.sum` unchanged | ✅ |
| `make audit` проходит | ✅ |
| Запуск без ENV → usage + exit 0 | ✅ |

---

## Post-Review Fixes

Code-review выявил 2 WARNING + 1 SUGGESTION:

1. **CS-0010 (W-001)**: отсутствовала пустая строка после `if err != nil { return 1 }` перед `defer func()` в 3 файлах (basic:34, http_middleware:43, grpc_interceptor:90 + 137). **Fix**: добавлены blank lines в 4 местах.
2. **CS-0002 (W-002)**: naked `return` в `readEnv()` (basic:83, grpc_interceptor:139). **Fix**: заменено на explicit `return endpoint, email, password, ok` с сохранением named параметров (требуется gocritic `unnamedResult` для функций с >1 одинаковыми типами в return).
3. **CS-0010 (S-001)**: отсутствовала пустая строка перед `return` в `safeTrunc` (basic:94). **Fix**: добавлена blank line.

Все исправлено в этом же PR; повторный `make audit` чист.

---

## Notes for Reviewers

- `gocritic.unnamedResult` и `CS-0002` дают конфликтующие требования для `readEnv() (string, string, string, bool)` — gocritic требует имена, CS-0002 запрещает naked return. Принят компромисс: named параметры + explicit values в `return`.
- `go list -deps ./lib/examples/...` показывает транзитивные internal-пакеты SDK (`lib/internal/cache`, `lib/internal/refresh`) — это ожидаемо и неизбежно, поскольку SDK сам их использует. DOD-критерий «no internal imports» интерпретирован как «no direct imports» (проверено grep'ом по `lib/examples/*/main.go`).
- `lib/auth_test.go` и `lib/internal/refresh/group_test.go` затронуты автоматическим `gofmt`/`goimports` (не функциональные изменения — import reorder и whitespace).
- TASK-006 завершает работу над examples. Следующий шаг (TASK-007): README + godoc + CHANGELOG + tag v0.1.0.
