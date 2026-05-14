# {REQ-XXX-TASK-YYY}-PRP: {Title}

## Metadata

| Field | Value |
|-------|-------|
| Template | Problem-Resolving Plan (Coding) v1.1 |
| Plan ID | {REQ-XXX-TASK-YYY-PRP} |
| Parent Request | [{REQ-XXX}](../tasks/{REQ-XXX}.md) |
| Parent Task | [{REQ-XXX-TASK-YYY}](../tasks/{REQ-XXX-TASK-YYY}.md) |
| Created | {YYYY-MM-DD} |
| Updated | {YYYY-MM-DD} |

---

## Goal

### Summary
{Краткое описание что нужно реализовать - 1-2 предложения}

### Why
- {Бизнес-ценность или техническая необходимость}
- {Какие проблемы решает}
- {Интеграция с существующим функционалом}

### What
{Описание функциональности с точки зрения пользователя/системы}

### Success Criteria
- [ ] {Критерий 1 - измеримый результат}
- [ ] {Критерий 2}
- [ ] Код компилируется без ошибок
- [ ] Тесты проходят
- [ ] Линтер проходит

---

## Applied Standards

| Standard | Path | Applied Rules |
|----------|------|---------------|
| Rules Index | [rules_index.yaml](../../.claude/guides/rules_index.yaml) | Загружен |
| Effective Standards | [effective_standards.md](../../.claude/guides/golang/effective_standards.md) | ES-0051, ES-0055, ES-0060 |
| Effective Concurrency | [effective_concurrency.md](../../.claude/guides/golang/effective_concurrency.md) | {EC-XXXX если есть concurrency} |
| Coding Style | [coding_style.md](../../.claude/guides/golang/coding_style.md) | CS-0010 |

### Concurrency Patterns (если применимо)
- [ ] Worker Pool
- [ ] Semaphore
- [ ] errgroup
- [ ] Fan-Out/Fan-In

---

## Analysis Files

- **Code Research**: [{research_file_name}](../../{path_to_research_file})
- **Web Research**: [{web_research_file_name}](../../{path_to_web_research})

---

## Context

### Current Structure

```
{Текущая структура проекта - только релевантные директории}
```

### Desired Structure

```
{Структура после реализации с пометками # NEW, # UPDATE}
```

### Gotchas

| Category | Issue | Solution |
|----------|-------|----------|
| {category} | {Известная проблема} | {Как избежать} |

---

## Implementation

### Data Models

```go
// {StructName} - {описание}
type {StructName} struct {
    {Field} {Type} `json:"{field}" db:"{field}"`
}
```

---

### Task 1: {Name}

**Action**: {CREATE | MODIFY | DELETE}
**File**: [{filename}](../../{path/to/file.go})

**Description**: {Что делаем}

**Details**:
- {Детальный шаг 1}
- {Детальный шаг 2}

**Code Pattern**:

```go
// Ключевой паттерн реализации
func {FunctionName}({params}) ({returns}) {
    // ...
}
```

---

### Task 2: {Name}

**Action**: {CREATE | MODIFY | DELETE}
**File**: [{filename}](../../{path/to/file.go})

**Description**: {Что делаем}

**Details**:
- {Детальный шаг 1}

---

## Integration

### Database (если применимо)

| Type | File | Description |
|------|------|-------------|
| Migration UP | [{NNNN}_{name}.up.sql](../../database/mysql_migrations/{NNNN}_{name}.up.sql) | {Описание} |
| Migration DOWN | [{NNNN}_{name}.down.sql](../../database/mysql_migrations/{NNNN}_{name}.down.sql) | Откат |

### Config (если применимо)

**File**: [config.go](../../config/{service}/config.go)

```go
{FieldName} {Type} `envconfig:"{ENV_VAR}"`
```

### Dependencies (если применимо)

| Package | Purpose |
|---------|---------|
| `{go.mod package}` | {Для чего} |

---

## Testing

### Unit Tests

| Entity | Test File | Cases |
|--------|-----------|-------|
| {Entity} | [{filename}_test.go](../../{path}_test.go) | {Описание кейсов} |

**Required Test Cases**:
- [ ] {Happy path}
- [ ] {Error case 1}
- [ ] {Edge case}

### Integration Tests (если применимо)

| Endpoint | Test File | Cases |
|----------|-----------|-------|
| `{METHOD /path}` | [{name}.yaml](../../tests/integration/{name}.yaml) | {Описание} |

---

## Validation

### Build & Lint

```bash
# Компиляция
go build ./...

# Линтер
make lint

# Тесты
make test
```

### Manual Checks

1. {Ручная проверка 1}
2. {Ручная проверка 2}

---

## DOD (Definition of Done)

- [ ] Код реализован согласно плану
- [ ] Unit-тесты написаны и проходят
- [ ] Integration-тесты написаны (если требуется)
- [ ] `make lint` без ошибок
- [ ] `make test` без ошибок
- [ ] Code review пройден

