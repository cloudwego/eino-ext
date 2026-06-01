# Runnable -- The Universal Execution Interface

All compiled Graph, Chain, and Workflow produce a `Runnable[I, O]`. This is the single interface that downstream code (including ADK agents) uses to execute orchestrated pipelines.

```go
// github.com/cloudwego/eino/compose
type Runnable[I, O any] interface {
    Invoke(ctx context.Context, input I, opts ...Option) (output O, err error)
    Stream(ctx context.Context, input I, opts ...Option) (output *schema.StreamReader[O], err error)
    Collect(ctx context.Context, input *schema.StreamReader[I], opts ...Option) (output O, err error)
    Transform(ctx context.Context, input *schema.StreamReader[I], opts ...Option) (output *schema.StreamReader[O], err error)
}
```

## Four Execution Modes

| Mode | Input | Output | When to Use |
|------|-------|--------|-------------|
| **Invoke** | value | value | Default: send input, get complete result |
| **Stream** | value | stream | Real-time output: send input, receive chunks incrementally |
| **Collect** | stream | value | Aggregate streaming input into a single result |
| **Transform** | stream | stream | Full streaming: process input stream, produce output stream |

## How It Works

The compose engine automatically converts between streaming and non-streaming at node boundaries. If you call `Stream()` on a compiled graph but an internal node only supports `Invoke`, the framework handles the conversion transparently.

```go
// Compile a graph into a Runnable
compiled, err := graph.Compile(ctx)

// All four modes are available on the same compiled object
result, err := compiled.Invoke(ctx, input)
stream, err := compiled.Stream(ctx, input)
result, err := compiled.Collect(ctx, inputStream)
outStream, err := compiled.Transform(ctx, inputStream)
```

## Passing Options at Runtime

Use `compose.WithCallbacks` and `compose.WithCallOption` to pass per-request configuration:

```go
result, err := compiled.Invoke(ctx, input,
    compose.WithCallbacks(handler),
    compose.WithChatModelOption(model.WithTemperature(0.7)).DesignateNode("ChatModel"),
)
```

See `/eino-compose` for details on call options and callbacks.
