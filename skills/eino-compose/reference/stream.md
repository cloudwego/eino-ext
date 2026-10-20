# Stream Programming Reference

Eino stream primitives and how streaming works within Graph/Chain/Workflow orchestration.

## StreamReader and StreamWriter

From `github.com/cloudwego/eino/schema`:

```go
// Create a stream pair with buffered capacity
sr, sw := schema.Pipe[T](capacity int) (*StreamReader[T], *StreamWriter[T])

// Writer side
closed := sw.Send(chunk T, err error) bool  // returns true if reader closed
sw.Close()                                   // signals EOF to reader

// Reader side
chunk, err := sr.Recv() (T, error)   // io.EOF when stream ends
sr.Close()                           // release resources, signal writer to stop
```

### Usage Pattern

```go
sr, sw := schema.Pipe[string](10)

go func() {
    defer sw.Close()
    sw.Send("hello ", nil)
    sw.Send("world", nil)
}()

defer sr.Close()
for {
    chunk, err := sr.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }
    fmt.Print(chunk)
}
```

## Stream Operations

### Copy (fan-out)

```go
copies := sr.Copy(n int) []*StreamReader[T]
// Each copy independently reads every element
// Original reader becomes unusable after Copy
```

**WARNING — Copy duplicates readers, NOT elements.** For pointer elements (e.g. `*schema.Message`), all branches receive the same object and may consume it concurrently. Treat received elements as read-only; never mutate them in place (especially map fields like `msg.Extra[k] = v`), or you risk concurrent map read/write panics. See "Never Mutate Stream Elements In Place" below.

### Merge (fan-in)

```go
merged := schema.MergeStreamReaders(srs []*StreamReader[T]) *StreamReader[T]
// Interleaves elements from all sources; EOF after all exhausted

// Named merge (emits SourceEOF per-source)
merged := schema.MergeNamedStreamReaders(map[string]*StreamReader[T]{
    "a": srA,
    "b": srB,
})
```

### Convert / Filter

```go
strReader := schema.StreamReaderWithConvert(intReader,
    func(i int) (string, error) {
        if i == 0 {
            return "", schema.ErrNoValue // skip this element
        }
        return fmt.Sprintf("val_%d", i), nil
    },
)
```

The convert callback must NOT modify its input in place — the element may be shared with other stream branches (see Copy above). Return a new value instead.

### Never Mutate Stream Elements In Place

Stream elements flowing through Graph/Chain/Workflow are routinely fanned out via `Copy` (callbacks, branches), so a `*schema.Message` / `*schema.AgenticMessage` you receive in a convert callback, callback handler, or middleware may be read concurrently by other goroutines. In-place writes — especially to map fields like `Extra` — cause `concurrent map read/write` panics.

Always use copy-on-write:

```go
// WRONG: mutates a shared message
schema.StreamReaderWithConvert(sr, func(msg *schema.Message) (*schema.Message, error) {
    msg.Extra["trace_id"] = traceID // may panic: concurrent map read/write
    return msg, nil
})

// RIGHT: shallow-copy the message, clone the map you change, modify the clone
schema.StreamReaderWithConvert(sr, func(msg *schema.Message) (*schema.Message, error) {
    cp := *msg // shallow copy
    extra := make(map[string]any, len(msg.Extra)+1)
    for k, v := range msg.Extra {
        extra[k] = v
    }
    extra["trace_id"] = traceID
    cp.Extra = extra
    return &cp, nil
})
```

The same rule applies to any map/slice field you modify (`ToolCalls`, `ContentBlocks`, nested `Extra` on `ToolCall`/`ContentBlock`, ...): clone the field before writing, and return the copy.

### From Array

```go
sr := schema.StreamReaderFromArray([]string{"a", "b", "c"})
```

## Four Interaction Modes

The `Runnable[I, O]` interface exposes four modes:

```go
type Runnable[I, O any] interface {
    Invoke(ctx context.Context, input I, opts ...Option) (O, error)
    Stream(ctx context.Context, input I, opts ...Option) (*schema.StreamReader[O], error)
    Collect(ctx context.Context, input *schema.StreamReader[I], opts ...Option) (O, error)
    Transform(ctx context.Context, input *schema.StreamReader[I], opts ...Option) (*schema.StreamReader[O], error)
}
```

## Lambda Constructors

```go
// Non-streaming: I -> O
compose.InvokableLambda(func(ctx context.Context, in I) (O, error))

// Streaming output: I -> StreamReader[O]
compose.StreamableLambda(func(ctx context.Context, in I) (*schema.StreamReader[O], error))

// Streaming input: StreamReader[I] -> O
compose.CollectableLambda(func(ctx context.Context, in *schema.StreamReader[I]) (O, error))

// Bidirectional streaming: StreamReader[I] -> StreamReader[O]
compose.TransformableLambda(func(ctx context.Context, in *schema.StreamReader[I]) (*schema.StreamReader[O], error))

// Combine multiple modes
compose.AnyLambda(invokeFn, streamFn, collectFn, transformFn)
```

## Auto-Conversion Rules

The framework automatically converts between streaming modes:

### When Graph is called with Invoke

All internal nodes run in Invoke mode. If a node only implements Stream, the framework auto-concats the output stream into a single value.

### When Graph is called with Stream/Collect/Transform

All internal nodes run in Transform mode. Missing modes are auto-filled:

| Node implements | Framework wraps to Transform by |
|-----------------|-------------------------------|
| Stream          | Concat input stream, use Stream output |
| Collect         | Use Collect input, box output as single-chunk stream |
| Invoke          | Concat input stream, box output as single-chunk stream |

### Auto-concat built-in types

The framework can automatically concat `StreamReader[T]` into `T` for:

- `*schema.Message` (via `schema.ConcatMessages()`)
- `string` (concatenation)
- `[]*schema.Message`
- `map[string]any` (merge by key)

For custom types, register a concat function:

```go
compose.RegisterStreamChunkConcatFunc(func(chunks []MyType) (MyType, error) {
    // combine chunks into one MyType
    return combined, nil
})
```

## Complete Streaming Example

```go
package main

import (
    "context"
    "fmt"
    "io"

    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()

    g := compose.NewGraph[string, string]()

    // StreamableLambda: takes string, returns stream of string chunks
    _ = g.AddLambdaNode("streamer", compose.StreamableLambda(
        func(ctx context.Context, in string) (*schema.StreamReader[string], error) {
            sr, sw := schema.Pipe[string](10)
            go func() {
                defer sw.Close()
                for _, word := range []string{"Hello ", "streaming ", "world!"} {
                    sw.Send(word, nil)
                }
            }()
            return sr, nil
        },
    ))

    // TransformableLambda: transforms stream chunk by chunk
    _ = g.AddLambdaNode("upper", compose.TransformableLambda(
        func(ctx context.Context, in *schema.StreamReader[string]) (*schema.StreamReader[string], error) {
            return schema.StreamReaderWithConvert(in, func(s string) (string, error) {
                return "[" + s + "]", nil
            }), nil
        },
    ))

    _ = g.AddEdge(compose.START, "streamer")
    _ = g.AddEdge("streamer", "upper")
    _ = g.AddEdge("upper", compose.END)

    r, _ := g.Compile(ctx)

    // Stream call: get output as a stream
    stream, _ := r.Stream(ctx, "input")
    defer stream.Close()

    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            break
        }
        fmt.Print(chunk) // prints: [Hello ][streaming ][world!]
    }
}
```

## ErrNoValue

`schema.ErrNoValue` is a sentinel used in `StreamReaderWithConvert` to skip elements:

```go
filtered := schema.StreamReaderWithConvert(sr, func(msg *schema.Message) (*schema.Message, error) {
    if msg.Content == "" {
        return nil, schema.ErrNoValue // skip empty messages
    }
    return msg, nil
})
```

Do NOT use `ErrNoValue` in any other context.
