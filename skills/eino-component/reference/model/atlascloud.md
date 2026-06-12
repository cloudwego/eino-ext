# Atlas Cloud ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/atlascloud"
```

## Configuration

```go
chatModel, err := atlascloud.NewChatModel(ctx, &atlascloud.ChatModelConfig{
    APIKey: "your-key",       // Required
    Model:  "deepseek-ai/DeepSeek-V3-0324", // Required
    // BaseURL: "https://api.atlascloud.ai/v1", // Default
})
```

## Notes

- Atlas Cloud LLM API is OpenAI-compatible.
- The default `BaseURL` is `https://api.atlascloud.ai/v1`.
- The `/v1` suffix is required.
- You can use the same request modifiers and extra fields supported by the OpenAI-compatible component.
