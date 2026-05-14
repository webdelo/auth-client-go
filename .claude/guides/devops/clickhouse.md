# ClickHouse Best Practices and Standards

This document contains ClickHouse deployment rules for DevOps tasks. Each rule has a unique identifier (CHD-XXXX) for reference in code reviews and task execution.

> **Related**: For ClickHouse query patterns, see `dba/clickhouse/`
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми ClickHouse задачами
**Текущая версия**: v25.8.13.73-lts (LTS), v25.11.4.19 (Stable)

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| CHD-001 | Use separate disks for data/logs | ✅ | [Storage Configuration](#chd-001-use-separate-disks-for-datalogs) |
| CHD-002 | Configure ClickHouse Keeper for cluster | ✅ | [Cluster Configuration](#chd-002-configure-clickhouse-keeper-for-cluster) |
| CHD-003 | Set max_memory_usage per query | ✅ | [Memory Management](#chd-003-set-max_memory_usage-per-query) |
| CHD-004 | Partition tables by date | | [Partitioning](#chd-004-partition-tables-by-date) |

---

## Содержание

1. [Installation & Configuration](#installation--configuration)
2. [Load Balancing & High Availability](#load-balancing--high-availability)
3. [Security Standards](#security-standards)
4. [Performance Optimization](#performance-optimization)
5. [Storage Management](#storage-management)
6. [Monitoring & Observability](#monitoring--observability)
7. [Backup & Recovery](#backup--recovery)
8. [Kubernetes Deployment](#kubernetes-deployment)
9. [Common Anti-Patterns](#common-anti-patterns)

---

## Installation & Configuration

### CHD-001: 1. Version Selection (CRITICAL)

**Правило**: ВСЕГДА используй LTS версии в production.

| Version Type | Current | Stability | Use Case |
|-------------|---------|-----------|----------|
| **LTS** | v25.8.13.73 | Highest | Production (REQUIRED) |
| **Stable** | v25.11.4.19 | Medium | Staging, new features |
| **Testing** | - | Low | Development only |

**LTS Support**: 1 год с момента релиза

### CHD-002: 2. Installation Methods

#### Debian/Ubuntu (Recommended)

```bash
# Add repository
sudo apt-get install -y apt-transport-https ca-certificates dirmngr
sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 8919F6BD2B48D754

echo "deb https://packages.clickhouse.com/deb stable main" | \
  sudo tee /etc/apt/sources.list.d/clickhouse.list

# Install specific LTS version
sudo apt-get update
sudo apt-get install -y clickhouse-server=25.8.13.73 clickhouse-client=25.8.13.73

# Hold version to prevent auto-updates
sudo apt-mark hold clickhouse-server clickhouse-client
```

#### Docker (Development)

```dockerfile
FROM clickhouse/clickhouse-server:25.8.13.73-alpine

# Non-root user
USER clickhouse

EXPOSE 8123 9000 9009

HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --spider -q http://localhost:8123/ping || exit 1
```

### CHD-003: 3. System Tuning (ОБЯЗАТЕЛЬНО для production)

#### CPU Governor

```bash
# Set to performance mode
echo 'performance' | sudo tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor

# Make persistent
cat << 'EOF' | sudo tee /etc/rc.local
#!/bin/bash
echo 'performance' | tee /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor
EOF
sudo chmod +x /etc/rc.local
```

#### Memory Requirements

| Scenario | RAM Recommendation |
|----------|-------------------|
| **Compressed data < 200GB** | RAM ≥ compressed data size |
| **Interactive queries** | 128+ GB |
| **Production cluster** | 256+ GB per node |
| **Low memory (<16GB)** | Special tuning required (see below) |

#### Low Memory Configuration

```xml
<!-- /etc/clickhouse-server/config.d/low_memory.xml -->
<clickhouse>
    <mark_cache_size>5368709120</mark_cache_size> <!-- 5GB -->
    <max_thread_pool_size>1</max_thread_pool_size>
    <max_block_size>8192</max_block_size>

    <!-- Disable expensive logs -->
    <asynchronous_metric_log remove="1"/>
    <metric_log remove="1"/>
    <text_log remove="1"/>
    <trace_log remove="1"/>
</clickhouse>
```

#### Storage Configuration

```bash
# Filesystem tuning (ext4 recommended)
sudo mkfs.ext4 -b 4096 /dev/nvme0n1
sudo mount -o noatime,nodiratime /dev/nvme0n1 /var/lib/clickhouse

# Add to /etc/fstab
/dev/nvme0n1 /var/lib/clickhouse ext4 noatime,nodiratime 0 2

# Enable TRIM for SSDs
sudo systemctl enable fstrim.timer
sudo systemctl start fstrim.timer
```

**Storage Requirements**:
- **Disk type**: NVMe SSD (production), SATA SSD (staging), HDD 7200+ RPM (archives)
- **RAID**: RAID-10 preferred, RAID-6/50 for 4+ disks
- **Block size**: 64KB
- **Min capacity**: 3x compressed data size

**AVOID**: FAT-32, exFAT, compressed/encrypted filesystems

#### Network

```bash
# Network tuning for 10Gbps+
cat << 'EOF' | sudo tee -a /etc/sysctl.conf
# TCP buffers
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864

# Connection tracking
net.netfilter.nf_conntrack_max = 1048576

# File descriptors
fs.file-max = 1000000
EOF

sudo sysctl -p
```

**Requirements**:
- **Minimum**: 10 Gbps network (production)
- **Latency**: <20ms RTT between cluster nodes
- **Degraded**: 1 Gbps acceptable for small workloads

### CHD-004: 4. Base Configuration

```xml
<!-- /etc/clickhouse-server/config.d/server.xml -->
<clickhouse>
    <!-- Listen addresses -->
    <listen_host>0.0.0.0</listen_host>

    <!-- Ports -->
    <tcp_port_secure>9440</tcp_port_secure>
    <https_port>8443</https_port>
    <!-- Disable insecure ports in production -->
    <!-- <tcp_port>9000</tcp_port> -->
    <!-- <http_port>8123</http_port> -->

    <!-- Server settings -->
    <max_connections>4096</max_connections>
    <keep_alive_timeout>3</keep_alive_timeout>
    <max_concurrent_queries>100</max_concurrent_queries>

    <!-- Memory management -->
    <max_server_memory_usage_to_ram_ratio>0.8</max_server_memory_usage_to_ram_ratio>
    <max_server_memory_usage>107374182400</max_server_memory_usage> <!-- 100GB -->

    <!-- Background tasks -->
    <background_pool_size>16</background_pool_size>
    <background_merges_mutations_concurrency_ratio>2</background_merges_mutations_concurrency_ratio>
</clickhouse>
```

---

## Load Balancing & High Availability

### CHD-005: 1. Cluster Architecture Patterns (CRITICAL)

#### Recommended Topology для Production

```
                    +------------------+
                    |  Load Balancer   |
                    |  (CHProxy/nginx) |
                    +--------+---------+
                             |
         +-------------------+-------------------+
         |                   |                   |
    +----v----+         +----v----+         +----v----+
    | Shard 1 |         | Shard 2 |         | Shard 3 |
    +---------+         +---------+         +---------+
         |                   |                   |
    +----+----+         +----+----+         +----+----+
    |Replica 1|         |Replica 1|         |Replica 1|
    |Replica 2|         |Replica 2|         |Replica 2|
    |Replica 3|         |Replica 3|         |Replica 3|
    +---------+         +---------+         +---------+
              \            |            /
               \           |           /
                +----------v----------+
                |  ClickHouse Keeper  |
                |  Ensemble (3 nodes) |
                +---------------------+
```

**Key Principles**:
- **3+ replicas** per shard для HA
- **Separate AZs**: Replicas в разных availability zones
- **Network latency**: <20ms RTT между всеми нодами
- **Physical separation**: Separate hosts, power, network

### CHD-006: 2. Cluster Configuration

```xml
<!-- /etc/clickhouse-server/config.d/remote_servers.xml -->
<clickhouse>
    <remote_servers>
        <production_cluster>
            <!-- Shard 1 -->
            <shard>
                <internal_replication>true</internal_replication>
                <weight>1</weight>
                <replica>
                    <host>ch-s1-r1.example.com</host>
                    <port>9440</port>
                    <secure>1</secure>
                </replica>
                <replica>
                    <host>ch-s1-r2.example.com</host>
                    <port>9440</port>
                    <secure>1</secure>
                </replica>
                <replica>
                    <host>ch-s1-r3.example.com</host>
                    <port>9440</port>
                    <secure>1</secure>
                </replica>
            </shard>

            <!-- Shard 2 -->
            <shard>
                <internal_replication>true</internal_replication>
                <weight>1</weight>
                <replica>
                    <host>ch-s2-r1.example.com</host>
                    <port>9440</port>
                    <secure>1</secure>
                </replica>
                <replica>
                    <host>ch-s2-r2.example.com</host>
                    <port>9440</port>
                    <secure>1</secure>
                </replica>
                <replica>
                    <host>ch-s2-r3.example.com</host>
                    <port>9440</port>
                    <secure>1</secure>
                </replica>
            </shard>
        </production_cluster>
    </remote_servers>
</clickhouse>
```

**CRITICAL Settings**:
- `internal_replication: true` - Inserts идут только на одну реплику, распространяются автоматически
- `weight` - Для неоднородных шардов (разная мощность)
- `secure: 1` - ОБЯЗАТЕЛЬНО в production

### CHD-007: 3. Sharding Strategies

#### Creating Distributed Tables

```sql
-- Local table на каждом шарде (ReplicatedMergeTree)
CREATE TABLE events_local ON CLUSTER production_cluster
(
    event_date Date,
    event_time DateTime,
    user_id UInt64,
    tenant_id UInt32,
    event_type LowCardinality(String),
    data String
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/events', '{replica}')
PARTITION BY toYYYYMM(event_date)
ORDER BY (tenant_id, user_id, event_time)
TTL event_date + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

-- Distributed table для queries
CREATE TABLE events ON CLUSTER production_cluster
AS events_local
ENGINE = Distributed(production_cluster, default, events_local, sipHash64(user_id));
```

#### Sharding Key Selection

| Strategy | Sharding Key | Use Case | Distribution |
|----------|--------------|----------|--------------|
| **Hash by ID** | `sipHash64(user_id)` | Events, logs | Even, predictable |
| **Tenant ID** | `tenant_id` | Multi-tenant SaaS | Tenant isolation |
| **Random** | `rand()` | Analytics, append-only | Perfect distribution |
| **Date-based** | `toYYYYMM(date)` | Time-series archives | Time-based queries |

**Best Practices**:
```sql
-- GOOD: High cardinality, even distribution
ENGINE = Distributed(cluster, db, table, sipHash64(user_id));

-- GOOD: Multi-tenant isolation
ENGINE = Distributed(cluster, db, table, tenant_id);

-- GOOD: Random для max распределения
ENGINE = Distributed(cluster, db, table, rand());

-- BAD: Low cardinality
ENGINE = Distributed(cluster, db, table, status); -- Only 3-5 values!

-- BAD: Sequential IDs без hash
ENGINE = Distributed(cluster, db, table, id); -- Uneven distribution!
```

#### What to Shard vs NOT to Shard

| Shard | Don't Shard |
|-------|-------------|
| Large fact tables (events, logs) | Dimension tables (users, products) |
| Unbounded growth | Slowly changing dimensions |
| High write volume | Frequently JOINed tables (<1M rows) |
| Time-series data | Reference/lookup tables |

### CHD-008: 4. Replication with ReplicatedMergeTree

```sql
-- ZooKeeper path pattern: /clickhouse/tables/{shard}/table_name
-- {shard} - автозамена на shard номер
-- {replica} - автозамена на replica ID

CREATE TABLE metrics_local ON CLUSTER production_cluster
(
    timestamp DateTime,
    metric_name LowCardinality(String),
    value Float64,
    tags Map(String, String)
)
ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/metrics', '{replica}')
PARTITION BY toYYYYMM(timestamp)
ORDER BY (metric_name, timestamp);
```

**Replication Behavior**:
- **Async replication**: No write latency increase
- **Automatic sync**: New replicas catch up automatically
- **Consistency**: Eventual consistency (configurable)
- **Leader election**: Automatic via ClickHouse Keeper

### CHD-009: 5. ClickHouse Keeper Configuration

**ВАЖНО**: Используй ClickHouse Keeper вместо ZooKeeper (v23.3+)

#### Keeper Server Configuration

```xml
<!-- /etc/clickhouse-keeper/keeper.xml -->
<clickhouse>
    <keeper_server>
        <tcp_port>2181</tcp_port>
        <server_id>1</server_id> <!-- Unique: 1, 2, 3 -->

        <log_storage_path>/var/lib/clickhouse/coordination/log</log_storage_path>
        <snapshot_storage_path>/var/lib/clickhouse/coordination/snapshots</snapshot_storage_path>

        <coordination_settings>
            <operation_timeout_ms>10000</operation_timeout_ms>
            <session_timeout_ms>30000</session_timeout_ms>
            <raft_logs_level>warning</raft_logs_level>
            <compress_logs>true</compress_logs>
            <compress_snapshots_with_zstd_format>true</compress_snapshots_with_zstd_format>
        </coordination_settings>

        <raft_configuration>
            <server>
                <id>1</id>
                <hostname>keeper-01.example.com</hostname>
                <port>9444</port>
            </server>
            <server>
                <id>2</id>
                <hostname>keeper-02.example.com</hostname>
                <port>9444</port>
            </server>
            <server>
                <id>3</id>
                <hostname>keeper-03.example.com</hostname>
                <port>9444</port>
            </server>
        </raft_configuration>
    </keeper_server>
</clickhouse>
```

#### ClickHouse Connection to Keeper

```xml
<!-- /etc/clickhouse-server/config.d/zookeeper.xml -->
<clickhouse>
    <zookeeper>
        <node>
            <host>keeper-01.example.com</host>
            <port>2181</port>
        </node>
        <node>
            <host>keeper-02.example.com</host>
            <port>2181</port>
        </node>
        <node>
            <host>keeper-03.example.com</host>
            <port>2181</port>
        </node>
        <session_timeout_ms>30000</session_timeout_ms>
        <operation_timeout_ms>10000</operation_timeout_ms>
    </zookeeper>

    <!-- Distributed DDL coordination -->
    <distributed_ddl>
        <path>/clickhouse/task_queue/ddl</path>
        <profile>default</profile>
    </distributed_ddl>
</clickhouse>
```

**Production Requirements**:
- **3 nodes minimum** (odd number для quorum)
- **Separate servers** от ClickHouse (dedicated hardware)
- **Fast disks**: NVMe preferred, 128GB+ capacity
- **Network**: 10 Gbps, low latency
- **Unique paths**: Different `distributed_ddl` path per cluster

### CHD-010: 6. Load Balancing with CHProxy

#### Installation

```bash
# Download latest release
wget https://github.com/ContentSquare/chproxy/releases/download/v1.27.0/chproxy-linux-amd64.tar.gz
tar -xzf chproxy-linux-amd64.tar.gz
sudo mv chproxy /usr/local/bin/

# Create config directory
sudo mkdir -p /etc/chproxy
```

#### CHProxy Configuration

```yaml
# /etc/chproxy/config.yml
server:
  http:
    listen_addr: ":9090"
    allowed_networks: ["10.0.0.0/8"]
  https:
    listen_addr: ":9443"
    autocert:
      cache_dir: "/var/cache/chproxy/autocert"
      allowed_hosts: ["chproxy.example.com"]

log_debug: false
hack_me_please: false

# User mapping and rate limiting
users:
  - name: "web_ro"
    password: "readonly_pass"
    to_cluster: "analytics"
    to_user: "readonly"
    max_concurrent_queries: 10
    max_execution_time: 60s
    requests_per_minute: 600
    deny_http: false
    allow_cors: true
    cache: "query_cache"

  - name: "etl_writer"
    password: "writer_pass"
    to_cluster: "analytics"
    to_user: "writer"
    max_concurrent_queries: 5
    max_execution_time: 300s
    deny_http: false

# Query result caching
caches:
  - name: query_cache
    mode: file_system
    file_system:
      dir: "/var/cache/chproxy/cache"
      max_size: 50Gb
    expire: 300s
    grace_time: 10s

# Backend clusters
clusters:
  - name: "analytics"
    scheme: "https"

    # Cluster nodes with health checks
    nodes:
      - "ch-s1-r1.example.com:8123"
      - "ch-s1-r2.example.com:8123"
      - "ch-s1-r3.example.com:8123"
      - "ch-s2-r1.example.com:8123"
      - "ch-s2-r2.example.com:8123"
      - "ch-s2-r3.example.com:8123"

    # Backend credentials
    users:
      - name: "readonly"
        password: "ch_readonly_password"
      - name: "writer"
        password: "ch_writer_password"

    # Health check configuration
    heartbeat:
      interval: 1s
      timeout: 3s
      request: "/?query=SELECT%201"

    # Kill queries on timeout
    kill_query_user:
      name: "killer"
      password: "killer_password"
```

**CHProxy Features**:
- **Round-robin + least-loaded** балансировка
- **Query caching** с TTL и grace periods
- **Per-user rate limiting** и query timeouts
- **Automatic failover** при падении нод
- **HTTPS/TLS** с Let's Encrypt autocert
- **Prometheus metrics** export на `/metrics`
- **Query logging** и audit trail

#### Alternative: nginx Load Balancer

```nginx
# /etc/nginx/conf.d/clickhouse-lb.conf
upstream clickhouse_backend {
    least_conn;

    server ch-s1-r1.example.com:8123 max_fails=3 fail_timeout=30s;
    server ch-s1-r2.example.com:8123 max_fails=3 fail_timeout=30s;
    server ch-s1-r3.example.com:8123 max_fails=3 fail_timeout=30s;

    keepalive 32;
}

server {
    listen 80;
    server_name clickhouse.example.com;

    location / {
        proxy_pass http://clickhouse_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # Timeouts
        proxy_connect_timeout 10s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;

        # Health check
        health_check interval=5s fails=3 passes=2 uri=/?query=SELECT%201;
    }
}
```

**nginx pros/cons**:
- ✅ Lightweight, proven
- ✅ Good for simple load balancing
- ❌ No query caching
- ❌ No per-user rate limiting
- ❌ Limited ClickHouse-specific features

**Recommendation**: CHProxy для production (feature-rich), nginx для simple setups.

### CHD-011: 7. Distributed Queries Performance

#### Query Routing

```sql
-- Queries на distributed table распределяются по шардам
SELECT count(*) FROM events WHERE event_date >= today();

-- Каждый шард выполняет локально:
SELECT count(*) FROM events_local WHERE event_date >= today();

-- Results aggregated на initiator node
```

**Best Practices**:
```sql
-- GOOD: Filter pushdown to local tables
SELECT user_id, count()
FROM events
WHERE event_date = today() AND tenant_id = 123
GROUP BY user_id;

-- BAD: Cross-shard JOIN
SELECT e.*, u.name
FROM events e
JOIN users u ON e.user_id = u.id; -- users должен быть NOT sharded!

-- GOOD: Use GLOBAL JOIN для dimension tables
SELECT e.*, u.name
FROM events e
GLOBAL JOIN users u ON e.user_id = u.id;

-- BETTER: Use Dictionary вместо JOIN
SELECT event_id, dictGet('users_dict', 'name', user_id) AS user_name
FROM events;
```

### CHD-012: 8. Failover & Recovery

#### Monitoring Replication Health

```sql
-- Check replication status
SELECT
    database,
    table,
    is_leader,
    total_replicas,
    active_replicas,
    queue_size,
    inserts_in_queue,
    merges_in_queue,
    log_max_index - log_pointer AS replication_lag
FROM system.replicas
WHERE active_replicas < total_replicas;

-- Check for readonly tables
SELECT * FROM system.replicas WHERE is_readonly;
```

#### Manual Recovery Commands

```sql
-- Restart stuck replica
SYSTEM RESTART REPLICA database.table;

-- Drop replica metadata (после удаления сервера)
SYSTEM DROP REPLICA 'replica_name' FROM TABLE database.table;
SYSTEM DROP REPLICA 'replica_name' FROM ZKPATH '/clickhouse/tables/{shard}/table';

-- Force fetch from specific replica
SYSTEM FETCH PARTITION partition_id FROM 'zk_path' TABLE database.table;
```

#### Automatic Failover Process

1. **Detection**: CHProxy health checks fail (3 consecutive)
2. **Removal**: Node removed from load balancer pool
3. **Routing**: Queries redirected to healthy replicas
4. **Recovery**: Node rejoins when health checks pass
5. **Sync**: Replica catches up via async replication

**RTO/RPO**:
- **RTO** (Recovery Time): <30 seconds (health check interval)
- **RPO** (Recovery Point): 0 (no data loss with quorum inserts)

---

## Security Standards

### CHD-013: 1. TLS/SSL Configuration (ОБЯЗАТЕЛЬНО)

```xml
<!-- /etc/clickhouse-server/config.d/ssl.xml -->
<clickhouse>
    <openSSL>
        <server>
            <certificateFile>/etc/clickhouse-server/certs/server.crt</certificateFile>
            <privateKeyFile>/etc/clickhouse-server/certs/server.key</privateKeyFile>
            <caConfig>/etc/clickhouse-server/certs/ca.crt</caConfig>

            <verificationMode>strict</verificationMode>
            <loadDefaultCAFile>true</loadDefaultCAFile>
            <cacheSessions>true</cacheSessions>
            <disableProtocols>sslv2,sslv3,tlsv1,tlsv1_1</disableProtocols>
            <preferServerCiphers>true</preferServerCiphers>
            <cipherList>ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256</cipherList>
        </server>
    </openSSL>

    <!-- Enable secure ports -->
    <tcp_port_secure>9440</tcp_port_secure>
    <https_port>8443</https_port>

    <!-- DISABLE insecure ports in production -->
    <!-- <tcp_port>9000</tcp_port> -->
    <!-- <http_port>8123</http_port> -->
</clickhouse>
```

**Certificate Generation**:
```bash
# Self-signed (development)
openssl req -newkey rsa:2048 -nodes -keyout server.key \
  -x509 -days 365 -out server.crt

# Let's Encrypt (production)
certbot certonly --standalone -d clickhouse.example.com
```

### CHD-014: 2. Authentication & RBAC

#### User Management

```sql
-- Create admin user
CREATE USER admin IDENTIFIED WITH sha256_password BY 'strong_password'
    HOST IP '10.0.0.0/8', IP '192.168.0.0/16';

-- Create read-only role
CREATE ROLE analyst;
GRANT SELECT ON database.* TO analyst;

-- Create read-write role
CREATE ROLE data_engineer;
GRANT SELECT, INSERT ON database.* TO data_engineer;
GRANT CREATE TABLE ON database.* TO data_engineer;

-- Assign role to user
CREATE USER john IDENTIFIED WITH sha256_password BY 'john_password'
    HOST IP '10.0.0.0/8';
GRANT analyst TO john;

-- Row-level security
CREATE ROW POLICY tenant_isolation ON database.events
    FOR SELECT
    USING tenant_id = currentUser()
    TO analyst;
```

#### Default User Hardening

```xml
<!-- /etc/clickhouse-server/users.d/default.xml -->
<clickhouse>
    <users>
        <default>
            <!-- DISABLE default user in production -->
            <password remove="1"/>
            <password_sha256_hex remove="1"/>
            <!-- OR set strong password -->
            <password_sha256_hex>HASH</password_sha256_hex>

            <!-- Restrict network access -->
            <networks>
                <ip>::1</ip>
                <ip>127.0.0.1</ip>
            </networks>

            <!-- Read-only by default -->
            <readonly>1</readonly>
        </default>
    </users>
</clickhouse>
```

### CHD-015: 3. Network Security

```xml
<!-- /etc/clickhouse-server/config.d/network.xml -->
<clickhouse>
    <!-- Bind to specific interfaces (NOT 0.0.0.0 in production) -->
    <listen_host>10.0.1.10</listen_host>

    <!-- Interserver communication -->
    <interserver_http_host>clickhouse-01.internal</interserver_http_host>
    <interserver_https_port>9010</interserver_https_port>
</clickhouse>
```

**Firewall Rules** (iptables):
```bash
# Allow ClickHouse native protocol (secure)
sudo iptables -A INPUT -p tcp --dport 9440 -s 10.0.0.0/8 -j ACCEPT

# Allow HTTP interface (secure)
sudo iptables -A INPUT -p tcp --dport 8443 -s 10.0.0.0/8 -j ACCEPT

# Allow interserver replication
sudo iptables -A INPUT -p tcp --dport 9009 -s 10.0.1.0/24 -j ACCEPT

# Drop all other
sudo iptables -A INPUT -p tcp --dport 8123 -j DROP
sudo iptables -A INPUT -p tcp --dport 9000 -j DROP
```

### CHD-016: 4. Quotas & Resource Limits

```sql
-- Create quota для web users
CREATE QUOTA web_users
    FOR INTERVAL 1 hour
        MAX queries = 1000,
        MAX errors = 100,
        MAX result_rows = 1000000000,
        MAX result_bytes = 100000000000,
        MAX read_rows = 100000000000,
        MAX execution_time = 3600
    TO web_ro;

-- Create quota для ETL jobs
CREATE QUOTA etl_jobs
    FOR INTERVAL 1 day
        MAX queries = 10000,
        MAX execution_time = 86400
    TO etl_writer;
```

---

## Performance Optimization

### CHD-017: 1. Table Engine Selection

| Engine | Use Case | Features |
|--------|----------|----------|
| **MergeTree** | OLAP, single-node | Partitioning, sorting, TTL |
| **ReplicatedMergeTree** | OLAP, HA cluster | Async replication |
| **ReplacingMergeTree** | Deduplication | Keep last version |
| **SummingMergeTree** | Pre-aggregation | Auto-sum numeric columns |
| **AggregatingMergeTree** | Complex aggregates | State functions |
| **CollapsingMergeTree** | Updates/deletes | Sign-based collapsing |
| **VersionedCollapsingMergeTree** | Ordered updates | Version + sign |

**Example**:
```sql
-- Events с deduplication
CREATE TABLE events_deduplicated
(
    event_id UUID,
    event_time DateTime,
    user_id UInt64,
    data String,
    version UInt16
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{shard}/events', '{replica}', version)
PARTITION BY toYYYYMM(event_time)
ORDER BY (user_id, event_id);
```

### CHD-018: 2. Partitioning Best Practices

```sql
-- GOOD: Monthly partitioning
CREATE TABLE logs
(
    log_date Date,
    log_time DateTime,
    message String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(log_date)
ORDER BY log_time;

-- GOOD: Weekly для high-volume
PARTITION BY toMonday(log_date)

-- GOOD: Multi-column partitioning
PARTITION BY (toYYYYMM(log_date), region)

-- BAD: High cardinality partitioning
PARTITION BY log_time  -- Создаст ТЫСЯЧИ партиций!

-- BAD: No business value
PARTITION BY user_id  -- Не ускоряет queries!
```

**Partition Limits**:
- **Target**: Dozens to hundreds of partitions
- **Maximum**: <1000 partitions per table
- **Partition size**: 10GB - 100GB optimal

### CHD-019: 3. Primary Key Design (CRITICAL)

```sql
-- Order columns by cardinality (lowest to highest)
ORDER BY (status, user_id, timestamp)
-- status: 5-10 values
-- user_id: millions
-- timestamp: infinite

-- Multi-column для specific queries
ORDER BY (tenant_id, project_id, event_time)
-- Accelerates: WHERE tenant_id = X AND project_id = Y

-- AVOID too many columns
ORDER BY (col1, col2, col3, col4, col5) -- TOO MANY! Worse compression!
```

**Optimization Rules**:
1. **First column**: Lowest cardinality, frequently filtered
2. **2-3 columns optimal**: Balance query speed vs compression
3. **Sparse index**: Every 8192 rows (index_granularity)
4. **ORDER BY ≠ PRIMARY KEY**: Can be different!

```sql
-- Advanced: Separate ORDER BY and PRIMARY KEY
CREATE TABLE metrics
(
    timestamp DateTime,
    metric_name LowCardinality(String),
    value Float64
)
ENGINE = MergeTree()
ORDER BY (metric_name, timestamp)
PRIMARY KEY metric_name  -- Shorter primary key = better compression
```

### CHD-020: 4. Compression Codecs

```sql
CREATE TABLE optimized_events
(
    -- IDs: Delta + ZSTD
    id UInt64 CODEC(Delta, ZSTD(3)),

    -- Timestamps: DoubleDelta + LZ4
    timestamp DateTime CODEC(DoubleDelta, LZ4),

    -- Floats: Gorilla + LZ4
    price Float64 CODEC(Gorilla, LZ4),

    -- Low cardinality strings: Dictionary encoding
    status LowCardinality(String),

    -- High cardinality strings: ZSTD
    description String CODEC(ZSTD(1)),

    -- Binary data: LZ4 (fastest)
    payload String CODEC(LZ4)
)
ENGINE = MergeTree()
ORDER BY id;
```

| Codec | Best For | Compression Ratio | CPU Cost |
|-------|----------|------------------|----------|
| **LZ4** | General, binary | ~2-3x | Low |
| **ZSTD(1-3)** | Strings, mixed | ~3-5x | Medium |
| **Delta** | Monotonic integers | ~10-100x | Low |
| **DoubleDelta** | Timestamps, counters | ~50-500x | Low |
| **Gorilla** | Float/Double | ~10-50x | Medium |
| **LowCardinality** | Enums, statuses | ~10-100x | Low |

**Testing Codecs**:
```sql
-- Compare compression ratios
SELECT
    name,
    formatReadableSize(sum(data_compressed_bytes)) AS compressed,
    formatReadableSize(sum(data_uncompressed_bytes)) AS uncompressed,
    round(sum(data_uncompressed_bytes) / sum(data_compressed_bytes), 2) AS ratio
FROM system.columns
WHERE table = 'my_table'
GROUP BY name;
```

### CHD-021: 5. Query Optimization

#### Using EXPLAIN

```sql
-- Analyze query plan
EXPLAIN indexes = 1
SELECT user_id, count()
FROM events
WHERE event_date >= '2025-01-01' AND status = 'active'
GROUP BY user_id;

-- Check estimated rows scanned
EXPLAIN ESTIMATE
SELECT * FROM events WHERE user_id = 123;

-- Pipeline analysis
EXPLAIN PIPELINE
SELECT count() FROM events;
```

#### Slow Query Analysis

```sql
-- Top 20 slow queries (last 24h)
SELECT
    type,
    query_start_time,
    query_duration_ms / 1000 AS duration_sec,
    formatReadableSize(memory_usage) AS memory,
    formatReadableQuantity(read_rows) AS rows_read,
    query
FROM system.query_log
WHERE type = 'QueryFinish'
  AND query_start_time > now() - INTERVAL 1 DAY
  AND query_duration_ms > 1000
ORDER BY query_duration_ms DESC
LIMIT 20;

-- Queries by normalized hash (find similar)
SELECT
    normalized_query_hash,
    count() AS query_count,
    sum(query_duration_ms) / 1000 AS total_duration_sec,
    avg(read_rows) AS avg_rows,
    any(query) AS example_query
FROM system.query_log
WHERE type = 'QueryFinish'
  AND event_time > now() - INTERVAL 1 DAY
GROUP BY normalized_query_hash
ORDER BY total_duration_sec DESC
LIMIT 20;
```

### CHD-022: 6. Memory Management

```xml
<!-- /etc/clickhouse-server/users.d/profiles.xml -->
<clickhouse>
    <profiles>
        <!-- Default profile -->
        <default>
            <!-- Per-query memory limit -->
            <max_memory_usage>10000000000</max_memory_usage> <!-- 10GB -->

            <!-- Per-user total memory limit -->
            <max_memory_usage_for_user>30000000000</max_memory_usage_for_user>

            <!-- Enable external sorting/grouping -->
            <max_bytes_before_external_group_by>5000000000</max_bytes_before_external_group_by>
            <max_bytes_before_external_sort>5000000000</max_bytes_before_external_sort>

            <!-- Query complexity limits -->
            <max_rows_to_group_by>1000000</max_rows_to_group_by>
            <max_rows_to_sort>1000000</max_rows_to_sort>
        </default>

        <!-- Heavy analytics profile -->
        <analytics>
            <max_memory_usage>50000000000</max_memory_usage> <!-- 50GB -->
            <max_execution_time>3600</max_execution_time> <!-- 1 hour -->
            <max_bytes_before_external_group_by>20000000000</max_bytes_before_external_group_by>
        </analytics>
    </profiles>
</clickhouse>
```

---

## Storage Management

### CHD-023: 1. Storage Policies (Tiered Storage)

```xml
<!-- /etc/clickhouse-server/config.d/storage.xml -->
<clickhouse>
    <storage_configuration>
        <disks>
            <!-- Hot tier: NVMe SSD -->
            <hot_nvme>
                <path>/mnt/nvme/clickhouse/</path>
            </hot_nvme>

            <!-- Warm tier: SATA SSD -->
            <warm_ssd>
                <path>/mnt/ssd/clickhouse/</path>
            </warm_ssd>

            <!-- Cold tier: HDD -->
            <cold_hdd>
                <path>/mnt/hdd/clickhouse/</path>
            </cold_hdd>

            <!-- Archive tier: S3 -->
            <s3_archive>
                <type>s3</type>
                <endpoint>https://s3.us-east-1.amazonaws.com/my-bucket/clickhouse/</endpoint>
                <access_key_id>ACCESS_KEY</access_key_id>
                <secret_access_key>SECRET_KEY</secret_access_key>
                <region>us-east-1</region>
                <use_environment_credentials>false</use_environment_credentials>
            </s3_archive>
        </disks>

        <policies>
            <tiered_storage>
                <volumes>
                    <hot>
                        <disk>hot_nvme</disk>
                    </hot>
                    <warm>
                        <disk>warm_ssd</disk>
                    </warm>
                    <cold>
                        <disk>cold_hdd</disk>
                    </cold>
                    <archive>
                        <disk>s3_archive</disk>
                    </archive>
                </volumes>
                <move_factor>0.1</move_factor> <!-- Move when disk 90% full -->
            </tiered_storage>
        </policies>
    </storage_configuration>
</clickhouse>
```

### CHD-024: 2. TTL and Data Lifecycle

```sql
-- Table with tiered TTL
CREATE TABLE logs_tiered
(
    log_date Date,
    log_time DateTime,
    level LowCardinality(String),
    message String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(log_date)
ORDER BY log_time
TTL
    log_date + INTERVAL 7 DAY TO VOLUME 'warm',
    log_date + INTERVAL 30 DAY TO VOLUME 'cold',
    log_date + INTERVAL 90 DAY TO VOLUME 'archive',
    log_date + INTERVAL 365 DAY DELETE
SETTINGS storage_policy = 'tiered_storage';

-- Column-level TTL (GDPR compliance)
CREATE TABLE user_events
(
    user_id UInt64,
    event_time DateTime,
    ip_address String TTL event_time + INTERVAL 90 DAY,  -- Delete PII
    event_data String
)
ENGINE = MergeTree()
ORDER BY (user_id, event_time);

-- Performance: Drop entire parts instead of rows
ALTER TABLE logs_tiered MODIFY SETTING ttl_only_drop_parts = 1;
```

### CHD-025: 3. Materialized Views

```sql
-- Aggregated materialized view
CREATE MATERIALIZED VIEW daily_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (event_date, user_id)
AS SELECT
    toDate(event_time) AS event_date,
    user_id,
    count() AS event_count,
    sum(value) AS total_value
FROM events_local
GROUP BY event_date, user_id;

-- Query materialized view (much faster)
SELECT
    event_date,
    user_id,
    sum(event_count) AS total_events,
    sum(total_value) AS total
FROM daily_stats_mv
WHERE event_date >= '2025-01-01'
GROUP BY event_date, user_id;
```

**Best Practices**:
- **Limit MVs**: <50 materialized views per table (slow inserts)
- **Incremental**: Materialized views auto-update on INSERT
- **Idempotent**: Use `ReplicatedSummingMergeTree` для consistency

---

## Monitoring & Observability

### CHD-026: 1. Key Metrics (CRITICAL)

#### Replication Health

```sql
-- Check replication lag and status
SELECT
    database,
    table,
    is_leader,
    total_replicas,
    active_replicas,
    queue_size,
    inserts_in_queue,
    merges_in_queue,
    log_max_index - log_pointer AS replication_lag,
    last_queue_update
FROM system.replicas
WHERE active_replicas < total_replicas OR queue_size > 100;
```

#### Parts & Merges

```sql
-- Too many parts check (CRITICAL)
SELECT
    database,
    table,
    count() AS parts_count,
    sum(rows) AS total_rows,
    formatReadableSize(sum(bytes_on_disk)) AS total_size
FROM system.parts
WHERE active
GROUP BY database, table
HAVING parts_count > 300  -- Warning threshold
ORDER BY parts_count DESC;

-- Merge activity
SELECT
    database,
    table,
    elapsed,
    progress,
    formatReadableSize(total_size_bytes_compressed) AS size,
    formatReadableSize(bytes_read_uncompressed) AS read,
    formatReadableSize(bytes_written_uncompressed) AS written
FROM system.merges;
```

#### Query Performance

```sql
-- Current running queries
SELECT
    query_id,
    user,
    elapsed,
    formatReadableSize(memory_usage) AS memory,
    formatReadableQuantity(read_rows) AS rows_read,
    query
FROM system.processes
WHERE elapsed > 10;  -- Running >10 seconds

-- Kill long-running query
KILL QUERY WHERE query_id = 'query_id_here';
```

### CHD-027: 2. Prometheus Integration

```xml
<!-- /etc/clickhouse-server/config.d/prometheus.xml -->
<clickhouse>
    <prometheus>
        <endpoint>/metrics</endpoint>
        <port>9363</port>
        <metrics>true</metrics>
        <events>true</events>
        <asynchronous_metrics>true</asynchronous_metrics>
        <status_info>true</status_info>
    </prometheus>
</clickhouse>
```

**Prometheus Scrape Config**:
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'clickhouse'
    static_configs:
      - targets:
        - 'ch-01.example.com:9363'
        - 'ch-02.example.com:9363'
        - 'ch-03.example.com:9363'
    honor_labels: true
    scrape_interval: 30s
    params:
      filtered_metrics: ['true']  # Only critical metrics
```

**Key Prometheus Metrics**:
- `ClickHouseMetrics_Query` - Current queries running
- `ClickHouseMetrics_MemoryTracking` - Memory usage
- `ClickHouseMetrics_ReplicasMaxQueueSize` - Replication lag
- `ClickHouseAsyncMetrics_ReplicasSumMergesInQueue` - Merge backlog
- `ClickHouseProfileEvents_Query` - Query rate

### CHD-028: 3. Critical Alerts

| Metric | Threshold | Severity | Action |
|--------|-----------|----------|--------|
| **Parts count** | >300 per partition | Warning | Increase batch size |
| **Replication queue** | >100 | Critical | Check network, Keeper |
| **Memory usage** | >80% | Warning | Scale up, reduce queries |
| **Disk usage** | >85% | Critical | Add storage, cleanup |
| **Query duration** | >60s | Warning | Optimize query |
| **Failed queries** | >5% | Critical | Investigate errors |
| **ZK/Keeper latency** | >50ms | Warning | Check Keeper load |

---

## Backup & Recovery

### CHD-029: 1. clickhouse-backup Tool

**Installation**:
```bash
# Download latest release
wget https://github.com/Altinity/clickhouse-backup/releases/download/v2.5.0/clickhouse-backup-linux-amd64.tar.gz
tar -xzf clickhouse-backup-linux-amd64.tar.gz
sudo mv clickhouse-backup /usr/local/bin/

# Create config
sudo mkdir -p /etc/clickhouse-backup
```

**Configuration**:
```yaml
# /etc/clickhouse-backup/config.yml
general:
  remote_storage: s3
  disable_progress_bar: false
  backups_to_keep_local: 3
  backups_to_keep_remote: 14
  log_level: info

clickhouse:
  host: localhost
  port: 9000
  username: backup_user
  password: backup_password
  secure: true
  skip_verify: false
  timeout: 5m
  freeze_by_part: false

s3:
  bucket: clickhouse-backups
  region: us-east-1
  access_key: AWS_ACCESS_KEY
  secret_key: AWS_SECRET_KEY
  path: backups/prod-cluster
  compression_format: zstd
  compression_level: 3
  sse: AES256
  storage_class: STANDARD_IA
```

### CHD-030: 2. Backup Commands

```bash
# Create local backup
clickhouse-backup create daily_backup_$(date +%Y%m%d)

# Upload to S3
clickhouse-backup upload daily_backup_20250130

# Create and upload in one command
clickhouse-backup create_remote daily_backup_$(date +%Y%m%d)

# List backups
clickhouse-backup list
clickhouse-backup list remote

# Download from S3
clickhouse-backup download daily_backup_20250130

# Restore backup
clickhouse-backup restore daily_backup_20250130

# Restore specific tables
clickhouse-backup restore --tables=database.table1,database.table2 daily_backup_20250130
```

### CHD-031: 3. Backup Strategy

**Production Backup Schedule**:
```bash
# /etc/cron.d/clickhouse-backup

# Daily incremental backup at 2 AM
0 2 * * * root /usr/local/bin/clickhouse-backup create_remote daily_$(date +\%Y\%m\%d) 2>&1 | logger -t clickhouse-backup

# Weekly full backup on Sunday at 3 AM
0 3 * * 0 root /usr/local/bin/clickhouse-backup create_remote weekly_$(date +\%Y\%m\%d) 2>&1 | logger -t clickhouse-backup

# Monthly cleanup (keep last 14 backups)
0 4 1 * * root /usr/local/bin/clickhouse-backup delete remote $(clickhouse-backup list remote | tail -n +15) 2>&1 | logger -t clickhouse-backup
```

**Backup Requirements**:
- **Frequency**: Daily minimum, hourly для critical data
- **Retention**: 7 days local, 30-90 days remote
- **Replicas**: Backup at least one replica per shard
- **Testing**: Monthly restore tests
- **Validation**: Verify backup integrity после creation

### CHD-032: 4. Point-in-Time Recovery

```bash
# Restore to specific timestamp (requires query log)
# 1. Find last good backup before incident
clickhouse-backup list remote

# 2. Restore backup
clickhouse-backup restore weekly_20250125

# 3. Replay query log
SELECT query
FROM system.query_log
WHERE type = 'QueryFinish'
  AND query_start_time > '2025-01-25 00:00:00'
  AND query_start_time < '2025-01-28 12:00:00'
  AND query LIKE 'INSERT%'
ORDER BY query_start_time;
```

---

## Kubernetes Deployment

### CHD-033: 1. Altinity Kubernetes Operator

**Installation via Helm**:
```bash
# Add Altinity repo
helm repo add altinity https://altinity.github.io/clickhouse-operator
helm repo update

# Install operator
helm install clickhouse-operator altinity/altinity-clickhouse-operator \
  --namespace clickhouse-system \
  --create-namespace \
  --set metrics.enabled=true

# Verify installation
kubectl get pods -n clickhouse-system
```

### CHD-034: 2. ClickHouseInstallation CRD

```yaml
apiVersion: clickhouse.altinity.com/v1
kind: ClickHouseInstallation
metadata:
  name: production-cluster
  namespace: clickhouse
spec:
  defaults:
    templates:
      podTemplate: clickhouse-pod
      volumeClaimTemplate: data-volume
      serviceTemplate: clickhouse-service

  configuration:
    # Cluster topology
    clusters:
      - name: analytics
        layout:
          shardsCount: 3
          replicasCount: 2
        templates:
          podTemplate: clickhouse-pod
          volumeClaimTemplate: data-volume

    # ClickHouse Keeper (ZooKeeper replacement)
    zookeeper:
      nodes:
        - host: keeper-0.keeper-headless.clickhouse.svc.cluster.local
          port: 2181
        - host: keeper-1.keeper-headless.clickhouse.svc.cluster.local
          port: 2181
        - host: keeper-2.keeper-headless.clickhouse.svc.cluster.local
          port: 2181

    # Users and access control
    users:
      admin/password_sha256_hex: "65e84be33532fb784c48129675f9eff3a682b27168c0ea744b2cf58ee02337c5"
      admin/networks/ip:
        - "10.0.0.0/8"
        - "172.16.0.0/12"

    # Server settings
    settings:
      max_memory_usage: 10000000000
      max_bytes_before_external_group_by: 5000000000
      max_concurrent_queries: 100

    # Files configuration
    files:
      config.d/storage.xml: |
        <clickhouse>
          <storage_configuration>
            <disks>
              <default>
                <keep_free_space_bytes>10737418240</keep_free_space_bytes>
              </default>
            </disks>
          </storage_configuration>
        </clickhouse>

  templates:
    # Pod template
    podTemplates:
      - name: clickhouse-pod
        spec:
          securityContext:
            runAsUser: 101
            runAsGroup: 101
            fsGroup: 101

          containers:
            - name: clickhouse
              image: clickhouse/clickhouse-server:25.8.13.73-alpine
              imagePullPolicy: IfNotPresent

              resources:
                requests:
                  memory: "16Gi"
                  cpu: "4"
                limits:
                  memory: "32Gi"
                  cpu: "8"

              livenessProbe:
                httpGet:
                  path: /ping
                  port: 8123
                initialDelaySeconds: 30
                periodSeconds: 10
                timeoutSeconds: 5

              readinessProbe:
                httpGet:
                  path: /ping
                  port: 8123
                initialDelaySeconds: 10
                periodSeconds: 5
                timeoutSeconds: 3

    # Volume claim templates
    volumeClaimTemplates:
      - name: data-volume
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 1Ti
          storageClassName: gp3  # AWS EBS gp3

    # Service templates
    serviceTemplates:
      - name: clickhouse-service
        spec:
          type: ClusterIP
          ports:
            - name: http
              port: 8123
              targetPort: 8123
            - name: tcp
              port: 9000
              targetPort: 9000
```

### CHD-035: 3. StatefulSet Best Practices

**Storage Classes**:
```yaml
# AWS EBS gp3 (recommended)
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp3-clickhouse
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  iops: "16000"
  throughput: "1000"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

**Anti-Affinity Rules**:
```yaml
spec:
  templates:
    podTemplates:
      - name: clickhouse-pod
        spec:
          affinity:
            podAntiAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                - labelSelector:
                    matchLabels:
                      clickhouse.altinity.com/chi: production-cluster
                  topologyKey: kubernetes.io/hostname
```

### CHD-036: 4. Service & Ingress

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: clickhouse-lb
  namespace: clickhouse
spec:
  type: LoadBalancer
  selector:
    clickhouse.altinity.com/chi: production-cluster
  ports:
    - name: https
      port: 8443
      targetPort: 8123
    - name: tcp-secure
      port: 9440
      targetPort: 9000

---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: clickhouse-ingress
  namespace: clickhouse
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - clickhouse.example.com
      secretName: clickhouse-tls
  rules:
    - host: clickhouse.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: clickhouse-lb
                port:
                  number: 8123
```

---

## Common Anti-Patterns

### CHD-037: 13 Deadly Sins

| Sin | Problem | Solution |
|-----|---------|----------|
| **Too Many Parts** | Slow queries, merge pressure | Batch inserts (1000+ rows) |
| **Too Many Partitions** | Metadata overhead | Monthly/weekly partitions only |
| **Wrong PRIMARY KEY** | Full table scans | Order by cardinality (low→high) |
| **Heavy Mutations** | CPU/IO intensive | Use ReplacingMergeTree |
| **Small Inserts** | Part explosion | Batch or async_insert=1 |
| **Complex Types** | Performance degradation | Use primitive types |
| **Overuse Skip Indices** | Slow inserts | Optimize primary key first |
| **LIMIT Misconception** | Still reads all data | Add WHERE clause |
| **Memory Exceeded** | Query failures | External sorting/grouping |
| **Too Many MVs** | Slow inserts | <50 MVs per table |
| **Experimental Features** | Instability | Test in dev, avoid prod |
| **No Monitoring** | Blind operations | Prometheus + alerts |
| **No Backups** | Data loss risk | Daily backups + testing |

### CHD-038: Query Anti-Patterns

```sql
-- BAD: Many small inserts
INSERT INTO events VALUES (1, now(), 'event1');
INSERT INTO events VALUES (2, now(), 'event2');
-- Creates TOO MANY PARTS!

-- GOOD: Batch inserts
INSERT INTO events VALUES
    (1, now(), 'event1'),
    (2, now(), 'event2'),
    -- ... 1000+ rows
    (1000, now(), 'event1000');

-- GOOD: Use async inserts
SET async_insert = 1, wait_for_async_insert = 0;
INSERT INTO events VALUES (1, now(), 'event1');

-- BAD: SELECT * from large table
SELECT * FROM events WHERE event_date = today();

-- GOOD: Select only needed columns
SELECT user_id, event_type, event_time
FROM events
WHERE event_date = today();

-- BAD: Large table JOIN
SELECT * FROM huge_facts f JOIN huge_dims d ON f.id = d.id;

-- GOOD: Use GLOBAL JOIN
SELECT * FROM huge_facts f GLOBAL JOIN small_dims d ON f.id = d.id;

-- BETTER: Use Dictionary
SELECT *, dictGet('dims_dict', 'name', dim_id) AS name FROM huge_facts;
```

---

## Quick Reference

### CHD-039: Production Deployment Checklist

- [ ] LTS version installed (v25.8.13.73)
- [ ] System tuning: CPU governor, filesystem, network
- [ ] TLS/SSL configured, insecure ports disabled
- [ ] RBAC users configured, default user secured
- [ ] 3+ replicas per shard
- [ ] ClickHouse Keeper cluster (3 nodes)
- [ ] Load balancer configured (CHProxy/nginx)
- [ ] Resource limits set (memory, CPU)
- [ ] Monitoring enabled (Prometheus)
- [ ] Alerts configured (parts, replication, memory)
- [ ] Backup automation (daily, tested)
- [ ] Storage policy configured (tiered if needed)
- [ ] Network policies/firewall rules
- [ ] Query quotas configured

### CHD-040: Critical Commands

```bash
# Check cluster health
clickhouse-client --query "SELECT * FROM system.clusters"

# Check replication lag
clickhouse-client --query "SELECT * FROM system.replicas WHERE queue_size > 0"

# Restart stuck replica
clickhouse-client --query "SYSTEM RESTART REPLICA database.table"

# Kill long query
clickhouse-client --query "KILL QUERY WHERE query_id = 'query_id'"

# Force merge
clickhouse-client --query "OPTIMIZE TABLE database.table FINAL"

# Clear caches
clickhouse-client --query "SYSTEM DROP MARK CACHE"
```

---

## Дополнительные ресурсы

### CHD-041: Official Documentation
- [ClickHouse Documentation](https://clickhouse.com/docs) - Comprehensive docs
- [ClickHouse Blog](https://clickhouse.com/blog) - Technical articles
- [ClickHouse GitHub](https://github.com/ClickHouse/ClickHouse) - Source code

### CHD-042: Altinity Resources
- [Altinity Documentation](https://docs.altinity.com/) - Enterprise guides
- [Altinity Knowledge Base](https://kb.altinity.com/) - Practical solutions
- [Altinity Kubernetes Operator](https://github.com/Altinity/clickhouse-operator)

### CHD-043: Tools
- [CHProxy](https://github.com/ContentSquare/chproxy) - Load balancer & cache
- [clickhouse-backup](https://github.com/Altinity/clickhouse-backup) - Backup utility
- [Grafana Dashboards](https://grafana.com/grafana/dashboards/?search=clickhouse)

### CHD-044: Research Report
См. полный отчет: `.memory_bank/agent_task_results/2025-12-29_web-research_clickhouse-devops-best-practices.md`

---

**При возникновении вопросов**: Обратись к DevOps engineer agent или см. research report.
