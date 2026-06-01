# OpenRouter ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/openrouter"
```

## Configuration

```go
chatModel, err := openrouter.NewChatModel(ctx, &openrouter.Config{
    APIKey: "your-key",                       // Required
    Model:  "anthropic/claude-sonnet-4-20250514",       // Required
    // BaseURL: "https://openrouter.ai/api/v1", // Default
    Reasoning: &openrouter.Reasoning{
        Effort: openrouter.EffortOfMedium,
    },
})
```
