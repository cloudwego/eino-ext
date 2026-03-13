# Elasticsearch 8 Retriever

```
import "github.com/cloudwego/eino-ext/components/retriever/es8"
```

## Configuration

```go
esClient, _ := elasticsearch.NewTypedClient(elasticsearch.Config{
    Addresses: []string{"http://localhost:9200"},
})

retriever, err := es8.NewRetriever(ctx, &es8.RetrieverConfig{
    Client:     esClient,
    Index:      "my_index",
    TopK:       5,
    Embedding:  embedder,
    SearchMode: search_mode.NewApproximateMode("content_vector"),
})
```
