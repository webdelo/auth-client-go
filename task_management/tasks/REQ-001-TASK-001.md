# Bootstrap зависимостей и integration test infra (auth-proto, grpc, docker-compose с AUTH, test.env, make test-integration)

---

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](./REQ-001.md)

**Priority**: high
**Depends on**: —

---

## WHY

До любого кода SDK нужно зафиксировать в `go.mod` контракт (`github.com/webdelo/auth-proto/go@v0.1.0`), gRPC-стек и singleflight, а также иметь автоматизированный способ запустить SDK против реального AUTH-сервиса. Без этого ни one TASK после не может писать unit/integration тесты, которые требуются по DOD каждой задачи (см. [REQ-001-ADR §A13](../plans/REQ-001-ADR.md)).

---

## WHAT

1. **Добавить прямые зависимости в `go.mod`.** Через `go get` зафиксировать: `github.com/webdelo/auth-proto/go@v0.1.0`, `google.golang.org/grpc`, `google.golang.org/protobuf`, `golang.org/x/sync` (для singleflight). Запустить `go mod tidy`, убедиться что `go.sum` чистый и нет лишних транзитивных пакетов уровня logger/DI/metrics.
2. **Создать `dev/deployments/compose.test.yaml`.** Поднимает контейнер AUTH (image из локально-собранного `webdelo/auth` либо опубликованный GHCR-tag, default — локальный билд) + PostgreSQL. Включить healthcheck на gRPC-эндпоинт AUTH (`grpc_health_probe` или TCP-probe порта 50051) для синхронной готовности к тестам.
3. **Создать `dev/deployments/test.env`.** Положить переменные для AUTH (DB connection, JWT key paths, admin seed) и для тестов SDK (`AUTH_ENDPOINT=localhost:50051`, тайминги для wait-for-ready).
4. **Расширить Makefile.** Добавить таргеты: `env-up` (compose up + wait-for-healthy), `env-down` (compose down с volumes), `test-integration` (env-up → `go test -tags=integration ./lib/...` → env-down через `trap` чтобы env гарантированно гасился).
5. **Расширить `.github/workflows/ci.yaml`.** Добавить опциональный job `integration` (matrix entry, не блокирующий PR), запускающий `make test-integration`. Использовать `workflow_dispatch` input или label gate (`if: contains(github.event.pull_request.labels.*.name, 'integration')`).
6. **Документировать в README.** Раздел «Local integration testing» с примером команд и зависимостями (Docker required), пояснение про AUTH-image (локальный билд vs GHCR), упоминание ограничений CI (опциональный job).

### Акценты

- _Триггерные правила: ES-0051, ES-0140 (структура lib), ES-0171 (никаких `nolint:staticcheck`)._
- _Версии grpc/protobuf не пинить вручную — MVS должен выбрать максимум из транзитива (см. [REQ-001-ADR §Риск R1](../plans/REQ-001-ADR.md))._
- _Build tag `//go:build integration` на интеграционных файлах — без этого `make test` потащит их во все прогоны._
- _AUTH-image публикуется отдельным DevOps-REQ; здесь — accepted limitation, в CI integration-job опциональный (см. [REQ-001-ADR §A13](../plans/REQ-001-ADR.md))._
- _`make test-integration` использует bash `trap "make env-down" EXIT` — env гасится при любом исходе тестов._

---

## DOD (Definition of Done)

- [ ] `go.mod` содержит прямые `require` для `github.com/webdelo/auth-proto/go v0.1.0`, `google.golang.org/grpc`, `golang.org/x/sync`. `go mod tidy` чистый, `go mod why github.com/webdelo/auth-proto/go` подтверждает прямую зависимость.
- [ ] `go mod why github.com/sirupsen/logrus` (и `go.uber.org/fx`, `github.com/prometheus/client_golang`) возвращают `package ... is not needed` — никаких побочных deps.
- [ ] `make env-up` поднимает AUTH через `dev/deployments/compose.test.yaml`, healthcheck на gRPC-порт отвечает в течение 30s.
- [ ] `make env-down` останавливает env и удаляет volumes.
- [ ] `make test-integration` поднимает env, прогоняет `go test -tags=integration ./lib/...` (пока — пустой набор; добавится в TASK-003..004), всегда останавливает env через trap.
- [ ] `dev/deployments/test.env` содержит `AUTH_ENDPOINT=localhost:50051` и все необходимые AUTH ENV.
- [ ] `.github/workflows/ci.yaml` имеет job `integration` (не блокирующий) с corresponding gate.
- [ ] README содержит раздел «Local integration testing» с командами и caveat про AUTH-image.
- [ ] `make audit` (vet + lint + test) проходит чисто — установка deps не должна сломать существующие unit-тесты.
