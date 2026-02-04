# Milvus 2.x Indexer

English | [中文](./README_zh.md)

This package provides a Milvus 2.x (V2 SDK) indexer implementation for the EINO framework. It enables document storage and vector indexing in Milvus.

> **Note**: This package requires **Milvus 2.5+** for server-side function support (e.g., BM25).

## Features

- **Milvus V2 SDK**: Uses the latest `milvus-io/milvus/client/v2` SDK
- **Flexible Index Types**: Supports comprehensive index types including Auto, HNSW, IVF variants, SCANN, DiskANN, GPU indexes, and RaBitQ (Milvus 2.6+)
- **Hybrid Search Ready**: Native support for Sparse Vectors (BM25/SPLADE) alongside Dense Vectors
- **Service-side Vector Generation**: Automatically generate sparse vectors using Milvus Functions (BM25)
- **Auto Management**: Handles collection schema creation, index building, and loading automatically
- **Field Analysis**: Configurable text analyzers (English, Chinese, Standard, etc.)
- **Custom Document Conversion**: Flexible mapping from Eino documents to Milvus columns

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

		Vector: &milvus2.VectorConfig{
			Dimension:  1024, // Match your embedding model dimension
			MetricType: milvus2.COSINE,
			IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
		},
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
| `Vector` | `*VectorConfig` | - | Dense vector configuration (Dimension, MetricType, IndexBuilder) |
| `Sparse` | `*SparseVectorConfig` | - | Sparse vector configuration (MetricType, FieldName) |
| `Embedding` | `embedding.Embedder` | - | Embedder for vectorization (optional). If nil, documents must have vectors (BYOV). |
| `DocumentConverter` | `func` | default converter | Custom document to Milvus column converter |
| `ConsistencyLevel` | `ConsistencyLevel` | `ConsistencyLevelDefault` | Consistency level (`ConsistencyLevelDefault` uses Milvus default: Bounded; stays at collection level if not explicitly set) |
| `PartitionName` | `string` | - | Default partition for insertion |
| `EnableDynamicSchema` | `bool` | `false` | Enable dynamic field support |
| `Functions` | `[]*entity.Function` | - | Schema functions (e.g., BM25) for server-side processing |
| `FieldParams` | `map[string]map[string]string` | - | Parameters for fields (e.g., enable_analyzer) |

### Vector Configuration (`VectorConfig`)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Dimension` | `int64` | - | Vector dimension (Required) |
| `MetricType` | `MetricType` | `L2` | Similarity metric (L2, IP, COSINE, etc.) |
| `IndexBuilder` | `IndexBuilder` | `AutoIndexBuilder` | Index type builder (HNSW, IVF, etc.) |
| `VectorField` | `string` | `"vector"` | Field name for dense vector |

### Sparse Vector Configuration (`SparseVectorConfig`)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `VectorField` | `string` | `"sparse_vector"` | Field name for sparse vector |
| `MetricType` | `MetricType` | `BM25` | Similarity metric |
| `Method` | `SparseMethod` | `SparseMethodAuto` | Generation method (`SparseMethodAuto` or `SparseMethodPrecomputed`) |
| `IndexBuilder` | `SparseIndexBuilder` | `SparseInvertedIndex` | Index builder (`NewSparseInvertedIndexBuilder` or `NewSparseWANDIndexBuilder`) |

> **Note**: `Method` defaults to `Auto` only if `MetricType` is `BM25`. `Auto` implies using Milvus server-side functions (remote function). For other metrics (e.g., `IP`), it defaults to `Precomputed`.

## Index Builders

### Dense Index Builders

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
| `NewSCANNIndexBuilder()` | Fast with high recall | `NList`, `WithRawDataEnabled` |
| `NewBinFlatIndexBuilder()` | Brute-force for binary vectors | - |
| `NewBinIVFFlatIndexBuilder()` | Cluster-based for binary vectors | `NList` |
| `NewGPUBruteForceIndexBuilder()` | GPU-accelerated brute-force | - |
| `NewGPUIVFFlatIndexBuilder()` | GPU-accelerated IVF_FLAT | - |
| `NewGPUIVFPQIndexBuilder()` | GPU-accelerated IVF_PQ | - |
| `NewGPUCagraIndexBuilder()` | GPU-accelerated graph-based (CAGRA) | `IntermediateGraphDegree`, `GraphDegree` |

### Sparse Index Builders

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

### Example: Sparse Inverted Index

```go
indexBuilder := milvus2.NewSparseInvertedIndexBuilder().
	WithDropRatioBuild(0.2) // Drop ratio for small values (0.0-1.0)
```

### Dense Vector Metrics
| Metric | Description |
|--------|-------------|
| `L2` | Euclidean distance |
| `IP` | Inner Product |
| `COSINE` | Cosine similarity |

### Sparse Vector Metrics
| Metric | Description |
|--------|-------------|
| `BM25` | Okapi BM25 (Required for `SparseMethodAuto`) |
| `IP` | Inner Product (Suitable for precomputed sparse vectors) |

### Binary Vector Metrics
| Metric | Description |
|--------|-------------|
| `HAMMING` | Hamming distance |
| `JACCARD` | Jaccard distance |
| `TANIMOTO` | Tanimoto distance |
| `SUBSTRUCTURE` | Substructure search |
| `SUPERSTRUCTURE` | Superstructure search |

## Sparse Vector Support

The indexer supports two modes for sparse vectors: **Auto-Generation** and **Precomputed**.

### 1. Auto-Generation (BM25)

Uses Milvus server-side functions to automatically generate sparse vectors from the content field.

- **Requirement**: Milvus 2.5+
- **Configuration**: Set `MetricType: milvus2.BM25`.

```go
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    // ... basic config ...
    Collection:        "hybrid_collection",
    
    Sparse: &milvus2.SparseVectorConfig{
        VectorField: "sparse_vector",
        MetricType:  milvus2.BM25, 
        // Method defaults to SparseMethodAuto for BM25
    },
    
    // Analyzer configuration for BM25
    FieldParams: map[string]map[string]string{
        "content": {
            "enable_analyzer": "true",
            "analyzer_params": `{"type": "standard"}`,
        },
    },
})
```

### 2. Precomputed (SPLADE, BGE-M3, etc.)

Allows storing sparse vectors generated by external models (e.g., SPLADE, BGE-M3) or custom logic.

- **Configuration**: Set `MetricType` (usually `IP`) and `Method: milvus2.SparseMethodPrecomputed`.
- **Usage**: Provide sparse vectors via `doc.WithSparseVector()`.

```go
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    Collection: "sparse_collection",
    
    Sparse: &milvus2.SparseVectorConfig{
        VectorField: "sparse_vector",
        MetricType:  milvus2.IP,
        Method:      milvus2.SparseMethodPrecomputed,
    },
})

// Store documents with sparse vectors
doc := &schema.Document{ID: "1", Content: "..."}
doc.WithSparseVector(map[int]float64{
    1024: 0.5,
    2048: 0.3,
})
indexer.Store(ctx, []*schema.Document{doc})
```

## Bring Your Own Vectors (BYOV)

You can use the indexer without an embedder if your documents already have vectors.

```go
// Create indexer without embedding
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    ClientConfig: &milvusclient.ClientConfig{
        Address: "localhost:19530",
    },
    Collection:   "my_collection",
    Vector: &milvus2.VectorConfig{
        Dimension:  128,
        MetricType: milvus2.L2,
    },
    // Embedding: nil, // Leave nil
})

// Store documents with pre-computed vectors
docs := []*schema.Document{
    {
        ID:      "doc1",
        Content: "Document with existing vector",
    },
}

// Attach dense vector to document
// Vector dimension must match the collection dimension
vector := []float64{0.1, 0.2, ...} 
docs[0].WithDenseVector(vector)

// Attach sparse vector (optional, if Sparse is configured)
// Sparse vectors are maps of index -> weight
sparseVector := map[int]float64{
    10: 0.5,
    25: 0.8,
}
docs[0].WithSparseVector(sparseVector)

ids, err := indexer.Store(ctx, docs)
```

For sparse vectors in BYOV mode, configured the sparse vector as **Precomputed** (see above).

## Examples

See the [examples](./examples) directory for complete working examples:

- [demo](./examples/demo) - Basic collection setup with HNSW index
- [hnsw](./examples/hnsw) - HNSW index example
- [ivf_flat](./examples/ivf_flat) - IVF_FLAT index example
- [rabitq](./examples/rabitq) - IVF_RABITQ index example (Milvus 2.6+)
- [auto](./examples/auto) - AutoIndex example
- [diskann](./examples/diskann) - DISKANN index example
- [hybrid](./examples/hybrid) - Hybrid search setup (Dense + BM25 sparse) (Milvus 2.5+)
- [hybrid_chinese](./examples/hybrid_chinese) - Hybrid search with Chinese analyzer (Milvus 2.5+)
- [sparse](./examples/sparse) - Sparse-only index example (BM25)
- [byov](./examples/byov) - Bring Your Own Vectors example

## License

Apache License 2.0
