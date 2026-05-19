# REQ-001-TASK-004 — Result Report

## Metadata

| Field | Value |
|-------|-------|
| Task ID | REQ-001-TASK-004 |
| Title | Auth operations: Login / Refresh (singleflight) / Logout / LogoutAll / ChangePassword / GetCurrentUser |
| Parent REQ | [REQ-001](../tasks/REQ-001.md) |
| PRP | [REQ-001-TASK-004-PRP.md](../plans/REQ-001-TASK-004-PRP.md) |
| Code Review | [REQ-001-TASK-004-CODE-REVIEW.yaml](../reviews/REQ-001-TASK-004-CODE-REVIEW.yaml) |
| Branch | `feature/req-001-task-004` |
| Date | 2026-05-18 |
| Verdict | APPROVED_WITH_COMMENTS (0 critical, 0 warnings remaining after fixes) |

---

## Summary

Реализованы все шесть auth-операций SDK поверх gRPC-контракта `auth_v1`: `Login`, `Refresh`, `Logout`, `LogoutAll`, `ChangePassword`, `GetCurrentUser`. Центральный нетривиальный кусок — `Refresh` с защитой от refresh-race через `golang.org/x/sync/singleflight`: две параллельные goroutine с одним `refresh_token` отправляют в AUTH ровно один RPC, остальные получают тот же результат. Ключ singleflight — `hashToken(refreshToken)` (raw-токен не уходит в heap). Обёртка `refresh.Group.Do` гарантирует `Forget(key)` сразу после возврата `sf.Do` (REQ-001-ADR §A3/§R8) — без этого следующий refresh с тем же ключом получил бы stale rotated-token. Panic-recovery внутри fn `singleflight.Do` (ES-0064) защищает от паник в gRPC-клиенте / транзитивных libs — followers получают wrapped `ErrUnavailable`, не висят навсегда. Таймауты раздельные: `LoginTimeout` (Login/ChangePassword — bcrypt), `RefreshTimeout` (Refresh/Logout), `ValidateTimeout` (LogoutAll/GetCurrentUser). Логи success — Info с `user_uuid` (без email/IP/токенов); UNAVAILABLE — Warn с `method`; refresh-panic — Error. `expiresIn(unix)` — helper `time.Until(time.Unix(...))`, нули → `0`. Tests: 173/173 unit (race-detector clean), 6 integration сценариев (gated `RUN_INTEGRATION_TESTS=true`), `-race -count=10` зелёный для `TestClient_Refresh_SingleFlight` и пакета `refresh`. Coverage: `lib` 97.2%, `lib/internal/refresh` 100% (DOD ≥80% met). Lint clean.

---

## Files Changed

### New (production)

| Path | Purpose |
|------|---------|
| `lib/internal/refresh/group.go` | `Group` — thin wrapper над `singleflight.Group` с обязательным `Forget(key)` сразу после `Do` (REQ-001-ADR §A3/§R8) |
| `lib/auth.go` | Публичные типы `LoginInput`, `LoginOutput`, `RefreshOutput`; 6 методов на `*Client`: `Login`, `Refresh`, `Logout`, `LogoutAll`, `ChangePassword`, `GetCurrentUser`; приватные `expiresIn`, `logUnavailableIfNeeded` |

### Updated (production)

| Path | Change |
|------|--------|
| `lib/client.go` | Extended consumer-side `authServiceClient` interface с 6 методами (Login/RefreshToken/Logout/LogoutAll/ChangePassword/GetCurrentUser); добавлено поле `sf *refresh.Group` + init в `New` через `&refresh.Group{}` |

### New (tests)

| Path | Tests |
|------|-------|
| `lib/internal/refresh/group_test.go` | `TestGroup_Do` (table-driven: happy_path, different_keys_independent, key_forgotten_next_call_reruns, fn_error_propagates) + `TestGroup_Do_ConcurrentDedup` (N=10 goroutine rendezvous, UT-0002 exception) |
| `lib/auth_internal_test.go` | `TestExpiresIn` (table-driven: happy_path, zero_returns_zero, negative_returns_zero, past_timestamp_returns_negative) |
| `lib/auth_test.go` | `TestClient_Login` (5), `TestClient_Refresh` (4) + `TestClient_Refresh_SingleFlight` + `TestClient_Refresh_FnPanicRecovered`, `TestClient_Logout` (3), `TestClient_LogoutAll` (3), `TestClient_ChangePassword` (3), `TestClient_GetCurrentUser` (3) |
| `lib/auth_integration_test.go` | `//go:build integration`; `TestLogin_Happy`, `TestRefresh_HappyAndRotation`, `TestRefresh_Race` (10 goroutine), `TestLogout`, `TestChangePassword`, `TestGetCurrentUser_HasEmail` (seed via direct SQL INSERT + bcrypt cost=4) |

### Updated (tests)

| Path | Change |
|------|--------|
| `lib/export_test.go` | Extended `FakeAuthClient` с 6 новыми методами + call counters; `RefreshTokenCalls` — `atomic.Int32` для race-safe SingleFlight ассерта; `ClientConfig` — добавлены `LoginTimeout`/`RefreshTimeout`; `NewTestClient` инициализирует `sf: &refresh.Group{}` |
| `lib/mocks/mock_authServiceClient.go` | Regenerated через `make generate-mocks` — теперь покрывает все 7 методов `authServiceClient` (был 1) |

### New (dependencies — integration only)

| Module | Purpose |
|--------|---------|
| `github.com/jackc/pgx/v5/stdlib` | Direct-SQL seeding test-users в AUTH Postgres |
| `golang.org/x/crypto/bcrypt` | Хэширование test-паролей (cost=4 — fast) |

---

## DOD Validation

| Criterion | Status | Evidence |
|-----------|--------|----------|
| `lib/internal/refresh/group.go` exports `Group.Do(key, fn)` with Forget inside | ✅ | `lib/internal/refresh/group.go:35-39` (Forget after Do, no defer) |
| `lib/auth.go` defines `LoginInput`/`LoginOutput`/`RefreshOutput` + 6 methods | ✅ | `lib/auth.go:20,32,44,75,137,197,223,255,281` |
| `TestClient_Refresh_SingleFlight`: 2 goroutine → 1 RPC | ✅ | `lib/auth_test.go:279` + `m.auth.RefreshTokenCalls.Load() == 1` assert |
| `errors.Is(err, ErrInvalidToken / ErrRateLimited / ErrUnavailable / ...)` works | ✅ | tests use `errors.Is` and `assert.ErrorIs`; `mapGRPCError` returns proper sentinels |
| `AccessExpiresIn` / `RefreshExpiresIn` computed via `time.Until(time.Unix(s, 0))` | ✅ | `lib/auth.go:59` + `TestExpiresIn` 4 cases pass |
| Integration tests `TestLogin_Happy`, `TestRefresh_HappyAndRotation`, `TestRefresh_Race` (10 goroutine), `TestLogout`, `TestChangePassword`, `TestGetCurrentUser_HasEmail` | ✅ | all 6 present in `lib/auth_integration_test.go`; compile-clean under `//go:build integration` |
| `go test -race -run TestClient_Refresh_SingleFlight -count=10` зелёный | ✅ | PASS (no FAIL) |
| `go test -race -count=10 ./lib/internal/refresh/...` зелёный | ✅ | PASS |
| Unit coverage `auth.go` + `internal/refresh/` ≥ 80% | ✅ | `lib` 97.2%; `lib/internal/refresh` 100% |
| Логи не содержат токены/пароли | ✅ | `c.logger.Info` использует только `"user_uuid"` field (UUID, не email/token); проверка `grep` подтверждает |
| `make audit` passes | ✅ | build OK; lint 0 issues; tests 173/173 PASS |

---

## Code Review Findings — Status

All 4 warnings + 6 suggestions addressed inline (no separate bug-fix cycle needed):

| ID | Severity | Rule | Status |
|----|----------|------|--------|
| TEST-W001 | WARNING | UT-0002 | ✅ Fixed: 5 `TestGroup_Do_*` → 1 table-driven `TestGroup_Do` + 1 concurrent (with exception comment) |
| TEST-W002 | WARNING | UT-0002 | ✅ Fixed: UT-0002 exception comments added above `TestClient_Refresh_SingleFlight` and `TestClient_Refresh_FnPanicRecovered` |
| TEST-W003 | WARNING | UT-0011 | ✅ Fixed: `"happy_path_returns_rotated_token"` → `"happy_path"`; `"happy_path_returns_user_with_email"` → `"happy_path"` |
| TEST-W004 | WARNING | UT-0029 | ✅ Fixed: `var matchCtx` → `matchAnyCtx()` helper function |
| TEST-S001 | SUGGESTION | coverage | ✅ Fixed: rationale comment added at `lib/auth.go:163-168` type-assert guard |
| TEST-S002 | SUGGESTION | UT-0001 | ✅ Absorbed by TEST-W001 fix (single-case table merged) |
| TEST-S003 | SUGGESTION | general | ℹ️ No action — `TestRefresh_Race` non-parallel exception already documented in-code |
| AUTH-S001 | SUGGESTION | CS-0110 | ✅ Fixed: removed `"See REQ-001-ADR.md §A3 + §R8."` trailing reference from `group.go` package comment |
| AUTH-S002 | SUGGESTION | PRP gotcha-7 | ℹ️ No code change — opaque-token rationale documented here: PRP task spec asked for `user_uuid` in Refresh/Logout/LogoutAll/ChangePassword success logs, but those endpoints operate on opaque tokens (no JWT parsing in SDK per ADR §14). Log fields intentionally omit `user_uuid` for these methods. Tests encode this expectation via `mocks.MockLogger.EXPECT().Info(matchAnyCtx(), "msg").Once()` without a key-value pair. |
| AUTH-S003 | SUGGESTION | CS-0110 | ⏭️ Deferred: godoc references to `TASK-005` (IPFromRequest/UserAgentFromRequest) will be replaced with concrete helper names once TASK-005 is merged. Tracked. |

---

## Static Analysis

| Check | Result |
|-------|--------|
| `make build` (via `go build ./lib/...`) | ✅ PASS |
| `make test` (173 tests) | ✅ PASS |
| `make test-race` (race detector) | ✅ PASS |
| `make lint` (golangci-lint) | 0 issues |
| `make audit` (vet + lint + test) | ✅ PASS |
| `go test -race -run TestClient_Refresh_SingleFlight -count=10 ./lib/...` | ✅ PASS |
| `go test -race -count=10 ./lib/internal/refresh/...` | ✅ PASS |

---

## Coverage

| Package | Coverage | DOD Threshold | Status |
|---------|---------:|--------------:|--------|
| `lib` (includes `auth.go`) | 97.2% | 80% | ✅ |
| `lib/internal/refresh` | 100.0% | 80% | ✅ |
| `lib/auth.go::Refresh` | 94.7% | — | ✅ (sole gap = defensive type-assert guard, documented unreachable) |
| `lib/auth.go::Login` | 100.0% | — | ✅ |
| `lib/auth.go::Logout` | 100.0% | — | ✅ |
| `lib/auth.go::LogoutAll` | 100.0% | — | ✅ |
| `lib/auth.go::ChangePassword` | 100.0% | — | ✅ |
| `lib/auth.go::GetCurrentUser` | 100.0% | — | ✅ |
| `lib/auth.go::expiresIn` | 100.0% | — | ✅ |

---

## Known Limitations & Forward Work

- **Cross-instance refresh dedup** (REQ-001-ADR §R3): singleflight работает в пределах одного процесса. Multi-replica BFF deployments не покрыты в v0.1.0. Resolution path — sticky sessions ИЛИ распределённый lock в v0.2.0+.
- **`ChangePassword` wrong-password mapping** (PRP gotcha): AUTH возвращает `codes.InvalidArgument` для wrong-current-password / weak-new-password. Текущая `mapGRPCError` маппит в `ErrUnavailable` (default case). Документировано в godoc `ChangePassword`; в v0.2.0 — explicit `ErrInvalidPassword` sentinel + `mapGRPCError` ветка для `InvalidArgument`.
- **AUTH-S003**: `TASK-005` ссылки в godoc auth.go (`IPFromRequest`, `UserAgentFromRequest`) — будут заменены на конкретные имена helper'ов после merge TASK-005.

---

## Next Steps

1. Auto-commit на ветку `feature/req-001-task-004` через `/generate-commit-worktree`.
2. После merge — освободить `task_management/results/REQ-001-TASK-004-result-report.md` в main.
3. TASK-005 (Context/Cookie/HTTP helpers) — unblocked; будет использовать `LoginInput.ClientIP/UserAgent` поля.
4. TASK-006 (HTTP middleware integration) — unblocked после TASK-005.
