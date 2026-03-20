# Qdrant Indexer

```
import qdrantIndexer "github.com/cloudwego/eino-ext/components/indexer/qdrant"
```

## Configuration

```go
indexer, err := qdrantIndexer.NewIndexer(ctx, &qdrantIndexer.IndexerConfig{
    Client:         qdrantClient,
    CollectionName: "my_collection",
    Embedding:      embedder,
})

ids, err := indexer.Store(ctx, docs)
```
