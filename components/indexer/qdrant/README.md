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
  qindexer "github.com/cloudwego/eino-ext/components/indexer/qdrant"
)

func main() {
  ctx := context.Background()

  // Create Qdrant client
  client, _ := qdrant.NewClient(&qdrant.Config{
    Host: "localhost",
    Port: 6333,
  })

  // Create indexer
  indexer, _ := qindexer.NewIndexer(ctx, &qindexer.Config{
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
    VectorDim  int                   // Required unless Vectors is set: vector dimension
    Distance   qdrant.Distance       // Required unless Vectors is set: distance metric
    BatchSize  int                   // Optional: Batch size (default: 10)
    Embedding  embedding.Embedder    // Required unless Vectors is set: embedding component
    Vectors    []VectorConfig        // Optional: several named vector spaces
}

type VectorConfig struct {
    Name      string                 // Required: name of the space in Qdrant
    Dimension int                    // Required: dimension of this space
    Distance  qdrant.Distance        // Required: distance metric of this space
    Embedding embedding.Embedder     // Required: embedder of this space
}
```

**Distance Metrics**: `Distance_Cosine`, `Distance_Dot`, `Distance_Euclid`, `Distance_Manhattan`

## Named Vectors

By default the collection holds a single, unnamed vector, so switching the
embedding model means re-indexing everything. `Vectors` declares several named
vector spaces instead, one per embedding model, each with its own dimension and
embedder:

```go
indexer, _ := qindexer.NewIndexer(ctx, &qindexer.Config{
    Client:     client,
    Collection: "docs",
    Vectors: []qindexer.VectorConfig{
        {Name: "bge-m3", Dimension: 1024, Distance: qdrant.Distance_Cosine, Embedding: bgeM3},
        {Name: "text-embedding-3-small", Dimension: 1536, Distance: qdrant.Distance_Cosine, Embedding: openAI},
    },
})
```

Every `Store` call embeds a document once per space and writes all spaces in a
single upsert, so both models index the same points and switching back to a
model that was already indexed costs no re-indexing. Use the retriever's
`VectorName` to pick which space a query is compared against.

`Vectors` is mutually exclusive with `VectorDim`, `Distance` and `Embedding`:
each space carries its own dimension, distance and embedder. `Distance` is
required per space - the zero value is `Distance_UnknownDistance`, which
`NewIndexer` rejects instead of letting the server fail the creation.

### Why every space has to be written in one request

A Qdrant upsert replaces the whole point. As the
[points documentation](https://qdrant.tech/documentation/manage-data/points/) puts
it, "the entire point is replaced, and any unspecified vectors are set to
null". Writing one space at a time from two indexers pointed at the same
document IDs therefore erases the other spaces - which is why all configured
spaces go into the same request. For the same reason a point written by this
indexer loses every vector space it does not configure, including spaces
created outside it.

### Named vectors must be planned with the collection

If the collection already exists, `NewIndexer` checks that each configured
space is present with the expected dimension and distance, and returns an error
otherwise. It cannot add the space itself: in the go-client version used here
(v1.15.x) `UpdateCollection.vectors_config` is a `VectorsConfigDiff`, whose
`VectorParamsDiff` carries no size or distance, so the gRPC API cannot
introduce a named vector with a dimension. Qdrant has supported this over REST
since v1.18, in the [Update Vector Schema](https://qdrant.tech/documentation/manage-data/collections/#update-vector-schema)
section (`PUT /collections/{collection_name}/vectors/{vector_name}`), but the Go
client has no equivalent yet. Recreate the collection with all named vectors
configured up front.

Leaving `Vectors` empty keeps the previous behaviour exactly, including
compatibility with collections created by earlier versions of this component.

## Examples

See `examples/default_indexer.go` for a complete working example.

## Documentation

- [Eino](https://github.com/cloudwego/eino)
- [Qdrant Go Client](https://github.com/qdrant/go-client)
- [Qdrant Documentation](https://qdrant.tech/documentation/)
