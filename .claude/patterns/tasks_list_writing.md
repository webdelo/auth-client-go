# Правила заполнения tasks_list.yaml

> ⚠️ **ВАЖНО: НЕ редактировать tasks_list.yaml вручную!**
>
> Используйте скрипты для управления задачами:
> ```bash
> # Автоматическое обнаружение и добавление задач
> python3 .claude/scripts/req_refresh.py
>
> # Регистрация файлов (result, code_review, debug_report, prp, task)
> python3 .claude/scripts/req_status.py add-file {TASK-ID} {type} {path}
>
> # Обновление lifecycle (статус, категория)
> python3 .claude/scripts/req_status.py refresh-lifecycle {TASK-ID}
>
> # Генерация tasks_list.md
> python3 .claude/scripts/req_status.py generate-md
> ```
>
> Ручное редактирование приводит к рассинхронизации со скриптами!

Данный документ описывает правила и стандарты ведения списка задач проекта в YAML формате.

## Расположение файла

```
.memory_bank/tasks_list.yaml
```

## Структура файла

```yaml
# Tasks List - {Project Name}
# Список задач проекта

# Ожидающие задачи (готовы к выполнению)
pending_tasks:
  - task_id: "REQ-001-TASK-001"
    branch: "feature/REQ-001-TASK-001-add-feature"
    description: "Краткое описание задачи"
    prp_file: "task_management/plans/REQ-001-TASK-001-PRP.md"
    status: "prp_ready"
    blockers: []
    priority: "high"
    depends_on: []

# Активные задачи (в работе)
active_tasks:
  - task_id: "REQ-001-TASK-002"
    branch: "feature/REQ-001-TASK-002-fix-bug"
    description: "Исправление бага в обработке"
    prp_file: "task_management/plans/REQ-001-TASK-002-PRP.md"
    status: "in_progress"
    blockers: []
    assignee: ""

# Завершённые задачи
completed_tasks:
  - task_id: "REQ-001-TASK-000"
    branch: "feature/REQ-001-TASK-000-init"
    description: "Инициализация проекта"
    status: "completed"
    completed_at: "2026-01-05T14:30:00Z"
    pr_link: "https://github.com/org/repo/pull/123"

# Заблокированные задачи
blocked_tasks:
  - task_id: "REQ-001-TASK-003"
    branch: "feature/REQ-001-TASK-003-integration"
    description: "Интеграция с внешним сервисом"
    prp_file: ""
    status: "blocked"
    blockers:
      - "Ожидание API от внешнего сервиса"
      - "REQ-001-TASK-002"
    depends_on: ["REQ-001-TASK-002"]

# Метаданные
metadata:
  generated_at: "2026-01-05T10:00:00Z"
  last_updated: "2026-01-05T14:30:00Z"
  total_pending: 1
  total_active: 1
  total_completed: 1
  total_blocked: 1
```

## Типы задач и ID-формат

| Тип задачи | Формат ID | Пример |
|------------|-----------|--------|
| Feature task | `REQ-{XXX}-TASK-{YYY}` | `REQ-001-TASK-005` |
| Architecture Decision | `REQ-{XXX}-ADR-{YYY}` | `REQ-001-ADR-001` |
| Research task | `REQ-{XXX}-RESEARCH-{YYY}` | `REQ-001-RESEARCH-002` |
| Bug fix | `REQ-{XXX}-BUGFIX-{YYY}` | `REQ-001-BUGFIX-001` |
| Memory Bank task | `MB_TASK-{XXX}` | `MB_TASK-001` |

## Статусы задач

### Feature tasks (TASK-XXX)

| Статус | Описание |
|--------|----------|
| `pending` | Задача создана, ожидает начала |
| `prp_ready` | PRP сгенерирован, готова к выполнению |
| `in_progress` | Задача выполняется |
| `pending_review` | Код написан, ожидает ревью |
| `testing` | На тестировании |
| `completed` | Завершена |
| `merged` | PR смержен |
| `blocked` | Заблокирована |

### Architecture (ADR-XXX)

| Статус | Описание |
|--------|----------|
| `pending_analysis` | Ожидает анализа |
| `analysis_complete` | Анализ завершен |
| `adr_drafted` | ADR написан |
| `pending_review` | Ожидает ревью |
| `approved` | Утвержден |

### Research (RESEARCH-XXX)

| Статус | Описание |
|--------|----------|
| `pending_research` | Ожидает начала |
| `in_progress` | Исследование в процессе |
| `research_complete` | Исследование завершено |
| `documented` | Результаты задокументированы |

### Bug fixes (BUGFIX-XXX)

| Статус | Описание |
|--------|----------|
| `pending_analysis` | Ожидает анализа |
| `analysis_complete` | Причина найдена |
| `in_progress` | Исправление в процессе |
| `pending_review` | Ожидает ревью |
| `completed` | Исправлено |

## Поля задачи

### Обязательные поля

| Поле | Тип | Описание |
|------|-----|----------|
| `task_id` | string | Уникальный идентификатор задачи |
| `description` | string | Краткое описание (1-2 предложения) |
| `status` | string | Текущий статус из справочника |

### Опциональные поля

| Поле | Тип | Описание |
|------|-----|----------|
| `branch` | string | Название git ветки |
| `prp_file` | string | Путь к PRP файлу |
| `blockers` | list | Список блокирующих факторов |
| `depends_on` | list | Список task_id зависимостей |
| `priority` | string | `high` / `medium` / `low` |
| `assignee` | string | Исполнитель (опционально) |
| `completed_at` | string | ISO 8601 дата завершения |
| `pr_link` | string | Ссылка на PR |

## Правила обновления

### При создании задачи

1. Добавить в `pending_tasks`
2. Установить статус `pending`
3. Заполнить обязательные поля

```yaml
pending_tasks:
  - task_id: "REQ-001-TASK-010"
    description: "Новая задача"
    status: "pending"
    blockers: []
```

### При генерации PRP

1. Обновить статус на `prp_ready`
2. Добавить путь к PRP

```yaml
pending_tasks:
  - task_id: "REQ-001-TASK-010"
    description: "Новая задача"
    prp_file: "task_management/plans/REQ-001-TASK-010-PRP.md"
    status: "prp_ready"
```

### При начале выполнения

1. Переместить из `pending_tasks` в `active_tasks`
2. Обновить статус на `in_progress`
3. Добавить название ветки

```yaml
active_tasks:
  - task_id: "REQ-001-TASK-010"
    branch: "feature/REQ-001-TASK-010-feature-name"
    description: "Новая задача"
    prp_file: "task_management/plans/REQ-001-TASK-010-PRP.md"
    status: "in_progress"
    blockers: []
```

### При завершении

1. Переместить в `completed_tasks`
2. Обновить статус на `completed`
3. Добавить `completed_at`
4. Добавить `pr_link` если есть

```yaml
completed_tasks:
  - task_id: "REQ-001-TASK-010"
    branch: "feature/REQ-001-TASK-010-feature-name"
    description: "Новая задача"
    status: "completed"
    completed_at: "2026-01-07T15:00:00Z"
    pr_link: "https://github.com/org/repo/pull/456"
```

### При блокировке

1. Переместить в `blocked_tasks`
2. Обновить статус на `blocked`
3. Заполнить `blockers` с описанием причин

```yaml
blocked_tasks:
  - task_id: "REQ-001-TASK-010"
    description: "Новая задача"
    status: "blocked"
    blockers:
      - "Ожидание решения по архитектуре"
      - "REQ-001-ADR-002"
    depends_on: ["REQ-001-ADR-002"]
```

## Обновление метаданных

**ВСЕГДА** обновлять `metadata` при изменении списка:

```yaml
metadata:
  last_updated: "2026-01-07T15:30:00Z"  # Текущая дата/время
  total_pending: 3
  total_active: 2
  total_completed: 15
  total_blocked: 1
```

## Связь с REQ файлами

После создания или завершения задачи - обновить checkbox в REQ файле:

```markdown
## Связанные задачи
- [x] [REQ-001-TASK-001: Инициализация](REQ-001-TASK-001.md) ✓ completed
- [ ] [REQ-001-TASK-002: Основная логика](REQ-001-TASK-002.md) → in_progress
- [ ] [REQ-001-TASK-003: Тестирование](REQ-001-TASK-003.md) → pending
```

## Пример полного файла

```yaml
# Tasks List - Outboxer
# Список задач проекта

pending_tasks:
  - task_id: "REQ-002-TASK-001"
    branch: "feature/REQ-002-TASK-001-new-transport"
    description: "Добавить новый транспорт для Kafka"
    prp_file: "task_management/plans/REQ-002-TASK-001-PRP.md"
    status: "prp_ready"
    blockers: []
    priority: "high"
    depends_on: []

active_tasks:
  - task_id: "REQ-001-TASK-015"
    branch: "feature/REQ-001-TASK-015-metrics"
    description: "Расширить метрики Prometheus"
    prp_file: "task_management/plans/REQ-001-TASK-015-PRP.md"
    status: "in_progress"
    blockers: []

completed_tasks:
  - task_id: "REQ-001-TASK-014"
    branch: "feature/REQ-001-TASK-014-partitioning"
    description: "Реализовать партиционирование таблиц"
    status: "completed"
    completed_at: "2026-01-06T18:00:00Z"
    pr_link: "https://github.com/webdelo/outboxer/pull/42"

blocked_tasks: []

metadata:
  generated_at: "2026-01-05T10:00:00Z"
  last_updated: "2026-01-07T09:15:00Z"
  total_pending: 1
  total_active: 1
  total_completed: 14
  total_blocked: 0
```

## Валидация

При редактировании проверять:

1. **Уникальность task_id** - не должно быть дубликатов
2. **Корректность статуса** - только из справочника
3. **Актуальность metadata** - счётчики соответствуют реальности
4. **Форматирование дат** - ISO 8601 формат

## Ссылки

- [tasks_list.yaml](../../.memory_bank/tasks_list.yaml) - текущий файл задач
- [tasks_list template](../../.claude/templates/tasks_list.yaml) - шаблон
