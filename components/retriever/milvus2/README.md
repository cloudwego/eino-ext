# Milvus 2.x Retriever

English | [中文](./README_zh.md)

This package provides a Milvus 2.x (V2 SDK) retriever implementation for the EINO framework. It enables vector similarity search with multiple search modes.

> **Note**: This package requires **Milvus 2.5+** for server-side function support (e.g., BM25).

## Features

- **Milvus V2 SDK**: Uses the latest `milvus-io/milvus/client/v2` SDK
- **Multiple Search Modes**: Approximate, Range, Hybrid, Iterator, and Scalar search
- **Dense + Sparse Hybrid Search**: Combine dense and sparse vectors with RRF reranking
- **Custom Result Conversion**: Configurable result-to-document conversion

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/milvus2
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
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

	// Create a retriever
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address:  addr,
			Username: username,
			Password: password,
		},
		Collection: "my_collection",
		TopK:       10,
		SearchMode: search_mode.NewApproximate(milvus2.COSINE),
		Embedding:  emb,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}
	log.Printf("Retriever created successfully")

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "search query")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}

	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("  ID: %s\n", doc.ID)
		fmt.Printf("  Content: %s\n", doc.Content)
		fmt.Printf("  Score: %v\n", doc.Score())
	}
}
```

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Client` | `*milvusclient.Client` | - | Pre-configured Milvus client (optional) |
| `ClientConfig` | `*milvusclient.ClientConfig` | - | Client configuration (required if Client is nil) |
| `Collection` | `string` | `"eino_collection"` | Collection name |
| `TopK` | `int` | `5` | Number of results to return |
| `VectorField` | `string` | `"vector"` | Dense vector field name |
| `SparseVectorField` | `string` | `"sparse_vector"` | Sparse vector field name |
| `OutputFields` | `[]string` | all fields | Fields to return in results |
| `SearchMode` | `SearchMode` | - | Search strategy (required) |
| `Embedding` | `embedding.Embedder` | - | Embedder for query vectorization (optional, required for vector search) |
| `DocumentConverter` | `func` | default converter | Custom result-to-document converter |
| `ConsistencyLevel` | `ConsistencyLevel` | `ConsistencyLevelDefault` | Consistency level (`ConsistencyLevelDefault` uses the collection's level; no per-request override is applied) |
| `Partitions` | `[]string` | - | Partitions to search |

### VectorType (for Hybrid Search)

| Value | Description |
|-------|-------------|
| `DenseVector` | Standard dense floating-point vectors (default) |
| `SparseVector` | Sparse vectors (used with BM25 or precomputed sparse embeddings) |

## Search Modes

Import search modes from `github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode`.

### Approximate Search

Standard approximate nearest neighbor (ANN) search.

```go
mode := search_mode.NewApproximate(milvus2.COSINE)
```

### Range Search

Search within a distance range (vectors within `Radius`).

```go
// L2: Distance <= Radius
// IP/Cosine: Score >= Radius
mode := search_mode.NewRange(milvus2.L2, 0.5).
    WithRangeFilter(0.1) // Optional: Inner boundary for ring search
```

### Sparse Search (BM25)

Sparse-only search using BM25. Requires Milvus 2.5+ with sparse vector fields and enabled Functions.

```go
// OutputFields is required to retrieve content for sparse-only search (BM25)
// MetricType: BM25 (default) or IP
mode := search_mode.NewSparse(milvus2.BM25)

// In config, use "*" or specific fields to ensure content is returned:
// OutputFields: []string{"*"}
```

### Hybrid Search (Dense + Sparse)

Multi-vector search combining dense and sparse vectors with result reranking. Requires a collection with both dense and sparse vector fields (see indexer sparse example).

```go
import (
    "github.com/milvus-io/milvus/client/v2/milvusclient"
    milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
    "github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

// Define hybrid search with Dense + Sparse sub-requests
hybridMode := search_mode.NewHybrid(
    milvusclient.NewRRFReranker().WithK(60), // RRF reranker
    &search_mode.SubRequest{
        VectorField: "vector",             // Dense vector field
        VectorType:  milvus2.DenseVector,  // Default, can be omitted
        TopK:        10,
        MetricType:  milvus2.L2,
    },
    // Sparse SubRequest
    &search_mode.SubRequest{
        VectorField: "sparse_vector",       // Sparse vector field
        VectorType:  milvus2.SparseVector,  // Specify sparse type
        TopK:        10,
        MetricType:  milvus2.BM25,          // Use BM25 or IP based on index
    },
)

// Create retriever (Sparse embedding handled server-side by Milvus Function)
retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
    ClientConfig:      &milvusclient.ClientConfig{Address: "localhost:19530"},
    Collection:        "hybrid_collection",
    VectorField:       "vector",             // Default dense field
    SparseVectorField: "sparse_vector",      // Default sparse field
    TopK:              5,
    SearchMode:        hybridMode,
    Embedding:         denseEmbedder,        // Standard embedder for dense vectors
})
```

### Iterator Search

Batch-based traversal for large result sets.

> [!WARNING]
> The `Retrieve` method in `Iterator` mode fetches **all** results until the total limit (`TopK`) or end-of-collection is reached. For extremely large datasets, this may consume significant memory.

```go
// 100 is the batch size (items per network call)
mode := search_mode.NewIterator(milvus2.COSINE, 100).
    WithSearchParams(map[string]string{"nprobe": "10"})

// Use RetrieverConfig.TopK to set the total limit (IteratorLimit).
```

### Scalar Search

Metadata-only filtering without vector similarity (uses filter expressions as query).

```go
mode := search_mode.NewScalar()

// Query with filter expression
docs, err := retriever.Retrieve(ctx, `category == "electronics" AND year >= 2023`)
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
| `BM25` | Okapi BM25 (Required for BM25 Search) |
| `IP` | Inner Product (Suitable for precomputed sparse vectors) |

### Binary Vector Metrics
| Metric | Description |
|--------|-------------|
| `HAMMING` | Hamming distance |
| `JACCARD` | Jaccard distance |
| `TANIMOTO` | Tanimoto distance |
| `SUBSTRUCTURE` | Substructure search |
| `SUPERSTRUCTURE` | Superstructure search |

> **Important**: The metric type in SearchMode must match the index metric type used when creating the collection.

## Examples

See the [examples](./examples) directory for complete working examples:

- [approximate](./examples/approximate) - Basic ANN search
- [range](./examples/range) - Range search example
- [hybrid](./examples/hybrid) - Hybrid multi-vector search (Dense + BM25)
- [hybrid_chinese](./examples/hybrid_chinese) - Chinese text hybrid search with BM25
- [iterator](./examples/iterator) - Batch iterator search
- [scalar](./examples/scalar) - Scalar/metadata filtering
- [grouping](./examples/grouping) - Grouping search results
- [filtered](./examples/filtered) - Filtered vector search
- [sparse](./examples/sparse) - Sparse-only search example (BM25)

## License

Apache License 2.0
