# PostgreSQL Storage and Table Types

## Quick Reference

This guide covers PostgreSQL storage engines and table types.

| ID | Rule | Context |
|----|------|---------|
| PGD-001 | Use persistent volumes for data | Storage configuration |
| PGD-002 | Set shared_buffers = 25% RAM | Memory configuration |

---

## Table Access Methods

PostgreSQL 12+ supports pluggable storage engines via Table Access Methods (TAM). Default is heap.

### PG-600: Use Heap Storage for Standard OLTP Tables

Standard row-based storage. Best for OLTP workloads.

```sql
-- Explicit heap (optional, it's the default)
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    customer_id INT,
    total DECIMAL(10,2)
) USING heap;
```

**Characteristics:**
- Row-oriented storage
- MVCC with dead tuples
- HOT updates when possible
- Supports all index types

### PG-601: Use Fillfactor for HOT Update Optimization

Controls how much space to leave for updates (HOT optimization).

```sql
-- Leave 20% space for in-place updates
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    status VARCHAR(20)
) WITH (fillfactor = 80);

-- For tables with frequent updates to non-indexed columns
ALTER TABLE orders SET (fillfactor = 70);

-- For append-only tables
ALTER TABLE event_log SET (fillfactor = 100);
```

**HOT Updates (Heap-Only Tuples):**
- When updated columns aren't indexed
- And there's space in the same page
- No new index entries needed
- Significantly faster

## Table Types

### PG-602: Use Regular WAL-Logged Tables for Production Data

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

- WAL-logged (durable)
- Survives crashes
- Replicated

### PG-603: Use Unlogged Tables for Caches and Temporary Data

```sql
CREATE UNLOGGED TABLE session_cache (
    session_id UUID PRIMARY KEY,
    user_id INT,
    data JSONB,
    expires_at TIMESTAMPTZ
);
```

**Characteristics:**
- **NOT WAL-logged** - much faster writes (2-10x)
- **NOT crash-safe** - truncated on crash/restart
- **NOT replicated** - invisible to replicas
- **No VACUUM overhead** for dead tuple cleanup

**Use cases:**
- Session data
- Caches
- ETL staging tables
- Temporary computation results

```sql
-- Convert existing table to unlogged
ALTER TABLE staging_data SET UNLOGGED;

-- Convert back to logged (requires full rewrite)
ALTER TABLE important_data SET LOGGED;
```

### PG-604: Use Temporary Tables for Session-Scoped Data

```sql
-- Session-scoped temporary table
CREATE TEMP TABLE temp_results (
    id INT,
    value NUMERIC
);

-- With explicit scope
CREATE TEMPORARY TABLE temp_calc (
    id INT
) ON COMMIT DROP;  -- Drop at end of transaction

CREATE TEMPORARY TABLE temp_work (
    id INT
) ON COMMIT DELETE ROWS;  -- Truncate at end of transaction

CREATE TEMPORARY TABLE temp_session (
    id INT
) ON COMMIT PRESERVE ROWS;  -- Keep data (default)
```

**Characteristics:**
- Visible only to creating session
- Automatically dropped at session end
- Stored in temp_tablespaces
- Not WAL-logged (fast)
- No catalog pollution

**Best practice:**

```sql
-- Use IF NOT EXISTS for reusable temp tables
CREATE TEMP TABLE IF NOT EXISTS batch_ids (id INT);
TRUNCATE batch_ids;
```

### PG-605: Access External Data with Foreign Tables

Access external data sources via Foreign Data Wrappers (FDW).

```sql
-- Install FDW extension
CREATE EXTENSION postgres_fdw;

-- Create foreign server
CREATE SERVER remote_db
FOREIGN DATA WRAPPER postgres_fdw
OPTIONS (host 'remote-host', dbname 'remote_db', port '5432');

-- Map users
CREATE USER MAPPING FOR local_user
SERVER remote_db
OPTIONS (user 'remote_user', password 'secret');

-- Create foreign table
CREATE FOREIGN TABLE remote_users (
    id INT,
    email VARCHAR(255),
    created_at TIMESTAMPTZ
)
SERVER remote_db
OPTIONS (schema_name 'public', table_name 'users');

-- Use like regular table
SELECT * FROM remote_users WHERE id = 123;
```

**Available FDWs:**
- `postgres_fdw` - Other PostgreSQL servers
- `file_fdw` - CSV/text files
- `mysql_fdw` - MySQL/MariaDB
- `oracle_fdw` - Oracle
- `mongo_fdw` - MongoDB
- `redis_fdw` - Redis

### PG-606: Use Partitioned Tables for Large Datasets

See `replication_partitioning.md` for details.

```sql
CREATE TABLE events (
    id BIGSERIAL,
    event_time TIMESTAMPTZ NOT NULL,
    data JSONB
) PARTITION BY RANGE (event_time);

CREATE TABLE events_2024_01 PARTITION OF events
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
```

## Storage Parameters

### PG-607: Configure Table-Level Storage Parameters

```sql
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    data JSONB
) WITH (
    fillfactor = 80,                    -- Space for HOT updates
    autovacuum_enabled = true,          -- Enable autovacuum
    autovacuum_vacuum_threshold = 50,   -- Min dead tuples to trigger
    autovacuum_vacuum_scale_factor = 0.02,  -- % of table size
    toast_tuple_target = 2048           -- TOAST threshold
);

-- Modify existing table
ALTER TABLE orders SET (fillfactor = 70);
ALTER TABLE orders RESET (fillfactor);
```

### PG-608: Optimize TOAST for Large Columns

Large values (>2KB) are compressed and/or stored out-of-line.

```sql
-- Check TOAST table
SELECT relname, reltoastrelid::regclass as toast_table
FROM pg_class
WHERE relname = 'documents';

-- TOAST strategies per column
ALTER TABLE documents ALTER COLUMN content SET STORAGE EXTENDED;
```

**Storage strategies:**

| Strategy | Compression | Out-of-line | Use for |
|----------|-------------|-------------|---------|
| PLAIN | No | No | Fixed-size, small (int, float) |
| MAIN | Yes | Last resort | Medium-sized, often accessed |
| EXTERNAL | No | Yes | Pre-compressed, large blobs |
| EXTENDED | Yes | Yes | Default for large types |

```sql
-- Check column storage
SELECT attname, attstorage
FROM pg_attribute
WHERE attrelid = 'documents'::regclass AND attnum > 0;

-- attstorage: p=PLAIN, m=MAIN, e=EXTERNAL, x=EXTENDED
```

## Tablespaces

### PG-609: Use Tablespaces for Storage Distribution

Store tables on different storage devices.

```sql
-- Create tablespace
CREATE TABLESPACE fast_ssd LOCATION '/mnt/ssd/pgdata';
CREATE TABLESPACE slow_hdd LOCATION '/mnt/hdd/archive';

-- Create table in specific tablespace
CREATE TABLE hot_data (
    id INT PRIMARY KEY,
    data JSONB
) TABLESPACE fast_ssd;

-- Move existing table
ALTER TABLE cold_data SET TABLESPACE slow_hdd;

-- Move index
ALTER INDEX idx_hot_data_pkey SET TABLESPACE fast_ssd;

-- Default tablespace for database
ALTER DATABASE mydb SET TABLESPACE fast_ssd;
```

**Use cases:**
- Hot data on SSD, cold data on HDD
- Spread I/O across devices
- Separate temp tables tablespace

```sql
-- Set temp tablespace
SET temp_tablespaces = 'fast_ssd';
```

## Comparison Table

| Type | WAL | Crash-Safe | Replicated | Visible | Performance |
|------|-----|------------|------------|---------|-------------|
| Regular | Yes | Yes | Yes | All sessions | Baseline |
| Unlogged | No | No | No | All sessions | 2-10x faster writes |
| Temporary | No | No | No | Creating session | Fast, session-scoped |
| Foreign | N/A | Depends | No | All sessions | Network latency |

## Go Integration Considerations

### PG-610: Handle Temporary Tables in Connection Pools

```go
// Problem: temp tables per session, but pool reuses connections
// Solution: clean up or use transaction-scoped temp tables

func (r *Repository) ProcessBatch(ctx context.Context, ids []int64) error {
    return r.db.WithTx(ctx, func(tx *sqlx.Tx) error {
        // Create temp table that drops at end of transaction
        _, err := tx.ExecContext(ctx, `
            CREATE TEMP TABLE batch_ids (id BIGINT) ON COMMIT DROP
        `)
        if err != nil {
            return err
        }

        // Use temp table
        // ... process ...

        return nil  // Temp table dropped automatically
    })
}
```

### PG-611: Implement Caching with Unlogged Tables

```go
type CacheRepo struct {
    db *sqlx.DB
}

func (r *CacheRepo) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO cache_store (key, value, expires_at)
        VALUES ($1, $2, NOW() + $3::interval)
        ON CONFLICT (key) DO UPDATE
        SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at
    `, key, value, ttl.String())
    return err
}

// Table definition (run once)
// CREATE UNLOGGED TABLE cache_store (
//     key VARCHAR(255) PRIMARY KEY,
//     value BYTEA,
//     expires_at TIMESTAMPTZ
// );
```

### PG-612: Optimize Large JSON Retrieval with TOAST

```go
// For large JSONB documents, consider compression settings
// and retrieval patterns

func (r *Repository) GetDocumentMetadata(ctx context.Context, id int64) (*Metadata, error) {
    // Select only needed columns to avoid fetching TOASTed content
    row := r.db.QueryRowContext(ctx, `
        SELECT id, title, created_at, pg_column_size(content) as content_size
        FROM documents
        WHERE id = $1
    `, id)
    // ...
}
```
