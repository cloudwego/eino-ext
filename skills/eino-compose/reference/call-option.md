# CallOption Reference

Pass per-request configuration to specific nodes at invocation time via `compose.Option`.

## Component-Type Options

Apply options to all nodes of a given component type:

```go
out, _ := r.Invoke(ctx, input,
    compose.WithChatModelOption(model.WithTemperature(0.5)),
    compose.WithChatModelOption(model.WithMaxTokens(1024)),
    compose.WithEmbeddingOption(embedding.WithModel("text-embedding-3-small")),
)
```

Available helpers: `WithChatModelOption`, `WithToolOption`, `WithRetrieverOption`, `WithEmbeddingOption`, `WithIndexerOption`, `WithLoaderOption`, `WithChatTemplateOption`, `WithDocTransformerOption`.

## Node-Targeted Options

Apply options to a specific named node:

```go
out, _ := r.Invoke(ctx, input,
    compose.WithChatModelOption(model.WithTemperature(0.9)).DesignateNode("CreativeModel"),
    compose.WithChatModelOption(model.WithTemperature(0.1)).DesignateNode("FactualModel"),
)
```

## Nested Graph Targeting

Target a node inside a nested sub-graph:

```go
compose.WithChatModelOption(model.WithTemperature(0.5)).
    DesignateNodeWithPath(compose.NewNodePath("SubGraph", "InnerModel"))
```

## Option Scoping

- **No `DesignateNode`**: applies to all matching component-type nodes, including those in nested graphs.
- **`DesignateNode("Key")`**: only the named node in the top-level graph.
- **`DesignateNodeWithPath(path)`**: a specific node at any nesting depth.
