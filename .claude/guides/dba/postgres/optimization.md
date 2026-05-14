# PostgreSQL Query Optimization

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| PG-006 | Use prepared statements for repeated queries | [Query Planning](#query-planning) |
| PG-008 | Use context with timeout for all queries | [Query Timeouts](#query-timeouts) |

---

## EXPLAIN ANALYZE

### Basic usage
```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT * FROM orders WHERE user_id = $1;
```

### Full options
```sql
EXPLAIN (
    ANALYZE,      -- Actually run the query
    BUFFERS,      -- Show buffer usage
    COSTS,        -- Show estimated costs
    TIMING,       -- Show actual timing
    VERBOSE,      -- Show additional info
    FORMAT TEXT   -- TEXT, JSON, YAML, XML
) SELECT ...;
```

### Reading EXPLAIN output

```
Seq Scan on orders  (cost=0.00..1234.00 rows=100 width=50) (actual time=0.1..10.5 rows=95 loops=1)
  Filter: (status = 'pending')
  Rows Removed by Filter: 9905
  Buffers: shared hit=100 read=50
Planning Time: 0.1 ms
Execution Time: 10.6 ms
```

Key metrics:
- `cost`: estimated startup..total cost
- `rows`: estimated row count
- `actual time`: real startup..total time in ms
- `loops`: number of iterations
- `Buffers: shared hit/read`: cache hits vs disk reads

### Warning signs

| Pattern | Problem | Solution |
|---------|---------|----------|
| `Seq Scan` on large table | Missing index | Add index |
| `actual rows` >> `rows` | Stale statistics | `ANALYZE table` |
| `Nested Loop` with large outer | Inefficient join | Consider Hash Join |
| High `read` vs `hit` | Cold cache / too much data | Optimize query, add index |
| `Sort` with `external` | Memory pressure | Increase work_mem |

## Index Strategies

### PG-200: Use B-tree Indexes for Standard Queries
```sql
-- Equality, range, sorting
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_created ON orders(created_at DESC);

-- Composite (order matters!)
CREATE INDEX idx_orders_user_status ON orders(user_id, status);
-- Efficient for: WHERE user_id = ? AND status = ?
-- Efficient for: WHERE user_id = ?
-- NOT efficient for: WHERE status = ?
```

### PG-201: Use Partial Indexes for Selective Data
```sql
-- Index only active records
CREATE INDEX idx_orders_pending ON orders(created_at)
WHERE status = 'pending';

-- Much smaller than full index
```

### PG-202: Use Covering Indexes with INCLUDE for Index-Only Scans
```sql
-- Include non-key columns to avoid table lookup
CREATE INDEX idx_orders_user_covering ON orders(user_id)
INCLUDE (status, total, created_at);

-- Index-only scan possible for:
SELECT status, total, created_at FROM orders WHERE user_id = $1;
```

### PG-203: Create Expression Indexes for Function Calls
```sql
-- Index on expression
CREATE INDEX idx_users_email_lower ON users(LOWER(email));

-- Query must match exactly
SELECT * FROM users WHERE LOWER(email) = $1;
```

### PG-204: Use GIN Indexes for JSONB and Arrays
```sql
-- JSONB
CREATE INDEX idx_items_data ON items USING GIN(data);
CREATE INDEX idx_items_data_path ON items USING GIN(data jsonb_path_ops);

-- Array
CREATE INDEX idx_posts_tags ON posts USING GIN(tags);

-- Full-text
CREATE INDEX idx_articles_tsv ON articles USING GIN(tsv);
```

### PG-205: Use GiST Indexes for Geometric and Range Types
```sql
CREATE INDEX idx_events_during ON events USING GIST(during);
```

### PG-206: Use BRIN Indexes for Time-Series Data
```sql
-- Very compact, good for time-series with natural ordering
CREATE INDEX idx_logs_created ON logs USING BRIN(created_at);
```

## Query Optimization Techniques

### PG-207: Avoid SELECT * and Specify Columns
```sql
-- Bad: reads all columns from disk
SELECT * FROM users WHERE id = $1;

-- Good: reads only needed columns
SELECT id, name, email FROM users WHERE id = $1;
```

### PG-208: Use EXISTS Instead of IN for Subqueries
```sql
-- Slower for large subquery results
SELECT * FROM users WHERE id IN (SELECT user_id FROM orders);

-- Faster: stops at first match
SELECT * FROM users u WHERE EXISTS (
    SELECT 1 FROM orders o WHERE o.user_id = u.id
);
```

### PG-209: Avoid OR on Different Columns, Use UNION
```sql
-- Cannot use single index efficiently
SELECT * FROM orders WHERE user_id = $1 OR status = 'pending';

-- Better: UNION
SELECT * FROM orders WHERE user_id = $1
UNION
SELECT * FROM orders WHERE status = 'pending' AND user_id != $1;
```

### PG-210: Use UNION ALL Instead of UNION
```sql
-- UNION removes duplicates (slow)
SELECT id FROM table1 UNION SELECT id FROM table2;

-- UNION ALL keeps all rows (fast)
SELECT id FROM table1 UNION ALL SELECT id FROM table2;
```

### PG-211: Optimize LIKE Queries with Trigram Indexes
```sql
-- Cannot use B-tree index
SELECT * FROM users WHERE name LIKE '%john%';

-- Can use B-tree index (prefix search)
SELECT * FROM users WHERE name LIKE 'john%';

-- For substring search, use trigram index
CREATE EXTENSION pg_trgm;
CREATE INDEX idx_users_name_trgm ON users USING GIN(name gin_trgm_ops);
SELECT * FROM users WHERE name LIKE '%john%';
```

### PG-212: Batch INSERT Operations for Performance
```sql
-- Bad: N separate queries
INSERT INTO items (name) VALUES ($1);
INSERT INTO items (name) VALUES ($2);
...

-- Good: single query
INSERT INTO items (name) VALUES ($1), ($2), ($3), ...;

-- Best for large batches: COPY
COPY items (name) FROM STDIN;
```

### PG-213: Use NOT MATERIALIZED and MATERIALIZED CTEs Wisely
```sql
-- PostgreSQL < 12: CTE always materializes
-- PostgreSQL >= 12: optimizer decides

-- Force inline (no materialization)
WITH cte AS NOT MATERIALIZED (
    SELECT * FROM large_table WHERE condition
)
SELECT * FROM cte WHERE another_condition;

-- Force materialization
WITH cte AS MATERIALIZED (
    SELECT expensive_function(id) as result FROM table
)
SELECT * FROM cte WHERE result > 100;
```

## Statistics and Maintenance

### PG-214: Run ANALYZE to Update Query Statistics
```sql
-- Single table
ANALYZE orders;

-- All tables
ANALYZE;

-- Verbose
ANALYZE VERBOSE orders;
```

### PG-215: Monitor Table Bloat with Size Queries
```sql
SELECT
    schemaname, tablename,
    pg_size_pretty(pg_total_relation_size(schemaname || '.' || tablename)) as total_size,
    pg_size_pretty(pg_relation_size(schemaname || '.' || tablename)) as table_size,
    pg_size_pretty(pg_indexes_size(schemaname || '.' || tablename)) as index_size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname || '.' || tablename) DESC;
```

### PG-216: Use VACUUM and ANALYZE for Maintenance
```sql
-- Regular vacuum (reclaims space, updates visibility map)
VACUUM orders;

-- Full vacuum (rewrites table, exclusive lock)
VACUUM FULL orders;

-- Analyze after vacuum
VACUUM ANALYZE orders;
```

### PG-217: Rebuild Indexes with REINDEX
```sql
-- Single index
REINDEX INDEX idx_orders_user_id;

-- All indexes on table
REINDEX TABLE orders;

-- Concurrent reindex (no lock, PostgreSQL 12+)
REINDEX INDEX CONCURRENTLY idx_orders_user_id;
```

## Configuration Tuning

### PG-218: Configure Memory Settings for Performance
```sql
-- Show current settings
SHOW shared_buffers;     -- Shared memory for caching (25% of RAM)
SHOW work_mem;           -- Per-operation memory for sorts, hashes
SHOW maintenance_work_mem; -- For VACUUM, CREATE INDEX
SHOW effective_cache_size; -- Planner's estimate of OS cache

-- Set for session
SET work_mem = '256MB';
```

### PG-219: Tune Query Planner Settings
```sql
-- Force index scan (for testing)
SET enable_seqscan = off;

-- Adjust random page cost (lower for SSD)
SET random_page_cost = 1.1;  -- Default 4.0, use 1.1-1.5 for SSD
```

## Common Anti-patterns

### PG-220: Avoid N+1 Query Problem with JOINs
```sql
-- Bad: N+1 queries
FOR user IN SELECT * FROM users LOOP
    SELECT * FROM orders WHERE user_id = user.id;
END LOOP;

-- Good: single query with JOIN
SELECT u.*, o.*
FROM users u
LEFT JOIN orders o ON o.user_id = u.id;
```

### PG-221: Avoid Large OFFSET, Use Keyset Pagination
```sql
-- Bad: scans all rows up to offset
SELECT * FROM orders ORDER BY id LIMIT 10 OFFSET 100000;

-- Good: keyset pagination
SELECT * FROM orders WHERE id > $last_seen_id ORDER BY id LIMIT 10;
```

### PG-222: Use Correct Type Casting in WHERE Clauses
```sql
-- Bad: prevents index usage
SELECT * FROM users WHERE id = '123';  -- id is integer

-- Good: correct type
SELECT * FROM users WHERE id = 123;
```

### PG-223: Avoid Function Calls on Indexed Columns
```sql
-- Bad: cannot use index on created_at
SELECT * FROM orders WHERE DATE(created_at) = '2024-01-01';

-- Good: range query uses index
SELECT * FROM orders
WHERE created_at >= '2024-01-01' AND created_at < '2024-01-02';
```

### PG-224: Use NOT EXISTS Instead of NOT IN
```sql
-- Dangerous: returns no rows if subquery contains NULL
SELECT * FROM users WHERE id NOT IN (SELECT user_id FROM banned);

-- Safe: handles NULLs correctly
SELECT * FROM users u
WHERE NOT EXISTS (SELECT 1 FROM banned b WHERE b.user_id = u.id);
```

## Performance Monitoring Queries

### PG-225: Use pg_stat_statements to Find Slow Queries
```sql
SELECT
    query,
    calls,
    total_exec_time / 1000 as total_seconds,
    mean_exec_time / 1000 as mean_seconds,
    rows
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 20;
```

### PG-226: Monitor Index Usage Statistics
```sql
SELECT
    schemaname, tablename, indexname,
    idx_scan as scans,
    idx_tup_read as tuples_read,
    idx_tup_fetch as tuples_fetched
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;
```

### PG-227: Find and Remove Unused Indexes
```sql
SELECT
    schemaname, tablename, indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as size
FROM pg_stat_user_indexes
WHERE idx_scan = 0
AND schemaname = 'public';
```

### PG-228: Monitor Table Access Patterns
```sql
SELECT
    schemaname, relname,
    seq_scan, seq_tup_read,
    idx_scan, idx_tup_fetch,
    n_tup_ins, n_tup_upd, n_tup_del
FROM pg_stat_user_tables
ORDER BY seq_scan DESC;
```

### PG-229: Monitor Locks and Blocking
```sql
SELECT
    blocked_locks.pid AS blocked_pid,
    blocked_activity.usename AS blocked_user,
    blocking_locks.pid AS blocking_pid,
    blocking_activity.usename AS blocking_user,
    blocked_activity.query AS blocked_statement,
    blocking_activity.query AS blocking_statement
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks
    ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;
```
