# OpenSearch Retriever

English | [简体中文](README_zh.md)

An OpenSearch retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever` interface. This enables seamless integration with Eino's vector retrieval system for enhanced semantic search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever`
- Easy integration with Eino's retrieval system
- Configurable OpenSearch parameters
- Support for vector similarity search and keyword search
- Multiple search modes:
  - KNN (Approximate)
  - Exact Match (Term/Match)
  - Raw String (JSON Body)
  - Dense Vector Similarity (Script Score)
  - Neural Sparse (Sparse Vector)
- Custom result parsing support

## Search Mode Compatibility

| Search Mode | Minimum OpenSearch Version | Notes |
|-------------|----------------------------|-------|
| `ExactMatch` | 1.0+ | Standard query DSL |
| `RawString` | 1.0+ | Standard query DSL |
| `DenseVectorSimilarity` | 1.0+ | Uses `script_score` and painless vector functions |
| `Approximate` (KNN) | 1.0+ | Basic KNN supported since 1.0. Efficient filtering (post-filtering) requires 2.4+ (Lucene HNSW) or 2.9+ (Faiss). |
| `Approximate` (Hybrid) | 2.10+ | Generates `bool` query. Requires 2.10+ `normalization-processor` for advanced score normalization (Convex Combination). Basic `bool` query works on earlier versions (1.0+). |
| `Approximate` (RRF) | 2.19+ | Requires `score-ranker-processor` (2.19+) and `neural-search` plugin. |
| `NeuralSparse` (Query Text) | 2.11+ | Requires `neural-search` plugin and deployed model. |
| `NeuralSparse` (TokenWeights) | 2.11+ | Requires `neural-search` plugin. |

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/opensearch2@latest
```

## Quick Start

Here's a quick example of how to use the retriever:

```go
package main

import (
	"context"
	"log"
	
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v2"

	"github.com/cloudwego/eino-ext/components/retriever/opensearch2"
	"github.com/cloudwego/eino-ext/components/retriever/opensearch2/search_mode"
)

func main() {
	ctx := context.Background()

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// create retriever component
	retriever, _ := opensearch2.NewRetriever(ctx, &opensearch2.RetrieverConfig{
		Client: client,
		Index:  "your_index_name",
		TopK:   5,
		// Choose a search mode
		SearchMode: search_mode.Approximate(&search_mode.ApproximateConfig{
			VectorField: "content_vector",
			K:           5,
		}),
		ResultParser: func(ctx context.Context, hit map[string]interface{}) (*schema.Document, error) {
			// Parse the hit map to a Document
			id, _ := hit["_id"].(string)
			source := hit["_source"].(map[string]interface{})
			content, _ := source["content"].(string)
			return &schema.Document{ID: id, Content: content}, nil
		},
		Embedding: createYourEmbedding(),
	})

	docs, _ := retriever.Retrieve(ctx, "search query")
}
```

## Configuration

The retriever can be configured using the `RetrieverConfig` struct:

```go
type RetrieverConfig struct {
    Client *opensearch.Client // Required: OpenSearch client instance
    Index  string             // Required: Index name to retrieve documents from
    TopK   int                // Required: Number of results to return

    // Required: Search mode configuration
    // Prepared implementations in search_mode package:
    // - search_mode.Approximate(&ApproximateConfig{...})
    // - search_mode.ExactMatch(field)
    // - search_mode.RawStringRequest()
    // - search_mode.DenseVectorSimilarity(type, vectorField)
    // - search_mode.NeuralSparse(vectorField, &NeuralSparseConfig{...})
    SearchMode SearchMode

    // Optional: Function to parse OpenSearch hits (map[string]interface{}) into Documents
    // If not provided, a default parser is used.
    ResultParser func(ctx context.Context, hit map[string]interface{}) (doc *schema.Document, err error)

    // Optional: Required only if query vectorization is needed
    Embedding embedding.Embedder
}
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [OpenSearch Go Client Documentation](https://github.com/opensearch-project/opensearch-go)
