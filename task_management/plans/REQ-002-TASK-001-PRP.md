# REQ-002-TASK-001-PRP: ConnHealth — read-only снимок состояния gRPC-соединения

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-002-TASK-001-PRP |
| Parent Request | [REQ-002](../tasks/REQ-002.md) |
| Parent Task | [REQ-002-TASK-001](../tasks/REQ-002-TASK-001.md) |
| Created | 2026-06-16 |
| Updated | 2026-06-16 |
| Score | 9/10 — −1 за то, что состояния CONNECTING/READY/TRANSIENT_FAILURE проверяются через драйв реального `*grpc.ClientConn` (gated-dialer + bufconn), а не через мок интерфейса: метод делегирует напрямую конкретному `*grpc.ClientConn`, который не подменяется. Детерминизм обеспечен блокирующим dialer'ом и `WaitForStateChange`-циклом с таймаутом 5s. |

---

## Goal

### Summary

Добавить публичный read-only аксессор состояния gRPC-соединения на `*Client`: тип
`ConnHealth{State connectivity.State; Target string}` и метод `(*Client).ConnHealth() ConnHealth`.
Метод делегирует к `grpcConn.GetState()`/`Target()` (O(1), без сети, goroutine-safe), с nil-guard
→ `{Shutdown, ""}`. `*grpc.ClientConn` не раскрывается. Аддитивное, несломающее изменение → `v0.3.0`.

### Why

- BFF funding-bot health-эндпоинт нуждается в реальных `{state, target}` по auth-зависимости; без
  аксессора отдаёт синтетику (`state` = «инициализирован», `target = "n/a"`) — низкая диагностическая
  ценность.
- Раскрывать сам `*grpc.ClientConn` нельзя: внешний код мог бы закрыть/сломать соединение, нарушив
  жизненный цикл `Client`. Поэтому — value-снимок.

### What

```go
type ConnHealth struct {
    State  connectivity.State
    Target string
}

func (c *Client) ConnHealth() ConnHealth
```

- nil `grpcConn` → `ConnHealth{State: connectivity.Shutdown, Target: ""}`.
- Иначе → `ConnHealth{State: c.grpcConn.GetState(), Target: c.grpcConn.Target()}`.

---

## Implementation Steps

1. **`lib/conn_health.go`** (новый файл) — тип `ConnHealth` + метод с nil-guard. Импорт только
   `google.golang.org/grpc/connectivity`. Godoc: read-only, by-value, no `*grpc.ClientConn`, O(1),
   connection-level (не probe).
2. **`lib/conn_health_test.go`** (`package authclient`) — конструирует `&Client{grpcConn: conn}`
   напрямую (доступ к приватному полю):
   - `NilConn` → `{Shutdown, ""}`.
   - `Idle` → свежий `grpc.NewClient(passthrough:///…)` без `Connect()`; assert `Idle` + target.
   - `Shutdown` → `conn.Close()` затем assert `Shutdown`.
   - `ConnectingThenReady` → gated-dialer (блокирует до `release`) + `bufconn`-сервер: после
     `Connect()` залипает в `Connecting`; `close(release)` → `Ready`. `WaitForStateChange`-цикл.
   - `TransientFailure` → dialer возвращает ошибку; `Connect()` → ждать `TransientFailure`.
   - `Concurrent` → 50 горутин вызывают `ConnHealth()` (для `-race`).
3. **`README.md`** — раздел «Connection Health» + пример маппинга state→светофор для BFF.
4. **`CHANGELOG.md`** — запись `v0.3.0` (Added) + ссылки.
5. **Качество** — `make audit` (vet+lint+test) и `make test-race` зелёные.
6. **Релиз** — merge в `main`, `git tag v0.3.0`, `git push origin main --tags`.

---

## Validation

- `make audit` — зелёный (vet 0, lint «0 issues», 210 тестов pass).
- `make test-race` — зелёный.
- Публичный surface: `grep` по `lib/` — нет экспортируемого `*grpc.ClientConn` (поле/возврат/геттер).
- `go get github.com/webdelo/auth-client-go@v0.3.0` резолвится после push тега.

---

## Risks & Mitigations

| Риск | Митигация |
|------|-----------|
| Флапающие тесты состояний (гонки переходов) | Блокирующий gated-dialer фиксирует `Connecting`; `WaitForStateChange`-цикл с ctx-таймаутом 5s детерминирует `Ready`/`TransientFailure`. |
| Случайное раскрытие conn в будущем | Godoc + DOD-инвариант: возвращается только value-снимок. |
| Семантический дрейф (ждут active probe) | Godoc и README явно: connection-level, probe — вне scope. |
