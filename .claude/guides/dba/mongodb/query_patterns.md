# MongoDB Query Patterns

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| MG-003 | Use aggregation pipeline for complex queries | [Aggregation Pipeline](#aggregation-pipeline) |
| MG-005 | Use projection to limit returned fields | [Find](#find) |
| MG-004 | Avoid unbounded arrays in documents | [Document Design](#document-design) |

---

## Basic Operations

### MG-100: Query operations with projection and filtering
```javascript
// Find one
db.users.findOne({ _id: ObjectId("...") })

// Find with projection
db.users.find(
    { status: "active" },
    { name: 1, email: 1, _id: 0 }
)

// Find with conditions
db.orders.find({
    status: "pending",
    createdAt: { $gt: ISODate("2024-01-01") }
}).sort({ createdAt: -1 }).limit(10)
```

### MG-101: Batch insert operations
```javascript
// Insert one
db.users.insertOne({
    name: "John",
    email: "john@example.com",
    createdAt: new Date()
})

// Insert many
db.users.insertMany([
    { name: "John", email: "john@example.com" },
    { name: "Jane", email: "jane@example.com" }
])
```

### MG-102: Atomic updates with operators
```javascript
// Update one
db.users.updateOne(
    { _id: ObjectId("...") },
    { $set: { name: "New Name", updatedAt: new Date() } }
)

// Update many
db.users.updateMany(
    { status: "inactive" },
    { $set: { archived: true } }
)

// Upsert
db.stats.updateOne(
    { userId: "123" },
    { $inc: { loginCount: 1 }, $set: { lastLogin: new Date() } },
    { upsert: true }
)
```

### MG-103: Delete operations with time-based filters
```javascript
// Delete one
db.users.deleteOne({ _id: ObjectId("...") })

// Delete many
db.logs.deleteMany({ createdAt: { $lt: ISODate("2023-01-01") } })
```

## Query Operators

### MG-104: Comparison operators for range queries
```javascript
db.products.find({
    price: { $gt: 100 },           // Greater than
    stock: { $gte: 10 },           // Greater than or equal
    discount: { $lt: 50 },         // Less than
    rating: { $lte: 5 },           // Less than or equal
    category: { $eq: "electronics" }, // Equal
    status: { $ne: "deleted" },    // Not equal
    tags: { $in: ["sale", "new"] }, // In array
    type: { $nin: ["draft", "archived"] } // Not in array
})
```

### MG-105: Logical operators for complex conditions
```javascript
// AND (implicit)
db.users.find({ status: "active", role: "admin" })

// AND (explicit)
db.users.find({ $and: [{ age: { $gte: 18 } }, { age: { $lte: 65 } }] })

// OR
db.users.find({ $or: [{ status: "active" }, { role: "admin" }] })

// NOR
db.users.find({ $nor: [{ status: "deleted" }, { banned: true }] })

// NOT
db.users.find({ age: { $not: { $lt: 18 } } })
```

### MG-106: Element operators for field existence and type checks
```javascript
// Field exists
db.users.find({ phone: { $exists: true } })

// Type check
db.items.find({ price: { $type: "number" } })
db.items.find({ tags: { $type: "array" } })
```

### MG-107: Array operators for querying embedded arrays
```javascript
// Contains element
db.posts.find({ tags: "mongodb" })

// Contains all elements
db.posts.find({ tags: { $all: ["mongodb", "database"] } })

// Array size
db.posts.find({ tags: { $size: 3 } })

// Element match (for array of objects)
db.orders.find({
    items: {
        $elemMatch: {
            product: "laptop",
            quantity: { $gte: 2 }
        }
    }
})
```

### MG-108: Text indexes and full-text search
```javascript
// Create text index
db.articles.createIndex({ title: "text", content: "text" })

// Search
db.articles.find({ $text: { $search: "mongodb database" } })

// With score
db.articles.find(
    { $text: { $search: "mongodb" } },
    { score: { $meta: "textScore" } }
).sort({ score: { $meta: "textScore" } })
```

## Update Operators

### MG-109: Field modification operators
```javascript
// Set fields
db.users.updateOne(
    { _id: id },
    {
        $set: { name: "New Name" },        // Set value
        $unset: { tempField: "" },         // Remove field
        $rename: { oldName: "newName" },   // Rename field
        $setOnInsert: { createdAt: new Date() } // Only on insert
    }
)

// Increment
db.stats.updateOne(
    { userId: id },
    {
        $inc: { views: 1, score: 10 },     // Increment
        $mul: { price: 1.1 },              // Multiply
        $min: { lowScore: 50 },            // Set if less than
        $max: { highScore: 100 }           // Set if greater than
    }
)
```

### MG-110: Array modification operators
```javascript
db.posts.updateOne(
    { _id: id },
    {
        $push: { tags: "new-tag" },        // Add to array
        $addToSet: { tags: "unique-tag" }, // Add if not exists
        $pop: { tags: 1 },                 // Remove last (-1 for first)
        $pull: { tags: "old-tag" },        // Remove matching
        $pullAll: { tags: ["a", "b"] }     // Remove all matching
    }
)

// Push with modifiers
db.posts.updateOne(
    { _id: id },
    {
        $push: {
            comments: {
                $each: [{ text: "comment1" }, { text: "comment2" }],
                $sort: { createdAt: -1 },
                $slice: 10  // Keep only last 10
            }
        }
    }
)

// Update array element
db.posts.updateOne(
    { _id: id, "comments._id": commentId },
    { $set: { "comments.$.text": "updated text" } }
)

// Update all array elements
db.posts.updateOne(
    { _id: id },
    { $set: { "comments.$[].read": true } }
)

// Update with arrayFilters
db.posts.updateOne(
    { _id: id },
    { $set: { "comments.$[elem].approved": true } },
    { arrayFilters: [{ "elem.author": "admin" }] }
)
```

## Aggregation Pipeline

### MG-111: Core aggregation stages for data transformation
```javascript
db.orders.aggregate([
    // Match (filter)
    { $match: { status: "completed" } },

    // Project (select fields)
    { $project: { userId: 1, total: 1, date: "$createdAt" } },

    // Group
    { $group: {
        _id: "$userId",
        totalOrders: { $sum: 1 },
        totalSpent: { $sum: "$total" },
        avgOrder: { $avg: "$total" }
    }},

    // Sort
    { $sort: { totalSpent: -1 } },

    // Limit
    { $limit: 10 }
])
```

### MG-112: JOIN operations with $lookup
```javascript
db.orders.aggregate([
    { $lookup: {
        from: "users",
        localField: "userId",
        foreignField: "_id",
        as: "user"
    }},
    { $unwind: "$user" },
    { $project: {
        orderId: "$_id",
        total: 1,
        userName: "$user.name"
    }}
])

// Lookup with pipeline
db.orders.aggregate([
    { $lookup: {
        from: "users",
        let: { userId: "$userId" },
        pipeline: [
            { $match: { $expr: { $eq: ["$_id", "$$userId"] } } },
            { $project: { name: 1, email: 1 } }
        ],
        as: "user"
    }}
])
```

### MG-113: Flatten arrays with $unwind stage
```javascript
db.orders.aggregate([
    { $unwind: "$items" },
    { $group: {
        _id: "$items.productId",
        totalQuantity: { $sum: "$items.quantity" },
        totalRevenue: { $sum: { $multiply: ["$items.price", "$items.quantity"] } }
    }}
])

// Preserve null and empty arrays
{ $unwind: { path: "$items", preserveNullAndEmptyArrays: true } }
```

### MG-114: Grouping and aggregation functions
```javascript
db.orders.aggregate([
    { $group: {
        _id: { year: { $year: "$createdAt" }, month: { $month: "$createdAt" } },
        count: { $sum: 1 },
        total: { $sum: "$total" },
        avg: { $avg: "$total" },
        min: { $min: "$total" },
        max: { $max: "$total" },
        items: { $push: "$_id" },           // Array of all values
        uniqueUsers: { $addToSet: "$userId" }, // Unique values
        first: { $first: "$createdAt" },
        last: { $last: "$createdAt" }
    }}
])
```

### MG-115: Faceted search with multiple pipelines
```javascript
db.products.aggregate([
    { $facet: {
        "byCategory": [
            { $group: { _id: "$category", count: { $sum: 1 } } }
        ],
        "byPrice": [
            { $bucket: {
                groupBy: "$price",
                boundaries: [0, 100, 500, 1000],
                default: "expensive"
            }}
        ],
        "stats": [
            { $group: {
                _id: null,
                avgPrice: { $avg: "$price" },
                total: { $sum: 1 }
            }}
        ]
    }}
])
```

### MG-116: Window functions for running totals and analytics
```javascript
db.orders.aggregate([
    { $setWindowFields: {
        partitionBy: "$userId",
        sortBy: { createdAt: 1 },
        output: {
            runningTotal: {
                $sum: "$total",
                window: { documents: ["unbounded", "current"] }
            },
            orderRank: { $rank: {} },
            movingAvg: {
                $avg: "$total",
                window: { documents: [-2, 0] }
            }
        }
    }}
])
```

## Indexing

### MG-117: Create appropriate index types
```javascript
// Single field
db.users.createIndex({ email: 1 })

// Compound
db.orders.createIndex({ userId: 1, createdAt: -1 })

// Unique
db.users.createIndex({ email: 1 }, { unique: true })

// Partial
db.orders.createIndex(
    { status: 1 },
    { partialFilterExpression: { status: "pending" } }
)

// TTL (auto-delete)
db.sessions.createIndex({ createdAt: 1 }, { expireAfterSeconds: 3600 })

// Text
db.articles.createIndex({ title: "text", content: "text" })

// Geospatial
db.places.createIndex({ location: "2dsphere" })
```

### MG-118: Index lifecycle management
```javascript
// List indexes
db.users.getIndexes()

// Drop index
db.users.dropIndex("email_1")

// Hide index (test without dropping)
db.users.hideIndex("email_1")
db.users.unhideIndex("email_1")
```

## Transactions

### MG-119: Multi-document ACID transactions with sessions

```javascript
const session = client.startSession()
session.startTransaction()

try {
    db.accounts.updateOne(
        { _id: from },
        { $inc: { balance: -amount } },
        { session }
    )

    db.accounts.updateOne(
        { _id: to },
        { $inc: { balance: amount } },
        { session }
    )

    session.commitTransaction()
} catch (error) {
    session.abortTransaction()
    throw error
} finally {
    session.endSession()
}
```

## Change Streams

### MG-120: Monitor real-time collection changes

```javascript
const changeStream = db.orders.watch([
    { $match: { "fullDocument.status": "pending" } }
])

changeStream.on("change", (change) => {
    console.log(change.operationType)
    console.log(change.fullDocument)
})
```

## Common Patterns

### MG-121: Keyset-based pagination for efficiency
```javascript
// First page
db.orders.find({})
    .sort({ _id: -1 })
    .limit(10)

// Next page
db.orders.find({ _id: { $lt: lastId } })
    .sort({ _id: -1 })
    .limit(10)
```

### MG-122: Bulk write operations
```javascript
db.items.bulkWrite([
    { insertOne: { document: { name: "item1" } } },
    { updateOne: {
        filter: { _id: id1 },
        update: { $set: { status: "active" } }
    }},
    { deleteOne: { filter: { _id: id2 } } }
])
```

### MG-123: Atomic counter increments
```javascript
db.counters.findOneAndUpdate(
    { _id: "orderId" },
    { $inc: { seq: 1 } },
    { upsert: true, returnDocument: "after" }
)
```
