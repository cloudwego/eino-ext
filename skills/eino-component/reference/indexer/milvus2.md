# Milvus 2.x Indexer

```
import milvusIndexer "github.com/cloudwego/eino-ext/components/indexer/milvus2"
```

## Configuration

```go
milvusClient, _ := milvusclient.New(ctx, &milvusclient.ClientConfig{
    Address: "localhost:19530",
})

indexer, err := milvusIndexer.NewIndexer(ctx, &milvusIndexer.IndexerConfig{
    Client:     milvusClient,
    Collection: "my_collection",
    Embedding:  embedder,
})

ids, err := indexer.Store(ctx, docs)
```
