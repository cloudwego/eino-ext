# Qdrant Indexer

A [Qdrant](https://qdrant.tech/) indexer implementation for [Eino](https://github.com/cloudwego/eino).

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/qdrant@latest
```

## Quick Start

```go
import (
  "context"
  "github.com/cloudwego/eino/schema"
  qdrant "github.com/qdrant/go-client/qdrant"
  "github.com/cloudwego/eino-ext/components/indexer/qdrant"
)

func main() {
  ctx := context.Background()

  // Create Qdrant client
  client, _ := qdrant.NewClient(&qdrant.Config{
    Host: "localhost",
    Port: 6333,
  })

  // Create indexer
  indexer, _ := qdrant.NewIndexer(ctx, &qdrant.Config{
    Client:     client,
    Collection: "my_collection",
    VectorDim:  384,
    Distance:   qdrant.Distance_Cosine,
    Embedding:  yourEmbedding, // Your embedding component
  })

  // Store documents
  docs := []*schema.Document{
    {ID: "1", Content: "Hello world", MetaData: map[string]interface{}{"type": "text"}},
  }
  ids, _ := indexer.Store(ctx, docs)
}
```

## Configuration

```go
type Config struct {
    Client     *qdrant.Client        // Required: Qdrant client
    Collection string                // Required: Collection name
    VectorDim  int                   // Required: Vector dimension
    Distance   qdrant.Distance       // Required: Distance metric
    BatchSize  int                   // Optional: Batch size (default: 10)
    Embedding  embedding.Embedder    // Required: Embedding component
}
```

**Distance Metrics**: `Distance_Cosine`, `Distance_Dot`, `Distance_Euclid`, `Distance_Manhattan`

## Examples

See `examples/default_indexer.go` for a complete working example.

## Documentation

- [Eino](https://github.com/cloudwego/eino)
- [Qdrant Go Client](https://github.com/qdrant/go-client)
- [Qdrant Documentation](https://qdrant.tech/documentation/)
