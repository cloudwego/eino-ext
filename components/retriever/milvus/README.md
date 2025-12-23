# Milvus Retriever

English | [简体中文](README_zh.md)

An Milvus 2.x retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever`
interface. This enables seamless integration
with Eino's vector storage and retrieval system for enhanced semantic search capabilities.

## Quick Start

### Installation

```bash
go get github.com/milvus-io/milvus-sdk-go/v2@2.4.2
go get github.com/cloudwego/eino-ext/components/retriever/milvus@latest
```

### Create the Milvus Retriever

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/milvus-io/milvus-sdk-go/v2/client"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
)

func main() {
	// Get the environment variables
	addr := os.Getenv("MILVUS_ADDR")
	username := os.Getenv("MILVUS_USERNAME")
	password := os.Getenv("MILVUS_PASSWORD")
	arkApiKey := os.Getenv("ARK_API_KEY")
	arkModel := os.Getenv("ARK_MODEL")

	// Create a client
	ctx := context.Background()
	cli, err := client.NewClient(ctx, client.Config{
		Address:  addr,
		Username: username,
		Password: password,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer cli.Close()

	// Create an embedding model
	emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: arkApiKey,
		Model:  arkModel,
	})

	// Create a retriever
	retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      cli,
		Collection:  "",
		Partition:   nil,
		VectorField: "",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		DocumentConverter: nil,
		MetricType:        "",
		TopK:              0,
		ScoreThreshold:    5,
		Sp:                nil,
		Embedding:         emb,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "milvus")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}
	
	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("title: %s\n", doc.ID)
		fmt.Printf("content: %s\n", doc.Content)
		fmt.Printf("metadata: %v\n", doc.MetaData)
	}
}
```

## Configuration

```go
type RetrieverConfig struct {
	// Client is the Milvus client used for database operations.
	// This field is required and must be a milvus-sdk-go client of version 2.4.x.
	Client client.Client

	// Collection specifies the collection name in the Milvus database.
	// Optional; defaults to "eino_collection".
	Collection string

	// Partition specifies the partitions to search within.
	// Optional; defaults to an empty slice (search all partitions).
	Partition []string

	// VectorField specifies the name of the vector field in the collection.
	// Optional; defaults to "vector".
	VectorField string

	// OutputFields specifies which fields to include in search results.
	// Optional; defaults to an empty slice (only return IDs and distances).
	OutputFields []string

	// DocumentConverter transforms Milvus search results into schema.Document instances.
	// Optional; defaults to a converter compatible with the built-in schema.
	DocumentConverter func(ctx context.Context, doc client.SearchResult) ([]*schema.Document, error)

	// VectorConverter transforms embedding vectors into Milvus entity.Vector format.
	// Optional; defaults to a BinaryVector converter.
	VectorConverter func(ctx context.Context, vectors [][]float64) ([]entity.Vector, error)

	// TopK specifies the maximum number of results to return.
	// Optional; defaults to 5.
	TopK int

	// ScoreThreshold filters results below this similarity score.
	// Optional; defaults to 0 (no filtering).
	ScoreThreshold float64

	// SearchMode defines the search strategy and parameters for different index types.
	// Use search_mode.SearchModeHNSW, SearchModeIvfFlat, SearchModeAuto, or SearchModeFlat.
	// Optional; defaults to AUTOINDEX with COSINE metric.
	// When SearchMode is set, MetricType and Sp fields are ignored.
	SearchMode SearchMode

	// Deprecated: MetricType is deprecated; set the metric type in SearchMode instead.
	MetricType entity.MetricType

	// Deprecated: Sp is deprecated; use SearchMode instead to configure search parameters.
	Sp entity.SearchParam

	// Embedding provides the embedding model for vectorizing query strings.
	// This field is required.
	Embedding embedding.Embedder
}
```

## SearchMode

Flexible search configuration with `SearchMode` interface. Match your search mode with your index type:

| SearchMode | Index Type | Key Parameter |
|------------|------------|---------------|
| `SearchModeAuto` | AUTOINDEX | `level` (1-5, speed vs accuracy) |
| `SearchModeHNSW` | HNSW | `ef` (search width, higher = more accurate) |
| `SearchModeIvfFlat` | IVF_FLAT | `nprobe` (clusters to search) |
| `SearchModeFlat` | FLAT | N/A (brute force) |

> **Important**: The SearchMode's metric type must match the index's metric type.

**Usage Examples:**

```go
import "github.com/cloudwego/eino-ext/components/retriever/milvus/search_mode"

// AUTOINDEX search mode (recommended for most cases)
autoMode := search_mode.SearchModeAuto(&search_mode.AutoConfig{
    Level:  1,             // 1=fastest, 5=most accurate
    Metric: entity.COSINE, // Must match index metric
})
retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
    Client:     cli,
    Collection: "my_collection",
    SearchMode: autoMode,
    Embedding:  emb,
})

// HNSW search mode - for HNSW indexed collections
hnswMode, _ := search_mode.SearchModeHNSW(&search_mode.HNSWConfig{
    Ef:     64,        // Higher = more accurate, slower
    Metric: entity.L2, // Must match index metric
})
retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
    Client:     cli,
    Collection: "hnsw_collection",
    SearchMode: hnswMode,
    Embedding:  emb,
})

// IVF_FLAT search mode
ivfMode, _ := search_mode.SearchModeIvfFlat(&search_mode.IvfFlatConfig{
    Nprobe: 16,            // Number of clusters to search
    Metric: entity.COSINE,
})
```