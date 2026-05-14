# File Organization Policy

> Строгие правила размещения файлов для поддержания чистоты проекта.

## Размещение отчетов и результатов

| Директория | Назначение | Примеры |
|------------|------------|---------|
| `.memory_bank/agent_task_results/` | Промежуточные отчеты субагентов | `{DATE}_code-review_{package}.yaml` |
| `task_management/reviews/` | Итоговые Code Review отчеты | `{TASK-ID}-CODE-REVIEW.yaml` |
| `task_management/bug_reports/` | Bug Report отчеты (IT testing) | `{TASK-ID}-BUG-REPORT.yaml` |
| `task_management/results/` | Результаты выполнения задач | `{TASK-ID}-result-report.md` |
| `task_management/session_analysis/` | Временный анализ оркестратора | `{DATE}-{TOPIC}.md` |

## Правила

### ✅ Правильно
```
.memory_bank/agent_task_results/2026-01-27_code-review_lib-core.yaml
task_management/reviews/REQ-001-TASK-023-CODE-REVIEW.yaml
task_management/results/REQ-001-TASK-005-result-report.md
task_management/session_analysis/2026-01-27-REVIEW-ANALYSIS.md
```

### ❌ Запрещено
- Создавать отчеты в корне проекта
- Создавать отчеты в произвольных директориях
- Путать промежуточные и итоговые отчеты

## Временные файлы

Используй `.temp/` директорию:
```bash
echo "data" > .temp/verification.tmp  # Создать
rm -rf .temp/*                         # Очистить после работы
```

## Очистка перед коммитом

```bash
# Проверить лишние файлы в корне
ls -la | grep -E "\.md$|\.tmp$|\.yaml$"

# Переместить в правильные директории
mv *_ANALYSIS.md task_management/session_analysis/
mv *_SUMMARY.md task_management/session_analysis/
```
