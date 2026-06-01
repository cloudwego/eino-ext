# Model Overview

Eino has two model paths:
- Classic ChatModel uses `*schema.Message`.
- AgenticModel uses `*schema.AgenticMessage` and preserves native block-based content.

## Interfaces

```go
type BaseModel[M any] interface {
    Generate(ctx context.Context, input []M, opts ...Option) (M, error)
    Stream(ctx context.Context, input []M, opts ...Option) (*schema.StreamReader[M], error)
}

type BaseChatModel = BaseModel[*schema.Message]
type AgenticModel = BaseModel[*schema.AgenticMessage]

type ToolCallingChatModel interface {
    BaseChatModel
    WithTools(tools []*schema.ToolInfo) (ToolCallingChatModel, error)
}
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

## AgenticModel

AgenticModel does not add a tool-binding method to the interface. Pass tools at request time:

```go
resp, err := agenticModel.Generate(ctx,
    []*schema.AgenticMessage{schema.UserAgenticMessage("use tools if needed")},
    model.WithTools(toolInfos),
)
```

Use provider-specific references for constructor and config details:

| Provider | Reference |
|----------|-----------|
| OpenAI | `reference/model/agenticopenai.md` |
| Gemini | `reference/model/agenticgemini.md` |
| DeepSeek | `reference/model/agenticdeepseek.md` |
| Ark | `reference/model/agenticark.md` |
| Qwen | `reference/model/agenticqwen.md` |
