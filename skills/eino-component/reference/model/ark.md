# Ark (Volcengine) ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/ark"
```

## Configuration

```go
chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
    APIKey: "your-key",     // Required (or AccessKey+SecretKey)
    Model:  "endpoint-id",  // Required: Ark endpoint ID
    // BaseURL: "https://ark.cn-beijing.volces.com/api/v3", // Default
})
```
