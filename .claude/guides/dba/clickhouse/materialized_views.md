# ClickHouse Materialized Views

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| CH-006 | Use AggregatingMergeTree for real-time aggregates | [Aggregation MVs](#incremental-materialized-views-классические) |
| CH-003 | Partition by date/month for time-series | [Target Table Design](#target-table-design) |

---

## Типы Materialized Views

ClickHouse поддерживает два типа материализованных представлений:

| Тип | Описание | Обновление |
|-----|----------|------------|
| **Incremental (классические)** | Триггерятся на INSERT в source таблицу | При вставке данных |
| **Refreshable** | Периодически пересчитываются полностью | По расписанию |

---

## Incremental Materialized Views (Классические)

### CH-400: Базовый синтаксис

```sql
CREATE MATERIALIZED VIEW mv_name
TO target_table
AS SELECT ...
FROM source_table
...
```

### CH-401: Как работает

1. При `INSERT INTO source_table` данные **также** проходят через MV
2. MV выполняет трансформацию/агрегацию
3. Результат записывается в `target_table`

**Важно:** MV видит только **новые** данные, не существующие!

### CH-402: Пример: агрегация событий

```sql
-- Source таблица (сырые события)
CREATE TABLE events (
    event_id UUID,
    user_id UInt64,
    event_type String,
    amount Decimal(18, 2),
    created_at DateTime
) ENGINE = MergeTree()
ORDER BY (user_id, created_at);

-- Target таблица (агрегаты)
CREATE TABLE daily_user_stats (
    date Date,
    user_id UInt64,
    events_count UInt64,
    total_amount Decimal(18, 2)
) ENGINE = SummingMergeTree()
ORDER BY (date, user_id);

-- Materialized View
CREATE MATERIALIZED VIEW events_to_stats TO daily_user_stats AS
SELECT
    toDate(created_at) as date,
    user_id,
    count() as events_count,
    sum(amount) as total_amount
FROM events
GROUP BY date, user_id;
```

### CH-403: MV без явной target таблицы

```sql
-- ClickHouse создаст скрытую таблицу .inner.mv_name
CREATE MATERIALIZED VIEW mv_daily_stats
ENGINE = SummingMergeTree()
ORDER BY (date, user_id)
AS SELECT
    toDate(created_at) as date,
    user_id,
    count() as events_count
FROM events
GROUP BY date, user_id;
```

### CH-404: Цепочки MV (Chained)

```sql
-- Raw → Hourly
CREATE MATERIALIZED VIEW hourly_stats TO stats_hourly AS
SELECT
    toStartOfHour(created_at) as hour,
    user_id,
    count() as events
FROM events
GROUP BY hour, user_id;

-- Hourly → Daily
CREATE MATERIALIZED VIEW daily_stats TO stats_daily AS
SELECT
    toDate(hour) as date,
    user_id,
    sum(events) as events
FROM stats_hourly
GROUP BY date, user_id;
```

### CH-405: Populate существующих данных

```sql
-- При создании MV можно заполнить из существующих данных
CREATE MATERIALIZED VIEW mv_stats TO target_stats
POPULATE  -- Загрузит все существующие данные
AS SELECT ...
FROM source_table
...;
```

**Осторожно с POPULATE:**
- Блокирует новые вставки во время заполнения
- Для больших таблиц лучше заполнять отдельно:

```sql
-- Создаём MV без POPULATE
CREATE MATERIALIZED VIEW mv_stats TO target_stats
AS SELECT ...;

-- Заполняем вручную
INSERT INTO target_stats
SELECT ... FROM source_table;
```

---

## Refreshable Materialized Views (Обновляемые)

### CH-406: Доступность

Доступны с ClickHouse 23.12+ (стабильно с 24.x).

### CH-407: Синтаксис

```sql
CREATE MATERIALIZED VIEW mv_name
REFRESH EVERY 1 HOUR
ENGINE = MergeTree()
ORDER BY (...)
AS SELECT ...
```

### CH-408: Режимы обновления

#### EVERY — периодическое обновление

```sql
-- Каждый час
REFRESH EVERY 1 HOUR

-- Каждый день в полночь
REFRESH EVERY 1 DAY

-- Каждые 5 минут
REFRESH EVERY 5 MINUTE
```

#### AFTER — после завершения предыдущего

```sql
-- 30 минут после завершения предыдущего refresh
REFRESH AFTER 30 MINUTE
```

#### CH-409: Комбинированный

```sql
-- Каждый час, но не чаще чем через 10 минут после завершения
REFRESH EVERY 1 HOUR AFTER 10 MINUTE
```

### CH-410: Пример: полная пересчёт статистики

```sql
CREATE MATERIALIZED VIEW user_totals_mv
REFRESH EVERY 1 HOUR
ENGINE = ReplacingMergeTree()
ORDER BY user_id
AS SELECT
    user_id,
    count() as total_events,
    sum(amount) as total_amount,
    min(created_at) as first_event,
    max(created_at) as last_event
FROM events
GROUP BY user_id;
```

### CH-411: Пример: с агрегатами за период

```sql
CREATE MATERIALIZED VIEW monthly_stats_mv
REFRESH EVERY 1 DAY
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (year_month, user_id)
AS SELECT
    toYYYYMM(created_at) as year_month,
    user_id,
    count() as events,
    sum(amount) as total,
    uniqExact(session_id) as sessions,
    now() as updated_at
FROM events
WHERE created_at >= toStartOfMonth(now() - INTERVAL 2 MONTH)
GROUP BY year_month, user_id;
```

### CH-412: Управление Refreshable MV

```sql
-- Статус обновления
SELECT * FROM system.view_refreshes
WHERE view = 'user_totals_mv';

-- Принудительное обновление
SYSTEM REFRESH VIEW user_totals_mv;

-- Остановка обновлений
SYSTEM STOP VIEW user_totals_mv;

-- Возобновление
SYSTEM START VIEW user_totals_mv;

-- Отмена текущего обновления
SYSTEM CANCEL VIEW user_totals_mv;
```

### CH-413: Dependencies (зависимости)

```sql
-- MV зависит от другого MV
CREATE MATERIALIZED VIEW summary_mv
REFRESH EVERY 1 HOUR
DEPENDS ON detailed_mv  -- Будет обновляться только после detailed_mv
ENGINE = ...
AS SELECT ...;
```

### CH-414: Atomicity (атомарность)

```sql
-- APPEND — добавляет к существующим данным
REFRESH EVERY 1 HOUR APPEND
AS SELECT ...;

-- По умолчанию — REPLACE (атомарная замена)
REFRESH EVERY 1 HOUR
AS SELECT ...;
```

---

## Сравнение: Incremental vs Refreshable

| Характеристика | Incremental | Refreshable |
|----------------|-------------|-------------|
| Триггер | INSERT в source | По расписанию |
| Данные | Только новые | Полный пересчёт |
| Consistency | Eventually consistent | Snapshot на момент refresh |
| Нагрузка | Распределена по вставкам | Пиковая при refresh |
| Исторические данные | Не видит | Полный доступ |
| Сложные JOIN | Ограничено | Полная поддержка |
| Удаление/обновление source | Не отражается | Отражается |

### Когда использовать Incremental

- ✅ Потоковая агрегация
- ✅ Append-only данные
- ✅ Высокая частота вставок
- ✅ Минимальная задержка (real-time)
- ✅ Простые агрегации (count, sum)

### Когда использовать Refreshable

- ✅ Сложные JOIN между таблицами
- ✅ Нужны точные исторические данные
- ✅ Source данные обновляются/удаляются
- ✅ Допустима задержка (hourly/daily)
- ✅ Полный пересчёт проще чем incremental

---

## Паттерны использования

### CH-415: Паттерн 1: Денормализация

```sql
-- Incremental MV для денормализации с dictGet
CREATE MATERIALIZED VIEW enriched_events TO events_enriched AS
SELECT
    event_id,
    user_id,
    dictGet('user_dict', 'name', user_id) as user_name,
    dictGet('user_dict', 'country', user_id) as country,
    amount,
    created_at
FROM events;
```

### CH-416: Паттерн 2: Pre-aggregation для дашбордов

```sql
-- Многоуровневая агрегация
-- Level 1: Raw → Minute
CREATE MATERIALIZED VIEW raw_to_minute TO stats_minute AS
SELECT
    toStartOfMinute(timestamp) as minute,
    metric_id,
    avg(value) as avg_value,
    count() as samples
FROM raw_metrics
GROUP BY minute, metric_id;

-- Level 2: Minute → Hour (Refreshable для точности)
CREATE MATERIALIZED VIEW minute_to_hour
REFRESH EVERY 5 MINUTE
ENGINE = ReplacingMergeTree()
ORDER BY (hour, metric_id)
AS SELECT
    toStartOfHour(minute) as hour,
    metric_id,
    avg(avg_value) as avg_value,
    sum(samples) as samples
FROM stats_minute
WHERE minute >= now() - INTERVAL 2 HOUR
GROUP BY hour, metric_id;
```

### CH-417: Паттерн 3: Window-based aggregation

```sql
-- Скользящее окно 7 дней (Refreshable)
CREATE MATERIALIZED VIEW rolling_7d_stats
REFRESH EVERY 1 HOUR
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY user_id
AS SELECT
    user_id,
    count() as events_7d,
    sum(amount) as total_7d,
    uniqExact(session_id) as sessions_7d,
    now() as updated_at
FROM events
WHERE created_at >= now() - INTERVAL 7 DAY
GROUP BY user_id;
```

### CH-418: Паттерн 4: Data transformation pipeline

```sql
-- Kafka → Raw → Cleaned → Aggregated

-- 1. Kafka source
CREATE TABLE kafka_events ENGINE = Kafka(...);

-- 2. Raw storage
CREATE TABLE raw_events ENGINE = MergeTree()...;

-- 3. MV: Kafka → Raw
CREATE MATERIALIZED VIEW kafka_to_raw TO raw_events AS
SELECT * FROM kafka_events;

-- 4. Cleaned storage
CREATE TABLE clean_events ENGINE = MergeTree()...;

-- 5. MV: Raw → Clean (с фильтрацией и валидацией)
CREATE MATERIALIZED VIEW raw_to_clean TO clean_events AS
SELECT *
FROM raw_events
WHERE isValidUUID(user_id)
  AND amount >= 0
  AND event_type IN ('click', 'view', 'purchase');

-- 6. Aggregated (Refreshable для полноты)
CREATE MATERIALIZED VIEW daily_aggregates
REFRESH EVERY 1 HOUR
ENGINE = ReplacingMergeTree()
ORDER BY (date, user_id)
AS SELECT
    toDate(created_at) as date,
    user_id,
    countIf(event_type = 'click') as clicks,
    countIf(event_type = 'view') as views,
    countIf(event_type = 'purchase') as purchases,
    sumIf(amount, event_type = 'purchase') as revenue
FROM clean_events
GROUP BY date, user_id;
```

---

## Best Practices

### CH-419: 1. Выбор target engine

```sql
-- Для sum/count — SummingMergeTree
CREATE TABLE target ENGINE = SummingMergeTree((col1, col2))...

-- Для uniq/quantile — AggregatingMergeTree
CREATE TABLE target ENGINE = AggregatingMergeTree()...

-- Для last state — ReplacingMergeTree
CREATE TABLE target ENGINE = ReplacingMergeTree(version)...
```

### CH-420: 2. ORDER BY в target = GROUP BY в MV

```sql
-- GROUP BY в MV
GROUP BY date, user_id, category

-- ORDER BY в target таблице
ORDER BY (date, user_id, category)
```

### CH-421: 3. Обработка ошибок

```sql
-- MV не должны падать на bad data
SELECT
    toDate(created_at) as date,
    user_id,
    -- Защита от NULL
    coalesce(amount, 0) as amount,
    -- Защита от невалидных значений
    if(amount < 0, 0, amount) as safe_amount
FROM events;
```

### CH-422: 4. Мониторинг

```sql
-- Статус Refreshable MV
SELECT
    database,
    view,
    status,
    last_refresh_time,
    last_refresh_result,
    next_refresh_time
FROM system.view_refreshes;

-- Размер MV
SELECT
    table,
    formatReadableSize(sum(bytes_on_disk)) as size,
    sum(rows) as rows
FROM system.parts
WHERE table LIKE '%mv%' OR table LIKE '.inner%'
GROUP BY table;
```

### CH-423: 5. Версионирование схемы

```sql
-- При изменении схемы MV:
-- 1. Создайте новую версию
CREATE MATERIALIZED VIEW mv_stats_v2 TO stats_v2 AS ...;

-- 2. Переключите приложение
-- 3. Удалите старую версию
DROP VIEW mv_stats_v1;
DROP TABLE stats_v1;
```
