# Redis Retriever

English | [中文](README_zh.md)

A Redis retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever` interface. This component uses Redis vector search capabilities (FT.SEARCH) to retrieve documents based on semantic similarity.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever`
- Easy integration with Eino's retriever system
- Two search modes:
  - KNN vector search for top-k results
  - Vector range search with distance threshold
- Supports custom filters for refined queries
- Configurable return fields
- Custom document parser support
- Distance-based sorting

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/redis@latest
```

## Prerequisites

**Important**: To use Redis vector search, your Redis client must be configured with:
1. **Protocol 2**: Default is 3, must be set to 2 for FT.SEARCH
2. **UnstableResp3**: Default is false, must be set to true

```go
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Protocol: 2,              // Required for FT.SEARCH
})
client.Options().UnstableResp3 = true  // Required for vector search
```

## Quick Start

### KNN Vector Search

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	redisRetriever "github.com/cloudwego/eino-ext/components/retriever/redis"
)

func main() {
	ctx := context.Background()

	// 1. Create Redis client with proper configuration
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		Protocol: 2,  // Required for FT.SEARCH
	})
	client.Options().UnstableResp3 = true  // Required

	// 2. Create embedding component
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 3. Create Redis retriever with KNN search
	retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
		Client:      client,
		Index:       "my_index",
		VectorField: "content_vector",
		TopK:        5,
		Embedding:   emb,
	})

	// 4. Retrieve documents
	docs, err := retriever.Retrieve(ctx, "search query")
	if err != nil {
		fmt.Printf("retrieve error: %v\n", err)
		return
	}

	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s\n", doc.ID, doc.Content)
	}
}
```

### Vector Range Search

Use distance threshold for range-based search:

```go
threshold := 0.8

retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
	Client:            client,
	Index:             "my_index",
	VectorField:       "content_vector",
	DistanceThreshold: &threshold,  // Enable range search
	Embedding:         emb,
})

docs, _ := retriever.Retrieve(ctx, "search query")
```

## Configuration

```go
type RetrieverConfig struct {
    // Required: Redis client instance (must have Protocol=2 and UnstableResp3=true)
    Client *redis.Client

    // Required: Index name for vector search
    Index string

    // Optional: Vector field name in search query (default: "vector_content")
    // Should match the EmbedKey from redis indexer
    VectorField string

    // Optional: Distance threshold for range search
    // If set: uses vector range search
    // If nil: uses KNN vector search (default)
    DistanceThreshold *float64

    // Optional: Query dialect (default: 2)
    // See: https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/dialects/
    Dialect int

    // Optional: Fields to return from documents (default: ["content", "vector_content"])
    ReturnFields []string

    // Optional: Custom document converter
    // Default: defaultResultParser
    DocumentConverter func(ctx context.Context, doc redis.Document) (*schema.Document, error)

    // Optional: Number of results to return (default: 5)
    TopK int

    // Required: Embedding method for query vectorization
    Embedding embedding.Embedder
}
```

## Search Modes

### KNN Vector Search

Returns top-k most similar documents:

```go
retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:      client,
    Index:       "my_index",
    VectorField: "content_vector",
    TopK:        10,  // Return top 10 results
    Embedding:   emb,
})
```

Query format: `(*)=>[KNN 10 @content_vector $vector AS __vector_distance]`

### Vector Range Search

Returns all documents within distance threshold:

```go
threshold := 0.5

retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:            client,
    Index:             "my_index",
    VectorField:       "content_vector",
    DistanceThreshold: &threshold,
    Embedding:         emb,
})
```

Query format: `@content_vector:[VECTOR_RANGE $distance_threshold $vector]=>{$yield_distance_as: __vector_distance}`

## With Filters

Use filters to narrow down search results:

```go
docs, _ := retriever.Retrieve(ctx, "search query", 
    redisRetriever.WithFilterQuery("@category:{technology}"))
```

## Custom Return Fields

Specify which fields to return:

```go
retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:       client,
    Index:        "my_index",
    VectorField:  "content_vector",
    ReturnFields: []string{"content", "content_vector", "title", "category"},
    Embedding:    emb,
})
```

## Custom Document Parser

Customize how search results are parsed:

```go
retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:      client,
    Index:       "my_index",
    VectorField: "content_vector",
    DocumentConverter: func(ctx context.Context, doc redis.Document) (*schema.Document, error) {
        return &schema.Document{
            ID:      doc.ID,
            Content: doc.Fields["content"],
            MetaData: map[string]any{
                "title":    doc.Fields["title"],
                "category": doc.Fields["category"],
            },
        }, nil
    },
    Embedding: emb,
})
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Redis Go Client Documentation](https://github.com/redis/go-redis)
- [Redis Vector Search](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/)
- [KNN Vector Search](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#knn-vector-search)
- [Vector Range Queries](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/#vector-range-queries)
## Examples

See the following examples for more usage:

- [Customized Retriever](./examples/customized_retriever/)
- [Default Retriever](./examples/default_retriever/)

