# Ark Embedder

```
import "github.com/cloudwego/eino-ext/components/embedding/ark"
```

## Configuration

```go
embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
    APIKey: os.Getenv("ARK_API_KEY"),
    Region: os.Getenv("ARK_REGION"),
    Model:  os.Getenv("ARK_MODEL"),
})
```
