# Callback Reference

Inject observability (logging, tracing, metrics) into orchestrated graphs.

## Five Timing Points

```go
type Handler interface {
    OnStart(ctx context.Context, info *RunInfo, input CallbackInput) context.Context
    OnEnd(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context
    OnError(ctx context.Context, info *RunInfo, err error) context.Context
    OnStartWithStreamInput(ctx context.Context, info *RunInfo, input *schema.StreamReader[CallbackInput]) context.Context
    OnEndWithStreamOutput(ctx context.Context, info *RunInfo, output *schema.StreamReader[CallbackOutput]) context.Context
}
```

- **OnStart / OnEnd**: triggered for non-streaming input/output.
- **OnStartWithStreamInput / OnEndWithStreamOutput**: triggered when input/output is a stream.
- **OnError**: triggered on error before returning.

Which timing fires depends on the node's actual execution mode at runtime: if the node ultimately runs as a non-streaming endpoint, OnStart/OnEnd fire; if it runs as a streaming endpoint, the streaming variants fire instead.

## RunInfo

```go
type RunInfo struct {
    Name      string               // node name (set via WithNodeName)
    Type      string               // implementation type (e.g., "OpenAI")
    Component components.Component // abstract type (e.g., "ChatModel")
}
```

## Building Handlers

Use `HandlerBuilder` to implement only the timings you care about:

```go
import "github.com/cloudwego/eino/callbacks"

handler := callbacks.NewHandlerBuilder().
    OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        log.Printf("[Start] %s/%s input=%T", info.Component, info.Name, input)
        return ctx
    }).
    OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
        log.Printf("[End] %s/%s", info.Component, info.Name)
        return ctx
    }).
    OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
        log.Printf("[Error] %s/%s err=%v", info.Component, info.Name, err)
        return ctx
    }).
    Build()
```

## HandlerHelper (Component-Typed Callbacks)

Filter callbacks to specific component types with typed input/output:

```go
import ucb "github.com/cloudwego/eino/utils/callbacks"

handler := ucb.NewHandlerHelper().
    ChatModel(&ucb.ModelCallbackHandler{
        OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *model.CallbackInput) context.Context {
            log.Printf("Model input: %d messages", len(input.Messages))
            return ctx
        },
        OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
            log.Printf("Model output: %s", output.Message.Content)
            return ctx
        },
    }).
    Handler()
```

Supported: `ChatModel`, `ChatTemplate`, `Retriever`, `Indexer`, `Embedding`, `Loader`, `DocumentTransformer`, `Tool`, `ToolsNode`, `Lambda`, `Graph`.

## Registering Handlers

### Global handlers (init-time)

```go
callbacks.AppendGlobalHandlers(handler)  // applies to all runs; not concurrency-safe
```

### Per-invocation handlers

```go
out, err := r.Invoke(ctx, input,
    compose.WithCallbacks(handler),  // all nodes
)
```

### Node-targeted handlers

```go
out, err := r.Invoke(ctx, input,
    compose.WithCallbacks(handler).DesignateNode("Model"),           // top-level node
    compose.WithCallbacks(handler).DesignateNodeWithPath(
        compose.NewNodePath("SubGraph", "InnerNode"),                // nested node
    ),
)
```

## Callback Trigger Rules

- **Component triggers** (inside implementation): if `IsCallbacksEnabled()` returns true, the component fires its own callbacks with rich typed input/output.
- **Node triggers** (graph wrapper): if the component does NOT implement callbacks, the graph wraps it and fires callbacks with the component's interface input/output types.
- **Graph triggers**: the graph itself fires OnStart/OnEnd with the graph's overall input/output.

### Stream callback rule

Always close streams in callback handlers. The framework copies the stream for callbacks -- if a callback's copy is not closed, the original stream cannot release resources.

## Example: Adding Tracing to a Graph

```go
handler := callbacks.NewHandlerBuilder().
    OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        log.Printf("[TRACE Start] component=%s name=%s", info.Component, info.Name)
        return ctx
    }).
    OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
        log.Printf("[TRACE End] component=%s name=%s", info.Component, info.Name)
        return ctx
    }).
    Build()

// Register globally (fires for all graphs)
callbacks.AppendGlobalHandlers(handler)

// Or pass per-invocation
r, _ := g.Compile(ctx)
_, _ = r.Invoke(ctx, input, compose.WithCallbacks(handler))
```
