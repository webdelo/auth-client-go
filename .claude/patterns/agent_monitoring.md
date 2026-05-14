# Проактивный мониторинг агентов

## Проблема

**Текущее поведение (пассивное):**
```
1. Оркестратор запускает N агентов в background
2. ЖДЁТ уведомлений от системы
3. Если агент зависнет → ничего не происходит
4. Оркестратор не знает о проблеме
5. Пользователь вынужден вручную проверять статус
```

**Последствия:**
- Зависшие агенты не обнаруживаются
- Нет информации о прогрессе
- Невозможно оценить время завершения
- Токены расходуются неэффективно

## Решение: Проактивный мониторинг

**Новое поведение:**
```
1. Запустить N агентов в background
2. Сохранить список agent IDs
3. ПЕРИОДИЧЕСКИ (каждые 30-60 сек) проверять статус:
   - TaskOutput(agent_id, block=false, timeout=5000)
   - Если completed → обработать результат
   - Если running → показать прогресс (tokens, duration)
   - Если зависло (no progress > 5 min) → предупредить
4. Если все завершены → продолжить workflow
5. Если есть ошибки → показать и предложить действия
```

**Преимущества:**
- ✅ Раннее обнаружение зависших агентов
- ✅ Видимость прогресса для пользователя
- ✅ Возможность остановки/перезапуска проблемных агентов
- ✅ Лучшая диагностика проблем

---

## Паттерн реализации

### Шаг 1: Запуск агентов с сохранением ID

После запуска агентов сохранить их IDs в структурированном виде:

```
Запущено 10 агентов в background:

Tracking агентов:
- agent_1 (ab3f8aa): code-reviewer lib/core
- agent_2 (a28f29b): code-reviewer lib/core/config
- agent_3 (aceade7): code-reviewer lib/core/consumer
... (всего 10)

Начинаю мониторинг с интервалом 30 сек...
```

### Шаг 2: Цикл мониторинга

**Псевдокод:**

```python
agents = [
    {"id": "ab3f8aa", "name": "code-reviewer (lib/core)", "status": "running"},
    {"id": "a28f29b", "name": "code-reviewer (lib/core/config)", "status": "running"},
    # ... остальные агенты
]

iteration = 0
max_iterations = 20  # 20 * 30 сек = 10 минут максимум

while has_running_agents(agents) and iteration < max_iterations:
    iteration += 1

    # Ждём 30 секунд перед проверкой
    sleep(30)

    # Проверяем каждого агента
    for agent in agents:
        if agent["status"] == "completed":
            continue

        # Проверить статус через TaskOutput
        status = check_agent_status(agent["id"])

        if status == "completed":
            agent["status"] = "completed"
            agent["result"] = get_agent_result(agent["id"])
            print(f"✅ {agent['name']} завершён")

        elif status == "running":
            progress = get_agent_progress(agent["id"])
            print(f"⏳ {agent['name']}: {progress['tokens']} tokens, {progress['duration']}")

        elif status == "error":
            agent["status"] = "error"
            print(f"❌ {agent['name']}: ОШИБКА")

    # Показать общий прогресс
    completed = count_completed(agents)
    print(f"\nПрогресс: {completed}/{len(agents)} агентов завершено")

# Проверка результата
if all_completed(agents):
    print("✅ Все агенты завершены успешно")
    proceed_to_aggregation()
else:
    print("⚠️ Некоторые агенты не завершились:")
    show_stuck_agents(agents)
```

### Шаг 3: Реализация в Claude Code

**Важно:** Claude Code (LLM) не может выполнять циклы напрямую. Вместо этого используем рекурсивный подход:

```
1. Запустить агентов
2. Сохранить IDs в текстовом сообщении
3. Через Bash sleep 30 && проверить статус
4. Показать прогресс пользователю
5. Если не все завершены → повторить шаг 3
6. Если все завершены → продолжить workflow
```

**Пример реализации:**

```markdown
## Monitoring Iteration 1

<bash>
sleep 30
</bash>

Проверяю статус агентов...

<TaskOutput task_id="ab3f8aa" block=false timeout=5000 />
<TaskOutput task_id="a28f29b" block=false timeout=5000 />
... (для каждого агента)

Результат:
- ✅ ab3f8aa (lib/core): completed (45.2k tokens, 2m 15s)
- ⏳ a28f29b (lib/core/config): running (23.1k tokens, 1m 30s)
- ⏳ aceade7 (lib/core/consumer): running (67.8k tokens, 3m 05s)
...

Прогресс: 1/10 агентов завершено

Агенты ещё работают. Продолжаю мониторинг...
```

**Если все завершены:**

```markdown
Все агенты завершены успешно! ✅

Сводка:
- code-reviewer (lib/core):         2m 15s,  45.2k tokens
- code-reviewer (lib/core/config):  1m 42s,  30.8k tokens
- code-reviewer (lib/core/consumer): 3m 20s, 72.4k tokens
...

Общее время: 3m 20s (параллельное выполнение)
Общие токены: 524k tokens
Средняя стоимость: $2.48 USD

Запускаю агрегатор...
```

---

## Интеграция в команды

Мониторинг применяется во всех командах, использующих background агентов:

- `/review-code` — code review (до 10 агентов параллельно)
- `/execute-bug-fix` — исправление багов (агенты по категориям)
- `/execute-prp` — выполнение PRP (волны агентов)
- `/run-tasks` — оркестрация задач

**Добавления к шагам запуска агентов:**

1. После запуска всех агентов сохранить список agent IDs
2. Запустить цикл мониторинга:
   - Каждые 30 секунд проверять статус через TaskOutput(block=false)
   - Показывать прогресс (tokens, duration)
   - Обнаруживать зависшие агенты (no progress > 5 min)
3. Когда все агенты завершены → продолжить к следующему этапу

---

## Обнаружение зависших агентов

**Критерии "зависания":**

1. **No progress > 5 min:**
   - Tokens не растут
   - Duration растёт, но output не меняется

2. **Excessive duration:**
   - Агент работает > 10 минут
   - Превышен ожидаемый максимум

3. **Error state:**
   - TaskOutput возвращает error
   - Output содержит stack trace

**Действия при обнаружении:**

```markdown
⚠️ Обнаружен зависший агент!

Agent: aceade7 (code-reviewer lib/core/consumer)
Status: No progress for 5m 30s
Last tokens: 67.8k (не растёт)
Duration: 8m 15s

Возможные причины:
- Ожидание внешнего ресурса (timeout)
- Бесконечный цикл в агенте
- Deadlock в concurrency коде

Действия:
1. Прочитать последние 100 строк output:
   tail -100 /tmp/claude/.../tasks/aceade7.output

2. Если агент точно завис:
   KillShell(shell_id="aceade7")

3. Перезапустить агента с меньшим scope или другой стратегией
```

---

## Оптимизация токенов

**Проблема:** 10 агентов × частые проверки = много вызовов TaskOutput

**Решение:**

1. **Batch checking:**
   - Проверять все агенты в одном сообщении (10 TaskOutput calls)
   - Интервал 30-60 сек (не чаще)

2. **Adaptive interval:**
   - Первые 2 минуты: проверка каждые 30 сек
   - 2-5 минут: проверка каждые 60 сек
   - После 5 минут: проверка каждые 2 минуты

3. **Early stop:**
   - Если 90% агентов завершены → проверять чаще (каждые 15 сек)
   - Последние 1-2 агента обычно завершаются быстро

**Оценка стоимости:**

```
10 агентов, среднее время 3 минуты, интервал 30 сек:

Проверок: 3 мин / 30 сек = 6 iterations
Вызовов: 6 × 10 агентов = 60 TaskOutput calls
Токены: 60 × ~500 tokens = 30k tokens

Стоимость: ~$0.15 USD (haiku model)
```

Для сравнения:
- Пассивное ожидание: 0 tokens, но агенты могут зависнуть
- Ручная проверка пользователем: friction, плохой UX

**Вывод:** Мониторинг стоит дополнительных $0.15 для лучшего UX.

---

## Примеры использования

### Пример 1: Code Review (10 агентов)

```markdown
Запущено 10 агентов code review в background.

Tracking:
┌─────────────────────────────────────────────────────────────┐
│ ID       │ Agent                      │ Status   │ Progress │
├─────────────────────────────────────────────────────────────┤
│ ab3f8aa  │ code-reviewer (lib/core)   │ running  │ 45.2k    │
│ a28f29b  │ code-reviewer (config)     │ running  │ 30.8k    │
│ aceade7  │ code-reviewer (consumer)   │ running  │ 67.8k    │
│ ...      │ ...                        │ ...      │ ...      │
└─────────────────────────────────────────────────────────────┘

Начинаю мониторинг с интервалом 30 сек...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Iteration 1 (00:30):
- ✅ lib/core: completed (2m 15s, 45.2k tokens)
- ⏳ config: running (1m 30s, 30.8k tokens)
- ⏳ consumer: running (1m 45s, 67.8k tokens)
...

Progress: [██        ] 1/10 агентов

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Iteration 2 (01:00):
- ✅ lib/core: completed (2m 15s, 45.2k tokens)
- ✅ config: completed (1m 58s, 42.1k tokens)
- ⏳ consumer: running (2m 15s, 89.3k tokens)
...

Progress: [████      ] 2/10 агентов

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Все 10 агентов завершены!

Сводка:
- Общее время: 3m 42s (параллельное выполнение)
- Общие токены: 524k tokens
- Средняя стоимость: $2.48 USD

Запускаю агрегатор...
```

### Пример 2: Bug Fix (4 агента)

```markdown
Запущено 4 агента для исправления категорий:
- STYLE (refactorer): 36 issues
- LOGIC (refactorer): 5 issues
- CONCURRENCY (debugger): 2 issues
- TESTS (unit-test-writer): 10 issues

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Iteration 1 (00:30):
- ⏳ STYLE: running (1m 15s, 34.2k tokens) - 18/36 fixes done
- ⏳ LOGIC: running (1m 20s, 28.7k tokens) - 2/5 fixes done
- ✅ CONCURRENCY: completed (1m 05s, 19.4k tokens) - 2/2 fixes ✓
- ⏳ TESTS: running (1m 30s, 45.8k tokens) - writing tests...

Progress: [██        ] 1/4 категорий

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

...

✅ Все исправления завершены!

Результат:
- STYLE: 36/36 fixes ✓
- LOGIC: 5/5 fixes ✓
- CONCURRENCY: 2/2 fixes ✓
- TESTS: 10/10 tests written ✓

Запускаю валидацию (make test && make lint)...
```

---

## Checklist для внедрения

### Для разработчика Claude Code:

- [x] Создать `.claude/patterns/agent_monitoring.md` (этот файл)
- [ ] Обновить `.claude/AGENTS.yaml` с примерами мониторинга
- [ ] Добавить секцию в `CLAUDE.md` про мониторинг

### Для оркестратора (при выполнении):

При запуске background агентов:

- [ ] Сохранить agent IDs в структурированном списке
- [ ] Вывести таблицу tracking с колонками: ID, Agent, Status, Progress
- [ ] Запустить цикл мониторинга с интервалом 30 сек
- [ ] Каждую итерацию показывать progress bar и статус каждого агента
- [ ] При завершении всех агентов показать summary
- [ ] Обнаруживать зависшие агенты (no progress > 5 min)
- [ ] При обнаружении зависания предложить действия

---

## FAQ

**Q: Что делать если агент зависнет?**

A:
1. Прочитать последние 100 строк output для диагностики
2. Если точно завис → убить через KillShell
3. Проанализировать причину (timeout, бесконечный цикл, etc)
4. Перезапустить с меньшим scope или другой стратегией

**Q: Как часто проверять статус?**

A:
- Оптимально: каждые 30-60 сек
- Слишком часто (< 30 сек): лишние токены
- Слишком редко (> 2 мин): плохой UX, поздно обнаружим проблемы

**Q: Сколько стоит мониторинг?**

A:
- 10 агентов, 3 минуты работы, проверка каждые 30 сек
- ~60 TaskOutput calls × 500 tokens = 30k tokens
- Стоимость: ~$0.15 USD (haiku model)
- Это приемлемо для улучшения UX

**Q: Можно ли показывать progress bar?**

A: Да! Claude Code может выводить ASCII progress bars:

```
Progress: [████████  ] 8/10 агентов (80%)

Completed:
✅ lib/core
✅ lib/core/config
✅ lib/core/consumer
✅ lib/core/message
✅ lib/core/observability
✅ lib/core/outboxer
✅ lib/core/producer
✅ lib/core/storage

Running:
⏳ lib/core/transport (2m 15s, 67.8k tokens)
⏳ test-reviewer (3m 05s, 89.2k tokens)
```

**Q: Что если все агенты завершились раньше следующей проверки?**

A: Система отправит уведомление о завершении агента, и оркестратор получит его сразу. Периодические проверки — это дополнительная страховка для случаев, когда уведомления не приходят.

---

## Итого

Проактивный мониторинг агентов:

✅ Решает проблему зависших агентов
✅ Улучшает UX (видимость прогресса)
✅ Позволяет раннюю диагностику проблем
✅ Стоит всего $0.10-0.20 за сессию
✅ Легко интегрируется во все workflow

**Рекомендация:** Внедрить во все команды, использующие background агентов.
