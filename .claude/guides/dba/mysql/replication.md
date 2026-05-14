# MySQL Replication

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MYD-002 | Enable binary logging for replication | [Master Configuration](#master-configuration) |
| MYD-003 | Use ProxySQL for connection pooling | [Load Balancing](#load-balancing) |

---

## Replication Types

### MY-400: Statement-Based (SBR)
- Replicates SQL statements
- Compact logs
- May have inconsistencies with non-deterministic functions

### MY-401: Row-Based (RBR)
- Replicates actual row changes
- Larger logs
- More consistent
- **Recommended**

### MY-402: Mixed-Based (MBR)
- Automatically chooses between SBR and RBR

```ini
# my.cnf
[mysqld]
binlog_format = ROW  # Recommended
```

## Standard Replication (Async)

### MY-403: Master Configuration

```ini
# my.cnf on master
[mysqld]
server-id = 1
log_bin = mysql-bin
binlog_format = ROW

# For crash safety
sync_binlog = 1
innodb_flush_log_at_trx_commit = 1

# Binary log retention
binlog_expire_logs_seconds = 604800  # 7 days
```

### MY-404: Master Setup

```sql
-- Create replication user
CREATE USER 'replicator'@'%' IDENTIFIED BY 'password';
GRANT REPLICATION SLAVE ON *.* TO 'replicator'@'%';
FLUSH PRIVILEGES;

-- Get current position
SHOW MASTER STATUS;
-- Note File and Position
```

### MY-405: Replica Configuration (Standard)

```ini
# my.cnf on replica
[mysqld]
server-id = 2  # Must be unique
relay_log = mysql-relay
read_only = ON  # Prevent accidental writes
```

### MY-406: Replica Setup (Standard)

```sql
-- Configure replication source
CHANGE REPLICATION SOURCE TO
    SOURCE_HOST='master-host',
    SOURCE_USER='replicator',
    SOURCE_PASSWORD='password',
    SOURCE_LOG_FILE='mysql-bin.000001',
    SOURCE_LOG_POS=154;

-- Start replication
START REPLICA;

-- Check status
SHOW REPLICA STATUS\G
```

## GTID Replication (Recommended)

### MY-407: Benefits
- Easier failover
- No need to track log positions
- Automatic position tracking

### MY-408: Master Configuration (GTID)

```ini
[mysqld]
server-id = 1
log_bin = mysql-bin
binlog_format = ROW
gtid_mode = ON
enforce_gtid_consistency = ON
```

### MY-409: Replica Configuration (GTID)

```ini
[mysqld]
server-id = 2
relay_log = mysql-relay
read_only = ON
gtid_mode = ON
enforce_gtid_consistency = ON
```

### MY-410: Replica Setup with GTID

```sql
-- Configure with GTID
CHANGE REPLICATION SOURCE TO
    SOURCE_HOST='master-host',
    SOURCE_USER='replicator',
    SOURCE_PASSWORD='password',
    SOURCE_AUTO_POSITION=1;

START REPLICA;
```

## Semi-Synchronous Replication

Waits for at least one replica to acknowledge before commit.

### MY-411: Master Setup

```sql
-- Install plugin
INSTALL PLUGIN rpl_semi_sync_source SONAME 'semisync_source.so';

-- Enable
SET GLOBAL rpl_semi_sync_source_enabled = 1;
SET GLOBAL rpl_semi_sync_source_timeout = 10000;  -- 10 seconds

-- Make persistent
-- Add to my.cnf:
-- rpl_semi_sync_source_enabled = 1
```

### MY-412: Replica Setup

```sql
INSTALL PLUGIN rpl_semi_sync_replica SONAME 'semisync_replica.so';
SET GLOBAL rpl_semi_sync_replica_enabled = 1;
```

## Group Replication (MySQL InnoDB Cluster)

Multi-master, automatic failover cluster.

### MY-413: Configuration

```ini
[mysqld]
server-id = 1
gtid_mode = ON
enforce_gtid_consistency = ON
binlog_checksum = NONE
log_slave_updates = ON
binlog_format = ROW

# Group Replication
plugin_load_add = 'group_replication.so'
group_replication_group_name = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
group_replication_start_on_boot = OFF
group_replication_local_address = "node1:33061"
group_replication_group_seeds = "node1:33061,node2:33061,node3:33061"
group_replication_bootstrap_group = OFF
```

### MY-414: Bootstrap First Node

```sql
SET GLOBAL group_replication_bootstrap_group = ON;
START GROUP_REPLICATION;
SET GLOBAL group_replication_bootstrap_group = OFF;
```

### MY-415: Join Other Nodes

```sql
START GROUP_REPLICATION;
```

## Monitoring Replication

### MY-416: Check Replica Status

```sql
SHOW REPLICA STATUS\G

-- Key fields:
-- Replica_IO_Running: Yes
-- Replica_SQL_Running: Yes
-- Seconds_Behind_Source: 0
-- Last_Error:
```

### MY-417: Monitor Lag

```sql
-- On replica
SELECT
    TIMESTAMPDIFF(SECOND,
        (SELECT MAX(UPDATE_TIME) FROM information_schema.TABLES
         WHERE TABLE_SCHEMA = 'mydb'),
        NOW()
    ) AS estimated_lag;

-- Using pt-heartbeat
pt-heartbeat --update --database mydb --create-table
pt-heartbeat --monitor --database mydb
```

### MY-418: Check GTID Status

```sql
-- On master
SHOW MASTER STATUS;

-- On replica
SHOW REPLICA STATUS\G
-- Check: Retrieved_Gtid_Set, Executed_Gtid_Set
```

## Failover Operations

### MY-419: Promote Replica to Master

```sql
-- On replica (new master)
STOP REPLICA;
RESET REPLICA ALL;
SET GLOBAL read_only = OFF;

-- Update application connection strings
```

### MY-420: Point Other Replicas to New Master

```sql
-- GTID makes this simple
STOP REPLICA;
CHANGE REPLICATION SOURCE TO
    SOURCE_HOST='new-master',
    SOURCE_AUTO_POSITION=1;
START REPLICA;
```

## Go Integration

### MY-421: Read/Write Splitting

```go
import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

type DBCluster struct {
    master   *sql.DB
    replicas []*sql.DB
    counter  uint64
}

func NewDBCluster(masterDSN string, replicaDSNs []string) (*DBCluster, error) {
    master, err := sql.Open("mysql", masterDSN)
    if err != nil {
        return nil, err
    }

    replicas := make([]*sql.DB, len(replicaDSNs))
    for i, dsn := range replicaDSNs {
        replica, err := sql.Open("mysql", dsn)
        if err != nil {
            return nil, err
        }
        replicas[i] = replica
    }

    return &DBCluster{master: master, replicas: replicas}, nil
}

func (c *DBCluster) Master() *sql.DB {
    return c.master
}

func (c *DBCluster) Replica() *sql.DB {
    idx := atomic.AddUint64(&c.counter, 1) % uint64(len(c.replicas))
    return c.replicas[idx]
}

// Usage
func (r *Repository) GetUser(ctx context.Context, id int64) (*User, error) {
    // Read from replica
    row := r.cluster.Replica().QueryRowContext(ctx,
        "SELECT id, name, email FROM users WHERE id = ?", id)
    // ...
}

func (r *Repository) CreateUser(ctx context.Context, user *User) error {
    // Write to master
    _, err := r.cluster.Master().ExecContext(ctx,
        "INSERT INTO users (name, email) VALUES (?, ?)", user.Name, user.Email)
    return err
}
```

### MY-422: Health Check

```go
func (c *DBCluster) HealthCheck(ctx context.Context) error {
    // Check master
    if err := c.master.PingContext(ctx); err != nil {
        return fmt.Errorf("master unhealthy: %w", err)
    }

    // Check replicas
    for i, replica := range c.replicas {
        if err := replica.PingContext(ctx); err != nil {
            return fmt.Errorf("replica %d unhealthy: %w", i, err)
        }

        // Check replication lag
        var lag sql.NullInt64
        err := replica.QueryRowContext(ctx,
            "SELECT TIMESTAMPDIFF(SECOND, ts, NOW()) FROM heartbeat.heartbeat").Scan(&lag)
        if err == nil && lag.Valid && lag.Int64 > 30 {
            return fmt.Errorf("replica %d lag too high: %d seconds", i, lag.Int64)
        }
    }

    return nil
}
```

## ProxySQL for Automatic Routing (MY-423)

```ini
# ProxySQL configuration
mysql_servers =
(
    { hostgroup_id=10, hostname="master", port=3306 },
    { hostgroup_id=20, hostname="replica1", port=3306 },
    { hostgroup_id=20, hostname="replica2", port=3306 }
)

mysql_query_rules =
(
    { rule_id=1, match_pattern="^SELECT", destination_hostgroup=20 },
    { rule_id=2, match_pattern=".*", destination_hostgroup=10 }
)
```

```go
// Connect through ProxySQL
dsn := "user:password@tcp(proxysql:6033)/mydb"
db, err := sql.Open("mysql", dsn)
```

## Best Practices

1. **Use GTID replication** - easier failover
2. **Use ROW-based replication** - more consistent
3. **Enable semi-sync for critical data**
4. **Monitor replication lag** - alert if > threshold
5. **Test failover regularly**
6. **Use read replicas for read scaling**
7. **Separate write/read connections** in application
8. **Use ProxySQL** for automatic routing
9. **Keep binary logs** for point-in-time recovery
10. **Document your topology**
