# Qdrant Retriever

```
import qdrantRetriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
```

## Configuration

```go
retriever, err := qdrantRetriever.NewRetriever(ctx, &qdrantRetriever.Config{
    Client:         qdrantClient,
    CollectionName: "my_collection",
    Embedding:      embedder,
})

docs, err := retriever.Retrieve(ctx, "what is eino?")
```