# MySQL Best Practices and Standards

This document contains MySQL deployment rules for DevOps tasks. Each rule has a unique identifier (MYD-XXXX) for reference in code reviews and task execution.

> **Related**: For MySQL query patterns, see `dba/mysql/`
> **Related**: For full DevOps index, see [rules_index.yaml](./rules_index.yaml)

**Последнее обновление**: 2026-02-02
**Статус**: Обязательно к прочтению перед любыми MySQL задачами
**Текущая версия MySQL**: 8.4 LTS (2025)

---

## Quick Reference

| ID | Rule | Critical | Section |
|----|------|----------|---------|
| MYD-001 | Set innodb_buffer_pool_size = 70% RAM | ✅ | [InnoDB Configuration](#myd-001-set-innodb_buffer_pool_size--70-ram) |
| MYD-002 | Enable binary logging for replication | ✅ | [Binary Logging](#myd-002-enable-binary-logging-for-replication) |
| MYD-003 | Use ProxySQL for connection pooling | | [Connection Pooling](#myd-003-use-proxysql-for-connection-pooling) |

---

## Содержание

1. [Installation & Configuration](#installation--configuration)
2. [Security Standards](#security-standards)
3. [Load Balancing](#load-balancing)
4. [Replication](#replication)
5. [Performance Optimization](#performance-optimization)
6. [Backup & Recovery](#backup--recovery)
7. [Monitoring](#monitoring)
8. [Production Patterns](#production-patterns)
9. [Common Anti-Patterns](#common-anti-patterns)

---

## Installation & Configuration

### MYD-001: 1. MySQL Version Strategy

| Version | Type | Support Until | Recommendation |
|---------|------|---------------|----------------|
| **8.4 LTS** | Innovation | April 2032 | Production (RECOMMENDED) |
| **8.0 LTS** | LTS | April 2026 | Legacy support |
| **9.0** | Innovation | July 2025 | Testing only |

**КРИТИЧНО**: Всегда используй LTS версии для production.

### MYD-002: 2. Docker Production Setup

**ОБЯЗАТЕЛЬНО**: Multi-stage build с security hardening.

```dockerfile
# syntax=docker/dockerfile:1
FROM mysql:8.4 AS production

# Security: Run as mysql user (uid 999)
USER mysql

# Custom configuration
COPY --chown=mysql:mysql my.cnf /etc/mysql/conf.d/custom.cnf

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD mysqladmin ping -h localhost -u root -p${MYSQL_ROOT_PASSWORD} || exit 1

EXPOSE 3306

# Volumes
VOLUME /var/lib/mysql
VOLUME /var/log/mysql
```

#### Docker Compose (Production-Ready)

```yaml
services:
  mysql:
    image: mysql:8.4
    container_name: mysql-primary
    restart: unless-stopped

    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
      MYSQL_DATABASE: myapp
      MYSQL_USER: appuser
      MYSQL_PASSWORD_FILE: /run/secrets/mysql_user_password

    secrets:
      - mysql_root_password
      - mysql_user_password

    volumes:
      - mysql_data:/var/lib/mysql
      - mysql_logs:/var/log/mysql
      - ./my.cnf:/etc/mysql/conf.d/custom.cnf:ro
      - ./init:/docker-entrypoint-initdb.d:ro

    ports:
      - "127.0.0.1:3306:3306"  # Bind to localhost only

    networks:
      - backend

    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s

    # Resource limits
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 4G
        reservations:
          cpus: '1'
          memory: 2G

networks:
  backend:
    internal: true

volumes:
  mysql_data:
  mysql_logs:

secrets:
  mysql_root_password:
    file: ./secrets/mysql_root_password.txt
  mysql_user_password:
    file: ./secrets/mysql_user_password.txt
```

### MYD-003: 3. Core Configuration (`my.cnf`)

**Путь**: `/etc/mysql/conf.d/custom.cnf`

```ini
[mysqld]
# === Server Identification ===
server-id = 1
bind-address = 0.0.0.0
port = 3306

# === Character Set (ОБЯЗАТЕЛЬНО utf8mb4) ===
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci

# === InnoDB Configuration ===
innodb_buffer_pool_size = 2G        # 70-80% RAM для dedicated server
innodb_log_file_size = 512M         # 25% от buffer pool
innodb_flush_log_at_trx_commit = 1  # ACID compliance (durability)
innodb_flush_method = O_DIRECT       # Avoid double buffering

# === Connection Settings ===
max_connections = 200
max_connect_errors = 1000000
connect_timeout = 10
wait_timeout = 28800
interactive_timeout = 28800

# === Query Cache (Deprecated in 8.0+) ===
# query_cache_type = 0
# query_cache_size = 0

# === Logging ===
log_error = /var/log/mysql/error.log
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 2
log_queries_not_using_indexes = 1

# === Binary Logging (for replication) ===
log_bin = /var/log/mysql/mysql-bin.log
binlog_format = ROW                 # Recommended for consistency
expire_logs_days = 7
max_binlog_size = 100M
binlog_cache_size = 1M

# === Replication ===
relay_log = /var/log/mysql/relay-bin
sync_binlog = 1                     # Crash-safe replication

# === Security ===
local_infile = 0                    # Disable LOAD DATA LOCAL INFILE
skip_name_resolve = 1               # Use IPs, not DNS

# === Performance Schema ===
performance_schema = ON
performance_schema_instrument = 'stage/%=ON'
performance_schema_consumer_events_stages_current = ON
```

**ВАЖНО**: Настрой `innodb_buffer_pool_size` под свою RAM:
- Dedicated DB server: 70-80% RAM
- Shared server: 50% RAM

---

## Security Standards

### MYD-004: 1. User Management (CRITICAL)

#### NEVER Use Root in Application

```sql
-- ❌ WRONG: Using root
CREATE USER 'root'@'%' IDENTIFIED BY 'password';

-- ✅ CORRECT: Application user with minimal privileges
CREATE USER 'appuser'@'%' IDENTIFIED BY 'StrongPassword123!';
GRANT SELECT, INSERT, UPDATE, DELETE ON myapp.* TO 'appuser'@'%';
FLUSH PRIVILEGES;
```

#### Host-Based Access Control

```sql
-- Specific host
CREATE USER 'appuser'@'192.168.1.100' IDENTIFIED BY 'password';

-- Subnet
CREATE USER 'appuser'@'192.168.1.%' IDENTIFIED BY 'password';

-- Localhost only (most secure)
CREATE USER 'appuser'@'localhost' IDENTIFIED BY 'password';

-- Grant privileges
GRANT SELECT, INSERT, UPDATE, DELETE ON myapp.* TO 'appuser'@'192.168.1.%';
FLUSH PRIVILEGES;
```

#### Password Policy (MySQL 8.0+)

```sql
-- Check current policy
SHOW VARIABLES LIKE 'validate_password%';

-- Set policy
SET GLOBAL validate_password.policy = STRONG;
SET GLOBAL validate_password.length = 12;
SET GLOBAL validate_password.mixed_case_count = 1;
SET GLOBAL validate_password.number_count = 1;
SET GLOBAL validate_password.special_char_count = 1;
```

### MYD-005: 2. SSL/TLS Encryption (ОБЯЗАТЕЛЬНО для production)

#### Generate Certificates

```bash
# Using mysql_ssl_rsa_setup (MySQL 8.0+)
mysql_ssl_rsa_setup --datadir=/var/lib/mysql

# Verify
ls -la /var/lib/mysql/*.pem
```

#### Configure SSL

```ini
# my.cnf
[mysqld]
ssl_ca = /var/lib/mysql/ca.pem
ssl_cert = /var/lib/mysql/server-cert.pem
ssl_key = /var/lib/mysql/server-key.pem
require_secure_transport = ON  # Force SSL for all connections
```

#### Create User with SSL Requirement

```sql
CREATE USER 'ssluser'@'%' IDENTIFIED BY 'password' REQUIRE SSL;
GRANT ALL PRIVILEGES ON myapp.* TO 'ssluser'@'%';
```

#### Client Connection

```bash
mysql -u ssluser -p \
  --ssl-ca=/path/to/ca.pem \
  --ssl-cert=/path/to/client-cert.pem \
  --ssl-key=/path/to/client-key.pem
```

### MYD-006: 3. Audit Logging (Enterprise Edition)

```sql
-- Install audit plugin
INSTALL PLUGIN audit_log SONAME 'audit_log.so';

-- Configure
SET GLOBAL audit_log_file = '/var/log/mysql/audit.log';
SET GLOBAL audit_log_format = JSON;
SET GLOBAL audit_log_policy = ALL;

-- Verify
SELECT PLUGIN_NAME, PLUGIN_STATUS FROM INFORMATION_SCHEMA.PLUGINS
WHERE PLUGIN_NAME = 'audit_log';
```

### MYD-007: 4. Firewall Rules

```bash
# iptables (только для trusted IPs)
iptables -A INPUT -p tcp -s 192.168.1.0/24 --dport 3306 -j ACCEPT
iptables -A INPUT -p tcp --dport 3306 -j DROP

# ufw
ufw allow from 192.168.1.0/24 to any port 3306
ufw deny 3306
```

---

## Load Balancing

**КРИТИЧНО**: Load balancing обеспечивает высокую доступность и горизонтальное масштабирование.

### MYD-008: 1. Replication Patterns

| Pattern | Read Scalability | Write Scalability | Complexity | Use Case |
|---------|------------------|-------------------|------------|----------|
| **Master-Slave** | High | Low | Low | Read-heavy |
| **Master-Master** | Medium | Medium | Medium | Multi-region |
| **Galera Cluster** | High | High | High | Multi-master sync |
| **Group Replication** | High | High | High | Auto failover |

### MYD-009: 2. Master-Slave Replication + ProxySQL

**Architecture**:
```
┌─────────┐
│ ProxySQL│ (Port 6033)
└────┬────┘
     │
     ├──────────────┬──────────────┐
     │              │              │
┌────▼────┐   ┌────▼────┐   ┌────▼────┐
│ Master  │   │ Slave 1 │   │ Slave 2 │
│ (Write) │──▶│ (Read)  │   │ (Read)  │
└─────────┘   └─────────┘   └─────────┘
```

**Benefits**:
- Read/write split автоматически
- Connection pooling
- Query caching
- Health checks & automatic failover
- Zero downtime maintenance

#### Docker Compose with ProxySQL

```yaml
services:
  mysql-master:
    image: mysql:8.4
    container_name: mysql-master
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
      MYSQL_REPLICATION_MODE: master
      MYSQL_REPLICATION_USER: repl_user
      MYSQL_REPLICATION_PASSWORD_FILE: /run/secrets/mysql_repl_password
    volumes:
      - master_data:/var/lib/mysql
      - ./master.cnf:/etc/mysql/conf.d/custom.cnf:ro
    networks:
      - mysql_cluster
    secrets:
      - mysql_root_password
      - mysql_repl_password

  mysql-slave-1:
    image: mysql:8.4
    container_name: mysql-slave-1
    depends_on:
      - mysql-master
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
      MYSQL_REPLICATION_MODE: slave
      MYSQL_MASTER_HOST: mysql-master
      MYSQL_MASTER_PORT: 3306
      MYSQL_REPLICATION_USER: repl_user
      MYSQL_REPLICATION_PASSWORD_FILE: /run/secrets/mysql_repl_password
    volumes:
      - slave1_data:/var/lib/mysql
      - ./slave.cnf:/etc/mysql/conf.d/custom.cnf:ro
    networks:
      - mysql_cluster
    secrets:
      - mysql_root_password
      - mysql_repl_password

  mysql-slave-2:
    image: mysql:8.4
    container_name: mysql-slave-2
    depends_on:
      - mysql-master
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
      MYSQL_REPLICATION_MODE: slave
      MYSQL_MASTER_HOST: mysql-master
      MYSQL_MASTER_PORT: 3306
      MYSQL_REPLICATION_USER: repl_user
      MYSQL_REPLICATION_PASSWORD_FILE: /run/secrets/mysql_repl_password
    volumes:
      - slave2_data:/var/lib/mysql
      - ./slave.cnf:/etc/mysql/conf.d/custom.cnf:ro
    networks:
      - mysql_cluster
    secrets:
      - mysql_root_password
      - mysql_repl_password

  proxysql:
    image: proxysql/proxysql:2.5
    container_name: proxysql
    depends_on:
      - mysql-master
      - mysql-slave-1
      - mysql-slave-2
    ports:
      - "6033:6033"  # MySQL proxy port
      - "6032:6032"  # Admin interface
    volumes:
      - ./proxysql.cnf:/etc/proxysql.cnf:ro
      - proxysql_data:/var/lib/proxysql
    networks:
      - mysql_cluster
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "-P", "6032"]
      interval: 10s
      timeout: 5s
      retries: 3

networks:
  mysql_cluster:

volumes:
  master_data:
  slave1_data:
  slave2_data:
  proxysql_data:

secrets:
  mysql_root_password:
    file: ./secrets/mysql_root_password.txt
  mysql_repl_password:
    file: ./secrets/mysql_repl_password.txt
```

#### ProxySQL Configuration (`proxysql.cnf`)

```ini
datadir="/var/lib/proxysql"

admin_variables=
{
    admin_credentials="admin:admin"
    mysql_ifaces="0.0.0.0:6032"
}

mysql_variables=
{
    threads=4
    max_connections=2048
    default_query_delay=0
    default_query_timeout=36000000
    have_compress=true
    poll_timeout=2000
    interfaces="0.0.0.0:6033"
    default_schema="information_schema"
    stacksize=1048576
    server_version="8.4.0"
    connect_timeout_server=3000
    monitor_username="monitor"
    monitor_password="monitor"
    monitor_history=600000
    monitor_connect_interval=60000
    monitor_ping_interval=10000
    monitor_read_only_interval=1500
    monitor_read_only_timeout=500
    ping_interval_server_msec=120000
    ping_timeout_server=500
    commands_stats=true
    sessions_sort=true
    connect_retries_on_failure=10
}

# Define MySQL servers
mysql_servers =
(
    # Master (hostgroup 0 - write)
    {
        address="mysql-master"
        port=3306
        hostgroup=0
        max_connections=200
        weight=1
        comment="Master"
    },
    # Slave 1 (hostgroup 1 - read)
    {
        address="mysql-slave-1"
        port=3306
        hostgroup=1
        max_connections=200
        weight=1
        comment="Slave 1"
    },
    # Slave 2 (hostgroup 1 - read)
    {
        address="mysql-slave-2"
        port=3306
        hostgroup=1
        max_connections=200
        weight=1
        comment="Slave 2"
    }
)

# Define users
mysql_users =
(
    {
        username = "appuser"
        password = "StrongPassword123!"
        default_hostgroup = 0
        max_connections = 200
        active = 1
    }
)

# Query routing rules
mysql_query_rules =
(
    # Route SELECT to read hostgroup (slaves)
    {
        rule_id=1
        active=1
        match_pattern="^SELECT .* FOR UPDATE$"
        destination_hostgroup=0
        apply=1
    },
    {
        rule_id=2
        active=1
        match_pattern="^SELECT"
        destination_hostgroup=1
        apply=1
    },
    # Route writes to master
    {
        rule_id=3
        active=1
        match_pattern="^(INSERT|UPDATE|DELETE|REPLACE)"
        destination_hostgroup=0
        apply=1
    }
)
```

#### ProxySQL Runtime Configuration

```sql
-- Connect to ProxySQL admin interface
mysql -u admin -p -h 127.0.0.1 -P 6032

-- Add servers
INSERT INTO mysql_servers(hostgroup_id, hostname, port) VALUES (0, 'mysql-master', 3306);
INSERT INTO mysql_servers(hostgroup_id, hostname, port) VALUES (1, 'mysql-slave-1', 3306);
INSERT INTO mysql_servers(hostgroup_id, hostname, port) VALUES (1, 'mysql-slave-2', 3306);

-- Add users
INSERT INTO mysql_users(username, password, default_hostgroup)
VALUES ('appuser', 'StrongPassword123!', 0);

-- Add query rules (read/write split)
INSERT INTO mysql_query_rules(rule_id, active, match_pattern, destination_hostgroup, apply)
VALUES (1, 1, '^SELECT .* FOR UPDATE$', 0, 1);

INSERT INTO mysql_query_rules(rule_id, active, match_pattern, destination_hostgroup, apply)
VALUES (2, 1, '^SELECT', 1, 1);

-- Load to runtime
LOAD MYSQL SERVERS TO RUNTIME;
LOAD MYSQL USERS TO RUNTIME;
LOAD MYSQL QUERY RULES TO RUNTIME;

-- Persist to disk
SAVE MYSQL SERVERS TO DISK;
SAVE MYSQL USERS TO DISK;
SAVE MYSQL QUERY RULES TO DISK;

-- Check status
SELECT * FROM stats_mysql_connection_pool;
SELECT * FROM stats_mysql_query_rules;
```

### MYD-010: 3. MySQL Router (Official Oracle Solution)

**Architecture**:
```
┌──────────────┐
│ MySQL Router │ (Port 6446 R/W, 6447 R/O)
└──────┬───────┘
       │
       ├──────────────┬──────────────┐
       │              │              │
  ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
  │Primary  │   │Secondary│   │Secondary│
  │(R/W)    │──▶│(R/O)    │   │(R/O)    │
  └─────────┘   └─────────┘   └─────────┘
```

#### Docker Compose with MySQL Router

```yaml
services:
  mysql-primary:
    image: mysql:8.4
    container_name: mysql-primary
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
    volumes:
      - primary_data:/var/lib/mysql
    networks:
      - mysql_cluster
    secrets:
      - mysql_root_password

  mysql-router:
    image: mysql/mysql-router:8.4
    container_name: mysql-router
    depends_on:
      - mysql-primary
    environment:
      MYSQL_HOST: mysql-primary
      MYSQL_PORT: 3306
      MYSQL_USER: root
      MYSQL_PASSWORD_FILE: /run/secrets/mysql_root_password
      MYSQL_INNODB_CLUSTER_MEMBERS: 3
    ports:
      - "6446:6446"  # R/W port
      - "6447:6447"  # R/O port
    networks:
      - mysql_cluster
    secrets:
      - mysql_root_password

networks:
  mysql_cluster:

volumes:
  primary_data:

secrets:
  mysql_root_password:
    file: ./secrets/mysql_root_password.txt
```

### MYD-011: 4. Galera Cluster (Multi-Master Synchronous Replication)

**Architecture**: All nodes are masters - synchronous replication.

```
┌────────────┐     ┌────────────┐     ┌────────────┐
│  Galera 1  │◄───►│  Galera 2  │◄───►│  Galera 3  │
│ (R/W)      │     │ (R/W)      │     │ (R/W)      │
└────────────┘     └────────────┘     └────────────┘
      ▲                  ▲                  ▲
      │                  │                  │
      └──────────────────┴──────────────────┘
                    Load Balancer
```

**Benefits**:
- True multi-master (все узлы равны)
- Synchronous replication (нет data loss)
- Automatic node provisioning
- Read AND write scalability

**Tradeoffs**:
- Write latency (consensus required)
- Not suitable for WAN latency >100ms
- Cluster size должен быть нечётный (3, 5, 7)

#### Docker Compose Galera Cluster

```yaml
services:
  galera-1:
    image: mariadb:11.2
    container_name: galera-1
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
      MYSQL_DATABASE: myapp
    command:
      - --wsrep-new-cluster
      - --wsrep_node_address=galera-1
      - --wsrep_cluster_address=gcomm://galera-1,galera-2,galera-3
      - --wsrep_cluster_name=galera_cluster
      - --wsrep_node_name=galera-1
    volumes:
      - galera1_data:/var/lib/mysql
    networks:
      - galera_cluster
    secrets:
      - mysql_root_password

  galera-2:
    image: mariadb:11.2
    container_name: galera-2
    depends_on:
      - galera-1
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
    command:
      - --wsrep_node_address=galera-2
      - --wsrep_cluster_address=gcomm://galera-1,galera-2,galera-3
      - --wsrep_cluster_name=galera_cluster
      - --wsrep_node_name=galera-2
    volumes:
      - galera2_data:/var/lib/mysql
    networks:
      - galera_cluster
    secrets:
      - mysql_root_password

  galera-3:
    image: mariadb:11.2
    container_name: galera-3
    depends_on:
      - galera-1
    environment:
      MYSQL_ROOT_PASSWORD_FILE: /run/secrets/mysql_root_password
    command:
      - --wsrep_node_address=galera-3
      - --wsrep_cluster_address=gcomm://galera-1,galera-2,galera-3
      - --wsrep_cluster_name=galera_cluster
      - --wsrep_node_name=galera-3
    volumes:
      - galera3_data:/var/lib/mysql
    networks:
      - galera_cluster
    secrets:
      - mysql_root_password

  haproxy:
    image: haproxy:2.9
    container_name: haproxy
    depends_on:
      - galera-1
      - galera-2
      - galera-3
    ports:
      - "3306:3306"
      - "8404:8404"  # Stats page
    volumes:
      - ./haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro
    networks:
      - galera_cluster

networks:
  galera_cluster:

volumes:
  galera1_data:
  galera2_data:
  galera3_data:

secrets:
  mysql_root_password:
    file: ./secrets/mysql_root_password.txt
```

#### HAProxy Configuration for Galera

```cfg
global
    maxconn 4096
    log stdout format raw local0

defaults
    mode tcp
    timeout connect 5s
    timeout client 30s
    timeout server 30s
    log global

listen stats
    bind *:8404
    mode http
    stats enable
    stats uri /stats
    stats refresh 30s

listen mysql-cluster
    bind *:3306
    mode tcp
    option mysql-check user haproxy
    balance roundrobin

    server galera-1 galera-1:3306 check
    server galera-2 galera-2:3306 check
    server galera-3 galera-3:3306 check
```

### MYD-012: 5. Load Balancing Comparison

| Solution | Type | Complexity | Failover | Read Split | Write Split | Best For |
|----------|------|------------|----------|------------|-------------|----------|
| **ProxySQL** | Proxy | Medium | Auto | Yes | No | Read-heavy workloads |
| **MySQL Router** | Proxy | Low | Auto | Yes | No | InnoDB Cluster |
| **Galera + HAProxy** | Multi-Master | High | Auto | Yes | Yes | Multi-region, HA |
| **Group Replication** | Native | Medium | Auto | Yes | Limited | Oracle ecosystem |

**Рекомендации**:
- **Read-heavy (90%+ reads)**: ProxySQL + Master-Slave
- **Write-heavy**: Galera Cluster
- **Simple HA**: MySQL Router + InnoDB Cluster
- **Cloud-native**: Aurora MySQL / Cloud SQL

---

## Replication

### MYD-013: 1. Master-Slave Replication Setup

#### Master Configuration

```sql
-- Create replication user
CREATE USER 'repl_user'@'%' IDENTIFIED BY 'ReplicationPassword123!';
GRANT REPLICATION SLAVE ON *.* TO 'repl_user'@'%';
FLUSH PRIVILEGES;

-- Get master status
SHOW MASTER STATUS;
-- Note: File and Position
```

**my.cnf (Master)**:
```ini
[mysqld]
server-id = 1
log_bin = /var/log/mysql/mysql-bin.log
binlog_format = ROW
binlog_do_db = myapp  # Optional: only replicate this DB
```

#### Slave Configuration

**my.cnf (Slave)**:
```ini
[mysqld]
server-id = 2
relay_log = /var/log/mysql/relay-bin
read_only = 1  # Prevent writes
```

**Connect Slave to Master**:
```sql
CHANGE MASTER TO
  MASTER_HOST='mysql-master',
  MASTER_USER='repl_user',
  MASTER_PASSWORD='ReplicationPassword123!',
  MASTER_LOG_FILE='mysql-bin.000001',  # From SHOW MASTER STATUS
  MASTER_LOG_POS=12345;                # From SHOW MASTER STATUS

START SLAVE;

-- Check status
SHOW SLAVE STATUS\G
-- Look for: Slave_IO_Running: Yes, Slave_SQL_Running: Yes
```

### MYD-014: 2. Replication Monitoring

```sql
-- Master
SHOW MASTER STATUS;
SHOW SLAVE HOSTS;

-- Slave
SHOW SLAVE STATUS\G

-- Check lag
SELECT
  TIMESTAMPDIFF(SECOND, ts, NOW()) AS lag_seconds
FROM (
  SELECT MAX(ts) AS ts
  FROM mysql.gtid_executed
) t;
```

### MYD-015: 3. GTID-Based Replication (Recommended)

**Benefits**: Automatic failover, easier management

**my.cnf**:
```ini
[mysqld]
gtid_mode = ON
enforce_gtid_consistency = ON
log_slave_updates = ON
```

**Slave Setup**:
```sql
CHANGE MASTER TO
  MASTER_HOST='mysql-master',
  MASTER_USER='repl_user',
  MASTER_PASSWORD='ReplicationPassword123!',
  MASTER_AUTO_POSITION=1;  # Use GTID

START SLAVE;
```

---

## Performance Optimization

### MYD-016: 1. InnoDB Tuning

```ini
[mysqld]
# Buffer Pool (CRITICAL)
innodb_buffer_pool_size = 8G          # 70-80% RAM
innodb_buffer_pool_instances = 8      # 1-2 per GB

# Redo Logs
innodb_log_file_size = 2G             # 25% buffer pool
innodb_log_buffer_size = 64M

# Flush Method
innodb_flush_method = O_DIRECT         # Avoid double buffering
innodb_flush_log_at_trx_commit = 1     # 1=ACID, 2=performance

# I/O Capacity
innodb_io_capacity = 2000              # For SSD
innodb_io_capacity_max = 4000

# Thread Concurrency
innodb_thread_concurrency = 0          # Auto
innodb_read_io_threads = 4
innodb_write_io_threads = 4

# Table per File
innodb_file_per_table = ON
```

### MYD-017: 2. Query Optimization

#### EXPLAIN Analysis

```sql
EXPLAIN SELECT * FROM users WHERE email = 'user@example.com';
EXPLAIN FORMAT=JSON SELECT ...;

-- Key columns to check:
-- type: Should be 'const', 'eq_ref', 'ref' (avoid 'ALL' = full scan)
-- key: Which index is used
-- rows: Estimated rows scanned
```

#### Index Strategy

```sql
-- Composite index (left-to-right rule)
CREATE INDEX idx_user_email_status ON users(email, status);

-- Covering index (include all columns in SELECT)
CREATE INDEX idx_user_cover ON users(email, status, name);

-- Check index usage
SHOW INDEX FROM users;
SELECT * FROM sys.schema_unused_indexes;
```

#### Query Performance Schema

```sql
-- Enable Performance Schema
SET GLOBAL performance_schema = ON;

-- Top 10 slow queries
SELECT
  DIGEST_TEXT,
  COUNT_STAR,
  AVG_TIMER_WAIT/1000000000 AS avg_ms
FROM performance_schema.events_statements_summary_by_digest
ORDER BY AVG_TIMER_WAIT DESC
LIMIT 10;
```

### MYD-018: 3. Connection Pooling (Application Side)

**Go Example** (database/sql):
```go
db, err := sql.Open("mysql", "user:password@tcp(mysql:3306)/myapp")
db.SetMaxOpenConns(100)        // Max connections
db.SetMaxIdleConns(10)         // Idle pool size
db.SetConnMaxLifetime(time.Hour)
```

**ВАЖНО**: Connection pooling на application side, НЕ в MySQL.

---

## Backup & Recovery

### MYD-019: 1. Backup Strategies

| Method | Type | Downtime | Speed | Use Case |
|--------|------|----------|-------|----------|
| **mysqldump** | Logical | No* | Slow | Small DBs (<100GB) |
| **MySQL Enterprise Backup** | Physical | No | Fast | Large DBs |
| **Percona XtraBackup** | Physical | No | Fast | Large DBs (free) |
| **Snapshots** | Physical | No | Very Fast | Cloud/VM |

### MYD-020: 2. mysqldump (Logical Backup)

```bash
# Full backup
mysqldump -u root -p \
  --single-transaction \
  --routines \
  --triggers \
  --events \
  --all-databases \
  --master-data=2 \
  | gzip > backup_$(date +%F).sql.gz

# Single database
mysqldump -u root -p myapp | gzip > myapp_$(date +%F).sql.gz

# Restore
gunzip < backup_2025-12-30.sql.gz | mysql -u root -p
```

**ВАЖНО**: `--single-transaction` для InnoDB (consistent snapshot).

### MYD-021: 3. Percona XtraBackup (Physical Backup)

```bash
# Install
apt-get install percona-xtrabackup-84

# Full backup
xtrabackup --backup \
  --target-dir=/backups/full \
  --user=root \
  --password=password

# Incremental backup
xtrabackup --backup \
  --target-dir=/backups/inc1 \
  --incremental-basedir=/backups/full \
  --user=root \
  --password=password

# Prepare backup
xtrabackup --prepare --target-dir=/backups/full

# Restore
systemctl stop mysql
rm -rf /var/lib/mysql/*
xtrabackup --copy-back --target-dir=/backups/full
chown -R mysql:mysql /var/lib/mysql
systemctl start mysql
```

### MYD-022: 4. Point-in-Time Recovery (PITR)

```bash
# 1. Restore from last full backup
gunzip < backup_2025-12-30.sql.gz | mysql -u root -p

# 2. Apply binary logs from backup time to target time
mysqlbinlog \
  --start-datetime="2025-12-30 10:00:00" \
  --stop-datetime="2025-12-30 14:30:00" \
  /var/log/mysql/mysql-bin.000001 \
  /var/log/mysql/mysql-bin.000002 \
  | mysql -u root -p
```

### MYD-023: 5. Automated Backup with Docker

```yaml
services:
  mysql-backup:
    image: fradelg/mysql-cron-backup:latest
    depends_on:
      - mysql
    environment:
      MYSQL_HOST: mysql
      MYSQL_USER: root
      MYSQL_PASS_FILE: /run/secrets/mysql_root_password
      MAX_BACKUPS: 7  # Keep last 7 backups
      INIT_BACKUP: 1
      CRON_TIME: "0 2 * * *"  # Daily at 2 AM
    volumes:
      - ./backups:/backup
    secrets:
      - mysql_root_password
    networks:
      - backend
```

---

## Monitoring

### MYD-024: 1. Key Metrics to Monitor

| Metric | Tool | Alert Threshold | Action |
|--------|------|-----------------|--------|
| CPU Usage | Prometheus | >80% | Scale up |
| Memory Usage | Prometheus | >90% | Increase buffer pool |
| Disk I/O | iostat | >80% | Add IOPS |
| Connections | SHOW STATUS | >80% max | Increase max_connections |
| Replication Lag | SHOW SLAVE STATUS | >10s | Check network |
| Slow Queries | slow_query_log | >100/hour | Optimize queries |

### MYD-025: 2. MySQL Exporter (Prometheus)

```yaml
services:
  mysql-exporter:
    image: prom/mysqld-exporter:latest
    container_name: mysql-exporter
    environment:
      DATA_SOURCE_NAME: "exporter:password@(mysql:3306)/"
    ports:
      - "9104:9104"
    networks:
      - monitoring
    depends_on:
      - mysql
```

**Create Exporter User**:
```sql
CREATE USER 'exporter'@'%' IDENTIFIED BY 'password';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'%';
FLUSH PRIVILEGES;
```

### MYD-026: 3. Grafana Dashboard

**Import Dashboard**: ID `7362` (MySQL Overview)

**Key Panels**:
- MySQL Uptime
- Current QPS (Queries Per Second)
- InnoDB Buffer Pool Hit Rate
- Slow Queries
- Connection Usage
- Replication Lag

### MYD-027: 4. Health Check Queries

```sql
-- Connection count
SHOW STATUS LIKE 'Threads_connected';
SHOW STATUS LIKE 'Max_used_connections';

-- Buffer pool efficiency (should be >99%)
SHOW STATUS LIKE 'Innodb_buffer_pool_read_requests';
SHOW STATUS LIKE 'Innodb_buffer_pool_reads';

-- Replication status
SHOW SLAVE STATUS\G

-- Table locks
SHOW STATUS LIKE 'Table_locks_waited';

-- Slow queries
SHOW STATUS LIKE 'Slow_queries';
```

---

## Production Patterns

### MYD-028: 1. Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql
spec:
  serviceName: mysql
  replicas: 1
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      containers:
      - name: mysql
        image: mysql:8.4
        ports:
        - containerPort: 3306
          name: mysql
        env:
        - name: MYSQL_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-secret
              key: root-password
        volumeMounts:
        - name: mysql-data
          mountPath: /var/lib/mysql
        - name: mysql-config
          mountPath: /etc/mysql/conf.d
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
        livenessProbe:
          exec:
            command:
            - mysqladmin
            - ping
            - -h
            - localhost
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          exec:
            command:
            - mysql
            - -h
            - localhost
            - -u
            - root
            - -p${MYSQL_ROOT_PASSWORD}
            - -e
            - SELECT 1
          initialDelaySeconds: 5
          periodSeconds: 5
  volumeClaimTemplates:
  - metadata:
      name: mysql-data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: fast-ssd
      resources:
        requests:
          storage: 100Gi
---
apiVersion: v1
kind: Service
metadata:
  name: mysql
spec:
  type: ClusterIP
  selector:
    app: mysql
  ports:
  - port: 3306
    targetPort: 3306
```

### MYD-029: 2. High Availability Checklist

- [ ] Multi-AZ deployment
- [ ] Automated backups (daily)
- [ ] Replication lag monitoring (<10s)
- [ ] Connection pooling configured
- [ ] SSL/TLS enabled
- [ ] Binary logging enabled
- [ ] Point-in-time recovery tested
- [ ] Failover procedure documented
- [ ] Resource limits set
- [ ] Slow query log enabled

---

## Common Anti-Patterns

| Anti-Pattern | Problem | Solution |
|--------------|---------|----------|
| Using root in apps | Security risk | Create app user with minimal privileges |
| No resource limits | OOM kills | Set memory/CPU limits |
| No SSL/TLS | Data exposure | Enable `require_secure_transport` |
| Storing passwords in plaintext | Credential theft | Use Docker secrets |
| No backups | Data loss | Automated daily backups |
| No replication lag monitoring | Data inconsistency | Monitor `Seconds_Behind_Master` |
| Using MyISAM | No ACID, no transactions | Use InnoDB |
| SELECT * | Network overhead | Select only needed columns |
| No indexes | Slow queries | Add indexes on WHERE/JOIN columns |
| Missing connection pooling | Connection exhaustion | Use connection pools |
| Binding to 0.0.0.0 without firewall | Security risk | Firewall rules or bind to localhost |
| No query cache analysis | Wasted memory | Query cache removed in 8.0+ |

---

## Quick Reference

### MYD-030: Essential Commands

```bash
# Connection
mysql -u root -p -h mysql -P 3306

# Backup
mysqldump -u root -p --all-databases | gzip > backup.sql.gz

# Restore
gunzip < backup.sql.gz | mysql -u root -p

# Check replication
mysql -u root -p -e "SHOW SLAVE STATUS\G"

# Check slow queries
mysql -u root -p -e "SHOW STATUS LIKE 'Slow_queries'"
```

### MYD-031: Performance Tuning Template

```ini
[mysqld]
innodb_buffer_pool_size = 8G
innodb_log_file_size = 2G
innodb_flush_method = O_DIRECT
max_connections = 200
slow_query_log = 1
long_query_time = 2
```

---

## Version Compatibility (2025)

| Version | Release Date | LTS Support Until | Status |
|---------|--------------|-------------------|--------|
| 8.4 LTS | April 2024 | April 2032 | **Recommended** |
| 8.0 LTS | April 2018 | April 2026 | Stable |
| 9.0 | July 2024 | July 2025 | Innovation (testing) |

**Upgrade Path**: 5.7 → 8.0 → 8.4

---

## Дополнительные ресурсы

### MYD-032: Official Documentation
- [MySQL 8.4 Reference Manual](https://dev.mysql.com/doc/refman/8.4/en/)
- [MySQL Performance Blog](https://www.percona.com/blog/)
- [High Performance MySQL (Book)](https://www.oreilly.com/library/view/high-performance-mysql/9781492080503/)

### MYD-033: Tools
- [ProxySQL](https://proxysql.com/) - Query routing & caching
- [Percona XtraBackup](https://www.percona.com/software/mysql-database/percona-xtrabackup) - Hot backups
- [MySQL Tuner](https://github.com/major/MySQLTuner-perl) - Configuration analyzer
- [pt-query-digest](https://www.percona.com/doc/percona-toolkit/LATEST/pt-query-digest.html) - Slow query analysis

### MYD-034: Research Report
См. полный отчет: `.memory_bank/agent_task_results/2025-12-29_web-research_mysql-devops-best-practices.md`

---

**При возникновении вопросов**: Обратись к DevOps engineer agent или см. research report.
