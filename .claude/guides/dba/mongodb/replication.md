# MongoDB Replication and Sharding

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MGD-001 | Use WiredTiger storage engine | [Storage Engine](#storage-engine) |
| MGD-002 | Configure replica set for HA | [Replica Set](#replica-set) |
| MGD-003 | Set appropriate write concern | [Write Concern](#write-concern) |

---

## Replica Set

### MG-300: Use replica sets for high availability

```
                    ┌─────────────┐
                    │   Primary   │ ← Writes
                    │  (votes: 1) │
                    └─────────────┘
                          │
            ┌─────────────┴─────────────┐
            │                           │
    ┌───────────────┐           ┌───────────────┐
    │  Secondary    │           │  Secondary    │
    │  (votes: 1)   │           │  (votes: 1)   │
    └───────────────┘           └───────────────┘
           ↑                           ↑
         Reads                       Reads
      (if configured)            (if configured)
```

### MG-301: Configure replica set with proper settings

```yaml
# mongod.conf
replication:
  replSetName: "rs0"
  oplogSizeMB: 10240  # Size in MB

net:
  bindIp: 0.0.0.0
  port: 27017

security:
  keyFile: /etc/mongodb/keyfile  # For internal auth
```

### MG-302: Initialize replica set with member configuration

```javascript
// Connect to primary
rs.initiate({
  _id: "rs0",
  members: [
    { _id: 0, host: "mongo1:27017", priority: 2 },  // Higher priority = preferred primary
    { _id: 1, host: "mongo2:27017", priority: 1 },
    { _id: 2, host: "mongo3:27017", priority: 1 }
  ]
})

// Check status
rs.status()

// Check configuration
rs.conf()
```

### MG-303: Configure specialized replica set member types

```javascript
// Standard member
{ _id: 0, host: "mongo1:27017" }

// Hidden member (not visible to clients)
{ _id: 1, host: "mongo2:27017", hidden: true, priority: 0 }

// Delayed member (for recovery)
{
  _id: 2,
  host: "mongo3:27017",
  priority: 0,
  hidden: true,
  secondaryDelaySecs: 3600  // 1 hour behind
}

// Arbiter (votes only, no data)
{ _id: 3, host: "arbiter:27017", arbiterOnly: true }

// Non-voting member
{ _id: 4, host: "mongo4:27017", votes: 0, priority: 0 }
```

### MG-304: Set read preferences for load distribution

```javascript
// Read from primary only (default)
db.collection.find().readPref("primary")

// Prefer primary, fallback to secondary
db.collection.find().readPref("primaryPreferred")

// Read from secondary only
db.collection.find().readPref("secondary")

// Prefer secondary
db.collection.find().readPref("secondaryPreferred")

// Nearest (lowest latency)
db.collection.find().readPref("nearest")

// With tags
db.collection.find().readPref("secondary", [{ region: "us-east" }])
```

### MG-305: Set write concern for durability guarantees

```javascript
// Acknowledge from primary only (default)
db.collection.insertOne(doc, { writeConcern: { w: 1 } })

// Acknowledge from majority
db.collection.insertOne(doc, { writeConcern: { w: "majority" } })

// Acknowledge from all members
db.collection.insertOne(doc, { writeConcern: { w: 3 } })

// With journal
db.collection.insertOne(doc, { writeConcern: { w: "majority", j: true } })

// With timeout
db.collection.insertOne(doc, { writeConcern: { w: "majority", wtimeout: 5000 } })
```

### MG-306: Configure Go client for replica sets

```go
import (
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/mongo/readpref"
    "go.mongodb.org/mongo-driver/mongo/writeconcern"
)

func NewMongoClient(ctx context.Context) (*mongo.Client, error) {
    // Connection string with replica set
    uri := "mongodb://mongo1:27017,mongo2:27017,mongo3:27017/?replicaSet=rs0"

    opts := options.Client().
        ApplyURI(uri).
        SetReadPreference(readpref.SecondaryPreferred()).
        SetWriteConcern(writeconcern.Majority()).
        SetRetryWrites(true).
        SetRetryReads(true)

    return mongo.Connect(ctx, opts)
}

// Per-operation read preference
func (r *MongoRepo) GetFromSecondary(ctx context.Context, id string) (*Doc, error) {
    opts := options.FindOne().SetReadPreference(readpref.Secondary())
    var doc Doc
    err := r.collection.FindOne(ctx, bson.M{"_id": id}, opts).Decode(&doc)
    return &doc, err
}

// Per-operation write concern
func (r *MongoRepo) InsertCritical(ctx context.Context, doc *Doc) error {
    opts := options.InsertOne().SetWriteConcern(writeconcern.Majority())
    _, err := r.collection.InsertOne(ctx, doc, opts)
    return err
}
```

## Sharding

### MG-307: Use sharding for horizontal scalability

```
         ┌─────────────┐
         │   mongos    │ ← Application connects here
         │   (router)  │
         └─────────────┘
                │
    ┌───────────┼───────────┐
    │           │           │
┌───────┐   ┌───────┐   ┌───────┐
│ Shard │   │ Shard │   │ Shard │
│  rs0  │   │  rs1  │   │  rs2  │
│(RS)   │   │(RS)   │   │(RS)   │
└───────┘   └───────┘   └───────┘
                │
         ┌─────────────┐
         │Config Server│
         │   (RS)      │
         └─────────────┘
```

### MG-308: Enable sharding on database and collections

```javascript
// Connect to mongos
// Enable sharding on database
sh.enableSharding("mydb")

// Shard a collection
// Range-based sharding
sh.shardCollection("mydb.orders", { userId: 1 })

// Hashed sharding (better distribution)
sh.shardCollection("mydb.events", { eventId: "hashed" })

// Compound shard key
sh.shardCollection("mydb.logs", { tenantId: 1, timestamp: 1 })
```

### MG-309: Choose appropriate shard key

| Shard Key Type | Pros | Cons | Use Case |
|----------------|------|------|----------|
| Hashed | Even distribution | No range queries | High write, even reads |
| Range | Range queries work | Hotspots possible | Time-series, ranges |
| Compound | Flexible | More planning needed | Multi-tenant |

```javascript
// Good shard keys:
// - High cardinality
// - Even distribution
// - Query isolation

// Bad shard keys:
// - Monotonically increasing (ObjectId, timestamp alone)
// - Low cardinality
// - Frequently updated
```

### MG-310: Use zone sharding for data locality

```javascript
// Add zones to shards
sh.addShardTag("shard0", "US")
sh.addShardTag("shard1", "EU")
sh.addShardTag("shard2", "APAC")

// Assign zone ranges
sh.addTagRange(
  "mydb.users",
  { region: "us" },
  { region: "ut" },  // Range: us <= region < ut
  "US"
)

sh.addTagRange(
  "mydb.users",
  { region: "eu" },
  { region: "ev" },
  "EU"
)
```

### MG-311: Monitor shard distribution and balancer
// Sharding status
sh.status()

// Chunk distribution
db.collection.getShardDistribution()

// Balancer status
sh.getBalancerState()
sh.isBalancerRunning()

// Enable/disable balancer
sh.stopBalancer()
sh.startBalancer()
```

### MG-312: Configure Go client for sharded clusters

```go
func NewShardedClient(ctx context.Context) (*mongo.Client, error) {
    // Connect to mongos routers
    uri := "mongodb://mongos1:27017,mongos2:27017,mongos3:27017"

    opts := options.Client().
        ApplyURI(uri).
        SetReadPreference(readpref.Nearest()).
        SetWriteConcern(writeconcern.Majority())

    return mongo.Connect(ctx, opts)
}
```

## High Availability Patterns

### MG-313: Use majority write concern for critical data

```go
func (r *MongoRepo) InsertWithMajority(ctx context.Context, doc interface{}) error {
    wc := writeconcern.Majority()
    wc.WTimeout = 5 * time.Second

    opts := options.InsertOne().SetWriteConcern(wc)
    _, err := r.collection.InsertOne(ctx, doc, opts)
    return err
}
```

### MG-314: Implement read-your-writes consistency pattern

```go
// After write, read from primary to ensure consistency
func (r *MongoRepo) UpdateAndRead(ctx context.Context, id string, update bson.M) (*Doc, error) {
    // Write with majority
    _, err := r.collection.UpdateOne(ctx,
        bson.M{"_id": id},
        bson.M{"$set": update},
        options.Update().SetWriteConcern(writeconcern.Majority()))
    if err != nil {
        return nil, err
    }

    // Read from primary
    var doc Doc
    opts := options.FindOne().SetReadPreference(readpref.Primary())
    err = r.collection.FindOne(ctx, bson.M{"_id": id}, opts).Decode(&doc)
    return &doc, err
}
```

### MG-315: Use causal consistency for related operations

```go
func (r *MongoRepo) CausalOperation(ctx context.Context) error {
    // Start causally consistent session
    sessionOpts := options.Session().SetCausalConsistency(true)
    session, err := r.client.StartSession(sessionOpts)
    if err != nil {
        return err
    }
    defer session.EndSession(ctx)

    return mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
        // Write
        _, err := r.collection.InsertOne(sc, bson.M{"test": "data"})
        if err != nil {
            return err
        }

        // Subsequent reads will see the write
        // even if reading from secondary
        cursor, err := r.collection.Find(sc,
            bson.M{},
            options.Find().SetReadPreference(readpref.Secondary()))
        if err != nil {
            return err
        }
        defer cursor.Close(sc)

        // Process results...
        return nil
    })
}
```

## Monitoring and Operations

### MG-316: Monitor replication lag between members

```javascript
// Check replication lag
rs.printSecondaryReplicationInfo()

// Detailed sync status
db.adminCommand({ replSetGetStatus: 1 }).members.forEach(m => {
    if (m.stateStr === "SECONDARY") {
        print(`${m.name}: ${m.optimeDate}, lag: ${(new Date() - m.optimeDate)/1000}s`)
    }
})
```

### MG-317: Trigger failover to test and manage recovery

```javascript
// Step down primary (triggers election)
rs.stepDown()

// Step down with options
rs.stepDown(60, 30)  // stepDownSecs, secondaryCatchUpPeriodSecs
```

### MG-318: Manage replica set membership

```javascript
// Add member
rs.add("mongo4:27017")

// Add with options
rs.add({ host: "mongo4:27017", priority: 0, hidden: true })

// Remove member
rs.remove("mongo4:27017")

// Reconfigure
cfg = rs.conf()
cfg.members[1].priority = 2
rs.reconfig(cfg)
```

## Best Practices

1. **Always use odd number of voting members** (3, 5, 7)
2. **Distribute across data centers** for disaster recovery
3. **Use majority write concern** for critical data
4. **Monitor replication lag** - alert if > threshold
5. **Test failover** regularly
6. **Choose shard key carefully** - cannot change easily
7. **Use hashed sharding** for even distribution
8. **Zone sharding** for data locality requirements
9. **Causal consistency** for read-your-writes
10. **Set appropriate timeouts** for write concerns
