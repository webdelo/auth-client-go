# Redis Best Practices and Standards

This document contains Redis deployment rules for DevOps tasks. Each rule has a unique identifier (RDD-XXXX) for reference in code reviews and task execution.

> **Related**: For Redis code patterns, see `dba/redis/`
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми Redis задачами
**Текущая версия Redis**: 7.4+ (2025)

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| RDD-001 | Set maxmemory and policy | ✅ | [Memory Management](#rdd-001-set-maxmemory-and-policy) |
| RDD-002 | Enable persistence (AOF/RDB) | ✅ | [Persistence](#rdd-002-enable-persistence-aofrdb) |
| RDD-003 | Use Redis Sentinel for HA | ✅ | [High Availability](#rdd-003-use-redis-sentinel-for-ha) |
| RDD-004 | Monitor memory usage | | [Monitoring](#rdd-004-monitor-memory-usage) |

---

## Содержание

1. [Installation & Setup](#installation--setup)
2. [Configuration Best Practices](#configuration-best-practices)
3. [Security Standards](#security-standards)
4. [Load Balancing & High Availability](#load-balancing--high-availability)
5. [Performance Optimization](#performance-optimization)
6. [Monitoring & Observability](#monitoring--observability)
7. [Backup & Recovery](#backup--recovery)
8. [Production Patterns](#production-patterns)
9. [Common Anti-Patterns](#common-anti-patterns)

---

## Installation & Setup

### RDD-001: 1. Production Installation (Recommended)

#### Docker (ОБЯЗАТЕЛЬНО для development/testing)

```dockerfile
# syntax=docker/dockerfile:1
FROM redis:7.4-alpine AS production

# Создаём non-root user
RUN addgroup -g 999 redis && \
    adduser -D -u 999 -G redis redis

# Custom конфигурация
COPY redis.conf /usr/local/etc/redis/redis.conf

# Директории для данных
RUN mkdir -p /data && \
    chown -R redis:redis /data /usr/local/etc/redis

# Security: read-only root filesystem
VOLUME /data

USER redis

EXPOSE 6379

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD redis-cli ping | grep -q PONG || exit 1

CMD ["redis-server", "/usr/local/etc/redis/redis.conf"]
```

#### Docker Compose

```yaml
services:
  redis:
    image: redis:7.4-alpine
    restart: unless-stopped
    command: redis-server /usr/local/etc/redis/redis.conf
    volumes:
      - redis_data:/data
      - ./redis.conf:/usr/local/etc/redis/redis.conf:ro
    ports:
      - "127.0.0.1:6379:6379"  # Bind только к localhost
    networks:
      - backend
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 30s
      timeout: 3s
      retries: 3
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'
        reservations:
          memory: 256M

volumes:
  redis_data:

networks:
  backend:
    internal: true
```

#### Kubernetes StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  namespace: production
spec:
  serviceName: redis
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 999
        fsGroup: 999
      containers:
      - name: redis
        image: redis:7.4-alpine
        command:
        - redis-server
        - /etc/redis/redis.conf
        ports:
        - containerPort: 6379
          name: redis
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          exec:
            command:
            - redis-cli
            - ping
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - redis-cli
            - ping
          initialDelaySeconds: 5
          periodSeconds: 5
        volumeMounts:
        - name: redis-data
          mountPath: /data
        - name: redis-config
          mountPath: /etc/redis
          readOnly: true
      volumes:
      - name: redis-config
        configMap:
          name: redis-config
  volumeClaimTemplates:
  - metadata:
      name: redis-data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: fast-ssd
      resources:
        requests:
          storage: 10Gi
```

---

## Configuration Best Practices

### RDD-002: 1. Production redis.conf (CRITICAL)

```conf
# === Network ===
bind 127.0.0.1 ::1
protected-mode yes
port 6379
tcp-backlog 511
timeout 300
tcp-keepalive 300

# === Security ===
requirepass YOUR_STRONG_PASSWORD_HERE
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command KEYS ""
rename-command CONFIG "CONFIG_ADMIN_ONLY"

# === Memory Management ===
maxmemory 512mb
maxmemory-policy allkeys-lru
maxmemory-samples 5

# === Persistence - AOF (RECOMMENDED) ===
appendonly yes
appendfilename "appendonly.aof"
appendfsync everysec
no-appendfsync-on-rewrite no
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb
aof-use-rdb-preamble yes

# === Persistence - RDB (Backup) ===
save 900 1
save 300 10
save 60 10000
stop-writes-on-bgsave-error yes
rdbcompression yes
rdbchecksum yes
dbfilename dump.rdb
dir /data

# === Replication ===
replica-read-only yes
repl-diskless-sync yes
repl-diskless-sync-delay 5

# === Slow Log ===
slowlog-log-slower-than 10000
slowlog-max-len 128

# === Limits ===
maxclients 10000

# === Logging ===
loglevel notice
logfile ""

# === Threading (Redis 6+) ===
io-threads 4
io-threads-do-reads yes
```

### RDD-003: 2. Memory Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| **noeviction** | Return errors when memory full | Never lose data |
| **allkeys-lru** | Evict least recently used keys | General cache |
| **allkeys-lfu** | Evict least frequently used | Hot data cache |
| **volatile-lru** | Evict LRU with TTL only | Mixed workload |
| **volatile-ttl** | Evict keys with shortest TTL | Time-sensitive |

**Рекомендация**: `allkeys-lru` для cache, `noeviction` для persistent data.

### RDD-004: 3. Persistence Strategies

| Mode | Durability | Performance | Recovery |
|------|------------|-------------|----------|
| **RDB only** | Low | High | Fast |
| **AOF everysec** | Medium | Medium | Complete |
| **AOF always** | High | Low | Complete |
| **RDB + AOF** | High | Medium | Fast + Complete |

**Production**: Используй **RDB + AOF** (`aof-use-rdb-preamble yes`).

#### AOF vs RDB

```conf
# AOF - Append Only File (RECOMMENDED)
appendonly yes
appendfsync everysec  # Good balance: fsync every second

# RDB - Snapshot
save 900 1      # Save after 900s if 1 key changed
save 300 10     # Save after 300s if 10 keys changed
save 60 10000   # Save after 60s if 10000 keys changed
```

**Hybrid Mode** (Redis 7+):
```conf
aof-use-rdb-preamble yes  # RDB snapshot + AOF incremental
```

---

## Security Standards

### RDD-005: 1. Authentication & Authorization (CRITICAL)

#### Basic AUTH

```conf
# redis.conf
requirepass YOUR_STRONG_PASSWORD_HERE
```

**НИКОГДА** не используй default password или слабые пароли!

```bash
# Generate strong password
openssl rand -base64 32
```

#### ACL (Access Control Lists) - Redis 6+

```conf
# redis.conf
aclfile /etc/redis/users.acl
```

**users.acl**:
```acl
# Admin user
user admin on >ADMIN_PASSWORD ~* &* +@all

# Application user (read/write specific keys)
user myapp on >APP_PASSWORD ~app:* +@read +@write +@hash +@string -@dangerous

# Read-only user
user readonly on >READONLY_PASSWORD ~* +@read

# Disable default user (CRITICAL)
user default off
```

**Dangerous Commands**:
- `FLUSHDB`, `FLUSHALL` - удаление всех данных
- `KEYS` - блокирует production
- `CONFIG` - изменение конфигурации
- `SHUTDOWN` - остановка сервера
- `BGSAVE`, `BGREWRITEAOF` - манипуляция персистентностью

#### Connection Example с ACL

```go
// Go client с ACL
import "github.com/redis/go-redis/v9"

client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Username: "myapp",
    Password: "APP_PASSWORD",
    DB:       0,
})
```

### RDD-006: 2. Network Security

#### TLS/SSL (ОБЯЗАТЕЛЬНО для production)

```conf
# redis.conf
port 0                    # Disable non-TLS
tls-port 6379
tls-cert-file /path/to/redis.crt
tls-key-file /path/to/redis.key
tls-ca-cert-file /path/to/ca.crt
tls-auth-clients optional
```

**Docker с TLS**:
```yaml
services:
  redis:
    volumes:
      - ./certs:/etc/redis/certs:ro
    environment:
      - REDIS_TLS_ENABLED=yes
      - REDIS_TLS_CERT_FILE=/etc/redis/certs/redis.crt
      - REDIS_TLS_KEY_FILE=/etc/redis/certs/redis.key
```

#### Firewall Rules

```bash
# Only allow from application subnet
iptables -A INPUT -p tcp --dport 6379 -s 10.0.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 6379 -j DROP
```

#### Kubernetes NetworkPolicy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: redis-netpol
spec:
  podSelector:
    matchLabels:
      app: redis
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: backend  # Only backend can access
    ports:
    - protocol: TCP
      port: 6379
```

### RDD-007: 3. Secrets Management

#### Kubernetes Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: redis-auth
type: Opaque
stringData:
  password: YOUR_STRONG_PASSWORD
```

**ConfigMap для redis.conf**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: redis-config
data:
  redis.conf: |
    bind 0.0.0.0
    protected-mode yes
    requirepass ${REDIS_PASSWORD}
    # ... rest of config
```

#### Docker Secrets

```yaml
services:
  redis:
    secrets:
      - redis_password
    environment:
      - REDIS_PASSWORD_FILE=/run/secrets/redis_password

secrets:
  redis_password:
    file: ./secrets/redis_password.txt
```

---

## Load Balancing & High Availability

### RDD-008: 1. Redis Sentinel (Automatic Failover)

**Architecture**: 1 Master + N Replicas + 3+ Sentinels

#### Sentinel Configuration

```conf
# sentinel.conf
port 26379
sentinel monitor mymaster 127.0.0.1 6379 2
sentinel auth-pass mymaster YOUR_PASSWORD
sentinel down-after-milliseconds mymaster 5000
sentinel parallel-syncs mymaster 1
sentinel failover-timeout mymaster 10000
```

#### Docker Compose - Sentinel Setup

```yaml
services:
  redis-master:
    image: redis:7.4-alpine
    command: redis-server /etc/redis/redis.conf
    volumes:
      - ./redis-master.conf:/etc/redis/redis.conf:ro
      - redis-master-data:/data
    networks:
      - redis

  redis-replica-1:
    image: redis:7.4-alpine
    command: redis-server /etc/redis/redis.conf
    volumes:
      - ./redis-replica.conf:/etc/redis/redis.conf:ro
      - redis-replica-1-data:/data
    depends_on:
      - redis-master
    networks:
      - redis

  redis-replica-2:
    image: redis:7.4-alpine
    command: redis-server /etc/redis/redis.conf
    volumes:
      - ./redis-replica.conf:/etc/redis/redis.conf:ro
      - redis-replica-2-data:/data
    depends_on:
      - redis-master
    networks:
      - redis

  sentinel-1:
    image: redis:7.4-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel.conf:/etc/redis/sentinel.conf:ro
    depends_on:
      - redis-master
      - redis-replica-1
      - redis-replica-2
    networks:
      - redis

  sentinel-2:
    image: redis:7.4-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel.conf:/etc/redis/sentinel.conf:ro
    depends_on:
      - redis-master
    networks:
      - redis

  sentinel-3:
    image: redis:7.4-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel.conf:/etc/redis/sentinel.conf:ro
    depends_on:
      - redis-master
    networks:
      - redis

volumes:
  redis-master-data:
  redis-replica-1-data:
  redis-replica-2-data:

networks:
  redis:
```

**redis-replica.conf**:
```conf
replicaof redis-master 6379
masterauth YOUR_PASSWORD
replica-read-only yes
```

#### Go Client с Sentinel

```go
import "github.com/redis/go-redis/v9"

client := redis.NewFailoverClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{
        "sentinel-1:26379",
        "sentinel-2:26379",
        "sentinel-3:26379",
    },
    Password: "YOUR_PASSWORD",
    DB:       0,
})
```

**Sentinel Benefits**:
- Automatic failover (master goes down → promote replica)
- Health monitoring
- Configuration provider

**Limitations**:
- Manual sharding (no auto-partitioning)
- Split-brain risk during network partitions

### RDD-009: 2. Redis Cluster (Sharding + HA)

**Architecture**: 3+ Master Nodes + Replicas

#### Cluster Benefits

- **Automatic Sharding**: 16384 hash slots distributed across nodes
- **High Availability**: Each master has replicas
- **Horizontal Scaling**: Add nodes to increase capacity

#### Kubernetes - Redis Cluster

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis-cluster
spec:
  serviceName: redis-cluster
  replicas: 6  # 3 masters + 3 replicas
  selector:
    matchLabels:
      app: redis-cluster
  template:
    metadata:
      labels:
        app: redis-cluster
    spec:
      containers:
      - name: redis
        image: redis:7.4-alpine
        command:
        - redis-server
        - /etc/redis/redis.conf
        - --cluster-enabled
        - "yes"
        - --cluster-config-file
        - nodes.conf
        - --cluster-node-timeout
        - "5000"
        ports:
        - containerPort: 6379
          name: client
        - containerPort: 16379
          name: gossip
        volumeMounts:
        - name: data
          mountPath: /data
        - name: config
          mountPath: /etc/redis
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

#### Initialize Cluster

```bash
# После запуска всех pods
redis-cli --cluster create \
  redis-cluster-0.redis-cluster:6379 \
  redis-cluster-1.redis-cluster:6379 \
  redis-cluster-2.redis-cluster:6379 \
  redis-cluster-3.redis-cluster:6379 \
  redis-cluster-4.redis-cluster:6379 \
  redis-cluster-5.redis-cluster:6379 \
  --cluster-replicas 1
```

#### Go Client с Cluster

```go
import "github.com/redis/go-redis/v9"

client := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{
        "redis-cluster-0:6379",
        "redis-cluster-1:6379",
        "redis-cluster-2:6379",
    },
    Password: "YOUR_PASSWORD",
})
```

**Cluster Gotchas**:
- Multi-key operations требуют hash tags: `{user:123}:profile`, `{user:123}:settings`
- `EVAL` scripts должны работать с одним hash slot
- Не поддерживает multiple databases (всегда DB 0)

### RDD-010: 3. Client-Side Sharding

**Pattern**: Application distributes keys across multiple Redis instances.

```go
type ShardedRedis struct {
    clients []*redis.Client
}

func (s *ShardedRedis) getShard(key string) *redis.Client {
    hash := crc32.ChecksumIEEE([]byte(key))
    index := int(hash) % len(s.clients)
    return s.clients[index]
}

func (s *ShardedRedis) Get(ctx context.Context, key string) (string, error) {
    client := s.getShard(key)
    return client.Get(ctx, key).Result()
}
```

**Pros**:
- Simple implementation
- Full control over sharding logic

**Cons**:
- No automatic rebalancing
- Application complexity
- No built-in failover

### RDD-011: 4. Proxy Solutions

#### Twemproxy (nutcracker)

**Features**:
- Connection pooling
- Automatic sharding
- Protocol support: Redis, Memcached

```yaml
# nutcracker.yml
redis_pool:
  listen: 0.0.0.0:6379
  hash: fnv1a_64
  distribution: ketama
  auto_eject_hosts: true
  timeout: 400
  redis: true
  servers:
   - 127.0.0.1:6380:1
   - 127.0.0.1:6381:1
   - 127.0.0.1:6382:1
```

#### Envoy Redis Proxy

```yaml
# envoy.yaml
static_resources:
  listeners:
  - name: redis_listener
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 6379
    filter_chains:
    - filters:
      - name: envoy.filters.network.redis_proxy
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.redis_proxy.v3.RedisProxy
          stat_prefix: redis
          settings:
            op_timeout: 5s
          prefix_routes:
            routes:
            - prefix: "/"
              cluster: redis_cluster
```

### RDD-012: 5. Load Balancing Pattern Matrix

| Pattern | Sharding | Failover | Complexity | Use Case |
|---------|----------|----------|------------|----------|
| **Single Instance** | No | No | Low | Dev/Testing |
| **Master-Replica** | No | Manual | Low | Read scaling |
| **Sentinel** | No | Auto | Medium | HA without sharding |
| **Cluster** | Auto | Auto | High | Horizontal scaling + HA |
| **Client Sharding** | Manual | Manual | Medium | Custom partitioning |
| **Proxy (Twemproxy)** | Auto | No | Medium | Connection pooling |
| **Proxy (Envoy)** | Auto | Yes | High | Service mesh integration |

**Production Recommendations**:
- **< 10 GB data, high availability**: Redis Sentinel
- **> 10 GB data, horizontal scaling**: Redis Cluster
- **Legacy applications**: Twemproxy/Envoy proxy
- **Microservices**: Service mesh (Envoy) + Sentinel/Cluster

---

## Performance Optimization

### RDD-013: 1. Connection Pooling

**CRITICAL**: Всегда используй connection pooling!

```go
// Go - redis/go-redis/v9
client := redis.NewClient(&redis.Options{
    Addr:         "localhost:6379",
    PoolSize:     10,           // Max connections
    MinIdleConns: 5,            // Min idle connections
    MaxRetries:   3,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
})
```

### RDD-014: 2. Pipelining

**Without Pipeline** (4 round trips):
```go
client.Set(ctx, "key1", "val1", 0)
client.Set(ctx, "key2", "val2", 0)
client.Set(ctx, "key3", "val3", 0)
client.Set(ctx, "key4", "val4", 0)
```

**With Pipeline** (1 round trip):
```go
pipe := client.Pipeline()
pipe.Set(ctx, "key1", "val1", 0)
pipe.Set(ctx, "key2", "val2", 0)
pipe.Set(ctx, "key3", "val3", 0)
pipe.Set(ctx, "key4", "val4", 0)
_, err := pipe.Exec(ctx)
```

**Performance**: 5-10x faster для bulk operations.

### RDD-015: 3. Efficient Data Structures

| Structure | Use Case | Memory | Performance |
|-----------|----------|--------|-------------|
| **String** | Cache values, counters | Medium | O(1) |
| **Hash** | Objects (user profiles) | Low | O(1) |
| **List** | Queues, recent items | Medium | O(1) head/tail |
| **Set** | Unique items, tags | Medium | O(1) add/remove |
| **Sorted Set** | Leaderboards, rankings | High | O(log N) |

**ИЗБЕГАЙ**:
```go
// BAD: Multiple keys per object
client.Set(ctx, "user:123:name", "John", 0)
client.Set(ctx, "user:123:email", "john@example.com", 0)
client.Set(ctx, "user:123:age", "30", 0)

// GOOD: Hash для objects
client.HSet(ctx, "user:123", map[string]interface{}{
    "name":  "John",
    "email": "john@example.com",
    "age":   30,
})
```

### RDD-016: 4. Key Naming Conventions

```
Format: <namespace>:<object>:<id>:<field>

Examples:
- user:123:profile
- session:abc123
- cache:api:posts:latest
- counter:page:views:homepage
```

**Best Practices**:
- Use colons `:` as separator
- Lowercase
- Semantic naming
- Include TTL hint: `cache:1h:api:posts`

### RDD-017: 5. Slow Log Analysis

```bash
# View slow commands (> 10ms)
redis-cli SLOWLOG GET 10

# Reset slow log
redis-cli SLOWLOG RESET
```

**Example Output**:
```
1) 1) (integer) 14
   2) (integer) 1309448221
   3) (integer) 15    # Execution time in microseconds
   4) 1) "KEYS"
      2) "*"
```

**Optimize**:
- Replace `KEYS *` with `SCAN` cursor
- Use pipelining
- Optimize Lua scripts
- Add indexes to data structures

---

## Monitoring & Observability

### RDD-018: 1. Key Metrics (CRITICAL)

| Metric | Command | Description | Alert Threshold |
|--------|---------|-------------|-----------------|
| **Memory Usage** | `INFO memory` | used_memory_rss | > 80% maxmemory |
| **Hit Rate** | `INFO stats` | keyspace_hits / (keyspace_hits + keyspace_misses) | < 80% |
| **Evicted Keys** | `INFO stats` | evicted_keys | > 0 (if noeviction) |
| **Blocked Clients** | `INFO clients` | blocked_clients | > 0 |
| **Connected Clients** | `INFO clients` | connected_clients | > maxclients * 0.8 |
| **Keyspace** | `INFO keyspace` | Total keys | Sudden drops |
| **Replication Lag** | `INFO replication` | master_repl_offset - slave_repl_offset | > 1MB |

### RDD-019: 2. Prometheus Exporter

#### Docker Compose

```yaml
services:
  redis:
    image: redis:7.4-alpine
    # ... config

  redis-exporter:
    image: oliver006/redis_exporter:latest
    environment:
      REDIS_ADDR: redis:6379
      REDIS_PASSWORD: YOUR_PASSWORD
    ports:
      - "9121:9121"
    depends_on:
      - redis
```

#### Kubernetes

```yaml
apiVersion: v1
kind: Service
metadata:
  name: redis-exporter
  labels:
    app: redis-exporter
spec:
  ports:
  - port: 9121
    name: metrics
  selector:
    app: redis-exporter
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis-exporter
  template:
    metadata:
      labels:
        app: redis-exporter
    spec:
      containers:
      - name: redis-exporter
        image: oliver006/redis_exporter:latest
        env:
        - name: REDIS_ADDR
          value: redis:6379
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-auth
              key: password
        ports:
        - containerPort: 9121
          name: metrics
```

### RDD-020: 3. Grafana Dashboard

**Import Dashboard ID**: 763 (Redis Dashboard for Prometheus)

**Key Panels**:
- Memory usage
- Commands per second
- Hit rate percentage
- Connected clients
- Keyspace size
- Network I/O

### RDD-021: 4. Health Checks

```bash
# Basic ping
redis-cli PING  # Should return PONG

# Check replication
redis-cli INFO replication

# Check memory
redis-cli INFO memory | grep used_memory_human

# Check slow queries
redis-cli SLOWLOG GET 10
```

**Kubernetes Probes**:
```yaml
livenessProbe:
  exec:
    command:
    - sh
    - -c
    - redis-cli -a $REDIS_PASSWORD ping | grep -q PONG
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  exec:
    command:
    - sh
    - -c
    - redis-cli -a $REDIS_PASSWORD ping | grep -q PONG
  initialDelaySeconds: 5
  periodSeconds: 5
```

---

## Backup & Recovery

### RDD-022: 1. RDB Snapshots

#### Manual Backup

```bash
# Blocking save (NOT recommended for production)
redis-cli SAVE

# Background save (RECOMMENDED)
redis-cli BGSAVE

# Check last save time
redis-cli LASTSAVE
```

#### Automated Backups

```conf
# redis.conf
save 900 1      # After 900s if 1 key changed
save 300 10     # After 300s if 10 keys changed
save 60 10000   # After 60s if 10000 keys changed
dir /data
dbfilename dump.rdb
```

#### Backup Script

```bash
#!/bin/bash
BACKUP_DIR="/backups/redis"
DATE=$(date +%Y%m%d_%H%M%S)

# Trigger BGSAVE
redis-cli BGSAVE

# Wait for completion
while [ $(redis-cli LASTSAVE) == $LAST_SAVE ]; do
    sleep 1
done

# Copy RDB file
cp /data/dump.rdb "$BACKUP_DIR/dump_$DATE.rdb"

# Compress
gzip "$BACKUP_DIR/dump_$DATE.rdb"

# Retention: keep last 7 days
find $BACKUP_DIR -name "dump_*.rdb.gz" -mtime +7 -delete
```

### RDD-023: 2. AOF Backups

```bash
# Rewrite AOF (compact)
redis-cli BGREWRITEAOF

# Backup AOF file
cp /data/appendonly.aof /backups/appendonly_$(date +%Y%m%d).aof
```

### RDD-024: 3. Kubernetes Backup with CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: redis-backup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: redis:7.4-alpine
            command:
            - /bin/sh
            - -c
            - |
              redis-cli -h redis -a $REDIS_PASSWORD BGSAVE
              sleep 10
              tar czf /backup/redis-$(date +%Y%m%d).tar.gz /data/dump.rdb
            env:
            - name: REDIS_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: redis-auth
                  key: password
            volumeMounts:
            - name: redis-data
              mountPath: /data
            - name: backup-storage
              mountPath: /backup
          restartPolicy: OnFailure
          volumes:
          - name: redis-data
            persistentVolumeClaim:
              claimName: redis-data
          - name: backup-storage
            persistentVolumeClaim:
              claimName: backup-pvc
```

### RDD-025: 4. Recovery Procedures

#### From RDB

```bash
# 1. Stop Redis
systemctl stop redis

# 2. Replace dump.rdb
cp /backups/dump_20250130.rdb /data/dump.rdb
chown redis:redis /data/dump.rdb

# 3. Start Redis
systemctl start redis
```

#### From AOF

```bash
# 1. Stop Redis
systemctl stop redis

# 2. Restore AOF
cp /backups/appendonly.aof /data/appendonly.aof

# 3. Check AOF integrity
redis-check-aof --fix /data/appendonly.aof

# 4. Start Redis
systemctl start redis
```

### RDD-026: 5. Point-in-Time Recovery

**Strategy**: RDB snapshots + AOF

```conf
# redis.conf
save 900 1
appendonly yes
appendfsync everysec
aof-use-rdb-preamble yes  # Hybrid persistence
```

**Recovery Steps**:
1. Restore latest RDB snapshot
2. Replay AOF from RDB timestamp
3. Verify data integrity

---

## Production Patterns

### RDD-027: 1. Cache-Aside Pattern

```go
func GetUser(ctx context.Context, userID string) (*User, error) {
    // 1. Try cache
    cached, err := redisClient.Get(ctx, "user:"+userID).Result()
    if err == nil {
        var user User
        json.Unmarshal([]byte(cached), &user)
        return &user, nil
    }

    // 2. Cache miss - fetch from DB
    user, err := db.GetUser(ctx, userID)
    if err != nil {
        return nil, err
    }

    // 3. Update cache
    data, _ := json.Marshal(user)
    redisClient.Set(ctx, "user:"+userID, data, 1*time.Hour)

    return user, nil
}
```

### RDD-028: 2. Rate Limiting

```go
func RateLimit(ctx context.Context, userID string, limit int64) (bool, error) {
    key := fmt.Sprintf("ratelimit:%s:%d", userID, time.Now().Unix()/60)

    count, err := redisClient.Incr(ctx, key).Result()
    if err != nil {
        return false, err
    }

    if count == 1 {
        // First request - set expiry
        redisClient.Expire(ctx, key, 1*time.Minute)
    }

    return count <= limit, nil
}
```

### RDD-029: 3. Distributed Locking

```go
import "github.com/go-redsync/redsync/v4"

// Create lock
mutex := redsync.New(pool).NewMutex("my-lock",
    redsync.WithExpiry(8*time.Second),
    redsync.WithTries(3),
)

// Acquire lock
if err := mutex.Lock(); err != nil {
    return err
}
defer mutex.Unlock()

// Critical section
performCriticalOperation()
```

### RDD-030: 4. Session Storage

```go
type Session struct {
    UserID    string
    ExpiresAt time.Time
}

func SaveSession(ctx context.Context, sessionID string, userID string) error {
    session := Session{
        UserID:    userID,
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    data, _ := json.Marshal(session)
    return redisClient.Set(ctx, "session:"+sessionID, data, 24*time.Hour).Err()
}

func GetSession(ctx context.Context, sessionID string) (*Session, error) {
    data, err := redisClient.Get(ctx, "session:"+sessionID).Result()
    if err != nil {
        return nil, err
    }

    var session Session
    json.Unmarshal([]byte(data), &session)
    return &session, nil
}
```

### RDD-031: 5. Pub/Sub for Real-time Events

```go
// Publisher
func PublishEvent(ctx context.Context, channel string, message string) error {
    return redisClient.Publish(ctx, channel, message).Err()
}

// Subscriber
func Subscribe(ctx context.Context, channel string) {
    pubsub := redisClient.Subscribe(ctx, channel)
    defer pubsub.Close()

    ch := pubsub.Channel()
    for msg := range ch {
        fmt.Printf("Received: %s from %s\n", msg.Payload, msg.Channel)
    }
}
```

**NOTE**: Pub/Sub не гарантирует доставку! Используй Redis Streams для reliable messaging.

### RDD-032: 6. Leaderboard with Sorted Sets

```go
// Add score
func AddScore(ctx context.Context, userID string, score float64) error {
    return redisClient.ZAdd(ctx, "leaderboard", redis.Z{
        Score:  score,
        Member: userID,
    }).Err()
}

// Get top 10
func GetTopPlayers(ctx context.Context) ([]string, error) {
    return redisClient.ZRevRange(ctx, "leaderboard", 0, 9).Result()
}

// Get user rank
func GetRank(ctx context.Context, userID string) (int64, error) {
    return redisClient.ZRevRank(ctx, "leaderboard", userID).Result()
}
```

---

## Common Anti-Patterns

| Anti-Pattern | Problem | Solution |
|--------------|---------|----------|
| **No password** | Security vulnerability | Always set `requirepass` |
| **Bind to 0.0.0.0** | Exposed to internet | `bind 127.0.0.1` or firewall |
| **KEYS in production** | Blocks server | Use `SCAN` with cursor |
| **No maxmemory** | OOM kills | Set `maxmemory` + eviction policy |
| **No persistence** | Data loss on restart | Enable RDB + AOF |
| **Large values (> 1MB)** | Performance degradation | Split into smaller keys |
| **Hot keys** | Single-threaded bottleneck | Shard or use local cache |
| **No connection pooling** | Connection exhaustion | Use client connection pools |
| **Blocking commands in pipeline** | Defeats purpose | Use async commands |
| **Using DEL for large keys** | Blocking operation | Use `UNLINK` (async delete) |
| **No monitoring** | Blind operations | Prometheus + Grafana |
| **Single instance** | No HA | Sentinel or Cluster |

---

## Quick Reference

### RDD-033: Essential Commands

```bash
# Info
INFO                       # All sections
INFO server                # Server info
INFO memory                # Memory stats
INFO replication           # Replication status
INFO stats                 # Statistics

# Monitoring
MONITOR                    # Real-time commands (DEV ONLY)
SLOWLOG GET 10             # Slow queries
CLIENT LIST                # Connected clients

# Persistence
SAVE                       # Blocking save
BGSAVE                     # Background save
BGREWRITEAOF               # Rewrite AOF

# Cluster
CLUSTER INFO               # Cluster status
CLUSTER NODES              # Node list
CLUSTER SLOTS              # Slot distribution

# Dangerous (disable in production)
FLUSHDB                    # Delete current DB
FLUSHALL                   # Delete all DBs
KEYS *                     # List all keys (blocking)
CONFIG                     # Modify config
```

### RDD-034: Memory Analysis

```bash
# Memory usage by key
redis-cli --bigkeys

# Sample keys
redis-cli --memkeys

# Detailed memory usage
redis-cli MEMORY STATS
```

### RDD-035: Configuration Checklist

```conf
# Security
bind 127.0.0.1
protected-mode yes
requirepass STRONG_PASSWORD

# Memory
maxmemory 512mb
maxmemory-policy allkeys-lru

# Persistence
appendonly yes
appendfsync everysec
save 900 1

# Limits
maxclients 10000
timeout 300

# Logging
loglevel notice
slowlog-log-slower-than 10000
```

---

## Architecture Decision Matrix

| Requirement | Single | Master-Replica | Sentinel | Cluster |
|-------------|--------|----------------|----------|---------|
| **High Availability** | ❌ | Manual | ✅ Auto | ✅ Auto |
| **Read Scaling** | ❌ | ✅ | ✅ | ✅ |
| **Write Scaling** | ❌ | ❌ | ❌ | ✅ |
| **Data Sharding** | ❌ | ❌ | ❌ | ✅ Auto |
| **Failover Time** | N/A | Manual | ~30s | ~few seconds |
| **Operational Complexity** | Low | Low | Medium | High |
| **Multi-key Operations** | ✅ | ✅ | ✅ | ⚠️ Hash tags |
| **Max Dataset Size** | ~256 GB | ~256 GB | ~256 GB | Terabytes |

---

## Дополнительные ресурсы

### RDD-036: Official Documentation
- [Redis Documentation](https://redis.io/docs/)
- [Redis Security](https://redis.io/docs/management/security/)
- [Redis Cluster Tutorial](https://redis.io/docs/management/scaling/)

### RDD-037: Tools
- [redis-cli](https://redis.io/docs/ui/cli/) - CLI client
- [RedisInsight](https://redis.com/redis-enterprise/redis-insight/) - GUI client
- [redis-exporter](https://github.com/oliver006/redis_exporter) - Prometheus exporter

### RDD-038: Client Libraries
- **Go**: [go-redis](https://github.com/redis/go-redis)
- **Python**: [redis-py](https://github.com/redis/redis-py)
- **Node.js**: [ioredis](https://github.com/redis/ioredis)
- **Java**: [Jedis](https://github.com/redis/jedis)

### RDD-039: Research Report
См. полный отчет: `.memory_bank/agent_task_results/2025-12-29_web-research_redis-best-practices.md`

---

**При возникновении вопросов**: Обратись к DevOps engineer agent или см. research report.
