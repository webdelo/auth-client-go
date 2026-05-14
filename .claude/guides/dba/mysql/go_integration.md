# MySQL + Go Integration

## Quick Reference

| ID | Rule | Section |
|----|------|---------|
| MY-001 | Use InnoDB for all tables | [Best Practices](#best-practices) |
| MY-002 | Set parseTime=true in DSN | [Connection Setup](#connection-setup) |
| MY-003 | Use prepared statements | [Prepared Statements](#prepared-statements) |
| MY-004 | Avoid SELECT * in production | [Query Patterns](#query-patterns) |
| MY-005 | Use covering indexes for frequent queries | [Best Practices](#best-practices) |

---

## Driver Selection

### go-sql-driver/mysql (standard)
```go
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)
```
- database/sql compatible
- Widely used, stable
- Good performance

### sqlx (enhanced database/sql)
```go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/go-sql-driver/mysql"
)
```
- Named parameters
- Struct scanning
- Better ergonomics

## Connection Setup

### Basic connection
```go
package mysql

import (
    "database/sql"
    "fmt"
    "time"

    _ "github.com/go-sql-driver/mysql"
)

type Config struct {
    Host         string
    Port         int
    User         string
    Password     string
    Database     string
    MaxOpenConns int
    MaxIdleConns int
    MaxLifetime  time.Duration
}

func NewDB(cfg Config) (*sql.DB, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC",
        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }

    db.SetMaxOpenConns(cfg.MaxOpenConns)
    db.SetMaxIdleConns(cfg.MaxIdleConns)
    db.SetConnMaxLifetime(cfg.MaxLifetime)

    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("ping db: %w", err)
    }

    return db, nil
}
```

### DSN options
```go
dsn := fmt.Sprintf(
    "%s:%s@tcp(%s:%d)/%s?"+
        "parseTime=true&"+          // Parse TIME/DATE to time.Time
        "loc=UTC&"+                 // Timezone
        "charset=utf8mb4&"+         // Character set
        "collation=utf8mb4_unicode_ci&"+
        "timeout=10s&"+             // Connection timeout
        "readTimeout=30s&"+         // Read timeout
        "writeTimeout=30s&"+        // Write timeout
        "maxAllowedPacket=16777216", // 16MB
    user, password, host, port, database,
)
```

### Connection pool settings
```go
// Rule of thumb for web servers:
// MaxOpenConns = max concurrent requests that need DB
// MaxIdleConns = typical concurrent DB usage

db.SetMaxOpenConns(100)
db.SetMaxIdleConns(10)
db.SetConnMaxLifetime(time.Hour)
db.SetConnMaxIdleTime(10 * time.Minute)
```

## Query Patterns

### MY-006: Single row query
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

### MY-007: Multiple rows
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

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows app_error: %w", err)
    }

    return orders, nil
}
```

### MY-008: Insert with last insert ID
```go
func (r *Repository) CreateUser(ctx context.Context, name, email string) (int64, error) {
    const query = `
        INSERT INTO users (name, email, created_at)
        VALUES (?, ?, NOW())
    `

    result, err := r.db.ExecContext(ctx, query, name, email)
    if err != nil {
        return 0, fmt.Errorf("insert user: %w", err)
    }

    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("get last insert id: %w", err)
    }

    return id, nil
}
```

### MY-009: Update with affected rows
```go
func (r *Repository) UpdateOrderStatus(ctx context.Context, id int64, status string) error {
    const query = `
        UPDATE orders
        SET status = ?, updated_at = NOW()
        WHERE id = ?
    `

    result, err := r.db.ExecContext(ctx, query, status, id)
    if err != nil {
        return fmt.Errorf("update order: %w", err)
    }

    affected, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("get rows affected: %w", err)
    }

    if affected == 0 {
        return ErrNotFound
    }

    return nil
}
```

## Using sqlx

### Setup
```go
import "github.com/jmoiron/sqlx"

func NewDBX(cfg Config) (*sqlx.DB, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

    db, err := sqlx.Connect("mysql", dsn)
    if err != nil {
        return nil, err
    }

    return db, nil
}
```

### MY-010: Struct scanning
```go
type User struct {
    ID        int64     `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at"`
}

func (r *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
    var user User
    err := r.db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = ?", id)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrNotFound
        }
        return nil, err
    }
    return &user, nil
}

func (r *Repository) GetUsers(ctx context.Context, ids []int64) ([]User, error) {
    query, args, err := sqlx.In("SELECT * FROM users WHERE id IN (?)", ids)
    if err != nil {
        return nil, err
    }

    var users []User
    err = r.db.SelectContext(ctx, &users, query, args...)
    return users, err
}
```

### MY-011: Named parameters
```go
func (r *Repository) CreateUser(ctx context.Context, user *User) error {
    const query = `
        INSERT INTO users (name, email, created_at)
        VALUES (:name, :email, NOW())
    `

    result, err := r.db.NamedExecContext(ctx, query, user)
    if err != nil {
        return err
    }

    id, _ := result.LastInsertId()
    user.ID = id
    return nil
}
```

## Batch Operations

### MY-012: Batch insert
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

### MY-013: Batch insert with sqlx
```go
func (r *Repository) CreateItemsSqlx(ctx context.Context, items []Item) error {
    const query = `
        INSERT INTO items (name, value)
        VALUES (:name, :value)
    `

    _, err := r.db.NamedExecContext(ctx, query, items)
    return err
}
```

### MY-014: Batch upsert
```go
func (r *Repository) UpsertStats(ctx context.Context, stats []UserStat) error {
    const query = `
        INSERT INTO user_stats (user_id, login_count, last_login)
        VALUES (?, ?, NOW())
        ON DUPLICATE KEY UPDATE
            login_count = login_count + VALUES(login_count),
            last_login = NOW()
    `

    stmt, err := r.db.PrepareContext(ctx, query)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, s := range stats {
        if _, err := stmt.ExecContext(ctx, s.UserID, s.LoginCount); err != nil {
            return err
        }
    }

    return nil
}
```

## Transactions

### MY-015: Basic transaction
```go
func (r *Repository) Transfer(ctx context.Context, from, to int64, amount int64) error {
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

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    return nil
}
```

### MY-016: Transaction helper
```go
func (r *Repository) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    if err := fn(tx); err != nil {
        return err
    }

    return tx.Commit()
}

// Usage
err := r.WithTx(ctx, func(tx *sql.Tx) error {
    // ... operations with tx ...
    return nil
})
```

### MY-017: Transaction with isolation level
```go
func (r *Repository) SerializableOperation(ctx context.Context) error {
    tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelSerializable,
    })
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // ... operations ...

    return tx.Commit()
}
```

## Prepared Statements

### MY-018: Single use (automatically prepared)
```go
// database/sql automatically prepares and caches on the connection
rows, err := db.QueryContext(ctx, "SELECT * FROM users WHERE id = ?", id)
```

### MY-019: Explicit preparation
```go
func (r *Repository) PrepareStatements(ctx context.Context) error {
    var err error

    r.stmtGetUser, err = r.db.PrepareContext(ctx,
        "SELECT id, name, email FROM users WHERE id = ?")
    if err != nil {
        return err
    }

    r.stmtUpdateUser, err = r.db.PrepareContext(ctx,
        "UPDATE users SET name = ?, email = ? WHERE id = ?")
    if err != nil {
        return err
    }

    return nil
}

func (r *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
    var user User
    err := r.stmtGetUser.QueryRowContext(ctx, id).Scan(
        &user.ID, &user.Name, &user.Email,
    )
    return &user, err
}
```

## NULL Handling

### MY-020: Using sql.Null types
```go
type User struct {
    ID       int64
    Name     string
    Nickname sql.NullString
    Score    sql.NullInt64
}

func (r *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
    var user User
    err := r.db.QueryRowContext(ctx,
        "SELECT id, name, nickname, score FROM users WHERE id = ?", id,
    ).Scan(&user.ID, &user.Name, &user.Nickname, &user.Score)

    // Check null
    if user.Nickname.Valid {
        fmt.Println(user.Nickname.String)
    }

    return &user, err
}
```

### MY-021: Using pointers
```go
type User struct {
    ID       int64
    Name     string
    Nickname *string
    Score    *int64
}

func (r *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
    var user User
    err := r.db.QueryRowContext(ctx,
        "SELECT id, name, nickname, score FROM users WHERE id = ?", id,
    ).Scan(&user.ID, &user.Name, &user.Nickname, &user.Score)

    return &user, err
}
```

## Error Handling

### MY-022: MySQL error codes
```go
import "github.com/go-sql-driver/mysql"

func handleMySQLError(err error) error {
    if err == nil {
        return nil
    }

    var mysqlErr *mysql.MySQLError
    if errors.As(err, &mysqlErr) {
        switch mysqlErr.Number {
        case 1062: // Duplicate entry
            return ErrDuplicateKey
        case 1452: // Foreign key constraint
            return ErrForeignKeyViolation
        case 1451: // Cannot delete, foreign key constraint
            return ErrForeignKeyRestrict
        case 1213: // Deadlock
            return ErrDeadlock
        case 1205: // Lock wait timeout
            return ErrLockTimeout
        }
    }

    return err
}
```

### MY-023: Retry on deadlock
```go
func (r *Repository) WithRetry(ctx context.Context, fn func() error) error {
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        lastErr = err

        var mysqlErr *mysql.MySQLError
        if errors.As(err, &mysqlErr) && mysqlErr.Number == 1213 {
            // Deadlock, retry
            time.Sleep(time.Duration(attempt*100) * time.Millisecond)
            continue
        }

        return err
    }
    return lastErr
}
```

## JSON Column Handling

### MY-024: Read JSON
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

### MY-025: Write JSON
```go
func (r *Repository) CreateItem(ctx context.Context, id int64, data interface{}) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }

    _, err = r.db.ExecContext(ctx,
        "INSERT INTO items (id, data) VALUES (?, ?)",
        id, jsonData,
    )
    return err
}
```

## Testing

### MY-026: Test with real database
```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()

    dsn := os.Getenv("TEST_MYSQL_DSN")
    if dsn == "" {
        t.Skip("TEST_MYSQL_DSN not set")
    }

    db, err := sql.Open("mysql", dsn)
    require.NoError(t, err)

    t.Cleanup(func() {
        db.Close()
    })

    return db
}
```

### MY-027: Transaction rollback for tests
```go
func TestWithRollback(t *testing.T) {
    db := setupTestDB(t)

    tx, err := db.Begin()
    require.NoError(t, err)
    defer tx.Rollback()

    repo := NewRepositoryWithTx(tx)
    // ... test with repo ...
    // tx.Rollback() on defer ensures cleanup
}
```

## Best Practices

### MY-028: Context with timeout
```go
func (r *Repository) SlowQuery(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    _, err := r.db.QueryContext(ctx, "SELECT SLEEP(10)")
    return err
}
```

### MY-029: Connection health check
```go
func (r *Repository) HealthCheck(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, time.Second)
    defer cancel()

    return r.db.PingContext(ctx)
}
```

### MY-030: Logging queries
```go
// Use a wrapper or middleware
type LoggingDB struct {
    db     *sql.DB
    logger *zap.Logger
}

func (l *LoggingDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    start := time.Now()
    rows, err := l.db.QueryContext(ctx, query, args...)
    l.logger.Debug("query executed",
        zap.String("query", query),
        zap.Duration("duration", time.Since(start)),
        zap.Error(err),
    )
    return rows, err
}
```
