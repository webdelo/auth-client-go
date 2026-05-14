# Qdrant + Go Integration

## Quick Reference

| ID | Rule | Section |
|----|------|---------|
| QD-001 | Use gRPC client for better performance | [Client Setup](#client-setup) |
| QD-002 | Create payload indexes for filtered searches | [Index Management](#index-management) |
| QD-003 | Use quantization for large collections | [Collection Management](#collection-management) |
| QD-004 | Batch upserts for bulk operations | [Best Practices](#best-practices) |
| QD-005 | Set appropriate HNSW parameters (m, ef) | [Collection Management](#collection-management) |

---

## Client Setup

### QD-001: Official Go client
```go
import (
    "github.com/qdrant/go-client/qdrant"
)
```

### QD-002: Connection
```go
package qdrant

import (
    "context"

    "github.com/qdrant/go-client/qdrant"
)

type Config struct {
    Host   string
    Port   int
    APIKey string
    UseTLS bool
}

func NewClient(cfg Config) (*qdrant.Client, error) {
    var opts []qdrant.Option

    if cfg.APIKey != "" {
        opts = append(opts, qdrant.WithAPIKey(cfg.APIKey))
    }

    if cfg.UseTLS {
        opts = append(opts, qdrant.WithTLS())
    }

    client, err := qdrant.NewClient(&qdrant.Config{
        Host: cfg.Host,
        Port: cfg.Port,
    }, opts...)
    if err != nil {
        return nil, err
    }

    return client, nil
}
```

## Collection Management

### QD-003: Create collection
```go
func (r *Repository) CreateCollection(ctx context.Context, name string, vectorSize uint64) error {
    return r.client.CreateCollection(ctx, &qdrant.CreateCollection{
        CollectionName: name,
        VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
            Size:     vectorSize,
            Distance: qdrant.Distance_Cosine,
        }),
    })
}
```

### QD-004: Create with multiple vectors
```go
func (r *Repository) CreateMultiVectorCollection(ctx context.Context, name string) error {
    return r.client.CreateCollection(ctx, &qdrant.CreateCollection{
        CollectionName: name,
        VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
            "title": {
                Size:     384,
                Distance: qdrant.Distance_Cosine,
            },
            "content": {
                Size:     768,
                Distance: qdrant.Distance_Cosine,
            },
        }),
    })
}
```

### QD-005: Create with quantization
```go
func (r *Repository) CreateOptimizedCollection(ctx context.Context, name string, vectorSize uint64) error {
    return r.client.CreateCollection(ctx, &qdrant.CreateCollection{
        CollectionName: name,
        VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
            Size:     vectorSize,
            Distance: qdrant.Distance_Cosine,
        }),
        QuantizationConfig: qdrant.NewQuantizationScalar(
            qdrant.QuantizationType_Int8,
            qdrant.WithAlwaysRam(true),
        ),
    })
}
```

### QD-006: Delete collection
```go
func (r *Repository) DeleteCollection(ctx context.Context, name string) error {
    return r.client.DeleteCollection(ctx, name)
}
```

### QD-007: Collection info
```go
func (r *Repository) GetCollectionInfo(ctx context.Context, name string) (*qdrant.CollectionInfo, error) {
    return r.client.GetCollectionInfo(ctx, name)
}
```

## Points (Vectors)

### QD-008: Upsert points
```go
type Document struct {
    ID       string
    Vector   []float32
    Title    string
    Category string
    Tags     []string
}

func (r *Repository) UpsertDocuments(ctx context.Context, docs []Document) error {
    points := make([]*qdrant.PointStruct, len(docs))

    for i, doc := range docs {
        points[i] = &qdrant.PointStruct{
            Id:      qdrant.NewIDString(doc.ID),
            Vectors: qdrant.NewVectors(doc.Vector...),
            Payload: qdrant.NewValueMap(map[string]any{
                "title":    doc.Title,
                "category": doc.Category,
                "tags":     doc.Tags,
            }),
        }
    }

    _, err := r.client.Upsert(ctx, &qdrant.UpsertPoints{
        CollectionName: r.collectionName,
        Points:         points,
    })

    return err
}
```

### QD-009: Upsert with named vectors
```go
func (r *Repository) UpsertWithNamedVectors(ctx context.Context, id string, titleVec, contentVec []float32) error {
    _, err := r.client.Upsert(ctx, &qdrant.UpsertPoints{
        CollectionName: r.collectionName,
        Points: []*qdrant.PointStruct{
            {
                Id: qdrant.NewIDString(id),
                Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
                    "title":   qdrant.NewVector(titleVec...),
                    "content": qdrant.NewVector(contentVec...),
                }),
            },
        },
    })

    return err
}
```

### QD-010: Get points
```go
func (r *Repository) GetDocuments(ctx context.Context, ids []string) ([]*qdrant.RetrievedPoint, error) {
    pointIDs := make([]*qdrant.PointId, len(ids))
    for i, id := range ids {
        pointIDs[i] = qdrant.NewIDString(id)
    }

    return r.client.Get(ctx, &qdrant.GetPoints{
        CollectionName: r.collectionName,
        Ids:            pointIDs,
        WithPayload:    qdrant.NewWithPayload(true),
        WithVectors:    qdrant.NewWithVectors(true),
    })
}
```

### QD-011: Delete points
```go
func (r *Repository) DeleteDocuments(ctx context.Context, ids []string) error {
    pointIDs := make([]*qdrant.PointId, len(ids))
    for i, id := range ids {
        pointIDs[i] = qdrant.NewIDString(id)
    }

    _, err := r.client.Delete(ctx, &qdrant.DeletePoints{
        CollectionName: r.collectionName,
        Points:         qdrant.NewPointsSelector(pointIDs...),
    })

    return err
}
```

### QD-012: Delete by filter
```go
func (r *Repository) DeleteByCategory(ctx context.Context, category string) error {
    _, err := r.client.Delete(ctx, &qdrant.DeletePoints{
        CollectionName: r.collectionName,
        Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
            Must: []*qdrant.Condition{
                qdrant.NewMatch("category", category),
            },
        }),
    })

    return err
}
```

## Search

### QD-013: Basic search
```go
type SearchResult struct {
    ID       string
    Score    float32
    Title    string
    Category string
}

func (r *Repository) Search(ctx context.Context, vector []float32, limit uint64) ([]SearchResult, error) {
    results, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: r.collectionName,
        Query:          qdrant.NewQuery(vector...),
        Limit:          qdrant.PtrOf(limit),
        WithPayload:    qdrant.NewWithPayload(true),
    })
    if err != nil {
        return nil, err
    }

    searchResults := make([]SearchResult, len(results))
    for i, point := range results {
        searchResults[i] = SearchResult{
            ID:       point.Id.GetUuid(),
            Score:    point.Score,
            Title:    point.Payload["title"].GetStringValue(),
            Category: point.Payload["category"].GetStringValue(),
        }
    }

    return searchResults, nil
}
```

### QD-014: Search with filter
```go
func (r *Repository) SearchInCategory(ctx context.Context, vector []float32, category string, limit uint64) ([]SearchResult, error) {
    results, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: r.collectionName,
        Query:          qdrant.NewQuery(vector...),
        Filter: &qdrant.Filter{
            Must: []*qdrant.Condition{
                qdrant.NewMatch("category", category),
            },
        },
        Limit:       qdrant.PtrOf(limit),
        WithPayload: qdrant.NewWithPayload(true),
    })
    if err != nil {
        return nil, err
    }

    return parseResults(results), nil
}
```

### QD-015: Search with score threshold
```go
func (r *Repository) SearchWithThreshold(ctx context.Context, vector []float32, threshold float32, limit uint64) ([]SearchResult, error) {
    results, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: r.collectionName,
        Query:          qdrant.NewQuery(vector...),
        ScoreThreshold: qdrant.PtrOf(threshold),
        Limit:          qdrant.PtrOf(limit),
        WithPayload:    qdrant.NewWithPayload(true),
    })
    if err != nil {
        return nil, err
    }

    return parseResults(results), nil
}
```

### QD-016: Search with named vector
```go
func (r *Repository) SearchByTitle(ctx context.Context, titleVector []float32, limit uint64) ([]SearchResult, error) {
    results, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: r.collectionName,
        Query:          qdrant.NewQueryNamed("title", titleVector...),
        Limit:          qdrant.PtrOf(limit),
        WithPayload:    qdrant.NewWithPayload(true),
    })
    if err != nil {
        return nil, err
    }

    return parseResults(results), nil
}
```

### QD-017: Batch search
```go
func (r *Repository) BatchSearch(ctx context.Context, vectors [][]float32, limit uint64) ([][]SearchResult, error) {
    searches := make([]*qdrant.QueryPoints, len(vectors))
    for i, vec := range vectors {
        searches[i] = &qdrant.QueryPoints{
            CollectionName: r.collectionName,
            Query:          qdrant.NewQuery(vec...),
            Limit:          qdrant.PtrOf(limit),
            WithPayload:    qdrant.NewWithPayload(true),
        }
    }

    results, err := r.client.QueryBatch(ctx, r.collectionName, searches)
    if err != nil {
        return nil, err
    }

    batchResults := make([][]SearchResult, len(results))
    for i, batch := range results {
        batchResults[i] = parseResults(batch.Result)
    }

    return batchResults, nil
}
```

## Filters

### QD-018: Build complex filter
```go
func buildFilter(category string, minPrice, maxPrice float64, tags []string) *qdrant.Filter {
    conditions := []*qdrant.Condition{}

    if category != "" {
        conditions = append(conditions, qdrant.NewMatch("category", category))
    }

    if minPrice > 0 || maxPrice > 0 {
        rangeCondition := &qdrant.Range{}
        if minPrice > 0 {
            rangeCondition.Gte = qdrant.PtrOf(minPrice)
        }
        if maxPrice > 0 {
            rangeCondition.Lte = qdrant.PtrOf(maxPrice)
        }
        conditions = append(conditions, qdrant.NewRange("price", rangeCondition))
    }

    if len(tags) > 0 {
        tagConditions := make([]*qdrant.Condition, len(tags))
        for i, tag := range tags {
            tagConditions[i] = qdrant.NewMatch("tags", tag)
        }
        conditions = append(conditions, &qdrant.Condition{
            ConditionOneOf: &qdrant.Condition_Filter{
                Filter: &qdrant.Filter{
                    Should: tagConditions,
                },
            },
        })
    }

    if len(conditions) == 0 {
        return nil
    }

    return &qdrant.Filter{
        Must: conditions,
    }
}
```

### QD-019: Search with complex filter
```go
func (r *Repository) SearchWithFilters(ctx context.Context, vector []float32, filter SearchFilter) ([]SearchResult, error) {
    qdrantFilter := buildFilter(filter.Category, filter.MinPrice, filter.MaxPrice, filter.Tags)

    results, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: r.collectionName,
        Query:          qdrant.NewQuery(vector...),
        Filter:         qdrantFilter,
        Limit:          qdrant.PtrOf(filter.Limit),
        WithPayload:    qdrant.NewWithPayload(true),
    })

    return parseResults(results), err
}
```

## Recommend

### QD-020: Recommend by positive examples
```go
func (r *Repository) Recommend(ctx context.Context, positiveIDs []string, limit uint64) ([]SearchResult, error) {
    positive := make([]*qdrant.PointId, len(positiveIDs))
    for i, id := range positiveIDs {
        positive[i] = qdrant.NewIDString(id)
    }

    results, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: r.collectionName,
        Query:          qdrant.NewQueryRecommend(positive, nil),
        Limit:          qdrant.PtrOf(limit),
        WithPayload:    qdrant.NewWithPayload(true),
    })

    return parseResults(results), err
}
```

### QD-021: Recommend with negative examples
```go
func (r *Repository) RecommendWithNegative(ctx context.Context, positiveIDs, negativeIDs []string, limit uint64) ([]SearchResult, error) {
    positive := make([]*qdrant.PointId, len(positiveIDs))
    for i, id := range positiveIDs {
        positive[i] = qdrant.NewIDString(id)
    }

    negative := make([]*qdrant.PointId, len(negativeIDs))
    for i, id := range negativeIDs {
        negative[i] = qdrant.NewIDString(id)
    }

    results, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: r.collectionName,
        Query:          qdrant.NewQueryRecommend(positive, negative),
        Limit:          qdrant.PtrOf(limit),
        WithPayload:    qdrant.NewWithPayload(true),
    })

    return parseResults(results), err
}
```

## Scroll (Pagination)

### QD-022: Scroll all points
```go
func (r *Repository) ScrollAll(ctx context.Context, limit uint32) ([]*qdrant.RetrievedPoint, error) {
    var allPoints []*qdrant.RetrievedPoint
    var offset *qdrant.PointId

    for {
        result, err := r.client.Scroll(ctx, &qdrant.ScrollPoints{
            CollectionName: r.collectionName,
            Limit:          qdrant.PtrOf(limit),
            Offset:         offset,
            WithPayload:    qdrant.NewWithPayload(true),
        })
        if err != nil {
            return nil, err
        }

        allPoints = append(allPoints, result.Result...)

        if result.NextPageOffset == nil {
            break
        }
        offset = result.NextPageOffset
    }

    return allPoints, nil
}
```

### QD-023: Scroll with filter
```go
func (r *Repository) ScrollByCategory(ctx context.Context, category string, limit uint32) ([]*qdrant.RetrievedPoint, error) {
    result, err := r.client.Scroll(ctx, &qdrant.ScrollPoints{
        CollectionName: r.collectionName,
        Filter: &qdrant.Filter{
            Must: []*qdrant.Condition{
                qdrant.NewMatch("category", category),
            },
        },
        Limit:       qdrant.PtrOf(limit),
        WithPayload: qdrant.NewWithPayload(true),
    })

    if err != nil {
        return nil, err
    }

    return result.Result, nil
}
```

## Payload Management

### QD-024: Update payload
```go
func (r *Repository) UpdatePayload(ctx context.Context, id string, payload map[string]any) error {
    _, err := r.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
        CollectionName: r.collectionName,
        Payload:        qdrant.NewValueMap(payload),
        PointsSelector: qdrant.NewPointsSelector(qdrant.NewIDString(id)),
    })

    return err
}
```

### QD-025: Delete payload keys
```go
func (r *Repository) DeletePayloadKeys(ctx context.Context, id string, keys []string) error {
    _, err := r.client.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
        CollectionName: r.collectionName,
        Keys:           keys,
        PointsSelector: qdrant.NewPointsSelector(qdrant.NewIDString(id)),
    })

    return err
}
```

## Index Management

### QD-026: Create payload index
```go
func (r *Repository) CreateIndex(ctx context.Context, field string, schemaType qdrant.PayloadSchemaType) error {
    _, err := r.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
        CollectionName: r.collectionName,
        FieldName:      field,
        FieldType:      qdrant.PtrOf(schemaType),
    })

    return err
}

// Usage
r.CreateIndex(ctx, "category", qdrant.PayloadSchemaType_Keyword)
r.CreateIndex(ctx, "price", qdrant.PayloadSchemaType_Float)
```

## Error Handling

### QD-027: Error handling pattern

```go
func handleQdrantError(err error) error {
    if err == nil {
        return nil
    }

    // Check for specific app_error types
    if strings.Contains(err.Error(), "collection not found") {
        return ErrCollectionNotFound
    }

    if strings.Contains(err.Error(), "point not found") {
        return ErrPointNotFound
    }

    return fmt.Errorf("qdrant app_error: %w", err)
}
```

## Testing

### QD-028: Test setup

```go
func setupTestQdrant(t *testing.T) *qdrant.Client {
    t.Helper()

    client, err := qdrant.NewClient(&qdrant.Config{
        Host: "localhost",
        Port: 6334,
    })
    require.NoError(t, err)

    // Create test collection
    collectionName := fmt.Sprintf("test_%d", time.Now().UnixNano())
    err = client.CreateCollection(context.Background(), &qdrant.CreateCollection{
        CollectionName: collectionName,
        VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
            Size:     384,
            Distance: qdrant.Distance_Cosine,
        }),
    })
    require.NoError(t, err)

    t.Cleanup(func() {
        client.DeleteCollection(context.Background(), collectionName)
    })

    return client
}
```

## Best Practices

### QD-029: Batch upserts
```go
// Upsert in batches of 100-1000 for best performance
const batchSize = 100

func (r *Repository) BulkUpsert(ctx context.Context, docs []Document) error {
    for i := 0; i < len(docs); i += batchSize {
        end := i + batchSize
        if end > len(docs) {
            end = len(docs)
        }

        if err := r.UpsertDocuments(ctx, docs[i:end]); err != nil {
            return err
        }
    }

    return nil
}
```

### QD-030: Reuse client
```go
// Create client once, reuse for all operations
var client *qdrant.Client

func init() {
    var err error
    client, err = qdrant.NewClient(&qdrant.Config{
        Host: "localhost",
        Port: 6334,
    })
    if err != nil {
        log.Fatal(err)
    }
}
```
