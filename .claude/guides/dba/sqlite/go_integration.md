# SQLite + Go Integration

## Quick Reference

| ID | Rule | Section |
|----|------|---------|
| SQ-001 | Enable WAL mode for concurrency | [Connection Setup](#connection-setup) |
| SQ-002 | Use PRAGMA synchronous=NORMAL for performance | [Connection Setup](#connection-setup) |
| SQ-003 | Enable foreign keys explicitly | [Connection Setup](#connection-setup) |
| SQ-004 | Use FTS5 for full-text search | [Full-Text Search](#full-text-search) |
| SQ-005 | Single writer, multiple readers | [Connection Setup](#connection-setup) |

---

## Driver Selection

### SQ-001: modernc.org/sqlite (pure Go, recommended)
```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)
```
- Pure Go, no CGO
- Cross-compilation friendly
- Slightly slower than CGO version

### SQ-002: mattn/go-sqlite3 (CGO)
```go
import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)
```
- CGO required
- Better performance
- More SQLite features

## Connection Setup

### SQ-003: Basic connection
```go
package sqlite

import (
    "database/sql"
    "fmt"

    _ "modernc.org/sqlite"
)

type Config struct {
    Path         string
    MaxOpenConns int
    MaxIdleConns int
}

func NewDB(cfg Config) (*sql.DB, error) {
    dsn := cfg.Path + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=1000000000&_foreign_keys=true"

    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }

    // SQLite works best with single connection for writes
    db.SetMaxOpenConns(cfg.MaxOpenConns)
    db.SetMaxIdleConns(cfg.MaxIdleConns)

    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("ping db: %w", err)
    }

    return db, nil
}
```

### SQ-004: Connection string options
```go
// Common pragmas via connection string
dsn := "file:mydb.db?" +
    "_journal_mode=WAL&" +       // Write-Ahead Logging (better concurrency)
    "_busy_timeout=5000&" +      // Wait 5s on lock (milliseconds)
    "_synchronous=NORMAL&" +     // Sync mode (OFF, NORMAL, FULL)
    "_cache_size=-64000&" +      // 64MB cache (negative = KB)
    "_foreign_keys=true&" +      // Enable foreign keys
    "_txlock=immediate"          // Transaction lock mode

// In-memory database
dsn := ":memory:?_foreign_keys=true"

// Shared cache for multiple connections
dsn := "file:mydb.db?cache=shared&_foreign_keys=true"
```

### SQ-005: Connection pool for SQLite
```go
func NewDB(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }

    // For writes: single connection prevents SQLITE_BUSY
    // For reads: multiple connections OK with WAL mode
    db.SetMaxOpenConns(1)  // Serialize writes
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0)  // No timeout

    // Enable WAL mode
    if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
        return nil, err
    }

    return db, nil
}
```

## Query Patterns

### SQ-006: Single row query
```go
func (r *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
    const query = `
        SELECT id, name, email, created_at
        FROM users
        WHERE id = ?
    `

    var user User
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
        &user.CreatedAt,
    )
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("query user: %w", err)
    }

    return &user, nil
}
```

### SQ-007: Multiple rows
```go
func (r *Repository) GetUserOrders(ctx context.Context, userID int64, limit int) ([]Order, error) {
    const query = `
        SELECT id, user_id, total, status, created_at
        FROM orders
        WHERE user_id = ?
        ORDER BY created_at DESC
        LIMIT ?
    `

    rows, err := r.db.QueryContext(ctx, query, userID, limit)
    if err != nil {
        return nil, fmt.Errorf("query orders: %w", err)
    }
    defer rows.Close()

    var orders []Order
    for rows.Next() {
        var o Order
        if err := rows.Scan(&o.ID, &o.UserID, &o.Total, &o.Status, &o.CreatedAt); err != nil {
            return nil, fmt.Errorf("scan order: %w", err)
        }
        orders = append(orders, o)
    }

    return orders, rows.Err()
}
```

### SQ-008: Insert with last insert ID
```go
func (r *Repository) CreateUser(ctx context.Context, name, email string) (int64, error) {
    const query = `
        INSERT INTO users (name, email, created_at)
        VALUES (?, ?, datetime('now'))
    `

    result, err := r.db.ExecContext(ctx, query, name, email)
    if err != nil {
        return 0, fmt.Errorf("insert user: %w", err)
    }

    return result.LastInsertId()
}
```

### SQ-009: Upsert
```go
func (r *Repository) UpsertSetting(ctx context.Context, key, value string) error {
    const query = `
        INSERT INTO settings (key, value, updated_at)
        VALUES (?, ?, datetime('now'))
        ON CONFLICT(key) DO UPDATE SET
            value = excluded.value,
            updated_at = datetime('now')
    `

    _, err := r.db.ExecContext(ctx, query, key, value)
    return err
}
```

## Batch Operations

### SQ-010: Batch insert
```go
func (r *Repository) CreateItems(ctx context.Context, items []Item) error {
    if len(items) == 0 {
        return nil
    }

    const baseQuery = "INSERT INTO items (name, value) VALUES "

    valueStrings := make([]string, len(items))
    valueArgs := make([]interface{}, 0, len(items)*2)

    for i, item := range items {
        valueStrings[i] = "(?, ?)"
        valueArgs = append(valueArgs, item.Name, item.Value)
    }

    query := baseQuery + strings.Join(valueStrings, ", ")
    _, err := r.db.ExecContext(ctx, query, valueArgs...)
    return err
}
```

### SQ-011: Batch with transaction
```go
func (r *Repository) CreateItemsTx(ctx context.Context, items []Item) error {
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

## Transactions

### SQ-012: Basic transaction
```go
func (r *Repository) Transfer(ctx context.Context, from, to int64, amount float64) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    // Debit
    result, err := tx.ExecContext(ctx,
        "UPDATE accounts SET balance = balance - ? WHERE id = ? AND balance >= ?",
        amount, from, amount,
    )
    if err != nil {
        return fmt.Errorf("debit: %w", err)
    }

    affected, _ := result.RowsAffected()
    if affected == 0 {
        return ErrInsufficientFunds
    }

    // Credit
    _, err = tx.ExecContext(ctx,
        "UPDATE accounts SET balance = balance + ? WHERE id = ?",
        amount, to,
    )
    if err != nil {
        return fmt.Errorf("credit: %w", err)
    }

    return tx.Commit()
}
```

### SQ-013: Transaction with immediate lock
```go
func (r *Repository) ExclusiveOperation(ctx context.Context, fn func(*sql.Tx) error) error {
    // Start with IMMEDIATE to get write lock immediately
    tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelSerializable, // Maps to IMMEDIATE in SQLite
    })
    if err != nil {
        return err
    }
    defer tx.Rollback()

    if err := fn(tx); err != nil {
        return err
    }

    return tx.Commit()
}
```

## JSON Handling

### SQ-014: Read JSON
```go
type Item struct {
    ID   int64
    Data json.RawMessage
}

func (r *Repository) GetItem(ctx context.Context, id int64) (*Item, error) {
    var item Item
    err := r.db.QueryRowContext(ctx,
        "SELECT id, data FROM items WHERE id = ?", id,
    ).Scan(&item.ID, &item.Data)

    return &item, err
}
```

### SQ-015: Query JSON fields
```go
func (r *Repository) GetActiveItems(ctx context.Context) ([]Item, error) {
    const query = `
        SELECT id, data
        FROM items
        WHERE json_extract(data, '$.status') = 'active'
    `

    rows, err := r.db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var items []Item
    for rows.Next() {
        var item Item
        if err := rows.Scan(&item.ID, &item.Data); err != nil {
            return nil, err
        }
        items = append(items, item)
    }

    return items, rows.Err()
}
```

## Error Handling

### SQ-016: SQLite error codes
```go
import "modernc.org/sqlite"

func handleSQLiteError(err error) error {
    if err == nil {
        return nil
    }

    var sqliteErr *sqlite.Error
    if errors.As(err, &sqliteErr) {
        switch sqliteErr.Code() {
        case sqlite3.SQLITE_CONSTRAINT_UNIQUE:
            return ErrDuplicateKey
        case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
            return ErrForeignKeyViolation
        case sqlite3.SQLITE_BUSY:
            return ErrDatabaseBusy
        case sqlite3.SQLITE_LOCKED:
            return ErrDatabaseLocked
        }
    }

    return err
}
```

### SQ-017: Retry on busy
```go
func (r *Repository) WithRetry(ctx context.Context, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        lastErr = err

        // Check if BUSY or LOCKED
        var sqliteErr *sqlite.Error
        if errors.As(err, &sqliteErr) {
            code := sqliteErr.Code()
            if code == sqlite3.SQLITE_BUSY || code == sqlite3.SQLITE_LOCKED {
                time.Sleep(time.Duration(attempt*100) * time.Millisecond)
                continue
            }
        }

        return err
    }
    return lastErr
}
```

## Full-Text Search

### SQ-018: Setup FTS5
```go
func (r *Repository) SetupFTS(ctx context.Context) error {
    const createFTS = `
        CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
            title, content,
            content='articles',
            content_rowid='id'
        )
    `

    const createTriggers = `
        CREATE TRIGGER IF NOT EXISTS articles_ai AFTER INSERT ON articles BEGIN
            INSERT INTO articles_fts(rowid, title, content)
            VALUES (new.id, new.title, new.content);
        END;

        CREATE TRIGGER IF NOT EXISTS articles_ad AFTER DELETE ON articles BEGIN
            INSERT INTO articles_fts(articles_fts, rowid, title, content)
            VALUES('delete', old.id, old.title, old.content);
        END;

        CREATE TRIGGER IF NOT EXISTS articles_au AFTER UPDATE ON articles BEGIN
            INSERT INTO articles_fts(articles_fts, rowid, title, content)
            VALUES('delete', old.id, old.title, old.content);
            INSERT INTO articles_fts(rowid, title, content)
            VALUES (new.id, new.title, new.content);
        END;
    `

    if _, err := r.db.ExecContext(ctx, createFTS); err != nil {
        return err
    }

    _, err := r.db.ExecContext(ctx, createTriggers)
    return err
}
```

### SQ-019: Search
```go
func (r *Repository) SearchArticles(ctx context.Context, query string) ([]Article, error) {
    const sql = `
        SELECT a.id, a.title, a.content
        FROM articles a
        JOIN articles_fts fts ON fts.rowid = a.id
        WHERE articles_fts MATCH ?
        ORDER BY bm25(articles_fts)
        LIMIT 20
    `

    rows, err := r.db.QueryContext(ctx, sql, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var articles []Article
    for rows.Next() {
        var a Article
        if err := rows.Scan(&a.ID, &a.Title, &a.Content); err != nil {
            return nil, err
        }
        articles = append(articles, a)
    }

    return articles, rows.Err()
}
```

## Testing

### SQ-020: In-memory database for tests
```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()

    db, err := sql.Open("sqlite", ":memory:?_foreign_keys=true")
    require.NoError(t, err)

    // Run migrations
    _, err = db.Exec(schema)
    require.NoError(t, err)

    t.Cleanup(func() {
        db.Close()
    })

    return db
}
```

### SQ-021: File database with cleanup
```go
func setupTestFileDB(t *testing.T) *sql.DB {
    t.Helper()

    tmpFile, err := os.CreateTemp("", "test-*.db")
    require.NoError(t, err)
    tmpFile.Close()

    db, err := sql.Open("sqlite", tmpFile.Name()+"?_journal_mode=WAL&_foreign_keys=true")
    require.NoError(t, err)

    _, err = db.Exec(schema)
    require.NoError(t, err)

    t.Cleanup(func() {
        db.Close()
        os.Remove(tmpFile.Name())
        os.Remove(tmpFile.Name() + "-wal")
        os.Remove(tmpFile.Name() + "-shm")
    })

    return db
}
```

## Best Practices

### SQ-022: WAL mode
```go
func EnableWAL(db *sql.DB) error {
    _, err := db.Exec("PRAGMA journal_mode=WAL")
    return err
}
```

### SQ-023: Optimize for reads
```go
func OptimizeForReads(db *sql.DB) error {
    pragmas := []string{
        "PRAGMA cache_size=-64000",     // 64MB
        "PRAGMA mmap_size=268435456",   // 256MB memory-mapped I/O
        "PRAGMA temp_store=MEMORY",
    }

    for _, pragma := range pragmas {
        if _, err := db.Exec(pragma); err != nil {
            return err
        }
    }

    return nil
}
```

### SQ-024: Vacuum periodically
```go
func (r *Repository) Vacuum(ctx context.Context) error {
    _, err := r.db.ExecContext(ctx, "VACUUM")
    return err
}
```

### SQ-025: Analyze for query planning
```go
func (r *Repository) Analyze(ctx context.Context) error {
    _, err := r.db.ExecContext(ctx, "ANALYZE")
    return err
}
```
