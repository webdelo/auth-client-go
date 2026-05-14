# MySQL Query Patterns

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MY-003 | Use prepared statements | [Prepared Statements](#prepared-statements) |
| MY-004 | Avoid SELECT * in production | [SELECT](#select) |
| MY-005 | Use covering indexes for frequent queries | [Index Optimization](#index-optimization) |

---

## Basic CRUD

### MY-100: SELECT
```sql
-- Simple select
SELECT id, name, email FROM users WHERE id = ?;

-- Multiple conditions
SELECT * FROM orders
WHERE status = ? AND created_at > ?
ORDER BY created_at DESC
LIMIT ?;

-- IN clause
SELECT * FROM users WHERE id IN (?, ?, ?);
```

### MY-101: INSERT
```sql
-- Single row
INSERT INTO users (name, email, created_at)
VALUES (?, ?, NOW());

-- Get last insert ID
SELECT LAST_INSERT_ID();

-- Batch insert
INSERT INTO items (name, value)
VALUES (?, ?), (?, ?), (?, ?);

-- INSERT IGNORE (skip duplicates)
INSERT IGNORE INTO users (id, name, email)
VALUES (?, ?, ?);

-- ON DUPLICATE KEY UPDATE
INSERT INTO user_stats (user_id, login_count, last_login)
VALUES (?, 1, NOW())
ON DUPLICATE KEY UPDATE
    login_count = login_count + 1,
    last_login = NOW();
```

### MY-102: UPDATE
```sql
-- Simple update
UPDATE users SET name = ?, updated_at = NOW()
WHERE id = ?;

-- Conditional update
UPDATE orders SET status = ?
WHERE id = ? AND status = 'pending';

-- Update with JOIN
UPDATE orders o
JOIN users u ON u.id = o.user_id
SET o.priority = 'high'
WHERE u.vip = 1;
```

### MY-103: DELETE
```sql
-- Simple delete
DELETE FROM users WHERE id = ?;

-- Delete with JOIN
DELETE o FROM orders o
JOIN users u ON u.id = o.user_id
WHERE u.status = 'inactive';

-- Delete with LIMIT (for batching)
DELETE FROM logs WHERE created_at < ? LIMIT 1000;
```

## JOINs

### MY-104: INNER JOIN
```sql
SELECT o.id, o.total, u.name, u.email
FROM orders o
INNER JOIN users u ON u.id = o.user_id
WHERE o.created_at > ?;
```

### MY-105: LEFT JOIN
```sql
SELECT u.id, u.name, COUNT(o.id) as order_count
FROM users u
LEFT JOIN orders o ON o.user_id = u.id
GROUP BY u.id, u.name;
```

### MY-106: Multi-table JOIN
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
WHERE o.id = ?;
```

### MY-107: Self JOIN
```sql
SELECT e.name as employee, m.name as manager
FROM employees e
LEFT JOIN employees m ON m.id = e.manager_id;
```

## Aggregation

### MY-108: GROUP BY
```sql
SELECT
    DATE(created_at) as date,
    COUNT(*) as total_orders,
    SUM(total) as revenue,
    AVG(total) as avg_order_value
FROM orders
WHERE created_at >= ?
GROUP BY DATE(created_at)
ORDER BY date;
```

### MY-109: HAVING
```sql
SELECT user_id, COUNT(*) as order_count
FROM orders
GROUP BY user_id
HAVING order_count > 5;
```

### MY-110: GROUP_CONCAT
```sql
SELECT
    user_id,
    GROUP_CONCAT(tag ORDER BY tag SEPARATOR ',') as tags
FROM user_tags
GROUP BY user_id;
```

### MY-111: Conditional aggregation
```sql
SELECT
    COUNT(*) as total,
    SUM(status = 'completed') as completed,
    SUM(status = 'pending') as pending,
    SUM(IF(status = 'completed', total, 0)) as completed_revenue
FROM orders;
```

## Window Functions (MySQL 8.0+)

### MY-112: ROW_NUMBER
```sql
SELECT * FROM (
    SELECT
        *,
        ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) as rn
    FROM orders
) sub
WHERE rn = 1;
```

### MY-113: Running totals
```sql
SELECT
    id,
    created_at,
    total,
    SUM(total) OVER (ORDER BY created_at) as running_total
FROM orders
WHERE user_id = ?;
```

### MY-114: RANK and DENSE_RANK
```sql
SELECT
    user_id,
    total_spent,
    RANK() OVER (ORDER BY total_spent DESC) as rank,
    DENSE_RANK() OVER (ORDER BY total_spent DESC) as dense_rank
FROM user_stats;
```

### MY-115: LAG/LEAD
```sql
SELECT
    id,
    total,
    LAG(total) OVER (ORDER BY created_at) as prev_total,
    total - LAG(total) OVER (ORDER BY created_at) as diff
FROM orders
WHERE user_id = ?;
```

## Subqueries

### MY-116: Scalar subquery
```sql
SELECT
    id,
    name,
    (SELECT COUNT(*) FROM orders WHERE user_id = users.id) as order_count
FROM users;
```

### MY-117: EXISTS
```sql
SELECT * FROM users u
WHERE EXISTS (
    SELECT 1 FROM orders o WHERE o.user_id = u.id
);
```

### MY-118: IN subquery
```sql
SELECT * FROM products
WHERE category_id IN (
    SELECT id FROM categories WHERE active = 1
);
```

### MY-119: Derived table
```sql
SELECT u.*, stats.order_count
FROM users u
JOIN (
    SELECT user_id, COUNT(*) as order_count
    FROM orders
    GROUP BY user_id
) stats ON stats.user_id = u.id;
```

## CTEs (MySQL 8.0+)

### MY-120: Simple CTE
```sql
WITH active_users AS (
    SELECT DISTINCT user_id
    FROM orders
    WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)
)
SELECT u.*
FROM users u
JOIN active_users au ON au.user_id = u.id;
```

### MY-121: Recursive CTE
```sql
WITH RECURSIVE category_tree AS (
    -- Base case
    SELECT id, name, parent_id, 0 as level
    FROM categories
    WHERE parent_id IS NULL

    UNION ALL

    -- Recursive case
    SELECT c.id, c.name, c.parent_id, ct.level + 1
    FROM categories c
    JOIN category_tree ct ON ct.id = c.parent_id
)
SELECT * FROM category_tree ORDER BY level, name;
```

## JSON Operations (MySQL 5.7+)

### MY-122: Query JSON
```sql
-- Get JSON field
SELECT data->>'$.name' as name FROM items WHERE id = ?;

-- Query by JSON field
SELECT * FROM items WHERE JSON_EXTRACT(data, '$.status') = 'active';

-- Shorthand
SELECT * FROM items WHERE data->'$.status' = '"active"';
```

### MY-123: JSON functions
```sql
-- Check if key exists
SELECT * FROM items WHERE JSON_CONTAINS_PATH(data, 'one', '$.metadata');

-- Search in array
SELECT * FROM items WHERE JSON_CONTAINS(data->'$.tags', '"important"');

-- Get array length
SELECT JSON_LENGTH(data->'$.items') as item_count FROM orders;
```

### MY-124: Modify JSON
```sql
-- Set value
UPDATE items SET data = JSON_SET(data, '$.status', 'inactive')
WHERE id = ?;

-- Remove key
UPDATE items SET data = JSON_REMOVE(data, '$.temp')
WHERE id = ?;

-- Add to array
UPDATE items SET data = JSON_ARRAY_APPEND(data, '$.tags', 'new_tag')
WHERE id = ?;
```

## Pagination

### MY-125: OFFSET pagination
```sql
SELECT * FROM orders
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
```

### MY-126: Keyset pagination (recommended)
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

## Full-Text Search

### MY-127: Natural language search
```sql
-- Create full-text index
ALTER TABLE articles ADD FULLTEXT INDEX idx_content (title, content);

-- Search
SELECT *, MATCH(title, content) AGAINST(? IN NATURAL LANGUAGE MODE) as relevance
FROM articles
WHERE MATCH(title, content) AGAINST(? IN NATURAL LANGUAGE MODE)
ORDER BY relevance DESC;
```

### MY-128: Boolean mode
```sql
SELECT * FROM articles
WHERE MATCH(title, content) AGAINST('+mysql -postgres' IN BOOLEAN MODE);
```

## Locking

### MY-129: SELECT FOR UPDATE
```sql
SELECT * FROM accounts
WHERE id = ?
FOR UPDATE;
```

### MY-130: SELECT FOR SHARE
```sql
SELECT * FROM accounts
WHERE id = ?
FOR SHARE;
```

### MY-131: SKIP LOCKED (MySQL 8.0+)
```sql
SELECT * FROM jobs
WHERE status = 'pending'
ORDER BY created_at
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

## Transactions

### MY-132: Basic transaction
```sql
START TRANSACTION;
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
UPDATE accounts SET balance = balance + 100 WHERE id = 2;
COMMIT;
```

### MY-133: Savepoints
```sql
START TRANSACTION;
INSERT INTO orders (...) VALUES (...);
SAVEPOINT order_created;
INSERT INTO order_items (...) VALUES (...);
-- On app_error:
ROLLBACK TO SAVEPOINT order_created;
COMMIT;
```

## Useful Functions

### MY-134: Date/Time
```sql
SELECT
    NOW() as current_time,
    CURDATE() as current_date,
    DATE_SUB(NOW(), INTERVAL 7 DAY) as week_ago,
    DATE_FORMAT(created_at, '%Y-%m-%d') as formatted,
    DATEDIFF(NOW(), created_at) as days_ago,
    TIMESTAMPDIFF(HOUR, created_at, NOW()) as hours_ago
FROM events;
```

### MY-135: String functions
```sql
SELECT
    CONCAT(first_name, ' ', last_name) as full_name,
    LOWER(email) as email_lower,
    SUBSTRING(description, 1, 100) as short_desc,
    REPLACE(text, 'old', 'new') as replaced,
    TRIM(name) as trimmed
FROM users;
```

### MY-136: NULL handling
```sql
SELECT
    COALESCE(nickname, name, 'Anonymous') as display_name,
    IFNULL(score, 0) as score,
    NULLIF(value, 0) as null_if_zero
FROM users;
```

## Common Anti-patterns to Avoid

### MY-137: Avoid SELECT *
```sql
-- Bad
SELECT * FROM users WHERE id = ?;

-- Good
SELECT id, name, email FROM users WHERE id = ?;
```

### MY-138: Avoid functions on indexed columns
```sql
-- Bad (can't use index)
SELECT * FROM users WHERE YEAR(created_at) = 2024;

-- Good
SELECT * FROM users
WHERE created_at >= '2024-01-01' AND created_at < '2025-01-01';
```

### MY-139: Avoid implicit type conversion
```sql
-- Bad (id is INT, string causes conversion)
SELECT * FROM users WHERE id = '123';

-- Good
SELECT * FROM users WHERE id = 123;
```

### MY-140: Use LIMIT with ORDER BY
```sql
-- Bad (returns random row)
SELECT * FROM users LIMIT 1;

-- Good
SELECT * FROM users ORDER BY id LIMIT 1;
```
