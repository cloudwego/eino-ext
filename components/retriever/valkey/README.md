# Valkey Retriever

A Valkey retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever` interface. This component uses Valkey Search capabilities (FT.SEARCH) via the [valkey-glide](https://github.com/valkey-io/valkey-glide) client to retrieve documents based on semantic similarity.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever`
- Two search modes:
  - KNN vector search for top-k results
  - Vector range search with distance threshold
- Hybrid search with filter expressions
- Configurable distance metric, vector field, return fields, dialect
- Embedding integration for automatic query vectorization
- Full Eino callback integration (OnStart, OnEnd, OnError)

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/valkey@latest
```

## Prerequisites

- Valkey 9.1+ with the Search module (e.g., `valkey/valkey-bundle`)
- [valkey-glide](https://github.com/valkey-io/valkey-glide) Go client (requires CGO and Rust core library)

## Quick Start

```go
import (
	"context"
	"fmt"

	glide "github.com/valkey-io/valkey-glide/go/v2"
	"github.com/valkey-io/valkey-glide/go/v2/config"

	valkeyRetriever "github.com/cloudwego/eino-ext/components/retriever/valkey"
)

func main() {
	ctx := context.Background()

	// 1. Create Valkey GLIDE client.
	// WithClientInfoTag adds the recommended "eino-valkey" attribution tag (see note below).
	cfg := config.NewClientConfiguration().
		WithAddress(&config.NodeAddress{Host: "localhost", Port: 6379}).
		WithClientInfoTag("eino-valkey")
	client, _ := glide.NewClient(cfg)

	// 2. Create embedding component (use your preferred embedder)
	emb := yourEmbedder()

	// 3. Create Valkey retriever with KNN search
	retriever, _ := valkeyRetriever.NewRetriever(ctx, &valkeyRetriever.RetrieverConfig{
		Client:      client,
		Index:       "my_index",
		VectorField: "vector_content",
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

## Configuration

```go
type RetrieverConfig struct {
    // Required: Valkey GLIDE client (must implement CustomCommand)
    Client SearchClient

    // Required: Index name for vector search
    Index string

    // Optional: Vector field name (default: "vector_content")
    VectorField string

    // Optional: Distance threshold for range search
    // If set: uses vector range search (requires Valkey Search 1.3/2.0+, not yet supported)
    // If nil: uses KNN vector search (default)
    DistanceThreshold *float64

    // Optional: Query dialect (default: 2)
    Dialect int

    // Optional: Fields to return (default: ["content", "vector_content"])
    ReturnFields []string

    // Optional: Custom document converter
    DocumentConverter func(ctx context.Context, doc models.FtSearchDocument) (*schema.Document, error)

    // Optional: Number of results (default: 5)
    TopK int

    // Required: Embedding method for query vectorization
    Embedding embedding.Embedder
}
```

## Search Modes

### KNN Vector Search

```go
retriever, _ := valkeyRetriever.NewRetriever(ctx, &valkeyRetriever.RetrieverConfig{
    Client:    client,
    Index:     "my_index",
    TopK:      10,
    Embedding: emb,
})
```

### Vector Range Search

> **Note:** VECTOR_RANGE requires Valkey Search 1.3/2.0+, which is not yet released.
> Using this option with current Valkey Search versions will result in an error.

```go
threshold := 0.5
retriever, _ := valkeyRetriever.NewRetriever(ctx, &valkeyRetriever.RetrieverConfig{
    Client:            client,
    Index:             "my_index",
    DistanceThreshold: &threshold,
    Embedding:         emb,
})
```

## With Filters

```go
docs, _ := retriever.Retrieve(ctx, "search query",
    valkeyRetriever.WithFilterQuery("@category:{technology}"))
```

## Client Attribution

This component takes a caller-provided Valkey GLIDE client rather than constructing one, so the
library-name tag must be set where you build the client. When you create the GLIDE client, set
`WithClientInfoTag("eino-valkey")` (requires `valkey-glide/go/v2` >= v2.5.2). This makes the client
report `lib-name=GlideGo(eino-valkey)` in `CLIENT INFO`, letting operators attribute Valkey usage to
this integration. It is metadata only and does not change behavior.

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Valkey GLIDE Go Client](https://github.com/valkey-io/valkey-glide)
- [Valkey Search](https://valkey.io/topics/search/)
