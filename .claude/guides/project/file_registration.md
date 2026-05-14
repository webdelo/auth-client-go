# Регистрация файлов в tasks_list.yaml

## ⚠️ ВАЖНО: Автоматическая регистрация

**С версии 2.0 (2026-01-27)** регистрация файлов происходит **автоматически** через команду `/refresh-req-status`.

**Ручная регистрация больше не требуется!**

Просто запустите после выполнения задачи:
```bash
/refresh-req-status REQ-001
```

Скрипт автоматически:
- ✅ Найдет все файлы задач
- ✅ Обнаружит связанные файлы (PRP, result, code review, bug report)
- ✅ Добавит отсутствующие задачи в tasks_list.yaml
- ✅ Обновит статусы и ссылки на файлы

## Зачем это нужно

`.memory_bank/tasks_list.yaml` используется для:
- Быстрого поиска связанных файлов (без сканирования директорий)
- Явных связей между задачами и файлами
- Корректной работы команды `/request-status` с кликабельными ссылками

## Какие файлы регистрировать

| Тип файла | Когда создается | Команда |
|-----------|----------------|---------|
| `prp` | Problem-Resolving Plan | `/generate-prp`, `/prp-generate` |
| `result` | Result Report после выполнения | `/execute-prp`, `/prp-execute` |
| `code_review` | Code Review Report | `/review_code` |
| `bug_report` | Bug Report | `/review-code-it` |
| `debug_report` | Debug Report после исправления багов | `/execute-bug-fix` |
| `task` | Task файл (редко) | Создается вручную |

## Ручная регистрация (Legacy, не рекомендуется)

> **Примечание**: Используйте только если автоматическая синхронизация не работает.

### Через Python скрипт

```bash
python3 .claude/scripts/req_status.py add-file <TASK-ID> <TYPE> <FILE-PATH>
```

**Типы файлов**:
- `prp` - Problem-Resolving Plan
- `result` - Result Report
- `code_review` - Code Review Report
- `bug_report` - Bug Report
- `debug_report` - Debug Report (результат исправления багов)
- `task` - Task файл

### Примеры

```bash
# После создания PRP (если refresh не запущен)
python3 .claude/scripts/req_status.py add-file REQ-001-TASK-007 prp task_management/plans/REQ-001-TASK-007-PRP.md

# После code review (итоговый агрегированный отчёт)
python3 .claude/scripts/req_status.py add-file REQ-001-TASK-007 code_review task_management/reviews/REQ-001-TASK-007-CODE-REVIEW.yaml
```

### Когда использовать ручную регистрацию

- Нестандартное имя файла (не соответствует паттерну)
- Файл в неожиданной директории
- Нужно срочно зарегистрировать файл без полной синхронизации

## Структура в tasks_list.yaml

После регистрации задача в YAML будет выглядеть так:

```yaml
- task_id: "REQ-001-TASK-007"
  branch: "feature/REQ-001-TASK-007"
  description: "Описание задачи"
  task_file: "task_management/tasks/REQ-001-TASK-007.md"
  prp_file: "task_management/plans/REQ-001-TASK-007-PRP.md"
  result_report: "task_management/results/REQ-001-TASK-007-result-report.md"
  code_review_files:
    - "task_management/reviews/REQ-001-TASK-007-CODE-REVIEW.yaml"
  bug_report_files:
    - "task_management/bug_reports/REQ-001-TASK-007-BUG-REPORT.yaml"
  debug_report_files:
    - "task_management/debug_reports/REQ-001-TASK-007-DEBUG-REPORT.yaml"
  lifecycle:
    execution:
      completed: true
      completed_at: "2026-01-27T12:00:00Z"
    code_review:
      completed: true
      completed_at: "2026-01-27"
      verdict: "APPROVED_WITH_COMMENTS"
      critical_issues: 0
      warning_issues: 5
      suggestion_issues: 10
    bug_fixing:
      has_bugs: true
      bugs_fixed: true
      critical_bugs: 0
      high_bugs: 2
      medium_bugs: 3
      low_bugs: 0
  status: "completed"
  priority: "high"
```

## Автоматическая синхронизация

Команда `/refresh-req-status` выполняет полную синхронизацию:

1. **Сканирует filesystem** для поиска всех файлов задач `{REQ-ID}-TASK-*.md`
2. **Находит связанные файлы** для каждой задачи:
   - PRP: `task_management/plans/{TASK-ID}-PRP.md`
   - Result: `task_management/results/{TASK-ID}-result-report.md`
   - Code Review: `task_management/reviews/{TASK-ID}-CODE-REVIEW.yaml`
   - Bug Report: `task_management/bug_reports/{TASK-ID}-BUG-REPORT.yaml`
   - Debug Report: `task_management/debug_reports/{TASK-ID}-DEBUG-REPORT.yaml`
3. **Добавляет отсутствующие задачи** в tasks_list.yaml
4. **Обновляет статусы** на основе наличия файлов
5. **Синхронизирует файлы** - добавляет новые ссылки на файлы

## Когда использовать

Запускайте `/refresh-req-status <REQ-ID>`:
- После выполнения задач
- После создания PRP
- После code review
- Периодически для синхронизации (раз в день/неделю)
- После git pull (если коллеги добавили задачи)

## Проверка

После регистрации проверьте результат:

```bash
# Посмотреть статус задачи
python3 .claude/scripts/req_status.py status REQ-001

# Или напрямую в YAML
grep -A 10 "REQ-001-TASK-007" .memory_bank/tasks_list.yaml
```
