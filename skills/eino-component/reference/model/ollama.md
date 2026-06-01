# Ollama ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/ollama"
```

## Configuration

```go
chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
    BaseURL: "http://localhost:11434", // Required
    Model:   "llama3",                 // Required
})
```
