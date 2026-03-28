# Ollama Embedder

```
import "github.com/cloudwego/eino-ext/components/embedding/ollama"
```

## Configuration

```go
embedder, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
    BaseURL: "http://localhost:11434",
    Model:   "nomic-embed-text",
})
```
