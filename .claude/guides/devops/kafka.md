# Apache Kafka Best Practices and Standards

This document contains Apache Kafka deployment rules for DevOps tasks. Each rule has a unique identifier (KF-XXXX) for reference in code reviews and task execution.

> **Related**: For Kafka producer/consumer patterns, see `internal/esb/kafka/`
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми Kafka задачами
**Текущая версия Kafka**: 3.9 (2025)

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| KF-001 | Use 3+ replicas for production | ✅ | [Replication](#kf-001-use-3-replicas-for-production) |
| KF-002 | Set min.insync.replicas >= 2 | ✅ | [High Availability](#kf-002-set-mininsync_replicas--2) |
| KF-003 | Configure retention by time and size | | [Log Retention](#kf-003-configure-retention-by-time-and-size) |
| KF-004 | Use separate ZooKeeper/KRaft cluster | ✅ | [Cluster Coordination](#kf-004-use-separate-zookeeper-kraft-cluster) |
| KF-005 | Monitor consumer lag | ✅ | [Consumer Lag Monitoring](#kf-005-monitor-consumer-lag) |

---

## Содержание

1. [Architecture Overview](#architecture-overview)
2. [Installation & Configuration](#installation--configuration)
3. [Load Balancing](#load-balancing)
4. [Security Standards](#security-standards)
5. [Performance Optimization](#performance-optimization)
6. [Monitoring & Observability](#monitoring--observability)
7. [Backup & Recovery](#backup--recovery)
8. [Production Patterns](#production-patterns)
9. [Common Anti-Patterns](#common-anti-patterns)

---

## Architecture Overview

### KF-001: Core Components

| Component | Purpose | Scalability |
|-----------|---------|-------------|
| **Broker** | Message storage & serving | Horizontal |
| **Topic** | Logical channel | Partitions |
| **Partition** | Ordered log segment | Replicas |
| **Producer** | Writes messages | Multiple |
| **Consumer** | Reads messages | Consumer groups |
| **ZooKeeper/KRaft** | Cluster coordination | Quorum-based |

### KF-002: KRaft Mode (Recommended для production с Kafka 3.3+)

**KRaft** (Kafka Raft) заменяет ZooKeeper для metadata management.

**Преимущества**:
- Упрощённая архитектура (нет внешней зависимости от ZooKeeper)
- Faster failover (controller election)
- Легче scaling (до миллионов партиций)

**Migration**: Kafka 3.6+ поддерживает seamless ZooKeeper → KRaft migration.

---

## Installation & Configuration

### KF-003: 1. Production Deployment Architecture

#### Минимальная HA Конфигурация

```
┌─────────────────────────────────────────┐
│         Load Balancer (Optional)        │
└───────────┬─────────────┬───────────────┘
            │             │
    ┌───────▼─────┐  ┌────▼──────┐  ┌──────────┐
    │  Broker 1   │  │ Broker 2  │  │ Broker 3 │
    │ Controller  │  │           │  │          │
    └─────────────┘  └───────────┘  └──────────┘
         │                │              │
         └────────────────┴──────────────┘
                   Cluster
```

**Минимум**: 3 brokers (для replication factor 3)

### KF-004: 2. Broker Configuration (server.properties)

#### Essential Settings

```properties
############################# Server Basics #############################
# Unique broker ID
broker.id=1

# KRaft mode (Kafka 3.3+)
process.roles=broker,controller
node.id=1
controller.quorum.voters=1@kafka1:9093,2@kafka2:9093,3@kafka3:9093

# OR ZooKeeper mode (legacy)
# zookeeper.connect=zoo1:2181,zoo2:2181,zoo3:2181/kafka

############################# Socket Server Settings #############################
# CRITICAL: Use separate listeners for internal and external
listeners=PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
advertised.listeners=PLAINTEXT://kafka1.example.com:9092

# Max connections per broker
max.connections=1000
max.connections.per.ip=100

############################# Log Basics #############################
# Data directory (MUST be on fast SSD)
log.dirs=/var/kafka/data

# Default partition count (CRITICAL for load balancing)
num.partitions=6

# Replication factor (CRITICAL for HA)
default.replication.factor=3
min.insync.replicas=2

# Segment size (1GB recommended)
log.segment.bytes=1073741824

############################# Log Retention #############################
# Retention by time (7 days)
log.retention.hours=168

# Retention by size (100GB per partition)
log.retention.bytes=107374182400

# Cleanup policy
log.cleanup.policy=delete

# Check interval for log segments
log.retention.check.interval.ms=300000

############################# Performance #############################
# Replica fetcher threads (increase for high throughput)
num.replica.fetchers=4

# Network threads
num.network.threads=8

# IO threads (should match disk count)
num.io.threads=8

# Socket buffer sizes
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600

############################# High Availability #############################
# Unclean leader election (DISABLE for data safety)
unclean.leader.election.enable=false

# Minimum in-sync replicas
min.insync.replicas=2

# Leader imbalance check
auto.leader.rebalance.enable=true
leader.imbalance.check.interval.seconds=300
```

#### Hardware Requirements

| Component | Minimum | Recommended | Critical |
|-----------|---------|-------------|----------|
| **CPU** | 8 cores | 16+ cores | More cores для high partition count |
| **RAM** | 32 GB | 64+ GB | OS page cache для performance |
| **Disk** | SSD 500GB | NVMe 1TB+ | **CRITICAL**: NO HDD! |
| **Network** | 1 Gbps | 10 Gbps | Low latency critical |

**CRITICAL**: Kafka relies heavily on **OS page cache** → More RAM = Better performance.

### KF-005: 3. JVM Settings

```bash
# kafka-server-start.sh environment
export KAFKA_HEAP_OPTS="-Xms6g -Xmx6g"
export KAFKA_JVM_PERFORMANCE_OPTS="-XX:+UseG1GC -XX:MaxGCPauseMillis=20 -XX:InitiatingHeapOccupancyPercent=35 -XX:G1HeapRegionSize=16M -XX:MinMetaspaceFreeRatio=50 -XX:MaxMetaspaceFreeRatio=80"
```

**Heap Size Guidelines**:
- **Small brokers** (< 100 partitions): 4-6 GB
- **Medium brokers** (100-1000 partitions): 6-8 GB
- **Large brokers** (> 1000 partitions): 8-12 GB
- **Max heap**: 16 GB (diminishing returns after)

**ВАЖНО**: Оставь достаточно RAM для OS page cache (минимум 50% RAM).

---

## Load Balancing

**CRITICAL**: Load balancing в Kafka реализуется через multiple mechanisms.

### KF-006: 1. Partitioning Strategy (Producer-Side Load Balancing)

Партиции — основной механизм распределения нагрузки в Kafka.

#### Partition Assignment Strategies

| Strategy | Key Required | Distribution | Use Case |
|----------|--------------|--------------|----------|
| **Round-robin** | No | Even | Balanced load, no ordering needed |
| **Key-hash** | Yes | Consistent per key | Ordered messages per key |
| **Custom** | Maybe | Configurable | Complex routing logic |

#### Producer Partitioner Configuration

```go
// Go example with Sarama library
config := sarama.NewConfig()
config.Producer.RequiredAcks = sarama.WaitForAll
config.Producer.Return.Successes = true

// Round-robin partitioning (default без key)
config.Producer.Partitioner = sarama.NewRoundRobinPartitioner

// OR Hash partitioning (с key)
config.Producer.Partitioner = sarama.NewHashPartitioner

// Custom partitioner
config.Producer.Partitioner = sarama.NewCustomPartitioner(func(topic string, numPartitions int32) sarama.Partitioner {
    return &MyCustomPartitioner{}
})
```

#### Partition Count Calculation

**Formula**: `partitions = max(expected_throughput / partition_throughput, num_consumers)`

**Example**:
- Expected throughput: 600 MB/s
- Single partition throughput: 10 MB/s
- Consumers: 20

**Result**: `max(600/10, 20) = max(60, 20) = 60 partitions`

**Best Practices**:
- Start with **6-12 partitions** per topic (allows scaling)
- Use **multiples of broker count** (e.g., 3 brokers → 6, 9, 12 partitions)
- **Max partitions per broker**: ~4000 (performance degradation after)
- **Cannot decrease** partition count (only increase)

### KF-007: 2. Consumer Groups (Consumer-Side Load Balancing)

Consumer groups обеспечивают automatic load balancing между consumers.

#### How It Works

```
Topic: orders (6 partitions)
┌─────────────────────────────────────────┐
│ P0 │ P1 │ P2 │ P3 │ P4 │ P5 │
└──┬───┴──┬───┴──┬───┴──┬───┴──┬───┴──┬───┘
   │      │      │      │      │      │
   │      │      │      │      │      │
┌──▼──┐ ┌─▼──┐ ┌─▼──┐ ┌─▼──┐ ┌─▼──┐ ┌▼──┐
│ C1  │ │ C1 │ │ C2 │ │ C2 │ │ C3 │ │C3 │
└─────┘ └────┘ └────┘ └────┘ └────┘ └───┘
       Consumer Group: order-processors
```

**Rules**:
- Each partition → assigned to **ONE** consumer в группе
- One consumer → can consume **multiple** partitions
- More consumers than partitions → some consumers idle

#### Partition Assignment Strategies

```properties
# Consumer configuration
partition.assignment.strategy=org.apache.kafka.clients.consumer.RangeAssignor
# OR
partition.assignment.strategy=org.apache.kafka.clients.consumer.RoundRobinAssignor
# OR (Recommended)
partition.assignment.strategy=org.apache.kafka.clients.consumer.StickyAssignor
# OR (Kafka 2.4+, BEST for cooperative rebalancing)
partition.assignment.strategy=org.apache.kafka.clients.consumer.CooperativeStickyAssignor
```

| Strategy | Behavior | Use Case |
|----------|----------|----------|
| **Range** | Distributes partitions per topic | Default, simple |
| **RoundRobin** | Distributes all partitions evenly | Balanced load |
| **Sticky** | Minimizes partition movement | Reduces rebalancing overhead |
| **CooperativeStickyAssignor** | Incremental rebalancing | **RECOMMENDED** (zero downtime) |

#### Consumer Configuration

```go
config := sarama.NewConfig()
config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
config.Consumer.Offsets.Initial = sarama.OffsetNewest
config.Consumer.Return.Errors = true

// Heartbeat & session timeout
config.Consumer.Group.Session.Timeout = 10 * time.Second
config.Consumer.Group.Heartbeat.Interval = 3 * time.Second

// Max poll interval (important для long processing)
config.Consumer.MaxProcessingTime = 5 * time.Minute
```

**CRITICAL**: `max.poll.interval.ms` ДОЛЖЕН быть > maximum message processing time.

### KF-008: 3. Replication & Broker Distribution

Replication обеспечивает HA и load distribution для reads (если configured).

#### Replication Factor

```bash
# Create topic с replication factor 3
kafka-topics.sh --create \
  --topic orders \
  --partitions 6 \
  --replication-factor 3 \
  --config min.insync.replicas=2 \
  --bootstrap-server kafka1:9092
```

**Best Practices**:
- **Production**: `replication.factor = 3` (MINIMUM)
- **min.insync.replicas = 2** (acks=all safety)
- **Leader distribution**: Auto-balanced across brokers

#### Replica Distribution Example

```
Topic: orders (6 partitions, RF=3, 3 brokers)

Partition 0: Leader=Broker1, Replicas=[Broker1, Broker2, Broker3]
Partition 1: Leader=Broker2, Replicas=[Broker2, Broker3, Broker1]
Partition 2: Leader=Broker3, Replicas=[Broker3, Broker1, Broker2]
Partition 3: Leader=Broker1, Replicas=[Broker1, Broker2, Broker3]
Partition 4: Leader=Broker2, Replicas=[Broker2, Broker3, Broker1]
Partition 5: Leader=Broker3, Replicas=[Broker3, Broker1, Broker2]
```

**Leader Balancing**:
- Automatic leader election при broker failure
- `auto.leader.rebalance.enable=true` для periodic rebalancing

#### Follower Fetching (Kafka 2.4+)

**Feature**: Consumers могут читать из **closest replica** (не только leader).

```properties
# Consumer configuration
replica.selector.class=org.apache.kafka.common.replica.RackAwareReplicaSelector

# Broker configuration
broker.rack=us-east-1a
```

**Benefits**:
- Reduced cross-AZ traffic costs
- Lower latency для distributed consumers

### KF-009: 4. Producer Load Balancing

#### Sticky Partitioning (Kafka 2.4+)

**Default behavior**: Producer batches messages для same partition → better compression & throughput.

```go
config := sarama.NewConfig()
config.Producer.Compression = sarama.CompressionSnappy

// Batching configuration
config.Producer.Flush.Messages = 100      // Batch size
config.Producer.Flush.Frequency = 100 * time.Millisecond  // Max wait
config.Producer.Flush.MaxMessages = 1000  // Max batch

// Sticky partitioning enabled по умолчанию
```

#### Acknowledgement Levels

```go
// acks=0: No wait (fire-and-forget, fastest)
config.Producer.RequiredAcks = sarama.NoResponse

// acks=1: Wait for leader only
config.Producer.RequiredAcks = sarama.WaitForLocal

// acks=all: Wait for min.insync.replicas (RECOMMENDED)
config.Producer.RequiredAcks = sarama.WaitForAll
```

**Production**: ALWAYS use `acks=all` + `min.insync.replicas=2`.

### KF-010: 5. Broker-Level Load Distribution

#### Partition Leader Rebalancing

```properties
# Broker configuration
auto.leader.rebalance.enable=true
leader.imbalance.per.broker.percentage=10
leader.imbalance.check.interval.seconds=300
```

**What it does**: Redistributes partition leaders across brokers для balanced load.

#### Manual Rebalancing

```bash
# Generate reassignment plan
kafka-reassign-partitions.sh \
  --bootstrap-server kafka1:9092 \
  --topics-to-move-json-file topics.json \
  --broker-list "1,2,3" \
  --generate

# Execute reassignment
kafka-reassign-partitions.sh \
  --bootstrap-server kafka1:9092 \
  --reassignment-json-file reassignment.json \
  --execute

# Verify
kafka-reassign-partitions.sh \
  --bootstrap-server kafka1:9092 \
  --reassignment-json-file reassignment.json \
  --verify
```

### KF-011: 6. Rack Awareness

**Feature**: Distribute replicas across availability zones.

```properties
# Broker configuration
broker.rack=us-east-1a  # Broker 1
broker.rack=us-east-1b  # Broker 2
broker.rack=us-east-1c  # Broker 3

# Topic creation
replica.selector.class=org.apache.kafka.common.replica.RackAwareReplicaSelector
```

**Benefits**:
- Survives AZ failure
- Reduced cross-AZ costs (с follower fetching)

---

## Security Standards

### KF-012: 1. Encryption (TLS/SSL)

#### Broker Configuration

```properties
############################# SSL Configuration #############################
listeners=PLAINTEXT://0.0.0.0:9092,SSL://0.0.0.0:9093
advertised.listeners=PLAINTEXT://kafka1:9092,SSL://kafka1:9093

# SSL settings
ssl.keystore.location=/var/private/ssl/kafka.server.keystore.jks
ssl.keystore.password=${SSL_KEYSTORE_PASSWORD}
ssl.key.password=${SSL_KEY_PASSWORD}
ssl.truststore.location=/var/private/ssl/kafka.server.truststore.jks
ssl.truststore.password=${SSL_TRUSTSTORE_PASSWORD}

# Client authentication (optional)
ssl.client.auth=required

# Protocols
ssl.enabled.protocols=TLSv1.3,TLSv1.2
ssl.protocol=TLSv1.3
```

#### Producer/Consumer Configuration

```go
config := sarama.NewConfig()
config.Net.TLS.Enable = true
config.Net.TLS.Config = &tls.Config{
    RootCAs:            caCertPool,
    Certificates:       []tls.Certificate{cert},
    InsecureSkipVerify: false,  // NEVER true in production
}
```

**Best Practice**: Use TLS 1.3 для production.

### KF-013: 2. Authentication (SASL)

#### SASL/PLAIN (Basic)

```properties
# Broker
listeners=SASL_SSL://0.0.0.0:9093
sasl.enabled.mechanisms=PLAIN
sasl.mechanism.inter.broker.protocol=PLAIN
listener.name.sasl_ssl.plain.sasl.jaas.config=org.apache.kafka.common.security.plain.PlainLoginModule required \
   username="admin" \
   password="admin-secret" \
   user_admin="admin-secret" \
   user_producer="producer-secret";
```

#### SASL/SCRAM (Recommended)

```bash
# Create SCRAM credentials
kafka-configs.sh --bootstrap-server kafka1:9092 \
  --alter --add-config 'SCRAM-SHA-512=[password=admin-secret]' \
  --entity-type users --entity-name admin
```

```properties
# Broker
sasl.enabled.mechanisms=SCRAM-SHA-512
sasl.mechanism.inter.broker.protocol=SCRAM-SHA-512
listener.name.sasl_ssl.scram-sha-512.sasl.jaas.config=org.apache.kafka.common.security.scram.ScramLoginModule required \
   username="admin" \
   password="admin-secret";
```

#### SASL/GSSAPI (Kerberos) - Enterprise

```properties
sasl.enabled.mechanisms=GSSAPI
sasl.kerberos.service.name=kafka
sasl.jaas.config=com.sun.security.auth.module.Krb5LoginModule required \
   useKeyTab=true \
   storeKey=true \
   keyTab="/etc/security/keytabs/kafka.keytab" \
   principal="kafka/kafka1.example.com@EXAMPLE.COM";
```

### KF-014: 3. Authorization (ACLs)

```properties
# Broker configuration
authorizer.class.name=kafka.security.authorizer.AclAuthorizer
super.users=User:admin
```

#### ACL Management

```bash
# Grant READ permission on topic
kafka-acls.sh --bootstrap-server kafka1:9092 \
  --add --allow-principal User:consumer1 \
  --operation Read --topic orders

# Grant WRITE permission
kafka-acls.sh --bootstrap-server kafka1:9092 \
  --add --allow-principal User:producer1 \
  --operation Write --topic orders

# Grant consumer group access
kafka-acls.sh --bootstrap-server kafka1:9092 \
  --add --allow-principal User:consumer1 \
  --operation Read --group order-processors

# List ACLs
kafka-acls.sh --bootstrap-server kafka1:9092 --list
```

**Best Practices**:
- Principle of least privilege
- Separate users для producers/consumers
- Audit ACLs regularly

### KF-015: 4. Network Segmentation

**Best Practice**: Isolate Kafka cluster в private network.

```
┌─────────────────────────────────────────┐
│          Public Internet                │
└─────────────┬───────────────────────────┘
              │
     ┌────────▼─────────┐
     │  Load Balancer   │
     │  (TLS offload)   │
     └────────┬─────────┘
              │
     ┌────────▼─────────┐
     │  Application     │
     │  (Private VPC)   │
     └────────┬─────────┘
              │
     ┌────────▼─────────┐
     │  Kafka Cluster   │
     │  (Private VPC)   │
     │  No internet     │
     └──────────────────┘
```

---

## Performance Optimization

### KF-016: 1. Producer Optimization

```go
config := sarama.NewConfig()

// Batching (CRITICAL для throughput)
config.Producer.Flush.Messages = 1000
config.Producer.Flush.Frequency = 100 * time.Millisecond
config.Producer.Flush.MaxMessages = 10000

// Compression (reduces network traffic)
config.Producer.Compression = sarama.CompressionLZ4  // Fastest
// OR
config.Producer.Compression = sarama.CompressionZSTD  // Best ratio (Kafka 2.1+)

// Async sending
config.Producer.Return.Successes = false  // Fire-and-forget mode
config.Producer.Return.Errors = true      // Still handle errors

// Buffer size
config.ChannelBufferSize = 256

// Idempotence (exactly-once semantics)
config.Producer.Idempotent = true
config.Producer.RequiredAcks = sarama.WaitForAll
config.Producer.Retry.Max = 5
```

**Throughput Formula**: `throughput = batch_size / flush_frequency`

### KF-017: 2. Consumer Optimization

```go
config := sarama.NewConfig()

// Fetch size
config.Consumer.Fetch.Min = 1024 * 1024      // 1 MB
config.Consumer.Fetch.Default = 10 * 1024 * 1024  // 10 MB
config.Consumer.Fetch.Max = 50 * 1024 * 1024      // 50 MB

// Max wait time
config.Consumer.MaxWaitTime = 500 * time.Millisecond

// Isolation level (для transactional reads)
config.Consumer.IsolationLevel = sarama.ReadCommitted

// Parallel processing
config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky

// Auto-commit (disable для manual offset management)
config.Consumer.Offsets.AutoCommit.Enable = false
```

**Manual Offset Commit** (для at-least-once):

```go
// Process message
if err := processMessage(msg); err != nil {
    log.Error(err)
    return
}

// Commit offset after successful processing
session.MarkMessage(msg, "")
session.Commit()
```

### KF-018: 3. Broker Tuning

#### OS-Level Optimization

```bash
# Increase file descriptors
ulimit -n 100000

# TCP settings
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=134217728
sysctl -w net.ipv4.tcp_rmem="4096 87380 134217728"
sysctl -w net.ipv4.tcp_wmem="4096 65536 134217728"

# Swappiness (disable swap для Kafka)
sysctl -w vm.swappiness=1

# Dirty page writeback
sysctl -w vm.dirty_ratio=80
sysctl -w vm.dirty_background_ratio=5
```

#### Disk Configuration

**CRITICAL**: Use **XFS** filesystem (better performance чем ext4).

```bash
# Mount with noatime (reduces disk writes)
mount -o noatime,nodiratime /dev/nvme0n1 /var/kafka/data
```

### KF-019: 4. Topic Configuration

```bash
kafka-configs.sh --bootstrap-server kafka1:9092 \
  --entity-type topics --entity-name high-throughput-topic \
  --alter --add-config \
    compression.type=lz4,\
    max.message.bytes=10485760,\
    retention.ms=604800000,\
    segment.ms=3600000,\
    min.insync.replicas=2
```

---

## Monitoring & Observability

### KF-020: 1. Key Metrics (ОБЯЗАТЕЛЬНО мониторить)

#### Broker Metrics

| Metric | Type | Alert Threshold | Meaning |
|--------|------|-----------------|---------|
| `UnderReplicatedPartitions` | Gauge | > 0 | Replication lag |
| `OfflinePartitionsCount` | Gauge | > 0 | **CRITICAL** data unavailability |
| `ActiveControllerCount` | Gauge | != 1 | Split-brain condition |
| `RequestHandlerAvgIdlePercent` | Gauge | < 20% | Broker overloaded |
| `NetworkProcessorAvgIdlePercent` | Gauge | < 30% | Network bottleneck |
| `BytesInPerSec` | Rate | - | Incoming throughput |
| `BytesOutPerSec` | Rate | - | Outgoing throughput |

#### Producer Metrics

| Metric | Type | Alert | Meaning |
|--------|------|-------|---------|
| `record-send-rate` | Rate | - | Messages/sec |
| `record-error-rate` | Rate | > 0 | Send failures |
| `request-latency-avg` | Gauge | > 100ms | Network latency |
| `buffer-available-bytes` | Gauge | Low | Backpressure |

#### Consumer Metrics

| Metric | Type | Alert | Meaning |
|--------|------|-------|---------|
| `records-consumed-rate` | Rate | - | Messages/sec |
| `records-lag-max` | Gauge | > 10000 | Consumer lag |
| `commit-latency-avg` | Gauge | > 100ms | Offset commit slow |
| `fetch-latency-avg` | Gauge | > 50ms | Fetch slow |

### KF-021: 2. Prometheus Integration

#### JMX Exporter Configuration

```yaml
# jmx_exporter_config.yml
rules:
- pattern: kafka.server<type=(.+), name=(.+), clientId=(.+), topic=(.+), partition=(.*)><>Value
  name: kafka_server_$1_$2
  labels:
    clientId: "$3"
    topic: "$4"
    partition: "$5"

- pattern: kafka.server<type=(.+), name=(.+)><>Value
  name: kafka_server_$1_$2
```

Start Kafka с JMX exporter:

```bash
export KAFKA_OPTS="-javaagent:/opt/jmx_exporter/jmx_prometheus_javaagent.jar=7071:/etc/kafka/jmx_exporter_config.yml"
kafka-server-start.sh /etc/kafka/server.properties
```

#### Prometheus Configuration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'kafka-broker'
    static_configs:
      - targets:
        - kafka1:7071
        - kafka2:7071
        - kafka3:7071
```

### KF-022: 3. Grafana Dashboards

**Recommended Dashboards**:
- [Kafka Overview](https://grafana.com/grafana/dashboards/7589)
- [Kafka Exporter](https://grafana.com/grafana/dashboards/7589)

### KF-023: 4. Consumer Lag Monitoring (CRITICAL)

```bash
# Check consumer lag
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --group order-processors \
  --describe

# Output:
# GROUP           TOPIC    PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG
# order-processors orders  0          1000000         1000100         100
```

**Alert Rules**:

```yaml
# Prometheus alert
- alert: KafkaConsumerLag
  expr: kafka_consumergroup_lag > 10000
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Consumer lag too high"
```

---

## Backup & Recovery

### KF-024: 1. Replication as Primary Backup

**Best Practice**: Replication factor 3 обеспечивает основную защиту.

**NOT a backup replacement**: Replication защищает от hardware failure, НЕ от:
- Logical corruption (bad data)
- Accidental topic deletion
- Cluster-wide disasters

### KF-025: 2. MirrorMaker 2 (Disaster Recovery)

**Use Case**: Cross-datacenter replication.

```properties
# mm2.properties
clusters = primary, backup
primary.bootstrap.servers = kafka1-dc1:9092
backup.bootstrap.servers = kafka1-dc2:9092

# Replication flows
primary->backup.enabled = true
primary->backup.topics = orders, payments

# Consumer group offset sync
sync.group.offsets.enabled = true
```

Start MirrorMaker 2:

```bash
connect-mirror-maker.sh mm2.properties
```

### KF-026: 3. Topic Snapshots (Manual Backup)

#### Export to JSON

```bash
kafka-console-consumer.sh \
  --bootstrap-server kafka1:9092 \
  --topic orders \
  --from-beginning \
  --max-messages 1000000 > orders_backup.json
```

**ВАЖНО**: НЕ масштабируется для large topics (use MirrorMaker).

### KF-027: 4. Configuration Backup

```bash
# Backup topic configurations
kafka-configs.sh --bootstrap-server kafka1:9092 \
  --describe --entity-type topics \
  > topic_configs_backup.txt

# Backup ACLs
kafka-acls.sh --bootstrap-server kafka1:9092 \
  --list > acls_backup.txt
```

### KF-028: 5. Recovery Procedures

#### Lost Broker Recovery

```bash
# 1. Stop failed broker
systemctl stop kafka

# 2. Replace hardware (if needed)

# 3. Clear data directory (if corrupted)
rm -rf /var/kafka/data/*

# 4. Start broker (will rejoin cluster & replicate)
systemctl start kafka

# 5. Verify replication
kafka-topics.sh --bootstrap-server kafka1:9092 \
  --describe --under-replicated-partitions
```

#### Topic Recovery from Backup

```bash
# Restore from JSON backup
cat orders_backup.json | kafka-console-producer.sh \
  --bootstrap-server kafka1:9092 \
  --topic orders-restored
```

### KF-029: 6. Offset Management

#### Export Consumer Offsets

```bash
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --group order-processors \
  --describe > offsets_backup.txt
```

#### Reset Offsets

```bash
# Reset to earliest
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --group order-processors \
  --topic orders \
  --reset-offsets --to-earliest --execute

# Reset to specific timestamp
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --group order-processors \
  --topic orders \
  --reset-offsets --to-datetime 2025-12-30T00:00:00.000 --execute
```

---

## Production Patterns

### KF-030: 1. Topic Naming Convention

```
<team>.<domain>.<dataset>.<version>

Examples:
- payments.transactions.orders.v1
- analytics.events.user-clicks.v2
```

### KF-031: 2. Message Schema Evolution

**Use Avro/Protobuf с Schema Registry**:

```go
import (
    "github.com/riferrei/srclient"
)

schemaRegistryClient := srclient.CreateSchemaRegistryClient("http://schema-registry:8081")

// Register schema
schema, err := schemaRegistryClient.CreateSchema("orders-value", avroSchema, srclient.Avro)

// Serialize with schema ID
serializedData, err := avroSerializer.Serialize(schema.ID(), data)
```

**Schema Compatibility**:
- **BACKWARD**: New consumers can read old data (default)
- **FORWARD**: Old consumers can read new data
- **FULL**: Both backward & forward

### KF-032: 3. Idempotent Producers

```go
config := sarama.NewConfig()
config.Producer.Idempotent = true
config.Producer.RequiredAcks = sarama.WaitForAll
config.Producer.Retry.Max = 5
config.Net.MaxOpenRequests = 1  // Required для idempotence
```

**Prevents**: Duplicate messages при retries.

### KF-033: 4. Transactional Messaging

```go
config := sarama.NewConfig()
config.Producer.Idempotent = true
config.Producer.Transaction.ID = "my-transactional-producer"

producer, err := sarama.NewAsyncProducer(brokers, config)

// Begin transaction
err = producer.BeginTxn()

// Send messages
producer.Input() <- &sarama.ProducerMessage{Topic: "orders", Value: sarama.StringEncoder("msg1")}
producer.Input() <- &sarama.ProducerMessage{Topic: "payments", Value: sarama.StringEncoder("msg2")}

// Commit transaction
err = producer.CommitTxn()
```

**Use Cases**: Exactly-once semantics, atomic multi-topic writes.

### KF-034: 5. Dead Letter Queue Pattern

```go
func processMessage(msg *sarama.ConsumerMessage, dlqProducer sarama.SyncProducer) {
    err := businessLogic(msg)
    if err != nil {
        log.Error("Processing failed, sending to DLQ", err)

        // Send to DLQ
        dlqMsg := &sarama.ProducerMessage{
            Topic: "orders-dlq",
            Value: sarama.ByteEncoder(msg.Value),
            Headers: []sarama.RecordHeader{
                {Key: []byte("app_error"), Value: []byte(err.Error())},
                {Key: []byte("original-topic"), Value: []byte(msg.Topic)},
            },
        }
        dlqProducer.SendMessage(dlqMsg)
    }
}
```

---

## Common Anti-Patterns

| Anti-Pattern | Problem | Solution |
|--------------|---------|----------|
| Single partition | No parallelism | Use multiple partitions |
| Too many partitions | Cluster overhead | Calculate optimal count |
| Low replication factor | Data loss risk | Use RF=3 minimum |
| `acks=0` in production | Data loss | Use `acks=all` |
| No consumer lag monitoring | Silent failures | Monitor lag continuously |
| Large messages (> 1MB) | Performance degradation | Use external storage + reference |
| Auto-create topics | Uncontrolled growth | Disable `auto.create.topics.enable` |
| Running as root | Security risk | Use dedicated `kafka` user |
| Single broker | No HA | Minimum 3 brokers |
| No backup strategy | Disaster recovery impossible | MirrorMaker 2 + config backups |
| HDD storage | Terrible performance | **MUST** use SSD/NVMe |
| Ignoring OS tuning | Suboptimal performance | Tune file descriptors, TCP, swap |

---

## Quick Reference

### KF-035: Topic Creation

```bash
kafka-topics.sh --create \
  --bootstrap-server kafka1:9092 \
  --topic orders \
  --partitions 12 \
  --replication-factor 3 \
  --config min.insync.replicas=2 \
  --config retention.ms=604800000 \
  --config compression.type=lz4
```

### KF-036: Performance Testing

```bash
# Producer performance test
kafka-producer-perf-test.sh \
  --topic test \
  --num-records 1000000 \
  --record-size 1024 \
  --throughput -1 \
  --producer-props \
    bootstrap.servers=kafka1:9092 \
    acks=all \
    compression.type=lz4

# Consumer performance test
kafka-consumer-perf-test.sh \
  --bootstrap-server kafka1:9092 \
  --topic test \
  --messages 1000000
```

### KF-037: Cluster Health Check

```bash
# List under-replicated partitions (CRITICAL)
kafka-topics.sh --bootstrap-server kafka1:9092 \
  --describe --under-replicated-partitions

# Check broker logs
tail -f /var/log/kafka/server.log | grep ERROR
```

---

## Version Compatibility (2025)

| Version | KRaft Production Ready | Notable Features | Status |
|---------|------------------------|------------------|--------|
| **3.9** | Yes | Enhanced KRaft | Current (2025) |
| **3.8** | Yes | Performance improvements | Maintained |
| **3.7** | Yes | KRaft metadata improvements | Maintained |
| **3.6** | Yes | ZK → KRaft migration | Maintained |
| **3.3** | Yes | KRaft early production | EOL soon |

**Migration Path**: ZooKeeper → KRaft (Kafka 3.6+).

---

## Дополнительные ресурсы

### KF-038: Official Documentation
- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [Kafka Improvement Proposals (KIPs)](https://cwiki.apache.org/confluence/display/KAFKA/Kafka+Improvement+Proposals)
- [Confluent Kafka Guide](https://docs.confluent.io/platform/current/kafka/introduction.html)

### KF-039: Tools
- **Kafka Manager/AKHQ**: Web UI для cluster management
- **Burrow**: Consumer lag monitoring (LinkedIn)
- **Cruise Control**: Automated cluster balancing (LinkedIn)

### KF-040: Research Report
См. полный отчет: `.memory_bank/agent_task_results/2025-12-29_web-research_kafka-devops-best-practices.md`

---

**При возникновении вопросов**: Обратись к DevOps engineer agent или см. research report.
