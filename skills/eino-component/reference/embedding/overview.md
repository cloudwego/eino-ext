# Embedder Overview

Embedders convert text to vectors for semantic similarity search.

## Interface

```go
// github.com/cloudwego/eino/components/embedding
type Embedder interface {
    EmbedStrings(ctx context.Context, texts []string, opts ...Option) ([][]float64, error)
}
```

Returns one vector per input text. Vector dimensions are fixed by the model (e.g., 1536 for ada-002).

## Implementations

| Provider | Package | Key Config |
|----------|---------|------------|
| OpenAI | `embedding/openai` | APIKey, Model |
| Ark | `embedding/ark` | APIKey, Region, Model |
| Gemini | `embedding/gemini` | Client, Model |
| DashScope | `embedding/dashscope` | APIKey, Model |
| Ollama | `embedding/ollama` | BaseURL, Model |
| Qianfan | `embedding/qianfan` | APIKey, SecretKey |
| TencentCloud | `embedding/tencentcloud` | SecretID, SecretKey |

See `embedding/{provider}.md` for per-provider config and examples.
