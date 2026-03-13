Runner is the core engine that executes agents, manages multi-agent coordination, context passing, and interrupt/resume.

## Creating a Runner

```go
import "github.com/cloudwego/eino/adk"

runner := adk.NewRunner(ctx, adk.RunnerConfig{
    Agent:           myAgent,
    EnableStreaming:  true,                // Suggest streaming output from components that support it
    CheckPointStore: myCheckPointStore,    // Required for interrupt/resume
})
```

## Running an Agent

```go
// Convenience: single user message
iter := runner.Query(ctx, "What is the weather in Tokyo?")

// Full control: multiple messages
iter := runner.Run(ctx, []adk.Message{
    schema.UserMessage("Hello"),
    schema.AssistantMessage("Hi! How can I help?", nil),
    schema.UserMessage("What's the weather?"),
}, adk.WithSessionValues(map[string]any{"user": "Alice"}))
```

## AgentInput

```go
type AgentInput struct {
    Messages       []Message  // Conversation messages (user, assistant, tool, system)
    EnableStreaming bool       // Suggest streaming mode for capable components
}

type Message = *schema.Message
```

`EnableStreaming` is a suggestion, not a constraint. Components that only support one mode will ignore it. `AgentOutput.IsStreaming` indicates the actual output mode.

## AgentEvent

Every event from `AsyncIterator` is an `AgentEvent`:

```go
type AgentEvent struct {
    AgentName string         // Which agent produced this event
    RunPath   []RunStep      // Full call chain from entry agent to current
    Output    *AgentOutput   // Message output (may be nil)
    Action    *AgentAction   // Control action (may be nil)
    Err       error          // Error (may be nil)
}
```

### AgentOutput

```go
type AgentOutput struct {
    MessageOutput    *MessageVariant  // Message content
    CustomizedOutput any              // Custom output data
}

type MessageVariant struct {
    IsStreaming   bool              // true = read from MessageStream, false = read from Message
    Message       Message           // Non-streaming: complete message
    MessageStream MessageStream     // Streaming: message chunk stream
    Role          schema.RoleType   // Assistant or Tool
    ToolName      string            // Set when Role is Tool
}
```

### Reading Messages

```go
// Non-streaming
if !mv.IsStreaming {
    fmt.Println(mv.Message.Content)
}

// Streaming
if mv.IsStreaming {
    for {
        chunk, err := mv.MessageStream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Fatal(err)
        }
        fmt.Print(chunk.Content)
    }
}

// Convenience method (works for both)
msg, err := mv.GetMessage()
```

### AgentAction

```go
type AgentAction struct {
    Exit            bool                    // Immediately exit the multi-agent system
    Interrupted     *InterruptInfo          // Pause execution, save checkpoint
    TransferToAgent *TransferToAgentAction  // Transfer control to another agent
    BreakLoop       *BreakLoopAction        // Break out of a LoopAgent
    CustomizedAction any                    // Custom action
}
```

## Event Iteration Pattern

```go
iter := runner.Query(ctx, "hello")
for {
    event, ok := iter.Next()
    if !ok {
        break  // No more events
    }

    if event.Err != nil {
        log.Fatal(event.Err)
        break
    }

    // Handle actions
    if event.Action != nil {
        if event.Action.Interrupted != nil {
            fmt.Printf("Interrupted: %v\n", event.Action.Interrupted)
            continue
        }
        if event.Action.TransferToAgent != nil {
            fmt.Printf("Transfer to: %s\n", event.Action.TransferToAgent.DestAgentName)
            continue
        }
    }

    // Handle output
    if event.Output != nil && event.Output.MessageOutput != nil {
        msg, err := event.Output.MessageOutput.GetMessage()
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("[%s][%s] %s\n", event.AgentName, event.Output.MessageOutput.Role, msg.Content)
    }
}
```

## AgentRunOption

Options that modify agent behavior per-run:

```go
// Inject session values accessible to all agents
adk.WithSessionValues(map[string]any{"key": "value"})

// Set checkpoint ID for interrupt/resume
adk.WithCheckPointID("session-123")

// Skip transfer messages in history
adk.WithSkipTransferMessages()

// Custom options (agent-specific)
adk.WrapImplSpecificOptFn(func(t *myOptions) { t.Field = "value" })

// Target option to specific agents
opt := adk.WithSessionValues(vals).DesignateAgent("agent_1", "agent_2")
```

## SessionValues

Cross-agent key-value storage within a single run:

```go
// Inside an agent or tool:
adk.AddSessionValue(ctx, "key", "value")
val, ok := adk.GetSessionValue(ctx, "key")
allVals := adk.GetSessionValues(ctx)

// Before running (must use option, not AddSessionValue):
runner.Run(ctx, msgs, adk.WithSessionValues(map[string]any{"key": "value"}))
```

## AsyncIterator

```go
type AsyncIterator[T any] struct { ... }

func (ai *AsyncIterator[T]) Next() (T, bool)
```

- `Next()` blocks until an event is available or the iterator is closed
- Returns `(event, true)` when an event is available
- Returns `(zero, false)` when the agent is done

## Custom Agent Implementation

```go
type MyAgent struct{}

func (a *MyAgent) Name(ctx context.Context) string        { return "MyAgent" }
func (a *MyAgent) Description(ctx context.Context) string  { return "My custom agent" }

func (a *MyAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
    iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
    go func() {
        defer gen.Close()
        // Process input, generate events
        gen.Send(adk.EventFromMessage(
            schema.AssistantMessage("Hello!", nil), nil, schema.Assistant, "",
        ))
    }()
    return iter
}
```

## Language Setting

```go
// Set language for all ADK built-in prompts (global)
adk.SetLanguage(adk.LanguageChinese)  // Chinese
adk.SetLanguage(adk.LanguageEnglish)  // English (default)
```
