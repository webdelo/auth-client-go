# MySQL Query Optimization

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MY-004 | Avoid SELECT * in production | [Query Patterns](#query-patterns) |
| MY-005 | Use covering indexes for frequent queries | [Index Optimization](#index-optimization) |
| MYD-001 | Set innodb_buffer_pool_size = 70% RAM | [Memory Configuration](#memory-configuration) |

---

## EXPLAIN Analysis

### Basic EXPLAIN
```sql
EXPLAIN SELECT * FROM orders WHERE user_id = 123;
```

### EXPLAIN FORMAT=JSON (more details)
```sql
EXPLAIN FORMAT=JSON SELECT * FROM orders WHERE user_id = 123;
```

### EXPLAIN ANALYZE (MySQL 8.0.18+)
```sql
EXPLAIN ANALYZE SELECT * FROM orders WHERE user_id = 123;
```

### Reading EXPLAIN output

| Column | Meaning |
|--------|---------|
| `type` | Join type (best to worst: system, const, eq_ref, ref, range, index, ALL) |
| `possible_keys` | Indexes that could be used |
| `key` | Index actually used |
| `key_len` | Length of index used |
| `rows` | Estimated rows to examine |
| `filtered` | % of rows filtered by condition |
| `Extra` | Additional information |

### Warning signs in Extra

| Extra | Problem | Solution |
|-------|---------|----------|
| `Using filesort` | Sorting not using index | Add index on ORDER BY columns |
| `Using temporary` | Temp table needed | Optimize query, add index |
| `Using where` | Filtering after fetch | Add index on WHERE columns |
| `Full scan on NULL key` | Subquery performance issue | Rewrite subquery |

## Index Strategies

### MY-201: B-Tree index (default)
```sql
-- Single column
CREATE INDEX idx_user_id ON orders(user_id);

-- Composite index (order matters!)
CREATE INDEX idx_user_status ON orders(user_id, status);
-- Good for: WHERE user_id = ?
-- Good for: WHERE user_id = ? AND status = ?
-- Bad for: WHERE status = ?

-- Covering index
CREATE INDEX idx_orders_covering ON orders(user_id, status, created_at, total);
-- For: SELECT status, total FROM orders WHERE user_id = ? AND created_at > ?
```

### MY-202: Index prefix (for strings)
```sql
-- Index first 10 characters
CREATE INDEX idx_email_prefix ON users(email(10));
```

### MY-203: Descending index (MySQL 8.0+)
```sql
CREATE INDEX idx_created_desc ON orders(created_at DESC);
```

### MY-204: Invisible index (testing without dropping)
```sql
ALTER TABLE orders ALTER INDEX idx_user_id INVISIBLE;
-- Test queries
ALTER TABLE orders ALTER INDEX idx_user_id VISIBLE;
```

### MY-205: Composite index guidelines
```sql
-- Put equality columns first, then range
-- Good: WHERE status = 'active' AND created_at > ?
CREATE INDEX idx_status_created ON orders(status, created_at);

-- Put high-cardinality columns first (usually)
-- Exception: if low-cardinality column is always in WHERE
```

## Query Optimization Techniques

### MY-206: Optimize JOINs
```sql
-- Ensure foreign keys are indexed
CREATE INDEX idx_orders_user_id ON orders(user_id);

-- Put smaller table first in STRAIGHT_JOIN
SELECT STRAIGHT_JOIN o.*, u.name
FROM users u  -- smaller
JOIN orders o ON o.user_id = u.id;  -- larger
```

### MY-207: Optimize subqueries
```sql
-- Bad: Correlated subquery (executes per row)
SELECT * FROM users u
WHERE (SELECT COUNT(*) FROM orders WHERE user_id = u.id) > 5;

-- Good: JOIN with derived table
SELECT u.* FROM users u
JOIN (
    SELECT user_id FROM orders GROUP BY user_id HAVING COUNT(*) > 5
) o ON o.user_id = u.id;
```

### MY-208: Use EXISTS instead of IN
```sql
-- Slower for large subquery results
SELECT * FROM users WHERE id IN (SELECT user_id FROM orders);

-- Faster: stops at first match
SELECT * FROM users u
WHERE EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id);
```

### MY-209: Avoid OR, use UNION
```sql
-- Bad: Can't use index efficiently
SELECT * FROM orders WHERE user_id = 1 OR status = 'pending';

-- Good: Each query can use index
SELECT * FROM orders WHERE user_id = 1
UNION
SELECT * FROM orders WHERE status = 'pending' AND user_id != 1;
```

### MY-210: Optimize LIKE
```sql
-- Can use index (prefix search)
SELECT * FROM users WHERE name LIKE 'John%';

-- Cannot use index
SELECT * FROM users WHERE name LIKE '%John%';

-- For suffix search, store reversed value
ALTER TABLE users ADD name_reversed VARCHAR(255) AS (REVERSE(name)) STORED;
CREATE INDEX idx_name_reversed ON users(name_reversed);
SELECT * FROM users WHERE name_reversed LIKE REVERSE('%son');
```

### MY-211: Batch operations
```sql
-- Bad: Many single inserts
INSERT INTO items VALUES (1, 'a');
INSERT INTO items VALUES (2, 'b');

-- Good: Batch insert
INSERT INTO items VALUES (1, 'a'), (2, 'b'), (3, 'c');

-- For large batches, use LOAD DATA
LOAD DATA INFILE '/path/to/data.csv' INTO TABLE items;
```

## Partitioning

### MY-212: Range partitioning
```sql
CREATE TABLE orders (
    id INT NOT NULL,
    user_id INT NOT NULL,
    total DECIMAL(10,2),
    created_at DATE NOT NULL
) PARTITION BY RANGE (YEAR(created_at)) (
    PARTITION p2022 VALUES LESS THAN (2023),
    PARTITION p2023 VALUES LESS THAN (2024),
    PARTITION p2024 VALUES LESS THAN (2025),
    PARTITION p_future VALUES LESS THAN MAXVALUE
);
```

### MY-213: List partitioning
```sql
CREATE TABLE orders (
    id INT NOT NULL,
    region VARCHAR(10) NOT NULL,
    total DECIMAL(10,2)
) PARTITION BY LIST COLUMNS(region) (
    PARTITION p_us VALUES IN ('US', 'CA'),
    PARTITION p_eu VALUES IN ('UK', 'DE', 'FR'),
    PARTITION p_asia VALUES IN ('JP', 'CN', 'KR')
);
```

### MY-214: Partition pruning
```sql
-- Efficient: uses partition
SELECT * FROM orders WHERE created_at >= '2024-01-01';

-- Inefficient: scans all partitions
SELECT * FROM orders WHERE MONTH(created_at) = 6;
```

## Configuration Tuning

### MY-215: Buffer pool
```sql
-- Check current size
SHOW VARIABLES LIKE 'innodb_buffer_pool_size';

-- Set to 70-80% of available RAM for dedicated DB server
SET GLOBAL innodb_buffer_pool_size = 8589934592;  -- 8GB
```

### MY-216: Query cache (MySQL 5.7, removed in 8.0)
```sql
-- Check status
SHOW VARIABLES LIKE 'query_cache%';
```

### MY-217: Sort and join buffers
```sql
-- Increase for complex queries
SET SESSION sort_buffer_size = 4194304;  -- 4MB
SET SESSION join_buffer_size = 4194304;  -- 4MB
```

### MY-218: Temporary tables
```sql
-- Check if temp tables go to disk
SHOW GLOBAL STATUS LIKE 'Created_tmp%';

-- Increase memory for temp tables
SET GLOBAL tmp_table_size = 268435456;  -- 256MB
SET GLOBAL max_heap_table_size = 268435456;
```

## Monitoring Queries

### MY-219: Slow query log
```sql
-- Enable slow query log
SET GLOBAL slow_query_log = 1;
SET GLOBAL long_query_time = 1;  -- Log queries > 1 second
SET GLOBAL log_queries_not_using_indexes = 1;
```

### MY-220: Performance schema
```sql
-- Top queries by total time
SELECT
    DIGEST_TEXT,
    COUNT_STAR as executions,
    SUM_TIMER_WAIT/1000000000000 as total_seconds,
    AVG_TIMER_WAIT/1000000000 as avg_ms
FROM performance_schema.events_statements_summary_by_digest
ORDER BY SUM_TIMER_WAIT DESC
LIMIT 10;
```

### MY-221: Index usage
```sql
-- Unused indexes
SELECT
    object_schema,
    object_name,
    index_name
FROM performance_schema.table_io_waits_summary_by_index_usage
WHERE index_name IS NOT NULL
    AND count_star = 0
    AND object_schema NOT IN ('mysql', 'performance_schema');
```

### MY-222: Table statistics
```sql
SELECT
    table_schema,
    table_name,
    table_rows,
    data_length / 1024 / 1024 as data_mb,
    index_length / 1024 / 1024 as index_mb
FROM information_schema.tables
WHERE table_schema = 'your_database'
ORDER BY data_length DESC;
```

### MY-223: Lock monitoring
```sql
-- Current locks
SELECT * FROM performance_schema.data_locks;

-- Lock waits
SELECT * FROM performance_schema.data_lock_waits;

-- InnoDB status (includes deadlock info)
SHOW ENGINE INNODB STATUS;
```

## Table Maintenance

### MY-224: Analyze table (update statistics)
```sql
ANALYZE TABLE orders;
```

### MY-225: Optimize table (reclaim space, rebuild indexes)
```sql
OPTIMIZE TABLE orders;
```

### MY-226: Check table
```sql
CHECK TABLE orders;
```

### MY-227: Repair table (MyISAM only)
```sql
REPAIR TABLE mytable;
```

## Common Anti-patterns

### MY-228: N+1 queries
```sql
-- Bad: Execute per user
FOR each user:
    SELECT * FROM orders WHERE user_id = ?;

-- Good: Single query
SELECT * FROM orders WHERE user_id IN (?, ?, ?);
```

### MY-229: Large OFFSET
```sql
-- Bad: Scans 10000 rows
SELECT * FROM orders ORDER BY id LIMIT 10 OFFSET 10000;

-- Good: Use keyset pagination
SELECT * FROM orders WHERE id > ? ORDER BY id LIMIT 10;
```

### MY-230: SELECT DISTINCT instead of EXISTS
```sql
-- Bad: Fetches and deduplicates all rows
SELECT DISTINCT u.* FROM users u JOIN orders o ON o.user_id = u.id;

-- Good: Stops at first match
SELECT u.* FROM users u WHERE EXISTS (
    SELECT 1 FROM orders o WHERE o.user_id = u.id
);
```

### MY-231: Counting with JOIN
```sql
-- Bad: Full join then count
SELECT COUNT(*) FROM orders o JOIN users u ON u.id = o.user_id WHERE u.status = 'active';

-- Good: Count with subquery
SELECT COUNT(*) FROM orders WHERE user_id IN (
    SELECT id FROM users WHERE status = 'active'
);
```

## InnoDB Specific

### MY-232: Primary key design
```sql
-- Good: Auto-increment (sequential inserts)
CREATE TABLE orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    ...
);

-- Avoid: UUID as primary key (random inserts, page splits)
-- If needed, use ordered UUID or store separately
```

### MY-233: Clustered index
```sql
-- InnoDB tables are clustered on PRIMARY KEY
-- Secondary indexes include PK columns
-- Keep PK small for better secondary index performance
```

### MY-234: Transaction isolation
```sql
-- Check current level
SELECT @@transaction_isolation;

-- Set for session
SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;

-- Options: READ UNCOMMITTED, READ COMMITTED, REPEATABLE READ, SERIALIZABLE
```
