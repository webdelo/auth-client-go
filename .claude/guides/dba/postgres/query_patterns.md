# PostgreSQL Query Patterns

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| PG-002 | Use ANY($1::type[]) instead of IN for arrays | [SELECT with conditions](#select-with-conditions) |
| PG-003 | Always use RETURNING for INSERT/UPDATE | [INSERT](#insert), [UPDATE](#update) |
| PG-005 | Batch operations with COPY or unnest() | [INSERT](#insert) |

---

## Basic CRUD

### PG-100: Write Efficient SELECT Queries with Conditions
```sql
-- Simple equality
SELECT id, name, email FROM users WHERE id = $1;

-- Multiple conditions
SELECT * FROM orders
WHERE status = $1 AND created_at > $2
ORDER BY created_at DESC
LIMIT $3;

-- IN clause (prefer ANY for Go)
SELECT * FROM users WHERE id = ANY($1::uuid[]);
```

### PG-101: Use INSERT with RETURNING and ON CONFLICT
```sql
-- Single row with RETURNING
INSERT INTO users (name, email, created_at)
VALUES ($1, $2, NOW())
RETURNING id, created_at;

-- Batch insert
INSERT INTO items (name, value)
VALUES
    ($1, $2),
    ($3, $4),
    ($5, $6)
ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value;

-- COPY for bulk (fastest)
COPY users (name, email) FROM STDIN WITH (FORMAT csv);
```

### PG-102: Write Efficient UPDATE Queries
```sql
-- Simple update
UPDATE users SET name = $2, updated_at = NOW()
WHERE id = $1;

-- Conditional update
UPDATE orders
SET status = $2, updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING id, status;

-- Batch update with VALUES
UPDATE items AS i
SET value = v.value
FROM (VALUES ($1::uuid, $2::int), ($3::uuid, $4::int)) AS v(id, value)
WHERE i.id = v.id;
```

### PG-103: Prefer Soft Deletes Over Hard Deletes
```sql
-- Soft delete (preferred)
UPDATE users SET deleted_at = NOW() WHERE id = $1;

-- Hard delete with RETURNING
DELETE FROM sessions WHERE user_id = $1 RETURNING id;
```

## JOIN Patterns

### PG-104: Use INNER JOIN for Required Relationships
```sql
SELECT o.id, o.total, u.name, u.email
FROM orders o
INNER JOIN users u ON u.id = o.user_id
WHERE o.created_at > $1;
```

### PG-105: Combine LEFT JOIN with Aggregation Functions
```sql
SELECT u.id, u.name, COUNT(o.id) as order_count, COALESCE(SUM(o.total), 0) as total_spent
FROM users u
LEFT JOIN orders o ON o.user_id = u.id
GROUP BY u.id, u.name;
```

### PG-106: Chain Multiple JOINs Efficiently
```sql
SELECT
    o.id as order_id,
    u.name as user_name,
    p.name as product_name,
    oi.quantity,
    oi.price
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN order_items oi ON oi.order_id = o.id
JOIN products p ON p.id = oi.product_id
WHERE o.id = $1;
```

### PG-107: Use Self JOIN for Hierarchical Data
```sql
-- Hierarchical data (employees with managers)
SELECT e.name as employee, m.name as manager
FROM employees e
LEFT JOIN employees m ON m.id = e.manager_id;
```

## Aggregation

### PG-108: Use GROUP BY with HAVING for Filtered Aggregations
```sql
SELECT user_id, COUNT(*) as order_count, SUM(total) as total_spent
FROM orders
WHERE created_at > $1
GROUP BY user_id
HAVING COUNT(*) > 5
ORDER BY total_spent DESC;
```

### PG-109: Use DISTINCT ON for Latest Record Per Group
```sql
-- Latest order per user
SELECT DISTINCT ON (user_id) *
FROM orders
ORDER BY user_id, created_at DESC;
```

### PG-110: Use FILTER Clause for Conditional Aggregates
```sql
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE status = 'completed') as completed,
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    SUM(total) FILTER (WHERE status = 'completed') as completed_revenue
FROM orders
WHERE created_at > $1;
```

## Window Functions

### PG-111: Use ROW_NUMBER for Pagination
```sql
SELECT * FROM (
    SELECT *, ROW_NUMBER() OVER (ORDER BY created_at DESC) as rn
    FROM orders
    WHERE user_id = $1
) sub
WHERE rn BETWEEN $2 AND $3;
```

### PG-112: Implement Running Totals with Window Functions
```sql
SELECT
    id,
    created_at,
    total,
    SUM(total) OVER (ORDER BY created_at) as running_total
FROM orders
WHERE user_id = $1;
```

### PG-113: Use RANK and DENSE_RANK for Ranking
```sql
SELECT
    user_id,
    total_spent,
    RANK() OVER (ORDER BY total_spent DESC) as rank,
    DENSE_RANK() OVER (ORDER BY total_spent DESC) as dense_rank
FROM user_stats;
```

### PG-114: Use LAG/LEAD for Access Previous/Next Rows
```sql
SELECT
    id,
    total,
    LAG(total) OVER (ORDER BY created_at) as prev_order_total,
    total - LAG(total) OVER (ORDER BY created_at) as diff_from_prev
FROM orders
WHERE user_id = $1;
```

### PG-115: Use PARTITION BY for Window Function Scope
```sql
SELECT
    user_id,
    order_id,
    total,
    SUM(total) OVER (PARTITION BY user_id) as user_total,
    AVG(total) OVER (PARTITION BY user_id) as user_avg
FROM orders;
```

## CTE (Common Table Expressions)

### PG-116: Use Simple CTEs for Query Readability
```sql
WITH active_users AS (
    SELECT DISTINCT user_id
    FROM orders
    WHERE created_at > NOW() - INTERVAL '30 days'
)
SELECT u.*
FROM users u
JOIN active_users au ON au.user_id = u.id;
```

### PG-117: Chain Multiple CTEs for Complex Logic
```sql
WITH
user_orders AS (
    SELECT user_id, COUNT(*) as order_count, SUM(total) as total_spent
    FROM orders
    GROUP BY user_id
),
user_segments AS (
    SELECT user_id,
        CASE
            WHEN total_spent > 10000 THEN 'vip'
            WHEN total_spent > 1000 THEN 'regular'
            ELSE 'new'
        END as segment
    FROM user_orders
)
SELECT u.*, us.segment
FROM users u
JOIN user_segments us ON us.user_id = u.id;
```

### PG-118: Use Recursive CTEs for Tree/Graph Traversal
```sql
-- Hierarchical categories
WITH RECURSIVE category_tree AS (
    -- Base case
    SELECT id, name, parent_id, 1 as level, ARRAY[id] as path
    FROM categories
    WHERE parent_id IS NULL

    UNION ALL

    -- Recursive case
    SELECT c.id, c.name, c.parent_id, ct.level + 1, ct.path || c.id
    FROM categories c
    JOIN category_tree ct ON ct.id = c.parent_id
)
SELECT * FROM category_tree ORDER BY path;
```

## JSONB Operations

### PG-119: Query and Extract JSONB Fields Efficiently
```sql
-- Get value
SELECT data->>'name' as name FROM items WHERE id = $1;

-- Query by JSONB field
SELECT * FROM items WHERE data->>'status' = 'active';

-- Query nested
SELECT * FROM items WHERE data->'metadata'->>'type' = 'premium';
```

### PG-120: Use JSONB Containment Operators for Queries
```sql
-- Contains
SELECT * FROM items WHERE data @> '{"status": "active"}';

-- Key exists
SELECT * FROM items WHERE data ? 'metadata';

-- Any key exists
SELECT * FROM items WHERE data ?| ARRAY['key1', 'key2'];
```

### PG-121: Modify JSONB with jsonb_set and Operators
```sql
-- Set field
UPDATE items SET data = jsonb_set(data, '{status}', '"inactive"')
WHERE id = $1;

-- Remove field
UPDATE items SET data = data - 'temp_field' WHERE id = $1;

-- Merge
UPDATE items SET data = data || '{"updated": true}'::jsonb WHERE id = $1;
```

### PG-122: Aggregate JSONB with jsonb_agg
```sql
SELECT jsonb_agg(jsonb_build_object(
    'id', id,
    'name', name,
    'total', total
)) as orders
FROM orders
WHERE user_id = $1;
```

## Full-Text Search

### PG-123: Implement Basic Full-Text Search
```sql
SELECT * FROM articles
WHERE to_tsvector('english', title || ' ' || content) @@ plainto_tsquery('english', $1);
```

### PG-124: Rank Full-Text Search Results
```sql
SELECT
    id,
    title,
    ts_rank(tsv, query) as rank
FROM articles, plainto_tsquery('english', $1) query
WHERE tsv @@ query
ORDER BY rank DESC
LIMIT 20;
```

### PG-125: Pre-compute tsvector for Indexed Full-Text Search
```sql
-- Create column
ALTER TABLE articles ADD COLUMN tsv tsvector;
UPDATE articles SET tsv = to_tsvector('english', title || ' ' || content);
CREATE INDEX idx_articles_tsv ON articles USING GIN(tsv);

-- Query
SELECT * FROM articles WHERE tsv @@ plainto_tsquery('english', $1);
```

## Pagination Patterns

### PG-126: Use Offset Pagination for Small Datasets
```sql
SELECT * FROM orders
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
```

### PG-127: Use Keyset Pagination for Large Datasets
```sql
-- First page
SELECT * FROM orders
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- Next pages
SELECT * FROM orders
WHERE (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $1;
```

### PG-128: Use Composite Keys for Advanced Pagination
```sql
SELECT * FROM orders
WHERE created_at < $1 OR (created_at = $1 AND id < $2)
ORDER BY created_at DESC, id DESC
LIMIT $3;
```

## Upsert Patterns

### PG-129: Use INSERT ... ON CONFLICT for Upserts
```sql
INSERT INTO user_stats (user_id, login_count, last_login)
VALUES ($1, 1, NOW())
ON CONFLICT (user_id) DO UPDATE SET
    login_count = user_stats.login_count + 1,
    last_login = NOW();
```

### PG-130: Implement Conditional Upsert Logic
```sql
INSERT INTO items (id, value, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (id) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = NOW()
WHERE items.updated_at < EXCLUDED.updated_at;
```

## Locking

### PG-131: Use SELECT FOR UPDATE for Row-Level Locking
```sql
SELECT * FROM accounts
WHERE id = $1
FOR UPDATE;
```

### PG-132: Use SKIP LOCKED for Job Queue Processing
```sql
SELECT * FROM jobs
WHERE status = 'pending'
ORDER BY created_at
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

### PG-133: Use Advisory Locks for Application-Level Synchronization
```sql
-- Session lock
SELECT pg_advisory_lock($1);
-- ... do work ...
SELECT pg_advisory_unlock($1);

-- Transaction lock (auto-released)
SELECT pg_advisory_xact_lock($1);
```
