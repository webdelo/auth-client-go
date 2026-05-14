# MySQL Storage Engines

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MY-001 | Use InnoDB for all tables | [InnoDB](#innodb-default) |
| MYD-001 | Set innodb_buffer_pool_size = 70% RAM | [InnoDB Configuration](#innodb-configuration) |

---

## Overview

MySQL supports pluggable storage engines. Each engine has different characteristics.

```sql
-- List available engines
SHOW ENGINES;

-- Check default engine
SELECT @@default_storage_engine;

-- Check table engine
SHOW TABLE STATUS LIKE 'tablename';
```

## InnoDB (Default)

### MY-300: Characteristics
- ACID compliant (transactions)
- Row-level locking
- Foreign key support
- Crash recovery via redo logs
- MVCC for concurrent reads
- Clustered index on primary key

### Best For
- OLTP workloads
- High concurrency
- Data integrity requirements
- Most general use cases

### MY-302: Configuration

```ini
# my.cnf
[mysqld]
# Buffer pool (50-80% of RAM)
innodb_buffer_pool_size = 4G
innodb_buffer_pool_instances = 4  # 1 per GB

# Log files
innodb_log_file_size = 1G
innodb_log_buffer_size = 64M

# Flush settings
innodb_flush_log_at_trx_commit = 1  # Full ACID (1), Performance (2)
innodb_flush_method = O_DIRECT      # Avoid double buffering

# I/O threads
innodb_read_io_threads = 8
innodb_write_io_threads = 8

# Change buffer
innodb_change_buffer_max_size = 25  # Percentage of buffer pool
```

### MY-303: Create InnoDB Table

```sql
CREATE TABLE orders (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    customer_id BIGINT NOT NULL,
    total DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_customer (customer_id),
    FOREIGN KEY (customer_id) REFERENCES customers(id)
) ENGINE=InnoDB
  ROW_FORMAT=DYNAMIC
  DEFAULT CHARSET=utf8mb4;
```

### MY-304: InnoDB File-Per-Table

```ini
# Enable (recommended)
innodb_file_per_table = ON
```

Benefits:
- Easier backup per table
- Space reclaimed on DROP TABLE
- Better disk space management

## MyISAM (Legacy)

### MY-305: Characteristics
- Table-level locking
- No transactions
- No foreign keys
- Full-text search (legacy)
- Compact storage
- Fast reads for read-heavy workloads

### MY-306: Best For
- Read-only or read-mostly tables
- Legacy applications
- NOT recommended for new projects

```sql
CREATE TABLE logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    message TEXT,
    created_at TIMESTAMP
) ENGINE=MyISAM;
```

### MY-307: When to Avoid
- Concurrent writes
- Transaction requirements
- Data integrity needs

## MEMORY (HEAP)

### MY-308: Characteristics
- Data stored in RAM only
- Hash or B-tree indexes
- Table-level locking
- Fixed row length
- Data lost on restart

### MY-309: Best For
- Temporary lookup tables
- Caching
- Session data (non-persistent)

```sql
CREATE TABLE cache_data (
    cache_key VARCHAR(255) PRIMARY KEY,
    cache_value TEXT,
    expires_at TIMESTAMP
) ENGINE=MEMORY
  MAX_ROWS=100000;

-- Convert existing table
ALTER TABLE temp_data ENGINE=MEMORY;
```

### MY-310: Limitations
- No BLOB/TEXT (use VARCHAR)
- Data lost on crash
- Limited by max_heap_table_size

```ini
max_heap_table_size = 256M
```

## ARCHIVE

### MY-311: Characteristics
- INSERT and SELECT only (no UPDATE/DELETE)
- High compression (zlib)
- No indexes except AUTO_INCREMENT
- Good for audit logs

```sql
CREATE TABLE audit_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    event_type VARCHAR(50),
    payload JSON,
    created_at TIMESTAMP
) ENGINE=ARCHIVE;
```

### MY-312: Best For
- Write-once, rarely-read data
- Historical records
- Compliance data

## CSV

### MY-313: Characteristics
- Data stored as CSV file
- No indexes
- Easy export/import

```sql
CREATE TABLE csv_export (
    id INT NOT NULL,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL
) ENGINE=CSV;
```

### MY-314: Use Case (CSV)
- Data exchange
- Simple exports

## BLACKHOLE

### MY-315: Characteristics (BLACKHOLE)
- Accepts data but discards it
- Useful for replication filtering

```sql
CREATE TABLE replication_filter (
    id INT PRIMARY KEY,
    data TEXT
) ENGINE=BLACKHOLE;
```

## Engine Comparison

| Feature | InnoDB | MyISAM | MEMORY | ARCHIVE |
|---------|--------|--------|--------|---------|
| Transactions | Yes | No | No | No |
| Row Locking | Yes | No | No | No |
| Foreign Keys | Yes | No | No | No |
| Full-text | Yes (5.6+) | Yes | No | No |
| Compression | Yes | Yes | No | Yes |
| Crash Recovery | Yes | No | N/A | Limited |
| MVCC | Yes | No | No | No |

## Converting Between Engines (MY-316)

```sql
-- Convert table
ALTER TABLE mytable ENGINE=InnoDB;

-- Check progress
SHOW PROCESSLIST;

-- Convert with pt-online-schema-change (no locking)
pt-online-schema-change --alter "ENGINE=InnoDB" D=mydb,t=mytable
```

## Go Integration Considerations

### MY-317: Handle Engine Differences

```go
// Check if transaction support needed
func (r *MySQLRepo) Insert(ctx context.Context, data *Data) error {
    // InnoDB: can use transactions
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    _, err = tx.ExecContext(ctx, "INSERT INTO orders (customer_id, total) VALUES (?, ?)",
        data.CustomerID, data.Total)
    if err != nil {
        return err
    }

    return tx.Commit()
}

// For MEMORY engine (no transactions)
func (r *MySQLRepo) InsertToCache(ctx context.Context, key, value string) error {
    _, err := r.db.ExecContext(ctx, `
        REPLACE INTO cache_data (cache_key, cache_value, expires_at)
        VALUES (?, ?, DATE_ADD(NOW(), INTERVAL 1 HOUR))
    `, key, value)
    return err
}
```

### MY-318: Handle MEMORY Engine Restart

```go
func (r *MySQLRepo) EnsureCacheTable(ctx context.Context) error {
    // MEMORY tables are empty after restart
    _, err := r.db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS cache_data (
            cache_key VARCHAR(255) PRIMARY KEY,
            cache_value TEXT,
            expires_at TIMESTAMP
        ) ENGINE=MEMORY
    `)
    return err
}
```

## Best Practices

1. **Use InnoDB by default** - it's the modern choice
2. **Avoid MyISAM** - legacy, no crash recovery
3. **MEMORY for temp data** - but handle restarts
4. **ARCHIVE for logs** - space efficient
5. **Monitor buffer pool** - hit rate should be >99%
6. **innodb_file_per_table** - easier management
7. **Tune InnoDB for your workload** - buffer pool, log files
