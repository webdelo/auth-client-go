# SQLite Query Patterns

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| SQ-003 | Enable foreign keys explicitly | [Foreign Keys](#foreign-keys) |
| SQ-004 | Use FTS5 for full-text search | [Full-Text Search](#full-text-search) |
| SQ-005 | Single writer, multiple readers | [Concurrent Access](#concurrent-access) |

---

## Basic Operations

### SQ-100: SELECT
```sql
-- Simple select
SELECT id, name, email FROM users WHERE id = ?;

-- With conditions
SELECT * FROM orders
WHERE status = ? AND created_at > ?
ORDER BY created_at DESC
LIMIT ?;

-- DISTINCT
SELECT DISTINCT category FROM products;
```

### SQ-101: INSERT
```sql
-- Single row
INSERT INTO users (name, email, created_at)
VALUES (?, ?, datetime('now'));

-- Get last insert ID
SELECT last_insert_rowid();

-- Multiple rows
INSERT INTO items (name, value) VALUES
    (?, ?), (?, ?), (?, ?);

-- INSERT OR REPLACE (upsert)
INSERT OR REPLACE INTO settings (key, value)
VALUES (?, ?);

-- INSERT OR IGNORE
INSERT OR IGNORE INTO users (email, name)
VALUES (?, ?);

-- INSERT with ON CONFLICT (SQLite 3.24+)
INSERT INTO user_stats (user_id, login_count)
VALUES (?, 1)
ON CONFLICT(user_id) DO UPDATE SET
    login_count = login_count + 1;
```

### SQ-102: UPDATE
```sql
-- Simple update
UPDATE users SET name = ?, updated_at = datetime('now')
WHERE id = ?;

-- Update with subquery
UPDATE orders SET total = (
    SELECT SUM(price * quantity)
    FROM order_items
    WHERE order_id = orders.id
);

-- Update with RETURNING (SQLite 3.35+)
UPDATE users SET name = ? WHERE id = ?
RETURNING id, name, updated_at;
```

### SQ-103: DELETE
```sql
-- Simple delete
DELETE FROM users WHERE id = ?;

-- Delete with RETURNING
DELETE FROM sessions WHERE expires_at < datetime('now')
RETURNING id, user_id;

-- Delete with LIMIT (needs ORDER BY)
DELETE FROM logs WHERE id IN (
    SELECT id FROM logs
    ORDER BY created_at
    LIMIT 1000
);
```

## JOINs

### SQ-104: Basic JOINs
```sql
-- INNER JOIN
SELECT o.id, o.total, u.name
FROM orders o
INNER JOIN users u ON u.id = o.user_id
WHERE o.status = 'completed';

-- LEFT JOIN
SELECT u.id, u.name, COUNT(o.id) as order_count
FROM users u
LEFT JOIN orders o ON o.user_id = u.id
GROUP BY u.id;

-- CROSS JOIN
SELECT p.name, c.name
FROM products p
CROSS JOIN categories c;
```

### SQ-105: Multi-table JOIN
```sql
SELECT
    o.id as order_id,
    u.name as user_name,
    p.name as product_name,
    oi.quantity
FROM orders o
JOIN users u ON u.id = o.user_id
JOIN order_items oi ON oi.order_id = o.id
JOIN products p ON p.id = oi.product_id
WHERE o.id = ?;
```

## Aggregation

### SQ-106: GROUP BY
```sql
SELECT
    date(created_at) as date,
    COUNT(*) as total_orders,
    SUM(total) as revenue,
    AVG(total) as avg_order
FROM orders
WHERE created_at >= date('now', '-7 days')
GROUP BY date(created_at);
```

### SQ-107: HAVING
```sql
SELECT user_id, COUNT(*) as order_count
FROM orders
GROUP BY user_id
HAVING order_count > 5;
```

### SQ-108: GROUP_CONCAT
```sql
SELECT
    user_id,
    GROUP_CONCAT(tag, ', ') as tags
FROM user_tags
GROUP BY user_id;

-- With DISTINCT
SELECT
    user_id,
    GROUP_CONCAT(DISTINCT tag) as unique_tags
FROM user_tags
GROUP BY user_id;
```

## Window Functions (SQLite 3.25+)

### SQ-109: ROW_NUMBER
```sql
SELECT * FROM (
    SELECT
        *,
        ROW_NUMBER() OVER (
            PARTITION BY user_id
            ORDER BY created_at DESC
        ) as rn
    FROM orders
)
WHERE rn = 1;
```

### SQ-110: Running totals
```sql
SELECT
    id,
    total,
    SUM(total) OVER (ORDER BY created_at) as running_total
FROM orders;
```

### SQ-111: RANK and DENSE_RANK
```sql
SELECT
    user_id,
    total_spent,
    RANK() OVER (ORDER BY total_spent DESC) as rank,
    DENSE_RANK() OVER (ORDER BY total_spent DESC) as dense_rank
FROM user_stats;
```

### SQ-112: LAG/LEAD
```sql
SELECT
    id,
    total,
    LAG(total) OVER (ORDER BY created_at) as prev_total,
    LEAD(total) OVER (ORDER BY created_at) as next_total
FROM orders;
```

## CTEs

### SQ-113: Simple CTE
```sql
WITH active_users AS (
    SELECT DISTINCT user_id
    FROM orders
    WHERE created_at > date('now', '-30 days')
)
SELECT u.*
FROM users u
JOIN active_users au ON au.user_id = u.id;
```

### SQ-114: Recursive CTE
```sql
WITH RECURSIVE category_tree AS (
    SELECT id, name, parent_id, 0 as level
    FROM categories
    WHERE parent_id IS NULL

    UNION ALL

    SELECT c.id, c.name, c.parent_id, ct.level + 1
    FROM categories c
    JOIN category_tree ct ON ct.id = c.parent_id
)
SELECT * FROM category_tree;
```

### SQ-115: Multiple CTEs
```sql
WITH
orders_summary AS (
    SELECT user_id, SUM(total) as total_spent
    FROM orders
    GROUP BY user_id
),
user_segments AS (
    SELECT
        user_id,
        CASE
            WHEN total_spent > 1000 THEN 'vip'
            WHEN total_spent > 100 THEN 'regular'
            ELSE 'new'
        END as segment
    FROM orders_summary
)
SELECT u.*, us.segment
FROM users u
JOIN user_segments us ON us.user_id = u.id;
```

## JSON Operations (SQLite 3.38+)

### SQ-116: Query JSON
```sql
-- Extract value
SELECT json_extract(data, '$.name') as name
FROM items WHERE id = ?;

-- Arrow operator (SQLite 3.38+)
SELECT data->>'$.name' as name FROM items;
SELECT data->'$.nested'->'$.field' FROM items;

-- Query by JSON field
SELECT * FROM items
WHERE json_extract(data, '$.status') = 'active';
```

### SQ-117: JSON functions
```sql
-- Check valid JSON
SELECT json_valid(data) FROM items;

-- Get JSON type
SELECT json_type(data, '$.field') FROM items;

-- Array operations
SELECT json_array_length(data, '$.items') FROM orders;
SELECT json_each.value FROM items, json_each(items.data, '$.tags');
```

### SQ-118: Modify JSON
```sql
-- Set value
UPDATE items SET data = json_set(data, '$.status', 'inactive')
WHERE id = ?;

-- Insert value
UPDATE items SET data = json_insert(data, '$.newField', 'value')
WHERE id = ?;

-- Remove key
UPDATE items SET data = json_remove(data, '$.temp')
WHERE id = ?;

-- Replace value
UPDATE items SET data = json_replace(data, '$.count', 10)
WHERE id = ?;
```

## Full-Text Search (FTS5)

### SQ-119: Create FTS table
```sql
CREATE VIRTUAL TABLE articles_fts USING fts5(
    title, content,
    content='articles',
    content_rowid='id'
);

-- Populate
INSERT INTO articles_fts(articles_fts) VALUES('rebuild');
```

### SQ-120: Search
```sql
-- Basic search
SELECT * FROM articles_fts WHERE articles_fts MATCH 'sqlite database';

-- Phrase search
SELECT * FROM articles_fts WHERE articles_fts MATCH '"sqlite database"';

-- Boolean operators
SELECT * FROM articles_fts
WHERE articles_fts MATCH 'sqlite AND database NOT mysql';

-- Column-specific
SELECT * FROM articles_fts WHERE title MATCH 'sqlite';
```

### SQ-121: Ranking
```sql
SELECT
    rowid,
    title,
    bm25(articles_fts) as rank
FROM articles_fts
WHERE articles_fts MATCH ?
ORDER BY rank;
```

## Date/Time Functions

### SQ-122: Current time
```sql
SELECT
    datetime('now') as utc_now,
    datetime('now', 'localtime') as local_now,
    date('now') as today,
    time('now') as current_time;
```

### SQ-123: Date arithmetic
```sql
SELECT
    datetime('now', '+1 day') as tomorrow,
    datetime('now', '-7 days') as week_ago,
    datetime('now', '+1 month') as next_month,
    datetime('now', 'start of month') as month_start,
    datetime('now', 'weekday 0') as next_sunday;
```

### SQ-124: Date formatting
```sql
SELECT strftime('%Y-%m-%d %H:%M:%S', created_at) as formatted
FROM events;

SELECT strftime('%Y', created_at) as year,
       strftime('%m', created_at) as month
FROM events;
```

### SQ-125: Date grouping
```sql
SELECT
    strftime('%Y-%m', created_at) as month,
    COUNT(*) as count
FROM orders
GROUP BY strftime('%Y-%m', created_at);
```

## Pagination

### SQ-126: OFFSET pagination
```sql
SELECT * FROM orders
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
```

### SQ-127: Keyset pagination (recommended)
```sql
-- First page
SELECT * FROM orders
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- Next pages
SELECT * FROM orders
WHERE (created_at, id) < (?, ?)
ORDER BY created_at DESC, id DESC
LIMIT ?;
```

## Transactions

### SQ-128: Basic transaction
```sql
BEGIN TRANSACTION;
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
UPDATE accounts SET balance = balance + 100 WHERE id = 2;
COMMIT;
```

### SQ-129: Savepoints
```sql
BEGIN TRANSACTION;
INSERT INTO orders (...) VALUES (...);
SAVEPOINT order_created;
INSERT INTO order_items (...) VALUES (...);
-- On app_error:
ROLLBACK TO SAVEPOINT order_created;
COMMIT;
```

### SQ-130: Transaction modes
```sql
BEGIN DEFERRED;   -- Default, acquires locks as needed
BEGIN IMMEDIATE;  -- Acquires RESERVED lock immediately
BEGIN EXCLUSIVE;  -- Acquires EXCLUSIVE lock immediately
```

## Indexing

### SQ-131: Create indexes
```sql
-- Basic index
CREATE INDEX idx_orders_user_id ON orders(user_id);

-- Composite index
CREATE INDEX idx_orders_user_status ON orders(user_id, status);

-- Unique index
CREATE UNIQUE INDEX idx_users_email ON users(email);

-- Partial index
CREATE INDEX idx_orders_pending ON orders(created_at)
WHERE status = 'pending';

-- Expression index
CREATE INDEX idx_users_email_lower ON users(lower(email));
```

### SQ-132: Index management
```sql
-- List indexes
SELECT * FROM sqlite_master WHERE type = 'index';

-- Drop index
DROP INDEX idx_orders_user_id;

-- Reindex
REINDEX idx_orders_user_id;
```

## Useful Functions

### SQ-133: String functions
```sql
SELECT
    lower(name) as lowercase,
    upper(name) as uppercase,
    length(name) as len,
    substr(name, 1, 10) as short,
    replace(text, 'old', 'new') as replaced,
    trim(name) as trimmed,
    instr(name, 'pattern') as position
FROM users;
```

### SQ-134: NULL handling
```sql
SELECT
    coalesce(nickname, name, 'Anonymous') as display_name,
    ifnull(score, 0) as score,
    nullif(value, 0) as null_if_zero,
    iif(condition, 'yes', 'no') as result  -- SQLite 3.32+
FROM users;
```

### SQ-135: Type conversion
```sql
SELECT
    cast(value as INTEGER) as int_val,
    cast(value as REAL) as float_val,
    cast(value as TEXT) as text_val,
    typeof(value) as type_name
FROM items;
```

## Performance Hints

### SQ-136: EXPLAIN QUERY PLAN
```sql
EXPLAIN QUERY PLAN
SELECT * FROM orders WHERE user_id = ?;
```

### SQ-137: Analyze
```sql
ANALYZE;                    -- All tables
ANALYZE orders;             -- Specific table
```

### SQ-138: Vacuum
```sql
VACUUM;                     -- Rebuild database
VACUUM INTO 'backup.db';    -- Create optimized copy
```

## Common Patterns

### SQ-139: Upsert
```sql
INSERT INTO user_prefs (user_id, key, value)
VALUES (?, ?, ?)
ON CONFLICT(user_id, key) DO UPDATE SET
    value = excluded.value,
    updated_at = datetime('now');
```

### SQ-140: Get or create
```sql
INSERT OR IGNORE INTO users (email, name) VALUES (?, ?);
SELECT * FROM users WHERE email = ?;
```

### SQ-141: Soft delete
```sql
UPDATE users SET deleted_at = datetime('now') WHERE id = ?;
SELECT * FROM users WHERE deleted_at IS NULL;
```
