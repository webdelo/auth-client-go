# ClickHouse + Go Integration

## Quick Reference

| ID | Rule | Section |
|----|------|---------|
| CH-001 | Use batch inserts, never single-row INSERT | [Batch Insert](#batch-insert) |
| CH-002 | Choose correct MergeTree engine for use case | [Best Practices](#best-practices) |
| CH-003 | Partition by date/month for time-series | [Best Practices](#best-practices) |
| CH-004 | Use PREWHERE for filtering before reading | [Query Patterns](#query-patterns) |
| CH-005 | Avoid JOINs, denormalize data | [Best Practices](#best-practices) |
| CH-006 | Use AggregatingMergeTree for real-time aggregates | [Aggregation Queries](#aggregation-queries) |
| CH-007 | Set max_execution_time for queries | [Query Settings](#query-settings) |

---

## Driver Selection

### clickhouse-go v2 (recommended)
```go
import "github.com/ClickHouse/clickhouse-go/v2"
```
- Native protocol (fastest)
- Batch inserts
- Compression support
- Connection pooling

### database/sql compatible
```go
import (
    "database/sql"
    _ "github.com/ClickHouse/clickhouse-go/v2"
)
```
- Standard library interface
- Works with sqlx

## Connection Setup

### Native interface
```go
package clickhouse

import (
    "context"
    "crypto/tls"
    "time"

    "github.com/ClickHouse/clickhouse-go/v2"
    "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Config struct {
    Hosts         []string
    Database      string
    Username      string
    Password      string
    MaxOpenConns  int
    MaxIdleConns  int
    DialTimeout   time.Duration
    ReadTimeout   time.Duration
    Compression   bool
    TLS           bool
}

func NewConnection(cfg Config) (driver.Conn, error) {
    opts := &clickhouse.Options{
        Addr: cfg.Hosts,
        Auth: clickhouse.Auth{
            Database: cfg.Database,
            Username: cfg.Username,
            Password: cfg.Password,
        },
        DialTimeout:      cfg.DialTimeout,
        ReadTimeout:      cfg.ReadTimeout,
        MaxOpenConns:     cfg.MaxOpenConns,
        MaxIdleConns:     cfg.MaxIdleConns,
        ConnMaxLifetime:  time.Hour,
    }

    if cfg.Compression {
        opts.Compression = &clickhouse.Compression{
            Method: clickhouse.CompressionLZ4,
        }
    }

    if cfg.TLS {
        opts.TLS = &tls.Config{
            InsecureSkipVerify: false,
        }
    }

    conn, err := clickhouse.Open(opts)
    if err != nil {
        return nil, err
    }

    if err := conn.Ping(context.Background()); err != nil {
        return nil, err
    }

    return conn, nil
}
```

### database/sql interface
```go
import (
    "database/sql"
    "time"

    _ "github.com/ClickHouse/clickhouse-go/v2"
)

func NewDB(dsn string) (*sql.DB, error) {
    // DSN format: clickhouse://user:password@host:9000/database?dial_timeout=10s
    db, err := sql.Open("clickhouse", dsn)
    if err != nil {
        return nil, err
    }

    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(time.Hour)

    if err := db.Ping(); err != nil {
        return nil, err
    }

    return db, nil
}
```

## Query Patterns

### Simple query
```go
func (r *Repository) GetEventCount(ctx context.Context, userID uint64, date time.Time) (uint64, error) {
    var count uint64
    err := r.conn.QueryRow(ctx, `
        SELECT count()
        FROM events
        WHERE user_id = @user_id AND date = @date
    `,
        clickhouse.Named("user_id", userID),
        clickhouse.Named("date", date),
    ).Scan(&count)

    if err != nil {
        return 0, fmt.Errorf("query event count: %w", err)
    }

    return count, nil
}
```

### Multiple rows
```go
type Event struct {
    ID        string    `ch:"id"`
    UserID    uint64    `ch:"user_id"`
    EventType string    `ch:"event_type"`
    CreatedAt time.Time `ch:"created_at"`
}

func (r *Repository) GetUserEvents(ctx context.Context, userID uint64, limit int) ([]Event, error) {
    rows, err := r.conn.Query(ctx, `
        SELECT id, user_id, event_type, created_at
        FROM events
        WHERE user_id = @user_id
        ORDER BY created_at DESC
        LIMIT @limit
    `,
        clickhouse.Named("user_id", userID),
        clickhouse.Named("limit", limit),
    )
    if err != nil {
        return nil, fmt.Errorf("query events: %w", err)
    }
    defer rows.Close()

    var events []Event
    for rows.Next() {
        var e Event
        if err := rows.ScanStruct(&e); err != nil {
            return nil, fmt.Errorf("scan event: %w", err)
        }
        events = append(events, e)
    }

    return events, rows.Err()
}
```

### Using Select helper
```go
func (r *Repository) GetUserEvents(ctx context.Context, userID uint64) ([]Event, error) {
    var events []Event
    err := r.conn.Select(ctx, &events, `
        SELECT id, user_id, event_type, created_at
        FROM events
        WHERE user_id = @user_id
    `, clickhouse.Named("user_id", userID))

    if err != nil {
        return nil, fmt.Errorf("select events: %w", err)
    }

    return events, nil
}
```

## Batch Insert

### CH-001: Basic batch
```go
func (r *Repository) InsertEvents(ctx context.Context, events []Event) error {
    batch, err := r.conn.PrepareBatch(ctx, `
        INSERT INTO events (id, user_id, event_type, created_at)
    `)
    if err != nil {
        return fmt.Errorf("prepare batch: %w", err)
    }

    for _, e := range events {
        if err := batch.Append(e.ID, e.UserID, e.EventType, e.CreatedAt); err != nil {
            return fmt.Errorf("append to batch: %w", err)
        }
    }

    if err := batch.Send(); err != nil {
        return fmt.Errorf("send batch: %w", err)
    }

    return nil
}
```

### CH-002: Batch with struct
```go
func (r *Repository) InsertEventsStruct(ctx context.Context, events []Event) error {
    batch, err := r.conn.PrepareBatch(ctx, `
        INSERT INTO events
    `)
    if err != nil {
        return fmt.Errorf("prepare batch: %w", err)
    }

    for _, e := range events {
        if err := batch.AppendStruct(&e); err != nil {
            return fmt.Errorf("append struct: %w", err)
        }
    }

    return batch.Send()
}
```

### CH-003: Async insert
```go
func (r *Repository) InsertEventsAsync(ctx context.Context, events []Event) error {
    for _, e := range events {
        err := r.conn.AsyncInsert(ctx, `
            INSERT INTO events (id, user_id, event_type, created_at)
            VALUES (@id, @user_id, @event_type, @created_at)
        `, false,
            clickhouse.Named("id", e.ID),
            clickhouse.Named("user_id", e.UserID),
            clickhouse.Named("event_type", e.EventType),
            clickhouse.Named("created_at", e.CreatedAt),
        )
        if err != nil {
            return fmt.Errorf("async insert: %w", err)
        }
    }
    return nil
}
```

## Complex Types

### CH-004: Arrays
```go
type EventWithTags struct {
    ID   string   `ch:"id"`
    Tags []string `ch:"tags"`
}

// Insert array
batch.Append(event.ID, event.Tags)

// Query array
var events []EventWithTags
err := conn.Select(ctx, &events, `
    SELECT id, tags FROM events WHERE has(tags, @tag)
`, clickhouse.Named("tag", "important"))
```

### CH-005: Maps
```go
type EventWithMetadata struct {
    ID       string            `ch:"id"`
    Metadata map[string]string `ch:"metadata"`
}

// Create table with Map type
// CREATE TABLE events (id String, metadata Map(String, String))
```

### CH-006: Nested types
```go
type Order struct {
    ID    string      `ch:"id"`
    Items []OrderItem `ch:"items"`
}

type OrderItem struct {
    ProductID string  `ch:"product_id"`
    Quantity  uint32  `ch:"quantity"`
    Price     float64 `ch:"price"`
}

// Table with Nested
// CREATE TABLE orders (
//     id String,
//     items Nested(
//         product_id String,
//         quantity UInt32,
//         price Float64
//     )
// )
```

### CH-007: UUID
```go
import "github.com/google/uuid"

type Event struct {
    ID uuid.UUID `ch:"id"`
}

// ClickHouse UUID type maps to google/uuid
```

### CH-008: Decimal
```go
import "github.com/shopspring/decimal"

type Transaction struct {
    ID     string          `ch:"id"`
    Amount decimal.Decimal `ch:"amount"`
}
```

## Aggregation Queries

### CH-009: Time series aggregation
```go
type HourlyStats struct {
    Hour   time.Time `ch:"hour"`
    Events uint64    `ch:"events"`
    Users  uint64    `ch:"users"`
}

func (r *Repository) GetHourlyStats(ctx context.Context, date time.Time) ([]HourlyStats, error) {
    var stats []HourlyStats
    err := r.conn.Select(ctx, &stats, `
        SELECT
            toStartOfHour(created_at) as hour,
            count() as events,
            uniq(user_id) as users
        FROM events
        WHERE date = @date
        GROUP BY hour
        ORDER BY hour
    `, clickhouse.Named("date", date))

    return stats, err
}
```

### CH-010: Quantiles
```go
type LatencyStats struct {
    P50 float64 `ch:"p50"`
    P95 float64 `ch:"p95"`
    P99 float64 `ch:"p99"`
}

func (r *Repository) GetLatencyStats(ctx context.Context) (*LatencyStats, error) {
    var stats LatencyStats
    err := r.conn.QueryRow(ctx, `
        SELECT
            quantile(0.5)(response_time) as p50,
            quantile(0.95)(response_time) as p95,
            quantile(0.99)(response_time) as p99
        FROM requests
        WHERE date = today()
    `).ScanStruct(&stats)

    return &stats, err
}
```

## Query Settings

### CH-011: Per-query settings
```go
ctx := clickhouse.Context(ctx, clickhouse.WithSettings(clickhouse.Settings{
    "max_execution_time": 60,
    "max_memory_usage":   10000000000,
}))

rows, err := conn.Query(ctx, query)
```

### CH-012: Query ID for tracking
```go
ctx := clickhouse.Context(ctx, clickhouse.WithQueryID("my-query-123"))
rows, err := conn.Query(ctx, query)
```

### CH-013: Progress tracking
```go
ctx := clickhouse.Context(ctx, clickhouse.WithProgress(func(p *clickhouse.Progress) {
    fmt.Printf("Read: %d rows, %d bytes\n", p.Rows, p.Bytes)
}))
```

## Error Handling

### CH-014: Exception types
```go
import "github.com/ClickHouse/clickhouse-go/v2/lib/proto"

func handleError(err error) error {
    var exception *proto.Exception
    if errors.As(err, &exception) {
        switch exception.Code {
        case 60: // UNKNOWN_TABLE
            return ErrTableNotFound
        case 62: // SYNTAX_ERROR
            return ErrSyntaxError
        case 159: // TIMEOUT_EXCEEDED
            return ErrTimeout
        case 241: // MEMORY_LIMIT_EXCEEDED
            return ErrMemoryLimit
        }
    }
    return err
}
```

## Testing

### CH-015: Test with real ClickHouse
```go
func setupTestClickHouse(t *testing.T) driver.Conn {
    t.Helper()

    conn, err := clickhouse.Open(&clickhouse.Options{
        Addr: []string{"localhost:9000"},
        Auth: clickhouse.Auth{
            Database: "test",
        },
    })
    require.NoError(t, err)

    t.Cleanup(func() {
        conn.Close()
    })

    return conn
}
```

### CH-016: Create test table
```go
func createTestTable(t *testing.T, conn driver.Conn) {
    t.Helper()

    tableName := fmt.Sprintf("test_%d", time.Now().UnixNano())

    err := conn.Exec(context.Background(), fmt.Sprintf(`
        CREATE TABLE %s (
            id UUID,
            name String,
            created_at DateTime
        ) ENGINE = MergeTree()
        ORDER BY id
    `, tableName))
    require.NoError(t, err)

    t.Cleanup(func() {
        conn.Exec(context.Background(), "DROP TABLE IF EXISTS "+tableName)
    })
}
```

## Best Practices

### CH-017: Batch sizes
```go
const (
    // Optimal batch size for inserts
    BatchSize = 10000

    // Max batch size before flush
    MaxBatchSize = 100000

    // Flush interval
    FlushInterval = time.Second
)
```

### CH-018: Connection pooling
```go
// For high-throughput: more connections
opts.MaxOpenConns = 20
opts.MaxIdleConns = 10

// For analytics queries: fewer, longer connections
opts.MaxOpenConns = 5
opts.MaxIdleConns = 2
opts.ConnMaxLifetime = 2 * time.Hour
```

### CH-019: Retry logic
```go
func (r *Repository) QueryWithRetry(ctx context.Context, query string, args ...interface{}) error {
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        if attempt > 0 {
            time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
        }

        err := r.conn.Exec(ctx, query, args...)
        if err == nil {
            return nil
        }

        lastErr = err

        // Don't retry syntax errors
        var exception *proto.Exception
        if errors.As(err, &exception) && exception.Code == 62 {
            return err
        }
    }
    return lastErr
}
```
