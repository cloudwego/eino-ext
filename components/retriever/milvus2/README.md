# Milvus 2.x Retriever

English | [中文](./README_zh.md)

This package provides a Milvus 2.x (V2 SDK) retriever implementation for the EINO framework. It enables vector similarity search with multiple search modes.

## Features

- **Milvus V2 SDK**: Uses the latest `milvus-io/milvus/client/v2` SDK
- **Multiple Search Modes**: Approximate, Range, Hybrid, Iterator, and Scalar search
- **Dense + Sparse Hybrid Search**: Combine dense and sparse vectors with RRF reranking
- **Score Filtering**: Filter results by similarity score threshold
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
		fmt.Printf("  MetaData: %v\n", doc.MetaData)
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
| `VectorField` | `string` | `"vector"` | Vector field name |
| `OutputFields` | `[]string` | all fields | Fields to return in results |
| `SearchMode` | `SearchMode` | - | Search strategy (required) |
| `Embedding` | `embedding.Embedder` | - | Embedder for query vectorization (required) |
| `ScoreThreshold` | `*float64` | - | Minimum score threshold |
| `ConsistencyLevel` | `ConsistencyLevel` | `Bounded` | Read consistency level |
| `Partitions` | `[]string` | - | Partitions to search |
| `SparseEmbedding` | `SparseEmbedder` | - | Sparse embedder for hybrid search |

## Search Modes

Import search modes from `github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode`.

### Approximate Search

Standard approximate nearest neighbor (ANN) search.

```go
mode := search_mode.NewApproximate(milvus2.COSINE)
```

### Range Search

Search within a distance range.

```go
mode := search_mode.NewRange(milvus2.L2).
    WithRadius(0.5).        // Minimum distance
    WithRangeFilter(1.0)    // Maximum distance
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
    &search_mode.SubRequest{
        VectorField: "sparse_vector",      // Sparse vector field
        VectorType:  milvus2.SparseVector, // Specify sparse type
        TopK:        10,
        MetricType:  milvus2.IP,            // Sparse uses IP metric
    },
)

// Create retriever with both embedders
retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
    ClientConfig:    &milvusclient.ClientConfig{Address: "localhost:19530"},
    Collection:      "hybrid_collection",
    VectorField:     "vector",
    TopK:            5,
    SearchMode:      hybridMode,
    Embedding:       denseEmbedder,        // Standard embedder for dense vectors
    SparseEmbedding: sparseEmbedder,       // SparseEmbedder for sparse queries
})
```

### Iterator Search

Batch-based traversal for large result sets.

```go
mode := search_mode.NewIterator(milvus2.COSINE).
    WithBatchSize(100).
    WithLimit(1000)
```

### Scalar Search

Metadata-only filtering without vector similarity (uses filter expressions as query).

```go
mode := search_mode.NewScalar()

// Query with filter expression
docs, err := retriever.Retrieve(ctx, `category == "electronics" AND year >= 2023`)
```

## Metric Types

| Metric | Description |
|--------|-------------|
| `L2` | Euclidean distance |
| `IP` | Inner Product |
| `COSINE` | Cosine similarity |
| `HAMMING` | Hamming distance (binary) |
| `JACCARD` | Jaccard distance (binary) |

> **Important**: The metric type in SearchMode must match the index metric type used when creating the collection.

## Examples

See the [examples](./examples) directory for complete working examples:

- [approximate](./examples/approximate) - Basic ANN search
- [range](./examples/range) - Range search example
- [hybrid](./examples/hybrid) - Hybrid multi-vector search
- [iterator](./examples/iterator) - Batch iterator search
- [scalar](./examples/scalar) - Scalar/metadata filtering
- [grouping](./examples/grouping) - Grouping search results
- [filtered](./examples/filtered) - Filtered vector search

## License

Apache License 2.0
