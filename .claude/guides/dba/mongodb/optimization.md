# MongoDB Optimization Guide

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MG-002 | Create indexes for query patterns | [Index Strategies](#index-strategies) |
| MG-005 | Use projection to limit returned fields | [Query Optimization](#query-optimization) |
| MGD-003 | Set appropriate write concern | [Write Concern](#write-concern) |

---

## Query Analysis

### MG-200: Use explain plans to analyze query performance

```javascript
// Basic explain
db.orders.find({ status: "pending" }).explain()

// Detailed execution stats
db.orders.find({ status: "pending" }).explain("executionStats")

// Full verbosity (includes rejected plans)
db.orders.find({ status: "pending" }).explain("allPlansExecution")
```

### MG-201: Interpret execution metrics from explain output

```javascript
{
  "executionStats": {
    "nReturned": 100,           // Documents returned
    "executionTimeMillis": 50,  // Total time
    "totalKeysExamined": 100,   // Index entries scanned
    "totalDocsExamined": 100,   // Documents scanned
    // Ideal: keysExamined ≈ docsExamined ≈ nReturned
  },
  "queryPlanner": {
    "winningPlan": {
      "stage": "FETCH",         // or IXSCAN, COLLSCAN
      "inputStage": {
        "stage": "IXSCAN",
        "indexName": "status_1"
      }
    }
  }
}
```

### MG-202: Optimize query stages for best performance

| Stage | Meaning | Performance |
|-------|---------|-------------|
| COLLSCAN | Full collection scan | Poor |
| IXSCAN | Index scan | Good |
| FETCH | Retrieve documents | Expected |
| SORT | In-memory sort | Watch memory |
| SORT_KEY_GENERATOR | Sort keys | Watch memory |
| PROJECTION_COVERED | Index-only | Excellent |

## Index Strategies

### MG-203: Create appropriate index types for query patterns

```javascript
// Single field
db.users.createIndex({ email: 1 })

// Compound (order matters!)
db.orders.createIndex({ userId: 1, createdAt: -1 })

// Multikey (arrays)
db.products.createIndex({ tags: 1 })

// Text index
db.articles.createIndex({ title: "text", content: "text" })

// Geospatial
db.locations.createIndex({ coordinates: "2dsphere" })

// Hashed (for sharding)
db.sessions.createIndex({ sessionId: "hashed" })

// TTL (auto-delete)
db.sessions.createIndex({ expiresAt: 1 }, { expireAfterSeconds: 0 })

// Unique
db.users.createIndex({ email: 1 }, { unique: true })

// Sparse (skip nulls)
db.users.createIndex({ phone: 1 }, { sparse: true })

// Partial (filtered)
db.orders.createIndex(
  { status: 1, createdAt: 1 },
  { partialFilterExpression: { status: "pending" } }
)
```

### MG-204: Design compound indexes using ESR rule

**Equality, Sort, Range** order:

```javascript
// Query: find by userId, sort by createdAt, filter by status range
db.orders.find({
  userId: ObjectId("..."),
  status: { $in: ["pending", "processing"] }
}).sort({ createdAt: -1 })

// Optimal index:
db.orders.createIndex({
  userId: 1,      // Equality first
  createdAt: -1,  // Sort second
  status: 1       // Range last
})
```

### MG-205: Use covered queries to avoid document fetches

```javascript
// Index covers all fields
db.users.createIndex({ email: 1, name: 1, createdAt: 1 })

// Query uses only indexed fields
db.users.find(
  { email: "user@example.com" },
  { _id: 0, name: 1, createdAt: 1 }  // Projection matches index
)
// Stage: PROJECTION_COVERED (no FETCH needed)
```

### MG-206: Monitor and maintain indexes

```javascript
// List indexes
db.collection.getIndexes()

// Drop index
db.collection.dropIndex("index_name")

// Rebuild indexes
db.collection.reIndex()

// Index stats
db.collection.aggregate([
  { $indexStats: {} }
])

// Unused indexes
db.collection.aggregate([
  { $indexStats: {} },
  { $match: { "accesses.ops": 0 } }
])
```

## Query Optimization

### MG-207: Use projection to limit returned fields

```javascript
// Bad: returns entire document
db.users.find({ email: "user@example.com" })

// Good: returns only needed fields
db.users.find(
  { email: "user@example.com" },
  { _id: 1, name: 1, email: 1 }
)
```

### MG-208: Avoid expensive operators like $where and unanchored $regex

```javascript
// Bad: full scan
db.users.find({ $where: "this.name.length > 10" })
db.users.find({ email: /gmail/ })

// Good: use operators
db.users.find({ $expr: { $gt: [{ $strLenCP: "$name" }, 10] } })
db.users.find({ email: /^user.*@gmail\.com$/ })  // Anchored regex uses index
```

### MG-209: Use range-based pagination instead of large skips

```javascript
// Bad: large skip is expensive
db.orders.find().skip(100000).limit(10)

// Good: range-based pagination
db.orders.find({ _id: { $gt: lastSeenId } }).limit(10)

// Or using indexed field
db.orders.find({ createdAt: { $lt: lastSeenDate } })
  .sort({ createdAt: -1 })
  .limit(10)
```

### MG-210: Prefer $in over $or for same-field conditions

```javascript
// Less efficient
db.orders.find({
  $or: [
    { status: "pending" },
    { status: "processing" },
    { status: "shipped" }
  ]
})

// More efficient
db.orders.find({
  status: { $in: ["pending", "processing", "shipped"] }
})
```

## Go Integration Optimization

### MG-211: Use projection in Go queries for efficiency

```go
import (
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func (r *MongoRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
    var user User

    // Use projection to limit returned fields
    opts := options.FindOne().SetProjection(bson.M{
        "_id":       1,
        "email":     1,
        "name":      1,
        "createdAt": 1,
    })

    err := r.collection.FindOne(ctx, bson.M{"email": email}, opts).Decode(&user)
    return &user, err
}
```

### MG-212: Use batch operations instead of single operations

```go
// Bad: N separate inserts
for _, doc := range docs {
    r.collection.InsertOne(ctx, doc)
}

// Good: bulk insert
r.collection.InsertMany(ctx, docs)

// For updates: bulk write
models := []mongo.WriteModel{}
for _, item := range items {
    models = append(models, mongo.NewUpdateOneModel().
        SetFilter(bson.M{"_id": item.ID}).
        SetUpdate(bson.M{"$set": bson.M{"status": item.Status}}))
}
r.collection.BulkWrite(ctx, models)
```

### MG-213: Configure cursor parameters for large result sets

```go
func (r *MongoRepo) ProcessAllOrders(ctx context.Context) error {
    opts := options.Find().
        SetBatchSize(100).           // Documents per batch
        SetNoCursorTimeout(true).    // For long operations
        SetProjection(bson.M{        // Only needed fields
            "_id":    1,
            "status": 1,
            "total":  1,
        })

    cursor, err := r.collection.Find(ctx, bson.M{}, opts)
    if err != nil {
        return err
    }
    defer cursor.Close(ctx)

    for cursor.Next(ctx) {
        var order Order
        if err := cursor.Decode(&order); err != nil {
            return err
        }
        // Process order...
    }

    return cursor.Err()
}
```

### MG-214: Configure connection pool for optimal concurrency

```go
clientOpts := options.Client().
    ApplyURI("mongodb://localhost:27017").
    SetMaxPoolSize(100).           // Max connections
    SetMinPoolSize(10).            // Keep minimum
    SetMaxConnIdleTime(10 * time.Minute).
    SetConnectTimeout(10 * time.Second).
    SetServerSelectionTimeout(5 * time.Second)

client, err := mongo.Connect(ctx, clientOpts)
```

## Memory Optimization

### MG-215: Monitor working set to ensure RAM availability

```javascript
// Check working set
db.serverStatus().wiredTiger.cache

// Key metrics:
// - "bytes currently in the cache" - should fit in RAM
// - "tracked dirty bytes in the cache" - pending writes
// - "pages evicted" - if high, need more RAM
```

### MG-216: Use allowDiskUse for large aggregations

```javascript
// Default: 100MB per aggregation stage
// For large aggregations:
db.orders.aggregate([
  { $group: { _id: "$userId", total: { $sum: "$amount" } } }
], { allowDiskUse: true })  // Spill to disk if needed
```

### MG-217: Track document sizes to optimize storage

```javascript
// Check document sizes
db.collection.find().forEach(function(doc) {
  print(Object.bsonsize(doc));
});

// Or aggregate
db.collection.aggregate([
  { $project: { size: { $bsonSize: "$$ROOT" } } },
  { $group: {
    _id: null,
    avgSize: { $avg: "$size" },
    maxSize: { $max: "$size" }
  }}
])
```

## Profiling

### MG-218: Enable query profiler to identify slow queries

```javascript
// Profile slow queries (>100ms)
db.setProfilingLevel(1, { slowms: 100 })

// Profile all queries
db.setProfilingLevel(2)

// Disable
db.setProfilingLevel(0)

// Check level
db.getProfilingLevel()
```

### MG-219: Analyze profiler output for optimization targets

```javascript
// Find slow queries
db.system.profile.find().sort({ ts: -1 }).limit(10)

// Find queries without index
db.system.profile.find({
  planSummary: "COLLSCAN"
}).sort({ millis: -1 })

// Find slow queries by collection
db.system.profile.find({
  ns: "mydb.orders",
  millis: { $gt: 100 }
})
```

## Configuration Tuning

### MG-220: Tune WiredTiger cache size appropriately

```yaml
# mongod.conf
storage:
  wiredTiger:
    engineConfig:
      cacheSizeGB: 4  # 50% of RAM - 1GB is default

# Or via command line
mongod --wiredTigerCacheSizeGB 4
```

### MG-221: Configure connection limits and oplog size

```yaml
net:
  maxIncomingConnections: 65536

# For replica set
replication:
  oplogSizeMB: 10240  # For high write loads
```

## Monitoring Queries

### MG-222: Monitor running operations and collection statistics

```javascript
// Current operations
db.currentOp()

// Kill long-running query
db.killOp(opId)

// Server status
db.serverStatus()

// Collection stats
db.collection.stats()

// Index usage stats
db.collection.aggregate([{ $indexStats: {} }])
```

## Checklist

1. **Use explain()** for slow queries
2. **Create compound indexes** following ESR rule
3. **Use projections** - return only needed fields
4. **Avoid large skips** - use range-based pagination
5. **Monitor working set** - should fit in RAM
6. **Use bulk operations** for multiple writes
7. **Profile slow queries** - find optimization targets
8. **Check unused indexes** - remove to save space
9. **Use covered queries** when possible
10. **Set appropriate timeouts** in Go client
