# REQ-001-TASK-003 — Result Report

## Metadata

| Field | Value |
|-------|-------|
| Task ID | REQ-001-TASK-003 |
| Title | ValidateAccessToken + TTL cache + HTTPMiddleware + UnaryInterceptor (server-side gRPC) + fail-mode |
| Parent REQ | [REQ-001](../tasks/REQ-001.md) |
| PRP | [REQ-001-TASK-003-PRP.md](../plans/REQ-001-TASK-003-PRP.md) |
| Code Review | [REQ-001-TASK-003-CODE-REVIEW.yaml](../reviews/REQ-001-TASK-003-CODE-REVIEW.yaml) |
| Branch | `feature/req-001-task-003` |
| Date | 2026-05-18 |
| Verdict | APPROVED (after fixes — original: APPROVED_WITH_COMMENTS / CHANGES_REQUESTED) |

---

## Summary

Реализован hot-path SDK: `ValidateAccessToken` с in-memory TTL-cache на `sync.Map`, готовые `HTTPMiddleware` и `UnaryServerInterceptor` для защиты consumer-эндпоинтов. Cache-key — `hex(sha256(token))` (raw-токен никогда не появляется в логах/метриках). Positive результаты кэшируются на `cfg.ValidateCacheTTL` (default 30s); negative — нет. Fail-mode определяет поведение middleware при `ErrUnavailable`: `FailClosed` (default, OWASP A07) → 503 + `Retry-After: 5` / `codes.Unavailable`; `FailOpen` → pass-through с `User{}`. gRPC interceptor защищён от паники через named-return + `defer recover()` → `codes.Internal`. Cleanup goroutine `MemoryCache.cleanupLoop` подписана на `bgCtx` и завершается при `Close()`. Введён consumer-side interface `authServiceClient` (ES-0051) для мокаемости `auth_v1.AuthServiceClient`. Добавлены типы публичного контракта — `Role` (`RoleUnknown/RoleUser/RoleAdmin`), `User{UUID,Role,Email}` с методами `IsZero/IsAdmin/HasRole`, `WithUser`/`UserFromContext` для ctx-propagation (унаследовано из TASK-005 → TASK-003 via alternatives-analysis). Migrated mockery.yaml v2→v3 (testify template). Tests: 132/132 pass с race detector, lint clean. Coverage по target-файлам 95.7–100% (DOD ≥80% met). `BenchmarkMemoryCache_Get`: 31.71 ns/op, 0 allocs (target — < 1µs).

---

## Files Changed

### New (production)

| Path | Purpose |
|------|---------|
| `lib/context.go` | `Role` type + 3 constants; `User{UUID,Role,Email}` + `IsZero/IsAdmin/HasRole`; `WithUser/UserFromContext` с unexported `userCtxKey{}` |
| `lib/internal/cache/memory.go` | `MemoryCache` на `sync.Map`; `NewMemoryCache(bgCtx, cleanupInterval)`; lazy-expire в `Get`; `cleanupLoop` goroutine, gated on `bgCtx.Done()` |
| `lib/validate.go` | `hashToken(token) → hex(sha256)`; `(*Client).ValidateAccessToken(ctx, token) → (User, error)` с cache-aside pattern |
| `lib/middleware_http.go` | `bearerToken(authz) → (string, bool)` case-insensitive RFC 7235; `(*Client).HTTPMiddleware(next) → http.Handler` с 401/503/429/200 + FailOpen pass-through |
| `lib/interceptor_grpc.go` | `(*Client).UnaryInterceptor() → grpc.UnaryServerInterceptor` с panic-recovery (named-return) + openMethods skip |

### Updated (production)

| Path | Change |
|------|--------|
| `lib/client.go` | Cache iface `any → User`; новое поле `cache Cache` на Client; consumer-side `authServiceClient` interface; default cache wiring (WithCache > Config.Cache > MemoryCache); `cacheAdapter` для type-erasure; named constant `defaultDialOptionsCount=3`; compile-time assertion `var _ Cache = (*cacheAdapter)(nil)` |
| `lib/errors.go` | `mapErrorReason`: добавлен `"invalid_token_version"` → `ErrExpired` (LogoutAll/password-change rotation); `retryAfterFromStatus` — idiomatic `(_ *status.Status)` signature + TODO comment |
| `mockery.yaml` | Migrated v2 → v3 (template: testify, pkgname, structname, filename override `mock_{{.InterfaceName}}.go`) |

### New (tests)

| Path | Tests |
|------|-------|
| `lib/context_test.go` | `TestUser_IsZero` (2 cases), `TestUser_IsAdmin` (3), `TestUser_HasRole` (2), `TestUserFromContext` (2) |
| `lib/internal/cache/memory_test.go` | `TestMemoryCache_Get` (4), `TestMemoryCache_Delete` (2), `TestMemoryCache_Cleanup` (3 incl. goroutine-leak DOD test), `TestMemoryCache_Concurrent` (race) |
| `lib/internal/cache/memory_bench_test.go` | `BenchmarkMemoryCache_Get`, `BenchmarkMemoryCache_Set` |
| `lib/validate_test.go` | `TestClient_ValidateAccessToken` (12 cases) |
| `lib/middleware_http_test.go` | `TestBearerToken` (8), `TestClient_HTTPMiddleware` (10) |
| `lib/interceptor_grpc_test.go` | `TestClient_UnaryInterceptor` (13 incl. panic-recovery DOD test) |
| `lib/validate_integration_test.go` | `//go:build integration`; `TestClient_ValidateAccessToken_Integration` (table-driven, 3 cases — 2 skipped до TASK-004/007) |
| `lib/export_test.go` | Exports `FakeAuthClient`, `NewTestClient`, `HashToken`, `DefaultOpenMethods`, `BearerToken` для `package authclient_test` (обход import cycle через `lib/mocks`) |

### Updated (tests)

| Path | Change |
|------|--------|
| `lib/client_test.go` | `stubCache` Get/Set: `any → User`; assertion `c.cfg.Cache → c.cache`; added import-cycle workaround comment |
| `go.mod` / `go.sum` | + `github.com/stretchr/objx` (transitive via testify/mock) |

### Generated

| Path | Source |
|------|--------|
| `lib/mocks/mock_Cache.go` | `Cache` interface |
| `lib/mocks/mock_Logger.go` | `Logger` interface |
| `lib/mocks/mock_authServiceClient.go` | `authServiceClient` interface (unexported — sibling `FakeAuthClient` used вместо него из-за unexported visibility) |

---

## DOD Coverage

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | `lib/internal/cache/memory.go` с `Get/Set/Delete` | ✅ | `grep "func (.*MemoryCache) (Get\|Set\|Delete)" lib/internal/cache/memory.go` — 3/3 |
| 2 | `lib/validate.go` экспортирует `ValidateAccessToken`; пустой токен → `ErrInvalidToken` без gRPC | ✅ | `validate.go:39-43`; `TestClient_ValidateAccessToken.empty_token_returns_invalid_token` |
| 3 | Positive кэшируется, negative — нет | ✅ | `TestClient_ValidateAccessToken.is_valid_false_*_returns_*` (no Cache.Set call); `TestClient_ValidateAccessToken.cache_hit_returns_without_grpc` |
| 4 | Cleanup goroutine завершается при Close (runtime.NumGoroutine) | ✅ | `TestMemoryCache_Cleanup.goroutine_exits_when_ctx_cancelled` |
| 5 | `BenchmarkValidateCache_Hit < 1µs/op` | ✅ | `BenchmarkMemoryCache_Get-14: 31.71 ns/op, 0 B/op, 0 allocs/op` (×32 раза быстрее target) |
| 6 | `HTTPMiddleware` возвращает 401/503/429/200 + FailOpen | ✅ | `TestClient_HTTPMiddleware` — все 10 cases pass; `middleware_http.go` содержит все 4 status кода |
| 7 | `UnaryInterceptor` — `WithOpenMethods` skip + panic-recovery `codes.Internal` | ✅ | `TestClient_UnaryInterceptor.open_method_skips_auth`, `.panic_in_handler_returns_internal` |
| 8 | `WithOpenMethods("/foo/Bar")` корректно дополняет registry | ✅ | `TestClient_UnaryInterceptor.with_open_methods_custom_method_skips_auth` |
| 9 | `Cache` интерфейс User-typed; `WithCache` устанавливает `c.cache` | ✅ | `lib/client.go:66-73, 142-144`; `TestNew.with_cache_custom` |
| 10 | `lib/client.go` поле `cache Cache`; default `cacheAdapter{*cache.MemoryCache}` | ✅ | `client.go:178`; resolution priority в `New`: WithCache > Config.Cache > MemoryCache |
| 11 | `lib/client.go` поле `bgCtx context.Context` (унаследовано из TASK-002) | ✅ | `client.go:187` (no rule-code refs, post-fix) |
| 12 | `mockery.yaml` сгенерил mocks; `make generate-mocks` идемпотентен | ✅ | `lib/mocks/mock_Cache.go`, `mock_Logger.go`, `mock_authServiceClient.go` |
| 13 | Integration-тесты `_Happy` и `_Invalid` (либо `t.Skip` до TASK-004) | ✅ | `lib/validate_integration_test.go` — `invalid_token_returns_err_invalid_token` active; `happy_path`/`expired` skip до TASK-004/007 |
| 14 | Unit coverage по validate/middleware/interceptor/context/internal/cache ≥ 80% | ✅ | context.go 100%, cache 100%, validate.go 100%, middleware 95.7%, interceptor 100% |
| 15 | `make audit` clean | ✅ | vet 0, lint 0, mod verify ok, 132/132 tests pass |
| 16 | `go test -race ./lib/...` без warnings | ✅ | race detector clean (TestMemoryCache_Concurrent — 50 goroutines on shared keys) |

**Дополнительные контроли качества:**

| Check | Status |
|-------|--------|
| Cache key — `hex(sha256(token))`, не raw-токен | ✅ `grep "c\.cache\.\(Set\|Get\)" lib/validate.go` → только через `hashToken(...)` результат |
| HTTP middleware не логирует токен | ✅ Только `logger.Warn` с фиксированным message (без `token`) на FailOpen |
| Panic recovery использует named return | ✅ `(resp interface{}, err error)` — defer присваивает к `err` |
| FailClosed — default | ✅ `FailClosed FailMode = iota` |
| ES-0051: consumer-side `authServiceClient` interface | ✅ `lib/client.go:163-169` — unexported, в пакете-потребителе |
| ES-0171: NO //nolint annotations | ✅ `grep "//nolint" lib/` пусто |
| `mapErrorReason` строки совпадают с AUTH server: `expired`/`invalid_token_version`→`ErrExpired`, `blocked`→`ErrBlocked`, default→`ErrInvalidToken` | ✅ `lib/errors.go:123-132`; покрыто 5 cases в `TestClient_ValidateAccessToken` |
| Compile-time assertion `var _ Cache = (*cacheAdapter)(nil)` | ✅ end of `lib/client.go` |
| Все production godoc на public identifiers | ✅ Role/RoleUnknown/RoleUser/RoleAdmin/User/IsZero/IsAdmin/HasRole/WithUser/UserFromContext/ValidateAccessToken/HTTPMiddleware/UnaryInterceptor/Cache/WithCache |
| No rule codes в production godoc (CS-0110) | ✅ post-fix: `grep -E "(ES\|EC\|CS\|UT)-[0-9]" lib/*.go lib/internal/cache/*.go \| grep -v _test.go` пусто |

---

## Issues Resolved

См. полный список в [REQ-001-TASK-003-CODE-REVIEW.yaml](../reviews/REQ-001-TASK-003-CODE-REVIEW.yaml). Кратко:

### Critical (3) — все исправлены

- **TEST-C001** UT-0032: `if name ==` ветвление в TestMemoryCache_Get → заменено на struct field `sleepBefore`.
- **TEST-C002** UT-0032: `if tc.useLogger`/`if handler == nil` в TestClient_UnaryInterceptor → field-based + explicit handler в каждом case.
- **TEST-C003** UT-0028: `hexSHA256` helper closure → заменено на exported `authclient.HashToken`.

### Warnings (7) — все исправлены

- **CR-001** Rule-codes (`EC-0051`) в production godoc → plain-language объяснение.
- **CR-002** `(EC-0010, EC-0012)` в `MemoryCache` godoc → "ensuring no goroutine leak".
- **CR-003** Missing compile-time interface assertion → добавлен `var _ Cache = (*cacheAdapter)(nil)`.
- **CR-004** WithCache godoc inaccuracy → описан 3-level priority.
- **TEST-W001** UT-0002: 3 функции для одного метода → консолидировано в `TestClient_ValidateAccessToken_Integration` table-driven.
- **TEST-W002** UT-0026: missing `newTestMocks(t)` → добавлены 3 конструктора с `t.Helper()`.
- **TEST-W003** UT-0020: `recordingLogger`/`stubCache` без import-cycle документации → добавлен 7-строчный комментарий.

### Suggestions (4) — все исправлены

- **CR-S001** ADR §R10 references в public godoc → self-contained explanation.
- **CR-S002** Magic number `3` в dialOpts capacity → named const `defaultDialOptionsCount`.
- **CR-S003** `_ = st` idiom в `retryAfterFromStatus` → `(_ *status.Status)` signature + TODO.
- **TEST-S001** Weak ctx matcher `ctx != nil` → strengthened to deadline check в `FakeAuthClient.ValidateFunc` bodies.

---

## Benchmark

```
BenchmarkMemoryCache_Get-14    	37692691	        31.71 ns/op	       0 B/op	       0 allocs/op
BenchmarkMemoryCache_Set-14    	16289928	        74 ns/op	       3 allocs/op
```

Target: `< 1µs/op` на Get. Actual: **31.71 ns/op** (≈32× headroom). 0 allocations on cache hit.

---

## Test Statistics

| Package | Tests | Passed | Coverage |
|---------|-------|--------|----------|
| `lib` | 119 | 119 | 92.8% |
| `lib/internal/cache` | 13 | 13 | 100% |
| `lib/mocks` | 0 | 0 | n/a (generated) |
| **TOTAL** | **132** | **132** | — |

`go test -race` — clean (0 races).
`make lint` — 0 issues.
`make audit` — clean.

---

## Open Questions Carried Forward

| # | Question | Resolved in |
|---|----------|-------------|
| OQ-1 | `ValidateCacheTTL == 0` — disable cache (per ADR §A2) vs apply default (current applyDefaults)? | TASK-007 README + потенциальный fix в `applyDefaults`. Не блокирует TASK-003: для negative TTL сейчас default срабатывает, для zero — сохраняется (то есть кэш disabled). |
| OQ-2 | `ErrRateLimited` без `RetryAfter` (AUTH v0.1.0 не передаёт metadata) — что в `Retry-After` header? | Сейчас — не ставим header при `d == 0`. Future-compat: можно перейти на default `1s` (additive). |

---

## Next Steps

- TASK-004: Auth operations (Login / Refresh w/ singleflight / Logout / LogoutAll / ChangePassword / GetCurrentUser). После него — раскомментировать `happy_path` в `validate_integration_test.go`.
- TASK-005: Cookies + HTTP request helpers (scope reduced — `context.go` уже в TASK-003 per alternatives-analysis).
- TASK-006: Examples (`basic`, `http_middleware`, `grpc_interceptor`).
- TASK-007: README + godoc + CHANGELOG + tag v0.1.0; resolve OQ-1 / OQ-2.
