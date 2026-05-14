# SQLite Optimization Guide

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| SQ-001 | Enable WAL mode for concurrency | [PRAGMA Settings](#pragma-settings) |
| SQ-002 | Use PRAGMA synchronous=NORMAL for performance | [PRAGMA Settings](#pragma-settings) |
| SQ-003 | Enable foreign keys explicitly | [PRAGMA Settings](#pragma-settings) |

---

## PRAGMA Settings

### SQ-200: Essential PRAGMAs for Performance

```sql
-- Enable Write-Ahead Logging (WAL mode)
-- Allows concurrent reads during writes
PRAGMA journal_mode = WAL;

-- Synchronous mode
-- FULL: safest, slowest
-- NORMAL: good balance (recommended with WAL)
-- OFF: fastest, risk of corruption on crash
PRAGMA synchronous = NORMAL;

-- Memory-mapped I/O (faster reads)
PRAGMA mmap_size = 268435456;  -- 256MB

-- Cache size (negative = KB, positive = pages)
PRAGMA cache_size = -64000;  -- 64MB

-- Temp store in memory
PRAGMA temp_store = MEMORY;

-- Foreign key enforcement
PRAGMA foreign_keys = ON;

-- Busy timeout (ms) for locked database
PRAGMA busy_timeout = 5000;

-- Page size (set before creating database)
PRAGMA page_size = 4096;
```

### SQ-201: Go Integration for PRAGMAs

```go
import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

func OpenDB(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }

    // Apply PRAGMAs
    pragmas := []string{
        "PRAGMA journal_mode = WAL",
        "PRAGMA synchronous = NORMAL",
        "PRAGMA cache_size = -64000",
        "PRAGMA temp_store = MEMORY",
        "PRAGMA mmap_size = 268435456",
        "PRAGMA foreign_keys = ON",
        "PRAGMA busy_timeout = 5000",
    }

    for _, pragma := range pragmas {
        if _, err := db.Exec(pragma); err != nil {
            return nil, fmt.Errorf("pragma %s: %w", pragma, err)
        }
    }

    // Limit connections for SQLite (1 writer at a time)
    db.SetMaxOpenConns(1)  // For writes
    // Or use connection pool with proper locking

    return db, nil
}
```

## WAL Mode

### SQ-202: Benefits
- Concurrent reads during writes
- Faster writes (sequential)
- Better crash recovery
- Less fsync calls

### Considerations
- Requires WAL and SHM files
- Not suitable for network filesystems
- WAL file can grow large

### SQ-203: Checkpoint Management

```sql
-- Manual checkpoint
PRAGMA wal_checkpoint(PASSIVE);   -- Non-blocking
PRAGMA wal_checkpoint(FULL);      -- Wait for readers
PRAGMA wal_checkpoint(RESTART);   -- Reset WAL file
PRAGMA wal_checkpoint(TRUNCATE);  -- Truncate WAL to zero

-- Auto-checkpoint threshold (pages)
PRAGMA wal_autocheckpoint = 1000;  -- Default
```

```go
// Periodic checkpoint
func (r *SQLiteRepo) Checkpoint(ctx context.Context) error {
    _, err := r.db.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)")
    return err
}
```

## Index Strategies

### SQ-204: Basic Indexes

```sql
-- Single column
CREATE INDEX idx_users_email ON users(email);

-- Composite (order matters!)
CREATE INDEX idx_orders_user_date ON orders(user_id, created_at DESC);

-- Unique index
CREATE UNIQUE INDEX idx_users_email_unique ON users(email);

-- Partial index (SQLite 3.8+)
CREATE INDEX idx_orders_pending ON orders(created_at)
WHERE status = 'pending';

-- Expression index
CREATE INDEX idx_users_email_lower ON users(lower(email));
```

### SQ-205: Covering Indexes

```sql
-- Index includes all columns needed by query
CREATE INDEX idx_orders_covering ON orders(user_id, status, total, created_at);

-- This query uses index-only scan
SELECT status, total, created_at FROM orders WHERE user_id = ?;
```

### SQ-206: Index Analysis

```sql
-- Query plan
EXPLAIN QUERY PLAN SELECT * FROM orders WHERE user_id = 1;

-- Index list
PRAGMA index_list('orders');

-- Index info
PRAGMA index_info('idx_orders_user_date');

-- Table info
PRAGMA table_info('orders');

-- Database stats
ANALYZE;
```

## Query Optimization

### SQ-207: Use EXPLAIN QUERY PLAN

```sql
EXPLAIN QUERY PLAN
SELECT * FROM orders WHERE user_id = 1 ORDER BY created_at DESC;

-- Look for:
-- SCAN (bad) vs SEARCH (good)
-- USING INDEX vs full table scan
-- USE TEMP B-TREE (sort needed)
```

### SQ-208: Avoid Full Table Scans

```sql
-- Bad: no index on status
SELECT * FROM orders WHERE status = 'pending';

-- Good: add index
CREATE INDEX idx_orders_status ON orders(status);

-- Bad: function prevents index use
SELECT * FROM users WHERE lower(email) = 'user@example.com';

-- Good: expression index
CREATE INDEX idx_users_email_lower ON users(lower(email));
```

### SQ-209: Batch Operations

```sql
-- Bad: many individual inserts
INSERT INTO items (name) VALUES ('item1');
INSERT INTO items (name) VALUES ('item2');
...

-- Good: single transaction
BEGIN;
INSERT INTO items (name) VALUES ('item1');
INSERT INTO items (name) VALUES ('item2');
COMMIT;

-- Better: multi-value insert
INSERT INTO items (name) VALUES ('item1'), ('item2'), ('item3');
```

### SQ-210: Go Batch Insert

```go
func (r *SQLiteRepo) BatchInsert(ctx context.Context, items []Item) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    stmt, err := tx.PrepareContext(ctx, "INSERT INTO items (name, value) VALUES (?, ?)")
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, item := range items {
        if _, err := stmt.ExecContext(ctx, item.Name, item.Value); err != nil {
            return err
        }
    }

    return tx.Commit()
}
```

### SQ-211: LIMIT and Pagination

```sql
-- Offset pagination (slow for large offsets)
SELECT * FROM orders ORDER BY id LIMIT 10 OFFSET 1000;

-- Keyset pagination (faster)
SELECT * FROM orders WHERE id > ? ORDER BY id LIMIT 10;
```

## Concurrent Access

### SQ-212: Read-Write Patterns

```go
// SQLite allows only one writer at a time
// Use connection pool carefully

type SQLiteDB struct {
    writeDB *sql.DB  // Single connection for writes
    readDB  *sql.DB  // Pool for reads
}

func NewSQLiteDB(path string) (*SQLiteDB, error) {
    // Write connection (single)
    writeDB, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
    if err != nil {
        return nil, err
    }
    writeDB.SetMaxOpenConns(1)

    // Read connections (pool)
    readDB, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&mode=ro")
    if err != nil {
        return nil, err
    }
    readDB.SetMaxOpenConns(10)

    return &SQLiteDB{writeDB: writeDB, readDB: readDB}, nil
}

func (db *SQLiteDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    return db.readDB.QueryContext(ctx, query, args...)
}

func (db *SQLiteDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
    return db.writeDB.ExecContext(ctx, query, args...)
}
```

### SQ-213: Handling SQLITE_BUSY

```go
func (r *SQLiteRepo) ExecWithRetry(ctx context.Context, query string, args ...interface{}) error {
    var err error
    for i := 0; i < 5; i++ {
        _, err = r.db.ExecContext(ctx, query, args...)
        if err == nil {
            return nil
        }

        // Check if busy app_error
        if sqliteErr, ok := err.(sqlite3.Error); ok {
            if sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked {
                time.Sleep(time.Duration(100*(i+1)) * time.Millisecond)
                continue
            }
        }
        return err
    }
    return err
}
```

## Maintenance

### SQ-214: VACUUM

```sql
-- Rebuild database, reclaim space
VACUUM;

-- Incremental vacuum (with auto_vacuum enabled)
PRAGMA auto_vacuum = INCREMENTAL;
PRAGMA incremental_vacuum(1000);  -- Free up to 1000 pages
```

### SQ-215: ANALYZE

```sql
-- Update statistics for query planner
ANALYZE;

-- Analyze specific table
ANALYZE orders;
```

### SQ-216: Integrity Check

```sql
-- Check database integrity
PRAGMA integrity_check;

-- Quick check (faster, less thorough)
PRAGMA quick_check;

-- Foreign key check
PRAGMA foreign_key_check;
```

## Memory and File Management

### SQ-217: In-Memory Database

```go
// In-memory (lost on close)
db, _ := sql.Open("sqlite3", ":memory:")

// Shared in-memory (multiple connections)
db, _ := sql.Open("sqlite3", "file::memory:?cache=shared")
```

### SQ-218: Connection String Options

```go
// Full connection string example
dsn := "file:mydb.db?" +
    "_journal_mode=WAL&" +
    "_synchronous=NORMAL&" +
    "_cache_size=-64000&" +
    "_foreign_keys=ON&" +
    "_busy_timeout=5000"

db, err := sql.Open("sqlite3", dsn)
```

## Performance Checklist

1. **Enable WAL mode** - concurrent reads, faster writes
2. **Set appropriate cache_size** - more memory = fewer I/O
3. **Use synchronous=NORMAL with WAL** - good balance
4. **Create proper indexes** - based on query patterns
5. **Use transactions for batches** - avoid autocommit overhead
6. **Set busy_timeout** - handle concurrent access
7. **Run ANALYZE** - keep statistics updated
8. **Use prepared statements** - reduce parsing overhead
9. **Limit write connections** - SQLite is single-writer
10. **Regular VACUUM** - reclaim space, defragment

## Limitations to Remember

- Single writer at a time
- No built-in replication
- Not suitable for high-concurrency writes
- Network filesystem issues
- 281 TB max database size
- 1 GB max row size
