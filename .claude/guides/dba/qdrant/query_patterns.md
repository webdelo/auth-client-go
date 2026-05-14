# Qdrant Query Patterns

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| QD-002 | Create payload indexes for filtered searches | [Filtering](#filtering) |
| QD-004 | Batch upserts for bulk operations | [Points Operations](#points-operations) |
| QD-001 | Use gRPC client for better performance | [API Access](#api-access) |

---

## Overview

Qdrant is a vector similarity search engine. It stores vectors (embeddings) with optional payloads and enables fast similarity search.

## Collections

### QD-100: Create collection
```json
PUT /collections/my_collection
{
    "vectors": {
        "size": 384,
        "distance": "Cosine"
    }
}
```

### QD-101: Create with multiple vector types
```json
PUT /collections/documents
{
    "vectors": {
        "title": {
            "size": 384,
            "distance": "Cosine"
        },
        "content": {
            "size": 768,
            "distance": "Cosine"
        }
    }
}
```

### Distance metrics
- `Cosine` - Cosine similarity (normalized, recommended for most cases)
- `Euclid` - Euclidean distance
- `Dot` - Dot product (for non-normalized vectors)

### QD-102: Collection info
```json
GET /collections/my_collection
```

### QD-103: Delete collection
```json
DELETE /collections/my_collection
```

## Points (Vectors)

### QD-104: Upsert points
```json
PUT /collections/my_collection/points
{
    "points": [
        {
            "id": 1,
            "vector": [0.05, 0.61, 0.76, ...],
            "payload": {
                "title": "Document 1",
                "category": "tech",
                "created_at": "2024-01-15T10:30:00Z"
            }
        },
        {
            "id": "uuid-string-id",
            "vector": [0.19, 0.81, 0.23, ...],
            "payload": {
                "title": "Document 2",
                "category": "science"
            }
        }
    ]
}
```

### QD-105: Upsert with named vectors
```json
PUT /collections/documents/points
{
    "points": [
        {
            "id": 1,
            "vector": {
                "title": [0.05, 0.61, ...],
                "content": [0.19, 0.81, ...]
            },
            "payload": {
                "title": "Document 1"
            }
        }
    ]
}
```

### QD-106: Get points
```json
POST /collections/my_collection/points
{
    "ids": [1, 2, 3],
    "with_vector": true,
    "with_payload": true
}
```

### QD-107: Delete points
```json
POST /collections/my_collection/points/delete
{
    "points": [1, 2, 3]
}
```

### QD-108: Delete by filter
```json
POST /collections/my_collection/points/delete
{
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "deprecated" } }
        ]
    }
}
```

## Search

### QD-109: Basic search
```json
POST /collections/my_collection/points/search
{
    "vector": [0.2, 0.1, 0.9, ...],
    "limit": 10,
    "with_payload": true
}
```

### QD-110: Search with filter
```json
POST /collections/my_collection/points/search
{
    "vector": [0.2, 0.1, 0.9, ...],
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "tech" } }
        ]
    },
    "limit": 10,
    "with_payload": true,
    "with_vector": false
}
```

### QD-111: Search with named vector
```json
POST /collections/documents/points/search
{
    "vector": {
        "name": "title",
        "vector": [0.2, 0.1, 0.9, ...]
    },
    "limit": 10
}
```

### QD-112: Search with score threshold
```json
POST /collections/my_collection/points/search
{
    "vector": [0.2, 0.1, 0.9, ...],
    "score_threshold": 0.7,
    "limit": 10
}
```

### QD-113: Batch search
```json
POST /collections/my_collection/points/search/batch
{
    "searches": [
        {
            "vector": [0.2, 0.1, ...],
            "limit": 5
        },
        {
            "vector": [0.5, 0.3, ...],
            "limit": 5
        }
    ]
}
```

## Filters

### QD-114: Match (equality)
```json
{
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "tech" } }
        ]
    }
}
```

### QD-115: Match any (IN)
```json
{
    "filter": {
        "must": [
            { "key": "category", "match": { "any": ["tech", "science"] } }
        ]
    }
}
```

### QD-116: Range
```json
{
    "filter": {
        "must": [
            {
                "key": "price",
                "range": {
                    "gte": 100,
                    "lte": 500
                }
            }
        ]
    }
}
```

### QD-117: Geo filter
```json
{
    "filter": {
        "must": [
            {
                "key": "location",
                "geo_radius": {
                    "center": { "lat": 52.52, "lon": 13.405 },
                    "radius": 10000
                }
            }
        ]
    }
}
```

### QD-118: Nested filter
```json
{
    "filter": {
        "must": [
            {
                "key": "metadata.author",
                "match": { "value": "John" }
            }
        ]
    }
}
```

### QD-119: Complex filter
```json
{
    "filter": {
        "must": [
            { "key": "status", "match": { "value": "active" } }
        ],
        "should": [
            { "key": "category", "match": { "value": "tech" } },
            { "key": "category", "match": { "value": "science" } }
        ],
        "must_not": [
            { "key": "deleted", "match": { "value": true } }
        ]
    }
}
```

### QD-120: Has ID filter
```json
{
    "filter": {
        "must": [
            { "has_id": [1, 2, 3, 4, 5] }
        ]
    }
}
```

### QD-121: Is empty / Is null
```json
{
    "filter": {
        "must": [
            { "is_empty": { "key": "tags" } }
        ],
        "must_not": [
            { "is_null": { "key": "description" } }
        ]
    }
}
```

## Recommend

### QD-122: Recommend by positive examples
```json
POST /collections/my_collection/points/recommend
{
    "positive": [1, 2, 3],
    "limit": 10
}
```

### QD-123: Recommend with negative examples
```json
POST /collections/my_collection/points/recommend
{
    "positive": [1, 2],
    "negative": [10, 11],
    "limit": 10
}
```

### QD-124: Recommend with filter
```json
POST /collections/my_collection/points/recommend
{
    "positive": [1, 2],
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "tech" } }
        ]
    },
    "limit": 10
}
```

## Scroll (Pagination)

### QD-125: Scroll all points
```json
POST /collections/my_collection/points/scroll
{
    "limit": 100,
    "with_payload": true,
    "with_vector": false
}
```

### QD-126: Scroll with offset
```json
POST /collections/my_collection/points/scroll
{
    "offset": "last-point-id",
    "limit": 100
}
```

### QD-127: Scroll with filter
```json
POST /collections/my_collection/points/scroll
{
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "tech" } }
        ]
    },
    "limit": 100
}
```

## Payload Management

### QD-128: Update payload
```json
POST /collections/my_collection/points/payload
{
    "payload": {
        "new_field": "new_value"
    },
    "points": [1, 2, 3]
}
```

### QD-129: Delete payload keys
```json
POST /collections/my_collection/points/payload/delete
{
    "keys": ["old_field"],
    "points": [1, 2, 3]
}
```

### QD-130: Clear payload
```json
POST /collections/my_collection/points/payload/clear
{
    "points": [1, 2, 3]
}
```

## Indexing

### QD-131: Create payload index
```json
PUT /collections/my_collection/index
{
    "field_name": "category",
    "field_schema": "keyword"
}
```

### Index types
- `keyword` - For exact match filtering
- `integer` - For integer range filtering
- `float` - For float range filtering
- `geo` - For geo filtering
- `text` - For full-text search

### QD-132: Full-text index
```json
PUT /collections/my_collection/index
{
    "field_name": "description",
    "field_schema": {
        "type": "text",
        "tokenizer": "word",
        "min_token_len": 2,
        "max_token_len": 15,
        "lowercase": true
    }
}
```

## Snapshots

### QD-133: Create snapshot
```json
POST /collections/my_collection/snapshots
```

### QD-134: List snapshots
```json
GET /collections/my_collection/snapshots
```

### QD-135: Recover from snapshot
```json
PUT /collections/my_collection/snapshots/recover
{
    "location": "http://storage/snapshot.tar"
}
```

## Cluster Operations

### QD-136: Collection cluster info
```json
GET /collections/my_collection/cluster
```

### QD-137: Update collection cluster
```json
POST /collections/my_collection/cluster
{
    "move_shard": {
        "shard_id": 0,
        "from_peer_id": 1,
        "to_peer_id": 2
    }
}
```

## Quantization

### QD-138: Create collection with quantization
```json
PUT /collections/my_collection
{
    "vectors": {
        "size": 384,
        "distance": "Cosine"
    },
    "quantization_config": {
        "scalar": {
            "type": "int8",
            "always_ram": true
        }
    }
}
```

### QD-139: Binary quantization (for faster search)
```json
{
    "quantization_config": {
        "binary": {
            "always_ram": true
        }
    }
}
```

### QD-140: Product quantization
```json
{
    "quantization_config": {
        "product": {
            "compression": "x16",
            "always_ram": true
        }
    }
}
```

## Performance Tips

### Use filters before search
```json
{
    "filter": { ... },  // Narrow down first
    "vector": [...],    // Then search
    "limit": 10
}
```

### Create payload indexes
```json
// Index frequently filtered fields
PUT /collections/my_collection/index
{
    "field_name": "category",
    "field_schema": "keyword"
}
```

### Use quantization for large collections
```json
// Reduces memory usage, speeds up search
{
    "quantization_config": {
        "scalar": { "type": "int8" }
    }
}
```

### Batch operations
```json
// Upsert in batches of 100-1000 points
PUT /collections/my_collection/points
{
    "points": [/* 100-1000 points */]
}
```
