# Chain API Reference

Chain is a simplified wrapper over Graph for building linear sequential pipelines. It uses a fluent builder pattern.

## Creating a Chain

```go
import "github.com/cloudwego/eino/compose"

chain := compose.NewChain[InputType, OutputType](opts ...NewGraphOption)
```

## Append Methods

All Append methods return `*Chain[I, O]` for method chaining:

```go
chain.AppendChatModel(node model.BaseChatModel, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendChatTemplate(node prompt.ChatTemplate, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendToolsNode(node *compose.ToolsNode, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendLambda(node *compose.Lambda, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendRetriever(node retriever.Retriever, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendEmbedding(node embedding.Embedder, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendLoader(node document.Loader, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendIndexer(node indexer.Indexer, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendDocumentTransformer(node document.Transformer, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendGraph(node compose.AnyGraph, opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendPassthrough(opts ...GraphAddNodeOpt) *Chain[I, O]
chain.AppendBranch(b *ChainBranch) *Chain[I, O]
chain.AppendParallel(p *Parallel) *Chain[I, O]
```

## Compile and Run

```go
r, err := chain.Compile(ctx, opts ...GraphCompileOption)
out, err := r.Invoke(ctx, input)
stream, err := r.Stream(ctx, input)
```

## When to Use Chain vs Graph

| Feature           | Chain | Graph |
|-------------------|-------|-------|
| Linear pipeline   | Yes   | Yes   |
| Branching         | Yes (via AppendBranch) | Yes |
| Parallel          | Yes (via AppendParallel) | Yes (fan-out edges) |
| Cycles/Loops      | No    | Yes (Pregel mode) |
| Explicit edges    | No (auto-wired) | Yes |

Use Chain when: nodes flow one after another, possibly with a branch or parallel step.
Use Graph when: you need cycles, complex fan-in/fan-out, or explicit edge control.

## Complete Example: Prompt -> Model -> Parse

```go
package main

import (
    "context"
    "fmt"

    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/components/prompt"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()

    tmpl := prompt.FromMessages(schema.FString,
        schema.SystemMessage("You are a helpful assistant."),
        schema.UserMessage("{question}"),
    )

    chain := compose.NewChain[map[string]any, string]()
    chain.
        AppendChatTemplate(tmpl).
        AppendChatModel(chatModel). // chatModel implements model.BaseChatModel
        AppendLambda(compose.InvokableLambda(func(ctx context.Context, msg *schema.Message) (string, error) {
            return msg.Content, nil
        }))

    r, err := chain.Compile(ctx)
    if err != nil {
        panic(err)
    }

    out, err := r.Invoke(ctx, map[string]any{"question": "What is Go?"})
    if err != nil {
        panic(err)
    }
    fmt.Println(out)
}
```

## Parallel in Chain

`AppendParallel` runs multiple nodes concurrently, all receiving the same input. Output is `map[string]any`.

```go
parallel := compose.NewParallel()
parallel.
    AddLambda("summary", compose.InvokableLambda(summarizeFn)).
    AddLambda("keywords", compose.InvokableLambda(extractKeywordsFn))

chain := compose.NewChain[string, map[string]any]()
chain.
    AppendParallel(parallel)

r, _ := chain.Compile(ctx)
result, _ := r.Invoke(ctx, "Some long document text...")
// result = map[string]any{"summary": "...", "keywords": "..."}
```

## Branch in Chain

```go
branchCond := func(ctx context.Context, input string) (string, error) {
    if len(input) > 100 {
        return "long", nil
    }
    return "short", nil
}

branch := compose.NewChainBranch(branchCond).
    AddLambda("long", compose.InvokableLambda(longHandler)).
    AddLambda("short", compose.InvokableLambda(shortHandler))

chain := compose.NewChain[string, string]()
chain.
    AppendBranch(branch).
    AppendLambda(compose.InvokableLambda(postProcess))

r, _ := chain.Compile(ctx)
```

All branch paths must converge to the same next node in the chain (or to END). The output type of all branches must align with the next node's input type.

## Nesting

A Chain implements `AnyGraph` and can be nested inside Graph or another Chain:

```go
inner := compose.NewChain[string, string]()
inner.AppendLambda(compose.InvokableLambda(fn1))

outer := compose.NewChain[string, string]()
outer.AppendGraph(inner).AppendLambda(compose.InvokableLambda(fn2))
```
