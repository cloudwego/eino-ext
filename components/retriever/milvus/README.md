# Milvus Retriever

English | [简体中文](README_zh.md)

## Quick Start

### Installation

```bash
go get github.com/milvus-io/milvus/client/v2
go get github.com/cloudwego/eino-ext/components/retriever/milvus
```

### Example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func main() {
	// Create Milvus client
	client, err := milvusclient.New(context.Background(), &milvusclient.ClientConfig{
		Address: "localhost:19530",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close(context.Background())

	// Create embedding model (example using a hypothetical embedding service)
	embeddingModel := embedding.NewOpenAIEmbedding(&embedding.OpenAIConfig{
		APIKey: "your-api-key",
		Model:  "text-embedding-ada-002",
	})

	// Create Milvus retriever
	retriever, err := milvus.NewRetriever(&milvus.RetrieverConfig{
		Client:     client,
		Collection: "my_documents",
		TopK:       10,
		Embedding:  embeddingModel,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Perform semantic search
	docs, err := retriever.Retrieve(context.Background(), "What is machine learning?")
	if err != nil {
		log.Fatal(err)
	}

	// Process results
	for i, doc := range docs {
		fmt.Printf("Document %d: %s\n", i+1, doc.PageContent)
		fmt.Printf("Score: %v\n", doc.MetaData["score"])
		fmt.Println("---")
	}
}
```

## Configuration

### RetrieverConfig

| Parameter | Type | Required/Optional | Default | Description |
|-----------|------|-------------------|---------|-------------|
| `Client` | `*milvusclient.Client` | **Required** | - | Milvus client instance for database operations |
| `Collection` | `string` | Optional | `"eino_collection"` | Milvus collection name to search in |
| `TopK` | `int` | Optional | `5` | Maximum number of documents to retrieve |
| `Embedding` | `embedding.Embedder` | Optional | `nil` | Embedder for converting text queries to vectors |
| `DocumentConverter` | `DocumentConverter` | Optional | Default converter | Converts Milvus search results to schema.Document objects |
| `VectorConverter` | `VectorConverter` | Optional | Default converter | Converts float64 vectors to Milvus entity.Vector format |

### Search Options

The retriever supports various search options that can be passed to the `Retrieve` method:

#### WithLimit

```go
// Override the TopK value for a specific search
docs, err := retriever.Retrieve(ctx, "query", milvus.WithLimit(20))
```

#### WithHybridSearchOption

```go
// Use hybrid search for more complex scenarios
hybridSearch := milvus.NewHybridSearchOption("vector_field", 10).
	WithFilter("category == 'technology'").
	WithOffset(5)

docs, err := retriever.Retrieve(ctx, "query", milvus.WithHybridSearchOption(hybridSearch))
```

### Hybrid Search Configuration

The `HybridSearch` type provides advanced search capabilities:

| Method | Description | Required/Optional |
|--------|-------------|-------------------|
| `WithANNSField(field)` | Set vector field name | Optional |
| `WithFilter(expr)` | Add boolean filter expression | Optional |
| `WithGroupByField(field)` | Group results by field | Optional |
| `WithGroupSize(size)` | Set group size | Optional |
| `WithStrictGroupSize(strict)` | Enforce strict group size | Optional |
| `WithSearchParam(key, value)` | Add search parameters | Optional |
| `WithAnnParam(param)` | Set ANN parameters | Optional |
| `WithOffset(offset)` | Skip results | Optional |
| `WithIgnoreGrowing(ignore)` | Ignore growing segments | Optional |
| `WithTemplateParam(key, val)` | Add template parameters | Optional |

## Vector Dimension Calculation

When working with Milvus, you need to ensure that your vector dimensions match the collection schema. The dimension depends on your embedding model.

For more information about vector dimensions and collection setup, refer to the [Milvus official documentation](https://milvus.io/docs).