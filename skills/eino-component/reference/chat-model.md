# ChatModel Reference

All ChatModel implementations implement `ToolCallingChatModel` from `github.com/cloudwego/eino/components/model`.

## Interfaces

```go
type BaseChatModel interface {
    Generate(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.Message, error)
    Stream(ctx context.Context, input []*schema.Message, opts ...Option) (
        *schema.StreamReader[*schema.Message], error)
}

type ToolCallingChatModel interface {
    BaseChatModel
    WithTools(tools []*schema.ToolInfo) (ToolCallingChatModel, error)
}
```

## OpenAI

```
import "github.com/cloudwego/eino-ext/components/model/openai"
```

Key config fields:
```go
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey:  "your-key",       // Required
    Model:   "gpt-4o",         // Required
    BaseURL: "",               // Optional, custom endpoint
    // Azure OpenAI:
    // ByAzure:    true,
    // BaseURL:    "https://{RESOURCE}.openai.azure.com",
    // APIVersion: "2024-06-01",
    Temperature:         ptrFloat32(0.7), // Optional, 0.0-2.0
    MaxCompletionTokens: ptrInt(4096),    // Optional
    ReasoningEffort:     openai.ReasoningEffortLevelHigh, // Optional
})
```

## Claude

```
import "github.com/cloudwego/eino-ext/components/model/claude"
```

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

// Extended thinking
resp, err := chatModel.Generate(ctx, messages, claude.WithThinking(&claude.Thinking{
    Enable:       true,
    BudgetTokens: 1024,
}))
thinking, ok := claude.GetThinking(resp)
```

## Gemini

```
import "github.com/cloudwego/eino-ext/components/model/gemini"
```

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

## Ark (Volcengine)

```
import "github.com/cloudwego/eino-ext/components/model/ark"
```

```go
chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
    APIKey: "your-key",     // Required (or AccessKey+SecretKey)
    Model:  "endpoint-id",  // Required: Ark endpoint ID
    // BaseURL: "https://ark.cn-beijing.volces.com/api/v3", // Default
})
```

## Ollama

```
import "github.com/cloudwego/eino-ext/components/model/ollama"
```

```go
chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
    BaseURL: "http://localhost:11434", // Required
    Model:   "llama3",                 // Required
})
```

## DeepSeek

```
import "github.com/cloudwego/eino-ext/components/model/deepseek"
```

```go
chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
    APIKey: "your-key",                      // Required
    Model:  "deepseek-reasoner",             // Required
    // BaseURL: "https://api.deepseek.com/", // Default
})

// Access reasoning content
reasoning, ok := deepseek.GetReasoningContent(resp)
```

## Qwen

```
import "github.com/cloudwego/eino-ext/components/model/qwen"
```

```go
chatModel, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
    BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", // Required
    APIKey:  "your-key",     // Required
    Model:   "qwen-plus",   // Required
})
```

## Qianfan (Baidu)

```
import "github.com/cloudwego/eino-ext/components/model/qianfan"
```

```go
chatModel, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
    APIKey:    "your-key",
    SecretKey: "your-secret",
    Model:     "ernie-4.0",
})
```

## OpenRouter

```
import "github.com/cloudwego/eino-ext/components/model/openrouter"
```

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

## Tool Binding

Use `WithTools` to bind tools (returns a new instance, safe for concurrent use):

```go
tools := []*schema.ToolInfo{
    {
        Name: "get_weather",
        Desc: "Get current weather for a city",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "city": {Type: "string", Desc: "City name", Required: true},
        }),
    },
}

withTools, err := chatModel.WithTools(tools)
resp, err := withTools.Generate(ctx, messages)

for _, tc := range resp.ToolCalls {
    fmt.Printf("Tool: %s, Args: %s\n", tc.Function.Name, tc.Function.Arguments)
}
```

## Streaming

```go
reader, err := chatModel.Stream(ctx, messages)
if err != nil {
    return err
}
defer reader.Close()

for {
    chunk, err := reader.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        return err
    }
    fmt.Print(chunk.Content)
}
```

To concatenate stream chunks into a single message:

```go
chunks := make([]*schema.Message, 0)
for { /* collect chunks */ }
msg, err := schema.ConcatMessages(chunks)
```

## Azure OpenAI

Use the OpenAI model with Azure-specific config:

```go
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    ByAzure:    true,
    BaseURL:    "https://{RESOURCE_NAME}.openai.azure.com",
    APIKey:     os.Getenv("AZURE_OPENAI_API_KEY"),
    APIVersion: "2024-06-01",
    Model:      "gpt-4o",
})
```
