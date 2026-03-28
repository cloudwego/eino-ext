# CozeLoop Callback

CozeLoop provides observability and tracing for Eino applications on the Coze platform.

```go
import (
    ccb "github.com/cloudwego/eino-ext/callbacks/cozeloop"
    "github.com/cloudwego/eino/callbacks"
    "github.com/coze-dev/cozeloop-go"
)
```

## Setup

```go
// Set environment variables:
//   COZELOOP_WORKSPACE_ID=your-workspace-id
//   COZELOOP_API_TOKEN=your-token

ctx := context.Background()
client, err := cozeloop.NewClient()
if err != nil {
    panic(err)
}
defer client.Close(ctx)

handler := ccb.NewLoopHandler(client)
callbacks.AppendGlobalHandlers(handler)
```

## Constructor

```go
func NewLoopHandler(client cozeloop.Client, opts ...Option) callbacks.Handler
```

The first argument is a `cozeloop.Client` created via `cozeloop.NewClient()`. Configuration is read from environment variables (`COZELOOP_WORKSPACE_ID`, `COZELOOP_API_TOKEN`).

## Options

| Option | Description |
|--------|-------------|
| `WithEnableTracing(enable bool)` | Enable/disable tracing (default: true) |
| `WithCallbackDataParser(parser)` | Custom callback data parser |
| `WithLogger(logger)` | Custom logger instance |
| `WithAggrMessageOutput(enable bool)` | Enable aggregated message output |
| `WithConcatFunction[T](fn)` | Register type-specific concatenation functions |

## Full Example

```go
func main() {
    ctx := context.Background()

    client, err := cozeloop.NewClient()
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close(ctx)

    handler := ccb.NewLoopHandler(client,
        ccb.WithEnableTracing(true),
    )
    callbacks.AppendGlobalHandlers(handler)

    // All Eino component calls are now traced
    chatModel, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        Model: "gpt-4o",
    })
    resp, _ := chatModel.Generate(ctx, []*schema.Message{
        {Role: schema.User, Content: "Hello"},
    })
}
```
