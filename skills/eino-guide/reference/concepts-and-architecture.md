Core types, component interfaces, orchestration model, and ADK architecture for the Eino framework.

## Design Philosophy

Eino provides a Go-idiomatic LLM application framework inspired by LangChain and LlamaIndex. Core goals:

- **Simplicity** -- Clean interfaces, minimal boilerplate
- **Composability** -- Components are interchangeable building blocks
- **Type safety** -- Compile-time type checking between connected nodes
- **Stream-native** -- First-class streaming support throughout the stack
- **Extensibility** -- Callbacks, middleware, and options at every layer

## Package Map

| Package | Purpose |
|---|---|
| `schema` | Core types: Message, Document, ToolInfo, StreamReader |
| `components/model` | ChatModel interface (Generate, Stream) |
| `components/tool` | Tool interfaces (BaseTool, InvokableTool, StreamableTool) |
| `components/embedding` | Embedding interface |
| `components/retriever` | Retriever interface |
| `components/indexer` | Indexer interface |
| `components/document` | Document loader/transformer interfaces |
| `components/prompt` | ChatTemplate interface |
| `compose` | Graph, Chain, Workflow orchestration engine |
| `callbacks` | Callback handler framework (OnStart, OnEnd, OnError) |
| `adk` | Agent Development Kit: Agent, Runner, ChatModelAgent |
| `adk/prebuilt/deep` | DeepAgent preset |
| `adk/prebuilt/supervisor` | Supervisor multi-agent pattern |

## Core Types

### schema.Message

The fundamental unit of conversation data:

```go
type Message struct {
    Role                  RoleType              // system, user, assistant, tool
    Content               string                // text content
    UserInputMultiContent []ChatMessagePart     // multimodal input (images, etc.)
    ToolCalls             []ToolCall            // tool calls (assistant only)
    ToolCallID            string                // tool response identifier
    ReasoningContent      string                // reasoning/thinking content
    // ...
}
```

Constructor helpers:

```go
schema.SystemMessage("You are a helpful assistant.")
schema.UserMessage("Hello")
schema.AssistantMessage("Hi there!", nil)       // content, toolCalls
schema.ToolMessage("result text", "call_id_1")
```

### schema.Document

Unit of document data for RAG pipelines:

```go
type Document struct {
    ID       string
    Content  string
    MetaData map[string]any
    // embedding vectors stored separately
}
```

### schema.ToolInfo

Describes a tool's interface for the model:

```go
type ToolInfo struct {
    Name string
    Desc string
    ParamsOneOf   *jsonschema.Schema  // JSON Schema for parameters
}
```

### schema.StreamReader[T]

Generic streaming reader used throughout Eino:

```go
type StreamReader[T any] struct { /* ... */ }

// Usage pattern:
stream, err := model.Stream(ctx, messages)
if err != nil { return err }
defer stream.Close()  // ALWAYS close

for {
    chunk, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil { return err }
    fmt.Print(chunk.Content)
}
```

## Component Interfaces

All components define standard interfaces. Implementations live in `eino-ext`.

| Component | Interface | Key Methods |
|---|---|---|
| ChatModel | `model.BaseChatModel` | `Generate(ctx, []*Message, ...Option) (*Message, error)` |
| | | `Stream(ctx, []*Message, ...Option) (*StreamReader[*Message], error)` |
| Tool | `tool.BaseTool` | `Info(ctx) (*ToolInfo, error)` |
| | `tool.InvokableTool` | `InvokableRun(ctx, argsJSON, ...Option) (string, error)` |
| | `tool.StreamableTool` | `StreamableRun(ctx, argsJSON, ...Option) (*StreamReader[string], error)` |
| Retriever | `retriever.Retriever` | `Retrieve(ctx, query, ...Option) ([]*Document, error)` |
| Indexer | `indexer.Indexer` | `Store(ctx, docs, ...Option) ([]string, error)` |
| Embedding | `embedding.Embedder` | `EmbedStrings(ctx, texts, ...Option) ([][]float64, error)` |
| ChatTemplate | `prompt.ChatTemplate` | `Format(ctx, params, ...Option) ([]*Message, error)` |
| Document Loader | `document.Loader` | `Load(ctx, ...Option) ([]*Document, error)` |

## Runnable Abstraction

Compiled graphs expose four execution modes:

```go
type Runnable[I, O any] interface {
    Invoke(ctx, I, ...Option) (O, error)                           // non-stream in, non-stream out
    Stream(ctx, I, ...Option) (*StreamReader[O], error)            // non-stream in, stream out
    Collect(ctx, *StreamReader[I], ...Option) (O, error)           // stream in, non-stream out
    Transform(ctx, *StreamReader[I], ...Option) (*StreamReader[O], error) // stream in, stream out
}
```

The framework automatically handles stream concatenation, splitting, copying, and merging between nodes.

## Orchestration: Graph, Chain, Workflow

### Chain

Simple linear pipeline:

```go
chain, _ := compose.NewChain[map[string]any, *schema.Message]().
    AppendChatTemplate(prompt).
    AppendChatModel(model).
    Compile(ctx)
result, _ := chain.Invoke(ctx, map[string]any{"query": "hello"})
```

### Graph

Directed graph with branching and cycles:

```go
graph := compose.NewGraph[map[string]any, *schema.Message]()
graph.AddChatTemplateNode("tpl", chatTpl)
graph.AddChatModelNode("model", chatModel)
graph.AddToolsNode("tools", toolsNode)
graph.AddEdge(compose.START, "tpl")
graph.AddEdge("tpl", "model")
graph.AddBranch("model", branch)
graph.AddEdge("tools", compose.END)
compiled, _ := graph.Compile(ctx)
```

Built-in features: type checking, stream handling, concurrency, callback injection, option assignment, state management, branching.

### Workflow

DAG with field-level data mapping:

```go
wf := compose.NewWorkflow[Input, *schema.Message]()
wf.AddChatModelNode("model", m).AddInput(compose.START)
wf.AddLambdaNode("lambda1", compose.InvokableLambda(fn1)).
    AddInput("model", compose.MapFields("Content", "Input"))
wf.End().AddInput("lambda1")
runnable, _ := wf.Compile(ctx)
```

## ADK Layer

### Agent Interface

```go
type Agent interface {
    Name(ctx context.Context) string
    Description(ctx context.Context) string
    Run(ctx context.Context, input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent]
}
```

### ChatModelAgent

ReAct-style agent with automatic tool calling loop:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "assistant",
    Description: "A helpful assistant",
    Model:       chatModel,
    Instruction: "You are a helpful assistant.",
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{
            Tools: []tool.BaseTool{weatherTool},
        },
    },
})
```

### Runner

Manages agent lifecycle, checkpointing, and event streams:

```go
runner := adk.NewRunner(ctx, adk.RunnerConfig{
    Agent:           agent,
    EnableStreaming:  true,
})
events := runner.Query(ctx, "What is the weather?")
for {
    event, ok := events.Next()
    if !ok { break }
    // process event.Output, event.Action, event.Err
}
```

### AgentEvent

```go
type AgentEvent struct {
    AgentName string
    Output    *AgentOutput  // MessageOutput (text or stream)
    Action    *AgentAction  // interrupt, transfer, exit
    Err       error
}
```

### Multi-Agent Patterns

```go
// Sub-agents: transfer control to specialized agents
mainAgent, _ = adk.SetSubAgents(ctx, mainAgent, []adk.Agent{researchAgent, codeAgent})

// Agent as Tool: wrap agent as a callable tool
researchTool := adk.NewAgentTool(ctx, researchAgent)

// Supervisor: one agent coordinates multiple experts
supervisorAgent, _ := supervisor.New(ctx, &supervisor.Config{
    Supervisor: coordinatorAgent,
    SubAgents:  []adk.Agent{writerAgent, reviewerAgent},
})

// Sequential: agents run in order
seqAgent, _ := adk.NewSequentialAgent(ctx, &adk.SequentialAgentConfig{
    SubAgents: []adk.Agent{plannerAgent, executorAgent},
})
```

### Interrupt and Resume

```go
// Inside a tool or agent, trigger interrupt
return adk.Interrupt(ctx, "Please confirm this action")

// Later, resume from checkpoint
events, _ := runner.Resume(ctx, checkpointID)
```

### Callbacks

Cross-cutting concerns (logging, tracing, metrics):

```go
handler := callbacks.NewHandlerBuilder().
    OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        log.Printf("onStart: %v", info)
        return ctx
    }).
    Build()

compiled.Invoke(ctx, input, compose.WithCallbacks(handler))
```

Five callback types: OnStart, OnEnd, OnError, OnStartWithStreamInput, OnEndWithStreamOutput.
