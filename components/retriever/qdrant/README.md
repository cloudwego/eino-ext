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
 qretriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
)

func main() {
 ctx := context.Background()

 // Create Qdrant client
 client, _ := qdrant.NewClient(&qdrant.Config{
  Host: "localhost",
  Port: 6334,
 })

 // Create retriever
 retriever, _ := qretriever.NewRetriever(ctx, &qretriever.Config{
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
    Client         *qdrant.Client      // Qdrant client
    Collection     string              // Collection name
    Embedding      embedding.Embedder  // Query embedding component
    ScoreThreshold *float64            // Optional score threshold
    TopK           int                 // Number of results
    VectorName     string              // Optional: named vector to query
}
```

## Advanced Usage

### Named Vectors

When the collection stores several vector spaces (e.g. one per embedding
model), set `VectorName` to the space that matches the query embedder. It must
be one of the names configured in the indexer's `VectorConfig` list, so the
query is compared against a space an embedding model actually wrote. Leaving it
empty queries the collection's default (unnamed) vector, which is the previous
behaviour.

```go
retriever, _ := qretriever.NewRetriever(ctx, &qretriever.Config{
    Client:     client,
    Collection: "docs",
    VectorName: "bge-m3",
    Embedding:  bgeM3,
})
```

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
retriever, _ := qretriever.NewRetriever(ctx, &qretriever.Config{
    // ... other config
    ScoreThreshold: &scoreThreshold,
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

