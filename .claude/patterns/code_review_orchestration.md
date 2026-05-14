# Code Review Orchestration Pattern

## Когда использовать

При выполнении `/review_code` команды для проверки множественных пакетов.

## Workflow

### Stage 1: Planning

```
1. Определить список пакетов для review
2. Разделить на batches (max 5 concurrent agents)
3. Запустить agents в background с blocking wait мониторингом
```

### Stage 2: Parallel Execution

```yaml
Launch code-reviewer agents in background:
  - run_in_background: true
  - max_concurrent: 5
  - monitor with: TaskOutput(block=true, timeout=120000)
```

### Stage 3: Aggregation (ОБЯЗАТЕЛЬНЫЙ АВТОМАТИЧЕСКИЙ ШАГ)

> **КРИТИЧНО**: Этот этап НЕЛЬЗЯ пропускать!
> Агрегация ВСЕГДА запускается АВТОМАТИЧЕСКИ после завершения всех субагентов.
> Это НЕ опциональный шаг - это ОБЯЗАТЕЛЬНАЯ часть workflow.

**КРИТИЧЕСКИ ВАЖНО:**

1. **Правильный путь к выходному файлу:**
   ```
   task_management/reviews/{TASK_ID}-CODE-REVIEW.yaml

   Пример: task_management/reviews/REQ-001-TASK-021-CODE-REVIEW.yaml
   ```

2. **Минимальный промпт агрегатору:**
   ```
   Aggregate all code review reports from .memory_bank/agent_task_results/{DATE}_code-review_*.yaml

   Output file: task_management/reviews/{TASK_ID}-CODE-REVIEW.yaml
   Report type: code-review

   Follow template: .claude/agents/report-aggregator.md
   ```

3. **Минимальный ответ оркестратора после агрегации:**
   ```
   ✅ Code review завершён!

   📄 Отчёт: task_management/reviews/{TASK_ID}-CODE-REVIEW.yaml

   {результат от агрегатора - НЕ дублировать}
   ```

## Антипаттерны (НЕ делать)

❌ **Указывать неправильный путь:**
```
Output File: .memory_bank/agent_task_results/2026-01-25_CODE-REVIEW_lib-core-FINAL.yaml
```

❌ **Дублировать информацию из агрегатора:**
```
## 🎉 ФИНАЛЬНЫЙ CODE REVIEW ЗАВЕРШЁН!

[огромная таблица на 100 строк]
[ТОП-5 критических проблем]
[детальный breakdown]
...
```

✅ **Правильный подход:**
```
✅ Code review завершён!

📄 Отчёт: task_management/reviews/REQ-001-TASK-021-CODE-REVIEW.yaml

[компактный ответ агрегатора БЕЗ изменений]
```

## Экономия токенов

| Подход | Токены | Примечание |
|--------|--------|------------|
| Дублирование info | ~8000 | Избыточно |
| Только ссылка на файл | ~500 | Оптимально |
| **Экономия** | **~7500** | **94% reduction** |

## Пример правильного промпта агрегатору

```
Aggregate all code review reports from .memory_bank/agent_task_results/2026-01-25_code-review_*.yaml

**Output File:**
task_management/reviews/REQ-001-TASK-021-CODE-REVIEW.yaml

**Report Type:** code-review

Follow aggregation process from .claude/agents/report-aggregator.md
```

## Пример правильного ответа оркестратора

```
✅ Code review завершён!

📄 Отчёт: task_management/reviews/REQ-001-TASK-021-CODE-REVIEW.yaml

[вставить ответ агрегатора БЕЗ изменений]
```
