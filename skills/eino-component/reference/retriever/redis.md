# Redis Retriever

```
import redisRetriever "github.com/cloudwego/eino-ext/components/retriever/redis"
```

## Configuration

```go
// Redis client MUST use Protocol 2 for FT.SEARCH
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Protocol: 2,
})
client.Options().UnstableResp3 = true

retriever, err := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client:      client,
    Index:       "my_index",
    VectorField: "content_vector",
    TopK:        5,
    Embedding:   embedder,
})

docs, err := retriever.Retrieve(ctx, "what is eino?")
```
