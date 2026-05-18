# REQ-001-TASK-007-PRP: README + godoc + CHANGELOG + tag v0.1.0

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | REQ-001-TASK-007-PRP |
| Parent Request | [REQ-001](../tasks/REQ-001.md) |
| Parent Task | [REQ-001-TASK-007](../tasks/REQ-001-TASK-007.md) |
| Parent ADR | [REQ-001-ADR](./REQ-001-ADR.md) |
| Created | 2026-05-18 |
| Updated | 2026-05-18 |
| Score | 8/10 — −1 за хрупкость BFF-smoke-test (зависит от внешнего разработчика, мягкий критерий, не блокер DOD); −1 за отсутствие CI-gate на godoc-completeness — `revive`/`godot` не активированы, проверка ручная через `go doc`-пайп. Чтобы поднять до 9: после merge — включить `revive.exported` в `.golangci.yaml` (out-of-scope этого TASK'а; tracked separately). |

---

## Goal

### Summary

Финализировать публикуемый артефакт `webdelo/auth-client-go@v0.1.0`: создать `lib/doc.go` (package godoc + quickstart + external links), переписать `README.md` под consumer-perspective (14 секций), записать `## v0.1.0 — 2026-05-18` в `CHANGELOG.md`, удостовериться, что godoc на всех публичных идентификаторах TASK-004/005/006 на месте, и опубликовать git-тег `v0.1.0` через `make release`. Завершить smoke-валидацией BFF-разработчиком (`go get @v0.1.0` → рабочий middleware за < 30 минут).

### Why

- **Без README SDK нельзя начать использовать.** Текущий `README.md` — developer-contributor-only (как собрать/тестировать), consumer-секций (Quickstart, Config, FailMode, Errors, Cookies, TLS, Troubleshooting) нет. BFF не сможет интегрироваться, опираясь на текущий файл.
- **Без `lib/doc.go` package-godoc разорван.** Сейчас package-doc живёт в `lib/errors.go:1-8` как сайд-эффект алфавитного порядка — это сработает, но идиоматично выносить в `doc.go` и добавлять туда quickstart (`go doc github.com/webdelo/auth-client-go/lib` будет выводить его первым блоком).
- **Без `## v0.1.0` секции в CHANGELOG апгрейд опасен.** Consumer не сможет диффить v0.1.0 → v0.2.0. Keep-a-Changelog формат — стандарт для Go OSS.
- **Без git-тега `v0.1.0` `go get @v0.1.0` ломается.** REQ-001 умbrella-DOD требует `BFF может через go get @v0.1.0 импортировать SDK и собрать middleware за < 30 минут`. Тег — единственный способ.
- **TASK-007 — последняя задача REQ-001.** Закрытие этого TASK'а ⇒ закрытие всего REQ-001, ⇒ unblock BFF-интеграции.

### What

После выполнения:

```bash
# Package godoc:
go doc github.com/webdelo/auth-client-go/lib
# → "Package authclient is the official Go SDK for the webdelo/auth service.
#     ...quickstart...links to webdelo/auth and webdelo/auth-proto..."

# Public surface:
go doc github.com/webdelo/auth-client-go/lib | grep -cE "^(func|type|var|const)"
# → ≥ 35 (5 sentinels + 4 With*Option + 5 With*CookieOption + 9 Client methods + 4 helpers
#         + Config + 2 enums + Role const + User type + 3 User methods + Logger/NoopLogger/Cache
#         + LoginInput/LoginOutput/RefreshOutput + CookieOption + AuthError + RetryAfter)

# Tag:
GOPROXY=direct go list -m github.com/webdelo/auth-client-go@v0.1.0
# → github.com/webdelo/auth-client-go v0.1.0

# README:
grep -cE "^## " README.md
# → ≥ 14 секций согласно WHAT §3 родительского TASK-007

# BFF-validator (другой разработчик):
go get github.com/webdelo/auth-client-go@v0.1.0
# → импорт + middleware за < 30 минут
```

### Success Criteria

- [ ] `lib/doc.go` создан; содержит package-godoc + 5-строчный quickstart + ссылки на `https://github.com/webdelo/auth` и `https://github.com/webdelo/auth-proto`.
- [ ] Package-doc удалён из `lib/errors.go:1-8` (чтобы не было дублирования) — заменён на просто `package authclient`.
- [ ] `README.md` содержит **все 14 секций** из WHAT §3 TASK-007 (Installation, Quickstart, Configuration table, FailMode, Typed errors, Adapter interfaces, Cookie helpers, HTTP request helpers, Examples, Local integration testing, TLS setup, Troubleshooting, Known limitations, Versioning policy).
- [ ] `README.md` install-команда обновлена `@latest` → `@v0.1.0`.
- [ ] CAPSLOCK security warning про `FailOpen` в README (`## SECURITY WARNING` или эквивалент).
- [ ] Таблица typed errors (gRPC code → SDK sentinel) + пример `errors.Is` в README.
- [ ] `CHANGELOG.md` содержит запись `## [v0.1.0] — 2026-05-18` (Keep a Changelog format) с секциями `Added` (полный feature list TASK-001..006), `Known limitations`, `Acknowledgements`. `[Unreleased]` опустошён (или удалён).
- [ ] CHANGELOG footer-link: `[v0.1.0]: https://github.com/webdelo/auth-client-go/releases/tag/v0.1.0`.
- [ ] Godoc-аудит на TASK-004/005/006 deliverables (`lib/auth.go`, `lib/cookies.go`, `lib/helpers.go`, `lib/examples/*/main.go`) пройден: все exported имеют godoc; все начинаются с имени; `FailOpen`, `WithCookieSameSite` несут `SECURITY WARNING:` или CAPSLOCK warning.
- [ ] Публичная поверхность (`go doc github.com/webdelo/auth-client-go/lib | grep -E "^(type|func|var|const)"`) полностью совпадает с REQ-001-ADR §API (542-699): 5 sentinels, 4 `With*Option`, 4 `WithCookie*Option`, 9 методов `*Client`, 4 helpers — ни больше, ни меньше.
- [ ] Тег `v0.1.0` создан через `make release`, опубликован в `origin/main` (`git ls-remote --tags origin v0.1.0`).
- [ ] `GOPROXY=direct go list -m github.com/webdelo/auth-client-go@v0.1.0` возвращает `v0.1.0` без ошибок.
- [ ] `go mod why github.com/webdelo/auth-proto/go` подтверждает прямую зависимость `v0.1.0`.
- [ ] `make audit` + `make build-examples` + `make test-integration` зелёные перед тегированием.
- [ ] BFF-валидатор (другой разработчик) собрал рабочий middleware через `go get github.com/webdelo/auth-client-go@v0.1.0` за < 30 минут; отчёт в `task_management/results/REQ-001-TASK-007-bff-validation.md`.
- [ ] Umbrella DOD из [REQ-001 §DOD](../tasks/REQ-001.md) полностью закрыт (все 13 пунктов чек-листа).

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | **ES-0080** (Doc comments precede declaration), **ES-0081** (Start doc comments with declaration name), ES-0140 (Constructor name is `New`) — применимы к ревизии godoc на TASK-004/005/006 коде |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | **CS-0060** (Godoc comments for public methods), CS-0070 (English-only naming), CS-0111 (Minimal inline comments — относится к коду, не к doc-комментам), CS-0001 (Line length ≤ 120; для godoc-блоков соблюдать) |
| Coding Scope | [.temp/rules/REQ-001-TASK-007/coding_scope.md](../../.temp/rules/REQ-001-TASK-007/coding_scope.md) | 13 загруженных правил (CS-0050, ES-0040, ES-0044, ES-0051, ES-0055, ES-0056, ES-0060, ES-0061, ES-0062, ES-0063, ES-0064, ES-0065, ES-0140) — большинство неприменимы (TASK документация, не код), но проверяются как часть финального audit пройденного кода |
| Review Scope | [.temp/rules/REQ-001-TASK-007/review_scope.md](../../.temp/rules/REQ-001-TASK-007/review_scope.md) | По умолчанию полный набор |

### Concurrency Patterns
Не применимы — TASK документации/релиза, новый код не пишется (только markdown + один Go-файл `doc.go` без логики).

---

## Analysis Files

- **Code Research**: [2026-05-18_code-research_req-001-task-007-docs-and-release.md](../../.memory_bank/agent_task_results/2026-05-18_code-research_req-001-task-007-docs-and-release.md) — полный inventory godoc-покрытия (все 35+ exported на TASK-002/003 ✅), gap-анализ README (12 секций missing), CHANGELOG (v0.1.0 entry отсутствует), флоу `dev/tools/release.sh:1-401`, верификация что `export_test.go` НЕ загрязняет `go doc` (Q1 из ресерча — false positive: `go doc` исключает `_test.go` файлы, подтверждено `go doc -all` пайпом).
- **Parent ADR**: [REQ-001-ADR.md](./REQ-001-ADR.md) §API (lines 542-699) — каноничная публичная поверхность, против которой сверяется `go doc` в Task 8.
- **Predecessor PRPs** (для понимания feature-list в CHANGELOG):
  - [REQ-001-TASK-001-PRP.md](./REQ-001-TASK-001-PRP.md) — bootstrap, integration test infra
  - [REQ-001-TASK-002-PRP.md](./REQ-001-TASK-002-PRP.md) — Client, Config, errors, Logger
  - [REQ-001-TASK-003-PRP.md](./REQ-001-TASK-003-PRP.md) — ValidateAccessToken, cache, HTTPMiddleware, UnaryInterceptor
  - [REQ-001-TASK-004-PRP.md](./REQ-001-TASK-004-PRP.md) — Login/Refresh/Logout/LogoutAll/ChangePassword/GetCurrentUser + singleflight
  - [REQ-001-TASK-005-PRP.md](./REQ-001-TASK-005-PRP.md) — context, cookies, request helpers
  - [REQ-001-TASK-006-PRP.md](./REQ-001-TASK-006-PRP.md) — examples (basic, http_middleware, grpc_interceptor)

---

## Context

### Current Structure

```
.
├── README.md                 # ~85 строк; только developer/contributor-секции
├── CHANGELOG.md              # Keep-a-Changelog shell + stale [Unreleased] (pre-TASK-001 content)
├── go.mod                    # module github.com/webdelo/auth-client-go; go 1.24.0; auth-proto v0.1.0
├── Makefile                  # release / release-dry-run / release-with-notes таргеты (dev/tools/release.sh)
├── lib/
│   ├── client.go             # Config/FailMode/Option/Client + 4 With* — godoc ✅
│   ├── errors.go             # 5 sentinels + AuthError + RetryAfter + (!!) package-godoc в lines 1-8
│   ├── validate.go           # ValidateAccessToken + hashToken — godoc ✅
│   ├── context.go            # Role/User + WithUser/UserFromContext — godoc ✅
│   ├── logger.go             # Logger interface + NoopLogger — godoc ✅
│   ├── middleware_http.go    # HTTPMiddleware — godoc ✅
│   ├── interceptor_grpc.go   # UnaryInterceptor — godoc ✅
│   ├── (NO doc.go)
│   ├── (NO auth.go — TASK-004 prereq)
│   ├── (NO cookies.go — TASK-005 prereq)
│   ├── (NO helpers.go — TASK-005 prereq)
│   ├── examples/
│   │   └── basic/main.go     # placeholder (8 строк) — TASK-006 prereq для full demo
│   └── internal/
│       ├── cache/            # TTL cache (TASK-003)
│       └── tools/            # singleflight pin
└── dev/tools/release.sh      # 401-строчный bash; main-flow: gh+git checks → version prompt → tag origin/main → gh release create
```

### Desired Structure

```
.
├── README.md                 # REWRITTEN — 14 секций consumer-perspective (≥ 350 строк)
├── CHANGELOG.md              # UPDATED — ## [v0.1.0] — 2026-05-18 + Added/Known limitations/Acknowledgements
├── lib/
│   ├── doc.go                # NEW — package authclient godoc + quickstart 5 строк + ссылки
│   ├── errors.go             # UPDATED — package-godoc удалён из lines 1-8 (оставить только `package authclient`)
│   ├── (все остальные — без изменений, godoc уже на месте)
│   ├── auth.go               # CREATED в TASK-004; в TASK-007 — только godoc audit
│   ├── cookies.go            # CREATED в TASK-005; в TASK-007 — только godoc audit (WithCookieSameSite — SECURITY WARNING)
│   ├── helpers.go            # CREATED в TASK-005; в TASK-007 — только godoc audit
│   └── examples/
│       ├── basic/            # CREATED в TASK-006; в TASK-007 — без изменений
│       ├── http_middleware/  # CREATED в TASK-006; в TASK-007 — без изменений
│       └── grpc_interceptor/ # CREATED в TASK-006; в TASK-007 — без изменений
└── (git tag v0.1.0 на origin/main HEAD после merge)
```

### Gotchas

| Category | Issue | Solution |
|----------|-------|----------|
| Prereq gate | TASK-007 depends on TASK-004/005/006. На момент написания PRP (branch `feature/req-001-task-003`) этих файлов нет: `lib/auth.go`, `lib/cookies.go`, `lib/helpers.go`, `lib/examples/http_middleware/`, `lib/examples/grpc_interceptor/`. README/CHANGELOG ссылаются на их API. | Task 0 — pre-flight verify: проверить наличие всех файлов + сигнатур из ADR §API. Если что-то не сходится — STOP, эскалировать. README пишется ТОЛЬКО против реально merged кода. |
| Package-godoc duplication | Сейчас package-godoc в `lib/errors.go:1-8` (8 строк перед `package authclient`). После создания `lib/doc.go` нельзя оставлять godoc в обоих — `go doc` сольёт их и выдаст странный вывод. | Task 1: создать `lib/doc.go` с полным package-godoc. Task 1b: удалить doc-комментарий из `lib/errors.go:1-7`, оставить **только** строку `package authclient` (line 8). Verify: `head -12 lib/errors.go` — первая строка должна быть `package authclient`, без комментариев перед ней. |
| Tag must be on main HEAD | `dev/tools/release.sh:290-291` создаёт тег на `git rev-parse "origin/$MAIN_BRANCH"` — на HEAD `origin/main`, не на текущей ветке. | Перед `make release`: смержить feature-ветку TASK-007 в `main`, дождаться, что `git rev-parse origin/main` совпадает с локальной HEAD. Иначе тег ляжет на устаревший коммит (без README/doc.go). |
| `@latest` install hint | `README.md:16` использует `go get @latest` — до создания тега ломается с `go: cannot find ...: no matching versions`. | После публикации `v0.1.0` поменять на `@v0.1.0` ХОТЯ БЫ в Installation. В Quickstart допустимо `@latest` (для будущих минор-релизов). Решение: обе формы — Installation `@v0.1.0` (явная версия), отдельная подсказка про `@latest` ниже («for the most recent release»). |
| `export_test.go` НЕ загрязняет `go doc` | Researcher Q1 предположил, что `FakeAuthClient`, `ClientConfig`, `NewTestClient`, `HashToken`, `DefaultOpenMethods`, `BearerToken` из `lib/export_test.go` (package `authclient`, не `_test`) видны в `go doc`. **Verified false** — `go doc github.com/webdelo/auth-client-go/lib` НЕ читает `*_test.go` файлы (Go specification — test files compile только в test binary). Подтверждено пайпом `go doc -all <pkg> \| grep` — ни одного test-helper не показано. | НЕ ТРОГАТЬ `export_test.go`. Это false alarm. Note в Task 8 явно фиксирует это, чтобы reviewer не поднял повторно. |
| `revive`/`godot` lint gap | Linter не ловит missing godoc — `revive` enabled без `exported`-rule, `godot` не включён (`code-research §8`). Любой пропуск godoc на новых файлах TASK-004/005/006 пройдёт CI незаметно. | Task 2: ручной godoc-audit через `go doc -all <pkg>` + grep на отсутствующие комментарии. Также вручную проверить `lib/auth.go`, `lib/cookies.go`, `lib/helpers.go` glob'ом — каждый exported func/type/var/const должен иметь предшествующий `// Name ...` блок. Команда в Task 2. |
| `make test-integration` требует Docker и `../auth` | DOD требует `make test-integration` зелёным перед тегом. Это поднимает AUTH через `dev/deployments/compose.test.yaml` — нужен Docker daemon + клонированный `webdelo/auth` репо как sibling. | Pre-flight (Task 6): убедиться, что `docker info` отвечает и `../auth/Dockerfile` существует. Если нет — задокументировать в результатах TASK как **skipped with reason** и поднять на reviewer. **НЕ пропускать молча** — DOD-нарушение. |
| BFF-validator timing | DOD WHAT §7: другой разработчик собирает middleware за <30 мин **читая ТОЛЬКО README**. Это validation gate — но `go get @v0.1.0` работает только ПОСЛЕ публикации тега. Если README плох — тег нельзя «откатить» (можно `git tag -d` + `git push --delete`, но это публичная дисциплина). | Order: (1) подготовить README → (2) self-review (имитировать BFF-разработчика на dev-машине, локальный `go.mod replace` на путь репо) → (3) если ок — `make release` → (4) попросить второго разработчика валидировать против опубликованного `@v0.1.0`. Если второй разработчик ругается — fix README → опубликовать как `v0.1.1` (semver patch), `v0.1.0` оставить как есть (тег не удаляется). |
| `make release` интерактивный | `dev/tools/release.sh:211` читает version из `/dev/tty`. В non-interactive (CI, скрипты) — повиснет. | Запускать вручную с терминала. Если хочется автоматизации — `echo "0.1.0" \| make release` НЕ сработает (script читает напрямую с `/dev/tty`). Acceptable для v0.1.0; в будущем можно добавить `--version=X.Y.Z` flag. |
| `release-with-notes` (`--generate-notes`) использует `claude --print` | Если запустить `make release-with-notes` — script зовёт `claude --print` для AI-генерации release notes. Это нарушает CLAUDE.md «НЕ добавляй AI-метки в коммиты и PR» (project-level rule из `/Users/a.popov/Development/TRADING/CLAUDE.md`). | Использовать ТОЛЬКО `make release` (без `--generate-notes`). Release notes — basic git-log format (`release.sh:259-281`) — это просто bullet-list коммитов, без AI-маркировки. |
| README длина | README легко уйдёт за 1000 строк. Это анти-паттерн (пользователи не дочитают). | TASK-007 §Акценты: "README — не туториал, а reference. Quickstart короткий, остальное — таблицы и ссылки." Target: 350-500 строк, всё кроме Quickstart — таблицы/ссылки. Если выйдет за 600 — пересмотреть. |
| `## SECURITY WARNING` syntax | TASK требует CAPSLOCK warning про `FailOpen`. Markdown `##` создаст h2-заголовок — он попадёт в TOC. | Использовать blockquote-callout: `> ⚠️ **SECURITY WARNING — FAILOPEN MODE**` (или без emoji, текст всё-равно CAPSLOCK). Не h2. CAPSLOCK буквами — соответствует акценту task spec. |

---

## Implementation

### Data Models

Не применимо — TASK документации, новых типов нет. Все типы (Config, Client, User, LoginInput, LoginOutput, RefreshOutput, AuthError, CookieOption, FailMode, Role, Option, Logger, NoopLogger, Cache) определены в TASK-002/003/004/005.

---

### Task 0 (pre-flight): Verify all predecessor TASKs are merged

**Action**: VERIFY
**File**: — (нет изменений в коде)

**Description**: TASK-007 не может стартовать, пока TASK-004/005/006 не merged. README пишется против реально существующего публичного API; CHANGELOG перечисляет реально доставленные фичи; godoc-аудит запускается на реально существующих файлах. Без gate — все артефакты могут разойтись с кодом.

**Details**:

```bash
# 1. File existence (TASK-004 / TASK-005 / TASK-006 deliverables):
for f in \
    lib/auth.go \
    lib/cookies.go \
    lib/helpers.go \
    lib/internal/refresh/group.go \
    lib/examples/basic/main.go \
    lib/examples/basic/README.md \
    lib/examples/http_middleware/main.go \
    lib/examples/http_middleware/README.md \
    lib/examples/grpc_interceptor/main.go \
    lib/examples/grpc_interceptor/README.md; do
    test -f "$f" || { echo "STOP: missing $f — TASK-004/005/006 not merged"; exit 1; }
done

# 2. ADR §API key signatures (TASK-004):
grep -q "func (c \*Client) Login("           lib/auth.go || { echo "API: Login"; exit 1; }
grep -q "func (c \*Client) Refresh("         lib/auth.go || { echo "API: Refresh"; exit 1; }
grep -q "func (c \*Client) Logout("          lib/auth.go || { echo "API: Logout"; exit 1; }
grep -q "func (c \*Client) LogoutAll("       lib/auth.go || { echo "API: LogoutAll"; exit 1; }
grep -q "func (c \*Client) ChangePassword("  lib/auth.go || { echo "API: ChangePassword"; exit 1; }
grep -q "func (c \*Client) GetCurrentUser("  lib/auth.go || { echo "API: GetCurrentUser"; exit 1; }
grep -q "^type LoginInput struct"            lib/auth.go || { echo "API: LoginInput"; exit 1; }
grep -q "^type LoginOutput struct"           lib/auth.go || { echo "API: LoginOutput"; exit 1; }
grep -q "^type RefreshOutput struct"         lib/auth.go || { echo "API: RefreshOutput"; exit 1; }

# 3. ADR §API key signatures (TASK-005):
grep -q "^type CookieOption"                 lib/cookies.go || { echo "API: CookieOption"; exit 1; }
grep -q "^func WithCookieDomain("            lib/cookies.go || { echo "API: WithCookieDomain"; exit 1; }
grep -q "^func WithCookiePath("              lib/cookies.go || { echo "API: WithCookiePath"; exit 1; }
grep -q "^func WithCookieMaxAge("            lib/cookies.go || { echo "API: WithCookieMaxAge"; exit 1; }
grep -q "^func WithCookieSameSite("          lib/cookies.go || { echo "API: WithCookieSameSite"; exit 1; }
grep -q "^func SetRefreshCookie("            lib/cookies.go || { echo "API: SetRefreshCookie"; exit 1; }
grep -q "^func ClearRefreshCookie("          lib/cookies.go || { echo "API: ClearRefreshCookie"; exit 1; }
grep -q "^func IPFromRequest("               lib/helpers.go || { echo "API: IPFromRequest"; exit 1; }
grep -q "^func UserAgentFromRequest("        lib/helpers.go || { echo "API: UserAgentFromRequest"; exit 1; }

# 4. Examples compile (TASK-006):
make build-examples || { echo "STOP: examples not compiling — fix TASK-006 first"; exit 1; }

# 5. All tests still green:
make audit || { echo "STOP: audit broken"; exit 1; }
```

**STOP-criterion**: если ЛЮБОЙ из шагов выходит с non-zero — TASK-007 не стартует. Эскалировать назад в TASK-004/005/006 reviewer'у. README и тег публикуются **только** после полной готовности.

---

### Task 1: CREATE `lib/doc.go`

**Action**: CREATE
**File**: [lib/doc.go](../../lib/doc.go)

**Description**: Создать выделенный файл с package-level godoc. Содержит one-paragraph description + 5-строчный quickstart code-block + ссылки на server и proto репо. Не содержит ничего, кроме package-doc и `package authclient`.

**Details**:

- Файл — пакет `authclient`. Без импортов, без объявлений (компилятор примет такой файл как валидный пакет-член).
- Package-godoc должен:
  - Начинаться с `// Package authclient ...` (ES-0081).
  - Содержать линки в формате `// See https://github.com/webdelo/auth` (godoc авто-рендерит URL).
  - Содержать quickstart code-block (с отступом 1 tab в комментарии — godoc-конвенция):
    ```
    //	client, err := authclient.New(authclient.Config{Endpoint: "auth:50051"})
    //	if err != nil { return err }
    //	defer client.Close()
    //	mux.Handle("/api", client.HTTPMiddleware(handler))
    ```
- Длина: 25-35 строк (включая пустые комментарий-строки между параграфами).

**Code Pattern**:

```go
// Package authclient is the official Go SDK for the webdelo/auth service.
//
// It is a thin wrapper over the gRPC contract from webdelo/auth-proto with
// non-trivial concerns baked in: a TTL cache for ValidateToken, an HTTP
// middleware, a gRPC unary interceptor, refresh-race serialization via
// singleflight, typed errors, and fail-closed-by-default semantics. The SDK
// has no DI, logger, or metrics dependencies — consumers plug in their own
// via small adapter interfaces (Logger, Cache).
//
// # Quickstart
//
// Create a Client, attach the HTTP middleware to a protected handler, and
// read the authenticated user out of the request context:
//
//	client, err := authclient.New(authclient.Config{Endpoint: "auth:50051"})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//	mux.Handle("/api/secure", client.HTTPMiddleware(handler))
//
// Inside the handler:
//
//	user, _ := authclient.UserFromContext(r.Context())
//	// user.UUID, user.Role are populated
//
// # Related
//
//   - Server: https://github.com/webdelo/auth
//   - gRPC contract: https://github.com/webdelo/auth-proto
//   - SDK versioning policy: see CHANGELOG.md and README.md.
package authclient
```

**Verification after creation**:

```bash
go doc github.com/webdelo/auth-client-go/lib | head -30
# Должен показать package-doc с quickstart первым блоком, затем — список exported.
```

---

### Task 1b: REMOVE duplicate package-doc from `lib/errors.go`

**Action**: MODIFY
**File**: [lib/errors.go](../../lib/errors.go)

**Description**: Текущие строки `lib/errors.go:1-8` содержат package-godoc как сайд-эффект. После создания `lib/doc.go` (Task 1) этот блок становится duplicate — `go doc` сольёт оба, рендеринг будет странным. Убрать.

**Details**:

- Сейчас:
  ```go
  // Package authclient is the official Go SDK for the webdelo/auth service.
  //
  // The SDK is a thin wrapper over the gRPC contract from
  // github.com/webdelo/auth-proto with non-trivial concerns baked in: a
  // ValidateToken TTL cache, an HTTP middleware, a gRPC unary interceptor,
  // refresh-race serialization, and typed errors. Create a Client with New,
  // dispose with Close.
  package authclient
  ```
- Должно стать (первая строка файла):
  ```go
  package authclient
  ```
- Никаких других изменений в `errors.go`. Импорты и тело файла остаются нетронутыми.

**Verification**:

```bash
head -3 lib/errors.go
# →  package authclient
# →  (пустая строка)
# →  import (
```

И package-doc уцелел в `lib/doc.go`:

```bash
go doc github.com/webdelo/auth-client-go/lib | head -5
# → package authclient // import "github.com/webdelo/auth-client-go/lib"
# →
# → Package authclient is the official Go SDK for the webdelo/auth service.
```

---

### Task 2: Godoc audit on TASK-004/005/006 deliverables

**Action**: VERIFY (and MODIFY if gaps found)
**File**: [lib/auth.go](../../lib/auth.go), [lib/cookies.go](../../lib/cookies.go), [lib/helpers.go](../../lib/helpers.go), [lib/examples/*/main.go](../../lib/examples/)

**Description**: TASK-002/003 код уже имеет полный godoc (35+ exported проверены в research §2). TASK-004/005/006 deliverables написаны параллельно — нужен явный godoc-аудит. revive не ловит — проверка ручная. Где gap — добавить.

**Details — checklist exported identifiers (по ADR §API)**:

#### `lib/auth.go` (TASK-004)

| Identifier | Required godoc | Security warning? |
|------------|----------------|-------------------|
| `type LoginInput struct` | YES — "LoginInput is the request payload for Login..." | NO |
| `LoginInput.Email` (field) | optional — pure data | NO |
| `LoginInput.Password` (field) | optional | NO |
| `LoginInput.ClientIP` (field) | optional, но **must explain "use IPFromRequest(r) instead of BFF own IP"** | NO |
| `LoginInput.UserAgent` (field) | optional, но **must explain "use UserAgentFromRequest(r) or r.UserAgent()"** | NO |
| `type LoginOutput struct` | YES | NO |
| `LoginOutput.AccessToken/RefreshToken/AccessExpiresIn/RefreshExpiresIn/User` | optional, struct-level OK | NO |
| `type RefreshOutput struct` | YES | NO |
| `func (c *Client) Login` | YES — signature, error semantics, что Email требует прокинуть `ClientIP`+`UserAgent` | NO |
| `func (c *Client) Refresh` | YES — **must mention singleflight intra-process only** | NO |
| `func (c *Client) Logout` | YES — accepts refresh token, не access | NO |
| `func (c *Client) LogoutAll` | YES — accepts access token, инвалидирует все sessions | NO |
| `func (c *Client) ChangePassword` | YES — accepts access + current + new; **must mention что после успеха все refresh tokens инвалидируются (token_version bump)** | NO |
| `func (c *Client) GetCurrentUser` | YES — accepts access, returns User с populated Email | NO |

#### `lib/cookies.go` (TASK-005)

| Identifier | Required godoc | Security warning? |
|------------|----------------|-------------------|
| `type CookieOption func(*http.Cookie)` | YES — "CookieOption customizes the refresh cookie..." | NO |
| `func WithCookieDomain` | YES — note: default empty (no Domain attribute) | NO |
| `func WithCookiePath` | YES — note: default `/auth/refresh` | NO |
| `func WithCookieMaxAge` | YES — note: default 14 дней; 0 → session cookie; negative → delete | NO |
| `func WithCookieSameSite` | YES + **SECURITY WARNING: `SameSite=Lax/None` weakens CSRF protection. Default is `Strict`. Use `Lax` only with explicit threat model review.** (TASK-007 акцент prescribes CAPSLOCK или `// WARNING:`) | **YES** |
| `func SetRefreshCookie` | YES — defaults: HttpOnly+Secure+SameSite=Strict+Path=/auth/refresh+MaxAge=14d; override через opts | NO |
| `func ClearRefreshCookie` | YES — sends Set-Cookie с Max-Age=-1; **must use same Path/Domain как SetRefreshCookie** | NO |

#### `lib/helpers.go` (TASK-005)

| Identifier | Required godoc | Security warning? |
|------------|----------------|-------------------|
| `func IPFromRequest` | YES — algorithm: `X-Real-IP` → `X-Forwarded-For[0]` → `RemoteAddr`. **NOTE: no trusted-proxies validation in v0.1.0 — spoofing risk if behind untrusted reverse proxy.** | warning advisable (`// NOTE:`) |
| `func UserAgentFromRequest` | YES — wrapper над `r.UserAgent()`, для симметрии с IPFromRequest | NO |

#### `lib/examples/*/main.go` (TASK-006)

- Каждый main.go должен начинаться с `// Package main demonstrates ...` (package-doc для main-пакета — revive default требует).
- Внутренние funcs (`loginHandler`, `usage`, `slogAdapter` и т.д.) — приватные, godoc optional.

**Verification commands**:

```bash
# 1. Авто-чек что все exported имеют доки. Использует трюк "before each ^func/^type/^var/^const exported - предыдущая строка должна быть //":
for f in lib/auth.go lib/cookies.go lib/helpers.go; do
  awk '
    /^(func|type|var|const) [A-Z]/ || /^func \([a-z*A-Z ]+\) [A-Z]/ {
      if (prev !~ /^\/\//) {
        print FILENAME ":" NR ": MISSING GODOC: " $0
      }
    }
    { prev = $0 }
  ' "$f"
done
# Желаемый вывод: пусто.

# 2. Sanity: package main godoc в examples:
for ex in lib/examples/basic lib/examples/http_middleware lib/examples/grpc_interceptor; do
  head -3 "$ex/main.go" | grep -q "^// Package main" || echo "MISSING package doc in $ex/main.go"
done

# 3. SECURITY WARNING присутствует:
grep -q "SECURITY WARNING\|WARNING:" lib/cookies.go || echo "Missing WithCookieSameSite warning"
grep -q "NOTE:\|spoofing\|trusted prox" lib/helpers.go || echo "Missing IPFromRequest spoofing note"
```

Если auto-check находит missing — добавить godoc. Если найдено и добавлено — отдельный commit `REQ-001-TASK-007: add missing godoc on TASK-004/005 exported`.

---

### Task 3: REWRITE `README.md`

**Action**: REWRITE (полная переработка)
**File**: [README.md](../../README.md)

**Description**: Текущий README — developer/contributor-focused (как собрать/тестировать). Заменяется новой версией, в которой первые 60% — consumer-perspective (Quickstart, Config, FailMode, Errors, Cookies, TLS, Troubleshooting), оставшиеся 40% — development/contributing/CI (сохраняем существующие секции `Local integration testing`, `Commands` под `## Development`).

**Структура (14 секций согласно TASK-007 WHAT §3 + Development:**

```markdown
# auth-client-go
[badges]
Short tagline (1-2 sentences) с ссылками на webdelo/auth, webdelo/auth-proto.

## Installation
- `go get github.com/webdelo/auth-client-go@v0.1.0`
- GOPRIVATE note
- Tip про `@latest` для future releases.

## Quickstart
- ≤ 15 строк (TASK акцент: "≤ 15 строк: New + HTTPMiddleware + handler читает UserFromContext")
- Один code-блок Go.
- Один curl-блок, чтобы показать как сервер отвечает.

## Configuration
- Table: все поля Config (Endpoint, ValidateCacheTTL, FailMode, ValidateTimeout, LoginTimeout, RefreshTimeout, DialOptions, Logger, Cache) — default + описание.
- Table: все Option-функции (WithFailMode, WithLogger, WithCache, WithOpenMethods).

## Fail Mode
> ⚠️ **SECURITY WARNING — FAILOPEN MODE**
>
> WHEN AUTH IS UNAVAILABLE AND FAILOPEN IS SET, EVERY PROTECTED ENDPOINT
> WILL EXECUTE WITHOUT AUTHENTICATION. THIS IS AN AUTH BYPASS BY DESIGN
> AND VIOLATES OWASP A07. USE ONLY ON HEALTH/PROBE ENDPOINTS, NEVER ON
> ENDPOINTS HANDLING USER DATA.

- Default: FailClosed (503 on AUTH down).
- FailOpen: pass-through с User{} в context.
- Configure: `Config.FailMode` или `WithFailMode(...)`.

## Typed Errors
- Table:
  | SDK sentinel | gRPC code | HTTP status from middleware | What client should do |
  | ErrInvalidToken | Unauthenticated | 401 | re-authenticate |
  | ErrExpired | (via is_valid=false / reason) | 401 | call Refresh |
  | ErrBlocked | PermissionDenied | 401 | display "account blocked" |
  | ErrRateLimited | ResourceExhausted | 429 + Retry-After | wait RetryAfter(err) |
  | ErrUnavailable | Unavailable / DeadlineExceeded | 503 (FailClosed) | retry with backoff |
- Code example:
  ```go
  if errors.Is(err, authclient.ErrRateLimited) {
      wait := authclient.RetryAfter(err)
      ...
  }
  ```

## Adapter Interfaces
### Logger
- Описание интерфейса + ссылка на godoc.
- Slog wrapper example (50 строк max — для тех, кто хочет slog):
  ```go
  type slogAdapter struct{ l *slog.Logger }
  func (a slogAdapter) Info(ctx, msg, fields ...any) { a.l.InfoContext(ctx, msg, fields...) }
  ...
  ```
### Cache
- Описание интерфейса.
- Default: built-in `MemoryCache` (in-process TTL).
- Note: для cross-replica deployments — consumer пишет свой Cache (Redis и т.п.).

## Cookie Helpers
- Defaults table:
  | Property | Default value | Override via |
  | HttpOnly | true | (не настраивается) |
  | Secure | true | (не настраивается) |
  | SameSite | http.SameSiteStrictMode | WithCookieSameSite |
  | Path | /auth/refresh | WithCookiePath |
  | MaxAge | 14 * 24h | WithCookieMaxAge |
  | Domain | (empty — host-only) | WithCookieDomain |
- Multi-domain override pattern:
  ```go
  authclient.SetRefreshCookie(w, token,
      authclient.WithCookieDomain(".example.com"),
      authclient.WithCookiePath("/"),
  )
  ```

## HTTP Request Helpers
- `IPFromRequest(r)`: extract algorithm (X-Real-IP → X-Forwarded-For[0] → RemoteAddr).
- `UserAgentFromRequest(r)`: r.UserAgent() wrapper.
- **Why fingerprint binding via SDK and not BFF own IP**: краткое объяснение (AUTH считает `SHA-256(UA + first-3-octets(IP))`; передача BFF-IP сломала бы привязку к юзеру).
- **Spoofing caveat**: trusted-proxies validation отсутствует в v0.1.0 → behind untrusted reverse proxy ⇒ ip-spoofing risk via X-Forwarded-For. Mitigation: terminate untrusted XFF на ingress (nginx) или drop X-Forwarded-For в gateway.

## Examples
- Links:
  - [lib/examples/basic/](lib/examples/basic/) — Login + Validate + Logout CLI demo.
  - [lib/examples/http_middleware/](lib/examples/http_middleware/) — Protected HTTP server (slog adapter).
  - [lib/examples/grpc_interceptor/](lib/examples/grpc_interceptor/) — Protected gRPC server.

## Local integration testing
- (сохранить текущую секцию, обновить hint: добавить `make env-up` пример с конкретными env vars).

## TLS Setup
- Default: insecure (dev).
- Production:
  ```go
  client, err := authclient.New(authclient.Config{
      Endpoint: "auth.example.com:443",
      DialOptions: []grpc.DialOption{
          grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
              ServerName: "auth.example.com",
              // MinVersion: tls.VersionTLS12, etc.
          })),
      },
  })
  ```
- Note: consumer-supplied transport credentials override the SDK default.

## Troubleshooting
### Debugging 401 responses
- Включить Logger (slogAdapter пример).
- Проверить `WWW-Authenticate` header в ответе: `Bearer realm="api"` (missing/malformed) vs `Bearer realm="api", error="invalid_token"` (invalid).
- Cross-check: вызвать `client.ValidateAccessToken(ctx, token)` напрямую — получить sentinel error.

### Fingerprint caveats (NAT, CGNAT, /24-binding)
- Algorithm: AUTH fingerprint = `SHA-256(UA + first-3-octets(IP))`.
- Refresh выдан для `203.0.113.55` → работает на `203.0.113.99` (одна /24), НЕ работает на `203.0.114.55`.
- За корпоративным NAT — обычно одна /24 → ок.
- За CGNAT — пользователь может «мигрировать» между /24 → периодические refresh failure.
- Mitigation: consumer может усилить fingerprint в своём прокси (например, привязка к full IPv6 хосту вместо /24).

### Keepalive auto-bump
- `keepalive.ClientParameters{Time}` < 10s ⇒ grpc-go silently bumps to 10s.
- SDK default: 30s (безопасно).
- Если consumer override через DialOptions — НЕ ставить Time < 10s.

### Distributed singleflight not supported
- Singleflight protects refresh race **только** в пределах одного процесса.
- Two BFF replicas → AUTH может детектить refresh-reuse → family detection → user logout.
- Mitigation для multi-replica: stickiness в LB по `refresh_token` cookie, или (future v0.2.0) distributed singleflight через Redis.

## Known Limitations
- No distributed singleflight (cross-replica BFF).
- No trusted-proxies validation in `IPFromRequest` — spoofing risk behind untrusted reverse proxy.
- No `Metrics` interface yet (planned for v0.2.0).
- `User.Email` is empty after `ValidateAccessToken` (proto v0.1.0 limitation — call `GetCurrentUser` for full profile).
- `ValidateAccessToken` cache lag: up to `ValidateCacheTTL` (default 30s) after AUTH bumps `token_version` (LogoutAll/BlockUser/ChangePassword).

## Versioning Policy
- Semver: `webdelo/auth-client-go` releases follow Semantic Versioning 2.0.
- Pre-v1.0.0: breaking changes allowed but flagged in CHANGELOG.
- Post-v1.0.0: breaking changes only on major version bumps (`v2.0.0`).
- Frozen after v1.0.0: signatures of methods on `*Client`, names/values of Role/FailMode constants, sentinel error names, struct field names/types, Logger/Cache method signatures, cookie defaults.
- Additive after v1.0.0: new methods, new role constants, new sentinel errors, new struct fields (zero-value-compatible), new `WithXxx` options, future `Metrics` interface.

## Development
(сохранить существующее: Prerequisites, Commands table.)

## License
Internal — webdelo organization.
```

**Линки в README используют относительные пути** (`[lib/examples/basic/](lib/examples/basic/)`) — на pkg.go.dev они автоматически конвертируются в ссылки на исходники в GitHub.

**Длина target**: 350-500 строк. Если выходит за 600 — пересмотреть (Quickstart должен быть короче, Troubleshooting — таблицы вместо параграфов).

---

### Task 4: REWRITE `CHANGELOG.md`

**Action**: REWRITE
**File**: [CHANGELOG.md](../../CHANGELOG.md)

**Description**: Заменить stale `[Unreleased]` секцию полноценной `## [v0.1.0] — 2026-05-18` записью с Added/Known limitations/Acknowledgements. Сохранить header и footer-link.

**Details — структура**:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

(пусто — будущие изменения)

## [v0.1.0] — 2026-05-18

Initial release of the Go SDK for [webdelo/auth](https://github.com/webdelo/auth).

### Added

- **Client lifecycle**:
  - `authclient.New(Config, ...Option) (*Client, error)` — constructor with functional options.
  - `(*Client).Close() error` — graceful shutdown (gRPC connection + background goroutines).
  - `Config` with fields: Endpoint, ValidateCacheTTL, FailMode, ValidateTimeout, LoginTimeout, RefreshTimeout, DialOptions, Logger, Cache.
  - Options: `WithFailMode`, `WithLogger`, `WithCache`, `WithOpenMethods`.

- **Token validation**:
  - `(*Client).ValidateAccessToken(ctx, accessToken) (User, error)` — gRPC `ValidateToken` with positive-result TTL cache (default 30s; key = sha256(token)).
  - Built-in in-memory cache (`lib/internal/cache.MemoryCache`) with background sweep; swappable via `Cache` interface.

- **Auth flow**:
  - `(*Client).Login(ctx, LoginInput) (LoginOutput, error)`.
  - `(*Client).Refresh(ctx, refreshToken, clientIP, userAgent) (RefreshOutput, error)` — singleflight-protected per `sha256(refreshToken)` (intra-process race serialization).
  - `(*Client).Logout(ctx, refreshToken) error`.
  - `(*Client).LogoutAll(ctx, accessToken) error`.
  - `(*Client).ChangePassword(ctx, accessToken, current, new) error`.
  - `(*Client).GetCurrentUser(ctx, accessToken) (User, error)`.

- **Middleware**:
  - `(*Client).HTTPMiddleware(next http.Handler) http.Handler` — Bearer-token check, 401 / 503 / 429 semantics, FailMode-aware.
  - `(*Client).UnaryInterceptor() grpc.UnaryServerInterceptor` — same semantics for gRPC; `WithOpenMethods` whitelist; default-open: `/grpc.health.v1.Health/Check`, `/grpc.health.v1.Health/Watch`.

- **Context conventions**:
  - `User{UUID, Role, Email}` (Role: `RoleUnknown`, `RoleUser`, `RoleAdmin`).
  - `WithUser(ctx, user) context.Context`, `UserFromContext(ctx) (User, bool)` — unexported context key per [EC-0056](.claude/guides/golang/effective_concurrency.md).

- **Cookie helpers**:
  - `SetRefreshCookie(w, token, ...CookieOption)` — defaults: HttpOnly+Secure+SameSite=Strict+Path=/auth/refresh+MaxAge=14d.
  - `ClearRefreshCookie(w, ...CookieOption)`.
  - Options: `WithCookieDomain`, `WithCookiePath`, `WithCookieMaxAge`, `WithCookieSameSite` (`SameSite=Lax/None` carries explicit SECURITY WARNING in godoc).

- **HTTP request helpers**:
  - `IPFromRequest(r)` — `X-Real-IP` → `X-Forwarded-For[0]` → `RemoteAddr` (no trusted-proxies validation in v0.1.0).
  - `UserAgentFromRequest(r)`.

- **Typed errors**:
  - Sentinels: `ErrInvalidToken`, `ErrExpired`, `ErrBlocked`, `ErrRateLimited`, `ErrUnavailable`.
  - `AuthError{Err, RetryAfter}` wrapper.
  - `RetryAfter(err) time.Duration` helper for ErrRateLimited.

- **Adapter interfaces**:
  - `Logger` (Info/Warn/Error) with slog-style variadic fields; `NoopLogger{}` default.
  - `Cache` (Get/Set/Delete) — built-in TTL implementation; consumer can swap.

- **Examples** (`lib/examples/`): `basic` (CLI Login→Validate→Logout), `http_middleware` (slog adapter, refresh cookie, /api/profile), `grpc_interceptor` (Health/List под auth).

- **Integration testing**: `make env-up`/`env-down`/`test-integration` with `webdelo/auth` server in Docker Compose.

### Known limitations

- **Distributed singleflight not supported** — refresh-race protection works only within a single SDK process. Multi-replica BFF deployments may trigger AUTH family detection.
- **No trusted-proxies validation** in `IPFromRequest` — caller must terminate untrusted `X-Forwarded-For` headers at the ingress.
- **No `Metrics` interface** — deferred to v0.2.0.
- **`User.Email` empty after `ValidateAccessToken`** — `ValidateTokenResponse` (auth-proto v0.1.0) does not carry it. Use `GetCurrentUser` for the full profile.
- **Cache invalidation lag** — after AUTH bumps `token_version` (LogoutAll/BlockUser/ChangePassword), the old access token remains valid in the SDK cache for up to `ValidateCacheTTL` (default 30s). Set `ValidateCacheTTL: 0` to disable cache entirely.
- **`make release` is interactive** — no `--version=X.Y.Z` flag yet; the release script reads from `/dev/tty`.

### Acknowledgements

- Based on the gRPC contract published in [`github.com/webdelo/auth-proto`](https://github.com/webdelo/auth-proto) v0.1.0.
- Follows [OWASP Session Management Cheat Sheet](https://owasp.org/www-project-cheat-sheets/cheatsheets/Session_Management_Cheat_Sheet.html) for cookie defaults.
- Follows [OWASP A07:2025 — Authentication Failures](https://owasp.org/Top10/) for fail-closed semantics.

[Unreleased]: https://github.com/webdelo/auth-client-go/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/webdelo/auth-client-go/releases/tag/v0.1.0
```

**Verification**:

```bash
grep -E "^## \[v0\.1\.0\]" CHANGELOG.md
# → ## [v0.1.0] — 2026-05-18

grep -E "^\[v0\.1\.0\]: https" CHANGELOG.md
# → [v0.1.0]: https://github.com/webdelo/auth-client-go/releases/tag/v0.1.0
```

---

### Task 5: Self-review README via local replace-directive

**Action**: VERIFY (manual)
**File**: — (имитация BFF-разработчика на dev-машине)

**Description**: До публикации тега — имитировать BFF-разработчика, который собирает middleware читая ТОЛЬКО README. Это «черновая» smoke-валидация перед формальным внешним gate (Task 8). Цель — поймать опечатки в Quickstart, missing imports, неверные сигнатуры до того, как реальный BFF-валидатор откроет issue.

**Details**:

```bash
# 1. В отдельной директории создать минимальный consumer:
mkdir /tmp/authclient-smoke && cd /tmp/authclient-smoke
go mod init smoke

# 2. Использовать replace-директиву (тег ещё не опубликован):
echo 'replace github.com/webdelo/auth-client-go => /Users/a.popov/Development/TRADING/bots/auth-client-go' >> go.mod

# 3. Создать main.go скопировав ТОЧНО Quickstart из README:
#    (copy-paste из README, не редактируя)

# 4. Собрать:
go mod tidy
go build ./...

# 5. Если ошибки — фикс README (импорты, сигнатуры, опечатки) и повторить.
```

**Acceptance**: `go build` проходит без ошибок, копируя Quickstart 1:1 из README.

**Cleanup**: `rm -rf /tmp/authclient-smoke` после успешной сборки.

---

### Task 6: Pre-release audit

**Action**: VERIFY
**File**: — (запуск quality-gates)

**Description**: Перед `make release` запустить полный audit-pipeline. Любая red — STOP, fix, re-run.

**Details**:

```bash
# 1. Compile / vet / lint / unit-tests / mod verify / tidy-diff:
make audit
# → должно пройти без warnings.

# 2. Examples собираются:
make build-examples
# → bin/example-basic, bin/example-http_middleware, bin/example-grpc_interceptor.

# 3. Integration-тесты против реального AUTH:
make test-integration
# → требует Docker daemon + ../auth/Dockerfile.
# → если Docker недоступен — задокументировать в результатах TASK как **SKIP** + reason,
#   эскалировать на reviewer'а (DOD-нарушение, не пропускать молча).

# 4. Coverage (sanity):
make coverage
# → ≥ 75% (REQ-001 DOD требование).

# 5. Дублирующая проверка публичной поверхности:
go doc -all github.com/webdelo/auth-client-go/lib | grep -cE "^func|^type|^var|^const"
# Сравнить с ADR §API (canonical count: ≥ 35).

# 6. go mod why на ключевую зависимость:
go mod why github.com/webdelo/auth-proto/go
# → должен показать `# github.com/webdelo/auth-proto/go` (direct, не indirect).

# 7. Branch ready for tag:
git status --porcelain   # → пусто (нет uncommitted).
git fetch origin
git log origin/main..HEAD --oneline   # → пусто (всё смержено).
```

Если ВСЁ green — переходим к Task 7. Иначе — STOP, fix, re-run.

---

### Task 7: Create `v0.1.0` tag via `make release`

**Action**: PUBLISH
**File**: — (тег + GitHub release)

**Description**: Зафиксировать `v0.1.0` через `make release`. Тег ложится на `origin/main` HEAD (`dev/tools/release.sh:290-291`), не на текущую ветку, поэтому README/CHANGELOG/doc.go должны быть уже merged в `main`.

**Details**:

```bash
# 1. Убедиться что на main HEAD есть нужные коммиты:
git fetch origin
git log origin/main --oneline -5
# Должно содержать REQ-001-TASK-007 коммит с README/CHANGELOG/doc.go.

# 2. Preview (dry-run):
make release-dry-run
# Скрипт интерактивный — вводим "0.1.0" в prompt. Покажет:
#   - tag будет создан на origin/main HEAD (печатает commit hash)
#   - release notes (auto-generated git-log format)
# НЕ запускает фактический tag/push.

# 3. Реальный release:
make release
# Вводим "0.1.0". Confirm "y". Скрипт:
#   - git tag -a v0.1.0 -m "Release v0.1.0" <origin/main hash>
#   - git push origin v0.1.0
#   - gh release create v0.1.0 --title v0.1.0 --notes-file -

# 4. ВЕРИФИКАЦИЯ:
git tag -l v0.1.0                        # → v0.1.0
git ls-remote --tags origin v0.1.0       # → <hash>  refs/tags/v0.1.0
gh release view v0.1.0                   # → URL + assets.
GOPROXY=direct go list -m github.com/webdelo/auth-client-go@v0.1.0
# → github.com/webdelo/auth-client-go v0.1.0
```

**ВАЖНО**: `make release-with-notes` НЕ использовать — он зовёт `claude --print` для AI-генерации, что нарушает project-level правило «НЕ добавляй AI-метки в коммиты и PR» (`/Users/a.popov/Development/TRADING/CLAUDE.md`).

**Rollback (если очень надо)**:

```bash
# Удалить локальный тег:
git tag -d v0.1.0
# Удалить remote тег:
git push --delete origin v0.1.0
# Удалить GH release:
gh release delete v0.1.0 --yes
```

Rollback **должен быть редким** — `v0.1.0` уже потенциально закэширован в `proxy.golang.org` (если кто-то успел `go get`). Лучше выпустить `v0.1.1` patch с fix'ами.

---

### Task 8: Public-surface census vs ADR §API

**Action**: VERIFY
**File**: — (sanity check после релиза)

**Description**: TASK-007 DOD §7: «Публичная поверхность SDK (`go doc lib`) соответствует ADR §API». Финальный check.

**Details**:

```bash
# Выгрузить exported:
go doc -all github.com/webdelo/auth-client-go/lib | grep -E "^(func|type|var|const)" | sort > /tmp/actual.txt

# Cross-check counts по ADR (REQ-001-ADR.md:542-699):
# 5 sentinel errors (ErrInvalidToken, ErrExpired, ErrBlocked, ErrRateLimited, ErrUnavailable)
grep -c "^var Err" /tmp/actual.txt
# → 5

# 4 With*-options (WithFailMode, WithLogger, WithCache, WithOpenMethods)
grep -cE "^func With(FailMode|Logger|Cache|OpenMethods)\(" /tmp/actual.txt
# → 4

# 4 With*Cookie* options (Domain, Path, MaxAge, SameSite)
grep -cE "^func WithCookie(Domain|Path|MaxAge|SameSite)\(" /tmp/actual.txt
# → 4

# 9 methods on *Client:
grep -cE "^func \(c \*Client\)" /tmp/actual.txt
# → 9 (Close + ValidateAccessToken + Login + Refresh + Logout + LogoutAll + ChangePassword + GetCurrentUser + HTTPMiddleware + UnaryInterceptor)
# Wait — это 10. Re-count: Close, Validate, Login, Refresh, Logout, LogoutAll, ChangePass, GetCurrent, HTTPMW, UnaryInt = 10.
# TASK-007 DOD §7 говорит "9 методов на Client". Расхождение с ADR — на ADR-стороне 10 методов, на TASK-007 написано "9".
# Resolution: подсчитать по ADR canonical list (542-699): Close, ValidateAccessToken, Login, Refresh, Logout, LogoutAll, ChangePassword, GetCurrentUser, HTTPMiddleware, UnaryInterceptor → 10.
# TASK-007:73 пишет "9" — это TASK-spec typo. Фактический канон — 10. Использовать 10 и зафиксировать в результатах TASK как noted typo.

# 4 helpers (SetRefreshCookie, ClearRefreshCookie, IPFromRequest, UserAgentFromRequest + WithUser/UserFromContext):
grep -E "^func (SetRefreshCookie|ClearRefreshCookie|IPFromRequest|UserAgentFromRequest|WithUser|UserFromContext|RetryAfter)\(" /tmp/actual.txt
# → 7 package-level funcs (вне Client). TASK §7 "4 helpers" — это, видимо, 4 request/cookie helpers (Set/Clear/IP/UA). Уточнить в результатах TASK.

# Compile-time check: export_test.go НЕ загрязняет go doc:
grep -E "FakeAuth|ClientConfig|NewTestClient|HashToken|DefaultOpenMethods|BearerToken" /tmp/actual.txt
# → пусто (go doc исключает _test.go файлы; verified в research §3 Q1).

# Compile-time check: НЕ должно быть лишнего exported из internal/:
grep "internal" /tmp/actual.txt
# → пусто.
```

**Acceptance**:
- 5 sentinels.
- 4 `With*Option` (non-cookie).
- 4 `WithCookie*Option`.
- 10 методов на `*Client` (см. resolution выше — TASK-spec говорит «9», ADR показывает 10; используем 10, фиксируем в task results).
- 4 request/cookie helpers (`SetRefreshCookie`, `ClearRefreshCookie`, `IPFromRequest`, `UserAgentFromRequest`) + 2 context helpers (`WithUser`, `UserFromContext`) + 1 error helper (`RetryAfter`) = 7 package-level funcs.

Если расхождение — STOP, эскалировать (либо ADR обновить, либо код выровнять).

---

### Task 9: BFF-validator smoke test

**Action**: EXTERNAL VALIDATION
**File**: [task_management/results/REQ-001-TASK-007-bff-validation.md](../../task_management/results/REQ-001-TASK-007-bff-validation.md) (CREATE)

**Description**: TASK-007 WHAT §7 + DOD §9 — другой Go-разработчик (не автор REQ-001) должен через `go get github.com/webdelo/auth-client-go@v0.1.0` собрать рабочий middleware за < 30 минут, читая **ТОЛЬКО** опубликованный README. Время засекается; результат сохраняется в `task_management/results/REQ-001-TASK-007-bff-validation.md`.

**Details**:

1. **Choose validator**: разработчик, который НЕ участвовал в TASK-001..006 (другой человек, чтобы не было «знания контекста»). Реально для платформы — BFF-разработчик из команды (например, owner будущего BFF-сервиса). До назначения — обсудить с reviewer'ом.

2. **Validator instructions** (передать копией):
   - Открыть `https://github.com/webdelo/auth-client-go` README.
   - Создать новый Go-проект.
   - Через `go get github.com/webdelo/auth-client-go@v0.1.0` подключить SDK.
   - Собрать минимальный HTTP-сервер с одним protected endpoint (например, `/me`), который возвращает `UserFromContext(r.Context())` как JSON.
   - Стартовать сервер локально, ткнуть curl'ом с реальным access-token (из AUTH dev-instance).
   - Засечь время от первого `go get` до первого успешного curl-ответа.

3. **Report template** (`task_management/results/REQ-001-TASK-007-bff-validation.md`):

   ```markdown
   # REQ-001-TASK-007 — BFF Validation Smoke Test

   ## Metadata
   | Field | Value |
   |-------|-------|
   | Validator | <name> |
   | Date | <YYYY-MM-DD> |
   | SDK version | github.com/webdelo/auth-client-go@v0.1.0 |
   | README revision | <commit hash> |

   ## Time

   - Start: <HH:MM>
   - First successful curl: <HH:MM>
   - Total: <NN minutes>

   ## Pass / Fail
   - [ ] PASS — < 30 minutes, working middleware, no out-of-README questions.
   - [ ] FAIL — > 30 minutes OR required out-of-README hints.

   ## Issues encountered
   (что было непонятно / где запнулся)

   ## README change requests
   (конкретные правки в README — приоритезированы; обрабатываются в TASK-008 / v0.1.1 patch)
   ```

4. **Acceptance**:
   - PASS → close TASK-007 / REQ-001.
   - FAIL → fix README issues → publish v0.1.1 patch (NOT re-tag v0.1.0) → re-run validation.

---

## Integration

### Database
Не применимо — SDK не имеет database layer.

### Config
Не применимо — TASK документации/релиза.

### Dependencies

| Package | Purpose | Action in TASK-007 |
|---------|---------|--------------------|
| `github.com/webdelo/auth-proto/go` | v0.1.0 — gRPC contract | No change (verify direct dep via `go mod why`). |
| `google.golang.org/grpc` | v1.79.3 — gRPC framework | No change. |
| `google.golang.org/protobuf` | v1.36.11 — protobuf runtime | No change. |
| `golang.org/x/sync` | v0.19.0 — singleflight | No change. |
| `github.com/stretchr/testify` | v1.11.1 — test only | No change. |

`go mod tidy` запускается как часть `make audit` (Task 6) — должен быть no-op (никаких новых зависимостей не добавляется TASK-007).

---

## Testing

### Unit Tests

Не применимо — TASK не добавляет код, требующий unit-тестов. Один `lib/doc.go` содержит только `package authclient` directive + doc-комментарий — нет executable code.

Unit-тесты, существующие для TASK-002/003/004/005/006 кода, должны продолжать проходить (`make audit` в Task 6).

### Integration Tests

Не пишутся новые. Существующие (`lib/*_integration_test.go`, `tests/`) должны проходить (`make test-integration` в Task 6).

### Documentation Validation Tests

**Manual verification gates** (вместо unit-тестов):

| Gate | Command | Pass criterion |
|------|---------|----------------|
| Package godoc renders | `go doc github.com/webdelo/auth-client-go/lib \| head -30` | First lines: "Package authclient is the official Go SDK..." + quickstart |
| All exported documented | `awk` script из Task 2 | пустой вывод (нет MISSING GODOC: ...) |
| Public surface matches ADR | Task 8 grep-pipeline | counts совпадают с ADR §API |
| README Quickstart compiles | Task 5 (`/tmp/authclient-smoke`) | `go build ./...` exit 0 |
| README has all 14 sections | `grep -cE "^## " README.md` | ≥ 14 |
| FailOpen warning CAPSLOCK | `grep -E "SECURITY WARNING\|FAILOPEN" README.md` | ≥ 1 match |
| Tag exists and resolves | `GOPROXY=direct go list -m ...@v0.1.0` | exit 0, returns `v0.1.0` |
| BFF validator < 30 min | Task 9 report | PASS |

---

## Validation

### Build & Lint

```bash
# Standard pipeline (Task 6):
make audit              # vet + lint + test + mod verify + tidy-diff
make build-examples     # все 3 примера собираются
make test-integration   # AUTH через docker-compose
make coverage           # ≥ 75%
```

### Manual Checks

1. **Open README.md в браузере (GitHub preview)** — проверить рендеринг таблиц, code-block syntax highlighting, ссылок.
2. **Open https://pkg.go.dev/github.com/webdelo/auth-client-go@v0.1.0 после публикации** — проверить:
   - Package overview показывает doc.go content.
   - Все exported идентификаторы появились (refresh ≤ 30 min после tag push).
   - Linked code-examples (если auto-detected) рендерятся.
3. **`go doc` локально**:
   - `go doc github.com/webdelo/auth-client-go/lib` — overview.
   - `go doc github.com/webdelo/auth-client-go/lib.Client.Login` — Login godoc видит singleflight note.
   - `go doc github.com/webdelo/auth-client-go/lib.WithCookieSameSite` — видит SECURITY WARNING.
   - `go doc github.com/webdelo/auth-client-go/lib.FailOpen` — видит SECURITY WARNING.
4. **CHANGELOG renders correctly** на GitHub release page (`gh release view v0.1.0 --web`).

---

## DOD (Definition of Done)

- [ ] `lib/doc.go` существует, содержит package-level godoc (Task 1).
- [ ] Все exported типы/функции/методы/константы/sentinel-errors имеют godoc-комментарии (Task 2 audit — auto-grep пустой).
- [ ] `README.md` содержит все 14 секций из WHAT §3 TASK-007 (Task 3).
- [ ] README имеет CAPSLOCK security warning про `FailOpen` (Task 3).
- [ ] README имеет таблицу typed errors и пример `errors.Is` (Task 3).
- [ ] `CHANGELOG.md` содержит запись `## [v0.1.0] — 2026-05-18` с `Added`, `Known limitations`, `Acknowledgements` секциями (Task 4).
- [ ] CHANGELOG footer-link `[v0.1.0]: ...` (Task 4).
- [ ] Публичная поверхность (`go doc github.com/webdelo/auth-client-go/lib`) соответствует ADR §API (5 sentinels, 4 `With*Option`, 4 `WithCookie*Option`, 10 `*Client` methods (TASK-007:73 typo says "9" — used 10), 4 request/cookie helpers + 2 context helpers + 1 error helper = 7 package-level funcs) (Task 8).
- [ ] Тег `v0.1.0` опубликован в `webdelo/auth-client-go` (`git ls-remote --tags origin v0.1.0` returns non-empty) (Task 7).
- [ ] `GOPROXY=direct go list -m github.com/webdelo/auth-client-go@v0.1.0` returns `v0.1.0` (Task 7).
- [ ] BFF-validator (другой разработчик) собрал working middleware через `go get @v0.1.0` за < 30 минут, отчёт в `task_management/results/REQ-001-TASK-007-bff-validation.md` (Task 9).
- [ ] `go mod why github.com/webdelo/auth-proto/go` подтверждает прямую зависимость v0.1.0 (Task 6).
- [ ] `make audit` + `make build-examples` + `make test-integration` зелёные (Task 6).
- [ ] Umbrella DOD из [REQ-001 §DOD](../tasks/REQ-001.md) полностью закрыт (все 13 пунктов чек-листа).
