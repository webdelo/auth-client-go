# ClickHouse Distributed Tables & Replication

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| CHD-002 | Configure ClickHouse Keeper for cluster | [ClickHouse Keeper](#clickhouse-keeper-vs-zookeeper) |
| CHD-001 | Use separate disks for data/logs | [Storage Configuration](#storage-configuration) |

---

## Overview

ClickHouse поддерживает два механизма масштабирования:

| Механизм | Назначение | Координатор |
|----------|------------|-------------|
| **Replication** | Отказоустойчивость | ClickHouse Keeper / ZooKeeper |
| **Sharding** | Горизонтальное масштабирование | Distributed engine |

---

## ClickHouse Keeper vs ZooKeeper

### CH-600: ClickHouse Keeper (рекомендуется)

Встроенный координатор, совместимый с ZooKeeper протоколом.

```xml
<!-- config.xml -->
<keeper_server>
    <tcp_port>9181</tcp_port>
    <server_id>1</server_id>
    <log_storage_path>/var/lib/clickhouse/coordination/log</log_storage_path>
    <snapshot_storage_path>/var/lib/clickhouse/coordination/snapshots</snapshot_storage_path>

    <coordination_settings>
        <operation_timeout_ms>10000</operation_timeout_ms>
        <session_timeout_ms>30000</session_timeout_ms>
    </coordination_settings>

    <raft_configuration>
        <server>
            <id>1</id>
            <hostname>keeper1.example.com</hostname>
            <port>9234</port>
        </server>
        <server>
            <id>2</id>
            <hostname>keeper2.example.com</hostname>
            <port>9234</port>
        </server>
        <server>
            <id>3</id>
            <hostname>keeper3.example.com</hostname>
            <port>9234</port>
        </server>
    </raft_configuration>
</keeper_server>
```

### CH-601: Клиентские настройки

```xml
<!-- config.xml на ClickHouse серверах -->
<zookeeper>
    <node>
        <host>keeper1.example.com</host>
        <port>9181</port>
    </node>
    <node>
        <host>keeper2.example.com</host>
        <port>9181</port>
    </node>
    <node>
        <host>keeper3.example.com</host>
        <port>9181</port>
    </node>
</zookeeper>
```

---

## Replicated Tables

### CH-602: ReplicatedMergeTree

```sql
CREATE TABLE events ON CLUSTER cluster_name (
    event_id UUID,
    user_id UInt64,
    event_type String,
    created_at DateTime
) ENGINE = ReplicatedMergeTree(
    '/clickhouse/tables/{shard}/events',  -- ZK path
    '{replica}'                            -- Replica name
)
PARTITION BY toYYYYMM(created_at)
ORDER BY (user_id, created_at);
```

### CH-603: Макросы {shard} и {replica}

```xml
<!-- config.xml -->
<macros>
    <shard>01</shard>
    <replica>replica1</replica>
    <cluster>production</cluster>
</macros>
```

### CH-604: Все Replicated* движки

```sql
-- Базовый
ReplicatedMergeTree('/path', '{replica}')

-- С агрегацией
ReplicatedSummingMergeTree('/path', '{replica}')
ReplicatedAggregatingMergeTree('/path', '{replica}')

-- С дедупликацией
ReplicatedReplacingMergeTree('/path', '{replica}', version)
ReplicatedCollapsingMergeTree('/path', '{replica}', sign)
ReplicatedVersionedCollapsingMergeTree('/path', '{replica}', sign, version)
```

### CH-605: Проверка репликации

```sql
-- Статус репликации
SELECT
    database,
    table,
    is_leader,
    total_replicas,
    active_replicas,
    queue_size,
    inserts_in_queue,
    merges_in_queue
FROM system.replicas;

-- Лаг репликации
SELECT
    database,
    table,
    replica_name,
    absolute_delay
FROM system.replicas
WHERE absolute_delay > 0;
```

---

## Cluster Configuration

### CH-606: Простой кластер (1 shard, 2 replicas)

```xml
<remote_servers>
    <cluster_name>
        <shard>
            <replica>
                <host>node1.example.com</host>
                <port>9000</port>
            </replica>
            <replica>
                <host>node2.example.com</host>
                <port>9000</port>
            </replica>
        </shard>
    </cluster_name>
</remote_servers>
```

### CH-607: Sharded кластер (3 shards, 2 replicas each)

```xml
<remote_servers>
    <production_cluster>
        <shard>
            <weight>1</weight>
            <internal_replication>true</internal_replication>
            <replica>
                <host>shard1-replica1.example.com</host>
                <port>9000</port>
            </replica>
            <replica>
                <host>shard1-replica2.example.com</host>
                <port>9000</port>
            </replica>
        </shard>
        <shard>
            <weight>1</weight>
            <internal_replication>true</internal_replication>
            <replica>
                <host>shard2-replica1.example.com</host>
                <port>9000</port>
            </replica>
            <replica>
                <host>shard2-replica2.example.com</host>
                <port>9000</port>
            </replica>
        </shard>
        <shard>
            <weight>1</weight>
            <internal_replication>true</internal_replication>
            <replica>
                <host>shard3-replica1.example.com</host>
                <port>9000</port>
            </replica>
            <replica>
                <host>shard3-replica2.example.com</host>
                <port>9000</port>
            </replica>
        </shard>
    </production_cluster>
</remote_servers>
```

### CH-608: Просмотр кластеров

```sql
SELECT * FROM system.clusters;
```

---

## Distributed Engine

### CH-609: Создание Distributed таблицы

```sql
-- 1. Создаём локальную таблицу на каждом shard
CREATE TABLE events_local ON CLUSTER production_cluster (
    event_id UUID,
    user_id UInt64,
    event_type String,
    created_at DateTime
) ENGINE = ReplicatedMergeTree(
    '/clickhouse/tables/{shard}/events',
    '{replica}'
)
ORDER BY (user_id, created_at);

-- 2. Создаём Distributed таблицу
CREATE TABLE events ON CLUSTER production_cluster AS events_local
ENGINE = Distributed(
    production_cluster,    -- Cluster name
    default,               -- Database
    events_local,          -- Local table
    user_id                -- Sharding key
);
```

### CH-610: Sharding Keys

```sql
-- По user_id (равномерное распределение)
ENGINE = Distributed(cluster, db, table, user_id)

-- По hash
ENGINE = Distributed(cluster, db, table, sipHash64(user_id))

-- Random (для insert балансировки)
ENGINE = Distributed(cluster, db, table, rand())

-- По дате (временное партиционирование)
ENGINE = Distributed(cluster, db, table, toYYYYMM(created_at))
```

### CH-611: Запись в Distributed

```sql
-- Синхронная запись (медленнее, но надёжнее)
SET insert_distributed_sync = 1;
INSERT INTO events VALUES (...);

-- Асинхронная запись (быстрее, буферизация)
SET insert_distributed_sync = 0;
INSERT INTO events VALUES (...);
```

### CH-612: Чтение из Distributed

```sql
-- Запрос ко всем shards
SELECT count() FROM events WHERE user_id = 123;

-- С GLOBAL для подзапросов
SELECT * FROM events
WHERE user_id GLOBAL IN (
    SELECT user_id FROM vip_users
);
```

---

## Replicated Databases (ClickHouse 20.1+)

### CH-613: Replicated Database Engine

```sql
CREATE DATABASE analytics ON CLUSTER production_cluster
ENGINE = Replicated(
    '/clickhouse/databases/analytics',
    '{shard}',
    '{replica}'
);
```

### CH-614: Преимущества

- DDL автоматически реплицируется
- Не нужно указывать ON CLUSTER для каждой таблицы
- Автоматические ZK пути

```sql
-- Создаём таблицу в Replicated базе
USE analytics;

-- ReplicatedMergeTree создаётся автоматически
CREATE TABLE events (
    id UUID,
    data String
) ENGINE = MergeTree()
ORDER BY id;

-- Эквивалентно:
-- ENGINE = ReplicatedMergeTree('/clickhouse/databases/analytics/events/{shard}', '{replica}')
```

---

## Архитектурные паттерны

### CH-615: Паттерн 1: Simple HA (2 replicas)

```
┌─────────────┐     ┌─────────────┐
│  Replica 1  │◄───►│  Replica 2  │
│  (leader)   │     │  (follower) │
└─────────────┘     └─────────────┘
       ▲                   ▲
       │     Load Balancer │
       └─────────┬─────────┘
                 │
            Application
```

**Когда использовать:**
- Отказоустойчивость без шардирования
- Данные помещаются на один сервер
- Read-heavy нагрузка

### CH-616: Паттерн 2: Sharded (3 shards, no replication)

```
┌─────────┐  ┌─────────┐  ┌─────────┐
│ Shard 1 │  │ Shard 2 │  │ Shard 3 │
│  33%    │  │  33%    │  │  33%    │
└─────────┘  └─────────┘  └─────────┘
      │           │            │
      └───────────┼────────────┘
                  │
           Distributed Table
```

**Когда использовать:**
- Данные не помещаются на один сервер
- Допустима потеря данных при сбое
- Dev/staging окружения

### CH-617: Паттерн 3: Sharded + Replicated (Production)

```
       Shard 1              Shard 2              Shard 3
┌───────────────────┐ ┌───────────────────┐ ┌───────────────────┐
│ ┌───────┐ ┌───────┐│ │ ┌───────┐ ┌───────┐│ │ ┌───────┐ ┌───────┐│
│ │ R1-1  │ │ R1-2  ││ │ │ R2-1  │ │ R2-2  ││ │ │ R3-1  │ │ R3-2  ││
│ │Primary│ │Standby││ │ │Primary│ │Standby││ │ │Primary│ │Standby││
│ └───────┘ └───────┘│ └───────┘ └───────┘│ └───────┘ └───────┘│
└───────────────────┘ └───────────────────┘ └───────────────────┘
            │                   │                   │
            └───────────────────┼───────────────────┘
                                │
                       Distributed Table
                                │
                          Application
```

**Когда использовать:**
- Production высокой доступности
- Большие объёмы данных
- Требуется масштабирование

---

## Операции с Distributed таблицами

### CH-618: ON CLUSTER DDL

```sql
-- Создание на всех узлах кластера
CREATE TABLE events ON CLUSTER production_cluster (...)
ENGINE = ReplicatedMergeTree(...);

-- Удаление на всех узлах
DROP TABLE events ON CLUSTER production_cluster;

-- Изменение на всех узлах
ALTER TABLE events ON CLUSTER production_cluster
ADD COLUMN new_col String;
```

### CH-619: Ребалансировка данных

```sql
-- 1. Добавить новый shard в конфигурацию кластера
-- 2. Создать локальную таблицу на новом shard
-- 3. Обновить Distributed таблицу (если нужен другой sharding key)

-- Миграция данных между shards вручную:
INSERT INTO events_local
SELECT * FROM events
WHERE sipHash64(user_id) % 4 = 3;  -- Данные для нового shard
```

### CH-620: Мониторинг Distributed запросов

```sql
-- Активные distributed запросы
SELECT * FROM system.distributed_ddl_queue;

-- Очередь записи в Distributed
SELECT
    database,
    table,
    data_files,
    formatReadableSize(data_compressed_bytes) as compressed,
    formatReadableSize(data_uncompressed_bytes) as uncompressed
FROM system.distribution_queue;
```

---

## Best Practices

### CH-621: 1. Sharding Key Selection

```sql
-- Хороший: равномерное распределение по пользователям
ENGINE = Distributed(cluster, db, table, sipHash64(user_id))

-- Плохой: неравномерное распределение
ENGINE = Distributed(cluster, db, table, country)  -- Если 80% из US

-- Хороший для time-series: по времени
ENGINE = Distributed(cluster, db, table, toYYYYMM(timestamp))
```

### CH-622: 2. Internal Replication

```xml
<!-- Всегда включайте для Replicated таблиц -->
<shard>
    <internal_replication>true</internal_replication>
    ...
</shard>
```

### CH-623: 3. Избегайте GLOBAL если возможно

```sql
-- Медленно: GLOBAL копирует данные на все shards
SELECT * FROM events WHERE user_id GLOBAL IN (SELECT user_id FROM temp);

-- Быстро: если фильтр по sharding key
SELECT * FROM events WHERE user_id IN (1, 2, 3);
```

### CH-624: 4. Distributed + local таблицы

```sql
-- Создавайте обе: local для прямого доступа, Distributed для прозрачного
CREATE TABLE events_local (...) ENGINE = ReplicatedMergeTree(...);
CREATE TABLE events AS events_local ENGINE = Distributed(...);

-- Для bulk insert напрямую в local (быстрее)
INSERT INTO events_local VALUES ...;

-- Для обычных операций через Distributed
SELECT * FROM events WHERE ...;
```

### CH-625: 5. Мониторинг здоровья

```sql
-- Проверка всех реплик
SELECT
    hostName() as host,
    database,
    table,
    is_leader,
    active_replicas,
    queue_size,
    absolute_delay
FROM clusterAllReplicas('production_cluster', system.replicas)
WHERE queue_size > 0 OR absolute_delay > 60;

-- Проверка Keeper
SELECT * FROM system.zookeeper WHERE path = '/clickhouse';
```
