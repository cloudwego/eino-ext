# Claude ChatModel

```
import "github.com/cloudwego/eino-ext/components/model/claude"
```

## Configuration

```go
chatModel, err := claude.NewChatModel(ctx, &claude.Config{
    APIKey:    "your-key",  // Required
    Model:     "claude-sonnet-4-20250514", // Required
    MaxTokens: 3000,        // Required
    // AWS Bedrock:
    // ByBedrock:       true,
    // AccessKey:       "...",
    // SecretAccessKey: "...",
    // Region:          "us-west-2",
})
```

## Extended Thinking

```go
resp, err := chatModel.Generate(ctx, messages, claude.WithThinking(&claude.Thinking{
    Enable:       true,
    BudgetTokens: 1024,
}))
thinking, ok := claude.GetThinking(resp)
```
