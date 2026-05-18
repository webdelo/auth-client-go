# REQ-001-TASK-001-PRP: Bootstrap зависимостей и integration test infra

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-001-TASK-001-PRP |
| Parent Request | [REQ-001](../tasks/REQ-001.md) |
| Parent Task | [REQ-001-TASK-001](../tasks/REQ-001-TASK-001.md) |
| ADR | [REQ-001-ADR §A13](./REQ-001-ADR.md) |
| Created | 2026-05-15 |
| Updated | 2026-05-15 |
| Estimated Score | 9/10 (см. секцию `PRP Quality Score`) |

---

## Goal

### Summary

Зафиксировать direct-зависимости SDK (`github.com/webdelo/auth-proto/go@v0.1.0`, `google.golang.org/grpc`, `golang.org/x/sync`) в `go.mod` и развернуть автоматизированную integration-test-инфраструктуру: docker-compose поднимает реальный AUTH gRPC-сервер с PostgreSQL, Makefile-таргеты `env-up`/`env-down`/`test-integration` гарантируют корректный lifecycle env'а, опциональный CI-job дополняет существующие lint/test/build шаги.

### Why

- **Блокирует TASK-002..006.** До фиксации `auth-proto/go` в `go.mod` SDK не на чём собрать, до Docker-инфры — не на чём прогнать integration-тесты, требуемые DOD каждой задачи.
- **Контроль зависимостей.** SDK обязан тащить ровно три direct-require (auth-proto, grpc, singleflight). Лишний транзитив (logger, DI, Prometheus client) попадёт в consumer'ов BFF/simple-trader/arbitrage-trader — anti-pattern (REQ-001 Risk #7).
- **MVS-резолв grpc/protobuf.** auth-proto/go@v0.1.0 транзитивно требует grpc v1.79.3 + protobuf v1.36.11. Если SDK сам пинит другие версии — потенциальные ABI-конфликты (REQ-001 Risk R1). PRP запрещает явный require этих модулей.
- **Реальный AUTH вместо mock'ов.** Контрактные регрессы (например, AUTH добавил поле в `ValidateTokenResponse`) ловятся только при тестах против настоящего сервера. Mock'и закрывают unit-кейсы, не контрактные.

### What

- `go.mod` соответствует `go 1.24` (downgrade с `1.26.2`), содержит ровно три direct-require, `go.sum` создан.
- `dev/deployments/compose.test.yaml` поднимает `auth-postgres` + `auth-migrate` + `auth-app` (локальный build `target: production` из sibling-репо `webdelo/auth`).
- `dev/deployments/test.env` содержит полный набор AUTH ENV (БД, JWT, peppers, rate limits) + `AUTH_ENDPOINT=localhost:50051` для SDK-тестов.
- `dev/deployments/extra.mk` (включён в `Makefile` через `-include`) реализует таргеты `dev-up-test-env`, `dev-down-test-env`, алиасы `env-up`/`env-down`, `test-integration` с гарантированным `env-down` через `TEST_EXIT` паттерн.
- `lib/client_integration_test.go` — placeholder с `//go:build integration` и `TestMain` (gate через `RUN_INTEGRATION_TESTS=true`). Реальные кейсы добавляются в TASK-003..004.
- `.github/workflows/ci.yaml` расширен: `workflow_dispatch.inputs.run_integration` (boolean) + job `integration` с `if: ${{ inputs.run_integration == true }}` + `continue-on-error: true` + `GOPRIVATE: github.com/webdelo/*` (для всех jobs).
- `README.md` имеет раздел «Local integration testing» с prereqs (Docker, sibling `webdelo/auth` repo), командами и caveat'ом про опциональность CI-job'а.

### Success Criteria

- [ ] `go mod tidy` не оставляет diff'а на чистом дереве.
- [ ] `go mod why github.com/webdelo/auth-proto/go` → direct dependency.
- [ ] `go mod why google.golang.org/grpc` → direct dependency.
- [ ] `go mod why golang.org/x/sync` → direct dependency.
- [ ] `go mod why github.com/sirupsen/logrus` → `(main module does not need package ...)`.
- [ ] `go mod why go.uber.org/fx` → `not needed`.
- [ ] `go mod why github.com/prometheus/client_golang` → `not needed`.
- [ ] `make env-up` доводит `auth-app` до `healthy` ≤ 30s.
- [ ] `make env-down` останавливает контейнеры и удаляет volume `auth-jwt-keys` + Postgres volume.
- [ ] `make test-integration` всегда вызывает `dev-down-test-env` (проверяется: упавший тест возвращает non-zero exit, env гасится).
- [ ] CI: при `push`/`pull_request` job `integration` отсутствует в run summary (skipped). При `workflow_dispatch` с `run_integration=true` job появляется и не блокирует merge.
- [ ] `make audit` (vet + lint + test) проходит чисто; существующие unit-тесты не сломаны.

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен; mandatory_guides не относятся напрямую к infra-задаче (нет нового Go-кода кроме placeholder `_test.go`) |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | ES-0051 (consumer-side interfaces — соблюдается отсутствием новых интерфейсов), ES-0060 (handle errors explicitly — `TestMain` без silent ignore), ES-0140 (constructor `New` — N/A), ES-0143 (noop defaults — N/A), ES-0171 (no nolint annotations — соблюдается в placeholder тесте) |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | CS-0050 (структура пакетов — `lib/internal/cache/`, `lib/internal/refresh/` создадутся в TASK-003/004, не здесь) |
| Unit Tests Standards | [unit_tests_standards.md](../../.claude/guides/golang/unit_tests_standards.md) | UT-0001 (table-driven — N/A в TASK-001, добавляется в последующих задачах) |
| Integration Tests Standards | [integration_tests_standards.md](../../.claude/guides/golang/integration_tests_standards.md) | Build tag `//go:build integration`, `TestMain` гард `RUN_INTEGRATION_TESTS=true`, изоляция через docker-compose |

### Concurrency Patterns

- [ ] Worker Pool — N/A
- [ ] Semaphore — N/A
- [ ] errgroup — N/A
- [ ] Fan-Out/Fan-In — N/A

TASK-001 не вводит concurrency-кода. Goroutine-lifecycle (cache cleaner) появляется в TASK-003.

---

## Analysis Files

- **Code Research**: [2026-05-15_code-research_REQ-001-TASK-001.md](../../.memory_bank/agent_task_results/2026-05-15_code-research_REQ-001-TASK-001.md) — анализ паттернов из `webdelo/auth`, `signal-analyzer`, `funding-bot`; контракт `auth-proto/go@v0.1.0`; Go version mismatch; healthcheck-стратегия; CI gate.
- **ADR**: [REQ-001-ADR §A13](./REQ-001-ADR.md) — integration testing infrastructure.
- **REQ**: [REQ-001.md](../tasks/REQ-001.md) — общий контекст SDK v0.1.0 (Risks R1/R6/R7 особенно).

---

## Context

### Current Structure

```
auth-client-go/
├── CLAUDE.md
├── CHANGELOG.md
├── Makefile                       # test/test-race/coverage/lint/build-examples
├── README.md                      # placeholder
├── go.mod                         # module ..., go 1.26.2 (MISMATCH — должно быть 1.24)
├── (нет go.sum)
├── .golangci.yaml
├── .github/workflows/ci.yaml      # lint + test + build, без integration / GOPRIVATE
├── lib/
│   ├── lib.go                     # const Version = "0.0.1"
│   ├── lib_test.go                # TestVersion (placeholder)
│   ├── internal/.gitkeep
│   └── examples/basic/main.go     # печатает Version
├── dev/
│   └── tools/                     # release.sh, tools.mk, test-runner-main.go
└── task_management/
    └── plans/REQ-001-ADR.md       # архитектурный план
```

### Desired Structure

```
auth-client-go/
├── go.mod                         # UPDATE: go 1.24; +3 direct require
├── go.sum                         # NEW
├── Makefile                       # UPDATE: -include dev/deployments/extra.mk; +test-integration в .PHONY
├── README.md                      # UPDATE: +«Local integration testing»
├── .gitignore                     # UPDATE: уверенно игнорим coverage/, bin/, .temp/ (если ещё нет)
├── .github/workflows/ci.yaml      # UPDATE: +workflow_dispatch input, +GOPRIVATE env, +integration job
├── dev/
│   ├── tools/                     # без изменений
│   └── deployments/               # NEW dir
│       ├── compose.test.yaml      # NEW: auth-postgres + auth-migrate + auth-app
│       ├── test.env               # NEW: AUTH env + AUTH_ENDPOINT для SDK
│       └── extra.mk               # NEW: dev-up-test-env, dev-down-test-env, env-up, env-down, test-integration
└── lib/
    ├── lib.go                     # unchanged
    ├── lib_test.go                # unchanged
    └── client_integration_test.go # NEW: //go:build integration; TestMain placeholder
```

### Gotchas

| Category | Issue | Solution |
|----------|-------|----------|
| Go version | `go.mod` `go 1.26.2` рассинхронизирован с CLAUDE.md / tech_stack.yaml / CI / auth-proto/go (везде `1.24`) | `Edit go.mod` → `go 1.24`. Без этого `go mod tidy` плюётся warning'ами в CI с runner `go-version: '1.24'` |
| Go modules | `golang.org/x/sync` это **модуль**, `singleflight` — это **пакет** в нём. `go get golang.org/x/sync` (НЕ `.../singleflight`) | Чтобы появилась direct require, нужен импорт пакета хотя бы в одном `.go` файле SDK. В TASK-001 ещё нет такого импорта. Решение: добавить blank import `_ "golang.org/x/sync/singleflight"` в `lib/client_integration_test.go` (под build tag), либо отложить direct-require до TASK-004. **Выбор PRP:** добавить blank import в integration test — это легитимно и держит зависимость зафиксированной с TASK-001 |
| go.mod direct vs indirect | После `go mod tidy` без реальных импортов в production-коде, deps попадут в `require ( ... // indirect )` | См. выше — blank import в `_test.go` под build tag `integration` обеспечивает direct dependency. Альтернатива: импорт `auth-proto/go/auth_v1` в `lib/lib.go` как `var _ = auth_v1.AuthServiceClient(nil)`. PRP принимает blank import в test-файле как меньшее зло |
| auth-proto/go private | Это private GitHub-репо `webdelo/auth-proto`. `go get` требует GOPRIVATE/GH_PAT | CI уже имеет `git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"`. Дополнительно нужно `GOPRIVATE: github.com/webdelo/*` env во всех jobs (включая новый integration). Локально разработчик должен иметь свой PAT в `~/.netrc` или `GIT_CREDENTIAL_*` (документируется в README) |
| AUTH image отсутствует в GHCR | `image: ghcr.io/webdelo/auth/...` пока не существует | Используем `build: { context: ../../../auth, target: production }` относительно расположения `compose.test.yaml`. Путь параметризуется через `${AUTH_REPO_PATH:-../../../auth}` — кастомные локации работают (CI checkout'ит auth репо в `../auth` и переопределяет переменную) |
| AUTH path в compose | `compose.test.yaml` лежит в `dev/deployments/`, sibling auth repo — `../../bots/auth` (3 уровня вверх от compose файла) | Использовать `${AUTH_REPO_PATH:-../../../auth}` в compose. В test.env установить `AUTH_REPO_PATH=../../../auth` для дефолта. В CI — checkout auth репо в `${{ github.workspace }}/auth-server` + export `AUTH_REPO_PATH=$GITHUB_WORKSPACE/auth-server` |
| JWT keys | AUTH server требует `AUTH_JWT_KEY_PATH` (директория) и сам генерирует Ed25519 keypair при первом запуске (`LoadOrGenerateKeyPair`) | **Не коммитим ключи, не пишем scripts.** Достаточно named volume `auth-jwt-keys` в compose, mount `/keys`, `AUTH_JWT_KEY_PATH=/keys`. AUTH создаст `ed25519_private.pem` + `ed25519_public.pem` на первом старте, при `down -v` volume удалится — фрешные ключи при следующем `env-up`. Тесты в TASK-003+ работают с любой парой |
| Healthcheck | gRPC TCP-probe (`nc -z`) проверяет порт, но не сервис | AUTH `production` Dockerfile уже содержит `HEALTHCHECK CMD ./auth healthcheck --grpc localhost:50051`. compose.test.yaml НЕ переопределяет healthcheck — наследует. `docker compose up -d --wait auth-app` блокирует до получения `healthy` status |
| Trap в Makefile | Bash `trap "make env-down" EXIT` ненадёжен внутри Makefile-рецепта (каждая строка — subshell, trap не пробрасывается) | Используем `TEST_EXIT=$$?; $(MAKE) dev-down-test-env; exit $$TEST_EXIT` — established platform pattern (auth, signal-analyzer, funding-bot) |
| AUTH ENV полнота | auth/internal/config/config.go с `required` тегами — пропуск ENV вызовет panic при старте | Полный список ENV см. ниже в `### Implementation > Task 2`. Если хоть один обязательный пропущен — auth-app не дойдёт до healthy, `env-up` зависнет до timeout `--wait` |
| `compose.test.env` уже есть в auth | auth-репо имеет свой `dev/deployments/compose.test.env` (используется auth-internal тестами) | НЕ переиспользовать через `env_file: ../../../auth/dev/deployments/compose.test.env` — это связывает наш SDK с внутренним файлом сервера, который может меняться. Копируем нужный набор в свой `test.env` и держим как single source of truth |
| `go.mod` toolchain directive | `go get` под Go 1.21+ может добавить `toolchain` директиву | После `go mod tidy` удалить `toolchain` строку (если появилась) — мы хотим стабильную минимальную версию `go 1.24` без auto-upgrade |
| CI integration runs only on manual trigger | Если оставить только `workflow_dispatch`, разработчик может забыть его запустить → регресс уйдёт в main | Acceptable per ADR §A13. Документируется в README: «Integration tests run locally on every feature branch; CI integration job triggered manually before tagging releases». В v0.2.0 рассмотреть переход на label-gate |

---

## Implementation

### Data Models

TASK-001 не вводит Go-структур. Все артефакты — конфиг/скрипты/инфра.

---

### Task 1: Align Go version and add direct dependencies

**Action**: MODIFY `go.mod`, CREATE `go.sum`
**File**: [go.mod](../../go.mod)

**Description**: Скорректировать Go version, добавить три direct-require'а, прогнать `tidy`.

**Details**:
- Открыть `go.mod`, заменить `go 1.26.2` на `go 1.24`.
- В терминале (из корня репо):
  ```bash
  go get github.com/webdelo/auth-proto/go@v0.1.0
  go get google.golang.org/grpc
  go get golang.org/x/sync
  go mod tidy
  ```
- Если после `tidy` появилась строка `toolchain go1.XX.Y` — удалить (мы держим `go 1.24` без auto-upgrade).
- Убедиться, что в `require` блоке три записи без `// indirect` (для этого blank import добавляется в `lib/client_integration_test.go` — Task 6 ниже; этот шаг и Task 6 связаны: запускать tidy ПОСЛЕ создания integration-test).

**Expected go.mod (после Task 1 + Task 6)**:

```go
module github.com/webdelo/auth-client-go

go 1.24

require (
    github.com/webdelo/auth-proto/go v0.1.0
    golang.org/x/sync v0.20.0    // или новее по MVS
    google.golang.org/grpc v1.79.3 // версия совпадает с auth-proto/go transitivе
)

require (
    // indirect списка — protobuf, x/net, x/sys, x/text, genproto и т.п.
    // НИ logger (logrus/zap), НИ DI (fx/wire), НИ Prometheus client здесь быть не должно
)
```

**Verify**:

```bash
go mod why github.com/webdelo/auth-proto/go    # → direct
go mod why google.golang.org/grpc              # → direct
go mod why golang.org/x/sync                   # → direct
go mod why github.com/sirupsen/logrus           # → not needed
go mod why go.uber.org/fx                       # → not needed
go mod why github.com/prometheus/client_golang  # → not needed
```

---

### Task 2: Create test environment ENV file

**Action**: CREATE
**File**: [dev/deployments/test.env](../../dev/deployments/test.env)

**Description**: Полный набор AUTH server ENV (БД, JWT, peppers, rate-limits, cookie) + SDK-tests ENV (AUTH_ENDPOINT, RUN_INTEGRATION_TESTS, AUTH_REPO_PATH).

**Details**: Файл — обычные KEY=VALUE строки, без `export`. compose читает через `env_file:`. Никаких реальных секретов: всё throwaway, помечено комментарием.

**Content**:

```dotenv
# === SDK-test-only AUTH server ENV ===
# TEST ONLY: throwaway secrets, do NOT reuse in prod.

# Logging
LOGGER_LEVEL=info
FEATURES_LOG_INCOMING_REQUESTS=false

# HTTP features (нужно для AUTH /healthcheck на :53000 — healthcheck в Dockerfile дёргает HTTP)
FEATURES_HTTP_ENABLED=true

# Timeouts
INTERNAL_TIMEOUT=5s
EXTERNAL_TIMEOUT=10s

# Postgres (имя сервиса = auth-postgres в compose сети)
POSTGRES_HOST=auth-postgres:5432
POSTGRES_USER=auth
POSTGRES_PASSWORD=auth
POSTGRES_DATABASE=auth

# JWT (AUTH server сгенерит Ed25519 keypair при первом старте в /keys)
AUTH_JWT_KEY_PATH=/keys
AUTH_JWT_ACCESS_TTL=15m
AUTH_JWT_REFRESH_TTL=336h
AUTH_JWT_ISSUER=auth-test
AUTH_JWT_AUDIENCE=webdelo-test

# Pepper (≥32 символов, AUTH validate'ит длину)
AUTH_PEPPER_VALUE=test-pepper-value-for-testing-min32!!
AUTH_PEPPER_VERSION=1

# Hasher (low cost для скорости integration-тестов)
AUTH_HASHER_MEMORY=65536
AUTH_HASHER_ITERATIONS=1
AUTH_HASHER_PARALLELISM=1
AUTH_HASHER_SALT_LENGTH=16
AUTH_HASHER_KEY_LENGTH=32

# Rate limits (высокие лимиты — тесты не должны их триггерить случайно)
AUTH_RATE_LIMIT_IP_MAX=1000
AUTH_RATE_LIMIT_IP_WINDOW=1m
AUTH_RATE_LIMIT_EMAIL_MAX=100
AUTH_RATE_LIMIT_EMAIL_WINDOW=15m
AUTH_RATE_LIMIT_LOCKOUT_BASE=1m
AUTH_RATE_LIMIT_LOCKOUT_MAX=15m
AUTH_RATE_LIMIT_LOCKOUT_MULTIPLIER=2.0
AUTH_RATE_LIMIT_CLEANUP_INTERVAL=10m
AUTH_RATE_LIMIT_ADMIN_WRITE_MAX=1000
AUTH_RATE_LIMIT_ADMIN_WRITE_WINDOW=1m
AUTH_RATE_LIMIT_SESSION_WRITE_MAX=1000
AUTH_RATE_LIMIT_SESSION_WRITE_WINDOW=1m

# HTTP cookie (тестовый scope — без TLS)
AUTH_HTTP_COOKIE_DOMAIN=
AUTH_HTTP_COOKIE_SECURE=false
AUTH_HTTP_COOKIE_SAME_SITE=strict

# Admin seed (для удобства интеграционных тестов TASK-003..004 — известный admin)
AUTH_ADMIN_SEED_EMAIL=admin@test.local
AUTH_ADMIN_SEED_PASSWORD=test-admin-password-32-chars-min!

# === SDK tests env ===

# Путь к sibling-репо webdelo/auth для build context.
# CI переопределяет на $GITHUB_WORKSPACE/auth-server.
AUTH_REPO_PATH=../../../auth

# Эндпоинт AUTH для SDK-тестов (по умолчанию compose маппит :50051 на хост)
AUTH_ENDPOINT=localhost:50051

# Gate для TestMain в lib/client_integration_test.go
RUN_INTEGRATION_TESTS=true
```

**Critical**: список AUTH ENV должен соответствовать `auth/internal/config/config.go` (`required` теги). Если AUTH добавит required-ENV — TASK-001 (или follow-up bugfix) обязан синхронизировать `test.env`.

---

### Task 3: Create docker-compose for tests

**Action**: CREATE
**File**: [dev/deployments/compose.test.yaml](../../dev/deployments/compose.test.yaml)

**Description**: Поднимает PostgreSQL, прогоняет миграции AUTH (одноразовый контейнер), стартует AUTH server с healthcheck.

**Details**:
- `auth-postgres` — Postgres 16-alpine с healthcheck `pg_isready`.
- `auth-migrate` — `migrate/migrate:v4.17.0`, монтирует `${AUTH_REPO_PATH}/migrations:/migrations:ro`, прогоняет `up`, завершается.
- `auth-app` — build из `${AUTH_REPO_PATH}` target `production`. Inheritит HEALTHCHECK из Dockerfile (`./auth healthcheck --grpc localhost:50051`). Named volume `auth-jwt-keys` для `/keys`.
- Все три сервиса в `profiles: [test]`.
- Network: default (compose creates project-scoped bridge).
- Port mappings: только `auth-app:50051` (SDK ходит на localhost:50051) + `auth-postgres:5432` (опционально, для отладки).

**Content**:

```yaml
# dev/deployments/compose.test.yaml
#
# Integration test environment for auth-client-go SDK.
# Brings up real webdelo/auth server (local build from sibling repo) + PostgreSQL.
#
# Usage:
#   make env-up          # bring up + wait for healthy
#   make env-down        # tear down + delete volumes
#   make test-integration # full cycle: up → tests → down (guaranteed)
#
# Requires:
#   - Docker + docker-compose plugin
#   - webdelo/auth repo cloned at $AUTH_REPO_PATH (default: ../../../auth)
#
# CAVEAT: image ghcr.io/webdelo/auth not yet published. We build locally.
# When GHCR is set up (separate DevOps REQ), replace `build:` with `image:`.

name: auth-client-go-test

services:
  auth-postgres:
    image: postgres:16-alpine
    container_name: auth-client-go-test-postgres
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-auth}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-auth}
      POSTGRES_DB: ${POSTGRES_DATABASE:-auth}
      POSTGRES_HOST_AUTH_METHOD: trust
    ports:
      - "5432:5432"
    volumes:
      - auth-postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-auth}"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 5s
    profiles: [test]

  auth-migrate:
    image: migrate/migrate:v4.17.0
    container_name: auth-client-go-test-migrate
    depends_on:
      auth-postgres:
        condition: service_healthy
    volumes:
      - ${AUTH_REPO_PATH:-../../../auth}/migrations:/migrations:ro
    command:
      - -path
      - /migrations/
      - -database
      - postgres://${POSTGRES_USER:-auth}:${POSTGRES_PASSWORD:-auth}@auth-postgres:5432/${POSTGRES_DATABASE:-auth}?sslmode=disable
      - up
    profiles: [test]

  auth-app:
    build:
      context: ${AUTH_REPO_PATH:-../../../auth}
      target: production
    container_name: auth-client-go-test-auth
    env_file:
      - test.env
    depends_on:
      auth-postgres:
        condition: service_healthy
      auth-migrate:
        condition: service_completed_successfully
    ports:
      - "50051:50051"
    volumes:
      - auth-jwt-keys:/keys
    # HEALTHCHECK inherited from Dockerfile production stage:
    #   ./auth healthcheck --http http://localhost:53000/healthcheck --grpc localhost:50051
    profiles: [test]

volumes:
  auth-postgres-data:
    name: auth-client-go-test-postgres-data
  auth-jwt-keys:
    name: auth-client-go-test-jwt-keys
```

**Key decisions**:
- `build.context` параметризован через `${AUTH_REPO_PATH:-../../../auth}` — путь относительно файла compose.test.yaml. Default ведёт к `~/Development/TRADING/bots/auth` при сохранении конвенции расположения репозиториев.
- `target: production` — финальный stage Dockerfile, бинарь `./auth` + HEALTHCHECK + ничего лишнего (alpine:3.20).
- HEALTHCHECK не переопределяется — наследуется из Dockerfile, `docker compose up -d --wait auth-app` блокирует до healthy.
- `auth-jwt-keys` — named volume; AUTH server при первом старте создаст `ed25519_private.pem` + `ed25519_public.pem`. `env-down -v` удалит volume → следующий `env-up` → свежий keypair.
- `name: auth-client-go-test` (project name) — изолирует от других compose-проектов на машине.

---

### Task 4: Create extra.mk with test environment targets

**Action**: CREATE
**File**: [dev/deployments/extra.mk](../../dev/deployments/extra.mk)

**Description**: Makefile-фрагмент с целями для управления тестовым окружением. Подключается через `-include` в основной Makefile.

**Details**:
- Используем переменную `_COMPOSE` для централизации `docker compose -f dev/deployments/compose.test.yaml --env-file dev/deployments/test.env`.
- `dev-up-test-env` поднимает всё необходимое: postgres, migrate (one-shot), auth-app — `up -d --wait` блокирует до healthy.
- `dev-down-test-env` — `down -v --remove-orphans` (удаляет volumes!).
- `env-up`/`env-down` — алиасы (per task description «Расширить Makefile. Добавить таргеты: env-up, env-down, test-integration»).
- `test-integration` — env-up → run tests → env-down c гарантированным cleanup через `TEST_EXIT=$$?` паттерн (sibling-repo convention, проверен в auth/signal-analyzer/funding-bot).

**Content**:

```makefile
# dev/deployments/extra.mk
#
# Integration test environment management for auth-client-go SDK.
# Included from the main Makefile via `-include dev/deployments/extra.mk`.

# docker compose с указанием compose файла и env-файла
_COMPOSE := docker compose \
    -f dev/deployments/compose.test.yaml \
    --env-file dev/deployments/test.env

.PHONY: dev-up-test-env dev-down-test-env env-up env-down test-integration

##@ Integration Testing

dev-up-test-env: ## Bring up AUTH test environment (postgres + migrations + auth-app), wait until healthy
	@echo "━━━ Starting AUTH test environment ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	${_COMPOSE} --profile test up -d --wait auth-postgres
	@echo "Running AUTH migrations..."
	${_COMPOSE} --profile test run --rm auth-migrate
	@echo "Starting AUTH server (waiting for healthy)..."
	${_COMPOSE} --profile test up -d --wait auth-app
	@echo "✅ AUTH test environment ready (gRPC on localhost:50051)"

dev-down-test-env: ## Tear down AUTH test environment and delete volumes
	@echo "━━━ Stopping AUTH test environment ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	${_COMPOSE} --profile test down -v --remove-orphans
	@echo "✅ AUTH test environment stopped"

env-up: dev-up-test-env ## Alias for dev-up-test-env

env-down: dev-down-test-env ## Alias for dev-down-test-env

test-integration: $(TEST_RUNNER) dev-up-test-env ## Run integration tests against real AUTH (auto env up/down)
	@echo "━━━ Integration Tests ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@set +e; \
		RUN_INTEGRATION_TESTS=true AUTH_ENDPOINT=localhost:50051 \
		$(TEST_RUNNER) -tags integration --title="Integration Tests" ./lib/...; \
		TEST_EXIT=$$?; \
		echo ""; \
		$(MAKE) --no-print-directory dev-down-test-env; \
		exit $$TEST_EXIT
```

**Key decisions**:
- `_COMPOSE` (а не переменная окружения) — соответствует sibling-конвенции и хорошо читается.
- Постгрес и migrate подняты последовательно, чтобы `auth-app` уже стартовал в чистой схеме. `--wait` блокирует до healthy.
- `test-integration` зависит и от `$(TEST_RUNNER)` (бинарник из `dev/tools/`) и от `dev-up-test-env` (env должен подняться до запуска тестов). Если env-up упал — make прерывается, и dev-down уже НЕ вызывается. Чтобы env гарантированно гасился даже при сбое env-up, добавляем `set +e; ...; exit $$TEST_EXIT` обвязку вокруг всего тестового шага: если тесты завершились ненулевым кодом — dev-down выполнится через `$(MAKE) --no-print-directory dev-down-test-env`.
- Альтернатива с `trap` отброшена — research показал, что Makefile-рецепты ненадёжны для trap-семантики (sibling repos используют TEST_EXIT pattern).

---

### Task 5: Update main Makefile

**Action**: MODIFY
**File**: [Makefile](../../Makefile)

**Description**: Подключить extra.mk и обновить .PHONY список.

**Details**:
- Найти строку `-include dev/tools/tools.mk` (line 14) и сразу после неё добавить `-include dev/deployments/extra.mk`.
- В .PHONY-строке добавить `test-integration env-up env-down dev-up-test-env dev-down-test-env`.

**Patch** (Edit):

```diff
 # Include dev tools
 -include dev/tools/tools.mk
+-include dev/deployments/extra.mk

-.PHONY: test test-race bench coverage lint lint-fix fmt vet audit tidy clean build-examples help init-mem-bank update-mem-bank release release-dry-run release-with-notes generate-mocks
+.PHONY: test test-race bench coverage lint lint-fix fmt vet audit tidy clean build-examples help init-mem-bank update-mem-bank release release-dry-run release-with-notes generate-mocks test-integration env-up env-down dev-up-test-env dev-down-test-env
```

---

### Task 6: Create integration test placeholder

**Action**: CREATE
**File**: [lib/client_integration_test.go](../../lib/client_integration_test.go)

**Description**: Минимальный файл с build tag и `TestMain`, который:
1. Поддерживает `singleflight` как direct-dependency (blank import).
2. Гейтит запуск через `RUN_INTEGRATION_TESTS=true`.
3. Содержит trivial smoke test, валидирующий что `AUTH_ENDPOINT` доступен (TCP-probe). Не более — реальные кейсы добавляются в TASK-003..004.

**Details**:
- Build tag `//go:build integration` — `make test` (без tag'а) этот файл пропускает; `make test-integration` (с `-tags integration`) — включает.
- Blank import singleflight держит его в direct-require'ах.
- TestMain: если `RUN_INTEGRATION_TESTS != "true"` — выходим без запуска (паттерн из signal-analyzer).
- Один smoke-тест `TestAuthEndpointReachable` — `net.DialTimeout` на `AUTH_ENDPOINT` (по умолчанию `localhost:50051`) с таймаутом 5s. Это проверяет, что `make env-up` довёл AUTH до сети — но не валидирует gRPC handshake (это работа TASK-003).

**Content**:

```go
//go:build integration

package lib_test

import (
	"net"
	"os"
	"testing"
	"time"

	// Blank import keeps golang.org/x/sync as a direct dependency
	// until TASK-004 wires singleflight into the refresh flow.
	_ "golang.org/x/sync/singleflight"
)

func TestMain(m *testing.M) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestAuthEndpointReachable(t *testing.T) {
	endpoint := os.Getenv("AUTH_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:50051"
	}
	conn, err := net.DialTimeout("tcp", endpoint, 5*time.Second)
	if err != nil {
		t.Fatalf("dial AUTH at %s: %v", endpoint, err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close conn: %v", err)
	}
}
```

**Notes**:
- Пакет `lib_test` (external test package) — соблюдает изоляцию интеграционного теста от внутренней реализации, не требует доступа к unexported. Когда TASK-003+ добавит реальные тесты — этот же пакет подойдёт.
- Никаких mock'ов, никаких `nolint`, никаких ignored errors (ES-0060: `conn.Close()` чекаем).
- TestMain без панике-recover — `go test` сам репортит панику.

---

### Task 7: Update GitHub Actions CI

**Action**: MODIFY
**File**: [.github/workflows/ci.yaml](../../.github/workflows/ci.yaml)

**Description**: Добавить `workflow_dispatch` с boolean input, `GOPRIVATE` env во все jobs, новый `integration` job.

**Details**:
- Добавить `workflow_dispatch.inputs.run_integration` (boolean, default false).
- В каждый job добавить `env: GOPRIVATE: github.com/webdelo/*` (рядом с уже существующим git config).
- Новый job `integration`:
  - `if: ${{ inputs.run_integration == true }}` — пропускает при push/pull_request.
  - `continue-on-error: true` — non-blocking.
  - Steps: checkout SDK → checkout `webdelo/auth` в `auth-server/` → setup-go → git config private modules → `make test-integration` с `AUTH_REPO_PATH=$GITHUB_WORKSPACE/auth-server`.

**Patch (full content of ci.yaml)**:

```yaml
name: CI

on:
  push:
    branches: [main, master]
  pull_request:
    branches: [main, master]
  workflow_dispatch:
    inputs:
      run_integration:
        description: 'Run integration tests (requires Docker + access to webdelo/auth repo)'
        type: boolean
        default: false

env:
  GOPRIVATE: github.com/webdelo/*

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Configure git for private modules
        run: git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v8
        with:
          version: v1.64.8

  test:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Configure git for private modules
        run: git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"

      - name: Run unit tests
        run: go test -v -race ./lib/...

  build:
    name: Build
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Configure git for private modules
        run: git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"

      - name: Build library
        run: go build ./lib/...

      - name: Build examples
        run: |
          for dir in lib/examples/*/; do
            go build "./${dir}"
          done

  integration:
    name: Integration Tests
    runs-on: ubuntu-latest
    if: ${{ inputs.run_integration == true }}
    continue-on-error: true
    steps:
      - name: Checkout SDK
        uses: actions/checkout@v4

      - name: Checkout webdelo/auth (sibling repo for compose build)
        uses: actions/checkout@v4
        with:
          repository: webdelo/auth
          token: ${{ secrets.GH_PAT }}
          path: auth-server

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Configure git for private modules
        run: git config --global url."https://${{ secrets.GH_PAT }}@github.com/".insteadOf "https://github.com/"

      - name: Run integration tests
        env:
          AUTH_REPO_PATH: ${{ github.workspace }}/auth-server
        run: make test-integration
```

**Notes**:
- `workflow_dispatch` без `run_integration=true` → `inputs.run_integration` is `null` → условие false → job скипается. Это покрывает дефолтное поведение push/PR.
- `continue-on-error: true` оставляет красную/жёлтую галочку, но не блокирует merge (ADR §A13).
- `GH_PAT` secret уже существует в org (per CLAUDE.md / общая практика репозиториев `webdelo/*`). Проверить наличие — отдельный pre-flight перед merge'ем PR.
- Сборка AUTH image из исходников в CI — медленно (~3 мин). Пока приемлемо. Когда выйдет GHCR-image (отдельный DevOps REQ) — заменить `Checkout webdelo/auth` + `build:` на `image:` в compose.

---

### Task 8: Update README

**Action**: MODIFY
**File**: [README.md](../../README.md)

**Description**: Заменить placeholder-текст на минимально-полезный SDK overview + новый раздел «Local integration testing».

**Details**:
- Существующий README — голый шаблон. TASK-007 наполнит его полностью (quickstart, API, errors, examples). В TASK-001 ограничиваемся:
  - Краткое описание SDK (1 абзац).
  - Раздел Development → команды.
  - **Новый раздел «Local integration testing»** — prereqs, команды, caveat.
- Не делаем больше — это TASK-001-scope.

**Content** (полная замена):

```markdown
# auth-client-go

[![Go Reference](https://pkg.go.dev/badge/github.com/webdelo/auth-client-go.svg)](https://pkg.go.dev/github.com/webdelo/auth-client-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/webdelo/auth-client-go)](https://goreportcard.com/report/github.com/webdelo/auth-client-go)

Go SDK for [webdelo/auth](https://github.com/webdelo/auth). Thin wrapper over the gRPC contract from
[webdelo/auth-proto](https://github.com/webdelo/auth-proto) with non-trivial concerns baked in:
ValidateToken cache, HTTP middleware, gRPC unary interceptor, refresh race serialization, typed errors.

> Status: **v0.1.0 — in development.** Public API is being filled in across REQ-001 tasks.
> See [task_management/tasks/REQ-001.md](task_management/tasks/REQ-001.md) for the umbrella plan.

## Installation

```bash
go get github.com/webdelo/auth-client-go@latest
```

Requires `GOPRIVATE=github.com/webdelo/*` and a Personal Access Token with read access to the
`webdelo` organization (the contract module `github.com/webdelo/auth-proto/go` is private).

## Development

### Prerequisites

- Go 1.24+
- golangci-lint v1.64.8
- (For integration tests only) Docker + docker-compose plugin
- (For integration tests only) `webdelo/auth` repo cloned as sibling: `../auth` relative to this repo

### Commands

```bash
make help                # Show all available commands
make test                # Run unit tests
make test-race           # Run unit tests with race detector
make coverage            # Generate coverage report
make lint                # Run linter
make audit               # vet + lint + test
make build-examples      # Build example programs
make tidy                # go mod tidy
```

### Local integration testing

Integration tests run the SDK against a real `webdelo/auth` gRPC server in Docker.

**Prerequisites**:

- Docker + docker-compose plugin available.
- `webdelo/auth` repo cloned alongside this one — by default
  `dev/deployments/compose.test.yaml` builds the auth image from `../../../auth` (relative to
  the compose file). Override via `AUTH_REPO_PATH` env var if your layout differs.

**Commands**:

```bash
make env-up              # Bring up AUTH (Postgres + migrations + auth server), wait for healthy
make env-down            # Tear down AUTH and delete volumes
make test-integration    # Full cycle: env-up → run integration tests → env-down (guaranteed)
```

`make test-integration` always tears down the environment, even when tests fail.

**CI**: integration tests are an opt-in job triggered via `workflow_dispatch` with
`run_integration=true`. Run via the GitHub Actions UI or:

```bash
gh workflow run ci.yaml -f run_integration=true
```

This job has `continue-on-error: true` — it does not block PR merges. The auth image is not yet
published to GHCR (tracked separately under a DevOps REQ); for now the CI job checks out the
`webdelo/auth` repo and builds the image on every run.

## License

Internal — webdelo organization.
```

---

### Task 9: Update .gitignore (if needed)

**Action**: MODIFY (verify; create if missing)
**File**: [.gitignore](../../.gitignore)

**Description**: Убедиться, что `.temp/`, `coverage/`, `bin/` игнорятся. Compose.test.yaml не пишет файлы в репозиторий (volumes managed by Docker daemon), отдельных entries не требуется.

**Details**:
- Если `.gitignore` уже содержит указанные паттерны — изменений не делаем.
- Если отсутствует или неполный — дополнить.

Минимально-необходимое содержимое:

```gitignore
# Build artefacts
bin/
coverage/

# Temp files (per CLAUDE.md policy: never write to project root, use .temp/)
.temp/

# Editor / IDE
.idea/
.vscode/
*.swp
.DS_Store

# Go
*.test
*.out

# Mockery generated mocks (regenerated via make generate-mocks)
# Adjust path once mockery.yaml output is known
```

---

## Integration

### Config

| File | Purpose |
|------|---------|
| `dev/deployments/test.env` | AUTH server ENV + SDK test ENV (`AUTH_ENDPOINT`, `AUTH_REPO_PATH`, `RUN_INTEGRATION_TESTS`) |
| `dev/deployments/compose.test.yaml` | Compose definition (auth-postgres + auth-migrate + auth-app) |
| `dev/deployments/extra.mk` | Makefile targets (`env-up`, `env-down`, `test-integration`) |

### Dependencies (go.mod direct)

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/webdelo/auth-proto/go` | `v0.1.0` | gRPC contract — single source of truth for `auth.v1` |
| `google.golang.org/grpc` | (MVS from auth-proto/go transitive: `v1.79.3`) | gRPC framework |
| `golang.org/x/sync` | (MVS, ≥ `v0.20.0`) | `singleflight` for refresh race serialization (used in TASK-004) |

**Forbidden direct dependencies** (DOD enforces `go mod why ... = not needed`):
- `github.com/sirupsen/logrus`
- `go.uber.org/zap`
- `go.uber.org/fx`
- `github.com/google/wire`
- `github.com/prometheus/client_golang`

`google.golang.org/protobuf` остаётся как indirect — SDK не импортирует protobuf-runtime напрямую, его тянет `auth-proto/go`.

### CI

| Component | Change |
|-----------|--------|
| Triggers | + `workflow_dispatch.inputs.run_integration` (boolean) |
| Top-level `env` | + `GOPRIVATE: github.com/webdelo/*` |
| Job `lint` / `test` / `build` | без структурных изменений (GOPRIVATE наследуется от top-level env) |
| Job `integration` | NEW: gated by input, `continue-on-error: true`, checks out auth repo, runs `make test-integration` |

---

## Testing

### Unit Tests

TASK-001 не вводит unit-testable Go-кода. Существующий `lib/lib_test.go::TestVersion` должен продолжать проходить (smoke).

**Required test cases**:
- [x] (existing) `lib_test.go::TestVersion` — Version constant ≠ ""

### Integration Tests

Один placeholder smoke-кейс `TestAuthEndpointReachable` для валидации инфраструктуры. Реальные SDK кейсы (Validate, Login, Refresh с race, Logout, etc.) добавляются в TASK-003..004.

| Entity | Test File | Cases |
|--------|-----------|-------|
| AUTH server reachability | [lib/client_integration_test.go](../../lib/client_integration_test.go) | TCP dial на `AUTH_ENDPOINT` с таймаутом 5s; gated by `//go:build integration` + `RUN_INTEGRATION_TESTS=true` |

**Required test cases**:
- [ ] `TestAuthEndpointReachable` — happy path: AUTH gRPC порт открыт после `env-up`.

### Mocks

Не используются — TASK-001 не имеет интерфейсов. `mockery.yaml` остаётся как есть. `make generate-mocks` пока no-op (предупреждение допустимо).

### Coverage

TASK-001 не влияет на coverage SDK (нет production-кода). Существующий `make coverage` должен продолжать работать (отчёт по `lib/lib.go`).

---

## Validation

### Build & Lint

```bash
# Зависимости и go.mod
go mod tidy
go mod verify
go mod why github.com/webdelo/auth-proto/go    # → direct
go mod why google.golang.org/grpc              # → direct
go mod why golang.org/x/sync                   # → direct
go mod why github.com/sirupsen/logrus           # → not needed
go mod why go.uber.org/fx                       # → not needed
go mod why github.com/prometheus/client_golang  # → not needed

# Стандартные quality gates
make fmt
make vet
make lint
make test
make audit
```

### Integration Lifecycle

```bash
# 1) Поднять окружение, дождаться healthy
make env-up

# 2) Проверить, что AUTH gRPC слушает
nc -z localhost 50051 && echo "AUTH up"

# 3) Прогнать smoke integration
make test-integration

# 4) Проверить, что env погашен (containers + volumes удалены)
docker ps -a --filter "label=com.docker.compose.project=auth-client-go-test"
docker volume ls --filter "name=auth-client-go-test"
# обе команды → пусто
```

### CI Manual Checks

1. После merge в main: открыть GitHub Actions → Workflows → CI → Run workflow → `run_integration: true` → дождаться completion. Job `integration` должен показать success (или yellow при сбое сборки AUTH image — это допустимо).
2. Обычный push: job `integration` отсутствует в run summary.
3. Открыть PR без integration label: lint/test/build обязательно проходят, integration не отображается.

### Failure-Mode Sanity

| Сценарий | Ожидание |
|----------|----------|
| `webdelo/auth` репо не клонирован → `make env-up` | `compose build` падает с понятным «directory does not exist» сообщением; env-up exit non-zero |
| AUTH server не дошёл до healthy (например, миграции не отработали) | `docker compose up -d --wait auth-app` блокирует и exit non-zero после default timeout (60s) |
| Тест в test-integration упал | Видим красный output теста, env гасится через `dev-down-test-env`, exit non-zero |
| `make test-integration` прервали (Ctrl+C) | docker контейнеры остаются работать; пользователь должен запустить `make env-down` вручную. **Документируем в README** как known limitation v0.1.0 |
| Лишний direct-dep попал в go.mod через `go get` ошибочно | `go mod why ... = direct` для запрещённого — code review TASK-001 ловит до merge |

---

## DOD (Definition of Done)

- [ ] `go.mod` содержит `go 1.24` (downgrade с `1.26.2`) и три direct-require: `github.com/webdelo/auth-proto/go v0.1.0`, `google.golang.org/grpc`, `golang.org/x/sync`.
- [ ] `go.sum` создан, `go mod verify` проходит.
- [ ] `go mod why github.com/webdelo/auth-proto/go` подтверждает direct dependency.
- [ ] `go mod why` для `github.com/sirupsen/logrus`, `go.uber.org/fx`, `github.com/prometheus/client_golang` → «not needed».
- [ ] Файл `dev/deployments/test.env` создан с полным набором AUTH ENV и `AUTH_ENDPOINT=localhost:50051`.
- [ ] Файл `dev/deployments/compose.test.yaml` создан и поднимает `auth-postgres` + `auth-migrate` + `auth-app` (профиль `test`).
- [ ] Файл `dev/deployments/extra.mk` создан с целями `dev-up-test-env`, `dev-down-test-env`, `env-up`, `env-down`, `test-integration`.
- [ ] `Makefile` обновлён: `-include dev/deployments/extra.mk` и новые цели в `.PHONY`.
- [ ] `make env-up` поднимает AUTH и доводит `auth-app` до `healthy` ≤ 30s (под нагрузкой ноутбука можно расширить до 60s).
- [ ] `make env-down` останавливает контейнеры и удаляет volumes (`auth-postgres-data`, `auth-jwt-keys`).
- [ ] `make test-integration` всегда вызывает `dev-down-test-env` — даже если тесты упали (проверяется ручным запуском с заведомо ломаемым тестом, после возвращается обратно).
- [ ] `lib/client_integration_test.go` создан с `//go:build integration`, `TestMain` gate'ом через `RUN_INTEGRATION_TESTS=true`, smoke-тестом `TestAuthEndpointReachable`, blank import `golang.org/x/sync/singleflight`.
- [ ] `.github/workflows/ci.yaml` обновлён: `workflow_dispatch.inputs.run_integration`, `GOPRIVATE: github.com/webdelo/*` на top-level, новый job `integration` с `if: ${{ inputs.run_integration == true }}` и `continue-on-error: true`.
- [ ] `README.md` обновлён: раздел «Local integration testing» с prereqs, командами и caveat про опциональный CI.
- [ ] `make audit` проходит чисто.
- [ ] Существующие unit-тесты не сломаны (`make test` зелёный).
- [ ] Code review пройден; ни одного `//nolint:` (ES-0171), ни одного silent error в новом коде (ES-0060).

---

## PRP Quality Score

**Самооценка: 9/10**

| Critique | Acknowledgement |
|----------|----------------|
| ✅ Все DOD из task.md покрыты конкретными Implementation Tasks. | Полное соответствие. |
| ✅ Все шесть Gotchas из ADR явно отражены в Gotchas-таблице с решениями. | Включая Go-version mismatch, MVS-выбор grpc/protobuf, JWT keys auto-gen. |
| ✅ Снимок реальной структуры AUTH (Dockerfile production target, Ed25519 keypair via LoadOrGenerateKeyPair, port 50051 + 53000) — подтверждён research-агентом и личной проверкой `key_pair.go`. | Не базируется на предположениях. |
| ✅ Sibling-конвенция (`TEST_EXIT` pattern, `--wait`, `--profile test`) — заимствована из auth/signal-analyzer, что гарантирует совместимость с командой. | См. research report §1.1–1.3. |
| ✅ CI-gate соответствует ADR §A13 (workflow_dispatch + continue-on-error). | Без отсебятины. |
| ⚠ Минус 1 балл: integration CI job требует GH_PAT доступ к `webdelo/auth` репо. Если PAT истёк или права не выданы — job будет красным даже при корректной инфре. Это accepted limitation (см. Risk #6 в REQ-001), задокументировано в README. Pre-flight перед merge — убедиться, что org-secret `GH_PAT` валиден. | Реальный риск, не purely-cosmetic. |

**Прохождение порога 7/10 — да, PRP готов к review и execution в TASK-001 workflow (execute-prp).**

---

## Out-of-Scope (явные исключения)

Чтобы code review не воспринимал отсутствие следующих пунктов как пропуск:

1. **Любая реализация SDK** (Client, Config, errors, validate, middleware) — это TASK-002+.
2. **mockery-конфигурация** — `mockery.yaml` уже существует, генерация моков активна в TASK-003+ (когда появятся первые интерфейсы).
3. **DevOps REQ на публикацию AUTH-image в GHCR** — отдельный track, не блокирует SDK v0.1.0. После публикации compose.test.yaml поменяется build → image (additive change).
4. **Distributed singleflight (через Redis)** — accepted limitation для v0.1.0 (REQ-001 Risk #3), не входит в scope ни одной TASK.
5. **WithTrustedProxies / IPFromRequest расширения** — отложены до v0.2.0 (TASK-005 покрывает только базовый chain X-Real-IP → XFF[0] → RemoteAddr).
6. **Metrics interface** — отложен до v0.2.0 (REQ-001 §A4, ADR §A2.1.2).
