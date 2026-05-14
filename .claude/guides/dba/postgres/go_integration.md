# PostgreSQL + Go Integration

## Quick Reference

| ID | Rule | Section |
|----|------|---------|
| PG-001 | Use pgxpool for connection pooling | [Connection Pool Setup](#connection-pool-setup) |
| PG-002 | Use ANY($1::type[]) instead of IN for arrays | [Array Parameters](#array-parameters) |
| PG-003 | Always use RETURNING for INSERT/UPDATE | [Insert with RETURNING](#insert-with-returning) |
| PG-004 | Use QueryRow for single row, Query for multiple | [Query Patterns](#query-patterns) |
| PG-005 | Batch operations with COPY or unnest() | [Batch Operations](#batch-operations) |
| PG-006 | Use prepared statements for repeated queries | [Prepared Statements](#prepared-statements) |
| PG-007 | Handle pgx.ErrNoRows explicitly | [Error Handling](#error-handling) |
| PG-008 | Use context with timeout for all queries | [Context and Timeouts](#context-and-timeouts) |

---

## Driver Selection

### pgx (recommended)
```go
import (
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)
```
- Native PostgreSQL protocol
- Best performance
- Connection pooling built-in
- Supports COPY, LISTEN/NOTIFY
- Prepared statements

### database/sql + pgx
```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)
```
- Standard library compatibility
- Works with sqlx, GORM
- Slightly lower performance

### lib/pq (legacy)
```go
import (
    "database/sql"
    _ "github.com/lib/pq"
)
```
- Pure Go
- Stable but less actively developed
- Use pgx for new projects

## Connection Pool Setup

### PG-001: Use pgxpool for Connection Pooling
```go
package postgres

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
    Host         string
    Port         int
    User         string
    Password     string
    Database     string
    MaxConns     int32
    MinConns     int32
    MaxConnLife  time.Duration
    MaxConnIdle  time.Duration
}

func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
    dsn := fmt.Sprintf(
        "postgres://%s:%s@%s:%d/%s?sslmode=disable",
        cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
    )

    poolCfg, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }

    poolCfg.MaxConns = cfg.MaxConns
    poolCfg.MinConns = cfg.MinConns
    poolCfg.MaxConnLifetime = cfg.MaxConnLife
    poolCfg.MaxConnIdleTime = cfg.MaxConnIdle

    pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
    if err != nil {
        return nil, fmt.Errorf("create pool: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("ping: %w", err)
    }

    return pool, nil
}
```

### PG-002: Connection Pool Sizing Rules
```go
// Rule of thumb:
// MaxConns = (CPU cores * 2) + effective_spindle_count
// For SSD: MaxConns = CPU cores * 2 + 1
// MinConns = MaxConns / 4

poolCfg.MaxConns = 20
poolCfg.MinConns = 5
poolCfg.MaxConnLifetime = time.Hour
poolCfg.MaxConnIdleTime = 30 * time.Minute
```

## Query Patterns

### PG-003: Use QueryRow for Single Row Queries
```go
func (r *Repository) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
    const query = `
        SELECT id, name, email, created_at
        FROM users
        WHERE id = $1
    `

    var user User
    err := r.pool.QueryRow(ctx, query, id).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
        &user.CreatedAt,
    )
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrNotFound
        }
        return nil, fmt.Errorf("query user: %w", err)
    }

    return &user, nil
}
```

### PG-004: Use Query for Multiple Rows
```go
func (r *Repository) GetUserOrders(ctx context.Context, userID uuid.UUID, limit int) ([]Order, error) {
    const query = `
        SELECT id, user_id, total, status, created_at
        FROM orders
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT $2
    `

    rows, err := r.pool.Query(ctx, query, userID, limit)
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
        return nil, fmt.Errorf("rows iteration: %w", err)
    }

    return orders, nil
}
```

### PG-005: Use pgx.CollectRows for Efficient Row Collection
```go
func (r *Repository) GetUserOrders(ctx context.Context, userID uuid.UUID) ([]Order, error) {
    const query = `SELECT id, user_id, total, status, created_at FROM orders WHERE user_id = $1`

    orders, err := pgx.CollectRows(
        r.pool.Query(ctx, query, userID),
        pgx.RowToStructByName[Order],
    )
    if err != nil {
        return nil, fmt.Errorf("collect orders: %w", err)
    }

    return orders, nil
}
```

### PG-006: Always Use RETURNING for INSERT Operations
```go
func (r *Repository) CreateUser(ctx context.Context, name, email string) (*User, error) {
    const query = `
        INSERT INTO users (name, email, created_at)
        VALUES ($1, $2, NOW())
        RETURNING id, name, email, created_at
    `

    var user User
    err := r.pool.QueryRow(ctx, query, name, email).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
        &user.CreatedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("insert user: %w", err)
    }

    return &user, nil
}
```

### PG-007: Check Affected Rows on UPDATE Operations
```go
func (r *Repository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status string) error {
    const query = `
        UPDATE orders
        SET status = $2, updated_at = NOW()
        WHERE id = $1
    `

    tag, err := r.pool.Exec(ctx, query, id, status)
    if err != nil {
        return fmt.Errorf("update order: %w", err)
    }

    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }

    return nil
}
```

## Batch Operations

### PG-008: Use pgx.Batch for Multiple Operations
```go
func (r *Repository) CreateItems(ctx context.Context, items []Item) error {
    const query = `
        INSERT INTO items (name, value, created_at)
        VALUES ($1, $2, NOW())
    `

    batch := &pgx.Batch{}
    for _, item := range items {
        batch.Queue(query, item.Name, item.Value)
    }

    results := r.pool.SendBatch(ctx, batch)
    defer results.Close()

    for range items {
        _, err := results.Exec()
        if err != nil {
            return fmt.Errorf("batch insert: %w", err)
        }
    }

    return nil
}
```

### PG-009: Use COPY Protocol for Bulk Inserts
```go
func (r *Repository) BulkInsertItems(ctx context.Context, items []Item) (int64, error) {
    rows := make([][]interface{}, len(items))
    for i, item := range items {
        rows[i] = []interface{}{item.Name, item.Value, time.Now()}
    }

    count, err := r.pool.CopyFrom(
        ctx,
        pgx.Identifier{"items"},
        []string{"name", "value", "created_at"},
        pgx.CopyFromRows(rows),
    )
    if err != nil {
        return 0, fmt.Errorf("copy from: %w", err)
    }

    return count, nil
}
```

## Transactions

### PG-010: Use Explicit Transactions for Multi-Step Operations
```go
func (r *Repository) TransferFunds(ctx context.Context, from, to uuid.UUID, amount int64) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx) // no-op if committed

    // Debit
    _, err = tx.Exec(ctx,
        `UPDATE accounts SET balance = balance - $2 WHERE id = $1 AND balance >= $2`,
        from, amount,
    )
    if err != nil {
        return fmt.Errorf("debit: %w", err)
    }

    // Credit
    _, err = tx.Exec(ctx,
        `UPDATE accounts SET balance = balance + $2 WHERE id = $1`,
        to, amount,
    )
    if err != nil {
        return fmt.Errorf("credit: %w", err)
    }

    if err := tx.Commit(ctx); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    return nil
}
```

### PG-011: Set Appropriate Isolation Levels for Transactions
```go
func (r *Repository) SerializableOperation(ctx context.Context) error {
    tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
        IsoLevel: pgx.Serializable,
    })
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    // ... operations ...

    return tx.Commit(ctx)
}
```

### PG-012: Create Transaction Helper Functions
```go
func (r *Repository) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
    tx, err := r.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx)

    if err := fn(tx); err != nil {
        return err
    }

    if err := tx.Commit(ctx); err != nil {
        return fmt.Errorf("commit tx: %w", err)
    }

    return nil
}

// Usage
err := r.WithTx(ctx, func(tx pgx.Tx) error {
    // ... operations with tx ...
    return nil
})
```

## Prepared Statements

### PG-013: Rely on pgx Automatic Prepared Statement Caching
```go
// pgx automatically prepares and caches statements
// No manual preparation needed for most cases
```

### PG-014: Manual Statement Preparation for Specific Cases
```go
func (r *Repository) PrepareStatements(ctx context.Context) error {
    conn, err := r.pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release()

    _, err = conn.Conn().Prepare(ctx, "get_user",
        "SELECT id, name FROM users WHERE id = $1")
    if err != nil {
        return err
    }

    return nil
}

// Use prepared statement
rows, err := conn.Query(ctx, "get_user", userID)
```

## Array Parameters

### PG-015: Use ANY Operator Instead of IN for Arrays
```go
func (r *Repository) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]User, error) {
    const query = `
        SELECT id, name, email
        FROM users
        WHERE id = ANY($1)
    `

    rows, err := r.pool.Query(ctx, query, ids)
    // ...
}
```

### PG-016: Leverage pgx Automatic Array Type Handling
```go
// pgx handles Go slices automatically
ids := []uuid.UUID{...}
tags := []string{"tag1", "tag2"}

// For complex arrays, use pgtype
import "github.com/jackc/pgx/v5/pgtype"

var arr pgtype.Int4Array
arr.Set([]int32{1, 2, 3})
```

## JSONB Handling

### PG-017: Use pgx Automatic JSONB Marshaling for Reads
```go
type Item struct {
    ID   uuid.UUID
    Data map[string]interface{}
}

func (r *Repository) GetItem(ctx context.Context, id uuid.UUID) (*Item, error) {
    const query = `SELECT id, data FROM items WHERE id = $1`

    var item Item
    err := r.pool.QueryRow(ctx, query, id).Scan(&item.ID, &item.Data)
    // pgx automatically unmarshals JSONB to map/struct
    return &item, err
}
```

### PG-018: Pass Maps/Structs to JSONB Columns Directly
```go
func (r *Repository) CreateItem(ctx context.Context, data map[string]interface{}) error {
    const query = `INSERT INTO items (data) VALUES ($1)`
    _, err := r.pool.Exec(ctx, query, data)
    return err
}
```

### PG-019: Define Typed Structs for JSONB Columns
```go
type Metadata struct {
    Type   string `json:"type"`
    Source string `json:"source"`
}

type Item struct {
    ID       uuid.UUID
    Metadata Metadata
}

// pgx will marshal/unmarshal automatically
```

## Error Handling

### PG-020: Handle Constraint Violations with pgconn Error Codes
```go
import "github.com/jackc/pgx/v5/pgconn"

func (r *Repository) CreateUser(ctx context.Context, user *User) error {
    _, err := r.pool.Exec(ctx, query, user.Name, user.Email)
    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) {
            switch pgErr.Code {
            case "23505": // unique_violation
                return ErrDuplicateEmail
            case "23503": // foreign_key_violation
                return ErrInvalidReference
            case "23514": // check_violation
                return ErrInvalidData
            }
        }
        return fmt.Errorf("create user: %w", err)
    }
    return nil
}
```

### PG-021: Define PostgreSQL Error Code Constants
```go
const (
    PgUniqueViolation     = "23505"
    PgForeignKeyViolation = "23503"
    PgCheckViolation      = "23514"
    PgNotNullViolation    = "23502"
    PgSerializationFailure = "40001"
)
```

## Context and Timeouts

### PG-022: Use Context Timeouts for All Queries
```go
func (r *Repository) SlowQuery(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    _, err := r.pool.Exec(ctx, "SELECT pg_sleep(10)")
    // Will be cancelled after 5 seconds
    return err
}
```

### PG-023: Set Statement Timeouts in Queries
```go
const query = `
    SET LOCAL statement_timeout = '5s';
    SELECT * FROM slow_view;
`
```

## LISTEN/NOTIFY

### PG-024: Use LISTEN/NOTIFY for Event Broadcasting
```go
func (r *Repository) NotifyOrderCreated(ctx context.Context, orderID uuid.UUID) error {
    _, err := r.pool.Exec(ctx,
        "SELECT pg_notify('order_created', $1)",
        orderID.String(),
    )
    return err
}
```

### PG-025: Implement LISTEN Subscribers
```go
func (r *Repository) ListenOrders(ctx context.Context, handler func(orderID string)) error {
    conn, err := r.pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release()

    _, err = conn.Exec(ctx, "LISTEN order_created")
    if err != nil {
        return err
    }

    for {
        notification, err := conn.Conn().WaitForNotification(ctx)
        if err != nil {
            return err
        }
        handler(notification.Payload)
    }
}
```

## Testing

### PG-026: Test with Real PostgreSQL Database
```go
func TestRepository(t *testing.T) {
    pool := setupTestDB(t)
    defer pool.Close()

    repo := NewRepository(pool)

    t.Run("CreateUser", func(t *testing.T) {
        user, err := repo.CreateUser(ctx, "test", "test@example.com")
        require.NoError(t, err)
        assert.NotEmpty(t, user.ID)
    })
}

func setupTestDB(t *testing.T) *pgxpool.Pool {
    t.Helper()

    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL not set")
    }

    pool, err := pgxpool.New(context.Background(), dsn)
    require.NoError(t, err)

    t.Cleanup(func() {
        pool.Close()
    })

    return pool
}
```

### PG-027: Use Transaction Rollback for Test Isolation
```go
func TestWithRollback(t *testing.T) {
    pool := setupTestDB(t)

    tx, err := pool.Begin(context.Background())
    require.NoError(t, err)
    defer tx.Rollback(context.Background())

    // All operations in tx will be rolled back
    repo := NewRepositoryWithTx(tx)
    // ... test ...
}
```
