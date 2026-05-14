# ClickHouse Table Engines

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| CH-002 | Choose correct MergeTree engine for use case | [MergeTree Family](#mergetree-family) |
| CH-003 | Partition by date/month for time-series | [Partitioning](#partitioning) |
| CH-006 | Use AggregatingMergeTree for real-time aggregates | [AggregatingMergeTree](#aggregatingmergetree) |

---

## Engine Categories

### MergeTree Family (Основные)
Самые используемые движки для OLAP-задач.

### Log Family
Для небольших таблиц (до ~1М строк), быстрая запись.

### Integration Engines
Для интеграции с внешними системами.

### Special Engines
Специальные случаи использования.

---

## MergeTree Family

### CH-200: MergeTree
Базовый движок для аналитических запросов.

```sql
CREATE TABLE events (
    id UUID,
    user_id UInt64,
    event_type LowCardinality(String),
    created_at DateTime,
    date Date MATERIALIZED toDate(created_at)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (user_id, created_at)
PRIMARY KEY user_id
TTL date + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;
```

**Когда использовать:**
- Основной выбор для аналитики
- Большие объёмы данных
- Нужны быстрые запросы по ORDER BY ключу

**Особенности:**
- Данные сортируются по ORDER BY
- Автоматическое слияние частей (parts)
- Поддержка TTL для auto-delete
- Поддержка skip-индексов

### CH-201: ReplacingMergeTree
Удаляет дубликаты по ORDER BY ключу при merge.

```sql
CREATE TABLE user_states (
    user_id UInt64,
    state String,
    updated_at DateTime
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY user_id;
```

**Когда использовать:**
- Дедупликация данных
- CDC (Change Data Capture) сценарии
- Последнее состояние объекта

**Особенности:**
- Дубликаты удаляются при merge (не сразу!)
- `updated_at` определяет какую версию оставить
- **ЗАПРЕЩЕНО:** `SELECT ... FINAL` — блокирует параллельное чтение, форсирует merge in-place
- **ОБЯЗАТЕЛЬНО:** `argMax(col, version_col)` + `GROUP BY` первичный ключ

```sql
-- ЗАПРЕЩЕНО: FINAL блокирует параллельное чтение
-- SELECT * FROM user_states FINAL WHERE user_id = 123;

-- ПРАВИЛЬНО: argMax по версионной колонке
SELECT user_id, argMax(state, updated_at) as state
FROM user_states
WHERE user_id = 123
GROUP BY user_id;

-- ПРАВИЛЬНО: выбрать все колонки через argMax
SELECT
    user_id,
    argMax(state, updated_at)    AS state,
    argMax(email, updated_at)    AS email,
    argMax(status, updated_at)   AS status
FROM user_states
GROUP BY user_id;
```

### CH-202: SummingMergeTree
Автоматически суммирует числовые колонки при merge.

```sql
CREATE TABLE daily_stats (
    date Date,
    user_id UInt64,
    clicks UInt64,
    views UInt64,
    revenue Decimal(18, 2)
) ENGINE = SummingMergeTree((clicks, views, revenue))
PARTITION BY toYYYYMM(date)
ORDER BY (date, user_id);
```

**Когда использовать:**
- Агрегированная статистика
- Счётчики
- Инкрементальные метрики

**Особенности:**
- Суммирует указанные колонки
- ORDER BY колонки не суммируются
- Строки с одинаковым ORDER BY объединяются

```sql
-- Вставка
INSERT INTO daily_stats VALUES
    ('2024-01-15', 123, 10, 100, 50.00),
    ('2024-01-15', 123, 5, 50, 25.00);

-- После merge будет одна строка:
-- ('2024-01-15', 123, 15, 150, 75.00)
```

### CH-203: AggregatingMergeTree
Хранит промежуточные состояния агрегатных функций.

```sql
CREATE TABLE user_stats_agg (
    user_id UInt64,
    total_events AggregateFunction(count, UInt64),
    unique_sessions AggregateFunction(uniq, String),
    total_revenue AggregateFunction(sum, Decimal(18, 2))
) ENGINE = AggregatingMergeTree()
ORDER BY user_id;

-- Вставка через -State функции
INSERT INTO user_stats_agg
SELECT
    user_id,
    countState(),
    uniqState(session_id),
    sumState(amount)
FROM events
GROUP BY user_id;

-- Чтение через -Merge функции
SELECT
    user_id,
    countMerge(total_events) as events,
    uniqMerge(unique_sessions) as sessions,
    sumMerge(total_revenue) as revenue
FROM user_stats_agg
GROUP BY user_id;
```

**Когда использовать:**
- Предагрегация данных
- Инкрементальные агрегаты (uniq, quantile, etc.)
- Материализованные представления с агрегацией

**Особенности:**
- Требует `-State` функции для вставки
- Требует `-Merge` функции для чтения
- Поддерживает сложные агрегаты (uniq, quantile)

### CH-204: CollapsingMergeTree
Схлопывает строки по sign колонке (+1/-1).

```sql
CREATE TABLE user_balances (
    user_id UInt64,
    balance Int64,
    sign Int8
) ENGINE = CollapsingMergeTree(sign)
ORDER BY user_id;

-- Вставка новой записи
INSERT INTO user_balances VALUES (123, 100, 1);

-- Обновление: отмена старой + новая
INSERT INTO user_balances VALUES
    (123, 100, -1),  -- Отмена
    (123, 150, 1);   -- Новое значение
```

**Когда использовать:**
- Часто обновляемые данные
- Отслеживание изменений
- CDC-сценарии

**Особенности:**
- `sign = 1` — вставка
- `sign = -1` — удаление/отмена
- При merge строки с противоположными sign схлопываются

### CH-205: VersionedCollapsingMergeTree
CollapsingMergeTree с версионированием.

```sql
CREATE TABLE user_data (
    user_id UInt64,
    data String,
    version UInt64,
    sign Int8
) ENGINE = VersionedCollapsingMergeTree(sign, version)
ORDER BY user_id;
```

**Когда использовать:**
- Когда порядок вставки не гарантирован
- Распределённые вставки
- CDC с out-of-order updates

---

## Log Family

### CH-206: Log
Самый простой движок, хранит данные последовательно.

```sql
CREATE TABLE tmp_data (
    id UInt64,
    value String
) ENGINE = Log;
```

**Когда использовать:**
- Временные таблицы
- Очень маленькие таблицы (< 1М строк)
- Быстрая запись без индексов

### CH-207: TinyLog
Ещё проще чем Log, каждая колонка в отдельном файле.

```sql
CREATE TABLE tiny_data (
    id UInt64,
    value String
) ENGINE = TinyLog;
```

**Когда использовать:**
- Самые маленькие таблицы
- Конфигурационные данные

### CH-208: StripeLog
Оптимизирован для параллельного чтения.

```sql
CREATE TABLE stripe_data (
    id UInt64,
    value String
) ENGINE = StripeLog;
```

**Когда использовать:**
- Средние таблицы
- Нужно параллельное чтение

---

## Integration Engines

### CH-209: Kafka
Чтение из Apache Kafka.

```sql
CREATE TABLE kafka_events (
    id UUID,
    event_type String,
    data String
) ENGINE = Kafka()
SETTINGS
    kafka_broker_list = 'kafka:9092',
    kafka_topic_list = 'events',
    kafka_group_name = 'clickhouse-consumer',
    kafka_format = 'JSONEachRow';
```

**Паттерн использования:**
```sql
-- Kafka source
CREATE TABLE kafka_events (...) ENGINE = Kafka(...);

-- Target table
CREATE TABLE events (...) ENGINE = MergeTree() ...;

-- Materialized view для переноса
CREATE MATERIALIZED VIEW kafka_to_events TO events AS
SELECT * FROM kafka_events;
```

### CH-210: MySQL
Чтение из MySQL.

```sql
CREATE TABLE mysql_users
ENGINE = MySQL('mysql_host:3306', 'database', 'users', 'user', 'password');
```

### CH-211: PostgreSQL
Чтение из PostgreSQL.

```sql
CREATE TABLE pg_orders
ENGINE = PostgreSQL('pg_host:5432', 'database', 'orders', 'user', 'password');
```

### CH-212: S3
Чтение/запись в S3.

```sql
CREATE TABLE s3_data (
    id UInt64,
    value String
) ENGINE = S3('https://bucket.s3.amazonaws.com/data/*.parquet', 'access_key', 'secret_key', 'Parquet');
```

---

## Special Engines

### CH-213: Memory
Данные только в памяти.

```sql
CREATE TABLE memory_cache (
    key String,
    value String
) ENGINE = Memory;
```

**Когда использовать:**
- Временные таблицы в запросах
- Кэширование маленьких lookup-таблиц
- Тестирование

**Особенности:**
- Данные теряются при перезапуске
- Нет компрессии
- Очень быстрое чтение/запись

### CH-214: Buffer
Буфер перед целевой таблицей.

```sql
CREATE TABLE events_buffer AS events
ENGINE = Buffer(
    currentDatabase(), events,
    16,           -- num_layers
    10, 100,      -- min/max_time (секунды)
    10000, 1000000,    -- min/max_rows
    10000000, 100000000 -- min/max_bytes
);
```

**Когда использовать:**
- Высокочастотные вставки
- Сглаживание нагрузки на целевую таблицу

**Особенности:**
- Накапливает данные перед flush
- Flush по времени или размеру
- Данные могут потеряться при crash

### CH-215: Dictionary
Хранение словарей в памяти.

```sql
CREATE DICTIONARY user_dict (
    id UInt64,
    name String,
    country String
)
PRIMARY KEY id
SOURCE(CLICKHOUSE(TABLE 'users'))
LAYOUT(HASHED())
LIFETIME(3600);

-- Использование
SELECT dictGet('user_dict', 'name', user_id) as user_name
FROM events;
```

**Когда использовать:**
- Замена JOIN на быстрый lookup
- Справочные данные
- Обогащение событий

**Layouts:**
- `HASHED` — хэш-таблица, быстрый key lookup
- `FLAT` — массив, самый быстрый для UInt64 ключей
- `COMPLEX_KEY_HASHED` — составной ключ
- `RANGE_HASHED` — ключ + диапазон

### CH-216: Null
Игнорирует все данные.

```sql
CREATE TABLE null_sink (
    id UInt64,
    value String
) ENGINE = Null;
```

**Когда использовать:**
- Тестирование производительности записи
- "Мусорная корзина" для materialized views

### CH-217: Merge
Виртуальная таблица над несколькими таблицами.

```sql
CREATE TABLE events_all
ENGINE = Merge(currentDatabase(), '^events_');

-- Читает из всех таблиц, начинающихся с 'events_'
SELECT * FROM events_all;
```

**Когда использовать:**
- Партиционирование по таблицам
- Объединение разнородных источников

---

## Выбор движка: Decision Tree

```
Нужен ли высокопроизводительный OLAP?
├── Да → MergeTree family
│   ├── Нужна дедупликация?
│   │   ├── Да → ReplacingMergeTree
│   │   └── Нет → Нужна автоматическая агрегация?
│   │       ├── Простые суммы → SummingMergeTree
│   │       ├── Сложные агрегаты → AggregatingMergeTree
│   │       └── Нет → MergeTree
│   ├── Нужен CDC / частые обновления?
│   │   ├── Гарантирован порядок → CollapsingMergeTree
│   │   └── Не гарантирован → VersionedCollapsingMergeTree
│   └── Нужна репликация?
│       └── Да → ReplicatedMergeTree
└── Нет
    ├── Маленькая таблица (< 1M строк)?
    │   └── Да → Log / TinyLog / Memory
    ├── Интеграция с внешней системой?
    │   └── Да → Kafka / MySQL / PostgreSQL / S3
    └── Буферизация записи?
        └── Да → Buffer
```
