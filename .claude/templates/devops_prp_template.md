# {REQ-XXX-TASK-YYY}-PRP: {Title}

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (DevOps) v1.1 |
| Plan ID | {REQ-XXX-TASK-YYY-PRP} |
| Parent Request | [{REQ-XXX}](../tasks/{REQ-XXX}.md) |
| Parent Task | [{REQ-XXX-TASK-YYY}](../tasks/{REQ-XXX-TASK-YYY}.md) |
| Created | {YYYY-MM-DD} |
| Updated | {YYYY-MM-DD} |

---

## Goal

### Summary
{Краткое описание инфраструктурной задачи - 1-2 предложения}

### Why
- {Бизнес/техническая необходимость}
- {Влияние на надёжность/масштабируемость/безопасность}
- {Какие проблемы решает}

### What
1. {Компонент/сервис 1}
2. {Компонент/сервис 2}
3. {Документация}

### Success Criteria
- [ ] {Критерий 1 - измеримый результат}
- [ ] {Критерий 2}
- [ ] Сервисы запускаются без ошибок
- [ ] Метрики собираются корректно
- [ ] Документация создана

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| DevOps Rules Index | `.claude/guides/devops/rules_index.yaml:1` | {Правила} |
| Docker Guide | `.claude/guides/devops/docker.md:1` | DK-001, DK-002 |
| {DB Guide} | `.claude/guides/devops/{db}.md:1` | {Если есть БД} |
| Tech Stack | `.memory_bank/tech_stack.yaml:1` | {Релевантные секции} |

---

## Analysis Files

- **Code Research**: `{path_to_research_file}`
- **Web Research**: `{path_to_web_research}`

---

## Context

### Current State

**{Component 1}** (`{path/to/config}`):
- {Текущее состояние 1}
- {Текущее состояние 2}

**{Component 2}**:
- {Текущее состояние}

### Current Structure

```
{Текущая структура - только релевантные файлы}
```

### Desired Structure

```
{Структура после реализации}
path/
├── file1.yaml          # NEW: описание
├── file2.json          # NEW: описание
└── existing.yaml       # UPDATE: что изменено
```

### Gotchas

| Category | Issue | Solution |
|----------|-------|----------|
| {docker} | {Проблема} | {Решение} |
| {prometheus} | {Проблема} | {Решение} |

---

## Implementation

### Task 1: {Name}

**Action**: {CREATE | MODIFY | DELETE}
**File**: `{path/to/file}`

**Description**: {Что делаем}

**Configuration**:

```yaml
# Пример конфигурации
service_name:
  image: {image}:{tag}
  container_name: {name}
  restart: unless-stopped
  ports:
    - "{host_port}:{container_port}"
  volumes:
    - {volume_mount}
  environment:
    - {ENV_VAR}={value}
  networks:
    - {network}
```

**Details**:
- {Детальный шаг 1}
- {Детальный шаг 2}

---

### Task 2: {Name}

**Action**: {CREATE | MODIFY | DELETE}
**File**: `{path/to/file}`

**Description**: {Что делаем}

**Add to existing config**:

```yaml
# Добавить в секцию X
{config_snippet}
```

---

### Task 3: Create Dashboard {Name}

**Action**: CREATE
**File**: `{path/to/dashboard.json}`

**Description**: {Описание дашборда}

**Key Panels**:

| Panel | Type | Query |
|-------|------|-------|
| {Panel Name} | {stat/timeseries/gauge} | `{promql_query}` |
| {Panel Name} | {type} | `{query}` |

**Reference**: [{External Dashboard}]({url})

---

## Security Checklist

- [ ] Container runs as non-root user
- [ ] No secrets hardcoded in configs
- [ ] Secrets via environment variables / secret store
- [ ] Network policies defined (if K8s)
- [ ] TLS configured (if applicable)

---

## Testing & Validation

### Validation Commands

```bash
# Проверить синтаксис конфигов
{validation_command_1}

# Проверить JSON файлы
{validation_command_2}
```

### Manual Checks

**{Component 1}**:
1. {Проверка 1}
2. {Проверка 2}

**{Component 2}**:
1. {Проверка 1}
2. {Проверка 2}

### Test Scenarios

| Scenario | Steps | Expected |
|----------|-------|----------|
| {Scenario 1} | {Шаги} | {Ожидаемый результат} |
| {Scenario 2} | {Шаги} | {Ожидаемый результат} |

---

## Deployment

### Steps

1. {Шаг деплоя 1}
2. {Шаг деплоя 2}

### Rollback

```bash
# Откат
{rollback_command}
```

---

## Dependencies & Exclusions

### Required Dependencies
- **{TASK-XXX}** ({STATUS}): {Описание зависимости}

### Future Work
- **{TASK-YYY}**: {Что будет сделано позже}

### Not In Scope
- {Что явно НЕ входит в задачу 1}
- {Что явно НЕ входит в задачу 2}

---

## DOD (Definition of Done)

- [ ] Все сервисы запускаются без ошибок
- [ ] Конфиги валидны (проверено командами)
- [ ] Метрики собираются (если мониторинг)
- [ ] Дашборды отображают данные (если Grafana)
- [ ] Документация создана
- [ ] Данные сохраняются между перезапусками (persistence)

