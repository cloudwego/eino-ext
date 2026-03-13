# Schema -- Core Data Types

The `github.com/cloudwego/eino/schema` package defines shared types used across all Eino layers (components, compose, ADK).

## Message

The fundamental unit of conversation data.

```go
type Message struct {
    Role                     RoleType            // system, user, assistant, tool
    Content                  string              // text content
    UserInputMultiContent    []MessageInputPart  // multimodal input from user (images, audio, etc.)
    AssistantGenMultiContent []MessageOutputPart // multi-part output from model (e.g., reasoning + text, audio + text). When non-empty, prefer this over Content/ReasoningContent.
    ToolCalls                []ToolCall          // tool calls requested by model (assistant only)
    ToolCallID               string              // identifies which tool call this message responds to (tool only)
    ToolName                 string              // tool name (tool only)
    ReasoningContent         string              // model's reasoning/thinking content
    ResponseMeta             *ResponseMeta       // model response metadata (token usage, finish reason, etc.)
}
```

### Roles

| Role | Constant | Purpose |
|------|----------|---------|
| System | `schema.System` | Instructions to the model |
| User | `schema.User` | User input |
| Assistant | `schema.Assistant` | Model output |
| Tool | `schema.Tool` | Tool execution result |

### Constructors

```go
schema.SystemMessage("You are a helpful assistant.")
schema.UserMessage("Hello")
schema.AssistantMessage("Hi there!", nil)       // content, toolCalls
schema.ToolMessage("result text", "call_id_1")  // content, toolCallID
```

### ToolCall

When the model decides to call a tool, it returns ToolCalls in the assistant message:

```go
type ToolCall struct {
    Index    *int              // used in stream mode to identify chunks for merging
    ID       string            // identifies the specific tool call
    Type     string            // default "function"
    Function FunctionCall
    Extra    map[string]any    // extra information for the tool call
}

type FunctionCall struct {
    Name      string // function name
    Arguments string // JSON-encoded arguments
}
```

## Document

Unit of data for RAG pipelines.

```go
type Document struct {
    ID       string
    Content  string
    MetaData map[string]any
}
```

Documents flow through: Loader (load) -> Transformer (split/enrich) -> Indexer (embed + store) -> Retriever (search).

## ToolInfo

Describes a tool's interface so the model knows how to call it.

```go
type ToolInfo struct {
    Name  string           // unique tool name
    Desc  string           // description for the model (how/when/why to use)
    Extra map[string]any   // extra information for the tool
    *ParamsOneOf           // embedded; nil means no input parameters
}
```

Parameters can be described in two ways:

```go
// Option A: use params map
schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
    "city": {Type: "string", Desc: "City name", Required: true},
})

// Option B: use raw JSON Schema
schema.NewParamsOneOfByJSONSchema(jsonSchemaBytes)
```

## StreamReader[T]

Generic streaming reader used throughout Eino. Returned by `Stream()`, `Transform()`, and streaming tool calls.

```go
type StreamReader[T any] struct { /* ... */ }
```

### Usage Pattern

```go
stream, err := model.Stream(ctx, messages)
if err != nil {
    return err
}
defer stream.Close() // ALWAYS close to release resources

for {
    chunk, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        return err
    }
    fmt.Print(chunk.Content)
}
```

### Creating Streams

```go
// Create a linked reader/writer pair
reader, writer := schema.Pipe[*schema.Message](bufferSize)

// Write from a goroutine
go func() {
    writer.Send(msg, nil)
    writer.Close()
}()

// Create from a slice (useful in tests)
reader := schema.StreamReaderFromArray(items)
```

### Key Rules

- **Always `defer stream.Close()`** -- failing to close causes resource leaks
- **Single consumer** -- a StreamReader can only be read by one goroutine
- **EOF signals completion** -- `errors.Is(err, io.EOF)` means the stream ended normally
