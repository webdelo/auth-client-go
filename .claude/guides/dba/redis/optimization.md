# Redis Optimization Guide

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| RD-001 | Use Pipeline for batch operations | [Batch Operations](#batch-operations) |
| RD-003 | Set TTL for cache keys | [Memory Optimization](#memory-optimization) |
| RDD-001 | Set maxmemory and policy | [Memory Configuration](#memory-configuration) |

---

## Memory Optimization

### RD-201: Memory Analysis

```bash
# Memory usage summary
redis-cli INFO memory

# Memory usage by key pattern
redis-cli --bigkeys

# Memory usage for specific key
redis-cli MEMORY USAGE mykey

# Sample keys for memory analysis
redis-cli MEMORY DOCTOR
```

### RD-202: Data Structure Selection

| Use Case | Best Structure | Memory | Notes |
|----------|---------------|--------|-------|
| Simple K/V | STRING | Base | Use for single values |
| Object fields | HASH | Less than multiple strings | Small hashes use ziplist |
| Unique items | SET | Higher for small sets | Consider INTSET for integers |
| Sorted data | ZSET | Higher | Use for leaderboards, ranges |
| Queue/Stack | LIST | Efficient | Use for FIFO/LIFO |
| Bitmap flags | STRING (bitmap) | Very low | 1 bit per flag |
| HyperLogLog | HLL | Fixed 12KB | Cardinality estimation |
| Geospatial | GEOHASH (ZSET) | Moderate | Radius queries |

### RD-203: Memory-Efficient Patterns

#### RD-204: Use Hashes for Objects

```bash
# Bad: multiple keys per object
SET user:1000:name "John"
SET user:1000:email "john@example.com"
SET user:1000:age "30"
# Memory: ~3 keys overhead

# Good: single hash
HSET user:1000 name "John" email "john@example.com" age "30"
# Memory: 1 key, ziplist encoding for small hashes
```

#### RD-205: Short Key Names

```bash
# Bad
SET user_profile_data_for_user_id_1000 "..."

# Good
SET u:1000:p "..."
```

#### RD-206: Integer IDs Instead of UUIDs

```bash
# Bad: 36 bytes per UUID
SET session:550e8400-e29b-41d4-a716-446655440000 "data"

# Good: 8 bytes for int
SET session:1000000001 "data"
```

#### RD-207: Compression for Large Values

```go
import (
    "compress/gzip"
    "bytes"
)

func (r *RedisRepo) SetCompressed(ctx context.Context, key string, data []byte) error {
    var buf bytes.Buffer
    w := gzip.NewWriter(&buf)
    w.Write(data)
    w.Close()

    return r.client.Set(ctx, key, buf.Bytes(), 0).Err()
}

func (r *RedisRepo) GetCompressed(ctx context.Context, key string) ([]byte, error) {
    compressed, err := r.client.Get(ctx, key).Bytes()
    if err != nil {
        return nil, err
    }

    reader, _ := gzip.NewReader(bytes.NewReader(compressed))
    defer reader.Close()

    var buf bytes.Buffer
    buf.ReadFrom(reader)
    return buf.Bytes(), nil
}
```

### RD-208: Memory Configuration

```conf
# redis.conf

# Max memory limit
maxmemory 4gb

# Eviction policy (when maxmemory reached)
maxmemory-policy allkeys-lru  # Evict any key using LRU
# Options:
# - noeviction: return errors (default)
# - allkeys-lru: evict any key, LRU
# - allkeys-lfu: evict any key, LFU (Redis 4.0+)
# - volatile-lru: evict keys with TTL, LRU
# - volatile-lfu: evict keys with TTL, LFU
# - allkeys-random: evict random keys
# - volatile-random: evict random keys with TTL
# - volatile-ttl: evict keys with shortest TTL

# LRU sample size (higher = more accurate, more CPU)
maxmemory-samples 10
```

## Command Optimization

### RD-209: Avoid Blocking Commands

```bash
# Bad: blocks all clients
KEYS user:*          # O(N) scan of all keys
SMEMBERS large_set   # Returns entire set
HGETALL large_hash   # Returns entire hash

# Good: use SCAN family
SCAN 0 MATCH user:* COUNT 100    # Cursor-based iteration
SSCAN large_set 0 COUNT 100
HSCAN large_hash 0 COUNT 100
```

### RD-210: Go Implementation of SCAN

```go
func (r *RedisRepo) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
    var keys []string
    var cursor uint64

    for {
        var batch []string
        var err error

        batch, cursor, err = r.client.Scan(ctx, cursor, pattern, 100).Result()
        if err != nil {
            return nil, err
        }

        keys = append(keys, batch...)

        if cursor == 0 {
            break
        }
    }

    return keys, nil
}
```

### RD-216: Pipeline for Multiple Commands

```go
// Bad: N round trips
for _, id := range userIDs {
    client.Get(ctx, fmt.Sprintf("user:%d", id))
}

// Good: 1 round trip with pipeline
pipe := client.Pipeline()
cmds := make([]*redis.StringCmd, len(userIDs))

for i, id := range userIDs {
    cmds[i] = pipe.Get(ctx, fmt.Sprintf("user:%d", id))
}

_, err := pipe.Exec(ctx)
if err != nil && err != redis.Nil {
    return err
}

for i, cmd := range cmds {
    val, _ := cmd.Result()
    // Process val...
}
```

### RD-217: Lua Scripts for Atomic Operations

```go
// Atomic increment with limit
var incrementWithLimit = redis.NewScript(`
    local current = tonumber(redis.call('GET', KEYS[1]) or '0')
    local limit = tonumber(ARGV[1])
    local increment = tonumber(ARGV[2])

    if current + increment > limit then
        return -1
    end

    return redis.call('INCRBY', KEYS[1], increment)
`)

func (r *RedisRepo) IncrementWithLimit(ctx context.Context, key string, limit, incr int64) (int64, error) {
    result, err := incrementWithLimit.Run(ctx, r.client, []string{key}, limit, incr).Int64()
    if err != nil {
        return 0, err
    }
    if result == -1 {
        return 0, ErrLimitExceeded
    }
    return result, nil
}
```

## Connection Optimization

### RD-211: Connection Pooling

```go
import "github.com/redis/go-redis/v9"

client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",

    // Pool settings
    PoolSize:     100,              // Max connections
    MinIdleConns: 10,               // Keep minimum idle
    PoolTimeout:  4 * time.Second,  // Wait for connection

    // Timeouts
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,

    // Retry
    MaxRetries:      3,
    MinRetryBackoff: 8 * time.Millisecond,
    MaxRetryBackoff: 512 * time.Millisecond,
})
```

### RD-212: Connection Health Check

```go
func (r *RedisRepo) HealthCheck(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    return r.client.Ping(ctx).Err()
}
```

## Data Pattern Optimization

### RD-213: Time-Series with Sorted Sets

```go
// Efficient time-series storage
func (r *RedisRepo) AddMetric(ctx context.Context, name string, value float64) error {
    timestamp := float64(time.Now().UnixNano())
    member := fmt.Sprintf("%d:%f", time.Now().UnixNano(), value)

    return r.client.ZAdd(ctx, "metrics:"+name, redis.Z{
        Score:  timestamp,
        Member: member,
    }).Err()
}

func (r *RedisRepo) GetMetrics(ctx context.Context, name string, start, end time.Time) ([]string, error) {
    return r.client.ZRangeByScore(ctx, "metrics:"+name, &redis.ZRangeBy{
        Min: fmt.Sprintf("%d", start.UnixNano()),
        Max: fmt.Sprintf("%d", end.UnixNano()),
    }).Result()
}

// Automatic cleanup of old data
func (r *RedisRepo) CleanOldMetrics(ctx context.Context, name string, olderThan time.Duration) error {
    threshold := float64(time.Now().Add(-olderThan).UnixNano())
    return r.client.ZRemRangeByScore(ctx, "metrics:"+name, "-inf", fmt.Sprintf("%f", threshold)).Err()
}
```

### RD-214: Rate Limiting with Sliding Window

```go
func (r *RedisRepo) IsRateLimited(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
    now := time.Now().UnixNano()
    windowStart := now - int64(window)

    pipe := r.client.Pipeline()

    // Remove old entries
    pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart))

    // Count current entries
    countCmd := pipe.ZCard(ctx, key)

    // Add current request
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})

    // Set expiry
    pipe.Expire(ctx, key, window)

    _, err := pipe.Exec(ctx)
    if err != nil {
        return false, err
    }

    count := countCmd.Val()
    return count >= int64(limit), nil
}
```

### RD-215: Caching with Read-Through Pattern

```go
func (r *RedisRepo) GetWithCache(ctx context.Context, key string, ttl time.Duration, fetch func() (string, error)) (string, error) {
    // Try cache first
    val, err := r.client.Get(ctx, key).Result()
    if err == nil {
        return val, nil
    }
    if err != redis.Nil {
        return "", err
    }

    // Cache miss - fetch from source
    val, err = fetch()
    if err != nil {
        return "", err
    }

    // Cache the result
    r.client.Set(ctx, key, val, ttl)

    return val, nil
}
```

## RD-216: Monitoring Commands

```bash
# Real-time stats
redis-cli INFO stats
redis-cli INFO clients
redis-cli INFO memory

# Slow log (commands taking > 10ms by default)
redis-cli SLOWLOG GET 10

# Configure slow log
CONFIG SET slowlog-log-slower-than 10000  # microseconds
CONFIG SET slowlog-max-len 128

# Monitor all commands (DANGEROUS in production)
redis-cli MONITOR

# Client list
redis-cli CLIENT LIST

# Memory stats per key type
redis-cli MEMORY STATS
```

## RD-217: Configuration Tuning

```conf
# redis.conf

# Disable persistence for pure cache (faster)
save ""
appendonly no

# TCP keepalive (seconds)
tcp-keepalive 300

# Disable commands that shouldn't be used in production
rename-command KEYS ""
rename-command FLUSHALL ""
rename-command FLUSHDB ""
rename-command DEBUG ""

# Background save settings
stop-writes-on-bgsave-error yes
rdbcompression yes
rdbchecksum yes

# Replication
replica-serve-stale-data yes
replica-read-only yes
repl-diskless-sync yes

# Lazy freeing (async deletion)
lazyfree-lazy-eviction yes
lazyfree-lazy-expire yes
lazyfree-lazy-server-del yes
replica-lazy-flush yes
```

## Performance Checklist

1. **Use appropriate data structures** - Hash for objects, not multiple keys
2. **Pipeline bulk operations** - Reduce round trips
3. **Avoid KEYS command** - Use SCAN instead
4. **Set proper TTL** - Prevent memory growth
5. **Use Lua for atomicity** - Instead of multiple commands
6. **Monitor slow log** - Identify bottlenecks
7. **Tune connection pool** - Match your concurrency
8. **Enable lazy freeing** - Avoid blocking on DEL
9. **Use compression** - For large values
10. **Short key names** - Save memory on key storage
