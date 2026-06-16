# ConnHealth — read-only снимок состояния gRPC-соединения

---

## REQ
**Родительский реквест**: [REQ-002: Read-only диагностический снимок состояния gRPC-соединения (`ConnHealth`)](./REQ-002.md)

**Priority**: medium
**Depends on**: —

---

## WHY

Потребитель SDK — **BFF funding-bot** — собирает health-эндпоинт `GET /api/v1/health` по
каждой gRPC-зависимости в формате `{ state, target }` и маппит в «светофор» (ok/degraded/down).
Сейчас `auth-client-go` держит `grpcConn *grpc.ClientConn` приватным без аксессоров, поэтому
BFF вынужден отдавать по `auth` синтетический статус и `target = "n/a"`. Нужно дать реальные
данные о соединении, **не раскрывая** `*grpc.ClientConn` (иначе внешний код сможет закрыть/
сломать соединение).

---

## WHAT

1. **Тип `ConnHealth`** — value-снимок `{State connectivity.State; Target string}`. Спроектирован
   расширяемым (будущие `LastError`, `LatencyMs` — аддитивно). НЕ содержит `*grpc.ClientConn`.
2. **Метод `(*Client).ConnHealth() ConnHealth`** — читает `c.grpcConn.GetState()` и `Target()`.
   O(1), без сетевых вызовов, потокобезопасно (методы `ClientConn` goroutine-safe).
3. **nil-guard** — при `c.grpcConn == nil` вернуть `{State: connectivity.Shutdown, Target: ""}`.
4. **Инкапсуляция** — `*grpc.ClientConn` не экспонируется ни полем, ни возвратом, ни геттером.
5. **Юнит-тесты** — состояния READY/IDLE/CONNECTING/TRANSIENT_FAILURE/SHUTDOWN (детерминированно
   через gated-dialer + `bufconn`), nil-guard, target, конкурентный вызов.
6. **Документация** — раздел в `README.md` (+ пример интеграции в health-эндпоинт) и запись
   `v0.3.0` в `CHANGELOG.md`.
7. **Релиз** — аддитивно → minor-бамп `v0.3.0` (pre-1.0). `make release` интерактивный, поэтому
   тег ставится явно: влить в `main`, `git tag v0.3.0 && git push origin main --tags`.

### Акценты

- _Триггерные правила: golang, grpc, testing (`.claude/guides/golang/`)._
- Новый файл `lib/conn_health.go` (один concern на файл — по образцу `validate.go`, `auth.go`).
- Тесты — `package authclient` (internal) для доступа к приватному `grpcConn`; test-файлы
  исключены из линтера (`.golangci.yaml` exclusions).
- Состояние — connection-level, НЕ активный health-RPC (probe `Ping` — вне scope, отдельная
  будущая задача без слома).

---

## DOD (Definition of Done)

- [x] `(*Client).ConnHealth()` возвращает реальные `{State, Target}` из `grpcConn` (проверено
      тестами с поднятым/неподнятым соединением).
- [x] `*grpc.ClientConn` не раскрыт нигде в публичном API; код компилируется; `make audit` зелёный.
- [x] nil-guard (`grpcConn == nil` → `{Shutdown, ""}`) покрыт юнит-тестом.
- [x] `make test-race` зелёный (метод вызывается конкурентно).
- [x] `README.md` (раздел + пример) и `CHANGELOG.md` (v0.3.0) обновлены.
- [x] Тег `v0.3.0` создан и запушен; `go get github.com/webdelo/auth-client-go@v0.3.0` резолвится.
- [x] В отчёте — точная публичная сигнатура (тип `ConnHealth` + метод) и `go get`-строка.
