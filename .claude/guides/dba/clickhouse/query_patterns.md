# ClickHouse Query Patterns

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| CH-001 | Use batch inserts, never single-row INSERT | [INSERT](#insert) |
| CH-004 | Use PREWHERE for filtering before reading | [SELECT](#select) |
| CH-005 | Avoid JOINs, denormalize data | [JOIN Patterns](#join-patterns) |
| CH-007 | Set max_execution_time for queries | [Query Settings](#query-settings) |

---

## Basic Queries

### SELECT
```sql
-- Simple select
SELECT id, name, created_at
FROM events
WHERE date = today()
LIMIT 100;

-- With sampling
SELECT count()
FROM events SAMPLE 0.1  -- 10% sample
WHERE date >= '2024-01-01';
```

### INSERT
```sql
-- Single row (not recommended for production)
INSERT INTO events (id, name, created_at)
VALUES ('uuid', 'event_name', now());

-- Batch insert (recommended)
INSERT INTO events (id, name, created_at)
VALUES
    ('uuid1', 'event1', now()),
    ('uuid2', 'event2', now()),
    ('uuid3', 'event3', now());

-- Insert from SELECT
INSERT INTO events_archive
SELECT * FROM events
WHERE date < today() - 30;
```

### Format specifications
```sql
-- CSV input
INSERT INTO events FORMAT CSV
'uuid1','event1','2024-01-01'
'uuid2','event2','2024-01-02'

-- JSONEachRow (common for APIs)
INSERT INTO events FORMAT JSONEachRow
{"id": "uuid1", "name": "event1", "created_at": "2024-01-01"}
{"id": "uuid2", "name": "event2", "created_at": "2024-01-02"}
```

## Table Engines

### MergeTree family
```sql
-- Basic MergeTree
CREATE TABLE events (
    id UUID,
    user_id UInt64,
    event_type String,
    created_at DateTime,
    date Date MATERIALIZED toDate(created_at)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (user_id, created_at)
TTL date + INTERVAL 90 DAY;

-- ReplacingMergeTree (deduplication)
CREATE TABLE user_states (
    user_id UInt64,
    state String,
    updated_at DateTime
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY user_id;

-- SummingMergeTree (automatic aggregation)
CREATE TABLE daily_stats (
    date Date,
    user_id UInt64,
    clicks UInt64,
    views UInt64
) ENGINE = SummingMergeTree((clicks, views))
PARTITION BY toYYYYMM(date)
ORDER BY (date, user_id);

-- AggregatingMergeTree
CREATE TABLE user_stats (
    user_id UInt64,
    total_clicks AggregateFunction(sum, UInt64),
    unique_pages AggregateFunction(uniq, String)
) ENGINE = AggregatingMergeTree()
ORDER BY user_id;
```

### Distributed tables
```sql
-- Local table
CREATE TABLE events_local ON CLUSTER cluster_name (
    id UUID,
    event_type String,
    created_at DateTime
) ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/events', '{replica}')
ORDER BY (event_type, created_at);

-- Distributed table
CREATE TABLE events ON CLUSTER cluster_name AS events_local
ENGINE = Distributed(cluster_name, default, events_local, rand());
```

## Aggregations

### CH-100: Basic aggregations
```sql
SELECT
    toDate(created_at) as date,
    count() as total_events,
    uniq(user_id) as unique_users,
    sum(amount) as total_amount,
    avg(amount) as avg_amount
FROM events
WHERE created_at >= today() - 7
GROUP BY date
ORDER BY date;
```

### CH-101: Approximate aggregations (faster)
```sql
SELECT
    uniqExact(user_id) as exact_unique,     -- Exact, slower
    uniq(user_id) as approx_unique,         -- ~2% app_error, fast
    uniqCombined(user_id) as combined,      -- Good balance
    uniqHLL12(user_id) as hll               -- HyperLogLog
FROM events;
```

### CH-102: Quantiles
```sql
SELECT
    quantile(0.5)(response_time) as median,
    quantile(0.95)(response_time) as p95,
    quantile(0.99)(response_time) as p99,
    quantiles(0.5, 0.95, 0.99)(response_time) as percentiles
FROM requests
WHERE date = today();
```

### CH-103: Window functions
```sql
SELECT
    user_id,
    event_type,
    created_at,
    row_number() OVER (PARTITION BY user_id ORDER BY created_at) as event_number,
    sum(amount) OVER (PARTITION BY user_id ORDER BY created_at) as running_total
FROM events
WHERE date = today();
```

## Array Operations

### CH-104: Array functions
```sql
-- Create array
SELECT [1, 2, 3] as arr;
SELECT array(1, 2, 3) as arr;

-- Array contains
SELECT * FROM events WHERE has(tags, 'important');

-- Array aggregation
SELECT groupArray(user_id) as all_users FROM events;

-- Array join (unnest)
SELECT
    event_id,
    tag
FROM events
ARRAY JOIN tags as tag;

-- Filter with arrays
SELECT * FROM events WHERE hasAll(tags, ['a', 'b']);
SELECT * FROM events WHERE hasAny(tags, ['a', 'b']);
```

### CH-105: Array aggregation patterns
```sql
-- Aggregate into array
SELECT
    user_id,
    groupArray(event_type) as events,
    groupUniqArray(event_type) as unique_events
FROM events
GROUP BY user_id;

-- Top N per group
SELECT
    user_id,
    topK(5)(event_type) as top_events
FROM events
GROUP BY user_id;
```

## Time Series Queries

### CH-106: Time bucketing
```sql
SELECT
    toStartOfHour(created_at) as hour,
    count() as events
FROM events
WHERE date = today()
GROUP BY hour
ORDER BY hour;

-- Custom intervals
SELECT
    toStartOfInterval(created_at, INTERVAL 15 MINUTE) as bucket,
    count() as events
FROM events
GROUP BY bucket;
```

### CH-107: Fill missing time buckets
```sql
SELECT
    toStartOfHour(created_at) as hour,
    count() as events
FROM events
WHERE date = today()
GROUP BY hour
ORDER BY hour
WITH FILL
    FROM toStartOfDay(now())
    TO now()
    STEP INTERVAL 1 HOUR;
```

### CH-108: Moving averages
```sql
SELECT
    date,
    events,
    avg(events) OVER (ORDER BY date ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) as moving_avg_7d
FROM daily_stats;
```

## JOINs

### CH-109: Regular JOIN
```sql
SELECT
    e.event_id,
    e.user_id,
    u.name as user_name
FROM events e
LEFT JOIN users u ON e.user_id = u.id
WHERE e.date = today();
```

### CH-110: JOIN with dictionary
```sql
-- Create dictionary
CREATE DICTIONARY user_dict (
    id UInt64,
    name String
)
PRIMARY KEY id
SOURCE(CLICKHOUSE(TABLE 'users'))
LAYOUT(HASHED())
LIFETIME(3600);

-- Use dictionary
SELECT
    event_id,
    user_id,
    dictGet('user_dict', 'name', user_id) as user_name
FROM events;
```

### CH-111: JOIN optimization
```sql
-- Set right table as smaller
SET join_use_nulls = 1;
SET max_rows_in_join = 1000000;

-- Use ANY JOIN for deduplication
SELECT e.*, u.name
FROM events e
ANY LEFT JOIN users u ON e.user_id = u.id;
```

## Materialized Views

### CH-112: Basic materialized view
```sql
CREATE MATERIALIZED VIEW hourly_events
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (hour, event_type)
AS SELECT
    toStartOfHour(created_at) as hour,
    event_type,
    count() as events,
    uniq(user_id) as unique_users
FROM events
GROUP BY hour, event_type;
```

### CH-113: Chained materialized views
```sql
-- Raw events -> hourly aggregates -> daily aggregates
CREATE MATERIALIZED VIEW daily_events
ENGINE = SummingMergeTree()
ORDER BY (date, event_type)
AS SELECT
    toDate(hour) as date,
    event_type,
    sum(events) as events,
    uniq(unique_users) as unique_users
FROM hourly_events
GROUP BY date, event_type;
```

## Subqueries and CTEs

### CH-114: WITH clause
```sql
WITH
    active_users AS (
        SELECT DISTINCT user_id
        FROM events
        WHERE date >= today() - 7
    )
SELECT count() as active_user_count
FROM active_users;
```

### CH-115: Subquery in WHERE
```sql
SELECT *
FROM events
WHERE user_id IN (
    SELECT user_id
    FROM users
    WHERE country = 'US'
);
```

### CH-116: Global IN (for distributed)
```sql
SELECT *
FROM events_distributed
WHERE user_id GLOBAL IN (
    SELECT user_id FROM power_users
);
```

## String Operations

### CH-117: String functions
```sql
SELECT
    lower(name) as name_lower,
    upper(name) as name_upper,
    length(name) as name_length,
    substring(name, 1, 10) as name_short,
    concat(first_name, ' ', last_name) as full_name,
    splitByChar(',', tags) as tag_array
FROM users;
```

### CH-118: Pattern matching
```sql
-- LIKE
SELECT * FROM events WHERE name LIKE '%click%';

-- Regular expressions
SELECT * FROM events WHERE match(name, '^user_\d+$');

-- Extract with regex
SELECT
    extractAll(url, '([a-z]+)://') as protocols
FROM logs;
```

## Conditional Logic

### CH-119: CASE expressions
```sql
SELECT
    user_id,
    CASE
        WHEN total_spent > 10000 THEN 'vip'
        WHEN total_spent > 1000 THEN 'regular'
        ELSE 'new'
    END as segment
FROM user_stats;
```

### CH-120: Conditional aggregation
```sql
SELECT
    countIf(status = 'success') as success_count,
    countIf(status = 'app_error') as error_count,
    sumIf(amount, status = 'success') as success_amount
FROM transactions;
```

### CH-121: multiIf
```sql
SELECT
    multiIf(
        age < 18, 'minor',
        age < 65, 'adult',
        'senior'
    ) as age_group
FROM users;
```

## Useful Functions

### CH-122: Date/Time functions
```sql
SELECT
    now() as current_time,
    today() as current_date,
    yesterday() as yesterday,
    toStartOfMonth(today()) as month_start,
    toStartOfWeek(today()) as week_start,
    dateDiff('day', created_at, now()) as days_since_creation,
    addDays(today(), 7) as next_week
FROM events;
```

### CH-123: NULL handling
```sql
SELECT
    coalesce(nullable_col, 'default') as with_default,
    ifNull(nullable_col, 'default') as with_default2,
    nullIf(col, '') as null_if_empty,
    isNull(col) as is_null,
    isNotNull(col) as is_not_null
FROM table;
```

### CH-124: Type conversion
```sql
SELECT
    toUInt64(string_col) as as_uint,
    toString(int_col) as as_string,
    toDate(timestamp_col) as as_date,
    toDateTime(date_col) as as_datetime,
    toUUID(string_uuid) as as_uuid
FROM table;
```
