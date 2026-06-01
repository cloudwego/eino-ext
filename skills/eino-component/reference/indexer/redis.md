# Redis Indexer

```
import redisIndexer "github.com/cloudwego/eino-ext/components/indexer/redis"
```

## Configuration

```go
indexer, err := redisIndexer.NewIndexer(ctx, &redisIndexer.IndexerConfig{
    Client:    redisClient,
    KeyPrefix: "doc:",
    BatchSize: 10,
    Embedding: embedder,
})

ids, err := indexer.Store(ctx, docs)
```
