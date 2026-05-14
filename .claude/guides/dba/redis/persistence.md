# Redis Persistence

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| RDD-002 | Enable persistence (AOF/RDB) | [RDB](#rdb-snapshotting), [AOF](#aof-append-only-file) |
| RDD-004 | Monitor memory usage | [Monitoring](#monitoring) |

---

## Overview

Redis offers two persistence mechanisms:
- **RDB (Redis Database)** - Point-in-time snapshots
- **AOF (Append Only File)** - Log of all write operations

## RDB (Snapshotting)

### RD-301: How It Works

1. Redis forks a child process
2. Child writes all data to temp RDB file
3. When complete, replaces old RDB file
4. Uses copy-on-write for memory efficiency

### RD-302: Configuration

```conf
# redis.conf

# Save snapshot if N keys changed in M seconds
save 900 1      # 1 key changed in 15 minutes
save 300 10     # 10 keys changed in 5 minutes
save 60 10000   # 10000 keys changed in 1 minute

# Disable RDB
save ""

# File name and location
dbfilename dump.rdb
dir /var/lib/redis

# Compression
rdbcompression yes

# Checksum (integrity check)
rdbchecksum yes

# Stop writes if RDB save fails
stop-writes-on-bgsave-error yes
```

### RD-303: Manual Snapshot Commands

```bash
# Synchronous save (blocks Redis)
redis-cli SAVE

# Background save (non-blocking)
redis-cli BGSAVE

# Check last save time
redis-cli LASTSAVE

# Check save status
redis-cli INFO persistence
```

### RD-304: Pros and Cons

| Pros | Cons |
|------|------|
| Compact single file | Data loss possible (between saves) |
| Fast restarts | Fork can be slow for large datasets |
| Perfect for backups | Memory spike during fork (copy-on-write) |
| Good for disaster recovery | |

## AOF (Append Only File)

### RD-305: How It Works

1. Every write command appended to AOF
2. File can be rewritten to compact
3. On restart, commands replayed

### RD-306: Configuration

```conf
# redis.conf

# Enable AOF
appendonly yes

# File name
appendfilename "appendonly.aof"

# Fsync policy
appendfsync everysec   # Recommended: sync every second
# appendfsync always   # Sync on every write (slowest, safest)
# appendfsync no       # Let OS handle (fastest, least safe)

# Rewrite settings
auto-aof-rewrite-percentage 100  # Rewrite when AOF is 2x size since last rewrite
auto-aof-rewrite-min-size 64mb   # Minimum size to trigger rewrite

# Truncate corrupted AOF on startup
aof-load-truncated yes

# Use RDB preamble in AOF (Redis 4.0+)
aof-use-rdb-preamble yes
```

### RD-307: Fsync Options Comparison

| Policy | Durability | Performance | Data Loss Risk |
|--------|------------|-------------|----------------|
| always | Best | Slowest | ~1 command |
| everysec | Good | Good | ~1 second |
| no | Worst | Best | Up to 30 seconds |

### RD-308: Manual AOF Commands

```bash
# Trigger AOF rewrite
redis-cli BGREWRITEAOF

# Check AOF status
redis-cli INFO persistence

# Fix corrupted AOF
redis-check-aof --fix appendonly.aof
```

### RD-309: Pros and Cons

| Pros | Cons |
|------|------|
| More durable | Larger file size |
| Append-only (no corruption on crash) | Slower than RDB |
| Human-readable | Replay can be slow |
| Auto-rewrite keeps size manageable | |

## Hybrid Persistence (Redis 4.0+)

Combines RDB and AOF advantages.

```conf
# Enable hybrid mode
aof-use-rdb-preamble yes
appendonly yes
```

### RD-310: How It Works

1. AOF rewrite creates RDB snapshot as preamble
2. Subsequent commands appended in AOF format
3. Faster restart (RDB load) + better durability (AOF)

## Recommended Configurations

### RD-311: Cache Only (No Persistence)

```conf
save ""
appendonly no
```

### RD-312: Maximum Durability

```conf
save 900 1
save 300 10
save 60 10000

appendonly yes
appendfsync always
aof-use-rdb-preamble yes
```

### RD-313: Balanced (Recommended for Most Cases)

```conf
# RDB for backups
save 900 1
save 300 10
save 60 10000

# AOF for durability
appendonly yes
appendfsync everysec
aof-use-rdb-preamble yes

# Reasonable rewrite thresholds
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb
```

### RD-314: High-Performance with Some Durability

```conf
# Less frequent RDB
save 3600 1

# AOF with OS fsync
appendonly yes
appendfsync no
aof-use-rdb-preamble yes
```

## Backup Strategies

### RD-315: RDB Backup

```bash
#!/bin/bash
# backup_redis.sh

REDIS_DIR=/var/lib/redis
BACKUP_DIR=/backup/redis
DATE=$(date +%Y%m%d_%H%M%S)

# Trigger background save
redis-cli BGSAVE

# Wait for completion
while [ $(redis-cli LASTSAVE) == $(redis-cli LASTSAVE) ]; do
    sleep 1
done

# Copy RDB file
cp ${REDIS_DIR}/dump.rdb ${BACKUP_DIR}/dump_${DATE}.rdb

# Compress
gzip ${BACKUP_DIR}/dump_${DATE}.rdb

# Remove old backups (keep 7 days)
find ${BACKUP_DIR} -name "dump_*.rdb.gz" -mtime +7 -delete
```

### RD-316: AOF Backup

```bash
#!/bin/bash
# AOF is harder to backup (constantly written)
# Best practice: use RDB for backups

# If you must backup AOF:
redis-cli BGREWRITEAOF
sleep 5  # Wait for rewrite

# Copy (might be inconsistent)
cp /var/lib/redis/appendonly.aof /backup/redis/
```

### RD-317: Restore from Backup

```bash
# Stop Redis
systemctl stop redis

# Restore RDB
cp /backup/redis/dump_20240101.rdb /var/lib/redis/dump.rdb

# Or restore AOF
cp /backup/redis/appendonly.aof /var/lib/redis/

# Fix permissions
chown redis:redis /var/lib/redis/*

# Start Redis
systemctl start redis
```

## Go Integration

### RD-318: Check Persistence Status

```go
func (r *RedisRepo) GetPersistenceInfo(ctx context.Context) (*PersistenceInfo, error) {
    info, err := r.client.Info(ctx, "persistence").Result()
    if err != nil {
        return nil, err
    }

    // Parse info string
    result := &PersistenceInfo{}
    for _, line := range strings.Split(info, "\n") {
        if strings.HasPrefix(line, "rdb_last_save_time:") {
            timestamp, _ := strconv.ParseInt(strings.TrimPrefix(line, "rdb_last_save_time:"), 10, 64)
            result.RDBLastSave = time.Unix(timestamp, 0)
        }
        if strings.HasPrefix(line, "aof_enabled:") {
            result.AOFEnabled = strings.TrimPrefix(line, "aof_enabled:") == "1"
        }
        // ... parse other fields
    }

    return result, nil
}

type PersistenceInfo struct {
    RDBLastSave  time.Time
    AOFEnabled   bool
    AOFRewriting bool
    AOFSize      int64
}
```

### RD-319: Trigger Manual Backup

```go
func (r *RedisRepo) TriggerBackup(ctx context.Context) error {
    // Get last save time before
    before, err := r.client.LastSave(ctx).Result()
    if err != nil {
        return err
    }

    // Trigger background save
    if err := r.client.BgSave(ctx).Err(); err != nil {
        return fmt.Errorf("bgsave failed: %w", err)
    }

    // Wait for completion (poll)
    timeout := time.After(5 * time.Minute)
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-timeout:
            return errors.New("bgsave timeout")
        case <-ticker.C:
            after, err := r.client.LastSave(ctx).Result()
            if err != nil {
                return err
            }
            if after.After(before) {
                return nil  // Save completed
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

## RD-320: Monitoring

```bash
# Check persistence status
redis-cli INFO persistence

# Key metrics to monitor:
# - rdb_last_save_time: Unix timestamp of last RDB save
# - rdb_bgsave_in_progress: 1 if BGSAVE running
# - aof_rewrite_in_progress: 1 if AOF rewrite running
# - aof_current_size: Current AOF file size
# - aof_base_size: AOF size at last rewrite

# Memory usage during fork
redis-cli INFO memory | grep used_memory
```

## Troubleshooting

### RD-321: Fork Failing (Not Enough Memory)

```bash
# Check available memory
free -h

# Solutions:
# 1. Reduce dataset size
# 2. Add more RAM
# 3. Enable memory overcommit
echo 1 > /proc/sys/vm/overcommit_memory
# Or in /etc/sysctl.conf
vm.overcommit_memory = 1
```

### RD-322: AOF Growing Too Large

```bash
# Manual rewrite
redis-cli BGREWRITEAOF

# Adjust thresholds
CONFIG SET auto-aof-rewrite-percentage 50
CONFIG SET auto-aof-rewrite-min-size 32mb
```

### RD-323: Slow Startup from Large AOF

```bash
# Use hybrid persistence (RDB preamble)
CONFIG SET aof-use-rdb-preamble yes

# Or if AOF is corrupted, fix it
redis-check-aof --fix appendonly.aof
```
