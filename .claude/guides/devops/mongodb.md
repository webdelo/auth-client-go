# MongoDB Production Best Practices and Standards

This document contains MongoDB deployment rules for DevOps tasks. Each rule has a unique identifier (MGD-XXXX) for reference in code reviews and task execution.

> **Related**: For MongoDB query patterns, see `dba/mongodb/`
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми MongoDB задачами
**Текущая версия MongoDB**: 8.0 (2025)

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| MGD-001 | Use WiredTiger storage engine | ✅ | [Storage Engine](#mgd-001-use-wiredtiger-storage-engine) |
| MGD-002 | Configure replica set for HA | ✅ | [Replica Set](#mgd-002-configure-replica-set-for-ha) |
| MGD-003 | Set appropriate write concern | ✅ | [Write Concern](#mgd-003-set-appropriate-write-concern) |

---

## Содержание

1. [Architecture Patterns](#architecture-patterns)
2. [Load Balancing](#load-balancing)
3. [Installation & Configuration](#installation--configuration)
4. [Security Standards](#security-standards)
5. [Replication & High Availability](#replication--high-availability)
6. [Sharding & Horizontal Scaling](#sharding--horizontal-scaling)
7. [Performance Optimization](#performance-optimization)
8. [Monitoring & Observability](#monitoring--observability)
9. [Backup & Recovery](#backup--recovery)
10. [Production Patterns](#production-patterns)
11. [Common Gotchas](#common-gotchas)

---

## Architecture Patterns

### MGD-001: MongoDB Deployment Types

| Type | Description | Use Case | HA | Scale |
|------|-------------|----------|-----|-------|
| **Standalone** | Single mongod instance | Dev/Test only | No | Vertical |
| **Replica Set** | 3+ nodes with automatic failover | Production apps | Yes | Read scale |
| **Sharded Cluster** | Distributed across shards | Large datasets (TB+) | Yes | Horizontal |

**Production Rule**: NEVER use standalone in production. Минимум - Replica Set из 3 узлов.

### MGD-002: Components Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Application Layer                    │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│                 mongos (Query Router)                   │  ← Load balancing point
│              (Multiple instances for HA)                │
└─────────────────────┬───────────────────────────────────┘
                      │
         ┌────────────┴────────────┐
         ▼                         ▼
┌──────────────────┐      ┌──────────────────┐
│   Config Servers │      │  Shard Replicas  │
│   (Replica Set)  │      │  (Replica Sets)  │
│                  │      │                  │
│  - Metadata      │      │  - Shard 1 RS    │
│  - Chunk info    │      │  - Shard 2 RS    │
│  - Routing       │      │  - Shard N RS    │
└──────────────────┘      └──────────────────┘
```

---

## Load Balancing

### MGD-003: 1. Replica Set Load Balancing

#### Read Preference Modes

| Mode | Behavior | Use Case | Consistency |
|------|----------|----------|-------------|
| **primary** | Reads from primary only | Strong consistency required | Strongest |
| **primaryPreferred** | Primary, fallback to secondary | Default for most apps | Strong |
| **secondary** | Reads from secondary only | Analytics, reporting | Eventual |
| **secondaryPreferred** | Secondary, fallback to primary | Reduce primary load | Eventual |
| **nearest** | Lowest network latency | Geo-distributed apps | Eventual |

#### Connection String Examples

```bash
# Primary only (default)
mongodb://mongo1:27017,mongo2:27017,mongo3:27017/mydb?replicaSet=rs0

# Secondary preferred (offload reads)
mongodb://mongo1:27017,mongo2:27017,mongo3:27017/mydb?replicaSet=rs0&readPreference=secondaryPreferred

# Nearest (geo-distributed)
mongodb://mongo1:27017,mongo2:27017,mongo3:27017/mydb?replicaSet=rs0&readPreference=nearest&maxStalenessSeconds=90
```

#### Go Driver Example

```go
package main

import (
    "context"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/mongo/readpref"
)

func NewMongoClient() (*mongo.Client, error) {
    ctx := context.Background()

    // Connection pool settings
    clientOpts := options.Client().
        ApplyURI("mongodb://mongo1:27017,mongo2:27017,mongo3:27017/mydb?replicaSet=rs0").
        SetMaxPoolSize(100).         // Max connections
        SetMinPoolSize(10).          // Min connections
        SetMaxConnIdleTime(60 * time.Second).
        SetServerSelectionTimeout(5 * time.Second)

    // Read preference: secondary preferred
    clientOpts.SetReadPreference(readpref.SecondaryPreferred())

    // Write concern: majority
    clientOpts.SetWriteConcern(writeconcern.Majority())

    // Read concern: majority
    clientOpts.SetReadConcern(readconcern.Majority())

    return mongo.Connect(ctx, clientOpts)
}

// Different read preferences for different operations
func QueryExamples(db *mongo.Database) {
    ctx := context.Background()

    // Critical read - primary only
    opts := options.FindOne().SetReadPreference(readpref.Primary())
    var result User
    db.Collection("users").FindOne(ctx, bson.M{"_id": userID}, opts).Decode(&result)

    // Analytics - secondary preferred
    opts = options.Find().SetReadPreference(readpref.SecondaryPreferred())
    cursor, _ := db.Collection("events").Find(ctx, bson.M{}, opts)

    // Geo-distributed - nearest
    opts = options.FindOne().SetReadPreference(readpref.Nearest())
    db.Collection("articles").FindOne(ctx, bson.M{"slug": slug}, opts).Decode(&result)
}
```

#### Priority & Tags для Advanced Routing

```javascript
// Replica set configuration с приоритетами и тегами
rs.reconfig({
  _id: "rs0",
  members: [
    {
      _id: 0,
      host: "mongo1:27017",
      priority: 2,  // Предпочтительный primary
      tags: { dc: "us-east", usage: "production" }
    },
    {
      _id: 1,
      host: "mongo2:27017",
      priority: 1,
      tags: { dc: "us-east", usage: "production" }
    },
    {
      _id: 2,
      host: "mongo3:27017",
      priority: 1,
      tags: { dc: "us-west", usage: "analytics" }
    },
    {
      _id: 3,
      host: "mongo4:27017",
      priority: 0,  // Never becomes primary
      hidden: true, // Invisible to clients
      tags: { usage: "backup" }
    }
  ]
})
```

**Tag-based Read Preference**:

```go
// Read from analytics nodes only
tag := tag.Set{
    {Name: "usage", Value: "analytics"},
}
rp := readpref.Nearest(readpref.WithTagSets(tag))
opts := options.Find().SetReadPreference(rp)
```

### MGD-004: 2. Sharded Cluster Load Balancing

#### mongos Query Router

**Правило**: Запускай МИНИМУМ 2 mongos instances для high availability.

```
Application ──┬──> mongos1 (L4 LB)
              ├──> mongos2
              └──> mongos3
```

#### mongos Configuration

```yaml
# mongos.conf
net:
  port: 27017
  bindIp: 0.0.0.0

sharding:
  configDB: configReplSet/config1:27017,config2:27017,config3:27017

systemLog:
  destination: file
  path: /var/log/mongodb/mongos.log
  logAppend: true

processManagement:
  fork: true
  pidFilePath: /var/run/mongodb/mongos.pid
```

#### L4 Load Balancer для mongos

**HAProxy Example**:

```haproxy
# /etc/haproxy/haproxy.cfg
frontend mongodb_frontend
    bind *:27017
    mode tcp
    option tcplog
    default_backend mongodb_backend

backend mongodb_backend
    mode tcp
    balance leastconn  # Connection-based balancing
    option tcp-check

    # Health check
    tcp-check connect
    tcp-check send-binary 3a000000  # MongoDB ping
    tcp-check expect binary 01

    server mongos1 10.0.1.10:27017 check inter 5s fall 3 rise 2
    server mongos2 10.0.1.11:27017 check inter 5s fall 3 rise 2
    server mongos3 10.0.1.12:27017 check inter 5s fall 3 rise 2
```

**NGINX TCP Load Balancer**:

```nginx
stream {
    upstream mongodb_cluster {
        least_conn;
        server mongos1:27017 max_fails=3 fail_timeout=30s;
        server mongos2:27017 max_fails=3 fail_timeout=30s;
        server mongos3:27017 max_fails=3 fail_timeout=30s;
    }

    server {
        listen 27017;
        proxy_pass mongodb_cluster;
        proxy_connect_timeout 5s;
        proxy_timeout 24h;  # Long-lived connections
    }
}
```

#### Kubernetes Service for mongos

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mongodb-mongos
spec:
  type: ClusterIP
  sessionAffinity: ClientIP  # Sticky sessions
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 10800  # 3 hours
  ports:
  - port: 27017
    targetPort: 27017
    protocol: TCP
  selector:
    app: mongos
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mongos
spec:
  replicas: 3  # Multiple mongos instances
  selector:
    matchLabels:
      app: mongos
  template:
    metadata:
      labels:
        app: mongos
    spec:
      containers:
      - name: mongos
        image: mongo:8.0
        command:
        - mongos
        - --configdb
        - configReplSet/config-0.config-service:27017,config-1.config-service:27017,config-2.config-service:27017
        - --bind_ip_all
        ports:
        - containerPort: 27017
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          tcpSocket:
            port: 27017
          initialDelaySeconds: 30
          periodSeconds: 10
```

### MGD-005: 3. Shard Key Design для Equal Distribution

**Правило**: Выбор shard key критичен для равномерного распределения нагрузки.

#### Good Shard Keys

| Pattern | Cardinality | Write Distribution | Query Routing |
|---------|-------------|-------------------|---------------|
| **Hashed _id** | High | Excellent | Poor (scatter-gather) |
| **user_id (hashed)** | High | Excellent | Good (targeted) |
| **location + timestamp** | High | Good | Excellent (range) |

#### Bad Shard Keys

| Pattern | Problem |
|---------|---------|
| **timestamp** | Monotonically increasing → all writes to one shard |
| **country** | Low cardinality → uneven distribution |
| **status** | Few values → hotspots |

#### Shard Key Examples

```javascript
// BAD: Monotonically increasing
sh.shardCollection("mydb.orders", { order_date: 1 })

// GOOD: Hashed _id (random distribution)
sh.shardCollection("mydb.orders", { _id: "hashed" })

// GOOD: Compound key (location-aware + distributed)
sh.shardCollection("mydb.orders", { region: 1, _id: 1 })

// EXCELLENT: Hashed field + range field
sh.shardCollection("mydb.events", { user_id: "hashed", timestamp: 1 })
```

### MGD-006: 4. Zone Sharding (Geo-Distribution)

```javascript
// Define zones
sh.addShardTag("shard0", "US-EAST")
sh.addShardTag("shard1", "US-WEST")
sh.addShardTag("shard2", "EU")

// Assign ranges to zones
sh.addTagRange(
  "mydb.users",
  { region: "us-east", _id: MinKey },
  { region: "us-east", _id: MaxKey },
  "US-EAST"
)

sh.addTagRange(
  "mydb.users",
  { region: "eu", _id: MinKey },
  { region: "eu", _id: MaxKey },
  "EU"
)
```

**Benefits**:
- Data locality (reduced latency)
- Regulatory compliance (GDPR)
- Cost optimization (regional pricing)

---

## Installation & Configuration

### MGD-007: 1. System Requirements (Production)

| Component | CPU | RAM | Disk | IOPS |
|-----------|-----|-----|------|------|
| **mongod (Replica)** | 4+ cores | 16+ GB | SSD | 3000+ |
| **mongod (Config Server)** | 2 cores | 4 GB | SSD | 1000+ |
| **mongos** | 2 cores | 4 GB | - | - |

**Disk**: ОБЯЗАТЕЛЬНО SSD или NVMe. HDD недопустимы для production.

### MGD-008: 2. Installation (Ubuntu 24.04)

```bash
# Import MongoDB GPG key
curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | \
  sudo gpg --dearmor -o /usr/share/keyrings/mongodb-server-8.0.gpg

# Add MongoDB repository
echo "deb [ arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-8.0.gpg ] \
https://repo.mongodb.org/apt/ubuntu noble/mongodb-org/8.0 multiverse" | \
  sudo tee /etc/apt/sources.list.d/mongodb-org-8.0.list

# Install
sudo apt update
sudo apt install -y mongodb-org

# Pin version (prevent auto-upgrade)
echo "mongodb-org hold" | sudo dpkg --set-selections
```

### MGD-009: 3. Production Configuration

```yaml
# /etc/mongod.conf
storage:
  dbPath: /var/lib/mongodb
  journal:
    enabled: true
  wiredTiger:
    engineConfig:
      cacheSizeGB: 8  # 50-60% of RAM
      journalCompressor: snappy
    collectionConfig:
      blockCompressor: snappy
    indexConfig:
      prefixCompression: true

systemLog:
  destination: file
  path: /var/log/mongodb/mongod.log
  logAppend: true
  logRotate: reopen
  timeStampFormat: iso8601-utc
  component:
    accessControl:
      verbosity: 1
    command:
      verbosity: 1

net:
  port: 27017
  bindIp: 0.0.0.0  # ВАЖНО: Используй firewall для ограничения доступа
  maxIncomingConnections: 65536
  compression:
    compressors: snappy,zstd,zlib

processManagement:
  fork: true
  pidFilePath: /var/run/mongodb/mongod.pid
  timeZoneInfo: /usr/share/zoneinfo

replication:
  replSetName: rs0

security:
  authorization: enabled
  keyFile: /etc/mongodb/keyfile  # Для replica set auth

operationProfiling:
  mode: slowOp
  slowOpThresholdMs: 100
  slowOpSampleRate: 1.0

setParameter:
  # Connection pool
  connPoolMaxConnsPerHost: 200

  # Query optimization
  internalQueryExecMaxBlockingSortBytes: 33554432  # 32MB

  # Diagnostic data
  diagnosticDataCollectionEnabled: true
```

### MGD-010: 4. System Tuning (CRITICAL)

#### Disable Transparent Huge Pages (THP)

```bash
# /etc/systemd/system/disable-transparent-huge-pages.service
[Unit]
Description=Disable Transparent Huge Pages (THP)
After=sysinit.target local-fs.target
Before=mongod.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'echo never | tee /sys/kernel/mm/transparent_hugepage/enabled > /dev/null'
ExecStart=/bin/sh -c 'echo never | tee /sys/kernel/mm/transparent_hugepage/defrag > /dev/null'

[Install]
WantedBy=basic.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable disable-transparent-huge-pages
sudo systemctl start disable-transparent-huge-pages
```

#### File Descriptor Limits

```bash
# /etc/security/limits.conf
mongodb soft nofile 64000
mongodb hard nofile 64000
mongodb soft nproc 64000
mongodb hard nproc 64000
```

#### Kernel Parameters

```bash
# /etc/sysctl.conf
# Increase max connections
net.core.somaxconn = 4096
net.ipv4.tcp_max_syn_backlog = 4096

# TCP keepalive (detect dead connections)
net.ipv4.tcp_keepalive_time = 120
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 9

# VM settings
vm.swappiness = 1  # Minimize swap usage
vm.dirty_ratio = 15
vm.dirty_background_ratio = 5

# Apply
sudo sysctl -p
```

### MGD-011: 5. Docker Deployment

```yaml
# docker-compose.yml для Replica Set
services:
  mongo1:
    image: mongo:8.0
    container_name: mongo1
    command: mongod --replSet rs0 --bind_ip_all --keyFile /etc/mongodb/keyfile
    ports:
      - "27017:27017"
    volumes:
      - mongo1_data:/data/db
      - ./keyfile:/etc/mongodb/keyfile:ro
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD_FILE: /run/secrets/mongo_root_password
    secrets:
      - mongo_root_password
    networks:
      - mongo_cluster
    restart: unless-stopped

  mongo2:
    image: mongo:8.0
    container_name: mongo2
    command: mongod --replSet rs0 --bind_ip_all --keyFile /etc/mongodb/keyfile
    ports:
      - "27018:27017"
    volumes:
      - mongo2_data:/data/db
      - ./keyfile:/etc/mongodb/keyfile:ro
    networks:
      - mongo_cluster
    restart: unless-stopped

  mongo3:
    image: mongo:8.0
    container_name: mongo3
    command: mongod --replSet rs0 --bind_ip_all --keyFile /etc/mongodb/keyfile
    ports:
      - "27019:27017"
    volumes:
      - mongo3_data:/data/db
      - ./keyfile:/etc/mongodb/keyfile:ro
    networks:
      - mongo_cluster
    restart: unless-stopped

volumes:
  mongo1_data:
  mongo2_data:
  mongo3_data:

networks:
  mongo_cluster:
    driver: bridge

secrets:
  mongo_root_password:
    file: ./secrets/mongo_root_password.txt
```

**Generate keyfile**:

```bash
openssl rand -base64 756 > keyfile
chmod 400 keyfile
```

**Initialize Replica Set**:

```bash
docker exec -it mongo1 mongosh -u admin -p

# In mongosh
rs.initiate({
  _id: "rs0",
  members: [
    { _id: 0, host: "mongo1:27017" },
    { _id: 1, host: "mongo2:27017" },
    { _id: 2, host: "mongo3:27017" }
  ]
})

rs.status()
```

---

## Security Standards

### MGD-012: 1. Authentication (ОБЯЗАТЕЛЬНО)

**Правило**: НИКОГДА не запускай MongoDB без authentication в production.

#### Create Admin User

```javascript
use admin
db.createUser({
  user: "admin",
  pwd: passwordPrompt(),  // Prompt for password
  roles: [
    { role: "userAdminAnyDatabase", db: "admin" },
    { role: "readWriteAnyDatabase", db: "admin" },
    { role: "dbAdminAnyDatabase", db: "admin" },
    { role: "clusterAdmin", db: "admin" }
  ]
})
```

#### Application User (Least Privilege)

```javascript
use mydb
db.createUser({
  user: "myapp",
  pwd: passwordPrompt(),
  roles: [
    { role: "readWrite", db: "mydb" }
  ]
})
```

#### Built-in Roles

| Role | Permissions |
|------|-------------|
| `read` | Read data |
| `readWrite` | Read + write data |
| `dbAdmin` | Manage database (indexes, stats) |
| `userAdmin` | Manage users |
| `clusterAdmin` | Cluster management |
| `root` | Superuser (avoid) |

### MGD-013: 2. Network Security

#### Firewall Rules (iptables)

```bash
# Allow MongoDB from application servers only
sudo iptables -A INPUT -p tcp -s 10.0.1.0/24 --dport 27017 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 27017 -j DROP

# Save rules
sudo netfilter-persistent save
```

#### TLS/SSL (REQUIRED for production)

```yaml
# mongod.conf
net:
  tls:
    mode: requireTLS
    certificateKeyFile: /etc/ssl/mongodb.pem
    CAFile: /etc/ssl/ca.pem
    allowConnectionsWithoutCertificates: false
```

**Generate Self-Signed Certificate** (Dev only):

```bash
openssl req -newkey rsa:2048 -nodes -keyout mongodb.key -x509 -days 365 -out mongodb.crt
cat mongodb.key mongodb.crt > mongodb.pem
```

**Production**: Используй Let's Encrypt или корпоративный CA.

### MGD-014: 3. Encryption at Rest

```yaml
# mongod.conf (MongoDB Enterprise only)
security:
  enableEncryption: true
  encryptionKeyFile: /etc/mongodb/encryption-key

# For Community Edition - use LUKS disk encryption
```

### MGD-015: 4. Auditing (Enterprise)

```yaml
# mongod.conf
auditLog:
  destination: file
  format: JSON
  path: /var/log/mongodb/audit.json
  filter: '{ atype: { $in: ["authenticate", "createUser", "dropDatabase"] } }'
```

---

## Replication & High Availability

### MGD-016: 1. Replica Set Architecture

**Правило**: Минимум 3 узла (1 primary + 2 secondaries) для автоматического failover.

```
┌─────────────┐
│   Primary   │ ← Все записи
│  (mongo1)   │ ← Reads (readPreference=primary)
└──────┬──────┘
       │ Oplog replication
   ┌───┴────┐
   ▼        ▼
┌──────┐ ┌──────┐
│ Sec1 │ │ Sec2 │ ← Reads (secondaryPreferred)
└──────┘ └──────┘
```

### MGD-017: 2. Initialization

```javascript
rs.initiate({
  _id: "rs0",
  version: 1,
  members: [
    { _id: 0, host: "mongo1.example.com:27017", priority: 2 },
    { _id: 1, host: "mongo2.example.com:27017", priority: 1 },
    { _id: 2, host: "mongo3.example.com:27017", priority: 1 }
  ]
})
```

### MGD-018: 3. Member Types

| Type | Votes | Priority | Use Case |
|------|-------|----------|----------|
| **Data-bearing** | Yes | >0 | Normal member |
| **Arbiter** | Yes | 0 | Tie-breaker (avoid) |
| **Hidden** | Yes | 0 | Backups, analytics |
| **Delayed** | Yes | 0 | Disaster recovery |

#### Add Hidden Member (Backups)

```javascript
rs.add({
  host: "mongo4:27017",
  priority: 0,
  hidden: true,
  tags: { usage: "backup" }
})
```

#### Add Delayed Member (Point-in-time recovery)

```javascript
rs.add({
  host: "mongo5:27017",
  priority: 0,
  slaveDelay: 3600,  // 1 hour delay
  hidden: true
})
```

### MGD-019: 4. Write Concern

**Правило**: Используй `w: majority` для критичных данных.

```go
// Go driver example
wc := writeconcern.Majority()
wc = wc.WithJournal(true)  // Durability guarantee

opts := options.Client().SetWriteConcern(wc)
client, _ := mongo.Connect(ctx, opts)
```

| Write Concern | Durability | Performance | Use Case |
|---------------|------------|-------------|----------|
| `w: 1` | Low | Fast | Logs, metrics |
| `w: majority` | High | Slower | Financial data |
| `w: 2` | Medium | Medium | General apps |

### MGD-020: 5. Read Concern

```go
rc := readconcern.Majority()  // Read committed data only
opts := options.Find().SetReadConcern(rc)
```

| Read Concern | Isolation | Use Case |
|--------------|-----------|----------|
| `local` | Read uncommitted | Default |
| `majority` | Read committed | Strong consistency |
| `linearizable` | Serializable | Strongest (slow) |

---

## Sharding & Horizontal Scaling

### MGD-021: 1. When to Shard?

Shard when:
- Dataset > 2 TB
- Working set не умещается в RAM
- Write throughput превышает single server capacity
- Geographic distribution требуется

**Не shard если**:
- Dataset < 500 GB → Используй Replica Set
- Можно масштабировать vertically

### MGD-022: 2. Sharded Cluster Setup

#### Config Servers (Replica Set)

```yaml
# config-server.conf
sharding:
  clusterRole: configsvr

replication:
  replSetName: configReplSet
```

```bash
# Initialize config server replica set
mongosh --host config1:27019

rs.initiate({
  _id: "configReplSet",
  configsvr: true,
  members: [
    { _id: 0, host: "config1:27019" },
    { _id: 1, host: "config2:27019" },
    { _id: 2, host: "config3:27019" }
  ]
})
```

#### Shard Servers (Replica Sets)

```yaml
# shard.conf
sharding:
  clusterRole: shardsvr

replication:
  replSetName: shard01
```

```bash
# Initialize each shard replica set
mongosh --host shard1a:27018

rs.initiate({
  _id: "shard01",
  members: [
    { _id: 0, host: "shard1a:27018" },
    { _id: 1, host: "shard1b:27018" },
    { _id: 2, host: "shard1c:27018" }
  ]
})
```

#### Add Shards to Cluster

```javascript
// Connect to mongos
mongosh --host mongos:27017

sh.addShard("shard01/shard1a:27018,shard1b:27018,shard1c:27018")
sh.addShard("shard02/shard2a:27018,shard2b:27018,shard2c:27018")

sh.status()
```

### MGD-023: 3. Enable Sharding

```javascript
// Enable sharding for database
sh.enableSharding("mydb")

// Shard collection
sh.shardCollection("mydb.users", { user_id: "hashed" })

// Compound shard key
sh.shardCollection("mydb.events", { region: 1, timestamp: 1 })
```

### MGD-024: 4. Balancer Management

```javascript
// Check balancer status
sh.getBalancerState()

// Stop balancer (maintenance)
sh.stopBalancer()

// Start balancer
sh.startBalancer()

// Schedule balancing window
db.settings.update(
  { _id: "balancer" },
  { $set: { activeWindow: { start: "23:00", stop: "06:00" } } },
  { upsert: true }
)
```

---

## Performance Optimization

### MGD-025: 1. Indexing Strategy

#### Index Types

| Type | Use Case | Syntax |
|------|----------|--------|
| **Single Field** | Simple queries | `{ field: 1 }` |
| **Compound** | Multiple fields | `{ field1: 1, field2: -1 }` |
| **Text** | Full-text search | `{ field: "text" }` |
| **Geospatial** | Location queries | `{ location: "2dsphere" }` |
| **Hashed** | Shard keys | `{ field: "hashed" }` |
| **Wildcard** | Flexible schemas | `{ "$**": 1 }` |

#### Create Indexes

```javascript
// Background index (non-blocking)
db.users.createIndex({ email: 1 }, { background: true, unique: true })

// Compound index (query order matters)
db.orders.createIndex({ user_id: 1, created_at: -1 })

// TTL index (auto-delete old documents)
db.sessions.createIndex({ created_at: 1 }, { expireAfterSeconds: 3600 })

// Partial index (space optimization)
db.orders.createIndex(
  { status: 1 },
  { partialFilterExpression: { status: "pending" } }
)
```

**Правило**: Index Selectivity - индексируй high-cardinality fields первыми.

```javascript
// GOOD: High selectivity field first
db.users.createIndex({ email: 1, country: 1 })

// BAD: Low selectivity field first
db.users.createIndex({ country: 1, email: 1 })
```

### MGD-026: 2. Query Optimization

#### Explain Plans

```javascript
// Analyze query performance
db.users.find({ email: "user@example.com" }).explain("executionStats")

// Key metrics:
// - executionTimeMillis
// - totalDocsExamined (should be ~ nReturned)
// - indexName (should use index)
```

#### Covered Queries (Best Performance)

```javascript
// Create covering index
db.users.createIndex({ email: 1, name: 1, status: 1 })

// Query only indexed fields
db.users.find(
  { email: "user@example.com" },
  { _id: 0, name: 1, status: 1 }  // Exclude _id
)
// Result: IXSCAN (index-only, no document fetch)
```

### MGD-027: 3. Connection Pooling

```go
// Go driver - optimal connection pool
opts := options.Client().
    SetMaxPoolSize(100).        // Max connections
    SetMinPoolSize(10).         // Warm connections
    SetMaxConnIdleTime(60 * time.Second).
    SetServerSelectionTimeout(5 * time.Second).
    SetSocketTimeout(30 * time.Second)
```

**Formula**: MaxPoolSize ≈ (expected concurrent requests) × 1.2

### MGD-028: 4. Aggregation Pipeline Optimization

```javascript
// GOOD: Filter early, project late
db.orders.aggregate([
  { $match: { status: "pending" } },      // Filter first
  { $sort: { created_at: -1 } },          // Sort early (use index)
  { $limit: 100 },                        // Limit early
  { $lookup: { ... } },                   // Join later
  { $project: { ... } }                   // Project last
])

// BAD: Project early (processes all docs)
db.orders.aggregate([
  { $project: { ... } },                  // Too early
  { $match: { status: "pending" } }       // Should be first
])
```

### MGD-029: 5. Bulk Operations

```go
// Ordered bulk write
models := []mongo.WriteModel{
    mongo.NewInsertOneModel().SetDocument(doc1),
    mongo.NewUpdateOneModel().SetFilter(...).SetUpdate(...),
    mongo.NewDeleteOneModel().SetFilter(...),
}

opts := options.BulkWrite().SetOrdered(false)  // Parallel execution
result, err := collection.BulkWrite(ctx, models, opts)
```

---

## Monitoring & Observability

### MGD-030: 1. Key Metrics (CRITICAL)

| Category | Metric | Threshold | Action |
|----------|--------|-----------|--------|
| **Performance** | Operation latency | >100ms | Optimize queries |
| **Replication** | Replication lag | >10s | Investigate oplog |
| **Connections** | Active connections | >80% pool | Scale or optimize |
| **Memory** | WiredTiger cache | >95% | Increase RAM |
| **Disk** | IOPS utilization | >80% | Upgrade storage |
| **Oplog** | Oplog window | <24h | Increase oplog size |

### MGD-031: 2. MongoDB Cloud Manager / Ops Manager

**Features**:
- Real-time performance metrics
- Automated backups
- Point-in-time recovery
- Query performance advisor
- Index suggestions

### MGD-032: 3. Prometheus + Grafana

#### MongoDB Exporter

```yaml
# docker-compose.yml
services:
  mongodb-exporter:
    image: percona/mongodb_exporter:0.40
    command:
      - '--mongodb.uri=mongodb://monitor:password@mongo1:27017'
      - '--collect-all'
    ports:
      - "9216:9216"
```

#### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'mongodb'
    static_configs:
      - targets: ['mongodb-exporter:9216']
```

#### Key Prometheus Queries

```promql
# Replication lag
mongodb_replset_member_replication_lag

# Operation latency
rate(mongodb_op_latencies_latency_total[5m])

# Connections
mongodb_connections{state="current"}

# Cache hit ratio
mongodb_wiredtiger_cache_bytes{type="dirty"} / mongodb_wiredtiger_cache_bytes{type="total"}
```

### MGD-033: 4. Slow Query Log

```javascript
// Enable profiling for slow queries
db.setProfilingLevel(1, { slowms: 100 })

// View slow queries
db.system.profile.find().sort({ ts: -1 }).limit(10)

// Analyze slow query
db.system.profile.find({ millis: { $gt: 1000 } })
```

### MGD-034: 5. Database Stats

```javascript
// Database statistics
db.stats()

// Collection statistics
db.users.stats()

// Index usage
db.users.aggregate([{ $indexStats: {} }])
```

---

## Backup & Recovery

### MGD-035: 1. Backup Strategies

| Method | Consistency | Downtime | Point-in-Time | Sharded |
|--------|-------------|----------|---------------|---------|
| **mongodump** | Consistent | No | No | Partial |
| **Filesystem Snapshot** | Consistent | Brief | No | Yes |
| **Ops Manager** | Consistent | No | Yes | Yes |
| **Cloud Manager** | Consistent | No | Yes | Yes |

### MGD-036: 2. mongodump / mongorestore

```bash
# Full backup
mongodump \
  --uri="mongodb://user:password@mongo1:27017/mydb?replicaSet=rs0&readPreference=secondary" \
  --out=/backups/$(date +%Y%m%d_%H%M%S) \
  --gzip \
  --oplog

# Incremental backup (oplog only)
mongodump \
  --uri="mongodb://user:password@mongo1:27017/local" \
  --collection=oplog.rs \
  --out=/backups/oplog_$(date +%Y%m%d_%H%M%S)

# Restore
mongorestore \
  --uri="mongodb://user:password@mongo1:27017" \
  --gzip \
  --oplogReplay \
  /backups/20250130_120000

# Specific collection
mongorestore \
  --uri="mongodb://user:password@mongo1:27017" \
  --collection=users \
  --db=mydb \
  /backups/20250130_120000/mydb/users.bson.gz
```

### MGD-037: 3. Filesystem Snapshots (Preferred)

```bash
# LVM snapshot
sudo lvcreate -L 50G -s -n mongo_snap /dev/vg0/mongo_data

# Mount snapshot
sudo mount /dev/vg0/mongo_snap /mnt/snapshot

# Copy data
tar czf /backups/mongo_$(date +%Y%m%d).tar.gz -C /mnt/snapshot .

# Cleanup
sudo umount /mnt/snapshot
sudo lvremove -y /dev/vg0/mongo_snap
```

### MGD-038: 4. Automated Backup Script

```bash
#!/bin/bash
# /usr/local/bin/mongodb-backup.sh

BACKUP_DIR="/backups/mongodb"
RETENTION_DAYS=7
MONGO_URI="mongodb://backup:password@localhost:27017/?replicaSet=rs0&readPreference=secondary"

# Create backup
BACKUP_PATH="${BACKUP_DIR}/$(date +%Y%m%d_%H%M%S)"
mongodump --uri="${MONGO_URI}" --out="${BACKUP_PATH}" --gzip --oplog

# Compress
tar czf "${BACKUP_PATH}.tar.gz" -C "${BACKUP_DIR}" "$(basename ${BACKUP_PATH})"
rm -rf "${BACKUP_PATH}"

# Upload to S3
aws s3 cp "${BACKUP_PATH}.tar.gz" "s3://my-backups/mongodb/"

# Cleanup old backups
find "${BACKUP_DIR}" -name "*.tar.gz" -mtime +${RETENTION_DAYS} -delete

# Log
echo "Backup completed: ${BACKUP_PATH}.tar.gz" | logger -t mongodb-backup
```

**Cron Schedule**:

```cron
# Daily at 2 AM
0 2 * * * /usr/local/bin/mongodb-backup.sh
```

### MGD-039: 5. Point-in-Time Recovery (Ops Manager)

**Manual PITR**:

```bash
# 1. Restore base backup
mongorestore --oplogReplay /backups/base_backup

# 2. Apply oplog up to specific timestamp
mongorestore \
  --oplogReplay \
  --oplogLimit "1706601600:0" \
  /backups/oplog_20250130
```

---

## Production Patterns

### MGD-040: 1. Capacity Planning

| Metric | Calculation | Example |
|--------|-------------|---------|
| **RAM** | Working set × 1.5 + 2GB overhead | 16 GB working set → 26 GB RAM |
| **CPU** | 4 cores minimum, 8+ для high load | 8 cores |
| **Disk** | Dataset × 1.2 (compression) + oplog (5-10% dataset) | 1TB data → 1.2TB + 100GB oplog |
| **IOPS** | 3000+ IOPS для SSD | SSD/NVMe required |

### MGD-041: 2. Connection String Best Practices

```go
// Production connection string
uri := "mongodb://user:password@mongo1:27017,mongo2:27017,mongo3:27017/mydb" +
    "?replicaSet=rs0" +
    "&readPreference=secondaryPreferred" +
    "&maxPoolSize=100" +
    "&minPoolSize=10" +
    "&maxIdleTimeMS=60000" +
    "&serverSelectionTimeoutMS=5000" +
    "&retryWrites=true" +
    "&w=majority" +
    "&readConcernLevel=majority" +
    "&authSource=admin" +
    "&tls=true" +
    "&tlsCAFile=/etc/ssl/ca.pem"
```

### MGD-042: 3. Application Retry Logic

```go
// Idempotent retry wrapper
func WithRetry(ctx context.Context, fn func() error) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }

        // Retry on transient errors
        if mongo.IsTimeout(err) || mongo.IsNetworkError(err) {
            backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
            time.Sleep(backoff)
            continue
        }

        return err  // Non-retryable app_error
    }
    return errors.New("max retries exceeded")
}
```

### MGD-043: 4. Schema Versioning

```javascript
// Document with schema version
{
  _id: ObjectId("..."),
  _schema_version: 2,
  user_id: 12345,
  email: "user@example.com",
  // ... fields
}

// Migration script
db.users.find({ _schema_version: { $exists: false } }).forEach(doc => {
  // Migrate v1 -> v2
  db.users.updateOne(
    { _id: doc._id },
    {
      $set: { _schema_version: 2, new_field: "default" },
      $unset: { old_field: "" }
    }
  )
})
```

### MGD-044: 5. Health Check Endpoint

```go
func HealthCheck(client *mongo.Client) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Ping database
    if err := client.Ping(ctx, readpref.Primary()); err != nil {
        return err
    }

    // Check replica set status
    var result bson.M
    err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}}).Decode(&result)
    if err != nil {
        return err
    }

    // Verify majority of members are healthy
    members := result["members"].(bson.A)
    healthy := 0
    for _, m := range members {
        member := m.(bson.M)
        if member["health"].(int32) == 1 {
            healthy++
        }
    }

    if healthy < len(members)/2+1 {
        return errors.New("majority of replica set members unhealthy")
    }

    return nil
}
```

---

## Common Gotchas

| Problem | Symptoms | Solution |
|---------|----------|----------|
| **Unbounded arrays** | Document size > 16MB | Use separate collection or bucketing |
| **Missing indexes** | Slow queries, COLLSCAN | Create indexes, monitor `system.profile` |
| **Oplog too small** | Replication lag, secondaries can't sync | Resize oplog (`replSetResizeOplog`) |
| **THP enabled** | Latency spikes | Disable Transparent Huge Pages |
| **No monitoring** | Blind to issues | Implement Prometheus + Grafana |
| **Poor shard key** | Hotspots, uneven distribution | Choose high-cardinality key |
| **Single mongos** | Single point of failure | Deploy 2+ mongos instances |
| **No backups** | Data loss risk | Automate daily backups + test restores |
| **Using `latest` tag** | Breaking changes | Pin specific version (e.g., `mongo:8.0.1`) |
| **No TLS** | Plaintext credentials | Enable TLS/SSL |

---

## Quick Reference

### MGD-045: mongosh Commands

```javascript
// Replica set
rs.status()              // Replica set status
rs.conf()                // Configuration
rs.add("host:port")      // Add member
rs.remove("host:port")   // Remove member
rs.stepDown()            // Force primary step down

// Sharding
sh.status()              // Sharding status
sh.enableSharding("db")  // Enable sharding
sh.shardCollection()     // Shard collection
sh.getBalancerState()    // Balancer status

// Diagnostics
db.currentOp()           // Current operations
db.killOp(opid)          // Kill operation
db.serverStatus()        // Server stats
db.stats()               // Database stats
db.collection.stats()    // Collection stats

// Profiling
db.setProfilingLevel(1, { slowms: 100 })
db.system.profile.find().sort({ ts: -1 })
```

### MGD-046: Backup Command

```bash
mongodump --uri="mongodb://user:pass@host:27017/db?replicaSet=rs0&readPreference=secondary" --out=/backup --gzip --oplog
```

### MGD-047: Health Check

```bash
mongosh --eval "db.adminCommand('ping')"
```

---

## Version Notes (2025)

| Version | Release | Key Features |
|---------|---------|--------------|
| **8.0** | Jul 2024 | Queryable Encryption GA, Time Series improvements |
| **7.0** | Aug 2023 | Cluster-to-cluster sync, encrypted collections |
| **6.0** | Jul 2022 | Time Series, versioned API |

**LTS Support**: 30 months от release date

---

## Дополнительные ресурсы

### MGD-048: Official Documentation
- [MongoDB Manual](https://www.mongodb.com/docs/manual/)
- [MongoDB Best Practices](https://www.mongodb.com/docs/manual/administration/production-notes/)
- [Sharding Guide](https://www.mongodb.com/docs/manual/sharding/)
- [Replication Guide](https://www.mongodb.com/docs/manual/replication/)

### MGD-049: Tools
- [MongoDB Compass](https://www.mongodb.com/products/compass) - GUI для MongoDB
- [Percona MongoDB Exporter](https://github.com/percona/mongodb_exporter) - Prometheus metrics

### MGD-050: Research Report
См. полный отчет: `.memory_bank/agent_task_results/2025-12-29_web-research_mongodb-devops-best-practices.md`

---

**При возникновении вопросов**: Обратись к DevOps engineer agent или см. research report.
