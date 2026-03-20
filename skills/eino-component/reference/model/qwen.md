# Qwen ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/qwen"
```

## Configuration

```go
chatModel, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
    BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", // Required
    APIKey:  "your-key",     // Required
    Model:   "qwen-plus",   // Required
})
```
