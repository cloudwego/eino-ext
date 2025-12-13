# ES7 Retriever

English | [中文](README_zh.md)

An Elasticsearch 7.x retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever` interface.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever`
- Easy integration with Eino's retriever system
- Configurable Elasticsearch parameters
- Multiple search modes:
  - Exact match for text search
  - Dense vector similarity for semantic search
  - Raw string for custom queries
- Default result parser with customization support
- Filter support for refined queries

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/es7@latest
```

## Quick Start

Here's a quick example of how to use the retriever:

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v7"

	"github.com/cloudwego/eino-ext/components/retriever/es7"
	"github.com/cloudwego/eino-ext/components/retriever/es7/search_mode"
)

func main() {
	ctx := context.Background()

	// Connect to Elasticsearch
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  username,
		Password:  password,
	})

	// Create embedding component for vector search
	emb := createYourEmbedding()

	// Create retriever with dense vector similarity search
	retriever, _ := es7.NewRetriever(ctx, &es7.RetrieverConfig{
		Client:     client,
		Index:      "my_index",
		TopK:       10,
		SearchMode: search_mode.DenseVectorSimilarity(search_mode.DenseVectorSimilarityTypeCosineSimilarity, "content_vector"),
		Embedding:  emb,
	})

	// Retrieve documents
	docs, _ := retriever.Retrieve(ctx, "search query")

	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s, Score: %v\n", doc.ID, doc.Content, doc.MetaData["score"])
	}
}
```

## Search Modes

### Exact Match

Simple text search using Elasticsearch match query:

```go
searchMode := search_mode.ExactMatch("content")
```

### Dense Vector Similarity

Semantic search using script_score with dense vectors:

```go
// Cosine similarity
searchMode := search_mode.DenseVectorSimilarity(
    search_mode.DenseVectorSimilarityTypeCosineSimilarity,
    "content_vector",
)

// Other similarity types:
// - DenseVectorSimilarityTypeDotProduct
// - DenseVectorSimilarityTypeL1Norm
// - DenseVectorSimilarityTypeL2Norm
```

### Raw String Request

Pass custom JSON query directly:

```go
searchMode := search_mode.RawStringRequest()

// Then use a JSON query string as the search query
query := `{"query": {"bool": {"must": [{"match": {"content": "search term"}}]}}}`
docs, _ := retriever.Retrieve(ctx, query)
```

## Configuration

```go
type RetrieverConfig struct {
    Client         *elasticsearch.Client  // Required: Elasticsearch client
    Index          string                 // Required: Index name
    TopK           int                    // Optional: Number of results (default: 10)
    ScoreThreshold *float64               // Optional: Minimum score threshold
    SearchMode     SearchMode             // Required: Search strategy
    ResultParser   func(ctx context.Context, hit map[string]interface{}) (*schema.Document, error) // Optional: Custom parser
    Embedding      embedding.Embedder     // Required for vector search modes
}
```

## With Filters

Use `WithFilters` option to add query filters:

```go
filters := []interface{}{
    map[string]interface{}{
        "term": map[string]interface{}{
            "category": "news",
        },
    },
}

docs, _ := retriever.Retrieve(ctx, "query", es7.WithFilters(filters))
```

## Full Examples

- [Indexer Example](../../indexer/es7/examples/indexer)
- [Dense Vector Similarity Example](./examples/dense_vector_similarity)
- [Exact Match Example](./examples/exact_match)
- [Raw String Request Example](./examples/raw_string)

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go Client Documentation](https://github.com/elastic/go-elasticsearch)
- [Elasticsearch 7.10 Query DSL](https://www.elastic.co/guide/en/elasticsearch/reference/7.10/query-dsl.html)
