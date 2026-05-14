# PostgreSQL VACUUM and AUTOVACUUM

## Quick Reference

This guide covers database maintenance operations. Related deployment rules:

| ID | Rule | Context |
|----|------|---------|
| PGD-003 | Configure WAL archiving for backup | Deployment |
| PGD-005 | Monitor with pg_stat_statements | Monitoring |

---

## MVCC and Dead Tuples

PostgreSQL uses Multi-Version Concurrency Control (MVCC). When a row is updated or deleted, the old version (dead tuple) remains in the table until cleaned up by VACUUM.

### Why VACUUM is essential

1. **Reclaim disk space** - Dead tuples occupy space
2. **Update statistics** - Planner needs accurate data
3. **Prevent transaction ID wraparound** - Critical for database health
4. **Update visibility map** - Enables index-only scans

## VACUUM Types

### PG-300: Use Regular VACUUM for Maintenance

```sql
-- Basic vacuum (does NOT return space to OS)
VACUUM orders;

-- Vacuum with statistics update
VACUUM ANALYZE orders;

-- Vacuum all tables
VACUUM;

-- Verbose output
VACUUM VERBOSE orders;
```

**Behavior:**
- Marks dead tuples as reusable
- Does NOT lock table for reads/writes
- Does NOT return space to OS
- Updates visibility map

### PG-301: Use VACUUM FULL for Severe Bloat Recovery

```sql
-- Rewrites entire table, returns space to OS
VACUUM FULL orders;

-- With statistics
VACUUM FULL ANALYZE orders;
```

**Behavior:**
- **EXCLUSIVE LOCK** - blocks all operations
- Rewrites table without dead tuples
- Returns space to OS
- Creates new table files
- Use only when significant bloat (>50%)

### PG-302: Use VACUUM FREEZE to Prevent Transaction ID Wraparound

```sql
-- Freeze old transaction IDs
VACUUM FREEZE orders;
```

**Purpose:** Prevents transaction ID wraparound by marking old rows as "frozen" (visible to all transactions).

## AUTOVACUUM Configuration

### PG-303: Query AUTOVACUUM Configuration Parameters

```sql
-- Check current settings
SELECT name, setting, unit, context, short_desc
FROM pg_settings
WHERE name LIKE '%autovacuum%' OR name LIKE '%vacuum%'
ORDER BY name;
```

### PG-304: Configure Global AUTOVACUUM Settings

```ini
# Enable autovacuum (default: on)
autovacuum = on

# Number of autovacuum workers (default: 3)
autovacuum_max_workers = 3

# Time between autovacuum runs (default: 1min)
autovacuum_naptime = 1min

# Thresholds for triggering vacuum
autovacuum_vacuum_threshold = 50          # Min dead tuples
autovacuum_vacuum_scale_factor = 0.2      # % of table
# Trigger when: dead_tuples > threshold + scale_factor * table_rows

# Thresholds for analyze
autovacuum_analyze_threshold = 50
autovacuum_analyze_scale_factor = 0.1

# Cost-based vacuum delay (to reduce I/O impact)
autovacuum_vacuum_cost_delay = 2ms        # Pause between batches
autovacuum_vacuum_cost_limit = 200        # Work before pause

# Freeze settings
autovacuum_freeze_max_age = 200000000     # Force freeze at this age
vacuum_freeze_min_age = 50000000          # Min age to freeze
vacuum_freeze_table_age = 150000000       # Whole-table scan threshold
```

### PG-305: Configure Per-Table AUTOVACUUM Settings

```sql
-- High-update table: more aggressive vacuum
ALTER TABLE orders SET (
    autovacuum_vacuum_scale_factor = 0.05,    -- 5% instead of 20%
    autovacuum_vacuum_threshold = 100,
    autovacuum_analyze_scale_factor = 0.02
);

-- Large table with few updates: less frequent
ALTER TABLE audit_log SET (
    autovacuum_vacuum_scale_factor = 0.5,
    autovacuum_enabled = true
);

-- Append-only table: minimal vacuum needed
ALTER TABLE events SET (
    autovacuum_vacuum_scale_factor = 0.0,
    autovacuum_vacuum_threshold = 10000
);

-- Disable autovacuum (NOT recommended in production)
ALTER TABLE temp_import SET (autovacuum_enabled = false);

-- Reset to defaults
ALTER TABLE orders RESET (autovacuum_vacuum_scale_factor);
```

## Transaction ID Wraparound Prevention

### PG-306: Understand Transaction ID Wraparound Risk

PostgreSQL uses 32-bit transaction IDs (about 4 billion). When IDs wrap around, old transactions would appear "in the future" causing data loss.

### PG-307: Monitor Transaction ID Age

```sql
-- Check transaction ID age for all tables
SELECT
    schemaname,
    relname,
    n_dead_tup,
    n_live_tup,
    age(relfrozenxid) as xid_age,
    pg_size_pretty(pg_total_relation_size(relid)) as size,
    last_autovacuum,
    last_vacuum
FROM pg_stat_user_tables
JOIN pg_class ON pg_stat_user_tables.relid = pg_class.oid
ORDER BY age(relfrozenxid) DESC
LIMIT 20;

-- Check database-level XID age
SELECT
    datname,
    age(datfrozenxid) as db_xid_age,
    current_setting('autovacuum_freeze_max_age')::bigint as max_age
FROM pg_database
ORDER BY age(datfrozenxid) DESC;
```

### PG-308: Alert on Transaction ID Age Thresholds

```sql
-- Alert if any table approaching wraparound
SELECT relname, age(relfrozenxid) as xid_age
FROM pg_class
WHERE relkind = 'r'
AND age(relfrozenxid) > 150000000  -- Approaching max_age
ORDER BY age(relfrozenxid) DESC;
```

### PG-309: Perform Emergency VACUUM FREEZE if Needed

```sql
-- If approaching wraparound, manually freeze
VACUUM FREEZE problematic_table;

-- For entire database
VACUUM FREEZE;
```

## Monitoring VACUUM Activity

### PG-310: Monitor Current VACUUM Progress

```sql
-- Running vacuum processes
SELECT
    pid,
    datname,
    relid::regclass as table_name,
    phase,
    heap_blks_total,
    heap_blks_scanned,
    heap_blks_vacuumed,
    index_vacuum_count,
    max_dead_tuples,
    num_dead_tuples
FROM pg_stat_progress_vacuum;
```

### PG-311: Check VACUUM History and Statistics

```sql
-- Last vacuum times
SELECT
    schemaname,
    relname,
    n_dead_tup,
    n_live_tup,
    round(100.0 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as dead_pct,
    last_vacuum,
    last_autovacuum,
    last_analyze,
    last_autoanalyze,
    vacuum_count,
    autovacuum_count
FROM pg_stat_user_tables
ORDER BY n_dead_tup DESC
LIMIT 20;
```

### PG-312: Estimate Table Bloat Percentage

```sql
-- Estimate table bloat
WITH constants AS (
    SELECT current_setting('block_size')::numeric AS bs,
           23 AS hdr, 4 AS ma
),
table_stats AS (
    SELECT
        schemaname, tablename,
        (SELECT SUM(avg_width) FROM pg_stats
         WHERE schemaname = t.schemaname AND tablename = t.tablename) as avg_row_width,
        reltuples::numeric as row_count,
        relpages::numeric as page_count
    FROM pg_tables t
    JOIN pg_class c ON c.relname = t.tablename
    WHERE schemaname = 'public'
)
SELECT
    schemaname,
    tablename,
    pg_size_pretty(page_count * bs::bigint) as current_size,
    round(100 * (page_count - GREATEST(1,
        ceil(row_count * (avg_row_width + hdr + ma - (CASE WHEN ma = 0 THEN 0 ELSE avg_row_width % ma END)) / (bs - hdr))
    )) / NULLIF(page_count, 0), 2) as bloat_pct
FROM table_stats, constants
WHERE page_count > 0
ORDER BY bloat_pct DESC NULLS LAST
LIMIT 20;
```

## Best Practices

### PG-313: Configure AUTOVACUUM for High-Update Tables

```sql
-- More aggressive vacuum settings
ALTER TABLE orders SET (
    autovacuum_vacuum_scale_factor = 0.02,   -- 2% of rows
    autovacuum_vacuum_threshold = 50,
    autovacuum_analyze_scale_factor = 0.01,
    autovacuum_vacuum_cost_delay = 0         -- No delay
);
```

### PG-314: Configure AUTOVACUUM for Large Tables

```sql
-- Less frequent but thorough vacuum
ALTER TABLE large_table SET (
    autovacuum_vacuum_scale_factor = 0.01,
    autovacuum_vacuum_threshold = 1000,
    fillfactor = 90   -- Leave space for HOT updates
);
```

### PG-315: Configure AUTOVACUUM for Append-Only Tables

```sql
-- Minimal vacuum, mainly for XID
ALTER TABLE event_log SET (
    autovacuum_vacuum_scale_factor = 0,
    autovacuum_vacuum_threshold = 50000,
    autovacuum_analyze_scale_factor = 0,
    autovacuum_analyze_threshold = 50000
);
```

### PG-316: Follow Production AUTOVACUUM Best Practices

1. **Never disable autovacuum globally**
2. **Monitor dead tuple ratios** - Alert if >10% dead tuples
3. **Monitor XID age** - Alert if >50% of autovacuum_freeze_max_age
4. **Increase workers for busy systems:**
   ```ini
   autovacuum_max_workers = 5
   ```
5. **Reduce cost delay for SSDs:**
   ```ini
   autovacuum_vacuum_cost_delay = 0
   autovacuum_vacuum_cost_limit = 1000
   ```

## Troubleshooting

### PG-317: Diagnose AUTOVACUUM Not Running

```sql
-- Check if enabled
SHOW autovacuum;

-- Check if blocked by long transactions
SELECT pid, age(backend_xmin), query
FROM pg_stat_activity
WHERE backend_xmin IS NOT NULL
ORDER BY age(backend_xmin) DESC
LIMIT 5;

-- Long-running transactions prevent vacuum from reclaiming space
```

### PG-318: Optimize Long-Running VACUUM Operations

```sql
-- Check progress
SELECT * FROM pg_stat_progress_vacuum;

-- May need to:
-- 1. Increase maintenance_work_mem
SET maintenance_work_mem = '1GB';

-- 2. Run on specific partitions
VACUUM partitioned_table_2024_01;
```

### PG-319: Reduce I/O Impact During VACUUM

```sql
-- Increase cost delay (spread I/O over time)
ALTER SYSTEM SET autovacuum_vacuum_cost_delay = '5ms';
SELECT pg_reload_conf();

-- Or reduce cost limit
ALTER SYSTEM SET autovacuum_vacuum_cost_limit = 100;
SELECT pg_reload_conf();
```

### PG-320: Handle Bloat After Bulk Delete Operations

```sql
-- Check bloat
SELECT pg_size_pretty(pg_total_relation_size('orders'));

-- Regular VACUUM won't shrink file
VACUUM orders;

-- For significant bloat, schedule VACUUM FULL during maintenance window
-- WARNING: Exclusive lock!
VACUUM FULL orders;

-- Alternative: pg_repack (online, no lock)
-- pg_repack -t orders -d mydb
```

## VACUUM vs pg_repack

### PG-321: Choose Between VACUUM FULL and pg_repack

| Feature | VACUUM FULL | pg_repack |
|---------|-------------|-----------|
| Lock | Exclusive (blocks all) | Minimal (brief locks) |
| Space | Needs 2x table space | Needs 2x table space |
| Indexes | Rebuilds | Rebuilds |
| Online | No | Yes |
| Built-in | Yes | Extension |

```bash
# Using pg_repack
pg_repack -d mydb -t bloated_table
pg_repack -d mydb --only-indexes -t table_name
```
