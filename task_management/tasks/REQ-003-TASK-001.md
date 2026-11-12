# `ABORTED` от `Refresh` — повторяемая ошибка, а не `ErrUnavailable`

---

## REQ
**Родительский реквест**: [REQ-003: Повтор продления — отдельная ошибка](./REQ-003.md)

## Jira ID
**Jira**: [FB-1602](https://webdelo-trading.atlassian.net/browse/FB-1602) (эпик [FB-1239](https://webdelo-trading.atlassian.net/browse/FB-1239))

## Маршрут выполнения
**Операционный объём**: согласован: публикация нового минорного тега по правилам релиза репозитория (`make release`). Выкладок нет

**Тип**: Code

Меняются `lib/errors.go`, при необходимости `lib/auth.go`, тесты, `README.md`, `CHANGELOG.md`.

**Priority**: high
**Depends on**: —

---

## WHY

Блокер FB-1601. Сервер auth (FB-1599) сохраняет сессию, когда продление с той же ротированной cookie приходит в пределах 30-секундного окна повтора, и отвечает gRPC `ABORTED` («повторите»). SDK превращает это в `ErrUnavailable`, поэтому bff tsa отвечает 503 «auth unavailable» и не может ответить 409 `REFRESH_RETRY`.

## Evidence

- auth `origin/main` `5f37315` (FB-1599, REQ-002-TASK-009, PR #20, выложен на stage): `internal/handlers/grpc/auth/errors.go:20,36` — `status.Error(codes.Aborted, "refresh in progress, retry")`. Подсказки Retry-After в деталях статуса сейчас нет (собственный HTTP API auth шлёт `Retry-After: 1`).
- auth-client-go `main` `c01d7d3` (v0.3.0):
  - `lib/errors.go:15-31` — sentinels `ErrInvalidToken`, `ErrExpired`, `ErrBlocked`, `ErrRateLimited`, `ErrUnavailable`; `AuthError` с `RetryAfterHint` и помощник `RetryAfter` `:66-75`;
  - `mapGRPCError` `:80-103`: `Unauthenticated`, `PermissionDenied`, `ResourceExhausted`, `Unavailable` / `DeadlineExceeded`, `Canceled`; любой другой код, включая `ABORTED`, становится `ErrUnavailable` (`:101-102`);
  - `retryAfterFromStatus` пока ничего не читает (TODO про `google.rpc.RetryInfo`);
  - `lib/auth.go:138` — `(*Client).Refresh`.
- Проба tsa на stage 2026-11-12: повтор продления в окне → bff 503, сессия жива. tsa FB-1601 (REQ-009-TASK-020) нужна отдельная ошибка, чтобы ответить 409.
- Доступ при выкладке: GH_PAT выкладки stage читает `webdelo/auth-client-go` (проверено 2026-11-12, только статус); tsa уже зависит от `v0.3.0` (`go.mod:119`).

---

## WHAT

1. **Новый экспортируемый sentinel** (например `ErrRefreshRetry`) для `ABORTED`, обёрнутый в `AuthError` с подсказкой Retry-After, если auth её прислал (`google.rpc.RetryInfo`); `RetryAfter(err)` её возвращает; значение по умолчанию описано в документации.
2. **`ErrUnavailable`** для `UNAVAILABLE` / `DEADLINE_EXCEEDED` не меняется; остальные соответствия тоже.
3. **README и CHANGELOG** обновлены; новый минорный тег выпущен по правилам релиза репозитория (`make release`).

---

## DOD (Definition of Done)

- [ ] Юнит-тесты: `ABORTED` → `ErrRefreshRetry` с подсказкой Retry-After (с `RetryInfo` и без); `UNAVAILABLE` → `ErrUnavailable`; `errors.Is` работает через обёртки.
- [ ] `make audit` зелёный; CI на PR зелёный.
- [ ] Тег опубликован; `go get github.com/webdelo/auth-client-go@<тег>` работает из tsa с PAT выкладки.
- [ ] Каждый пункт DoD доказан на финальной голове ветки до `generate-pull-request`.
