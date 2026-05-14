# Qdrant Indexing Guide

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| QD-005 | Set appropriate HNSW parameters (m, ef) | [HNSW Configuration](#hnsw-hierarchical-navigable-small-world) |
| QD-002 | Create payload indexes for filtered searches | [Payload Indexes](#payload-indexes) |
| QD-003 | Use quantization for large collections | [Quantization](#quantization) |

---

## Vector Indexes

### QD-300: HNSW (Hierarchical Navigable Small World)

Default vector index in Qdrant. Provides approximate nearest neighbor search.

```json
// HNSW configuration
PUT /collections/my_collection
{
    "vectors": {
        "size": 768,
        "distance": "Cosine"
    },
    "hnsw_config": {
        "m": 16,               // Connections per node (default: 16)
        "ef_construct": 100,   // Build-time quality (default: 100)
        "full_scan_threshold": 10000,  // Threshold for brute force
        "max_indexing_threads": 0,     // 0 = auto
        "on_disk": false       // Keep index in RAM
    }
}
```

### QD-301: HNSW Parameters Explained

| Parameter | Purpose | Trade-off |
|-----------|---------|-----------|
| m | Max edges per node | Higher = better recall, more memory |
| ef_construct | Build quality | Higher = better index, slower build |
| ef (search) | Search quality | Higher = better recall, slower search |

### QD-302: Choosing m Value

```json
// Small dataset (<100K vectors)
{ "m": 8 }

// Medium dataset (100K - 1M vectors)
{ "m": 16 }  // Default

// Large dataset (>1M vectors)
{ "m": 32 }

// Very high accuracy required
{ "m": 64 }
```

### QD-303: ef_construct vs ef (search)

```json
// Build time: ef_construct (one-time cost)
{
    "hnsw_config": {
        "ef_construct": 100  // Higher = better graph, slower build
    }
}

// Search time: ef (per-query cost)
POST /collections/my_collection/points/search
{
    "vector": [...],
    "params": {
        "hnsw_ef": 128  // Higher = better recall, slower search
    }
}
```

## Payload Indexes

### QD-304: Index Types

```json
// Keyword index (exact match)
PUT /collections/products/index
{
    "field_name": "category",
    "field_schema": "keyword"
}

// Integer index (range queries)
PUT /collections/products/index
{
    "field_name": "price",
    "field_schema": "integer"
}

// Float index
PUT /collections/products/index
{
    "field_name": "rating",
    "field_schema": "float"
}

// Boolean index
PUT /collections/products/index
{
    "field_name": "in_stock",
    "field_schema": "bool"
}

// Geo index
PUT /collections/locations/index
{
    "field_name": "coordinates",
    "field_schema": "geo"
}

// Datetime index
PUT /collections/events/index
{
    "field_name": "created_at",
    "field_schema": "datetime"
}

// UUID index (optimized for UUID format)
PUT /collections/users/index
{
    "field_name": "user_id",
    "field_schema": "uuid"
}
```

### QD-305: Full-Text Index

```json
// Text index for full-text search
PUT /collections/articles/index
{
    "field_name": "content",
    "field_schema": {
        "type": "text",
        "tokenizer": "word",      // word, whitespace, prefix, multilingual
        "min_token_len": 2,
        "max_token_len": 20,
        "lowercase": true
    }
}
```

### QD-306: Tokenizer Options

| Tokenizer | Description | Use Case |
|-----------|-------------|----------|
| word | Splits on non-alphanumeric | General text |
| whitespace | Splits on whitespace only | Preserve special chars |
| prefix | Creates prefix tokens | Autocomplete |
| multilingual | Language-aware | International content |

## Filtered Search Optimization

### QD-307: Pre-filtering vs Post-filtering

```json
// Pre-filtering (efficient - filters during HNSW traversal)
POST /collections/products/points/search
{
    "vector": [...],
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "electronics" } }
        ]
    },
    "limit": 10
}
```

### QD-308: Complex Filters

```json
// AND conditions
{
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "electronics" } },
            { "key": "price", "range": { "gte": 100, "lte": 500 } },
            { "key": "in_stock", "match": { "value": true } }
        ]
    }
}

// OR conditions
{
    "filter": {
        "should": [
            { "key": "brand", "match": { "value": "Apple" } },
            { "key": "brand", "match": { "value": "Samsung" } }
        ]
    }
}

// NOT conditions
{
    "filter": {
        "must_not": [
            { "key": "status", "match": { "value": "discontinued" } }
        ]
    }
}

// Nested conditions
{
    "filter": {
        "must": [
            {
                "should": [
                    { "key": "category", "match": { "value": "phones" } },
                    { "key": "category", "match": { "value": "tablets" } }
                ]
            },
            { "key": "price", "range": { "lte": 1000 } }
        ]
    }
}
```

### QD-309: Geo Filtering

```json
// Index geo field
PUT /collections/stores/index
{
    "field_name": "location",
    "field_schema": "geo"
}

// Search within radius
POST /collections/stores/points/search
{
    "vector": [...],
    "filter": {
        "must": [
            {
                "key": "location",
                "geo_radius": {
                    "center": { "lat": 40.7128, "lon": -74.0060 },
                    "radius": 5000  // meters
                }
            }
        ]
    }
}

// Search within bounding box
{
    "filter": {
        "must": [
            {
                "key": "location",
                "geo_bounding_box": {
                    "top_left": { "lat": 41.0, "lon": -75.0 },
                    "bottom_right": { "lat": 40.0, "lon": -73.0 }
                }
            }
        ]
    }
}
```

## Go Integration

### QD-310: Creating Indexes

```go
func (r *QdrantRepo) CreatePayloadIndexes(ctx context.Context) error {
    indexes := []struct {
        Field  string
        Schema qdrant.PayloadSchemaType
    }{
        {"category", qdrant.PayloadSchemaType_Keyword},
        {"price", qdrant.PayloadSchemaType_Float},
        {"in_stock", qdrant.PayloadSchemaType_Bool},
    }

    for _, idx := range indexes {
        _, err := r.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
            CollectionName: "products",
            FieldName:      idx.Field,
            FieldType:      &idx.Schema,
        })
        if err != nil {
            return fmt.Errorf("create index %s: %w", idx.Field, err)
        }
    }

    return nil
}
```

### QD-311: Building Filters

```go
func BuildProductFilter(category string, minPrice, maxPrice float64, inStock bool) *qdrant.Filter {
    conditions := []*qdrant.Condition{}

    if category != "" {
        conditions = append(conditions, &qdrant.Condition{
            ConditionOneOf: &qdrant.Condition_Field{
                Field: &qdrant.FieldCondition{
                    Key: "category",
                    Match: &qdrant.Match{
                        MatchValue: &qdrant.Match_Keyword{Keyword: category},
                    },
                },
            },
        })
    }

    if minPrice > 0 || maxPrice > 0 {
        rangeCondition := &qdrant.Range{}
        if minPrice > 0 {
            rangeCondition.Gte = &minPrice
        }
        if maxPrice > 0 {
            rangeCondition.Lte = &maxPrice
        }
        conditions = append(conditions, &qdrant.Condition{
            ConditionOneOf: &qdrant.Condition_Field{
                Field: &qdrant.FieldCondition{
                    Key:   "price",
                    Range: rangeCondition,
                },
            },
        })
    }

    if inStock {
        conditions = append(conditions, &qdrant.Condition{
            ConditionOneOf: &qdrant.Condition_Field{
                Field: &qdrant.FieldCondition{
                    Key: "in_stock",
                    Match: &qdrant.Match{
                        MatchValue: &qdrant.Match_Boolean{Boolean: true},
                    },
                },
            },
        })
    }

    return &qdrant.Filter{Must: conditions}
}
```

### QD-312: Full-Text Search

```go
func (r *QdrantRepo) FullTextSearch(ctx context.Context, query string, vector []float32) ([]*qdrant.ScoredPoint, error) {
    filter := &qdrant.Filter{
        Must: []*qdrant.Condition{
            {
                ConditionOneOf: &qdrant.Condition_Field{
                    Field: &qdrant.FieldCondition{
                        Key: "content",
                        Match: &qdrant.Match{
                            MatchValue: &qdrant.Match_Text{Text: query},
                        },
                    },
                },
            },
        },
    }

    return r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: "articles",
        Query:          qdrant.NewQuery(vector...),
        Filter:         filter,
        Limit:          qdrant.PtrOf(uint64(10)),
    })
}
```

## Index Maintenance

### QD-313: Optimizer Status

```bash
# Check optimizer status
GET /collections/my_collection

# Response includes:
# "optimizer_status": {
#     "ok": true
# }
```

### QD-314: Force Optimization

```bash
# Trigger optimizer (merge segments)
POST /collections/my_collection/index

# Wait for indexing to complete
PUT /collections/my_collection/index?wait=true
```

### QD-315: Index Statistics

```go
func (r *QdrantRepo) GetIndexStats(ctx context.Context) error {
    info, err := r.client.GetCollectionInfo(ctx, "my_collection")
    if err != nil {
        return err
    }

    log.Printf("Points count: %d", info.PointsCount)
    log.Printf("Indexed vectors: %d", info.IndexedVectorsCount)
    log.Printf("Segments: %d", len(info.Segments))

    for name, index := range info.PayloadSchema {
        log.Printf("Payload index: %s - %v", name, index)
    }

    return nil
}
```

## Best Practices

1. **Create payload indexes** for filtered fields
2. **Use appropriate HNSW m value** based on dataset size
3. **Higher ef_construct** for static datasets
4. **Adjust ef at search time** for recall/speed trade-off
5. **Use full-text index** for text search, not keyword
6. **Geo index** required for location-based filtering
7. **Monitor optimizer status** after bulk inserts
8. **Test recall** at different ef values for your use case
9. **Use keyword index** for exact match, not text
10. **Index only filterable fields** to save memory
