# Redis Clustering and High Availability

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| RDD-003 | Use Redis Sentinel for HA | [Sentinel](#redis-sentinel) |
| RDD-001 | Set maxmemory and policy | [Cluster Configuration](#cluster-configuration) |

---

## Architecture Options

| Option | Failover | Scaling | Complexity |
|--------|----------|---------|------------|
| Standalone | Manual | None | Lowest |
| Sentinel | Automatic | Read replicas | Medium |
| Cluster | Automatic | Full sharding | Highest |

## Replication (Master-Replica)

### RD-401: Basic Setup

**Master (no config needed):**
```conf
# Just run normally
bind 0.0.0.0
port 6379
```

**Replica:**
```conf
replicaof master-host 6379

# Authentication (if master requires)
masterauth your-password

# Serve stale data during sync
replica-serve-stale-data yes

# Read-only replica
replica-read-only yes
```

### RD-402: Commands

```bash
# Make server a replica
redis-cli REPLICAOF master-host 6379

# Check replication status
redis-cli INFO replication

# Promote replica to master
redis-cli REPLICAOF NO ONE
```

### RD-403: Go Client with Replicas

```go
import "github.com/redis/go-redis/v9"

// Read from replicas, write to master
client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{":26379", ":26380", ":26381"},

    // Route reads to replicas
    RouteByLatency: true,  // or RouteRandomly: true

    // Pool settings
    PoolSize:     100,
    MinIdleConns: 10,
})

// Explicit read from replica
ctx := context.Background()
pipe := client.Pipeline()
pipe.Do(ctx, "READONLY")  // Force read from replica
pipe.Get(ctx, "key")
pipe.Exec(ctx)
```

## Redis Sentinel (HA)

### RD-404: Architecture

```
+----------+     +----------+     +----------+
| Sentinel |     | Sentinel |     | Sentinel |
|   :26379 |     |   :26380 |     |   :26381 |
+----------+     +----------+     +----------+
      |               |               |
      +-------+-------+-------+-------+
              |               |
         +--------+      +--------+
         | Master |<---->| Replica|
         |  :6379 |      |  :6380 |
         +--------+      +--------+
```

### RD-405: Sentinel Configuration

```conf
# sentinel.conf
port 26379

# Monitor master (name, host, port, quorum)
sentinel monitor mymaster 127.0.0.1 6379 2

# Failover if master unreachable for 30 seconds
sentinel down-after-milliseconds mymaster 30000

# Max replicas to sync simultaneously during failover
sentinel parallel-syncs mymaster 1

# Failover timeout
sentinel failover-timeout mymaster 180000

# Authentication
sentinel auth-pass mymaster your-password
```

### RD-406: Start Sentinel

```bash
redis-sentinel /etc/redis/sentinel.conf

# Or
redis-server /etc/redis/sentinel.conf --sentinel
```

### RD-407: Sentinel Commands

```bash
# Get master address
redis-cli -p 26379 SENTINEL get-master-addr-by-name mymaster

# List replicas
redis-cli -p 26379 SENTINEL replicas mymaster

# Check sentinel status
redis-cli -p 26379 SENTINEL master mymaster

# Force failover
redis-cli -p 26379 SENTINEL failover mymaster
```

### RD-408: Go Client with Sentinel

```go
client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{
        "sentinel-1:26379",
        "sentinel-2:26379",
        "sentinel-3:26379",
    },
    Password:      "your-password",
    SentinelPassword: "sentinel-password",  // If sentinel requires auth

    // Connection pool
    PoolSize:     100,
    MinIdleConns: 10,
    PoolTimeout:  4 * time.Second,

    // Timeouts
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,

    // Read from replicas
    RouteByLatency: true,
})
```

## Redis Cluster (Sharding)

### RD-409: Architecture

```
+------------+    +------------+    +------------+
| Node A     |    | Node B     |    | Node C     |
| Master     |    | Master     |    | Master     |
| Slots 0-5460   | Slots 5461-10922| Slots 10923-16383|
+------------+    +------------+    +------------+
      |                |                |
+------------+    +------------+    +------------+
| Node A'    |    | Node B'    |    | Node C'    |
| Replica    |    | Replica    |    | Replica    |
+------------+    +------------+    +------------+
```

### RD-410: Cluster Configuration

```conf
# redis.conf for each node
port 7000
cluster-enabled yes
cluster-config-file nodes.conf
cluster-node-timeout 5000
appendonly yes
```

### RD-411: Create Cluster

```bash
# Start 6 nodes (3 masters, 3 replicas)
redis-server --port 7000 --cluster-enabled yes --cluster-config-file nodes-7000.conf
redis-server --port 7001 --cluster-enabled yes --cluster-config-file nodes-7001.conf
# ... repeat for all nodes

# Create cluster
redis-cli --cluster create \
    127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002 \
    127.0.0.1:7003 127.0.0.1:7004 127.0.0.1:7005 \
    --cluster-replicas 1
```

### RD-412: Cluster Commands

```bash
# Cluster info
redis-cli -c -p 7000 CLUSTER INFO

# Node info
redis-cli -c -p 7000 CLUSTER NODES

# Slot info
redis-cli -c -p 7000 CLUSTER SLOTS

# Add node
redis-cli --cluster add-node new-host:7006 existing-host:7000

# Remove node
redis-cli --cluster del-node host:7000 node-id

# Reshard (move slots)
redis-cli --cluster reshard host:7000

# Rebalance
redis-cli --cluster rebalance host:7000
```

### RD-413: Go Client for Cluster

```go
client := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{
        "node1:7000",
        "node2:7001",
        "node3:7002",
    },
    Password: "your-password",

    // Pool settings per node
    PoolSize:     100,
    MinIdleConns: 10,

    // Timeouts
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,

    // Read from replicas
    ReadOnly:       true,   // All reads to replicas
    RouteByLatency: true,   // Route to lowest latency node
    RouteRandomly:  false,  // Or random replica

    // Retries
    MaxRetries: 3,
})

// Regular usage (client handles routing)
ctx := context.Background()
client.Set(ctx, "key", "value", 0)
client.Get(ctx, "key")
```

### RD-414: Hash Tags (Force Keys to Same Slot)

```go
// Keys with same {tag} go to same slot
client.Set(ctx, "user:{1234}:profile", "...", 0)
client.Set(ctx, "user:{1234}:settings", "...", 0)

// These can be used in MULTI/EXEC
pipe := client.TxPipeline()
pipe.Get(ctx, "user:{1234}:profile")
pipe.Get(ctx, "user:{1234}:settings")
pipe.Exec(ctx)
```

### RD-415: Cluster Limitations

```go
// Multi-key operations require same slot
// BAD: different slots
client.MGet(ctx, "key1", "key2", "key3")  // May fail

// GOOD: use hash tags
client.MGet(ctx, "{user}:key1", "{user}:key2", "{user}:key3")

// Transactions must be single slot
// BAD
tx := client.TxPipeline()
tx.Set(ctx, "key1", "v1", 0)
tx.Set(ctx, "key2", "v2", 0)  // May fail if different slot

// GOOD
tx := client.TxPipeline()
tx.Set(ctx, "{order}:key1", "v1", 0)
tx.Set(ctx, "{order}:key2", "v2", 0)  // Same slot
```

## Choosing the Right Option

### RD-416: Use Sentinel When:
- Need automatic failover
- Dataset fits on single node
- Read scaling with replicas is sufficient
- Don't need horizontal write scaling

### RD-417: Use Cluster When:
- Dataset exceeds single node memory
- Need horizontal write scaling
- Can work with hash tag constraints
- Need automatic sharding

## Monitoring

### RD-418: Sentinel Monitoring

```bash
# Check all sentinels
redis-cli -p 26379 SENTINEL sentinels mymaster

# Subscript to failover events
redis-cli -p 26379 PSUBSCRIBE *
```

### RD-419: Cluster Monitoring

```bash
# Cluster health
redis-cli --cluster check host:7000

# Fix cluster issues
redis-cli --cluster fix host:7000

# Cluster info
redis-cli -c -p 7000 CLUSTER INFO
# cluster_state:ok
# cluster_slots_assigned:16384
# cluster_slots_ok:16384
# cluster_known_nodes:6
```

### RD-420: Go Health Check

```go
func (r *RedisClusterRepo) HealthCheck(ctx context.Context) error {
    // Ping all nodes
    err := r.client.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
        return shard.Ping(ctx).Err()
    })
    return err
}

func (r *RedisClusterRepo) GetClusterInfo(ctx context.Context) (map[string]string, error) {
    info := make(map[string]string)

    err := r.client.ForEachShard(ctx, func(ctx context.Context, shard *redis.Client) error {
        nodeInfo, err := shard.ClusterInfo(ctx).Result()
        if err != nil {
            return err
        }
        // Parse node info...
        return nil
    })

    return info, err
}
```

## Best Practices

1. **Always use odd number of Sentinels** (3, 5, 7) for quorum
2. **Sentinel on separate machines** from Redis nodes
3. **Use hash tags** for related keys in cluster
4. **Monitor replication lag** - alert if lag > threshold
5. **Test failover** regularly in staging
6. **Set appropriate timeouts** - not too aggressive
7. **Use connection pooling** - especially with cluster
8. **Handle MOVED/ASK redirects** - go-redis handles automatically
9. **Plan shard count** - hard to change later
10. **Document your topology** - for operations team
