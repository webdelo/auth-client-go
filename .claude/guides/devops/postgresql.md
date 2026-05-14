# PostgreSQL Production Best Practices and Standards

This document contains PostgreSQL deployment rules for DevOps tasks. Each rule has a unique identifier (PGD-XXXX) for reference in code reviews and task execution.

> **Related**: For PostgreSQL query patterns, see `dba/postgres/query_patterns.md`
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми PostgreSQL задачами
**Текущая версия PostgreSQL**: 17.x (2025)

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| PGD-001 | Use persistent volumes for data | ✅ | [Storage Configuration](#pgd-001-use-persistent-volumes-for-data) |
| PGD-002 | Set shared_buffers = 25% RAM | ✅ | [Memory Configuration](#pgd-002-set-shared_buffers--25-ram) |
| PGD-003 | Configure WAL archiving for backup | ✅ | [WAL Configuration](#pgd-003-configure-wal-archiving-for-backup) |
| PGD-004 | Use connection pooler (PgBouncer) | ✅ | [Connection Pooling](#pgd-004-use-connection-pooler-pgbouncer) |
| PGD-005 | Monitor with pg_stat_statements | | [Monitoring](#pgd-005-monitor-with-pg_stat_statements) |

---

## Содержание

1. [Installation & Setup](#installation--setup)
2. [Configuration Best Practices](#configuration-best-practices)
3. [Load Balancing](#load-balancing)
4. [Security Standards](#security-standards)
5. [Monitoring & Observability](#monitoring--observability)
6. [Backup & Recovery](#backup--recovery)
7. [High Availability](#high-availability)
8. [Performance Optimization](#performance-optimization)
9. [Production Patterns](#production-patterns)
10. [Common Gotchas](#common-gotchas)

---

## Installation & Setup

### 1. Production Installation (Ubuntu/Debian)

```bash
# Добавить официальный PostgreSQL APT repository
sudo apt install -y postgresql-common
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh

# Установить PostgreSQL 17
sudo apt update
sudo apt install -y postgresql-17 postgresql-contrib-17

# Установить дополнительные инструменты
sudo apt install -y \
  postgresql-client-17 \
  postgresql-17-pgaudit \
  postgresql-17-pg-stat-kcache \
  postgresql-17-pglogical
```

### 2. Docker Production Setup

**ОБЯЗАТЕЛЬНО**: Используй официальный образ с Alpine для минимального attack surface.

```dockerfile
FROM postgres:17-alpine

# Установить дополнительные расширения
RUN apk add --no-cache \
    postgresql-contrib \
    postgresql-pg_stat_statements

# Копировать конфигурации
COPY postgresql.conf /etc/postgresql/postgresql.conf
COPY pg_hba.conf /etc/postgresql/pg_hba.conf

# Копировать init scripts
COPY init-scripts/ /docker-entrypoint-initdb.d/

# Создать директории для healthcheck
RUN mkdir -p /var/run/postgresql && \
    chown -R postgres:postgres /var/run/postgresql

# Non-root user (уже postgres в образе)
USER postgres

EXPOSE 5432

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD pg_isready -U postgres || exit 1
```

### 3. Docker Compose Production

```yaml
services:
  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
      PGDATA: /var/lib/postgresql/data/pgdata
    secrets:
      - db_password
    volumes:
      # Data persistence
      - postgres_data:/var/lib/postgresql/data
      # Custom config
      - ./postgresql.conf:/etc/postgresql/postgresql.conf:ro
      - ./pg_hba.conf:/etc/postgresql/pg_hba.conf:ro
      # Init scripts
      - ./init-scripts:/docker-entrypoint-initdb.d:ro
    command: >
      postgres
      -c config_file=/etc/postgresql/postgresql.conf
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER}"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
    networks:
      - backend
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G

volumes:
  postgres_data:

secrets:
  db_password:
    file: ./secrets/db_password.txt

networks:
  backend:
    internal: true
```

---

## Configuration Best Practices

### PGD-001: Memory Configuration

**Правило**: Настройка памяти критична для производительности.

```ini
# postgresql.conf

# ------------------------
# Memory Settings
# ------------------------

# Shared buffers: 25% от RAM (для dedicated DB server)
# Для 8GB RAM → 2GB
shared_buffers = 2GB

# Effective cache: 50-75% от RAM
# OS + PostgreSQL shared buffers
effective_cache_size = 6GB

# Работает с shared_buffers
# maintenance_work_mem: 64MB - 2GB в зависимости от RAM
maintenance_work_mem = 512MB

# Per-connection sort/hash operations
# work_mem = RAM / max_connections / 2-4
# Для 8GB RAM, 100 connections
work_mem = 16MB

# Wal buffers: обычно 16MB достаточно
wal_buffers = 16MB
```

**Формулы расчёта**:
- `shared_buffers`: 25% RAM (max 8-16GB на modern systems)
- `effective_cache_size`: 50-75% RAM
- `maintenance_work_mem`: 5-10% RAM (max 2GB)
- `work_mem`: (RAM - shared_buffers) / (max_connections * 2)

### PGD-002: Connection Configuration

```ini
# ------------------------
# Connection Settings
# ------------------------

# Maximum connections
# ВАЖНО: Каждое соединение потребляет ~10MB RAM
max_connections = 100

# Для production: использовать connection pooler (PgBouncer)
# В таком случае можно снизить max_connections до 20-50

# Superuser reserved connections
superuser_reserved_connections = 3

# Statement timeout (предотвращает long-running queries)
statement_timeout = 30000  # 30 seconds

# Idle in transaction timeout
idle_in_transaction_session_timeout = 60000  # 60 seconds

# TCP keepalive
tcp_keepalives_idle = 60
tcp_keepalives_interval = 10
tcp_keepalives_count = 3
```

### PGD-003: WAL (Write-Ahead Logging) Configuration

```ini
# ------------------------
# WAL Settings
# ------------------------

# WAL level для replication
wal_level = replica

# Checkpoint settings
# Более редкие checkpoints = лучше performance, но дольше recovery
checkpoint_timeout = 15min
checkpoint_completion_target = 0.9

# WAL segment size (default 16MB)
max_wal_size = 4GB
min_wal_size = 1GB

# WAL archiving (для PITR backups)
archive_mode = on
archive_command = 'test ! -f /mnt/wal_archive/%f && cp %p /mnt/wal_archive/%f'
archive_timeout = 300  # 5 minutes

# WAL writer delay
wal_writer_delay = 200ms
```

### PGD-004: Query Planner Configuration

```ini
# ------------------------
# Query Planner
# ------------------------

# Random page cost (для SSD снизить)
# HDD: 4.0 (default), SSD: 1.1-1.3
random_page_cost = 1.1

# Effective IO concurrency (для SSD увеличить)
# HDD: 1-2, SSD: 200, NVMe: 400+
effective_io_concurrency = 200

# Default statistics target
default_statistics_target = 100

# Enable parallel query execution
max_parallel_workers_per_gather = 2
max_parallel_workers = 4
max_worker_processes = 4
```

### PGD-005: Logging Configuration

```ini
# ------------------------
# Logging
# ------------------------

# Куда логировать
log_destination = 'stderr'
logging_collector = on

# Log rotation
log_directory = 'log'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_rotation_age = 1d
log_rotation_size = 100MB

# Что логировать
log_min_duration_statement = 1000  # Log queries > 1s
log_checkpoints = on
log_connections = on
log_disconnections = on
log_lock_waits = on

# Detailed logging
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
log_statement = 'ddl'  # Log DDL statements

# Slow queries
log_min_duration_statement = 1000
```

### PGD-006: Auto-vacuum Configuration

```ini
# ------------------------
# Autovacuum
# ------------------------

autovacuum = on
autovacuum_max_workers = 3
autovacuum_naptime = 1min

# Vacuum thresholds
autovacuum_vacuum_threshold = 50
autovacuum_vacuum_scale_factor = 0.1

# Analyze thresholds
autovacuum_analyze_threshold = 50
autovacuum_analyze_scale_factor = 0.05

# Vacuum cost settings (не блокировать production)
autovacuum_vacuum_cost_delay = 20ms
autovacuum_vacuum_cost_limit = 200
```

---

## Load Balancing

### PGD-007: Connection Pooling (CRITICAL для Production)

**Проблема**: PostgreSQL использует process-per-connection модель. Каждое соединение потребляет ~10MB RAM.

**Решение**: Connection pooler снижает нагрузку на PostgreSQL, переиспользуя connections.

#### PgBouncer (Рекомендуется)

**Архитектура**:
```
Application → PgBouncer → PostgreSQL
  (1000 connections)   (20-50 connections)
```

**Установка**:
```bash
sudo apt install -y pgbouncer
```

**Конфигурация** (`/etc/pgbouncer/pgbouncer.ini`):
```ini
[databases]
myapp = host=localhost port=5432 dbname=myapp pool_size=20

[pgbouncer]
# Listen address
listen_addr = 0.0.0.0
listen_port = 6432

# Authentication
auth_type = scram-sha-256
auth_file = /etc/pgbouncer/userlist.txt

# Pool mode
# - session: Соединение держится всю сессию (safe, но меньше pooling)
# - transaction: Соединение возвращается после транзакции (РЕКОМЕНДУЕТСЯ)
# - statement: После каждого statement (DANGEROUS, breaks prepared statements)
pool_mode = transaction

# Connection limits
max_client_conn = 1000
default_pool_size = 20
min_pool_size = 5
reserve_pool_size = 5

# Timeouts
server_idle_timeout = 600
server_lifetime = 3600
client_idle_timeout = 0

# Log settings
log_connections = 1
log_disconnections = 1
log_pooler_errors = 1
```

**User Authentication** (`/etc/pgbouncer/userlist.txt`):
```
"myapp_user" "SCRAM-SHA-256$<hash>"
```

Generate hash:
```bash
echo -n "passwordSALT" | openssl dgst -sha256 -binary | xxd -p
```

**Docker PgBouncer**:
```yaml
services:
  pgbouncer:
    image: pgbouncer/pgbouncer:1.23.0
    restart: unless-stopped
    environment:
      DATABASES_HOST: postgres
      DATABASES_PORT: 5432
      DATABASES_DBNAME: myapp
      PGBOUNCER_POOL_MODE: transaction
      PGBOUNCER_MAX_CLIENT_CONN: 1000
      PGBOUNCER_DEFAULT_POOL_SIZE: 20
      PGBOUNCER_AUTH_TYPE: scram-sha-256
    ports:
      - "6432:6432"
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - backend

  postgres:
    image: postgres:17-alpine
    # ... (см. выше)
```

**Application Connection String**:
```
postgres://user:password@pgbouncer:6432/myapp
```

**Мониторинг PgBouncer**:
```sql
-- Подключиться к pgbouncer admin console
psql -p 6432 -U pgbouncer pgbouncer

-- Показать статистику пулов
SHOW POOLS;

-- Показать статистику баз данных
SHOW DATABASES;

-- Показать активные клиенты
SHOW CLIENTS;

-- Показать активные серверные соединения
SHOW SERVERS;
```

#### Connection Pooling: PgBouncer vs Application-Level

| Aspect | PgBouncer | Application Pool (e.g., HikariCP) |
|--------|-----------|-----------------------------------|
| Architecture | External proxy | In-application |
| Connection reuse | Across apps | Single app instance |
| Overhead | Network hop | None |
| Failover | Single point | Per instance |
| Resource usage | Minimal | Per app instance |
| Best for | Microservices, high connection count | Monolith |

**Рекомендация**: Используй **оба**:
- Application pool: 20-50 connections per app instance
- PgBouncer: Pool 20-50 connections к PostgreSQL

### PGD-008: Read Replicas (Horizontal Scaling)

**Архитектура**:
```
                  ┌─→ Read Replica 1 (read-only)
Load Balancer ────┼─→ Read Replica 2 (read-only)
                  └─→ Primary (read-write)
```

**Streaming Replication Setup**:

#### Primary Server (`postgresql.conf`):
```ini
# Replication settings
wal_level = replica
max_wal_senders = 10
max_replication_slots = 10
wal_keep_size = 1GB

# Standby feedback
hot_standby = on
hot_standby_feedback = on
```

**Create Replication User**:
```sql
CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'strong_password';
```

**Allow Replication** (`pg_hba.conf` на primary):
```
# TYPE  DATABASE        USER            ADDRESS                 METHOD
host    replication     replicator      10.0.0.0/8              scram-sha-256
```

#### Replica Server Setup:

**1. Создать base backup**:
```bash
# На replica server
pg_basebackup \
  -h primary.example.com \
  -D /var/lib/postgresql/17/main \
  -U replicator \
  -P \
  -R \
  --wal-method=stream
```

**2. Конфигурация replica** (`postgresql.auto.conf`):
```ini
primary_conninfo = 'host=primary.example.com port=5432 user=replicator password=strong_password'
primary_slot_name = 'replica_1_slot'
```

**3. Create Replication Slot** (на primary):
```sql
SELECT * FROM pg_create_physical_replication_slot('replica_1_slot');
```

**4. Start Replica**:
```bash
sudo systemctl start postgresql@17-main
```

**5. Verify Replication**:
```sql
-- На primary
SELECT * FROM pg_stat_replication;

-- На replica
SELECT pg_is_in_recovery();  -- Should return 't'
SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn();
```

#### Docker Compose: Primary + Replica

```yaml
services:
  postgres-primary:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
      POSTGRES_DB: myapp
    command: >
      postgres
      -c wal_level=replica
      -c max_wal_senders=10
      -c max_replication_slots=10
    volumes:
      - primary_data:/var/lib/postgresql/data
      - ./init-replica.sh:/docker-entrypoint-initdb.d/init-replica.sh:ro
    secrets:
      - db_password
    networks:
      - backend

  postgres-replica:
    image: postgres:17-alpine
    environment:
      PGUSER: replicator
      PGPASSWORD_FILE: /run/secrets/repl_password
    command: |
      bash -c "
      if [ ! -s /var/lib/postgresql/data/PG_VERSION ]; then
        until pg_basebackup -h postgres-primary -D /var/lib/postgresql/data -U replicator -R -P; do
          echo 'Waiting for primary to be ready...'
          sleep 5
        done
      fi
      exec postgres
      "
    volumes:
      - replica_data:/var/lib/postgresql/data
    depends_on:
      - postgres-primary
    secrets:
      - repl_password
    networks:
      - backend

volumes:
  primary_data:
  replica_data:

secrets:
  db_password:
    file: ./secrets/db_password.txt
  repl_password:
    file: ./secrets/repl_password.txt

networks:
  backend:
```

### PGD-009: HAProxy для Load Balancing

**Архитектура**:
```
Application → HAProxy → Primary (writes)
                      → Replica 1 (reads)
                      → Replica 2 (reads)
```

**HAProxy Configuration** (`haproxy.cfg`):

```
global
    maxconn 4096
    log stdout format raw local0

defaults
    mode tcp
    timeout connect 5s
    timeout client 30s
    timeout server 30s
    log global

# Primary для write operations
listen postgres_primary
    bind *:5432
    mode tcp
    option tcplog
    balance roundrobin
    option pgsql-check user postgres

    server primary postgres-primary:5432 check

# Read replicas для read operations
listen postgres_replicas
    bind *:5433
    mode tcp
    option tcplog
    balance leastconn
    option pgsql-check user postgres

    server replica1 postgres-replica1:5432 check
    server replica2 postgres-replica2:5432 check

# Stats interface
listen stats
    bind *:8404
    mode http
    stats enable
    stats uri /stats
    stats refresh 10s
```

**Docker Compose with HAProxy**:

```yaml
services:
  haproxy:
    image: haproxy:2.9-alpine
    restart: unless-stopped
    ports:
      - "5432:5432"   # Write traffic
      - "5433:5433"   # Read traffic
      - "8404:8404"   # Stats
    volumes:
      - ./haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro
    depends_on:
      - postgres-primary
      - postgres-replica1
      - postgres-replica2
    networks:
      - backend

  postgres-primary:
    # ... (см. выше)

  postgres-replica1:
    # ... (см. выше)

  postgres-replica2:
    # ... (см. выше)
```

**Application Usage**:
```go
// Write connection
writeDB := "postgres://user:pass@haproxy:5432/myapp"

// Read connection
readDB := "postgres://user:pass@haproxy:5433/myapp"
```

### 4. Load Balancing Patterns Summary

| Pattern | Use Case | Pros | Cons |
|---------|----------|------|------|
| **PgBouncer** | Connection pooling | Reduced connections, memory | Single point of failure |
| **Read Replicas** | Read-heavy workloads | Horizontal scaling | Replication lag, complexity |
| **HAProxy** | Multi-instance routing | Health checks, failover | Network hop |
| **Application Pool** | Simple apps | No external dependency | Limited to single app |

**Best Practice**: Комбинируй несколько паттернов:
```
Application → PgBouncer → HAProxy → Primary/Replicas
  (App pool)   (Connection pool) (LB)  (Replication)
```

---

## Security Standards

### 1. Authentication & Authorization

### PGD-010: SSL/TLS Encryption (ОБЯЗАТЕЛЬНО для production)

**Generate SSL Certificates**:
```bash
# Self-signed (dev/test only)
openssl req -new -x509 -days 365 -nodes -text \
  -out server.crt \
  -keyout server.key \
  -subj "/CN=postgres.example.com"

chmod 600 server.key
chown postgres:postgres server.key server.crt
```

**postgresql.conf**:
```ini
ssl = on
ssl_cert_file = '/etc/ssl/certs/server.crt'
ssl_key_file = '/etc/ssl/private/server.key'

# Enforce SSL
ssl_min_protocol_version = 'TLSv1.2'
ssl_ciphers = 'HIGH:!aNULL:!MD5'
```

**pg_hba.conf** (enforce SSL):
```
# TYPE  DATABASE        USER            ADDRESS                 METHOD
hostssl all             all             0.0.0.0/0               scram-sha-256
hostnossl all           all             0.0.0.0/0               reject
```

**Application Connection**:
```
postgres://user:pass@host:5432/db?sslmode=require
```

### PGD-011: SCRAM-SHA-256 Authentication (ОБЯЗАТЕЛЬНО)

**DEPRECATED**: MD5 authentication (weak)

**postgresql.conf**:
```ini
password_encryption = scram-sha-256
```

**Create User**:
```sql
CREATE ROLE myapp_user WITH LOGIN PASSWORD 'strong_password';
```

**pg_hba.conf**:
```
# TYPE  DATABASE        USER            ADDRESS                 METHOD
host    myapp           myapp_user      10.0.0.0/8              scram-sha-256
```

### 2. Access Control

### PGD-012: Least Privilege Principle

```sql
-- Создать read-only пользователя
CREATE ROLE readonly WITH LOGIN PASSWORD 'readonly_password';

-- Grant CONNECT
GRANT CONNECT ON DATABASE myapp TO readonly;

-- Grant USAGE на schema
GRANT USAGE ON SCHEMA public TO readonly;

-- Grant SELECT на все таблицы
GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly;

-- Для будущих таблиц
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT ON TABLES TO readonly;
```

### PGD-013: Application User Pattern

```sql
-- Application user (read-write)
CREATE ROLE app_user WITH LOGIN PASSWORD 'app_password';

-- Grant connect
GRANT CONNECT ON DATABASE myapp TO app_user;
GRANT USAGE, CREATE ON SCHEMA public TO app_user;

-- Grant DML (SELECT, INSERT, UPDATE, DELETE)
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user;

-- Запретить DDL (DROP, ALTER, etc.)
-- DDL выполняется через миграции (отдельный privileged user)
```

### PGD-014: Row-Level Security (RLS)

**Use Case**: Multi-tenant SaaS приложения.

```sql
-- Включить RLS на таблице
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;

-- Создать policy: пользователи видят только свои документы
CREATE POLICY tenant_isolation ON documents
  FOR ALL
  TO app_user
  USING (tenant_id = current_setting('app.current_tenant')::uuid);

-- Application устанавливает tenant_id
SET app.current_tenant = 'uuid-of-tenant';
```

### PGD-015: Audit Logging (pgAudit)

**Установка**:
```bash
sudo apt install -y postgresql-17-pgaudit
```

**postgresql.conf**:
```ini
shared_preload_libraries = 'pgaudit'

# Логировать все DDL и privilege changes
pgaudit.log = 'ddl, role'

# Логировать все операции для конкретной роли
pgaudit.log_role = 'auditor'
```

**Enable Extension**:
```sql
CREATE EXTENSION pgaudit;
```

### 5. Network Security

### PGD-016: pg_hba.conf Best Practices

```
# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Localhost (trusted)
local   all             postgres                                peer

# Replication
host    replication     replicator      10.0.0.0/8              scram-sha-256

# Application
hostssl myapp           app_user        10.0.0.0/8              scram-sha-256

# Monitoring
hostssl all             monitoring      10.0.0.0/8              scram-sha-256

# Reject everything else
host    all             all             0.0.0.0/0               reject
```

**ВАЖНО**: Порядок правил имеет значение (first match wins).

---

## Monitoring & Observability

### PGD-017: Built-in Statistics Views

```sql
-- Active queries
SELECT pid, usename, application_name, state, query, now() - query_start AS duration
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY duration DESC;

-- Slow queries
SELECT query, calls, total_exec_time, mean_exec_time, max_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 20;

-- Table sizes
SELECT schemaname, tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Index usage
SELECT schemaname, tablename, indexname, idx_scan
FROM pg_stat_user_indexes
WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;

-- Replication lag
SELECT client_addr, state,
  pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) AS lag_bytes,
  extract(epoch FROM (now() - pg_last_xact_replay_timestamp())) AS lag_seconds
FROM pg_stat_replication;
```

### PGD-018: pg_stat_statements (CRITICAL)

**Enable Extension**:
```sql
CREATE EXTENSION pg_stat_statements;
```

**postgresql.conf**:
```ini
shared_preload_libraries = 'pg_stat_statements'

pg_stat_statements.track = all
pg_stat_statements.max = 10000
```

**Полезные запросы**:
```sql
-- Top 10 slowest queries by mean time
SELECT query, calls, mean_exec_time, total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- Most frequently executed queries
SELECT query, calls
FROM pg_stat_statements
ORDER BY calls DESC
LIMIT 10;

-- Reset statistics
SELECT pg_stat_statements_reset();
```

### PGD-019: Prometheus Exporter

**postgres_exporter** (рекомендуется):

```yaml
services:
  postgres-exporter:
    image: prometheuscommunity/postgres-exporter:v0.15.0
    restart: unless-stopped
    environment:
      DATA_SOURCE_NAME: "postgresql://monitoring:password@postgres:5432/postgres?sslmode=disable"
    ports:
      - "9187:9187"
    depends_on:
      - postgres
    networks:
      - backend
      - monitoring

  postgres:
    # ... (см. выше)
```

**Monitoring User**:
```sql
CREATE ROLE monitoring WITH LOGIN PASSWORD 'monitoring_password';
GRANT pg_monitor TO monitoring;
```

**Key Metrics**:
- `pg_stat_database_xact_commit` - Transaction rate
- `pg_stat_database_blks_hit` / `pg_stat_database_blks_read` - Cache hit ratio
- `pg_stat_replication_replay_lag` - Replication lag
- `pg_locks` - Lock contention

### PGD-020: Health Checks

```bash
#!/bin/bash
# healthcheck.sh

pg_isready -h localhost -p 5432 -U postgres

if [ $? -eq 0 ]; then
  echo "PostgreSQL is ready"
  exit 0
else
  echo "PostgreSQL is not ready"
  exit 1
fi
```

**Kubernetes Probes**:
```yaml
livenessProbe:
  exec:
    command:
      - /bin/sh
      - -c
      - pg_isready -U postgres
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  exec:
    command:
      - /bin/sh
      - -c
      - pg_isready -U postgres && psql -U postgres -c "SELECT 1"
  initialDelaySeconds: 5
  periodSeconds: 5
```

---

## Backup & Recovery

### 1. Backup Strategies

| Strategy | Type | RPO | RTO | Use Case |
|----------|------|-----|-----|----------|
| **pg_dump** | Logical | Hours | Hours | Small DBs, migrations |
| **pg_basebackup** | Physical | Minutes | Minutes | Medium DBs |
| **WAL Archiving + PITR** | Physical | Seconds | Minutes | Production (RECOMMENDED) |
| **Streaming Replication** | Physical | Real-time | Seconds | HA |

### PGD-021: Logical Backup (pg_dump)

**Full Database Backup**:
```bash
# Plain SQL format (text)
pg_dump -U postgres -d myapp -F p -f myapp_backup.sql

# Custom format (compressed, parallel restore)
pg_dump -U postgres -d myapp -F c -f myapp_backup.dump

# Directory format (parallel dump and restore)
pg_dump -U postgres -d myapp -F d -j 4 -f myapp_backup_dir
```

**Restore**:
```bash
# From SQL file
psql -U postgres -d myapp < myapp_backup.sql

# From custom format
pg_restore -U postgres -d myapp -j 4 myapp_backup.dump

# From directory
pg_restore -U postgres -d myapp -j 4 myapp_backup_dir
```

**Automated Daily Backups**:
```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/mnt/backups/postgres"
DATE=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=7

# Create backup
pg_dump -U postgres -d myapp -F c -f "$BACKUP_DIR/myapp_$DATE.dump"

# Cleanup old backups
find "$BACKUP_DIR" -name "myapp_*.dump" -mtime +$RETENTION_DAYS -delete

# Upload to S3 (optional)
aws s3 cp "$BACKUP_DIR/myapp_$DATE.dump" s3://my-bucket/backups/
```

**Cron Job**:
```bash
# Daily at 2 AM
0 2 * * * /usr/local/bin/backup.sh >> /var/log/postgres_backup.log 2>&1
```

### PGD-022: Physical Backup (pg_basebackup)

```bash
# Create backup
pg_basebackup \
  -h postgres-primary \
  -D /mnt/backups/basebackup_$(date +%Y%m%d) \
  -U replicator \
  -P \
  -R \
  --wal-method=stream
```

**Restore**:
```bash
# Stop PostgreSQL
systemctl stop postgresql

# Restore data directory
rm -rf /var/lib/postgresql/17/main
cp -a /mnt/backups/basebackup_20251230 /var/lib/postgresql/17/main

# Start PostgreSQL
systemctl start postgresql
```

### 4. Point-In-Time Recovery (PITR)

**Архитектура**:
1. Базовый backup (pg_basebackup)
2. Continuous WAL archiving
3. Восстановление до любого момента времени

**Setup WAL Archiving** (`postgresql.conf`):
```ini
wal_level = replica
archive_mode = on
archive_command = 'test ! -f /mnt/wal_archive/%f && cp %p /mnt/wal_archive/%f'
archive_timeout = 300  # 5 minutes
```

**Create Base Backup**:
```bash
pg_basebackup \
  -D /mnt/backups/pitr_base \
  -U replicator \
  -P \
  --wal-method=none
```

**PITR Restore**:
```bash
# Stop PostgreSQL
systemctl stop postgresql

# Restore base backup
rm -rf /var/lib/postgresql/17/main
cp -a /mnt/backups/pitr_base /var/lib/postgresql/17/main

# Create recovery.signal
touch /var/lib/postgresql/17/main/recovery.signal

# Configure recovery
cat > /var/lib/postgresql/17/main/postgresql.auto.conf <<EOF
restore_command = 'cp /mnt/wal_archive/%f %p'
recovery_target_time = '2025-12-30 12:00:00'
recovery_target_action = 'promote'
EOF

# Start PostgreSQL (will recover to target time)
systemctl start postgresql
```

### PGD-024: Backup Automation with pgBackRest (Recommended)

**pgBackRest Features**:
- Parallel backup/restore
- Compression
- Encryption
- S3/Azure/GCS support
- Incremental/differential backups
- PITR

**Installation**:
```bash
sudo apt install -y pgbackrest
```

**Configuration** (`/etc/pgbackrest.conf`):
```ini
[global]
repo1-path=/var/lib/pgbackrest
repo1-retention-full=2
repo1-retention-diff=4

process-max=4
log-level-console=info
log-level-file=debug

[myapp]
pg1-path=/var/lib/postgresql/17/main
pg1-port=5432
```

**Backup Commands**:
```bash
# Full backup
pgbackrest --stanza=myapp backup --type=full

# Differential backup
pgbackrest --stanza=myapp backup --type=diff

# Incremental backup
pgbackrest --stanza=myapp backup --type=incr

# List backups
pgbackrest --stanza=myapp info

# Restore latest
pgbackrest --stanza=myapp restore

# PITR restore
pgbackrest --stanza=myapp restore \
  --type=time \
  --target="2025-12-30 12:00:00"
```

---

## High Availability

### PGD-025: Patroni (Recommended для Production HA)

**Patroni Features**:
- Automatic failover
- Leader election (via etcd/consul/zookeeper)
- Health checks
- REST API

**Architecture**:
```
        ┌─→ Patroni Node 1 (Leader)
etcd ───┼─→ Patroni Node 2 (Replica)
        └─→ Patroni Node 3 (Replica)
             ↑
          HAProxy
```

**Docker Compose: Patroni + etcd**:
```yaml
services:
  etcd:
    image: quay.io/coreos/etcd:v3.5.11
    command: >
      /usr/local/bin/etcd
      --name etcd0
      --advertise-client-urls http://etcd:2379
      --listen-client-urls http://0.0.0.0:2379
      --initial-advertise-peer-urls http://etcd:2380
      --listen-peer-urls http://0.0.0.0:2380
      --initial-cluster etcd0=http://etcd:2380
    networks:
      - backend

  patroni1:
    image: patroni/patroni:3.2.0
    environment:
      PATRONI_NAME: patroni1
      PATRONI_SCOPE: postgres-cluster
      PATRONI_RESTAPI_CONNECT_ADDRESS: patroni1:8008
      PATRONI_POSTGRESQL_CONNECT_ADDRESS: patroni1:5432
      PATRONI_POSTGRESQL_DATA_DIR: /var/lib/postgresql/data
      PATRONI_ETCD3_HOSTS: etcd:2379
      PATRONI_POSTGRESQL_PGPASS: /tmp/pgpass
    volumes:
      - patroni1_data:/var/lib/postgresql/data
    depends_on:
      - etcd
    networks:
      - backend

  patroni2:
    image: patroni/patroni:3.2.0
    environment:
      PATRONI_NAME: patroni2
      # ... (similar config)
    volumes:
      - patroni2_data:/var/lib/postgresql/data
    depends_on:
      - etcd
    networks:
      - backend

  haproxy:
    image: haproxy:2.9-alpine
    ports:
      - "5432:5432"  # Primary
      - "5433:5433"  # Replicas
    volumes:
      - ./haproxy-patroni.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro
    depends_on:
      - patroni1
      - patroni2
    networks:
      - backend

volumes:
  patroni1_data:
  patroni2_data:

networks:
  backend:
```

**HAProxy Config for Patroni** (`haproxy-patroni.cfg`):
```
listen postgres_primary
    bind *:5432
    mode tcp
    option httpchk GET /primary
    http-check expect status 200

    server patroni1 patroni1:5432 check port 8008
    server patroni2 patroni2:5432 check port 8008

listen postgres_replicas
    bind *:5433
    mode tcp
    option httpchk GET /replica
    http-check expect status 200

    server patroni1 patroni1:5432 check port 8008
    server patroni2 patroni2:5432 check port 8008
```

### PGD-026: Synchronous Replication

**Trade-off**: Higher durability vs latency.

**postgresql.conf**:
```ini
synchronous_commit = on
synchronous_standby_names = 'FIRST 1 (replica1, replica2)'
```

**Modes**:
- `off`: Asynchronous (fast, risk of data loss)
- `local`: Wait for local WAL write (default)
- `remote_write`: Wait for replica WAL write (some risk)
- `on`: Wait for replica WAL flush (safe, RECOMMENDED для HA)
- `remote_apply`: Wait for replica apply (slowest, strongest guarantee)

---

## Performance Optimization

### PGD-027: Index Strategies

```sql
-- B-tree index (default, most common)
CREATE INDEX idx_users_email ON users(email);

-- Unique index
CREATE UNIQUE INDEX idx_users_email_unique ON users(email);

-- Partial index (для конкретных условий)
CREATE INDEX idx_active_users ON users(email) WHERE active = true;

-- Composite index
CREATE INDEX idx_orders_user_date ON orders(user_id, created_at DESC);

-- BRIN index (для больших таблиц с естественной сортировкой)
CREATE INDEX idx_events_timestamp ON events USING BRIN(created_at);

-- GIN index (для full-text search, arrays, JSONB)
CREATE INDEX idx_documents_content ON documents USING GIN(to_tsvector('english', content));

-- Hash index (для equality comparisons)
CREATE INDEX idx_users_hash ON users USING HASH(user_id);
```

**When to Use**:
- B-tree: General purpose (90% of cases)
- BRIN: Large tables with natural ordering (timestamps)
- GIN: JSONB, arrays, full-text search
- Hash: Equality only (rarely used)

### PGD-028: Query Optimization

**EXPLAIN ANALYZE**:
```sql
EXPLAIN (ANALYZE, BUFFERS, VERBOSE)
SELECT u.name, COUNT(o.id)
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE u.created_at > '2025-01-01'
GROUP BY u.id, u.name;
```

**Key Metrics**:
- `Seq Scan` → Add index
- `Nested Loop` with high rows → Consider hash/merge join
- High `Buffers` → Cache misses

### PGD-029: Partitioning

**Range Partitioning** (для time-series data):
```sql
-- Parent table
CREATE TABLE events (
  id BIGSERIAL,
  user_id BIGINT,
  event_type VARCHAR(50),
  created_at TIMESTAMP NOT NULL
) PARTITION BY RANGE (created_at);

-- Partitions
CREATE TABLE events_2025_01 PARTITION OF events
  FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE events_2025_02 PARTITION OF events
  FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- Indexes на каждой partition
CREATE INDEX ON events_2025_01(user_id);
CREATE INDEX ON events_2025_02(user_id);
```

**Benefits**:
- Query performance (partition pruning)
- Easy archival (DROP partition вместо DELETE)
- Smaller indexes

### PGD-030: Vacuum & Analyze

```sql
-- Manual vacuum
VACUUM ANALYZE users;

-- Full vacuum (locks table, reclaims space)
VACUUM FULL users;

-- Check bloat
SELECT schemaname, tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
  n_dead_tup
FROM pg_stat_user_tables
ORDER BY n_dead_tup DESC;
```

---

## Production Patterns

### PGD-031: Database Migrations

**Best Practices**:
- Always use migration tool (golang-migrate, Flyway, Liquibase)
- Test migrations on staging
- Use transactions (`BEGIN; ... COMMIT;`)
- Avoid long-running DDL during peak hours

**Example: golang-migrate**:
```bash
# Install
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Create migration
migrate create -ext sql -dir migrations add_users_table

# Apply migrations
migrate -path migrations -database postgres://user:pass@host/db up

# Rollback
migrate -path migrations -database postgres://user:pass@host/db down 1
```

### PGD-032: Zero-Downtime Deployments

**Backward-Compatible Migrations**:

❌ **WRONG** (breaks old code):
```sql
-- Don't do this if old code is still running
ALTER TABLE users DROP COLUMN old_field;
ALTER TABLE users RENAME COLUMN old TO new;
```

✅ **CORRECT** (multi-step):
```sql
-- Step 1: Add new column (nullable)
ALTER TABLE users ADD COLUMN new_field VARCHAR(255);

-- Deploy code that writes to BOTH old and new fields

-- Step 2: Backfill data
UPDATE users SET new_field = old_field WHERE new_field IS NULL;

-- Step 3: Add NOT NULL constraint
ALTER TABLE users ALTER COLUMN new_field SET NOT NULL;

-- Deploy code that uses only new field

-- Step 4: Drop old column
ALTER TABLE users DROP COLUMN old_field;
```

### PGD-033: Monitoring Checklist

- [ ] Active queries monitoring
- [ ] Slow query log
- [ ] Replication lag
- [ ] Connection count
- [ ] Cache hit ratio (> 95%)
- [ ] Table/index bloat
- [ ] Disk space
- [ ] Backup success/failure
- [ ] Lock contention

---

## Common Gotchas

| Problem | Symptom | Solution |
|---------|---------|----------|
| **Connection exhaustion** | `too many connections` | Use PgBouncer |
| **Slow queries** | High latency | Add indexes, optimize queries |
| **Table bloat** | Disk full, slow queries | VACUUM FULL, autovacuum tuning |
| **Replication lag** | Stale reads | Tune `wal_sender_timeout`, check network |
| **Lock contention** | Blocked queries | Use `pg_locks`, avoid long transactions |
| **Memory OOM** | PostgreSQL crashes | Reduce `shared_buffers`, `work_mem` |
| **SSL certificate expired** | Connection refused | Renew certificates |
| **Transaction ID wraparound** | Database shutdown | Run VACUUM, monitor `age(datfrozenxid)` |

---

## Quick Reference

### Configuration Template (8GB RAM)

```ini
# Memory
shared_buffers = 2GB
effective_cache_size = 6GB
maintenance_work_mem = 512MB
work_mem = 16MB

# Connections
max_connections = 100
statement_timeout = 30000

# WAL
wal_level = replica
max_wal_size = 4GB
checkpoint_timeout = 15min

# Replication
max_wal_senders = 10
max_replication_slots = 10

# Query Planner
random_page_cost = 1.1
effective_io_concurrency = 200

# Logging
log_min_duration_statement = 1000
log_connections = on
log_checkpoints = on

# Autovacuum
autovacuum = on
autovacuum_max_workers = 3
```

### Useful Commands

```bash
# Check version
psql -U postgres -c "SELECT version();"

# List databases
psql -U postgres -l

# Database size
psql -U postgres -c "SELECT pg_database_size('myapp');"

# Connection count
psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"

# Kill connection
psql -U postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE pid = 12345;"

# Reload config
psql -U postgres -c "SELECT pg_reload_conf();"

# Current WAL location
psql -U postgres -c "SELECT pg_current_wal_lsn();"
```

---

## Version Compatibility (2025)

| Version | Status | Release | EOL |
|---------|--------|---------|-----|
| 17 | Current | Sep 2024 | Nov 2029 |
| 16 | Maintained | Sep 2023 | Nov 2028 |
| 15 | Maintained | Oct 2022 | Nov 2027 |
| 14 | Maintained | Sep 2021 | Nov 2026 |

**Upgrade Policy**: Always test on staging first. Use `pg_upgrade` for major version upgrades.

---

## Дополнительные ресурсы

### Official Documentation
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [High Availability](https://www.postgresql.org/docs/current/high-availability.html)
- [Performance Tips](https://wiki.postgresql.org/wiki/Performance_Optimization)

### Tools
- [PgBouncer](https://www.pgbouncer.org/) - Connection pooler
- [Patroni](https://patroni.readthedocs.io/) - HA solution
- [pgBackRest](https://pgbackrest.org/) - Backup & restore
- [postgres_exporter](https://github.com/prometheus-community/postgres_exporter) - Prometheus exporter

### Best Practices
- [Postgres Guide](https://www.postgresguide.com/)
- [Use The Index, Luke!](https://use-the-index-luke.com/) - SQL indexing guide

### Research Report
См. полный отчет: `.memory_bank/agent_task_results/2025-12-29_web-research_postgresql-devops-best-practices.md`

---

**При возникновении вопросов**: Обратись к DevOps engineer agent или см. research report.
