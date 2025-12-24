# Milvus 2.x Indexer

English | [中文](./README_zh.md)

This package provides a Milvus 2.x (V2 SDK) indexer implementation for the EINO framework. It enables document storage and vector indexing in Milvus.

## Features

- **Milvus V2 SDK**: Uses the latest `milvus-io/milvus/client/v2` SDK
- **Auto Collection Management**: Automatically creates collections and indexes when needed
- **Flexible Index Types**: Supports multiple index builders (Auto, HNSW, IVF_FLAT, FLAT, etc.)
- **Sparse Vector Support**: Store and index sparse vectors for hybrid retrieval
- **Custom Document Conversion**: Configurable document-to-column conversion

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/milvus2
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/indexer/milvus2"
)

func main() {
	// Get the environment variables
	addr := os.Getenv("MILVUS_ADDR")
	username := os.Getenv("MILVUS_USERNAME")
	password := os.Getenv("MILVUS_PASSWORD")
	arkApiKey := os.Getenv("ARK_API_KEY")
	arkModel := os.Getenv("ARK_MODEL")

	ctx := context.Background()

	// Create an embedding model
	emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: arkApiKey,
		Model:  arkModel,
	})
	if err != nil {
		log.Fatalf("Failed to create embedding: %v", err)
		return
	}

	// Create an indexer
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address:  addr,
			Username: username,
			Password: password,
		},
		Collection:   "my_collection",
		Dimension:    1024, // Match your embedding model dimension
		MetricType:   milvus2.COSINE,
		IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
		Embedding:    emb,
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
		return
	}
	log.Printf("Indexer created successfully")

	// Store documents
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "Milvus is an open-source vector database",
			MetaData: map[string]any{
				"category": "database",
				"year":     2021,
			},
		},
		{
			ID:      "doc2",
			Content: "EINO is a framework for building AI applications",
		},
	}
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store: %v", err)
		return
	}
	log.Printf("Store success, ids: %v", ids)
}
```

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Client` | `*milvusclient.Client` | - | Pre-configured Milvus client (optional) |
| `ClientConfig` | `*milvusclient.ClientConfig` | - | Client configuration (required if Client is nil) |
| `Collection` | `string` | `"eino_collection"` | Collection name |
| `Dimension` | `int64` | - | Vector dimension (required for new collections) |
| `VectorField` | `string` | `"vector"` | Vector field name |
| `MetricType` | `MetricType` | `L2` | Similarity metric (L2, IP, COSINE, etc.) |
| `IndexBuilder` | `IndexBuilder` | AutoIndex | Index type builder |
| `Embedding` | `embedding.Embedder` | - | Embedder for vectorization (optional). If nil, documents must have vectors. |
| `ConsistencyLevel` | `ConsistencyLevel` | `Bounded` | Read consistency level |
| `PartitionName` | `string` | - | Default partition for insertion |
| `EnableDynamicSchema` | `bool` | `false` | Enable dynamic field support |
| `SparseVectorField` | `string` | - | Sparse vector field name (enables sparse indexing) |
| `SparseIndexBuilder` | `SparseIndexBuilder` | SPARSE_INVERTED | Sparse index builder |

## Index Builders

| Builder | Description | Key Parameters |
|---------|-------------|----------------|
| `NewAutoIndexBuilder()` | Milvus auto-selects optimal index | - |
| `NewHNSWIndexBuilder()` | Graph-based with excellent performance | `M`, `EfConstruction` |
| `NewIVFFlatIndexBuilder()` | Cluster-based search | `NList` |
| `NewIVFPQIndexBuilder()` | Memory-efficient with product quantization | `NList`, `M`, `NBits` |
| `NewIVFSQ8IndexBuilder()` | Scalar quantization | `NList` |
| `NewIVFRabitQIndexBuilder()` | IVF + RaBitQ binary quantization (Milvus 2.6+) | `NList` |
| `NewFlatIndexBuilder()` | Brute-force exact search | - |
| `NewDiskANNIndexBuilder()` | Disk-based for large datasets | - |
| `NewSCANNIndexBuilder()` | Fast with high recall | `NList`, `WithReorder` |

#### Sparse Index Builders

| Builder | Description | Key Parameters |
|---------|-------------|----------------|
| `NewSparseInvertedIndexBuilder()` | Inverted index for sparse vectors | `DropRatioBuild` |
| `NewSparseWANDIndexBuilder()` | WAND algorithm for sparse vectors | `DropRatioBuild` |

### Example: HNSW Index

```go
indexBuilder := milvus2.NewHNSWIndexBuilder().
	WithM(16).              // Max connections per node (4-64)
	WithEfConstruction(200) // Index build search width (8-512)
```

### Example: IVF_FLAT Index

```go
indexBuilder := milvus2.NewIVFFlatIndexBuilder().
	WithNList(256) // Number of cluster units (1-65536)
```

### Example: IVF_PQ Index (Memory-efficient)

```go
indexBuilder := milvus2.NewIVFPQIndexBuilder().
	WithNList(256). // Number of cluster units
	WithM(16).      // Number of subquantizers
	WithNBits(8)    // Bits per subquantizer (1-16)
```

### Example: SCANN Index (Fast with high recall)

```go
indexBuilder := milvus2.NewSCANNIndexBuilder().
	WithNList(256).           // Number of cluster units
	WithRawDataEnabled(true)  // Enable raw data for reranking
```

### Example: DiskANN Index (Large datasets)

```go
indexBuilder := milvus2.NewDiskANNIndexBuilder() // Disk-based, no extra params
```

## Metric Types

| Metric | Description |
|--------|-------------|
| `L2` | Euclidean distance |
| `IP` | Inner Product |
| `COSINE` | Cosine similarity |
| `HAMMING` | Hamming distance (binary) |
| `JACCARD` | Jaccard distance (binary) |

## Examples

See the [examples](./examples) directory for complete working examples:

- [demo](./examples/demo) - Basic collection setup with HNSW index
- [hnsw](./examples/hnsw) - HNSW index example
- [ivf_flat](./examples/ivf_flat) - IVF_FLAT index example
- [rabitq](./examples/rabitq) - IVF_RABITQ index example (Milvus 2.6+)
- [auto](./examples/auto) - AutoIndex example
- [diskann](./examples/diskann) - DISKANN index example
- [sparse](./examples/sparse) - Sparse vector (Dense + Sparse hybrid) example
- [byov](./examples/byov) - Bring Your Own Vectors example

### Sparse Vector Support

Store documents with both dense and sparse vectors for hybrid retrieval:

```go
// Create indexer with sparse vector field
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    ClientConfig:      &milvusclient.ClientConfig{Address: "localhost:19530"},
    Collection:        "hybrid_collection",
    Dimension:         128,                   // Dense vector dimension
    SparseVectorField: "sparse_vector",      // Enable sparse field
    // SparseIndexBuilder defaults to SPARSE_INVERTED_INDEX
})

// Create document with both vector types
doc := &schema.Document{
    ID:      "doc1",
    Content: "Hybrid document with dense and sparse vectors",
}

// Attach vectors
doc.WithDenseVector(denseVector)   // []float64
doc.WithSparseVector(sparseVector) // map[int]float64

ids, err := indexer.Store(ctx, []*schema.Document{doc})
```

### Bring Your Own Vectors (BYOV)

You can use the indexer without an embedder if your documents already have vectors.

```go
// Create indexer without embedding
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    ClientConfig: &milvusclient.ClientConfig{
        Address: "localhost:19530",
    },
    Collection:   "my_collection",
    Dimension:    128,
    // Embedding: nil, // Leave nil
})

// Store documents with pre-computed vectors
docs := []*schema.Document{
    {
        ID:      "doc1",
        Content: "Document with existing vector",
    },
}

// Attach vector to document
// Vector dimension must match the collection dimension
vector := []float64{0.1, 0.2, ...} 
docs[0].WithDenseVector(vector)

ids, err := indexer.Store(ctx, docs)
```

## License

Apache License 2.0
