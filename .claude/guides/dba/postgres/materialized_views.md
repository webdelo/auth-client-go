# PostgreSQL Materialized Views

## Quick Reference

This guide covers materialized views for query optimization.

| ID | Rule | Context |
|----|------|---------|
| PG-006 | Use prepared statements for repeated queries | Refresh operations |
| PGD-005 | Monitor with pg_stat_statements | Performance tracking |

---

## Overview

Materialized views store query results physically, unlike regular views which execute on each access.

### Regular View vs Materialized View

```sql
-- Regular view: executes query each time
CREATE VIEW active_users AS
SELECT * FROM users WHERE status = 'active';

-- Materialized view: stores result physically
CREATE MATERIALIZED VIEW active_users_mv AS
SELECT * FROM users WHERE status = 'active';
```

| Feature | Regular View | Materialized View |
|---------|--------------|-------------------|
| Storage | None (query only) | Physical table |
| Read speed | Depends on query | Fast (pre-computed) |
| Data freshness | Always current | Snapshot (stale) |
| Indexes | No | Yes |
| Refresh needed | No | Yes |

## Creating Materialized Views

### PG-500: Create Materialized Views with Data

```sql
CREATE MATERIALIZED VIEW sales_summary AS
SELECT
    date_trunc('month', order_date) AS month,
    product_category,
    COUNT(*) AS order_count,
    SUM(total) AS total_revenue,
    AVG(total) AS avg_order_value
FROM orders
JOIN products ON orders.product_id = products.id
WHERE order_date >= '2024-01-01'
GROUP BY 1, 2
WITH DATA;  -- Populate immediately (default)

-- Create without initial data
CREATE MATERIALIZED VIEW sales_summary AS
SELECT ...
WITH NO DATA;  -- Empty until first REFRESH
```

### PG-501: Add Indexes to Materialized Views

```sql
-- Create materialized view
CREATE MATERIALIZED VIEW product_stats AS
SELECT
    product_id,
    COUNT(*) AS view_count,
    COUNT(DISTINCT user_id) AS unique_viewers,
    MAX(viewed_at) AS last_viewed
FROM product_views
GROUP BY product_id;

-- Add indexes for query performance
CREATE INDEX idx_product_stats_product ON product_stats(product_id);
CREATE INDEX idx_product_stats_views ON product_stats(view_count DESC);
CREATE UNIQUE INDEX idx_product_stats_unique ON product_stats(product_id);  -- Required for CONCURRENTLY refresh
```

## Refreshing Materialized Views

### PG-502: Use Standard REFRESH for Non-Blocking Refresh

```sql
-- Full refresh (locks view during refresh)
REFRESH MATERIALIZED VIEW sales_summary;

-- Returns immediately, view unavailable during refresh
```

**Behavior:**
- Exclusive lock on view
- View cannot be queried during refresh
- Replaces all data

### PG-503: Use CONCURRENTLY Refresh in Production

```sql
-- Refresh without blocking reads (requires UNIQUE INDEX)
REFRESH MATERIALIZED VIEW CONCURRENTLY sales_summary;
```

**Requirements:**
- Must have UNIQUE INDEX on view
- Takes longer (computes diff)
- View remains queryable during refresh

**Performance comparison:**

| Refresh Type | Lock | Queryable | Speed | Use Case |
|--------------|------|-----------|-------|----------|
| Standard | Exclusive | No | Fast | Off-peak hours |
| CONCURRENTLY | Share | Yes | Slower | Production uptime |

### PG-504: Schedule Regular Materialized View Refreshes

PostgreSQL doesn't have built-in scheduler. Use external tools:

**pg_cron (PostgreSQL extension):**
```sql
-- Install extension
CREATE EXTENSION pg_cron;

-- Refresh every hour
SELECT cron.schedule('refresh-sales', '0 * * * *',
    'REFRESH MATERIALIZED VIEW CONCURRENTLY sales_summary');

-- Refresh daily at 3 AM
SELECT cron.schedule('refresh-daily-stats', '0 3 * * *',
    'REFRESH MATERIALIZED VIEW stats_daily');

-- Check scheduled jobs
SELECT * FROM cron.job;

-- Remove job
SELECT cron.unschedule('refresh-sales');
```

**External cron:**
```bash
# crontab entry
0 * * * * psql -d mydb -c "REFRESH MATERIALIZED VIEW CONCURRENTLY sales_summary"
```

## Incremental Refresh (Manual)

### PG-505: Implement Manual Incremental Refresh

PostgreSQL doesn't support automatic incremental refresh. Implement manually:

### PG-506: Track Changes with Triggers for Incremental Refresh

```sql
-- Base table
CREATE TABLE events (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(50),
    user_id INT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Aggregated materialized view
CREATE MATERIALIZED VIEW event_counts AS
SELECT
    event_type,
    COUNT(*) AS count,
    COUNT(DISTINCT user_id) AS unique_users
FROM events
GROUP BY event_type;

-- Incremental update table
CREATE TABLE event_counts_delta (
    event_type VARCHAR(50) PRIMARY KEY,
    count_delta BIGINT DEFAULT 0,
    unique_users_delta BIGINT DEFAULT 0
);

-- Trigger function to track changes
CREATE OR REPLACE FUNCTION track_event_insert()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO event_counts_delta (event_type, count_delta)
    VALUES (NEW.event_type, 1)
    ON CONFLICT (event_type) DO UPDATE
    SET count_delta = event_counts_delta.count_delta + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER event_insert_trigger
AFTER INSERT ON events
FOR EACH ROW EXECUTE FUNCTION track_event_insert();
```

### PG-507: Create Incremental Refresh Functions

```sql
CREATE OR REPLACE FUNCTION refresh_event_counts_incremental()
RETURNS void AS $$
DECLARE
    last_refresh TIMESTAMPTZ;
BEGIN
    -- Get last refresh time
    SELECT last_refresh_time INTO last_refresh
    FROM mv_refresh_log
    WHERE mv_name = 'event_counts'
    FOR UPDATE;

    -- Insert new aggregates
    INSERT INTO event_counts_table
    SELECT
        event_type,
        COUNT(*),
        COUNT(DISTINCT user_id)
    FROM events
    WHERE created_at > last_refresh
    GROUP BY event_type
    ON CONFLICT (event_type) DO UPDATE
    SET
        count = event_counts_table.count + EXCLUDED.count;

    -- Update last refresh time
    UPDATE mv_refresh_log
    SET last_refresh_time = NOW()
    WHERE mv_name = 'event_counts';
END;
$$ LANGUAGE plpgsql;
```

## Patterns for Different Use Cases

### PG-508: Use Materialized Views for Dashboard Aggregations

```sql
-- Daily metrics dashboard
CREATE MATERIALIZED VIEW dashboard_daily AS
SELECT
    DATE(created_at) AS date,
    COUNT(*) AS total_orders,
    SUM(total) AS revenue,
    COUNT(DISTINCT customer_id) AS unique_customers,
    AVG(total) AS avg_order_value,
    COUNT(*) FILTER (WHERE status = 'completed') AS completed_orders,
    COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled_orders
FROM orders
WHERE created_at >= CURRENT_DATE - INTERVAL '90 days'
GROUP BY DATE(created_at);

CREATE UNIQUE INDEX idx_dashboard_daily_date ON dashboard_daily(date);

-- Refresh strategy: every 15 minutes CONCURRENTLY
```

### PG-509: Use Materialized Views for Search/Lookup Optimization

```sql
-- Denormalized view for fast lookups
CREATE MATERIALIZED VIEW product_search AS
SELECT
    p.id,
    p.name,
    p.description,
    p.price,
    c.name AS category_name,
    b.name AS brand_name,
    array_agg(DISTINCT t.name) AS tags,
    to_tsvector('english', p.name || ' ' || p.description) AS search_vector,
    p.updated_at
FROM products p
LEFT JOIN categories c ON p.category_id = c.id
LEFT JOIN brands b ON p.brand_id = b.id
LEFT JOIN product_tags pt ON p.id = pt.product_id
LEFT JOIN tags t ON pt.tag_id = t.id
GROUP BY p.id, p.name, p.description, p.price, c.name, b.name, p.updated_at;

CREATE UNIQUE INDEX idx_product_search_id ON product_search(id);
CREATE INDEX idx_product_search_fts ON product_search USING GIN(search_vector);
CREATE INDEX idx_product_search_category ON product_search(category_name);
```

### PG-510: Pre-compute Expensive Joins in Materialized Views

```sql
-- Pre-join for reporting
CREATE MATERIALIZED VIEW order_details_flat AS
SELECT
    o.id AS order_id,
    o.order_number,
    o.created_at AS order_date,
    o.total AS order_total,
    c.id AS customer_id,
    c.email AS customer_email,
    c.name AS customer_name,
    oi.product_id,
    oi.quantity,
    oi.unit_price,
    p.name AS product_name,
    p.sku AS product_sku,
    cat.name AS category_name
FROM orders o
JOIN customers c ON o.customer_id = c.id
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
LEFT JOIN categories cat ON p.category_id = cat.id;

CREATE INDEX idx_order_details_order ON order_details_flat(order_id);
CREATE INDEX idx_order_details_customer ON order_details_flat(customer_id);
CREATE INDEX idx_order_details_date ON order_details_flat(order_date);
```

## Monitoring and Maintenance

### PG-511: Monitor Materialized View Status and Size

```sql
-- Check materialized views
SELECT
    schemaname,
    matviewname,
    matviewowner,
    ispopulated,
    pg_size_pretty(pg_relation_size(matviewname::regclass)) AS size
FROM pg_matviews
WHERE schemaname = 'public';
```

### PG-512: Log Materialized View Refresh Duration

```sql
-- Create refresh log table
CREATE TABLE mv_refresh_log (
    id SERIAL PRIMARY KEY,
    mv_name VARCHAR(100),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    duration_ms BIGINT,
    row_count BIGINT
);

-- Refresh with logging
CREATE OR REPLACE FUNCTION refresh_mv_with_log(mv_name TEXT, concurrent BOOLEAN DEFAULT TRUE)
RETURNS void AS $$
DECLARE
    start_time TIMESTAMPTZ;
    end_time TIMESTAMPTZ;
    rows BIGINT;
BEGIN
    start_time := clock_timestamp();

    IF concurrent THEN
        EXECUTE format('REFRESH MATERIALIZED VIEW CONCURRENTLY %I', mv_name);
    ELSE
        EXECUTE format('REFRESH MATERIALIZED VIEW %I', mv_name);
    END IF;

    end_time := clock_timestamp();

    EXECUTE format('SELECT COUNT(*) FROM %I', mv_name) INTO rows;

    INSERT INTO mv_refresh_log (mv_name, started_at, finished_at, duration_ms, row_count)
    VALUES (mv_name, start_time, end_time,
            EXTRACT(EPOCH FROM (end_time - start_time)) * 1000, rows);
END;
$$ LANGUAGE plpgsql;

-- Usage
SELECT refresh_mv_with_log('sales_summary', true);
```

### PG-513: Monitor Materialized View Staleness

```sql
-- Check when views were last refreshed
SELECT
    mv.matviewname,
    COALESCE(log.finished_at, 'Never'::text) AS last_refresh,
    COALESCE(
        EXTRACT(EPOCH FROM (NOW() - log.finished_at)) / 60,
        -1
    )::int AS minutes_since_refresh,
    log.duration_ms,
    log.row_count
FROM pg_matviews mv
LEFT JOIN LATERAL (
    SELECT * FROM mv_refresh_log
    WHERE mv_name = mv.matviewname
    ORDER BY finished_at DESC
    LIMIT 1
) log ON true
WHERE mv.schemaname = 'public';
```

## Go Integration

### PG-514: Manage Materialized View Refresh from Go

```go
type MVRefresher struct {
    db     *sqlx.DB
    logger *zap.Logger
}

func (r *MVRefresher) RefreshConcurrently(ctx context.Context, viewName string) error {
    query := fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s",
        pq.QuoteIdentifier(viewName))

    start := time.Now()
    _, err := r.db.ExecContext(ctx, query)
    duration := time.Since(start)

    if err != nil {
        r.logger.Error("failed to refresh materialized view",
            zap.String("view", viewName),
            zap.Error(err))
        return fmt.Errorf("refresh %s: %w", viewName, err)
    }

    r.logger.Info("refreshed materialized view",
        zap.String("view", viewName),
        zap.Duration("duration", duration))

    return nil
}

func (r *MVRefresher) RefreshWithTimeout(ctx context.Context, viewName string, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    return r.RefreshConcurrently(ctx, viewName)
}
```

### PG-515: Schedule Background Materialized View Refreshes

```go
type RefreshScheduler struct {
    refresher *MVRefresher
    schedules map[string]time.Duration
    stop      chan struct{}
}

func NewRefreshScheduler(refresher *MVRefresher) *RefreshScheduler {
    return &RefreshScheduler{
        refresher: refresher,
        schedules: make(map[string]time.Duration),
        stop:      make(chan struct{}),
    }
}

func (s *RefreshScheduler) AddView(viewName string, interval time.Duration) {
    s.schedules[viewName] = interval
}

func (s *RefreshScheduler) Start(ctx context.Context) {
    for viewName, interval := range s.schedules {
        go s.runSchedule(ctx, viewName, interval)
    }
}

func (s *RefreshScheduler) runSchedule(ctx context.Context, viewName string, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := s.refresher.RefreshWithTimeout(ctx, viewName, interval/2); err != nil {
                // Log app_error, continue
            }
        case <-s.stop:
            return
        case <-ctx.Done():
            return
        }
    }
}

func (s *RefreshScheduler) Stop() {
    close(s.stop)
}
```

### PG-516: Implement Fallback Queries for Stale Views

```go
func (r *Repository) GetSalesSummary(ctx context.Context, month time.Time) (*SalesSummary, error) {
    // Try materialized view first (fast)
    var summary SalesSummary
    err := r.db.GetContext(ctx, &summary, `
        SELECT month, order_count, total_revenue, avg_order_value
        FROM sales_summary_mv
        WHERE month = $1
    `, month)

    if err == nil {
        return &summary, nil
    }

    // Fallback to base query (slow but fresh)
    if err == sql.ErrNoRows {
        return r.calculateSalesSummary(ctx, month)
    }

    return nil, err
}
```

## Best Practices

### PG-517: Follow Materialized View Best Practices

1. **Always add UNIQUE INDEX** for CONCURRENTLY refresh
2. **Monitor refresh duration** - alert if exceeding threshold
3. **Schedule refreshes during low-traffic periods** when possible
4. **Use CONCURRENTLY in production** to avoid blocking
5. **Consider data freshness requirements** when setting refresh intervals
6. **Index materialized views** appropriately for query patterns
7. **Track staleness** and expose metrics

## Limitations

### PG-518: Understand Materialized View Limitations

- No automatic incremental refresh
- CONCURRENTLY requires unique index
- No dependencies tracking between MVs
- No built-in scheduler (use pg_cron)
- Cannot reference temporary tables
- Cannot have rules or triggers
