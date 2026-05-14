# ClickHouse Query Optimization

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| CH-004 | Use PREWHERE for filtering before reading | [PREWHERE](#prewhere) |
| CH-005 | Avoid JOINs, denormalize data | [JOIN Optimization](#join-optimization) |
| CH-007 | Set max_execution_time for queries | [Query Settings](#query-settings) |
| CHD-003 | Set max_memory_usage per query | [Memory Settings](#memory-settings) |

---

## Query Analysis

### CH-300: EXPLAIN
```sql
-- Query plan
EXPLAIN SELECT * FROM events WHERE user_id = 123;

-- With pipeline
EXPLAIN PIPELINE SELECT * FROM events WHERE user_id = 123;

-- With AST
EXPLAIN AST SELECT * FROM events WHERE user_id = 123;

-- Estimated rows
EXPLAIN ESTIMATE SELECT * FROM events WHERE date = today();
```

### CH-301: System tables for analysis
```sql
-- Query log
SELECT
    query,
    read_rows,
    read_bytes,
    memory_usage,
    query_duration_ms
FROM system.query_log
WHERE type = 'QueryFinish'
ORDER BY event_time DESC
LIMIT 20;

-- Parts info
SELECT
    table,
    partition,
    count() as parts,
    sum(rows) as total_rows,
    formatReadableSize(sum(bytes_on_disk)) as size
FROM system.parts
WHERE active
GROUP BY table, partition;
```

## Primary Key Optimization

### CH-302: ORDER BY design
```sql
-- Good: queries filter by user_id first
CREATE TABLE events (...)
ORDER BY (user_id, event_type, created_at);

-- Efficient queries:
SELECT * FROM events WHERE user_id = 123;
SELECT * FROM events WHERE user_id = 123 AND event_type = 'click';

-- Inefficient (skips user_id):
SELECT * FROM events WHERE event_type = 'click';
```

### CH-303: Composite key rules
1. Put low-cardinality columns first (for better compression)
2. Put most-filtered columns in order of filter frequency
3. Date/time columns often go last in ORDER BY

```sql
-- Good for queries filtering by country, then city
ORDER BY (country, city, user_id, timestamp)

-- country has ~200 values
-- city has ~10000 values
-- user_id has millions
```

## Index Types

### CH-304: Skip indices
```sql
-- minmax index (for ranges)
CREATE TABLE events (
    id UUID,
    user_id UInt64,
    amount Decimal(18, 2),
    created_at DateTime,
    INDEX idx_amount minmax(amount) GRANULARITY 4
) ENGINE = MergeTree()
ORDER BY (user_id, created_at);

-- set index (for IN queries)
ALTER TABLE events
ADD INDEX idx_event_type set(event_type) GRANULARITY 4;

-- bloom_filter (for equality)
ALTER TABLE events
ADD INDEX idx_session bloom_filter() GRANULARITY 4;

-- ngrambf_v1 (for LIKE queries)
ALTER TABLE logs
ADD INDEX idx_message ngrambf_v1(4, 256, 2, 0) GRANULARITY 4;
```

### CH-305: Projection
```sql
-- Pre-aggregated data
ALTER TABLE events
ADD PROJECTION events_by_type (
    SELECT
        event_type,
        toDate(created_at) as date,
        count() as events
    GROUP BY event_type, date
);

-- Materialize projection for existing data
ALTER TABLE events MATERIALIZE PROJECTION events_by_type;
```

## PREWHERE Optimization

### CH-306: Use PREWHERE for filtering
```sql
-- Automatic PREWHERE (ClickHouse optimizes)
SELECT * FROM events WHERE user_id = 123 AND status = 'active';

-- Manual PREWHERE
SELECT * FROM events
PREWHERE user_id = 123  -- Read first, filter early
WHERE status = 'active' AND created_at > '2024-01-01';
```

### CH-307: PREWHERE vs WHERE
- PREWHERE: reads column first, filters, then reads remaining columns
- WHERE: reads all columns, then filters
- Put selective filters in PREWHERE
- ClickHouse usually auto-optimizes

## Partitioning Strategy

### CH-308: Partition design
```sql
-- Monthly partitions (common)
PARTITION BY toYYYYMM(date)

-- Daily partitions (high volume)
PARTITION BY toYYYYMMDD(date)

-- Custom partitions
PARTITION BY (toYYYYMM(date), region)
```

### CH-309: Partition pruning
```sql
-- Efficient: uses partition
SELECT * FROM events WHERE date = '2024-01-15';
SELECT * FROM events WHERE date >= '2024-01-01' AND date < '2024-02-01';

-- Inefficient: scans all partitions
SELECT * FROM events WHERE toMonth(date) = 1;
```

## Memory and Resource Settings

### CH-310: Query-level settings
```sql
-- Memory limit
SELECT * FROM large_table
SETTINGS max_memory_usage = 10000000000;  -- 10GB

-- Thread limit
SELECT * FROM large_table
SETTINGS max_threads = 4;

-- Read rows limit
SELECT * FROM large_table
SETTINGS max_rows_to_read = 1000000;
```

### CH-311: Distributed query settings
```sql
-- Parallel replicas
SELECT * FROM events_distributed
SETTINGS max_parallel_replicas = 3;

-- Timeout
SELECT * FROM events
SETTINGS max_execution_time = 60;
```

## Aggregation Optimization

### CH-312: Use AggregateFunction types
```sql
-- Pre-aggregate with AggregatingMergeTree
CREATE TABLE user_stats_agg (
    user_id UInt64,
    total_events AggregateFunction(count, UInt64),
    unique_sessions AggregateFunction(uniq, String)
) ENGINE = AggregatingMergeTree()
ORDER BY user_id;

-- Insert with state functions
INSERT INTO user_stats_agg
SELECT
    user_id,
    countState(),
    uniqState(session_id)
FROM events
GROUP BY user_id;

-- Query with merge functions
SELECT
    user_id,
    countMerge(total_events) as events,
    uniqMerge(unique_sessions) as sessions
FROM user_stats_agg
GROUP BY user_id;
```

### CH-313: Approximate functions
```sql
-- Use approximate when exact isn't needed
SELECT
    uniq(user_id),           -- Fast, ~2% app_error
    uniqExact(user_id),      -- Slow, exact
    uniqCombined(user_id),   -- Good balance
    approx_top_k(10)(event_type)  -- Approximate top K
FROM events;
```

## JOIN Optimization

### CH-314: Dictionary JOINs
```sql
-- Create dictionary (in-memory, fast lookups)
CREATE DICTIONARY user_dict (
    id UInt64,
    name String,
    country String
)
PRIMARY KEY id
SOURCE(CLICKHOUSE(TABLE users))
LAYOUT(HASHED())
LIFETIME(3600);

-- Fast lookup
SELECT
    event_id,
    dictGet('user_dict', 'name', user_id) as user_name
FROM events;
```

### CH-315: JOIN algorithms
```sql
-- Hash join (default, loads right table to memory)
SET join_algorithm = 'hash';

-- Parallel hash join
SET join_algorithm = 'parallel_hash';

-- Partial merge join (for sorted data)
SET join_algorithm = 'partial_merge';

-- Full sorting merge join
SET join_algorithm = 'full_sorting_merge';
```

### CH-316: JOIN best practices
```sql
-- Put smaller table on the right
SELECT e.*, u.name
FROM events e  -- Large
LEFT JOIN users u ON e.user_id = u.id;  -- Small

-- Filter before JOIN
SELECT e.*, u.name
FROM (
    SELECT * FROM events WHERE date = today()
) e
LEFT JOIN users u ON e.user_id = u.id;

-- Use GLOBAL for distributed
SELECT e.*, u.name
FROM events_distributed e
GLOBAL LEFT JOIN users u ON e.user_id = u.id;
```

## Data Types Optimization

### CH-317: Choose efficient types
```sql
-- Use smallest type that fits
UInt8    -- 0 to 255
UInt16   -- 0 to 65535
UInt32   -- 0 to 4B
UInt64   -- 0 to 18×10^18

-- Use LowCardinality for strings with few unique values
LowCardinality(String)  -- For country, status, category

-- Use FixedString for fixed-length strings
FixedString(2)  -- Country codes

-- Use Enum for known values
Enum8('pending' = 1, 'active' = 2, 'done' = 3)
```

### CH-318: Nullable considerations
```sql
-- Nullable adds overhead
Nullable(String)  -- Extra byte per value

-- Prefer default values when possible
String DEFAULT ''
UInt64 DEFAULT 0
```

## Compression

### CH-319: Column codecs
```sql
CREATE TABLE events (
    id UUID CODEC(NONE),                    -- No compression for UUIDs
    user_id UInt64 CODEC(Delta, ZSTD),      -- Good for sorted integers
    amount Decimal(18,2) CODEC(Gorilla),    -- Good for floats
    status LowCardinality(String),          -- Dictionary compression
    payload String CODEC(ZSTD(3)),          -- Strong compression
    created_at DateTime CODEC(DoubleDelta)  -- Good for timestamps
) ENGINE = MergeTree()
ORDER BY (user_id, created_at);
```

### CH-320: Compression comparison
| Codec | Use case | Ratio | Speed |
|-------|----------|-------|-------|
| LZ4 | Default, balanced | Medium | Fast |
| ZSTD | Better compression | High | Medium |
| Delta | Monotonic integers | Varies | Fast |
| DoubleDelta | Timestamps, sequences | High | Fast |
| Gorilla | Float time series | High | Fast |

## Common Anti-patterns

### CH-321: Avoid SELECT *
```sql
-- Bad: reads all columns
SELECT * FROM events WHERE user_id = 123;

-- Good: read only needed columns
SELECT id, event_type, created_at FROM events WHERE user_id = 123;
```

### CH-322: Avoid functions on indexed columns
```sql
-- Bad: cannot use index
SELECT * FROM events WHERE toDate(created_at) = '2024-01-15';

-- Good: use pre-materialized column
SELECT * FROM events WHERE date = '2024-01-15';
```

### CH-323: Avoid too many small inserts
```sql
-- Bad: many small inserts
INSERT INTO events VALUES (...);  -- Single row, repeated

-- Good: batch inserts
INSERT INTO events VALUES (...), (...), (...);  -- Many rows

-- Best: use Buffer table for high-frequency inserts
CREATE TABLE events_buffer AS events
ENGINE = Buffer(default, events, 16, 10, 100, 10000, 1000000, 10000000, 100000000);
```

### CH-324: Avoid FINAL in production
```sql
-- Slow: deduplicates at query time
SELECT * FROM user_states FINAL WHERE user_id = 123;

-- Better: periodic OPTIMIZE
OPTIMIZE TABLE user_states FINAL;

-- Best: query without FINAL, handle duplicates in app
SELECT argMax(state, updated_at) as state
FROM user_states
WHERE user_id = 123;
```

## Monitoring Queries

### CH-325: Slow queries
```sql
SELECT
    query,
    query_duration_ms,
    read_rows,
    formatReadableSize(read_bytes) as read_size,
    formatReadableSize(memory_usage) as memory
FROM system.query_log
WHERE type = 'QueryFinish'
    AND query_duration_ms > 1000
ORDER BY query_duration_ms DESC
LIMIT 20;
```

### CH-326: Resource usage
```sql
SELECT
    user,
    sum(query_duration_ms) as total_time,
    sum(read_rows) as total_rows,
    formatReadableSize(sum(read_bytes)) as total_read
FROM system.query_log
WHERE type = 'QueryFinish'
    AND event_date = today()
GROUP BY user
ORDER BY total_time DESC;
```
