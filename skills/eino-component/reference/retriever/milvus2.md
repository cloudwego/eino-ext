# Milvus 2.x Retriever

```
import milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
```

## Configuration

```go
milvusClient, _ := milvusclient.New(ctx, &milvusclient.ClientConfig{
    Address:  "localhost:19530",
    Username: "user",
    Password: "pass",
})

retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
    Client:         milvusClient,
    Collection:     "my_collection",
    Embedding:      embedder,
    SearchMode:     search_mode.NewApproximateSearchMode(10),
    DocumentParser: nil, // custom result-to-document parser
})
```
