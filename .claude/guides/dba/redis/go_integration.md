# Redis + Go Integration

## Quick Reference

| ID | Rule | Section |
|----|------|---------|
| RD-001 | Use Pipeline for batch operations | [Pipelining](#pipelining) |
| RD-002 | Use Lua scripts for atomic operations | [Lua Scripts](#lua-scripts) |
| RD-003 | Set TTL for cache keys | [Basic Operations](#basic-operations) |
| RD-004 | Use SCAN instead of KEYS in production | [Best Practices](#best-practices) |
| RD-005 | Handle redis.Nil for missing keys | [Error Handling](#error-handling) |
| RD-006 | Use Streams for reliable messaging | [Streams](#streams) |
| RD-007 | Prefix keys with namespace | [Best Practices](#best-practices) |

---

## Client Selection

### go-redis (recommended)
```go
import "github.com/redis/go-redis/v9"
```
- Full Redis API support
- Clustering, Sentinel
- Pipelining, transactions
- Connection pooling
- Context support

### redigo (alternative)
```go
import "github.com/gomodule/redigo/redis"
```
- Simpler API
- Lower-level control

## Connection Setup

### Single instance
```go
package redis

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
)

type Config struct {
    Addr         string
    Password     string
    DB           int
    PoolSize     int
    MinIdleConns int
    DialTimeout  time.Duration
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
}

func NewClient(cfg Config) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         cfg.Addr,
        Password:     cfg.Password,
        DB:           cfg.DB,
        PoolSize:     cfg.PoolSize,
        MinIdleConns: cfg.MinIdleConns,
        DialTimeout:  cfg.DialTimeout,
        ReadTimeout:  cfg.ReadTimeout,
        WriteTimeout: cfg.WriteTimeout,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("ping redis: %w", err)
    }

    return client, nil
}
```

### Cluster
```go
func NewClusterClient(addrs []string) (*redis.ClusterClient, error) {
    client := redis.NewClusterClient(&redis.ClusterOptions{
        Addrs:        addrs,
        PoolSize:     10,
        MinIdleConns: 5,
    })

    ctx := context.Background()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, err
    }

    return client, nil
}
```

### Sentinel
```go
func NewSentinelClient(masterName string, sentinelAddrs []string) (*redis.Client, error) {
    client := redis.NewFailoverClient(&redis.FailoverOptions{
        MasterName:    masterName,
        SentinelAddrs: sentinelAddrs,
        PoolSize:      10,
    })

    if err := client.Ping(context.Background()).Err(); err != nil {
        return nil, err
    }

    return client, nil
}
```

## Basic Operations

### RD-003: Strings
```go
func (r *Repository) SetValue(ctx context.Context, key, value string, ttl time.Duration) error {
    return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *Repository) GetValue(ctx context.Context, key string) (string, error) {
    val, err := r.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return "", ErrNotFound
    }
    return val, err
}

func (r *Repository) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
    return r.client.SetNX(ctx, key, value, ttl).Result()
}

func (r *Repository) Increment(ctx context.Context, key string) (int64, error) {
    return r.client.Incr(ctx, key).Result()
}
```

### RD-008: Hashes
```go
type User struct {
    ID    string `redis:"id"`
    Name  string `redis:"name"`
    Email string `redis:"email"`
}

func (r *Repository) SetUser(ctx context.Context, user *User) error {
    key := fmt.Sprintf("user:%s", user.ID)
    return r.client.HSet(ctx, key, map[string]interface{}{
        "id":    user.ID,
        "name":  user.Name,
        "email": user.Email,
    }).Err()
}

func (r *Repository) GetUser(ctx context.Context, id string) (*User, error) {
    key := fmt.Sprintf("user:%s", id)

    var user User
    err := r.client.HGetAll(ctx, key).Scan(&user)
    if err != nil {
        return nil, err
    }

    if user.ID == "" {
        return nil, ErrNotFound
    }

    return &user, nil
}

func (r *Repository) IncrementLoginCount(ctx context.Context, userID string) (int64, error) {
    key := fmt.Sprintf("user:%s", userID)
    return r.client.HIncrBy(ctx, key, "login_count", 1).Result()
}
```

### RD-009: Lists
```go
func (r *Repository) PushToQueue(ctx context.Context, queue string, items ...interface{}) error {
    return r.client.RPush(ctx, queue, items...).Err()
}

func (r *Repository) PopFromQueue(ctx context.Context, queue string) (string, error) {
    val, err := r.client.LPop(ctx, queue).Result()
    if err == redis.Nil {
        return "", ErrQueueEmpty
    }
    return val, err
}

func (r *Repository) BlockingPop(ctx context.Context, queue string, timeout time.Duration) (string, error) {
    result, err := r.client.BLPop(ctx, timeout, queue).Result()
    if err == redis.Nil {
        return "", ErrTimeout
    }
    if err != nil {
        return "", err
    }
    return result[1], nil // result[0] is queue name
}
```

### RD-010: Sets
```go
func (r *Repository) AddTags(ctx context.Context, itemID string, tags ...string) error {
    key := fmt.Sprintf("item:%s:tags", itemID)
    members := make([]interface{}, len(tags))
    for i, t := range tags {
        members[i] = t
    }
    return r.client.SAdd(ctx, key, members...).Err()
}

func (r *Repository) GetTags(ctx context.Context, itemID string) ([]string, error) {
    key := fmt.Sprintf("item:%s:tags", itemID)
    return r.client.SMembers(ctx, key).Result()
}

func (r *Repository) HasTag(ctx context.Context, itemID, tag string) (bool, error) {
    key := fmt.Sprintf("item:%s:tags", itemID)
    return r.client.SIsMember(ctx, key, tag).Result()
}
```

### RD-011: Sorted Sets
```go
func (r *Repository) UpdateLeaderboard(ctx context.Context, userID string, score float64) error {
    return r.client.ZAdd(ctx, "leaderboard", redis.Z{
        Score:  score,
        Member: userID,
    }).Err()
}

func (r *Repository) IncrementScore(ctx context.Context, userID string, delta float64) (float64, error) {
    return r.client.ZIncrBy(ctx, "leaderboard", delta, userID).Result()
}

func (r *Repository) GetTopPlayers(ctx context.Context, count int64) ([]redis.Z, error) {
    return r.client.ZRevRangeWithScores(ctx, "leaderboard", 0, count-1).Result()
}

func (r *Repository) GetRank(ctx context.Context, userID string) (int64, error) {
    rank, err := r.client.ZRevRank(ctx, "leaderboard", userID).Result()
    if err == redis.Nil {
        return -1, ErrNotFound
    }
    return rank, err
}
```

## Streams

### RD-006: Producer
```go
func (r *Repository) PublishEvent(ctx context.Context, event Event) (string, error) {
    return r.client.XAdd(ctx, &redis.XAddArgs{
        Stream: "events",
        MaxLen: 10000,
        Approx: true,
        Values: map[string]interface{}{
            "type":    event.Type,
            "user_id": event.UserID,
            "data":    event.Data,
        },
    }).Result()
}
```

### RD-012: Consumer (simple)
```go
func (r *Repository) ReadEvents(ctx context.Context, lastID string, count int64) ([]redis.XMessage, error) {
    result, err := r.client.XRead(ctx, &redis.XReadArgs{
        Streams: []string{"events", lastID},
        Count:   count,
        Block:   5 * time.Second,
    }).Result()

    if err == redis.Nil {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    if len(result) == 0 {
        return nil, nil
    }

    return result[0].Messages, nil
}
```

### RD-013: Consumer Group
```go
func (r *Repository) CreateConsumerGroup(ctx context.Context, stream, group string) error {
    err := r.client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
    if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
        return err
    }
    return nil
}

func (r *Repository) ReadGroupMessages(ctx context.Context, stream, group, consumer string) ([]redis.XMessage, error) {
    result, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    group,
        Consumer: consumer,
        Streams:  []string{stream, ">"},
        Count:    10,
        Block:    5 * time.Second,
    }).Result()

    if err == redis.Nil {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }

    if len(result) == 0 {
        return nil, nil
    }

    return result[0].Messages, nil
}

func (r *Repository) AckMessage(ctx context.Context, stream, group string, ids ...string) error {
    return r.client.XAck(ctx, stream, group, ids...).Err()
}
```

## Pipelining

### RD-001: Basic pipeline
```go
func (r *Repository) GetMultipleUsers(ctx context.Context, ids []string) ([]*User, error) {
    pipe := r.client.Pipeline()

    cmds := make([]*redis.MapStringStringCmd, len(ids))
    for i, id := range ids {
        key := fmt.Sprintf("user:%s", id)
        cmds[i] = pipe.HGetAll(ctx, key)
    }

    _, err := pipe.Exec(ctx)
    if err != nil && err != redis.Nil {
        return nil, err
    }

    users := make([]*User, len(ids))
    for i, cmd := range cmds {
        var user User
        if err := cmd.Scan(&user); err != nil {
            continue
        }
        if user.ID != "" {
            users[i] = &user
        }
    }

    return users, nil
}
```

### RD-014: Transaction pipeline
```go
func (r *Repository) Transfer(ctx context.Context, from, to string, amount int64) error {
    // Use TxPipeline for MULTI/EXEC
    pipe := r.client.TxPipeline()

    fromKey := fmt.Sprintf("balance:%s", from)
    toKey := fmt.Sprintf("balance:%s", to)

    pipe.DecrBy(ctx, fromKey, amount)
    pipe.IncrBy(ctx, toKey, amount)

    _, err := pipe.Exec(ctx)
    return err
}
```

## Lua Scripts

### RD-002: Define and execute script
```go
var rateLimitScript = redis.NewScript(`
    local current = redis.call('GET', KEYS[1])
    if current and tonumber(current) >= tonumber(ARGV[1]) then
        return 0
    end
    redis.call('INCR', KEYS[1])
    redis.call('EXPIRE', KEYS[1], ARGV[2])
    return 1
`)

func (r *Repository) CheckRateLimit(ctx context.Context, key string, limit int, window int) (bool, error) {
    result, err := rateLimitScript.Run(ctx, r.client, []string{key}, limit, window).Int()
    if err != nil {
        return false, err
    }
    return result == 1, nil
}
```

### RD-015: Distributed lock with Lua
```go
var releaseLockScript = redis.NewScript(`
    if redis.call('GET', KEYS[1]) == ARGV[1] then
        return redis.call('DEL', KEYS[1])
    else
        return 0
    end
`)

type Lock struct {
    client *redis.Client
    key    string
    value  string
}

func (r *Repository) AcquireLock(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
    value := uuid.New().String()
    ok, err := r.client.SetNX(ctx, key, value, ttl).Result()
    if err != nil {
        return nil, err
    }
    if !ok {
        return nil, ErrLockNotAcquired
    }
    return &Lock{client: r.client, key: key, value: value}, nil
}

func (l *Lock) Release(ctx context.Context) error {
    result, err := releaseLockScript.Run(ctx, l.client, []string{l.key}, l.value).Int()
    if err != nil {
        return err
    }
    if result == 0 {
        return ErrLockNotOwned
    }
    return nil
}
```

## Pub/Sub

### RD-016: Publisher
```go
func (r *Repository) Publish(ctx context.Context, channel string, message interface{}) error {
    data, err := json.Marshal(message)
    if err != nil {
        return err
    }
    return r.client.Publish(ctx, channel, data).Err()
}
```

### RD-017: Subscriber
```go
func (r *Repository) Subscribe(ctx context.Context, channels ...string) (<-chan *redis.Message, func()) {
    pubsub := r.client.Subscribe(ctx, channels...)
    ch := pubsub.Channel()

    cleanup := func() {
        pubsub.Close()
    }

    return ch, cleanup
}

// Usage
func processMessages(ctx context.Context, r *Repository) {
    ch, cleanup := r.Subscribe(ctx, "events", "notifications")
    defer cleanup()

    for {
        select {
        case msg := <-ch:
            fmt.Printf("Channel: %s, Message: %s\n", msg.Channel, msg.Payload)
        case <-ctx.Done():
            return
        }
    }
}
```

## Caching Patterns

### RD-018: Cache-aside
```go
func (r *Repository) GetUserCached(ctx context.Context, id string) (*User, error) {
    // Try cache first
    cacheKey := fmt.Sprintf("cache:user:%s", id)
    data, err := r.client.Get(ctx, cacheKey).Bytes()
    if err == nil {
        var user User
        if err := json.Unmarshal(data, &user); err == nil {
            return &user, nil
        }
    }

    // Cache miss - get from DB
    user, err := r.db.GetUser(ctx, id)
    if err != nil {
        return nil, err
    }

    // Store in cache
    data, _ = json.Marshal(user)
    r.client.Set(ctx, cacheKey, data, 5*time.Minute)

    return user, nil
}
```

### RD-019: Cache invalidation
```go
func (r *Repository) UpdateUser(ctx context.Context, user *User) error {
    // Update DB
    if err := r.db.UpdateUser(ctx, user); err != nil {
        return err
    }

    // Invalidate cache
    cacheKey := fmt.Sprintf("cache:user:%s", user.ID)
    return r.client.Del(ctx, cacheKey).Err()
}
```

## Error Handling

```go
func handleRedisError(err error) error {
    if err == nil {
        return nil
    }

    if err == redis.Nil {
        return ErrNotFound
    }

    if errors.Is(err, context.DeadlineExceeded) {
        return ErrTimeout
    }

    if errors.Is(err, context.Canceled) {
        return ErrCanceled
    }

    // Check for specific Redis errors
    var redisErr redis.Error
    if errors.As(err, &redisErr) {
        if strings.HasPrefix(string(redisErr), "READONLY") {
            return ErrReadOnly
        }
        if strings.HasPrefix(string(redisErr), "CLUSTERDOWN") {
            return ErrClusterDown
        }
    }

    return fmt.Errorf("redis app_error: %w", err)
}
```

## Testing

### Mock with miniredis
```go
import "github.com/alicebob/miniredis/v2"

func setupTestRedis(t *testing.T) *redis.Client {
    t.Helper()

    mr := miniredis.RunT(t)

    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })

    t.Cleanup(func() {
        client.Close()
        mr.Close()
    })

    return client
}

func TestRepository_SetUser(t *testing.T) {
    client := setupTestRedis(t)
    repo := NewRepository(client)

    user := &User{ID: "123", Name: "John"}
    err := repo.SetUser(context.Background(), user)
    require.NoError(t, err)

    got, err := repo.GetUser(context.Background(), "123")
    require.NoError(t, err)
    assert.Equal(t, user.Name, got.Name)
}
```

## Best Practices

### RD-020: Connection pool sizing
```go
// Rule of thumb:
// PoolSize = number of concurrent operations expected
// For web servers: PoolSize = MaxConcurrentRequests / 10

opts := &redis.Options{
    PoolSize:     100,
    MinIdleConns: 10,
    PoolTimeout:  4 * time.Second,
}
```

### RD-007: Key naming conventions
```go
const (
    // Pattern: service:entity:id:attribute
    UserKey       = "myapp:user:%s"
    UserCacheKey  = "myapp:cache:user:%s"
    SessionKey    = "myapp:session:%s"
    RateLimitKey  = "myapp:rate:%s:%s"
    LockKey       = "myapp:lock:%s"
)
```

### RD-021: Context timeouts
```go
func (r *Repository) GetWithTimeout(ctx context.Context, key string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
    defer cancel()

    return r.client.Get(ctx, key).Result()
}
```
