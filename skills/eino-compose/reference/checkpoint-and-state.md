# Checkpoint and State Reference

How to manage per-request state, interrupt graph execution, and resume from checkpoints.

## State Graph

Share state across nodes within a single graph run. State is created per-request and destroyed after the run completes.

### Enabling State

```go
type myState struct {
    Messages []*schema.Message
    Counter  int
}

g := compose.NewGraph[string, string](
    compose.WithGenLocalState(func(ctx context.Context) *myState {
        return &myState{}
    }),
)
```

### Reading/Writing State with Handlers

```go
// Before node executes
preHandler := func(ctx context.Context, in string, state *myState) (string, error) {
    state.Counter++
    return in, nil
}

// After node executes
postHandler := func(ctx context.Context, out string, state *myState) (string, error) {
    state.Messages = append(state.Messages, schema.AssistantMessage(out, nil))
    return out, nil
}

g.AddLambdaNode("node", lambda,
    compose.WithStatePreHandler(preHandler),
    compose.WithStatePostHandler(postHandler),
)
```

Handler type alignment:
- `StatePreHandler[I, S]`: `I` must match the node's non-stream input type.
- `StatePostHandler[O, S]`: `O` must match the node's non-stream output type.
- `StreamStatePreHandler`: input is `*schema.StreamReader[I]`.
- `StreamStatePostHandler`: input is `*schema.StreamReader[O]`.

### ProcessState (inside a node)

```go
lambda := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
    err := compose.ProcessState[*myState](ctx, func(_ context.Context, s *myState) error {
        s.Counter++
        return nil
    })
    return in, err
})
```

State access is mutex-protected by the framework.

## Checkpoint and Interrupt

Checkpoints save graph execution state so it can be resumed later. Interrupts pause execution at specific points.

### CheckpointStore Interface

```go
type CheckpointStore interface {
    Get(ctx context.Context, key string) (value []byte, existed bool, err error)
    Set(ctx context.Context, key string, value []byte) (err error)
}
```

You must implement this interface (e.g., backed by Redis, a database, or in-memory store).

### Enabling Checkpoints

```go
r, err := g.Compile(ctx,
    compose.WithCheckPointStore(store),
    compose.WithInterruptBeforeNodes([]string{"human_review"}),
    compose.WithInterruptAfterNodes([]string{"data_fetch"}),
)
```

### Running with Checkpoint ID

```go
checkpointID := "request-123"

// First run: will interrupt before "human_review"
result, err := r.Invoke(ctx, input, compose.WithCheckPointID(checkpointID))
if info, ok := compose.ExtractInterruptInfo(err); ok {
    // Graph interrupted. info contains state, interrupt nodes, etc.
    // result is zero-value (not meaningful)
}
```

### Resuming

```go
// Resume from checkpoint (input is ignored when resuming)
result, err = r.Invoke(ctx, "", compose.WithCheckPointID(checkpointID))
```

### Modifying State Before Resume

```go
result, err = r.Invoke(ctx, "",
    compose.WithCheckPointID(checkpointID),
    compose.WithStateModifier(func(ctx context.Context, path compose.NodePath, state any) error {
        s := state.(*myState)
        s.Approved = true
        return nil
    }),
)
```

## Dynamic Interrupt

A node can trigger interrupt at runtime by returning a special error:

### Basic (v0.7.0+)

```go
lambda := compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
    if needsApproval(in) {
        return "", compose.Interrupt(ctx, &ApprovalRequest{Content: in})
    }
    return process(in), nil
})
```

### Stateful Interrupt (preserves local state across resume)

```go
return "", compose.StatefulInterrupt(ctx, &ApprovalInfo{
    ToolName: "search",
    Args:     localState,
}, localState) // third param is local state to persist across resume
```

### Checking Interrupt State on Resume

```go
wasInterrupted, _, storedState := compose.GetInterruptState[string](ctx)
if wasInterrupted {
    // Node was previously interrupted; storedState has the persisted local state
}

isResumeTarget, hasData, data := compose.GetResumeContext[*ApprovalResult](ctx)
if isResumeTarget && hasData {
    // This node was explicitly resumed with data
    if data.Approved {
        // proceed
    }
}
```

## External Interrupt (Cancel With Checkpoint)

Interrupt a running graph from outside (e.g., graceful shutdown):

```go
ctx, interrupt := compose.WithGraphInterrupt(context.Background())

go func() {
    // Run graph with interruptible context
    result, err = r.Invoke(ctx, input, compose.WithCheckPointID(id))
}()

// Later, trigger interrupt from outside
interrupt(compose.WithGraphInterruptTimeout(5 * time.Second))
```

## Registering Custom Types for Serialization

Checkpoint serialization requires type registration for custom structs:

```go
import "github.com/cloudwego/eino/schema"

func init() {
    schema.RegisterName[*MyState]("my_state_v1")
}
```

- The registered name is used as a serialization key. Changing it after checkpoints have been persisted will make those checkpoints unrecoverable.
- Unexported fields are not serialized.
- The default serializer uses JSON (via sonic). For better performance, use a gob-based serializer via `compose.WithSerializer()`. The ADK layer already uses gob internally.
- `schema.RegisterName` registers the type with both gob and the internal JSON serializer, so it works with either approach.

## Stream Checkpoint

When checkpointing streams, register a concat function so the framework can combine chunks:

```go
compose.RegisterStreamChunkConcatFunc(func(chunks []MyChunk) (MyChunk, error) {
    var result MyChunk
    for _, c := range chunks {
        result.Body += c.Body
    }
    return result, nil
})
```

Built-in concat: `*schema.Message`, `[]*schema.Message`, `string`.

## Complete Example: Graph with Checkpoint

```go
package main

import (
    "context"
    "fmt"

    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

type reviewState struct {
    Approved bool
    Content  string
}

func main() {
    ctx := context.Background()
    store := NewMyCheckpointStore() // implements compose.CheckpointStore

    g := compose.NewGraph[string, string](
        compose.WithGenLocalState(func(ctx context.Context) *reviewState {
            return &reviewState{}
        }),
    )

    _ = g.AddLambdaNode("prepare", compose.InvokableLambda(
        func(ctx context.Context, in string) (string, error) {
            compose.ProcessState[*reviewState](ctx, func(_ context.Context, s *reviewState) error {
                s.Content = in
                return nil
            })
            return in, nil
        },
    ))

    _ = g.AddLambdaNode("review", compose.InvokableLambda(
        func(ctx context.Context, in string) (string, error) {
            return "reviewed: " + in, nil
        },
    ))

    _ = g.AddEdge(compose.START, "prepare")
    _ = g.AddEdge("prepare", "review")
    _ = g.AddEdge("review", compose.END)

    r, _ := g.Compile(ctx,
        compose.WithCheckPointStore(store),
        compose.WithInterruptBeforeNodes([]string{"review"}),
    )

    // First run: interrupts before "review"
    id := "session-1"
    _, err := r.Invoke(ctx, "hello", compose.WithCheckPointID(id))
    if info, ok := compose.ExtractInterruptInfo(err); ok {
        fmt.Printf("Interrupted before: %v\n", info.BeforeNodes)
    }

    // Resume with state modification
    result, _ := r.Invoke(ctx, "",
        compose.WithCheckPointID(id),
        compose.WithStateModifier(func(_ context.Context, _ compose.NodePath, state any) error {
            state.(*reviewState).Approved = true
            return nil
        }),
    )
    fmt.Println(result) // "reviewed: hello"
}
```

## Nested Graph Checkpoints

Enable interrupt in sub-graphs via `WithGraphCompileOptions`:

```go
g.AddGraphNode("sub", subGraph, compose.WithGraphCompileOptions(
    compose.WithInterruptAfterNodes([]string{"inner_node"}),
))

g.Compile(ctx, compose.WithCheckPointStore(store))
```

The parent graph must have a checkpoint store. When resuming from a sub-graph interrupt, `StateModifier` receives the sub-graph's state.
