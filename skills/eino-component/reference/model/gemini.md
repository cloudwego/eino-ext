# Gemini ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/gemini"
```

## Configuration

```go
client, _ := genai.NewClient(ctx, &genai.ClientConfig{APIKey: "your-key"})

chatModel, err := gemini.NewChatModel(ctx, &gemini.Config{
    Client: client,             // Required: *genai.Client
    Model:  "gemini-2.5-flash", // Required
    ThinkingConfig: &genai.ThinkingConfig{
        IncludeThoughts: true,
    },
})
```
