# Workflow API Reference

Workflow provides DAG orchestration with field-level mapping between nodes. Unlike Graph which requires whole input/output type alignment, Workflow maps individual struct fields between nodes.

## Creating a Workflow

```go
import "github.com/cloudwego/eino/compose"

wf := compose.NewWorkflow[InputType, OutputType](opts ...NewGraphOption)
```

## Adding Nodes

Same node types as Graph. Returns `*WorkflowNode` for method chaining:

```go
wf.AddChatModelNode(key, chatModel, opts...) *WorkflowNode
wf.AddChatTemplateNode(key, tmpl, opts...) *WorkflowNode
wf.AddToolsNode(key, toolsNode, opts...) *WorkflowNode
wf.AddLambdaNode(key, lambda, opts...) *WorkflowNode
wf.AddRetrieverNode(key, retriever, opts...) *WorkflowNode
wf.AddEmbeddingNode(key, embedder, opts...) *WorkflowNode
wf.AddIndexerNode(key, indexer, opts...) *WorkflowNode
wf.AddLoaderNode(key, loader, opts...) *WorkflowNode
wf.AddDocumentTransformerNode(key, transformer, opts...) *WorkflowNode
wf.AddGraphNode(key, graph, opts...) *WorkflowNode
wf.AddPassthroughNode(key, opts...) *WorkflowNode
```

## Field Mapping with AddInput

`AddInput` declares both a control dependency and a data dependency:

```go
node.AddInput(fromNodeKey string, mappings ...*FieldMapping) *WorkflowNode
```

### Mapping Helpers

```go
// Top-level field to top-level field
compose.MapFields("SourceField", "TargetField")

// Full output to a top-level field
compose.ToField("TargetField")

// Top-level field to full input
compose.FromField("SourceField")

// Nested field paths
compose.MapFieldPaths([]string{"Outer", "Inner"}, []string{"TargetField"})
compose.ToFieldPath([]string{"Target", "Nested"})
compose.FromFieldPath([]string{"Source", "Nested"})
```

### No mapping (whole output -> whole input)

```go
node.AddInput(compose.START) // maps all of START output to this node's input
```

## Control-Only and Data-Only Dependencies

### Data-only (no control dependency)

```go
node.AddInputWithOptions(fromNodeKey, []*compose.FieldMapping{
    compose.MapFields("Price", "InputPrice"),
}, compose.WithNoDirectDependency())
```

The source node's completion does NOT trigger this node. Data is available only if a control path exists through other nodes.

### Control-only (no data)

```go
node.AddDependency("Predecessor")
```

The predecessor must complete before this node runs, but no data is passed.

## Setting the END Node

```go
wf.End().AddInput("LastNode")
// or with field mapping:
wf.End().
    AddInput("NodeA", compose.ToField("ResultA")).
    AddInput("NodeB", compose.ToField("ResultB"))
```

## Static Values

Inject constant values into a node's input fields:

```go
wf.AddLambdaNode("Bidder", compose.InvokableLambda(bidderFn)).
    AddInput(compose.START, compose.ToField("Price")).
    SetStaticValue([]string{"Budget"}, 3.0)
```

`SetStaticValue(path FieldPath, value any)` sets the value at the given field path.

## Branches

Same branch API as Graph, but branches in Workflow are control-only (no data passing):

```go
wf.AddBranch("SourceNode", compose.NewGraphBranch(
    func(ctx context.Context, in float64) (string, error) {
        if in > threshold {
            return compose.END, nil
        }
        return "NextNode", nil
    },
    map[string]bool{compose.END: true, "NextNode": true},
))
```

Downstream nodes of a branch get their data through `AddInput`/`AddInputWithOptions`, not from the branch source.

## Compile and Run

```go
r, err := wf.Compile(ctx, opts ...GraphCompileOption)
out, err := r.Invoke(ctx, input)
stream, err := r.Stream(ctx, input)
```

## When to Use Workflow vs Graph

| Feature                    | Workflow | Graph |
|----------------------------|----------|-------|
| Field-level mapping        | Yes      | No    |
| Different node I/O types   | Yes      | Needs adapters |
| Cycles                     | No       | Yes (Pregel) |
| Control/data separation    | Yes      | No    |
| NodeTriggerMode            | Fixed AllPredecessor | Configurable |

Use Workflow when: nodes have different input/output struct types and you want direct field mapping without glue lambdas.

## Complete Example: Parallel Processing with Field Mapping

```go
package main

import (
    "context"
    "strings"

    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

type counterInput struct {
    FullStr string
    SubStr  string
}

func main() {
    ctx := context.Background()

    wordCounter := func(ctx context.Context, c counterInput) (int, error) {
        return strings.Count(c.FullStr, c.SubStr), nil
    }

    type input struct {
        *schema.Message
        SubStr string
    }

    wf := compose.NewWorkflow[input, map[string]any]()

    // C1 counts SubStr in Message.Content
    wf.AddLambdaNode("C1", compose.InvokableLambda(wordCounter)).
        AddInput(compose.START,
            compose.MapFields("SubStr", "SubStr"),
            compose.MapFieldPaths([]string{"Message", "Content"}, []string{"FullStr"}))

    // C2 counts SubStr in Message.ReasoningContent
    wf.AddLambdaNode("C2", compose.InvokableLambda(wordCounter)).
        AddInput(compose.START,
            compose.MapFields("SubStr", "SubStr"),
            compose.MapFieldPaths([]string{"Message", "ReasoningContent"}, []string{"FullStr"}))

    wf.End().
        AddInput("C1", compose.ToField("ContentCount")).
        AddInput("C2", compose.ToField("ReasoningCount"))

    r, err := wf.Compile(ctx)
    if err != nil {
        panic(err)
    }

    result, _ := r.Invoke(ctx, input{
        Message: &schema.Message{
            Content:          "Hello world!",
            ReasoningContent: "I need to say something meaningful",
        },
        SubStr: "o",
    })
    // result = map[string]any{"ContentCount": 2, "ReasoningCount": 1}
}
```

## Constraints

- Map keys must be `string` or types convertible to `string`.
- `WithNodeTriggerMode` and `WithMaxRunSteps` are not supported (fixed to AllPredecessor, no cycles).
- Cannot map multiple sources to the same target field.
- Struct fields used in mapping must be exported (reflection-based).
