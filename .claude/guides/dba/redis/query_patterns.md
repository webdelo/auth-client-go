# Redis Command Patterns

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| RD-001 | Use Pipeline for batch operations | [Pipelining](#pipelining) |
| RD-002 | Use Lua scripts for atomic operations | [Lua Scripts](#lua-scripts) |
| RD-003 | Set TTL for cache keys | [Strings](#strings) |
| RD-004 | Use SCAN instead of KEYS in production | [Key Operations](#key-operations) |
| RD-007 | Prefix keys with namespace | [Key Naming](#key-naming) |

---

## Data Structures

### RD-103: Strings
```redis
# Basic operations
SET key "value"
SET key "value" EX 3600          # With expiration (seconds)
SET key "value" PX 3600000       # With expiration (milliseconds)
SET key "value" NX               # Only if not exists
SET key "value" XX               # Only if exists
SETNX key "value"                # Set if not exists (deprecated, use SET NX)

GET key
MGET key1 key2 key3              # Multiple get
MSET key1 "v1" key2 "v2"         # Multiple set

# Counters
INCR counter
INCRBY counter 5
INCRBYFLOAT counter 1.5
DECR counter
DECRBY counter 5

# String manipulation
APPEND key "suffix"
STRLEN key
GETRANGE key 0 10                # Substring
SETRANGE key 10 "new"            # Replace at offset
```

### RD-104: Hashes
```redis
# Basic operations
HSET user:123 name "John" email "john@example.com"
HGET user:123 name
HMGET user:123 name email
HGETALL user:123
HDEL user:123 email

# Check existence
HEXISTS user:123 name
HLEN user:123
HKEYS user:123
HVALS user:123

# Counters in hash
HINCRBY user:123 login_count 1
HINCRBYFLOAT user:123 balance 10.50

# Scan hash fields
HSCAN user:123 0 MATCH "field*" COUNT 100
```

### RD-105: Lists
```redis
# Push operations
LPUSH queue "item"               # Push to head
RPUSH queue "item"               # Push to tail
LPUSHX queue "item"              # Push only if list exists

# Pop operations
LPOP queue                       # Pop from head
RPOP queue                       # Pop from tail
BLPOP queue 30                   # Blocking pop (30s timeout)
BRPOP queue 30
LMPOP 2 queue1 queue2 LEFT       # Pop from multiple queues

# Access elements
LINDEX queue 0                   # Get by index
LRANGE queue 0 -1                # Get range (all)
LRANGE queue 0 9                 # First 10 elements
LLEN queue                       # Length

# Modify
LSET queue 0 "new_value"         # Set by index
LINSERT queue BEFORE "pivot" "new"
LREM queue 0 "value"             # Remove all occurrences
LTRIM queue 0 99                 # Keep only first 100
```

### RD-106: Sets
```redis
# Basic operations
SADD tags "go" "redis" "database"
SREM tags "database"
SISMEMBER tags "go"              # Check membership
SMEMBERS tags                    # Get all members
SCARD tags                       # Count

# Random
SRANDMEMBER tags 3               # 3 random members
SPOP tags                        # Pop random member

# Set operations
SINTER set1 set2                 # Intersection
SUNION set1 set2                 # Union
SDIFF set1 set2                  # Difference
SINTERSTORE result set1 set2     # Store intersection
```

### RD-107: Sorted Sets
```redis
# Basic operations
ZADD leaderboard 100 "player1" 200 "player2"
ZADD leaderboard NX 150 "player3"    # Only if not exists
ZADD leaderboard XX 250 "player1"    # Only if exists
ZADD leaderboard GT 300 "player1"    # Only if > current score
ZINCRBY leaderboard 10 "player1"     # Increment score

# Get by rank
ZRANGE leaderboard 0 9               # Top 10 (ascending)
ZREVRANGE leaderboard 0 9            # Top 10 (descending)
ZRANGE leaderboard 0 9 WITHSCORES    # With scores

# Get by score
ZRANGEBYSCORE leaderboard 100 200
ZRANGEBYSCORE leaderboard (100 200   # Exclusive lower bound
ZRANGEBYSCORE leaderboard -inf +inf  # All

# Info
ZSCORE leaderboard "player1"         # Get score
ZRANK leaderboard "player1"          # Get rank (0-based)
ZREVRANK leaderboard "player1"       # Reverse rank
ZCARD leaderboard                    # Count
ZCOUNT leaderboard 100 200           # Count in score range

# Remove
ZREM leaderboard "player1"
ZREMRANGEBYRANK leaderboard 0 9      # Remove by rank
ZREMRANGEBYSCORE leaderboard 0 100   # Remove by score
```

### RD-108: Streams
```redis
# Add entries
XADD events * user_id 123 action "click"
XADD events MAXLEN ~1000 * user_id 123 action "view"  # With approximate cap

# Read entries
XRANGE events - +                    # All entries
XRANGE events - + COUNT 10           # First 10
XREVRANGE events + -                 # Reverse order
XREAD COUNT 10 STREAMS events 0      # Read from beginning

# Consumer groups
XGROUP CREATE events mygroup 0 MKSTREAM
XREADGROUP GROUP mygroup consumer1 COUNT 10 STREAMS events >
XACK events mygroup 1234567890-0     # Acknowledge
XPENDING events mygroup              # Check pending

# Info
XLEN events
XINFO STREAM events
XINFO GROUPS events
```

## Common Patterns

### RD-109: Distributed Lock
```redis
# Acquire lock
SET lock:resource "owner_id" NX EX 30

# Release lock (use Lua for atomicity)
# See Lua script below
```

### RD-110: Rate Limiting (Sliding Window)
```redis
# Add request timestamp
ZADD rate:user:123 <timestamp> <request_id>

# Remove old entries
ZREMRANGEBYSCORE rate:user:123 0 <timestamp - window>

# Count requests in window
ZCARD rate:user:123
```

### RD-111: Rate Limiting (Fixed Window)
```redis
# Increment counter with expiration
INCR rate:user:123:<minute>
EXPIRE rate:user:123:<minute> 60
```

### RD-112: Session Storage
```redis
# Store session
HSET session:abc123 user_id 456 name "John" role "admin"
EXPIRE session:abc123 3600

# Get session
HGETALL session:abc123

# Refresh TTL
EXPIRE session:abc123 3600
```

### RD-113: Caching Pattern
```redis
# Cache with TTL
SET cache:user:123 "{json}" EX 300

# Cache aside pattern (pseudocode)
# GET cache:key
# if miss: fetch from DB, SET cache:key value EX ttl
```

### RD-114: Pub/Sub
```redis
# Publish
PUBLISH channel:notifications '{"type": "alert", "message": "..."}'

# Subscribe (blocking)
SUBSCRIBE channel:notifications
PSUBSCRIBE channel:*              # Pattern subscribe
```

### RD-115: Job Queue
```redis
# Producer: add job
RPUSH jobs:pending '{"id": 1, "type": "email", ...}'

# Consumer: process job
BLPOP jobs:pending 30             # Blocking pop

# Reliable queue (move to processing)
RPOPLPUSH jobs:pending jobs:processing
# Or with blocking:
BRPOPLPUSH jobs:pending jobs:processing 30
```

### RD-116: Leaderboard
```redis
# Add/update score
ZINCRBY leaderboard 10 "user:123"

# Get rank
ZREVRANK leaderboard "user:123"

# Top 10
ZREVRANGE leaderboard 0 9 WITHSCORES

# Rank around user
ZREVRANGE leaderboard <rank-5> <rank+5> WITHSCORES
```

### RD-117: Geospatial
```redis
# Add locations
GEOADD locations 13.361389 52.519444 "Berlin"
GEOADD locations -122.419416 37.774929 "San Francisco"

# Get distance
GEODIST locations "Berlin" "San Francisco" km

# Search radius
GEOSEARCH locations FROMMEMBER "Berlin" BYRADIUS 500 km WITHDIST

# Search box
GEOSEARCH locations FROMMEMBER "Berlin" BYBOX 1000 1000 km
```

### RD-118: HyperLogLog (Cardinality)
```redis
# Add elements
PFADD visitors:2024-01 "user1" "user2" "user3"

# Count unique
PFCOUNT visitors:2024-01

# Merge periods
PFMERGE visitors:2024-q1 visitors:2024-01 visitors:2024-02 visitors:2024-03
```

### RD-119: Bitmap
```redis
# Set bit
SETBIT user:123:features 0 1     # Feature 0 enabled
SETBIT user:123:features 5 1     # Feature 5 enabled

# Get bit
GETBIT user:123:features 0

# Count set bits
BITCOUNT user:123:features

# Bitmap operations
BITOP AND result bitmap1 bitmap2
BITOP OR result bitmap1 bitmap2
```

## Transactions

### RD-120: MULTI/EXEC
```redis
MULTI
SET key1 "value1"
SET key2 "value2"
INCR counter
EXEC
```

### RD-121: WATCH for optimistic locking
```redis
WATCH key
val = GET key
MULTI
SET key (val + 1)
EXEC                             # Fails if key changed
```

## Lua Scripts

### RD-122: Atomic operations
```lua
-- Rate limiter
local current = redis.call('GET', KEYS[1])
if current and tonumber(current) >= tonumber(ARGV[1]) then
    return 0
end
redis.call('INCR', KEYS[1])
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
```

### RD-123: Distributed lock release
```lua
-- Release only if we own the lock
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
else
    return 0
end
```

### RD-124: Conditional update
```lua
-- Update only if version matches
local current_version = redis.call('HGET', KEYS[1], 'version')
if current_version == ARGV[1] then
    redis.call('HSET', KEYS[1], 'data', ARGV[2], 'version', ARGV[3])
    return 1
else
    return 0
end
```

## RD-001: Pipelining
```redis
# Send multiple commands without waiting for responses
# (client-side feature, reduces round trips)

PING
SET key1 "value1"
SET key2 "value2"
GET key1
GET key2
# ... all responses returned together
```

## RD-125: TTL Management
```redis
# Set TTL
EXPIRE key 3600                  # Seconds
PEXPIRE key 3600000              # Milliseconds
EXPIREAT key 1704067200          # Unix timestamp
PEXPIREAT key 1704067200000      # Unix timestamp (ms)

# Get TTL
TTL key                          # Seconds (-1 = no TTL, -2 = not exists)
PTTL key                         # Milliseconds

# Remove TTL
PERSIST key

# Set with TTL
SET key "value" EX 3600
SETEX key 3600 "value"           # Deprecated
```

## RD-004: Key Management
```redis
# Find keys
KEYS pattern                     # Don't use in production!
SCAN 0 MATCH "user:*" COUNT 100  # Use SCAN instead

# Check existence
EXISTS key
EXISTS key1 key2 key3            # Returns count of existing

# Delete
DEL key
UNLINK key                       # Async delete (faster for large keys)

# Type
TYPE key

# Rename
RENAME key newkey
RENAMENX key newkey              # Only if newkey doesn't exist

# Move between databases
MOVE key 1                       # Move to db 1
```
