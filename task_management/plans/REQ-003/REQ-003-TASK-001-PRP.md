# REQ-003-TASK-001-PRP: `ABORTED` от `Refresh` — повторяемая ошибка `ErrRefreshRetry`

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v2.0 |
| Plan ID | REQ-003-TASK-001-PRP |
| Parent Request | [REQ-003](../../tasks/REQ-003.md) |
| Parent Task | [REQ-003-TASK-001](../../tasks/REQ-003-TASK-001.md) |
| Created | 2026-11-12 |
| Updated | 2026-11-12 |

---

## Goal

### Summary
`(*Client).Refresh` начинает отдавать gRPC `ABORTED` как новый экспортируемый sentinel `ErrRefreshRetry` внутри `*AuthError` с подсказкой Retry-After (из `google.rpc.RetryInfo`, иначе документированная секунда); все остальные RPC и коды маппятся как раньше. README и CHANGELOG обновляются, после merge выпускается тег `v0.4.0`.

### Task
[REQ-003-TASK-001](../../tasks/REQ-003-TASK-001.md) — задача целиком; здесь она не дублируется.

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../../.claude/guides/rules_index.yaml) | Загружен; срез правил задачи собран `extract_rules.py` (signals: ES) |
| Effective Standards (errors) | [effective_standards.md](../../../.claude/guides/golang/effective_standards.md) | GO-ES-0060, GO-ES-0061, GO-ES-0062, GO-ES-0063 |
| Coding Style | [coding_style.md](../../../.claude/guides/golang/coding_style.md) | GO-CS-0010, GO-CS-0111, GO-CS-0112 |
| Unit Tests | [unit_tests_standards.md](../../../.claude/guides/golang/unit_tests_standards.md) | GO-UT-0001, GO-UT-0002, GO-UT-0010, GO-UT-0020, GO-UT-0021, GO-UT-0024, GO-UT-0030, GO-UT-0040 |
| Testify (lib TFY) | `${CLAUDE_PLUGIN_ROOT}/skill-packs/go/go-testify/references/testify.md` | TFY — require для критичных проверок, assert для дополнительных |
| Security core | `${CLAUDE_PLUGIN_ROOT}/rules/_prebuilt/go/core_rules_cheatsheet.md` | SEC-0001..SEC-0005 — PAT выкладки при проверке тега не выводится |

---

## Analysis Files

- **Code Research**: [code-research_refresh-retry.yaml](../../../.temp/reports/REQ-003-TASK-001/code-research_refresh-retry.yaml)

> PRP — смысловой план, не реализация: что и где меняем, в каком порядке, как поймём, что готово.
> Кода, SQL и конфигов в нём нет; единственный блок кода — команды проверки в `Validation`.

---

## Context

### Gotchas

| Category | Issue | Solution |
|----------|-------|----------|
| Поведение middleware | Общий `mapGRPCError` вызывают и `ValidateAccessToken`, и HTTP-middleware / gRPC-интерсептор (`errors.Is(..., ErrUnavailable)` решает fail-open и 503) | `ABORTED` → `ErrRefreshRetry` только в пути `Refresh`; общий маппер для остальных RPC не меняется |
| Различимость | Если `ErrRefreshRetry` будет совпадать с `ErrUnavailable` через `errors.Is`, bff, проверяющий недоступность первой, снова ответит 503 | `AuthError` не получает метода `Is`; `ErrRefreshRetry` — самостоятельный sentinel; смена поведения отмечается в CHANGELOG |
| Логи | `logUnavailableIfNeeded` пишет Warn только для `ErrUnavailable` — повтор продления больше не попадает в Warn | Так и задумано: повтор — бизнес-исход, не сбой; отражается в godoc `Refresh` |
| Подсказка RateLimited | Разбор `RetryInfo` в общем `retryAfterFromStatus` даёт подсказку и `ErrRateLimited`; существующие тесты без деталей ждут 0 | Без деталей по-прежнему 0 для `ResourceExhausted`; дефолт в одну секунду применяется только к `ABORTED` |
| Некорректный `RetryInfo` | Пустой, отрицательный или невалидный `retry_delay` | Трактуется как отсутствие подсказки; для `ABORTED` берётся дефолт |
| Зависимость | `google.golang.org/genproto/googleapis/rpc` сейчас `// indirect` | После импорта `errdetails` — `make tidy`, require становится прямым; версия не меняется |
| Релиз | `dev/tools/release.sh` интерактивный и тегирует `origin/main` | Тег — только после merge PR, отдельной операционной задачей |
| Singleflight | Ведомые продления получают сырую ошибку лидера | Маппинг выполняется у каждого вызывающего после `sf.Do`, как сейчас — ведомый тоже получает `ErrRefreshRetry` |

---

## Implementation

### Task 1: Sentinel, дефолт и разбор `RetryInfo`

**Action**: MODIFY
**File**: [errors.go](../../../lib/errors.go)

**Description**: Рядом с остальными sentinels (`lib/errors.go:13-31`) появляется экспортируемый `ErrRefreshRetry` с текстом в стиле `authclient: …` и godoc: сессия жива, продление уже выполняется, повторить через `RetryAfter(err)`. Появляется экспортируемая константа `DefaultRefreshRetryAfter` = 1 секунда с godoc о том, когда она применяется. `retryAfterFromStatus` (`:111`) перестаёт быть заглушкой: ищет в деталях статуса `google.rpc.RetryInfo` и возвращает его `retry_delay`, если он валиден и положителен, иначе 0; TODO удаляется. Появляется неэкспортируемый маппер ошибок продления: для `codes.Aborted` (статус ищется тем же `statusFromWrapped`) возвращает `*AuthError` с `ErrRefreshRetry` и подсказкой из `retryAfterFromStatus`, а при нуле — `DefaultRefreshRetryAfter`; любой другой случай делегирует в `mapGRPCError` без изменений. Godoc полей и функции `RetryAfter` (`:41`, `:66`) упоминает обе ошибки с подсказкой: `ErrRateLimited` и `ErrRefreshRetry`.

**Тесты**: В `lib/errors_test.go` — новая table-driven функция для маппера продления: доказывает `ABORTED` → `ErrRefreshRetry` с подсказкой из `RetryInfo` и с дефолтом без него (и при невалидной длительности), что `UNAVAILABLE`/`DEADLINE_EXCEEDED` по-прежнему дают `ErrUnavailable`, а `errors.Is` и `RetryAfter` работают через обёртку `fmt.Errorf` с `%w`. Новая функция для `retryAfterFromStatus` доказывает разбор `RetryInfo` и 0 без деталей. В `TestMapGRPCError` — случай, доказывающий, что общий маппер для `ABORTED` остался `ErrUnavailable`, и случай `ResourceExhausted` с `RetryInfo`.

**Ключевое решение**: `ABORTED` становится `ErrRefreshRetry` только в маппере продления; общий `mapGRPCError` не меняется.

---

### Task 2: `Refresh` использует маппер продления

**Action**: MODIFY
**File**: [auth.go](../../../lib/auth.go)

**Description**: В `(*Client).Refresh` (`lib/auth.go:138`) ветка ошибки после `c.sf.Do` (`:165`) вызывает маппер продления вместо `mapGRPCError`; вызов `logUnavailableIfNeeded` остаётся. Список `Returns:` в godoc `Refresh` (`:131-137`) дополняется: `ErrRefreshRetry` при повторе продления той же cookie в окне повтора auth — сессия жива, вызывающему нужно повторить через `RetryAfter(err)`. Остальные методы `auth.go` и `validate.go` не трогаются.

**Тесты**: В `TestClient_Refresh` (`lib/auth_test.go:147`) — случай на существующем харнессе пакета (`FakeAuthClient` через `newTestMocks` и `NewTestClient`, как соседние случаи; харнесс и `mockery.yaml` не меняются), где ответ продления — `ABORTED`: доказывает, что публичный `Refresh` отдаёт `ErrRefreshRetry` с ненулевой подсказкой и не `ErrUnavailable`; существующий случай `UNAVAILABLE` → `ErrUnavailable` остаётся зелёным.

**Ключевое решение**: Различение повтора и недоступности видно потребителю именно через `Refresh`.

---

### Task 3: README и CHANGELOG

**Action**: MODIFY
**File**: [README.md](../../../README.md)

**Description**: В разделе `## Typed Errors` (`README.md:128-162`) таблица sentinels получает строку `ErrRefreshRetry` (источник — `ABORTED` от `Refresh`, действие — повторить продление через `RetryAfter`, дефолт одна секунда, если auth не прислал `RetryInfo`); существующий пример на `Login` не трогается, а рядом появляется отдельный короткий пример обработки ошибки `Refresh`: проверка `ErrRefreshRetry` стоит до проверки `ErrUnavailable` и берёт задержку из `RetryAfter`; текст о `RetryAfter` упоминает `RetryInfo` для `ErrRateLimited`. В [CHANGELOG.md](../../../CHANGELOG.md) запись `v0.4.0` в формате Keep a Changelog: `Added` — `ErrRefreshRetry`, `DefaultRefreshRetryAfter`, разбор `RetryInfo`; `Changed` — `ABORTED` от `Refresh` больше не `ErrUnavailable` (изменение поведения для потребителей, проверяющих `ErrUnavailable` после `Refresh`, и отсутствие Warn-лога для этого случая); ссылки сравнения версий внизу файла.

**Тесты**: Не применимо — документация; корректность ссылок и примеров проверяется на ревью.

**Ключевое решение**: Минорный релиз `v0.4.0` — аддитивный sentinel по `## Versioning Policy`, смена поведения явно помечена.

---

### Task 4: Качество на голове ветки

**Kind**: operational
**После**: Task 3
**Нужно**: локальный тулчейн — `make help` (проверка доступности make-таргетов)
**Согласие владельца**: не требуется

**Description**: На финальной голове ветки прогоняются `make tidy` (diff `go.mod` — только перенос `genproto/googleapis/rpc` в прямые зависимости), `make audit` и `make test-race`; вывод фиксируется в отчёте `execute-prp`.

**Ключевое решение**: Пункт DOD о `make audit` закрыт, когда все три таргета зелёные, а `git status` чист после `make tidy`.

---

### Task 5: Публикация тега `v0.4.0`

**Kind**: operational
**После**: merge PR ветки `feature/FB-1602` в `main` (CI на PR зелёный)
**Нужно**: GitHub — `gh auth status` и `git ls-remote --tags origin`
**Согласие владельца**: дано в карточке задачи

**Description**: Фаза после merge: тег ставится от `origin/main` (`dev/tools/release.sh`), поэтому этот пункт и CI на PR не могут быть доказаны на голове ветки до `generate-pull-request` — до PR доказываются только код, тесты и `make audit` (Task 4). После merge — `make release-dry-run`, затем `make release` с версией `v0.4.0` (тег и GitHub release); `git ls-remote --tags origin` показывает тег.

**Ключевое решение**: Тег существует в `origin` и указывает на merge-коммит в `main`.

---

### Task 6: Проверка `go get` из tsa с PAT выкладки

**Kind**: operational
**После**: Task 5
**Нужно**: checkout trade-signal-analyzer — `git -C <tsa> rev-parse HEAD`; механизм PAT выкладки stage (секрет CI/выкладки tsa) — проверка доступа без чтения значения
**Согласие владельца**: отложено владельцу

**Description**: В отдельном чистом checkout tsa с изолированными `GOMODCACHE` и `GOPATH` в `.temp/`, `GOPROXY=direct`, `GOPRIVATE=github.com/webdelo` и учётными данными только из механизма PAT выкладки выполняется `go get github.com/webdelo/auth-client-go@v0.4.0`; изменения в checkout tsa не коммитятся. Значение PAT не читается и не выводится (SEC-0001..0005); фиксируется только код возврата и резолвнутая версия.

**Ключевое решение**: Пункт DOD о `go get` из tsa закрыт, когда команда с PAT выкладки и пустым кешем завершилась успешно и `go list -m` в tsa показывает `v0.4.0`.

---

## Validation

```bash
make tidy
git diff --exit-code -- go.sum
make fmt
make vet
make lint
make test
make test-race
make audit
make build-examples
make release-dry-run
```

---

## DOD (Definition of Done)

- [ ] [task] Юнит-тесты: `ABORTED` → `ErrRefreshRetry` с подсказкой Retry-After (с `RetryInfo` и без); `UNAVAILABLE` → `ErrUnavailable`; `errors.Is` работает через обёртки.
- [ ] [task] `make audit` зелёный; CI на PR зелёный.
- [ ] [task] Тег опубликован; `go get github.com/webdelo/auth-client-go@<тег>` работает из tsa с PAT выкладки.
- [ ] [task] Каждый пункт DoD доказан на финальной голове ветки до `generate-pull-request`.
- [ ] [+plan] Общий `mapGRPCError` для `ABORTED` по-прежнему отдаёт `ErrUnavailable` (тест), поведение `ValidateAccessToken`, middleware и интерсептора не меняется.
- [ ] [+plan] `RetryInfo` разбирается для `ErrRateLimited` и `ErrRefreshRetry`; без деталей `ErrRateLimited` по-прежнему 0.
- [ ] [+plan] `make test-race` зелёный; `make tidy` переводит `genproto/googleapis/rpc` в прямые зависимости без смены версии.
- [ ] [+plan] README (`Typed Errors`, отдельный пример для `Refresh`) и CHANGELOG (`v0.4.0`, `Added` + `Changed`) обновлены.
- [ ] [+plan] Фазы доказательства: юнит-тесты и `make audit` — на финальной голове ветки до `generate-pull-request`; CI на PR — после создания PR; тег и `go get` из tsa — после merge (Task 5, Task 6).

---

## Внешнее ревью и оценка PRP

### Внешнее ревью (пре-гейт)

| Параметр | Значение |
|----------|----------|
| Инструмент | `mcp__pal__clink`, CLI `codex`, роль `codereviewer` |
| Статус | `выполнено` |
| Покрытие | `full` |
| Оценка | `9/10` (раунд 2 после применения находок; раунд 1 — 6/10) |
| Находки | Critical × 0, High × 1, Medium × 3, Low × 0 |
| Сырой ответ | [prp-review-pregate_codex.md](../../../.temp/reports/REQ-003-TASK-001/prp-review-pregate_codex.md) — эфемерная ссылка |

### Хронология (коммитируется, отдельно от .temp/)

| Поле | Значение |
|------|----------|
| Вызов clink | `2026-11-12T19:30:00+03:00` |
| `continuation_id` | `a025e11f-8711-47ef-9dc1-f058aa595ecf` |
| sha256 raw-answer | `b3548afac7e6b602b2fcec560c7ad502c095610fa4e15c44146f8d68f51a0d60` |
| sha256 PRP (на момент ревью) | `5a48bc0651c5b734bd84bca7a1e692fed5af982fa8fc45c6157763e2db49e158` |
| sha256 PRP (текущий; снимок, не источник для `--mode audit`) | см. `binding.prp_sha256_current` в findings-файле — PRP включает эту таблицу и не может содержать собственный хеш |

| # | Находка | Что сделано |
|---|---------|-------------|
| H-1 | Тег ставится после merge, а DOD задачи требует доказать всё до generate-pull-request | Применено: DOD задачи дословно, фаза после merge явно в Task 5, фазы доказательства — пункт `[+plan]` |
| M-1 | Тест Refresh на моке mockery несовместим с харнессом `FakeAuthClient` | Применено: тест на существующем харнессе пакета, харнесс не меняется |
| M-2 | Локальный `go list` не доказывает `go get` из tsa с PAT выкладки | Применено: Task 6 — чистый checkout tsa, изолированный кеш, только PAT выкладки; согласие отложено владельцу |
| M-3 | Ветка повтора в примере README на `Login` недостижима | Применено: отдельный пример для `Refresh` |

### Итоговые оценки

**1. Сложность выполнения PRP — 3/10.** Две функции и один вызов в маленьком пакете, плюс релиз и проверка из tsa после merge.

**2. Качество самого PRP по внешнему ревью — 9/10** (порог ≥7). Минус балл — проверка из tsa зависит от согласия владельца и доступа к PAT выкладки.
