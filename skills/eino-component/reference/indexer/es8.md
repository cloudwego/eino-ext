# Elasticsearch 8 Indexer

```
import esIndexer "github.com/cloudwego/eino-ext/components/indexer/es8"
```

## Configuration

```go
esClient, _ := elasticsearch.NewTypedClient(elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
})

indexer, err := esIndexer.NewIndexer(ctx, &esIndexer.IndexerConfig{
    Client:    esClient,
    Index:     "my_index",
    Embedding: embedder,
})

ids, err := indexer.Store(ctx, docs)
```
