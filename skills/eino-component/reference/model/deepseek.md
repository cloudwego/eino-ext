# DeepSeek ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/deepseek"
```

## Configuration

```go
chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
    APIKey: "your-key",                      // Required
    Model:  "deepseek-reasoner",             // Required
    // BaseURL: "https://api.deepseek.com/", // Default
})
```

## Reasoning Content

```go
reasoning, ok := deepseek.GetReasoningContent(resp)
```
