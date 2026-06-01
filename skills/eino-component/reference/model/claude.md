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
    // Google Vertex AI:
    // ByVertex:        true,
    // VertexProjectID: "your-project-id",
    // VertexRegion:    "us-east5",
})
```

## Prompt Caching

Claude automatically caches repeated prefixes, but with **auto-cache explicitly enabled**, cache hits are billed at a significantly lower rate. Always enable it for multi-turn conversations or when using tools/system prompts.

### Auto Cache (Recommended)

Enable via per-request option. The caching strategy automatically sets cache breakpoints on system messages, tool definitions, and the last message of each turn.

```go
resp, err := chatModel.Generate(ctx, messages,
    claude.WithEnableAutoCache(true),
)
```

### Manual Breakpoints

For fine-grained control, set cache breakpoints on specific messages or tool definitions:

```go
// Cache a specific message (e.g., a long system prompt)
messages[0] = claude.SetMessageBreakpoint(messages[0])

// Cache a tool definition
toolInfo = claude.SetToolInfoBreakpoint(toolInfo)
```

Content before a breakpoint is cached. Subsequent requests reuse the cached prefix, reducing both latency and cost.

## Extended Thinking

```go
resp, err := chatModel.Generate(ctx, messages, claude.WithThinking(&claude.Thinking{
    Enable:       true,
    BudgetTokens: 1024,
}))
thinking, ok := claude.GetThinking(resp)
```
