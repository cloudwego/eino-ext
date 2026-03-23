# Graph API Reference

Full API reference for `compose.Graph` -- the most flexible orchestration primitive in Eino, supporting directed graphs with optional cycles.

## Creating a Graph

```go
import "github.com/cloudwego/eino/compose"

// I = graph input type, O = graph output type
g := compose.NewGraph[I, O](opts ...NewGraphOption)
```

## Adding Nodes

Every `Add*Node` method takes a string key, the component instance, and optional `GraphAddNodeOpt`:

```go
// Built-in component nodes
g.AddChatModelNode(key string, node model.BaseChatModel, opts ...GraphAddNodeOpt) error
g.AddChatTemplateNode(key string, node prompt.ChatTemplate, opts ...GraphAddNodeOpt) error
g.AddToolsNode(key string, node *compose.ToolsNode, opts ...GraphAddNodeOpt) error
g.AddRetrieverNode(key string, node retriever.Retriever, opts ...GraphAddNodeOpt) error
g.AddEmbeddingNode(key string, node embedding.Embedder, opts ...GraphAddNodeOpt) error
g.AddIndexerNode(key string, node indexer.Indexer, opts ...GraphAddNodeOpt) error
g.AddLoaderNode(key string, node document.Loader, opts ...GraphAddNodeOpt) error
g.AddDocumentTransformerNode(key string, node document.Transformer, opts ...GraphAddNodeOpt) error

// Lambda nodes (custom logic)
g.AddLambdaNode(key string, node *compose.Lambda, opts ...GraphAddNodeOpt) error

// Sub-graph node
g.AddGraphNode(key string, node compose.AnyGraph, opts ...GraphAddNodeOpt) error

// Passthrough node (forwards input unchanged)
g.AddPassthroughNode(key string, opts ...GraphAddNodeOpt) error
```

### Node Options

```go
compose.WithNodeName("display_name")    // Sets RunInfo.Name for callbacks
compose.WithInputKey("key")             // Extract map[string]any value by key as node input
compose.WithOutputKey("key")            // Wrap node output into map[string]any{key: output}
compose.WithStatePreHandler(handler)    // Pre-process with state access
compose.WithStatePostHandler(handler)   // Post-process with state access
compose.WithStreamStatePreHandler(h)    // Stream-aware pre-handler
compose.WithStreamStatePostHandler(h)   // Stream-aware post-handler
compose.WithGraphCompileOptions(opts)   // Pass compile options to sub-graph nodes
```

## Edges

```go
g.AddEdge(startNode, endNode string) error
```

Use constants `compose.START` and `compose.END` for graph entry/exit:

```go
g.AddEdge(compose.START, "first_node")
g.AddEdge("last_node", compose.END)
```

Type alignment: the output type of `startNode` must be assignable to the input type of `endNode`. This is checked at `AddEdge` time.

## Branches

Branches enable conditional routing. Only the selected branch executes.

### Single-select branch

```go
condition := func(ctx context.Context, in OutputType) (string, error) {
    if shouldGoLeft(in) {
        return "left", nil
    }
    return "right", nil
}
endNodes := map[string]bool{"left": true, "right": true}

branch := compose.NewGraphBranch(condition, endNodes)
g.AddBranch("source_node", branch)
```

### Multi-select branch

```go
condition := func(ctx context.Context, in OutputType) (map[string]bool, error) {
    return map[string]bool{"a": true, "b": shouldRunB(in)}, nil
}
branch := compose.NewGraphMultiBranch(condition, map[string]bool{"a": true, "b": true})
g.AddBranch("source_node", branch)
```

### Stream-aware branch

Use `NewStreamGraphBranch` when the branch can decide from partial stream input:

```go
branch := compose.NewStreamGraphBranch(
    func(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (string, error) {
        msg, err := sr.Recv()
        if err != nil { return "", err }
        defer sr.Close()
        if len(msg.ToolCalls) > 0 {
            return "tools", nil
        }
        return compose.END, nil
    },
    map[string]bool{"tools": true, compose.END: true},
)
```

## State Graph

Enable per-request state shared across all nodes:

```go
type agentState struct {
    Messages []*schema.Message
    Step     int
}

g := compose.NewGraph[string, string](
    compose.WithGenLocalState(func(ctx context.Context) *agentState {
        return &agentState{}
    }),
)
```

### StatePreHandler / StatePostHandler

```go
preHandler := func(ctx context.Context, in []*schema.Message, state *agentState) ([]*schema.Message, error) {
    state.Messages = append(state.Messages, in...)
    return in, nil
}

postHandler := func(ctx context.Context, out *schema.Message, state *agentState) (*schema.Message, error) {
    state.Step++
    return out, nil
}

g.AddChatModelNode("model", chatModel,
    compose.WithStatePreHandler(preHandler),
    compose.WithStatePostHandler(postHandler),
)
```

### ProcessState (inside a node)

```go
lambda := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
    err := compose.ProcessState[*agentState](ctx, func(_ context.Context, s *agentState) error {
        s.Messages = append(s.Messages, schema.UserMessage(in))
        return nil
    })
    return in, err
})
```

## NodeTriggerMode (Pregel vs DAG)

```go
// Pregel mode (default): supports cycles, AnyPredecessor triggers
r, _ := g.Compile(ctx) // default: Pregel

// DAG mode: no cycles, AllPredecessor triggers
r, _ := g.Compile(ctx, compose.WithNodeTriggerMode(compose.AllPredecessor))
```

- **AnyPredecessor (Pregel)**: a node runs when any predecessor completes. Supports cycles. Nodes run in SuperSteps.
- **AllPredecessor (DAG)**: a node runs only after all predecessors complete. No cycles allowed.

## Compile Options

```go
g.Compile(ctx,
    compose.WithGraphName("my_graph"),
    compose.WithNodeTriggerMode(compose.AllPredecessor),
    compose.WithMaxRunSteps(20),             // Pregel mode only
    compose.WithCheckPointStore(store),       // Enable checkpointing
    compose.WithInterruptBeforeNodes([]string{"node1"}),
    compose.WithInterruptAfterNodes([]string{"node2"}),
    compose.WithEagerExecution(),             // DAG mode: run nodes as soon as ready
)
```

## Complete Example: ReAct Agent Graph

A minimal ReAct loop: model → branch (has tool calls?) → tools → model cycle.

```go
func buildReActGraph(ctx context.Context, cm model.ToolCallingChatModel) (compose.Runnable[map[string]any, *schema.Message], error) {
    // Create ChatTemplate inline
    tpl := prompt.FromMessages(schema.FString,
        schema.SystemMessage("You are a helpful assistant."),
        schema.UserMessage("help me to book a ticket."),
    )

    // Create ToolsNode with tools
    tn, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
        Tools: []tool.BaseTool{bookTicketTool},
    })
    if err != nil {
        return nil, err
    }

    g := compose.NewGraph[map[string]any, *schema.Message]()

    _ = g.AddChatTemplateNode("ChatTemplate", tpl)
    _ = g.AddChatModelNode("ChatModel", cm)
    _ = g.AddToolsNode("ToolsNode", tn)

    // Edges: START -> ChatTemplate -> ChatModel
    _ = g.AddEdge(compose.START, "ChatTemplate")
    _ = g.AddEdge("ChatTemplate", "ChatModel")

    // Loop: ToolsNode -> ChatModel (creates a cycle)
    _ = g.AddEdge("ToolsNode", "ChatModel")

    // Branch after model: if tool calls → ToolsNode; otherwise → END
    // Stream branch: must consume all chunks to determine if tool calls exist,
    // because ToolCalls may appear in any chunk, not necessarily the first one.
    _ = g.AddBranch("ChatModel", compose.NewStreamGraphBranch(
        func(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (string, error) {
            defer sr.Close()
            for {
                msg, err := sr.Recv()
                if err != nil {
                    break
                }
                if len(msg.ToolCalls) > 0 {
                    return "ToolsNode", nil
                }
            }
            return compose.END, nil
        },
        map[string]bool{"ToolsNode": true, compose.END: true},
    ))

    return g.Compile(ctx)
}
```

> For state management and checkpoint/interrupt support in this pattern, see [checkpoint-and-state.md](checkpoint-and-state.md).

## Fan-in (Multiple Predecessors)

When multiple edges converge on one node, their outputs are merged:

- Default: all predecessors must output `map[string]any` with non-overlapping keys.
- Use `WithOutputKey("key")` to wrap a node's output into a map.
- Register custom merge: `compose.RegisterValuesMergeFunc[T](func([]T) (T, error))`.
