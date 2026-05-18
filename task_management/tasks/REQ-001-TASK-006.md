# Examples — basic, http_middleware, grpc_interceptor

---

## REQ
**Родительский реквест**: [REQ-001: SDK v0.1.0 — реализация Go-клиента для webdelo/auth](./REQ-001.md)

**Priority**: medium
**Depends on**: REQ-001-TASK-003, REQ-001-TASK-004, REQ-001-TASK-005

---

## WHY

Три рабочих примера в `lib/examples/` доказывают, что SDK реально работает в типовых сценариях, и служат живой документацией для consumer'ов. Каждый пример — минималистичная демонстрация ровно одной фичи (basic flow, HTTP middleware, gRPC interceptor). Если BFF-разработчик за < 30 минут собирает working middleware (см. DOD umbrella) — examples + README сделали свою работу.

---

## WHAT

1. **Создать `lib/examples/basic/main.go`.** Демонстрирует базовый flow: `New` → `Login` → `ValidateAccessToken` → `Logout` → `Close`. ENV-driven config (`AUTH_ENDPOINT`, `AUTH_TEST_EMAIL`, `AUTH_TEST_PASSWORD`). Вывод в stdout: каждый шаг + результат (без вывода токенов целиком — первые 8 символов для демо).
2. **Создать `lib/examples/http_middleware/main.go`.** HTTP-сервер на `net/http`:
    - Endpoint `/api/profile` защищён через `client.HTTPMiddleware`, handler читает `UserFromContext` и пишет JSON `{uuid, role}`.
    - Endpoint `/auth/login` (без middleware) — принимает `{email, password}`, вызывает `client.Login`, ставит `SetRefreshCookie`, возвращает `{access_token, expires_in, user}` в JSON.
    - Endpoint `/auth/logout` — читает refresh cookie, вызывает `client.Logout`, чистит cookie через `ClearRefreshCookie`.
    - Demo client использует `IPFromRequest`/`UserAgentFromRequest` для Login.
    - Логирование через `slog` adapter (показать, как обернуть `slog.Logger` в SDK `Logger` interface).
3. **Создать `lib/examples/grpc_interceptor/main.go`.** gRPC-сервер:
    - Тривиальный сервис `Echo` (proto inline через `reflection` пакет или предсгенерированные stubs в `examples/grpc_interceptor/proto/` — выбрать упрощённый вариант: использовать `grpc.NewServer(grpc.UnaryInterceptor(client.UnaryInterceptor()))` без custom proto, через `grpc.health.v1.Health` сервис как demo target).
    - Один метод требует auth, второй (`/grpc.health.v1.Health/Check`) пропускается через дефолтный `openMethods` registry.
    - Demo client отправляет 3 RPC: с валидным токеном, без токена, на health-check без токена.
4. **Создать README в каждом примере.** `lib/examples/basic/README.md`, `lib/examples/http_middleware/README.md`, `lib/examples/grpc_interceptor/README.md`. Содержит: цель примера, prerequisites (AUTH endpoint + test user), команды `go run`, ожидаемый output.
5. **Удалить placeholder `lib/examples/basic/main.go`** (текущий печатает `Version`) — заменить на полноценный basic.
6. **Обновить Makefile `build-examples`.** Должен собирать все три примера (`go build -o .temp/examples/<name> ./lib/examples/<name>`) без warnings. Запускать без аргументов — печатает usage и выходит с кодом 0 (для CI: build success ≠ run).
7. **Не добавлять зависимости consumer-уровня.** Logger в http_middleware — `slog` (stdlib, Go 1.21+); JSON — `encoding/json` (stdlib). Цель — пример не приносит deps в SDK go.sum.

### Акценты

- _Триггерные правила: ES-0051, ES-0140, ES-0143._
- _Examples — отдельные `main` пакеты в `lib/examples/<name>/`. SDK go.mod уже знает про них (build-examples прогоняется в CI)._
- _Минимализм: каждый пример демонстрирует ровно одну фичу. Не нагромождать config-driven сложность._
- _Использовать SDK API строго через публичные функции (`authclient.New`, `authclient.IPFromRequest`, etc.) — никаких `internal/` импортов в examples._
- _README каждого примера: < 50 строк, формат «What / Run / Expected output»._

---

## DOD (Definition of Done)

- [ ] `lib/examples/basic/main.go` собирается, демонстрирует Login → Validate → Logout flow.
- [ ] `lib/examples/http_middleware/main.go` собирается, имеет 3 endpoint'а (`/api/profile` защищённый, `/auth/login`, `/auth/logout`).
- [ ] `lib/examples/grpc_interceptor/main.go` собирается, использует `grpc.UnaryInterceptor(client.UnaryInterceptor())`, демонстрирует skip `Health/Check`.
- [ ] Каждый пример имеет README.md с командой запуска и expected output.
- [ ] `make build-examples` собирает все три без warnings и без новых записей в `go.sum`.
- [ ] Старый placeholder `lib/examples/basic/main.go` заменён.
- [ ] Examples используют только публичный API SDK (никаких `internal/` импортов — проверяется `go list -deps ./lib/examples/... | grep internal | wc -l == 0`).
- [ ] Examples не добавляют сторонних deps (logger/cli framework) — `go.sum` после build-examples идентичен `go.sum` до.
- [ ] `make audit` проходит.
