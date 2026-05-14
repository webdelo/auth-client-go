# MongoDB + Go Integration

## Quick Reference

| ID | Rule | Section |
|----|------|---------|
| MG-001 | Use struct tags for BSON mapping | [BSON Types](#bson-types) |
| MG-002 | Create indexes for query patterns | [Index Management](#index-management) |
| MG-003 | Use aggregation pipeline for complex queries | [Aggregation](#aggregation) |
| MG-004 | Avoid unbounded arrays in documents | [BSON Types](#bson-types) |
| MG-005 | Use projection to limit returned fields | [Query Building](#query-building) |

---

## Driver Setup

### MG-001: Official driver
```go
import (
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
)
```

### MG-002: Connection pooling configuration
```go
package mongodb

import (
    "context"
    "time"

    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
    URI            string
    Database       string
    MaxPoolSize    uint64
    MinPoolSize    uint64
    ConnectTimeout time.Duration
}

func NewClient(ctx context.Context, cfg Config) (*mongo.Client, error) {
    opts := options.Client().
        ApplyURI(cfg.URI).
        SetMaxPoolSize(cfg.MaxPoolSize).
        SetMinPoolSize(cfg.MinPoolSize).
        SetConnectTimeout(cfg.ConnectTimeout)

    client, err := mongo.Connect(ctx, opts)
    if err != nil {
        return nil, fmt.Errorf("connect: %w", err)
    }

    if err := client.Ping(ctx, nil); err != nil {
        return nil, fmt.Errorf("ping: %w", err)
    }

    return client, nil
}

func NewDatabase(client *mongo.Client, name string) *mongo.Database {
    return client.Database(name)
}
```

## BSON Types

### MG-003: Use struct tags for BSON mapping
```go
type User struct {
    ID        primitive.ObjectID `bson:"_id,omitempty"`
    Name      string             `bson:"name"`
    Email     string             `bson:"email"`
    Age       int                `bson:"age,omitempty"`
    Tags      []string           `bson:"tags,omitempty"`
    Metadata  map[string]string  `bson:"metadata,omitempty"`
    CreatedAt time.Time          `bson:"createdAt"`
    UpdatedAt time.Time          `bson:"updatedAt,omitempty"`
}
```

### MG-004: BSON tag options and naming conventions
```go
type Document struct {
    ID       primitive.ObjectID `bson:"_id,omitempty"`   // Omit if zero
    Name     string             `bson:"name"`            // Renamed field
    Internal string             `bson:"-"`               // Ignored
    Count    int                `bson:"count,minsize"`   // Use smallest int type
    Data     interface{}        `bson:"data,inline"`     // Inline embedded doc
}
```

## CRUD Operations

### MG-005: Batch inserts with InsertMany for performance
```go
func (r *Repository) CreateUser(ctx context.Context, user *User) error {
    user.ID = primitive.NewObjectID()
    user.CreatedAt = time.Now()

    _, err := r.collection.InsertOne(ctx, user)
    if err != nil {
        return fmt.Errorf("insert user: %w", err)
    }

    return nil
}

func (r *Repository) CreateUsers(ctx context.Context, users []User) ([]primitive.ObjectID, error) {
    docs := make([]interface{}, len(users))
    for i := range users {
        users[i].ID = primitive.NewObjectID()
        users[i].CreatedAt = time.Now()
        docs[i] = users[i]
    }

    result, err := r.collection.InsertMany(ctx, docs)
    if err != nil {
        return nil, err
    }

    ids := make([]primitive.ObjectID, len(result.InsertedIDs))
    for i, id := range result.InsertedIDs {
        ids[i] = id.(primitive.ObjectID)
    }

    return ids, nil
}
```

### MG-006: Use projection to limit returned fields
```go
func (r *Repository) GetUser(ctx context.Context, id primitive.ObjectID) (*User, error) {
    var user User
    err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            return nil, ErrNotFound
        }
        return nil, err
    }
    return &user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
    var user User
    err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            return nil, ErrNotFound
        }
        return nil, err
    }
    return &user, nil
}

func (r *Repository) GetActiveUsers(ctx context.Context, limit int64) ([]User, error) {
    opts := options.Find().
        SetSort(bson.D{{Key: "createdAt", Value: -1}}).
        SetLimit(limit)

    cursor, err := r.collection.Find(ctx, bson.M{"status": "active"}, opts)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var users []User
    if err := cursor.All(ctx, &users); err != nil {
        return nil, err
    }

    return users, nil
}
```

### MG-007: Use atomic update operators for consistency
```go
func (r *Repository) UpdateUser(ctx context.Context, id primitive.ObjectID, name string) error {
    result, err := r.collection.UpdateOne(
        ctx,
        bson.M{"_id": id},
        bson.M{
            "$set": bson.M{
                "name":      name,
                "updatedAt": time.Now(),
            },
        },
    )
    if err != nil {
        return err
    }

    if result.MatchedCount == 0 {
        return ErrNotFound
    }

    return nil
}

func (r *Repository) IncrementCounter(ctx context.Context, id primitive.ObjectID) error {
    _, err := r.collection.UpdateOne(
        ctx,
        bson.M{"_id": id},
        bson.M{
            "$inc": bson.M{"counter": 1},
            "$set": bson.M{"updatedAt": time.Now()},
        },
    )
    return err
}

func (r *Repository) Upsert(ctx context.Context, user *User) error {
    opts := options.Update().SetUpsert(true)

    _, err := r.collection.UpdateOne(
        ctx,
        bson.M{"email": user.Email},
        bson.M{
            "$set": bson.M{
                "name":      user.Name,
                "updatedAt": time.Now(),
            },
            "$setOnInsert": bson.M{
                "createdAt": time.Now(),
            },
        },
        opts,
    )
    return err
}
```

### MG-008: Check DeletedCount to verify document existence
```go
func (r *Repository) DeleteUser(ctx context.Context, id primitive.ObjectID) error {
    result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
    if err != nil {
        return err
    }

    if result.DeletedCount == 0 {
        return ErrNotFound
    }

    return nil
}

func (r *Repository) DeleteOldLogs(ctx context.Context, before time.Time) (int64, error) {
    result, err := r.collection.DeleteMany(ctx, bson.M{
        "createdAt": bson.M{"$lt": before},
    })
    if err != nil {
        return 0, err
    }

    return result.DeletedCount, nil
}
```

## Query Building

### MG-009: Build dynamic queries with conditional operators
```go
func (r *Repository) SearchUsers(ctx context.Context, filter UserFilter) ([]User, error) {
    query := bson.M{}

    if filter.Status != "" {
        query["status"] = filter.Status
    }

    if filter.MinAge > 0 {
        query["age"] = bson.M{"$gte": filter.MinAge}
    }

    if len(filter.Tags) > 0 {
        query["tags"] = bson.M{"$all": filter.Tags}
    }

    if filter.CreatedAfter != nil {
        query["createdAt"] = bson.M{"$gt": filter.CreatedAfter}
    }

    cursor, err := r.collection.Find(ctx, query)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var users []User
    return users, cursor.All(ctx, &users)
}
```

### MG-010: Project only necessary fields to reduce memory usage
```go
func (r *Repository) GetUserNames(ctx context.Context) ([]string, error) {
    opts := options.Find().SetProjection(bson.M{"name": 1, "_id": 0})

    cursor, err := r.collection.Find(ctx, bson.M{}, opts)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []struct {
        Name string `bson:"name"`
    }
    if err := cursor.All(ctx, &results); err != nil {
        return nil, err
    }

    names := make([]string, len(results))
    for i, r := range results {
        names[i] = r.Name
    }

    return names, nil
}
```

## Aggregation

### MG-011: Use aggregation pipeline for complex queries

```go
func (r *Repository) GetOrderStats(ctx context.Context) ([]OrderStats, error) {
    pipeline := mongo.Pipeline{
        {{Key: "$match", Value: bson.M{"status": "completed"}}},
        {{Key: "$group", Value: bson.M{
            "_id":        "$userId",
            "totalOrders": bson.M{"$sum": 1},
            "totalSpent":  bson.M{"$sum": "$total"},
            "avgOrder":    bson.M{"$avg": "$total"},
        }}},
        {{Key: "$sort", Value: bson.M{"totalSpent": -1}}},
        {{Key: "$limit", Value: 10}},
    }

    cursor, err := r.collection.Aggregate(ctx, pipeline)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var stats []OrderStats
    return stats, cursor.All(ctx, &stats)
}

func (r *Repository) GetOrdersWithUsers(ctx context.Context) ([]OrderWithUser, error) {
    pipeline := mongo.Pipeline{
        {{Key: "$lookup", Value: bson.M{
            "from":         "users",
            "localField":   "userId",
            "foreignField": "_id",
            "as":           "user",
        }}},
        {{Key: "$unwind", Value: "$user"}},
        {{Key: "$project", Value: bson.M{
            "orderId":  "$_id",
            "total":    1,
            "userName": "$user.name",
        }}},
    }

    cursor, err := r.collection.Aggregate(ctx, pipeline)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []OrderWithUser
    return results, cursor.All(ctx, &results)
}
```

## Transactions

### MG-012: Use sessions for multi-document ACID transactions

```go
func (r *Repository) Transfer(ctx context.Context, from, to primitive.ObjectID, amount float64) error {
    session, err := r.client.StartSession()
    if err != nil {
        return err
    }
    defer session.EndSession(ctx)

    _, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
        // Debit
        result, err := r.accounts.UpdateOne(sessCtx,
            bson.M{"_id": from, "balance": bson.M{"$gte": amount}},
            bson.M{"$inc": bson.M{"balance": -amount}},
        )
        if err != nil {
            return nil, err
        }
        if result.ModifiedCount == 0 {
            return nil, ErrInsufficientFunds
        }

        // Credit
        _, err = r.accounts.UpdateOne(sessCtx,
            bson.M{"_id": to},
            bson.M{"$inc": bson.M{"balance": amount}},
        )
        if err != nil {
            return nil, err
        }

        return nil, nil
    })

    return err
}
```

## Bulk Operations

### MG-013: Use BulkWrite for multiple operations in one request

```go
func (r *Repository) BulkUpsert(ctx context.Context, users []User) error {
    models := make([]mongo.WriteModel, len(users))

    for i, user := range users {
        models[i] = mongo.NewUpdateOneModel().
            SetFilter(bson.M{"email": user.Email}).
            SetUpdate(bson.M{
                "$set": bson.M{
                    "name":      user.Name,
                    "updatedAt": time.Now(),
                },
                "$setOnInsert": bson.M{
                    "createdAt": time.Now(),
                },
            }).
            SetUpsert(true)
    }

    _, err := r.collection.BulkWrite(ctx, models)
    return err
}
```

## Change Streams

### MG-014: Monitor collection changes with Watch pipeline

```go
func (r *Repository) WatchOrders(ctx context.Context) error {
    pipeline := mongo.Pipeline{
        {{Key: "$match", Value: bson.M{
            "operationType": "insert",
        }}},
    }

    stream, err := r.collection.Watch(ctx, pipeline)
    if err != nil {
        return err
    }
    defer stream.Close(ctx)

    for stream.Next(ctx) {
        var event struct {
            FullDocument Order `bson:"fullDocument"`
        }
        if err := stream.Decode(&event); err != nil {
            continue
        }

        // Process event
        fmt.Printf("New order: %v\n", event.FullDocument)
    }

    return stream.Err()
}
```

## Index Management

### MG-015: Create indexes for query patterns

```go
func (r *Repository) CreateIndexes(ctx context.Context) error {
    indexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "email", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{
                {Key: "status", Value: 1},
                {Key: "createdAt", Value: -1},
            },
        },
        {
            Keys:    bson.D{{Key: "createdAt", Value: 1}},
            Options: options.Index().SetExpireAfterSeconds(86400 * 30), // TTL 30 days
        },
    }

    _, err := r.collection.Indexes().CreateMany(ctx, indexes)
    return err
}
```

## Error Handling

### MG-016: Handle MongoDB-specific errors correctly

```go
func handleMongoError(err error) error {
    if err == nil {
        return nil
    }

    if err == mongo.ErrNoDocuments {
        return ErrNotFound
    }

    var writeErr mongo.WriteException
    if errors.As(err, &writeErr) {
        for _, we := range writeErr.WriteErrors {
            if we.Code == 11000 { // Duplicate key
                return ErrDuplicateKey
            }
        }
    }

    return err
}
```

## Testing

### MG-017: Use isolated test databases with cleanup

```go
func setupTestMongo(t *testing.T) *mongo.Database {
    t.Helper()

    uri := os.Getenv("TEST_MONGO_URI")
    if uri == "" {
        t.Skip("TEST_MONGO_URI not set")
    }

    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
    require.NoError(t, err)

    dbName := fmt.Sprintf("test_%d", time.Now().UnixNano())
    db := client.Database(dbName)

    t.Cleanup(func() {
        db.Drop(context.Background())
        client.Disconnect(context.Background())
    })

    return db
}
```
