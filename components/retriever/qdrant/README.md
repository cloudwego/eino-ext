# Qdrant Retriever

A [Qdrant](https://qdrant.tech/) retriever implementation for [Eino](https://github.com/cloudwego/eino) that provides vector similarity search capabilities.

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/qdrant@latest
```

## Quick Start

```go
import (
 "context"
 "github.com/cloudwego/eino/components/embedding"
 qdrant "github.com/qdrant/go-client/qdrant"
 "github.com/cloudwego/eino-ext/components/retriever/qdrant"
)

func main() {
 ctx := context.Background()

 // Create Qdrant client
 client, _ := qdrant.NewClient(&qdrant.Config{
  Host: "localhost",
  Port: 6334,
 })

 // Create retriever
 retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
  Client:     client,
  Collection: "my_collection",
  Embedding:  &myEmbedding{},
  TopK:       5,
 })

 // Search
 docs, _ := retriever.Retrieve(ctx, "tourist attraction")
}
```

## Configuration

```go
type Config struct {
    Client            *qdrant.Client      // Qdrant client
    Collection        string              // Collection name
    Embedding         embedding.Embedder  // Query embedding component
    ScoreThreshold    *float64            // Optional score threshold
    TopK              int                 // Number of results
    ReturnFields      []string            // Payload fields to fetch (default: ["metadata", "content"])
    DocumentConverter  func(ctx context.Context, point *qdrant.ScoredPoint) (*schema.Document, error) // Custom document converter
}
```

## Advanced Usage

### Filtering

```go
import "github.com/cloudwego/eino-ext/components/retriever/qdrant/options"

docs, _ := retriever.Retrieve(ctx, "query",
    options.WithFilter(&qdrant.Filter{
        Must: []*qdrant.Condition{
            qdrant.NewMatch("metadata.location", "Paris")
        },
    }),
)
```

### Score Threshold

```go
scoreThreshold := 0.7
retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
    // ... other config
    ScoreThreshold: &scoreThreshold,
})
```

### Return Fields

By default, the retriever fetches `"metadata"` and `"content"` from the payload. You can customize which fields are returned:

```go
retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
    // ... other config
    ReturnFields: []string{"content", "category", "author"},
})
```

This limits the payload fields at the protocol level — Qdrant only sends the requested fields over the wire.

### Custom Document Converter

For full control over how Qdrant points are converted to Eino documents, provide a custom `DocumentConverter`:

```go
retriever, _ := qdrant.NewRetriever(ctx, &qdrant.Config{
    // ... other config
    ReturnFields: []string{"text"},
    DocumentConverter: func(ctx context.Context, point *qdrant.ScoredPoint) (*schema.Document, error) {
        return &schema.Document{
            ID:      point.Id.GetUuid(),
            Content: point.Payload["text"].GetStringValue(),
            MetaData: map[string]any{
                "score": point.Score,
            },
        }, nil
    },
})
```

## Document Mapping

Documents are automatically mapped to Qdrant points:

- `doc.ID` → Point ID
- `doc.Content` → Payload `"content"`
- `doc.MetaData` → Payload `"metadata"`
- Embeddings → Point vectors

## References

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Qdrant Documentation](https://qdrant.tech/documentation/)
## Examples

See the following examples for more usage:

- [Default Retriever](./examples/default_retriever/)

