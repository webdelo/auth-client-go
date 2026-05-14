# MongoDB Aggregation Framework

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MG-003 | Use aggregation pipeline for complex queries | [Pipeline Stages](#pipeline-stages) |
| MG-002 | Create indexes for query patterns | [Pipeline Optimization](#pipeline-optimization) |
| MG-005 | Use projection to limit returned fields | [$project Stage](#core-stages) |

---

## Pipeline Stages

### MG-400: Use core aggregation stages for data transformation

```javascript
// $match - Filter documents (early = better)
{ $match: { status: "active", age: { $gte: 18 } } }

// $project - Select/compute fields
{ $project: { name: 1, email: 1, fullName: { $concat: ["$firstName", " ", "$lastName"] } } }

// $group - Aggregate values
{ $group: { _id: "$category", total: { $sum: "$amount" }, count: { $sum: 1 } } }

// $sort - Order results
{ $sort: { createdAt: -1, name: 1 } }

// $limit - Limit results
{ $limit: 10 }

// $skip - Skip results
{ $skip: 20 }

// $unwind - Deconstruct arrays
{ $unwind: "$tags" }

// $lookup - Join collections
{
  $lookup: {
    from: "users",
    localField: "userId",
    foreignField: "_id",
    as: "user"
  }
}

// $addFields - Add computed fields
{ $addFields: { totalWithTax: { $multiply: ["$total", 1.2] } } }

// $set - Alias for $addFields
{ $set: { updatedAt: "$$NOW" } }

// $unset - Remove fields
{ $unset: ["internalField", "tempData"] }

// $count - Count documents
{ $count: "totalOrders" }

// $out - Write to collection
{ $out: "aggregated_results" }

// $merge - Merge into collection
{
  $merge: {
    into: "monthly_stats",
    on: "_id",
    whenMatched: "replace",
    whenNotMatched: "insert"
  }
}
```

### MG-401: Optimize pipeline stage ordering

```javascript
// BAD: $match after $project
db.orders.aggregate([
  { $project: { total: 1, status: 1 } },
  { $match: { status: "completed" } }  // Full scan first
])

// GOOD: $match first (uses index)
db.orders.aggregate([
  { $match: { status: "completed" } },  // Index scan
  { $project: { total: 1, status: 1 } }
])

// BAD: $sort after $group (no index)
db.orders.aggregate([
  { $group: { _id: "$userId", total: { $sum: "$amount" } } },
  { $sort: { total: -1 } }  // In-memory sort
])

// GOOD: $sort before $group when possible
db.orders.aggregate([
  { $sort: { userId: 1 } },  // Can use index
  { $group: { _id: "$userId", total: { $sum: "$amount" } } }
])
```

## Common Aggregation Patterns

### MG-402: Group and aggregate data with accumulators

```javascript
// Count by category
db.products.aggregate([
  { $group: {
    _id: "$category",
    count: { $sum: 1 },
    avgPrice: { $avg: "$price" },
    minPrice: { $min: "$price" },
    maxPrice: { $max: "$price" },
    products: { $push: "$name" }  // Collect into array
  }},
  { $sort: { count: -1 } }
])
```

### MG-403: Aggregate time-series data by date components

```javascript
// Group by day
db.orders.aggregate([
  { $match: { createdAt: { $gte: ISODate("2024-01-01") } } },
  { $group: {
    _id: {
      year: { $year: "$createdAt" },
      month: { $month: "$createdAt" },
      day: { $dayOfMonth: "$createdAt" }
    },
    orders: { $sum: 1 },
    revenue: { $sum: "$total" }
  }},
  { $sort: { "_id.year": 1, "_id.month": 1, "_id.day": 1 } }
])

// Using $dateTrunc (MongoDB 5.0+)
db.orders.aggregate([
  { $group: {
    _id: { $dateTrunc: { date: "$createdAt", unit: "day" } },
    orders: { $sum: 1 }
  }}
])
```

### MG-404: Join collections with $lookup stage

```javascript
// Basic lookup
db.orders.aggregate([
  { $lookup: {
    from: "users",
    localField: "userId",
    foreignField: "_id",
    as: "user"
  }},
  { $unwind: "$user" }  // Convert array to object
])

// Pipeline lookup (more flexible)
db.orders.aggregate([
  { $lookup: {
    from: "users",
    let: { userId: "$userId" },
    pipeline: [
      { $match: { $expr: { $eq: ["$_id", "$$userId"] } } },
      { $project: { name: 1, email: 1 } }  // Select fields
    ],
    as: "user"
  }}
])

// Multiple lookups
db.orders.aggregate([
  { $lookup: { from: "users", localField: "userId", foreignField: "_id", as: "user" } },
  { $lookup: { from: "products", localField: "productIds", foreignField: "_id", as: "products" } },
  { $unwind: "$user" }
])
```

### MG-405: Manipulate arrays in aggregation pipeline

```javascript
// Unwind and regroup
db.orders.aggregate([
  { $unwind: "$items" },
  { $group: {
    _id: "$items.productId",
    totalSold: { $sum: "$items.quantity" },
    revenue: { $sum: { $multiply: ["$items.quantity", "$items.price"] } }
  }}
])

// Array operations without unwind
db.orders.aggregate([
  { $project: {
    itemCount: { $size: "$items" },
    totalQuantity: { $sum: "$items.quantity" },
    productIds: "$items.productId"
  }}
])

// Filter array elements
db.orders.aggregate([
  { $project: {
    highValueItems: {
      $filter: {
        input: "$items",
        as: "item",
        cond: { $gte: ["$$item.price", 100] }
      }
    }
  }}
])
```

### MG-406: Implement faceted search with $facet

```javascript
db.products.aggregate([
  { $match: { status: "active" } },
  { $facet: {
    // Get results
    results: [
      { $sort: { price: 1 } },
      { $skip: 0 },
      { $limit: 10 }
    ],
    // Count by category
    categories: [
      { $group: { _id: "$category", count: { $sum: 1 } } }
    ],
    // Price ranges
    priceRanges: [
      { $bucket: {
        groupBy: "$price",
        boundaries: [0, 50, 100, 200, 500, Infinity],
        output: { count: { $sum: 1 } }
      }}
    ],
    // Total count
    total: [
      { $count: "count" }
    ]
  }}
])
```

### MG-407: Use window functions for analytics

```javascript
db.transactions.aggregate([
  { $sort: { date: 1 } },
  { $setWindowFields: {
    partitionBy: "$accountId",
    sortBy: { date: 1 },
    output: {
      runningBalance: {
        $sum: "$amount",
        window: { documents: ["unbounded", "current"] }
      },
      movingAvg: {
        $avg: "$amount",
        window: { documents: [-5, 0] }  // Last 6 including current
      }
    }
  }}
])
```

## Go Integration

### MG-408: Build aggregation pipelines in Go

```go
import (
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
)

func (r *MongoRepo) GetSalesByCategory(ctx context.Context) ([]CategorySales, error) {
    pipeline := mongo.Pipeline{
        {{Key: "$match", Value: bson.M{"status": "completed"}}},
        {{Key: "$group", Value: bson.M{
            "_id":      "$category",
            "total":    bson.M{"$sum": "$amount"},
            "count":    bson.M{"$sum": 1},
            "avgPrice": bson.M{"$avg": "$amount"},
        }}},
        {{Key: "$sort", Value: bson.M{"total": -1}}},
    }

    cursor, err := r.collection.Aggregate(ctx, pipeline)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []CategorySales
    if err := cursor.All(ctx, &results); err != nil {
        return nil, err
    }

    return results, nil
}

type CategorySales struct {
    Category string  `bson:"_id"`
    Total    float64 `bson:"total"`
    Count    int     `bson:"count"`
    AvgPrice float64 `bson:"avgPrice"`
}
```

### MG-409: Use lookup in Go aggregation pipelines

```go
func (r *MongoRepo) GetOrdersWithUsers(ctx context.Context, status string) ([]OrderWithUser, error) {
    pipeline := mongo.Pipeline{
        {{Key: "$match", Value: bson.M{"status": status}}},
        {{Key: "$lookup", Value: bson.M{
            "from":         "users",
            "localField":   "userId",
            "foreignField": "_id",
            "as":           "user",
        }}},
        {{Key: "$unwind", Value: "$user"}},
        {{Key: "$project", Value: bson.M{
            "_id":       1,
            "total":     1,
            "status":    1,
            "createdAt": 1,
            "userName":  "$user.name",
            "userEmail": "$user.email",
        }}},
        {{Key: "$limit", Value: 100}},
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

### MG-410: Configure aggregation options for performance

```go
func (r *MongoRepo) GetLargeAggregation(ctx context.Context) ([]Result, error) {
    pipeline := mongo.Pipeline{
        // ... stages
    }

    opts := options.Aggregate().
        SetAllowDiskUse(true).          // For large datasets
        SetBatchSize(1000).             // Cursor batch size
        SetMaxTime(30 * time.Second).   // Timeout
        SetComment("monthly-report")    // For profiling

    cursor, err := r.collection.Aggregate(ctx, pipeline, opts)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []Result
    return results, cursor.All(ctx, &results)
}
```

### MG-411: Build dynamic aggregation pipelines

```go
func (r *MongoRepo) SearchProducts(ctx context.Context, filter ProductFilter) ([]Product, error) {
    pipeline := mongo.Pipeline{}

    // Dynamic match stage
    matchConditions := bson.M{}
    if filter.Category != "" {
        matchConditions["category"] = filter.Category
    }
    if filter.MinPrice > 0 {
        matchConditions["price"] = bson.M{"$gte": filter.MinPrice}
    }
    if filter.MaxPrice > 0 {
        if existing, ok := matchConditions["price"].(bson.M); ok {
            existing["$lte"] = filter.MaxPrice
        } else {
            matchConditions["price"] = bson.M{"$lte": filter.MaxPrice}
        }
    }

    if len(matchConditions) > 0 {
        pipeline = append(pipeline, bson.D{{Key: "$match", Value: matchConditions}})
    }

    // Sort
    sortField := "createdAt"
    sortOrder := -1
    if filter.SortBy != "" {
        sortField = filter.SortBy
    }
    if filter.SortAsc {
        sortOrder = 1
    }
    pipeline = append(pipeline, bson.D{{Key: "$sort", Value: bson.M{sortField: sortOrder}}})

    // Pagination
    if filter.Skip > 0 {
        pipeline = append(pipeline, bson.D{{Key: "$skip", Value: filter.Skip}})
    }
    pipeline = append(pipeline, bson.D{{Key: "$limit", Value: filter.Limit}})

    cursor, err := r.collection.Aggregate(ctx, pipeline)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []Product
    return results, cursor.All(ctx, &results)
}
```

## Performance Tips

1. **$match early** - Filter before other stages
2. **Use indexes** - First $match should use index
3. **$project early** - Remove unneeded fields
4. **Limit array growth** - Use $slice with $push
5. **allowDiskUse** - For large aggregations
6. **Avoid $where** - Use aggregation operators
7. **Use $facet wisely** - Each facet runs full pipeline
8. **Index for $sort** - Especially early in pipeline
