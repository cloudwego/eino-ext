# OpenAI Embedder

```
import "github.com/cloudwego/eino-ext/components/embedding/openai"
```

## Configuration

```go
embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
    APIKey: "your-key",
    Model:  "text-embedding-3-small",
    // ByAzure: true,  // for Azure OpenAI
    // BaseURL: "https://{RESOURCE}.openai.azure.com",
})

vectors, err := embedder.EmbedStrings(ctx, []string{"hello world", "foo bar"})
// vectors[0] is the embedding for "hello world"
```
