# PostgreSQL Replication and Partitioning

## Quick Reference

This guide covers replication and partitioning strategies. Related deployment rules:

| ID | Rule | Context |
|----|------|---------|
| PGD-001 | Use persistent volumes for data | Deployment |
| PGD-003 | Configure WAL archiving for backup | Replication |

---

## Table Partitioning

### PG-400: Plan Partitioning Strategy for Large Tables

Partitioning divides large tables into smaller, manageable pieces while maintaining a unified interface.

**Benefits:**
- Query performance (partition pruning)
- Easier data management (drop old partitions)
- Parallel query execution
- Maintenance isolation (vacuum per partition)

### Partition Types

#### PG-401: Use Range Partitioning for Time-Series Data

Best for time-series data, sequential values.

```sql
-- Create partitioned table
CREATE TABLE events (
    id BIGSERIAL,
    event_time TIMESTAMPTZ NOT NULL,
    event_type VARCHAR(50),
    data JSONB,
    PRIMARY KEY (id, event_time)  -- Must include partition key
) PARTITION BY RANGE (event_time);

-- Create partitions
CREATE TABLE events_2024_01 PARTITION OF events
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

CREATE TABLE events_2024_02 PARTITION OF events
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');

-- Default partition (catches unmatched values)
CREATE TABLE events_default PARTITION OF events DEFAULT;
```

#### PG-402: Use List Partitioning for Categorical Data

Best for categorical data, discrete values.

```sql
CREATE TABLE orders (
    id BIGSERIAL,
    region VARCHAR(20) NOT NULL,
    customer_id INT,
    total DECIMAL(10,2),
    PRIMARY KEY (id, region)
) PARTITION BY LIST (region);

CREATE TABLE orders_europe PARTITION OF orders
    FOR VALUES IN ('DE', 'FR', 'GB', 'IT', 'ES');

CREATE TABLE orders_americas PARTITION OF orders
    FOR VALUES IN ('US', 'CA', 'MX', 'BR');

CREATE TABLE orders_asia PARTITION OF orders
    FOR VALUES IN ('JP', 'CN', 'KR', 'IN');

CREATE TABLE orders_other PARTITION OF orders DEFAULT;
```

#### PG-403: Use Hash Partitioning for Even Distribution

Best for even distribution when no natural partition key.

```sql
CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL,
    data JSONB
) PARTITION BY HASH (user_id);

-- Create 4 hash partitions
CREATE TABLE sessions_0 PARTITION OF sessions
    FOR VALUES WITH (MODULUS 4, REMAINDER 0);
CREATE TABLE sessions_1 PARTITION OF sessions
    FOR VALUES WITH (MODULUS 4, REMAINDER 1);
CREATE TABLE sessions_2 PARTITION OF sessions
    FOR VALUES WITH (MODULUS 4, REMAINDER 2);
CREATE TABLE sessions_3 PARTITION OF sessions
    FOR VALUES WITH (MODULUS 4, REMAINDER 3);
```

#### PG-404: Implement Multi-Level Partitioning for Complex Scenarios

```sql
-- First level: range by date
CREATE TABLE sales (
    id BIGSERIAL,
    sale_date DATE NOT NULL,
    region VARCHAR(20) NOT NULL,
    amount DECIMAL(10,2)
) PARTITION BY RANGE (sale_date);

-- Second level: list by region
CREATE TABLE sales_2024_01 PARTITION OF sales
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01')
    PARTITION BY LIST (region);

CREATE TABLE sales_2024_01_europe PARTITION OF sales_2024_01
    FOR VALUES IN ('DE', 'FR', 'GB');

CREATE TABLE sales_2024_01_americas PARTITION OF sales_2024_01
    FOR VALUES IN ('US', 'CA');
```

### Partition Management

#### PG-405: Automate Partition Management with pg_partman

```sql
-- Install extension
CREATE EXTENSION pg_partman;

-- Configure automatic partitioning
SELECT partman.create_parent(
    p_parent_table := 'public.events',
    p_control := 'event_time',
    p_type := 'native',
    p_interval := '1 month',
    p_premake := 3  -- Create 3 future partitions
);

-- Maintenance: create new partitions, drop old ones
SELECT partman.run_maintenance();

-- Configure retention (drop partitions older than 1 year)
UPDATE partman.part_config
SET retention = '1 year', retention_keep_table = false
WHERE parent_table = 'public.events';
```

#### PG-406: Manually Manage Partitions When Needed

```sql
-- Add new partition
CREATE TABLE events_2024_12 PARTITION OF events
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');

-- Detach partition (keeps data, removes from parent)
ALTER TABLE events DETACH PARTITION events_2024_01;

-- Attach existing table as partition
ALTER TABLE events ATTACH PARTITION events_old
    FOR VALUES FROM ('2023-01-01') TO ('2023-02-01');

-- Drop partition (deletes data)
DROP TABLE events_2024_01;

-- Detach and archive
ALTER TABLE events DETACH PARTITION events_2024_01;
ALTER TABLE events_2024_01 SET TABLESPACE archive_storage;
```

### PG-407: Enable and Verify Partition Pruning

PostgreSQL automatically excludes irrelevant partitions.

```sql
-- Enable partition pruning
SET enable_partition_pruning = on;

-- This query only scans events_2024_01
EXPLAIN SELECT * FROM events
WHERE event_time >= '2024-01-15' AND event_time < '2024-01-20';

-- Check if pruning is working
EXPLAIN (ANALYZE, COSTS OFF)
SELECT * FROM events WHERE event_time = '2024-01-15';
-- Should show only relevant partition in plan
```

### PG-408: Create Indexes on Partitioned Tables

```sql
-- Index on parent (automatically created on partitions)
CREATE INDEX idx_events_type ON events(event_type);

-- Index only on specific partition
CREATE INDEX idx_events_2024_01_type ON events_2024_01(event_type);

-- Unique index (must include partition key)
CREATE UNIQUE INDEX idx_events_unique ON events(id, event_time);
```

## Replication

### PG-409: Implement Streaming Replication for HA

Full database replica for HA and read scaling.

#### PG-410: Configure Primary Server for Replication

```ini
# postgresql.conf
wal_level = replica
max_wal_senders = 10
wal_keep_size = 1GB
hot_standby = on

# pg_hba.conf
host replication replicator 10.0.0.0/8 scram-sha-256
```

```sql
-- Create replication user
CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'secret';
```

#### PG-411: Set Up a Standby Replica

```bash
# Base backup from primary
pg_basebackup -h primary-host -D /var/lib/postgresql/data \
    -U replicator -P -R -X stream

# -R creates standby.signal and configures recovery
```

```ini
# postgresql.conf on replica
hot_standby = on
primary_conninfo = 'host=primary-host port=5432 user=replicator password=secret'
```

#### PG-412: Monitor Replication Status and Lag

```sql
-- On primary: check connected replicas
SELECT
    client_addr,
    state,
    sent_lsn,
    write_lsn,
    flush_lsn,
    replay_lsn,
    pg_wal_lsn_diff(sent_lsn, replay_lsn) AS lag_bytes
FROM pg_stat_replication;

-- On replica: check replication status
SELECT
    pg_is_in_recovery() AS is_replica,
    pg_last_wal_receive_lsn() AS receive_lsn,
    pg_last_wal_replay_lsn() AS replay_lsn,
    pg_last_xact_replay_timestamp() AS last_replay_time,
    EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp())) AS lag_seconds;
```

### Logical Replication

Selective table replication, cross-version, allows writes on subscriber.

#### Publisher Setup

```sql
-- Create publication
CREATE PUBLICATION my_pub FOR ALL TABLES;

-- Or specific tables
CREATE PUBLICATION orders_pub FOR TABLE orders, order_items;

-- With row filter (PostgreSQL 15+)
CREATE PUBLICATION active_users_pub FOR TABLE users
    WHERE (status = 'active');

-- Check publications
SELECT * FROM pg_publication;
SELECT * FROM pg_publication_tables;
```

#### PG-414: Create Subscriptions to Receive Replicated Data

```sql
-- Create subscription
CREATE SUBSCRIPTION my_sub
    CONNECTION 'host=publisher-host dbname=mydb user=replicator password=secret'
    PUBLICATION my_pub;

-- With options
CREATE SUBSCRIPTION my_sub
    CONNECTION '...'
    PUBLICATION my_pub
    WITH (
        copy_data = true,      -- Initial data copy
        create_slot = true,    -- Create replication slot
        synchronous_commit = off
    );

-- Check subscriptions
SELECT * FROM pg_subscription;
SELECT * FROM pg_stat_subscription;
```

#### PG-415: Manage Logical Replication Subscriptions

```sql
-- Refresh subscription (after adding tables to publication)
ALTER SUBSCRIPTION my_sub REFRESH PUBLICATION;

-- Disable/enable subscription
ALTER SUBSCRIPTION my_sub DISABLE;
ALTER SUBSCRIPTION my_sub ENABLE;

-- Drop subscription
DROP SUBSCRIPTION my_sub;

-- On publisher: drop publication
DROP PUBLICATION my_pub;
```

### Comparison

| Feature | Streaming | Logical |
|---------|-----------|---------|
| Granularity | Full database | Per table |
| Cross-version | No | Yes |
| Schema changes | Automatic | Manual |
| Write on replica | No | Yes |
| Use case | HA, read replicas | Data integration |

## High Availability Patterns

### PG-416: Configure Synchronous Replication for Data Safety

```ini
# postgresql.conf on primary
synchronous_standby_names = 'replica1, replica2'
synchronous_commit = on  # or remote_apply
```

**Commit levels:**
- `off` - No wait (may lose data)
- `local` - Wait for local flush
- `remote_write` - Wait for replica to receive
- `on` - Wait for replica to flush
- `remote_apply` - Wait for replica to apply

### PG-417: Implement Automatic Failover with Patroni

```yaml
# patroni.yml
scope: postgres-cluster
name: node1

restapi:
  listen: 0.0.0.0:8008

etcd:
  hosts: etcd1:2379,etcd2:2379,etcd3:2379

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
    postgresql:
      use_pg_rewind: true
      parameters:
        wal_level: replica
        max_wal_senders: 10

postgresql:
  listen: 0.0.0.0:5432
  data_dir: /var/lib/postgresql/data
  authentication:
    replication:
      username: replicator
      password: secret
```

### PG-418: Route Reads to Replicas and Writes to Primary

```go
// Go: route reads to replicas
type RoutingDB struct {
    primary *sqlx.DB
    replicas []*sqlx.DB
    counter  uint64
}

func (r *RoutingDB) Primary() *sqlx.DB {
    return r.primary
}

func (r *RoutingDB) Replica() *sqlx.DB {
    // Round-robin
    idx := atomic.AddUint64(&r.counter, 1) % uint64(len(r.replicas))
    return r.replicas[idx]
}

// Usage
func (repo *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
    // Read from replica
    return repo.db.Replica().GetContext(ctx, &user, "SELECT * FROM users WHERE id = $1", id)
}

func (repo *Repository) CreateUser(ctx context.Context, user *User) error {
    // Write to primary
    _, err := repo.db.Primary().ExecContext(ctx, "INSERT INTO users ...", ...)
    return err
}
```

## Go Integration for Partitioned Tables

### PG-419: Insert Data Into Partitioned Tables

```go
func (r *Repository) InsertEvent(ctx context.Context, event *Event) error {
    // Partition routing is automatic
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO events (event_time, event_type, data)
        VALUES ($1, $2, $3)
    `, event.Time, event.Type, event.Data)
    return err
}

func (r *Repository) GetEventsByDateRange(ctx context.Context, start, end time.Time) ([]Event, error) {
    // PostgreSQL automatically prunes partitions
    var events []Event
    err := r.db.SelectContext(ctx, &events, `
        SELECT * FROM events
        WHERE event_time >= $1 AND event_time < $2
        ORDER BY event_time
    `, start, end)
    return events, err
}
```

### PG-420: Automate Partition Creation and Cleanup in Go

```go
func (r *Repository) CreateMonthlyPartition(ctx context.Context, year int, month time.Month) error {
    partitionName := fmt.Sprintf("events_%d_%02d", year, month)

    start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
    end := start.AddDate(0, 1, 0)

    query := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s PARTITION OF events
        FOR VALUES FROM ('%s') TO ('%s')
    `, pq.QuoteIdentifier(partitionName),
       start.Format("2006-01-02"),
       end.Format("2006-01-02"))

    _, err := r.db.ExecContext(ctx, query)
    return err
}

func (r *Repository) DropOldPartitions(ctx context.Context, olderThan time.Time) error {
    rows, err := r.db.QueryContext(ctx, `
        SELECT inhrelid::regclass::text as partition_name
        FROM pg_inherits
        JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
        WHERE parent.relname = 'events'
    `)
    if err != nil {
        return err
    }
    defer rows.Close()

    for rows.Next() {
        var partitionName string
        if err := rows.Scan(&partitionName); err != nil {
            return err
        }

        // Parse partition date from name (events_2024_01)
        // and drop if older than threshold
        // ...
    }
    return rows.Err()
}
```

## Best Practices

### PG-421: Follow Partitioning Best Practices

1. **Choose partition key wisely** - most common query filter
2. **Keep partition count reasonable** - hundreds, not thousands
3. **Use DEFAULT partition** - catch unexpected values
4. **Automate partition creation** - pg_partman or cron job
5. **Include partition key in PRIMARY KEY**
6. **Monitor partition sizes** - rebalance if uneven

### PG-422: Follow Replication Best Practices

1. **Always have replicas** - for HA and read scaling
2. **Use synchronous replication** for critical data
3. **Monitor replication lag** - alert on high lag
4. **Test failover regularly**
5. **Use connection pooler** (PgBouncer) with replicas
6. **Logical replication** for selective sync or cross-version

### PG-423: Monitor Partitions and Replication Status

```sql
-- Partition sizes
SELECT
    parent.relname AS parent,
    child.relname AS partition,
    pg_size_pretty(pg_relation_size(child.oid)) AS size,
    pg_stat_get_live_tuples(child.oid) AS live_tuples
FROM pg_inherits
JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
JOIN pg_class child ON pg_inherits.inhrelid = child.oid
WHERE parent.relname = 'events'
ORDER BY pg_relation_size(child.oid) DESC;

-- Replication lag (primary)
SELECT
    application_name,
    client_addr,
    state,
    pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)) AS lag
FROM pg_stat_replication;
```
