# Qianfan (Baidu) ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/qianfan"
```

## Configuration

```go
chatModel, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
    APIKey:    "your-key",
    SecretKey: "your-secret",
    Model:     "ernie-4.0",
})
```
