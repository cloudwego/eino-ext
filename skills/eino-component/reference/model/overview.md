# ChatModel Overview

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
