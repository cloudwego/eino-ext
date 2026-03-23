# Callback Overview

Callback handlers observe component execution for tracing and monitoring.

## Interface

```go
// github.com/cloudwego/eino/callbacks
type Handler interface {
    OnStart(ctx context.Context, info *RunInfo, input CallbackInput) context.Context
    OnEnd(ctx context.Context, info *RunInfo, output CallbackOutput)
    OnError(ctx context.Context, info *RunInfo, err error)
    OnStartWithStreamInput(ctx context.Context, info *RunInfo, input *schema.StreamReader[CallbackInput]) context.Context
    OnEndWithStreamOutput(ctx context.Context, info *RunInfo, output *schema.StreamReader[CallbackOutput])
}
```

## Available Implementations

| Handler | Package | Description |
|---------|---------|-------------|
| CozeLoop | `callbacks/cozeloop` | Coze observability platform |
| APMPlus | `callbacks/apmplus` | ByteDance APM tracing and metrics |
| Langfuse | `callbacks/langfuse` | Langfuse observability platform |
| Langsmith | `callbacks/langsmith` | LangSmith tracing |

## Scope

Callbacks work across all three layers of Eino:

- **Components** -- direct calls to ChatModel, Embedder, Retriever, etc. Compose and ADK handle callback injection automatically, but standalone component calls require explicit initialization (see below).
- **Compose orchestration** -- Graph, Chain, Workflow nodes are automatically traced per-node.
- **ADK Agents** -- ChatModelAgent, DeepAgent and their internal tool calls, multi-turn loops, and sub-agent invocations are all traced. Register once, the entire agent execution tree is observable.

A single global registration covers all layers. No extra wiring is needed for Compose graphs or ADK agents.

## Registration

```go
// Global -- applies to all components, compose graphs, and ADK agents
callbacks.AppendGlobalHandlers(handler1, handler2)

// Per-run -- pass via context in Compose graphs and Agent runners
ctx = callbacks.CtxWithHandlers(ctx, handler)
```

## Standalone Component Calls

When calling a component directly (outside a Graph or Agent), callbacks are not triggered unless you initialize them on the context with `InitCallbacks`. Compose graphs and ADK agents do this automatically.

```go
import (
    "github.com/cloudwego/eino/callbacks"
    "github.com/cloudwego/eino/components"
)

// Initialize callbacks for a standalone ChatModel call
ctx = callbacks.InitCallbacks(ctx, &callbacks.RunInfo{
    Name:      "my-model",
    Type:      "ChatModel",
    Component: components.ComponentOfChatModel,
}, handler)

resp, err := chatModel.Generate(ctx, messages)
// handler.OnStart / OnEnd / OnError are now triggered
```

If a component internally calls another component and wants callbacks to propagate, use `ReuseHandlers`:

```go
innerCtx := callbacks.ReuseHandlers(ctx, &callbacks.RunInfo{
    Name:      "inner-embedder",
    Type:      "Embedder",
    Component: components.ComponentOfEmbedding,
})
```

Callbacks automatically capture: component type, input/output, latency, errors, and streaming data.
