# Qdrant Optimization Guide

## Quick Reference

| ID | Rule | Applied In |
|----|------|------------|
| QD-003 | Use quantization for large collections | [Quantization](#quantization) |
| QD-005 | Set appropriate HNSW parameters (m, ef) | [HNSW Configuration](#collection-configuration) |
| QD-002 | Create payload indexes for filtered searches | [Payload Indexes](#payload-indexes) |

---

## Collection Configuration

### QD-200: Optimal Vector Configuration

```json
// Create collection with optimized settings
PUT /collections/my_collection
{
    "vectors": {
        "size": 768,                    // Vector dimension
        "distance": "Cosine",           // Cosine, Dot, Euclid
        "on_disk": false                // Keep in RAM for speed
    },
    "optimizers_config": {
        "indexing_threshold": 20000,    // Build HNSW after N vectors
        "memmap_threshold": 50000       // Use mmap after N vectors
    },
    "hnsw_config": {
        "m": 16,                         // Max connections per node
        "ef_construct": 100,             // Construction quality
        "full_scan_threshold": 10000     // Use full scan for small segments
    }
}
```

### QD-201: Distance Metrics

| Metric | Use Case | Speed |
|--------|----------|-------|
| Cosine | Text embeddings, normalized vectors | Fast |
| Dot | When magnitude matters | Fastest |
| Euclid | Spatial data, image embeddings | Medium |

```json
// Choose based on embedding model
{
    "vectors": {
        "size": 1536,
        "distance": "Cosine"  // OpenAI embeddings
    }
}
```

### QD-202: Multi-Vector Collections

```json
// Named vectors for different embedding types
PUT /collections/products
{
    "vectors": {
        "title": {
            "size": 384,
            "distance": "Cosine"
        },
        "description": {
            "size": 768,
            "distance": "Cosine"
        },
        "image": {
            "size": 512,
            "distance": "Euclid"
        }
    }
}
```

## HNSW Index Tuning

### QD-203: Parameters

| Parameter | Default | Effect |
|-----------|---------|--------|
| m | 16 | Higher = more accurate, more memory |
| ef_construct | 100 | Higher = better index, slower build |
| ef | 128 | Higher = more accurate search |

### QD-204: Tuning for Accuracy

```json
// High accuracy (slower)
{
    "hnsw_config": {
        "m": 32,
        "ef_construct": 200
    }
}

// Search with high ef
POST /collections/my_collection/points/search
{
    "vector": [...],
    "limit": 10,
    "params": {
        "hnsw_ef": 256  // Higher = more accurate
    }
}
```

### QD-205: Tuning for Speed

```json
// Fast search (less accurate)
{
    "hnsw_config": {
        "m": 8,
        "ef_construct": 50
    }
}

// Search params
{
    "params": {
        "hnsw_ef": 64
    }
}
```

## Memory Management

### QD-206: On-Disk vs In-Memory

```json
// Vectors on disk (larger datasets)
PUT /collections/large_collection
{
    "vectors": {
        "size": 768,
        "distance": "Cosine",
        "on_disk": true
    },
    "optimizers_config": {
        "memmap_threshold": 10000
    }
}

// Payload on disk
{
    "on_disk_payload": true
}
```

### QD-207: Quantization (Reduce Memory)

```json
// Scalar quantization (4x memory reduction)
PUT /collections/my_collection
{
    "vectors": {
        "size": 768,
        "distance": "Cosine"
    },
    "quantization_config": {
        "scalar": {
            "type": "int8",
            "quantile": 0.99,
            "always_ram": true
        }
    }
}

// Binary quantization (32x reduction, less accurate)
{
    "quantization_config": {
        "binary": {
            "always_ram": true
        }
    }
}

// Product quantization (best compression)
{
    "quantization_config": {
        "product": {
            "compression": "x16",
            "always_ram": true
        }
    }
}
```

### QD-208: Search with Quantization

```json
// Use quantization for speed, rescore for accuracy
POST /collections/my_collection/points/search
{
    "vector": [...],
    "limit": 10,
    "params": {
        "quantization": {
            "ignore": false,
            "rescore": true,     // Re-rank with full vectors
            "oversampling": 2.0  // Fetch 2x candidates
        }
    }
}
```

## Query Optimization

### QD-209: Filtered Search

```json
// Filters applied DURING search (efficient)
POST /collections/products/points/search
{
    "vector": [...],
    "limit": 10,
    "filter": {
        "must": [
            { "key": "category", "match": { "value": "electronics" } },
            { "key": "price", "range": { "lte": 1000 } }
        ]
    }
}
```

### QD-210: Payload Indexes

```json
// Create indexes for filter fields
PUT /collections/products/index
{
    "field_name": "category",
    "field_schema": "keyword"
}

PUT /collections/products/index
{
    "field_name": "price",
    "field_schema": "float"
}

// Full-text index
PUT /collections/products/index
{
    "field_name": "description",
    "field_schema": {
        "type": "text",
        "tokenizer": "word",
        "min_token_len": 2,
        "max_token_len": 20,
        "lowercase": true
    }
}
```

### QD-211: Batch Operations

```json
// Batch upsert (much faster than individual)
PUT /collections/my_collection/points
{
    "batch": {
        "ids": [1, 2, 3],
        "vectors": [[...], [...], [...]],
        "payloads": [{...}, {...}, {...}]
    }
}
```

## Go Client Optimization

### QD-212: Efficient Client Setup

```go
import (
    "github.com/qdrant/go-client/qdrant"
    "google.golang.org/grpc"
)

func NewQdrantClient(addr string) (*qdrant.Client, error) {
    // Use gRPC for better performance
    client, err := qdrant.NewClient(&qdrant.Config{
        Host: "localhost",
        Port: 6334,       // gRPC port
        GrpcOptions: []grpc.DialOption{
            grpc.WithDefaultCallOptions(
                grpc.MaxCallRecvMsgSize(100 * 1024 * 1024), // 100MB
            ),
        },
    })
    return client, err
}
```

### QD-213: Batch Upsert

```go
func (r *QdrantRepo) BatchUpsert(ctx context.Context, points []*qdrant.PointStruct) error {
    // Batch in chunks for large datasets
    const batchSize = 100

    for i := 0; i < len(points); i += batchSize {
        end := i + batchSize
        if end > len(points) {
            end = len(points)
        }

        _, err := r.client.Upsert(ctx, &qdrant.UpsertPoints{
            CollectionName: "my_collection",
            Points:         points[i:end],
            Wait:           qdrant.PtrOf(true),
        })
        if err != nil {
            return err
        }
    }

    return nil
}
```

### QD-214: Optimized Search

```go
func (r *QdrantRepo) Search(ctx context.Context, vector []float32, filter *qdrant.Filter, limit uint64) ([]*qdrant.ScoredPoint, error) {
    result, err := r.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: "my_collection",
        Query:          qdrant.NewQuery(vector...),
        Limit:          qdrant.PtrOf(limit),
        Filter:         filter,
        WithPayload:    qdrant.NewWithPayload(true),
        Params: &qdrant.SearchParams{
            HnswEf:     qdrant.PtrOf(uint64(128)),
            Exact:      qdrant.PtrOf(false),
            Quantization: &qdrant.QuantizationSearchParams{
                Rescore:      qdrant.PtrOf(true),
                Oversampling: qdrant.PtrOf(float64(2.0)),
            },
        },
    })
    if err != nil {
        return nil, err
    }

    return result, nil
}
```

### QD-215: Parallel Search

```go
func (r *QdrantRepo) ParallelSearch(ctx context.Context, queries [][]float32) ([][]*qdrant.ScoredPoint, error) {
    results := make([][]*qdrant.ScoredPoint, len(queries))
    var wg sync.WaitGroup
    errChan := make(chan error, len(queries))

    for i, query := range queries {
        wg.Add(1)
        go func(idx int, vec []float32) {
            defer wg.Done()
            result, err := r.Search(ctx, vec, nil, 10)
            if err != nil {
                errChan <- err
                return
            }
            results[idx] = result
        }(i, query)
    }

    wg.Wait()
    close(errChan)

    if err := <-errChan; err != nil {
        return nil, err
    }

    return results, nil
}
```

## Sharding and Replication

### QD-216: Sharded Collection

```json
// Create sharded collection
PUT /collections/large_collection
{
    "vectors": {
        "size": 768,
        "distance": "Cosine"
    },
    "shard_number": 6,         // Number of shards
    "replication_factor": 2,   // Copies per shard
    "write_consistency_factor": 1
}
```

### QD-217: Cluster Configuration

```yaml
# config.yaml
cluster:
  enabled: true
  p2p:
    port: 6335
  consensus:
    tick_period_ms: 100
```

## Monitoring

### QD-218: Collection Info

```bash
# Collection metrics
GET /collections/my_collection

# Cluster info
GET /cluster

# Telemetry
GET /telemetry
```

### QD-219: Key Metrics

```go
func (r *QdrantRepo) GetCollectionStats(ctx context.Context) (*Stats, error) {
    info, err := r.client.GetCollectionInfo(ctx, "my_collection")
    if err != nil {
        return nil, err
    }

    return &Stats{
        VectorCount:    info.PointsCount,
        IndexedVectors: info.IndexedVectorsCount,
        Segments:       len(info.Segments),
        Status:         info.Status.String(),
    }, nil
}
```

## Performance Checklist

1. **Choose correct distance metric** for your embeddings
2. **Use payload indexes** for filtered searches
3. **Enable quantization** for large collections
4. **Tune HNSW parameters** (m, ef_construct, ef)
5. **Batch upserts** instead of individual inserts
6. **Use gRPC client** for better performance
7. **Use on_disk** for vectors larger than RAM
8. **Set appropriate shard_number** for large datasets
9. **Use rescore with quantization** for accuracy
10. **Monitor segment count** - too many = slow search
