# ClickHouse Aggregation Tables

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| CH-006 | Use AggregatingMergeTree for real-time aggregates | [AggregatingMergeTree](#aggregatingmergetree) |
| CH-002 | Choose correct MergeTree engine for use case | [Engine Selection](#overview) |

---

## Overview

ClickHouse предоставляет специализированные движки для автоматической агрегации данных. Это ключевая техника для оптимизации аналитических запросов.

## Архитектура агрегации

```
Raw Events (MergeTree)
        │
        ▼
Materialized View (trigger)
        │
        ▼
Aggregation Table (SummingMergeTree / AggregatingMergeTree)
        │
        ▼
Query (с автоматической агрегацией)
```

---

## SummingMergeTree

### CH-500: Базовый пример

```sql
-- Таблица сырых событий
CREATE TABLE events (
    event_id UUID,
    user_id UInt64,
    event_type LowCardinality(String),
    amount Decimal(18, 2),
    created_at DateTime
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (user_id, event_type, created_at);

-- Агрегированная таблица
CREATE TABLE daily_stats (
    date Date,
    user_id UInt64,
    event_type LowCardinality(String),
    events_count UInt64,
    total_amount Decimal(18, 2)
) ENGINE = SummingMergeTree((events_count, total_amount))
PARTITION BY toYYYYMM(date)
ORDER BY (date, user_id, event_type);

-- Materialized View для автоматической агрегации
CREATE MATERIALIZED VIEW events_to_daily_stats TO daily_stats AS
SELECT
    toDate(created_at) as date,
    user_id,
    event_type,
    count() as events_count,
    sum(amount) as total_amount
FROM events
GROUP BY date, user_id, event_type;
```

### CH-501: Как это работает

1. **Вставка в events** → триггерит Materialized View
2. **MV агрегирует** данные по GROUP BY
3. **Вставка в daily_stats** — могут быть дубликаты по ORDER BY
4. **Background merge** — суммирует строки с одинаковым ORDER BY

### CH-502: Важные правила

```sql
-- ORDER BY колонки НЕ суммируются!
ORDER BY (date, user_id, event_type)  -- Ключ группировки

-- Суммируемые колонки указываются явно
SummingMergeTree((events_count, total_amount))

-- Или все числовые кроме ORDER BY (если не указаны)
SummingMergeTree()
```

### CH-503: Запросы к SummingMergeTree

```sql
-- Без гарантий (merge может не произойти)
SELECT date, user_id, events_count, total_amount
FROM daily_stats
WHERE date = '2024-01-15';

-- С гарантированной агрегацией
SELECT date, user_id, sum(events_count), sum(total_amount)
FROM daily_stats
WHERE date = '2024-01-15'
GROUP BY date, user_id;
```

### CH-504: Nested колонки

```sql
-- SummingMergeTree суммирует Nested колонки по первому элементу
CREATE TABLE product_stats (
    date Date,
    product_id UInt64,
    stats Nested(
        region String,
        sales UInt64,
        revenue Decimal(18, 2)
    )
) ENGINE = SummingMergeTree()
ORDER BY (date, product_id);

-- Данные будут суммироваться по (date, product_id, stats.region)
```

---

## AggregatingMergeTree

### CH-505: Когда использовать вместо SummingMergeTree

| Функция | SummingMergeTree | AggregatingMergeTree |
|---------|------------------|----------------------|
| sum() | ✅ | ✅ |
| count() | ✅ (как sum) | ✅ |
| min() / max() | ❌ | ✅ |
| avg() | ❌ | ✅ |
| uniq() / uniqExact() | ❌ | ✅ |
| quantile() | ❌ | ✅ |
| any() | ❌ | ✅ |

### CH-506: Базовый пример

```sql
-- Агрегированная таблица
CREATE TABLE user_stats_agg (
    date Date,
    user_id UInt64,
    total_events AggregateFunction(count, UInt64),
    total_amount AggregateFunction(sum, Decimal(18, 2)),
    unique_sessions AggregateFunction(uniq, String),
    avg_amount AggregateFunction(avg, Decimal(18, 2)),
    max_amount AggregateFunction(max, Decimal(18, 2)),
    p95_amount AggregateFunction(quantile(0.95), Decimal(18, 2))
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, user_id);

-- Materialized View с -State функциями
CREATE MATERIALIZED VIEW events_to_user_stats TO user_stats_agg AS
SELECT
    toDate(created_at) as date,
    user_id,
    countState() as total_events,
    sumState(amount) as total_amount,
    uniqState(session_id) as unique_sessions,
    avgState(amount) as avg_amount,
    maxState(amount) as max_amount,
    quantileState(0.95)(amount) as p95_amount
FROM events
GROUP BY date, user_id;
```

### CH-507: Чтение данных

```sql
-- Используем -Merge функции
SELECT
    date,
    user_id,
    countMerge(total_events) as events,
    sumMerge(total_amount) as total,
    uniqMerge(unique_sessions) as sessions,
    avgMerge(avg_amount) as avg,
    maxMerge(max_amount) as max,
    quantileMerge(0.95)(p95_amount) as p95
FROM user_stats_agg
WHERE date >= '2024-01-01'
GROUP BY date, user_id;
```

### CH-508: Комбинирование состояний

```sql
-- Агрегация по неделям из дневных данных
SELECT
    toStartOfWeek(date) as week,
    user_id,
    countMerge(total_events) as events,
    uniqMerge(unique_sessions) as sessions
FROM user_stats_agg
WHERE date >= '2024-01-01'
GROUP BY week, user_id;

-- Агрегация по всем пользователям
SELECT
    date,
    countMerge(total_events) as total_events,
    uniqMerge(unique_sessions) as total_sessions
FROM user_stats_agg
WHERE date = '2024-01-15'
GROUP BY date;
```

---

## SimpleAggregateFunction

### CH-509: Облегчённая версия AggregateFunction для простых агрегатов

### CH-510: Поддерживаемые функции

- `any`, `anyLast`
- `min`, `max`
- `sum`
- `sumWithOverflow`
- `groupBitAnd`, `groupBitOr`, `groupBitXor`
- `groupArrayArray`, `groupUniqArrayArray`

### CH-511: Пример

```sql
CREATE TABLE user_first_last (
    user_id UInt64,
    first_event SimpleAggregateFunction(any, DateTime),
    last_event SimpleAggregateFunction(anyLast, DateTime),
    min_amount SimpleAggregateFunction(min, Decimal(18, 2)),
    max_amount SimpleAggregateFunction(max, Decimal(18, 2))
) ENGINE = AggregatingMergeTree()
ORDER BY user_id;

-- Вставка без -State
INSERT INTO user_first_last
SELECT
    user_id,
    min(created_at) as first_event,
    max(created_at) as last_event,
    min(amount) as min_amount,
    max(amount) as max_amount
FROM events
GROUP BY user_id;

-- Чтение без -Merge (но GROUP BY всё равно нужен для гарантий)
SELECT user_id, first_event, last_event, min_amount, max_amount
FROM user_first_last;
```

---

## Multi-Level Aggregation

### CH-512: Паттерн: минуты → часы → дни

```sql
-- Level 1: Минутные агрегаты
CREATE TABLE stats_minute (
    minute DateTime,
    metric_id UInt32,
    value_sum SimpleAggregateFunction(sum, Float64),
    value_count SimpleAggregateFunction(sum, UInt64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMMDD(minute)
ORDER BY (metric_id, minute)
TTL minute + INTERVAL 7 DAY;

-- Level 2: Часовые агрегаты
CREATE TABLE stats_hour (
    hour DateTime,
    metric_id UInt32,
    value_sum SimpleAggregateFunction(sum, Float64),
    value_count SimpleAggregateFunction(sum, UInt64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (metric_id, hour)
TTL hour + INTERVAL 90 DAY;

-- Level 3: Дневные агрегаты
CREATE TABLE stats_day (
    date Date,
    metric_id UInt32,
    value_sum SimpleAggregateFunction(sum, Float64),
    value_count SimpleAggregateFunction(sum, UInt64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (metric_id, date);

-- MV: raw → minute
CREATE MATERIALIZED VIEW raw_to_minute TO stats_minute AS
SELECT
    toStartOfMinute(timestamp) as minute,
    metric_id,
    sum(value) as value_sum,
    count() as value_count
FROM raw_metrics
GROUP BY minute, metric_id;

-- MV: minute → hour
CREATE MATERIALIZED VIEW minute_to_hour TO stats_hour AS
SELECT
    toStartOfHour(minute) as hour,
    metric_id,
    sum(value_sum) as value_sum,
    sum(value_count) as value_count
FROM stats_minute
GROUP BY hour, metric_id;

-- MV: hour → day
CREATE MATERIALIZED VIEW hour_to_day TO stats_day AS
SELECT
    toDate(hour) as date,
    metric_id,
    sum(value_sum) as value_sum,
    sum(value_count) as value_count
FROM stats_hour
GROUP BY date, metric_id;
```

### CH-513: Запрос с автоматическим выбором уровня

```sql
-- Запрос за последний час — используем минуты
SELECT minute, sum(value_sum) / sum(value_count) as avg_value
FROM stats_minute
WHERE minute >= now() - INTERVAL 1 HOUR
GROUP BY minute;

-- Запрос за неделю — используем часы
SELECT hour, sum(value_sum) / sum(value_count) as avg_value
FROM stats_hour
WHERE hour >= now() - INTERVAL 7 DAY
GROUP BY hour;

-- Запрос за год — используем дни
SELECT date, sum(value_sum) / sum(value_count) as avg_value
FROM stats_day
WHERE date >= today() - 365
GROUP BY date;
```

---

## Combinator Functions

### CH-514: -State и -Merge

```sql
-- -State сохраняет промежуточное состояние
sumState(amount)      → AggregateFunction(sum, ...)
uniqState(user_id)    → AggregateFunction(uniq, ...)
quantileState(0.95)(x) → AggregateFunction(quantile(0.95), ...)

-- -Merge комбинирует состояния
sumMerge(state_column)
uniqMerge(state_column)
quantileMerge(0.95)(state_column)
```

### CH-515: -If (условная агрегация)

```sql
SELECT
    date,
    countState() as total,
    countStateIf(status = 'success') as success,
    sumStateIf(amount, status = 'success') as success_amount
FROM events
GROUP BY date;
```

### CH-516: -Array (агрегация массивов)

```sql
SELECT
    user_id,
    groupArrayState(event_type) as event_types,
    groupUniqArrayState(tags) as unique_tags
FROM events
GROUP BY user_id;
```

---

## Best Practices

### CH-517: 1. Выбор ORDER BY

```sql
-- ORDER BY = ключ группировки
-- Должен содержать ВСЕ колонки из GROUP BY MV

CREATE MATERIALIZED VIEW mv TO target AS
SELECT
    date,           -- ┐
    user_id,        -- ├─ Должны быть в ORDER BY target таблицы
    category,       -- ┘
    sum(amount) as total
FROM source
GROUP BY date, user_id, category;

CREATE TABLE target (...)
ENGINE = SummingMergeTree()
ORDER BY (date, user_id, category);  -- Точно соответствует GROUP BY
```

### CH-518: 2. Использование FINAL vs GROUP BY

```sql
-- FINAL — медленный, использует merge в query time
SELECT * FROM stats FINAL WHERE date = '2024-01-15';

-- GROUP BY — быстрее, рекомендуется
SELECT date, user_id, sum(total) as total
FROM stats
WHERE date = '2024-01-15'
GROUP BY date, user_id;
```

### CH-519: 3. Оптимизация хранения

```sql
-- Используйте LowCardinality для строк с малым количеством уникальных значений
event_type LowCardinality(String)

-- Используйте Nullable только когда NULL действительно нужен
-- Nullable добавляет overhead

-- Используйте правильные типы (UInt32 vs UInt64, Decimal vs Float)
```

### CH-520: 4. TTL для автоочистки

```sql
CREATE TABLE stats (
    date Date,
    ...
) ENGINE = SummingMergeTree()
ORDER BY (date, ...)
TTL date + INTERVAL 90 DAY;  -- Автоудаление через 90 дней
```

### CH-521: 5. Партиционирование

```sql
-- По месяцам — стандартный выбор
PARTITION BY toYYYYMM(date)

-- По дням — для очень высокой нагрузки
PARTITION BY toYYYYMMDD(date)

-- Не перепартиционируйте — максимум несколько тысяч партиций
```
