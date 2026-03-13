---
name: eino-compose
description: Eino orchestration with Graph, Chain, and Workflow. Use when a user needs to build multi-step pipelines, compose components into executable graphs, handle streaming between nodes, use branching or parallel execution, manage state with checkpoints, or understand the Runnable abstraction. Covers Graph (directed graph with cycles), Chain (linear sequential), and Workflow (DAG with field mapping).
---

## Orchestration Overview

The `github.com/cloudwego/eino/compose` package provides three orchestration APIs:

| API      | Topology          | Cycles | Type Alignment       |
|----------|-------------------|-------|----------------------|
| Graph    | Directed graph    | Yes (Pregel mode) / No (DAG mode) | Whole input/output   |
| Chain    | Linear sequence   | No    | Whole input/output   |
| Workflow | DAG               | No    | Field-level mapping  |

*Chain is implemented on top of Graph in Pregel mode but enforces linear topology.

All three compile into `Runnable[I, O]` which exposes Invoke, Stream, Collect, and Transform.

```go
import "github.com/cloudwego/eino/compose"
```

## Choosing an API

- **Chain** -- sequential pipeline: prompt -> model -> parser. Simplest API.
- **Graph** -- need branching, loops (ReAct agent), or fan-out/fan-in.
- **Workflow** -- need field-level mapping between nodes with different struct types; DAG only.

## Graph Quick Reference

```go
g := compose.NewGraph[InputType, OutputType]()

// Add nodes
g.AddChatModelNode("model", chatModel)
g.AddChatTemplateNode("tmpl", tmpl)
g.AddToolsNode("tools", toolsNode)
g.AddLambdaNode("fn", compose.InvokableLambda(myFunc))
g.AddPassthroughNode("pass")
g.AddGraphNode("sub", subGraph)

// Connect nodes
g.AddEdge(compose.START, "tmpl")
g.AddEdge("tmpl", "model")
g.AddEdge("model", compose.END)

// Branch (conditional routing)
branch := compose.NewGraphBranch(conditionFn, map[string]bool{"a": true, "b": true})
g.AddBranch("model", branch)

// Compile and run
r, err := g.Compile(ctx)
out, err := r.Invoke(ctx, input)
```

## Chain Quick Reference

```go
chain := compose.NewChain[InputType, OutputType]()
chain.
    AppendChatTemplate(tmpl).
    AppendChatModel(model).
    AppendLambda(compose.InvokableLambda(parseFn))

r, err := chain.Compile(ctx)
out, err := r.Invoke(ctx, input)
```

Append methods: `AppendChatModel`, `AppendChatTemplate`, `AppendToolsNode`, `AppendLambda`, `AppendGraph`, `AppendParallel`, `AppendBranch`, `AppendPassthrough`, `AppendRetriever`, `AppendEmbedding`, `AppendLoader`, `AppendIndexer`, `AppendDocumentTransformer`.

## Workflow Quick Reference

```go
wf := compose.NewWorkflow[InputStruct, OutputStruct]()

wf.AddLambdaNode("node1", compose.InvokableLambda(fn1)).
    AddInput(compose.START, compose.MapFields("FieldA", "InputField"))

wf.AddLambdaNode("node2", compose.InvokableLambda(fn2)).
    AddInput("node1", compose.ToField("Result"))

wf.End().AddInput("node2")

r, err := wf.Compile(ctx)
```

Field mapping helpers: `MapFields`, `ToField`, `FromField`, `MapFieldPaths`, `ToFieldPath`, `FromFieldPath`.

## Stream Programming

Four interaction modes on `Runnable[I, O]`:

| Mode      | Input          | Output              | Lambda Constructor           |
|-----------|----------------|---------------------|------------------------------|
| Invoke    | `I`            | `O`                 | `compose.InvokableLambda`    |
| Stream    | `I`            | `*StreamReader[O]`  | `compose.StreamableLambda`   |
| Collect   | `*StreamReader[I]` | `O`             | `compose.CollectableLambda`  |
| Transform | `*StreamReader[I]` | `*StreamReader[O]` | `compose.TransformableLambda` |

Framework auto-converts between modes:
- **Invoke** call: all internal nodes run in Invoke mode.
- **Stream/Collect/Transform** call: all internal nodes run in Transform mode; missing modes are auto-filled.

Stream primitives live in `github.com/cloudwego/eino/schema`:

```go
sr, sw := schema.Pipe[T](capacity)
// sw.Send(chunk, nil); sw.Close()
// chunk, err := sr.Recv(); sr.Close()
```

## Compile & Run

```go
r, err := g.Compile(ctx,
    compose.WithGraphName("my_graph"),
    compose.WithNodeTriggerMode(compose.AllPredecessor), // DAG mode
)

// Non-streaming
out, err := r.Invoke(ctx, input)

// Streaming
stream, err := r.Stream(ctx, input)
defer stream.Close()
for {
    chunk, err := stream.Recv()
    if err == io.EOF { break }
    if err != nil { return err }
    process(chunk)
}
```

## State Graph

Share state across nodes within a single request:

```go
g := compose.NewGraph[string, string](compose.WithGenLocalState(func(ctx context.Context) *MyState {
    return &MyState{}
}))

g.AddLambdaNode("node", lambda,
    compose.WithStatePreHandler(func(ctx context.Context, in string, state *MyState) (string, error) {
        // read/write state before node executes
        return in, nil
    }),
    compose.WithStatePostHandler(func(ctx context.Context, out string, state *MyState) (string, error) {
        // read/write state after node executes
        return out, nil
    }),
)
```

## Instructions to Agent

When helping users build orchestration:

1. Default to **Graph** for most use cases. Use Chain only for simple linear pipelines. Use Workflow when field-level mapping between different struct types is needed.
2. Always show the **Compile** step -- `g.Compile(ctx)` returns `Runnable[I,O]`.
3. Always **close StreamReaders** -- use `defer sr.Close()` immediately after obtaining a stream.
4. Upstream output type must match downstream input type (or use `WithInputKey`/`WithOutputKey` for map conversion).
5. For cyclic graphs (e.g., ReAct agent), use default Pregel mode (`AnyPredecessor`). For DAGs, set `AllPredecessor`.
6. Use `compose.WithCallbacks(handler)` to inject logging/tracing at runtime.
7. Use `compose.WithCheckPointStore(store)` with interrupt nodes for pause/resume workflows.

## Reference Files

Read these files on-demand for detailed API, examples, and advanced usage:

- [reference/graph.md](reference/graph.md) -- Full Graph API, branches, state graph, cyclic graph, complete ReAct example
- [reference/chain.md](reference/chain.md) -- Chain API, when to use, parallel/branch in chain
- [reference/workflow.md](reference/workflow.md) -- Workflow API, field-level mapping helpers, constraints
- [reference/stream.md](reference/stream.md) -- StreamReader/Writer, Pipe/Copy/Merge, lambda constructors, auto-conversion rules
- [reference/callback.md](reference/callback.md) -- Callback timings, handler registration, trigger rules, tracing example
- [reference/call-option.md](reference/call-option.md) -- Per-request CallOption, component-type options, node targeting
- [reference/checkpoint-and-state.md](reference/checkpoint-and-state.md) -- CheckPointStore, interrupt/resume, state management
