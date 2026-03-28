Human-in-the-loop (HITL) enables agents to pause execution, request human input, and resume from where they stopped.

## Core Concepts

1. **Interrupt**: Agent pauses and sends info to the user (e.g., "approve this action?")
2. **Checkpoint**: Framework saves execution state to a CheckPointStore
3. **Resume**: User provides input, framework restores state and continues

## Quick Start: Approval Pattern

### 1. Create a tool that can interrupt

```go
import (
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/components/tool/utils"
    "github.com/cloudwego/eino/compose"
)

type bookInput struct {
    Location      string `json:"location"`
    PassengerName string `json:"passenger_name"`
}

bookTool, _ := utils.InferTool("BookTicket", "Book a ticket",
    func(ctx context.Context, input *bookInput) (string, error) {
        return "success", nil
    })

// Wrap with approval -- interrupts before execution
approvalTool := &tool.InvokableApprovableTool{InvokableTool: bookTool}
```

### 2. Create agent and runner with CheckPointStore

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Name:        "TicketBooker",
    Instruction: "Book tickets using the BookTicket tool.",
    Model:       cm,
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{
            Tools: []tool.BaseTool{approvalTool},
        },
    },
})

runner := adk.NewRunner(ctx, adk.RunnerConfig{
    Agent:           agent,
    EnableStreaming:  true,
    CheckPointStore: store.NewInMemoryStore(), // Use Redis in production
})
```

### 3. Run and handle interrupt

```go
iter := runner.Query(ctx, "Book a ticket to Beijing for Martin", adk.WithCheckPointID("session-1"))

var interruptID string
for {
    event, ok := iter.Next()
    if !ok {
        break
    }
    if event.Err != nil {
        log.Fatal(event.Err)
    }
    if event.Action != nil && event.Action.Interrupted != nil {
        // Save the interrupt ID for resuming later
        interruptID = event.Action.Interrupted.InterruptContexts[0].ID
        fmt.Printf("Interrupt: %v\n", event.Action.Interrupted.InterruptContexts[0].Info)
        break
    }
    // Handle normal output...
}
```

### 4. Resume with user decision

```go
// User approves
apResult := &tool.ApprovalResult{Approved: true}

iter, err := runner.ResumeWithParams(ctx, "session-1", &adk.ResumeParams{
    Targets: map[string]any{
        interruptID: apResult,
    },
})
if err != nil {
    log.Fatal(err)
}

// Continue processing events
for {
    event, ok := iter.Next()
    if !ok {
        break
    }
    // Handle events...
}
```

## Interrupt APIs (ADK Layer)

### Simple Interrupt

```go
// In a custom agent's Run method:
return adk.Interrupt(ctx, "Please clarify your request.")
```

### Stateful Interrupt

```go
// Save internal state for later restoration
state := &MyState{ProcessedItems: 42}
return adk.StatefulInterrupt(ctx, "Need user feedback", state)
```

### Composite Interrupt (for multi-agent)

```go
// When a parent agent's sub-agent interrupts
return adk.CompositeInterrupt(ctx, "Sub-agent needs attention", parentState, subInterruptSignals...)
```

## Interrupt APIs (Compose Layer -- for tools/graph nodes)

```go
import "github.com/cloudwego/eino/compose"

// Simple interrupt from a tool
return "", compose.Interrupt(ctx, "Waiting for approval")

// Stateful interrupt
return "", compose.StatefulInterrupt(ctx, "Need input", myState)
```

## InterruptInfo Structure

When an interrupt occurs, the event contains structured info:

```go
if event.Action != nil && event.Action.Interrupted != nil {
    for _, point := range event.Action.Interrupted.InterruptContexts {
        fmt.Printf("ID: %s\n", point.ID)           // Unique interrupt address
        fmt.Printf("Info: %v\n", point.Info)         // User-facing info
        fmt.Printf("Root cause: %v\n", point.IsRootCause)
    }
}
```

## Resume APIs

### ResumeWithParams (recommended)

```go
iter, err := runner.ResumeWithParams(ctx, checkpointID, &adk.ResumeParams{
    Targets: map[string]any{
        interruptID1: userData1,
        interruptID2: userData2,
    },
})
```

### Legacy Resume (with tool options)

```go
iter, err := runner.Resume(ctx, checkpointID,
    adk.WithToolOptions([]tool.Option{WithNewInput("user response")}),
)
```

## ResumableAgent Interface

Agents that support interrupt/resume must implement:

```go
type ResumableAgent interface {
    Agent
    Resume(ctx context.Context, info *ResumeInfo, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]
}

type ResumeInfo struct {
    WasInterrupted bool   // Always true when Resume is called
    InterruptState any    // State saved via StatefulInterrupt
    IsResumeTarget bool   // Whether this agent is the explicit resume target
    ResumeData     any    // User-provided data for this agent
}
```

ChatModelAgent implements ResumableAgent by default.

## CheckPointStore

```go
type CheckPointStore interface {
    Set(ctx context.Context, key string, value []byte) error
    Get(ctx context.Context, key string) ([]byte, bool, error)
}
```

Built-in: `store.NewInMemoryStore()` (for development). Use Redis or similar for production.

State serialization uses `encoding/gob`. Register custom types:

```go
func init() {
    gob.RegisterName("mypackage.MyType", &MyType{})
}
```

## Common HITL Patterns

| Pattern | Description | Example |
|---------|-------------|---------|
| Approval | Pause before executing an action | Tool execution approval |
| Review & Edit | Let user modify tool arguments | Edit API call parameters |
| Feedback Loop | Iterative refinement with human feedback | Content generation review |
| Follow-up | Agent asks for clarification | Missing information prompts |

## Complete Clarification Example

```go
// Tool that asks user for clarification
type askInput struct {
    Question string `json:"question" jsonschema_description:"Question to ask the user"`
}

askTool, _ := utils.InferTool("ask_user", "Ask user for clarification",
    func(ctx context.Context, input *askInput) (string, error) {
        // Interrupt to ask user
        return "", compose.Interrupt(ctx, input.Question)
    })

// ... configure agent with askTool ...

// Handle interrupt
iter := runner.Query(ctx, "Recommend me some books", adk.WithCheckPointID("session-1"))
var interruptID string
for {
    event, ok := iter.Next()
    if !ok {
        break
    }
    if event.Action != nil && event.Action.Interrupted != nil {
        interruptID = event.Action.Interrupted.InterruptContexts[0].ID
        fmt.Printf("Agent asks: %v\n", event.Action.Interrupted.InterruptContexts[0].Info)
        break
    }
}

// Resume with user's answer
iter, _ = runner.ResumeWithParams(ctx, "session-1", &adk.ResumeParams{
    Targets: map[string]any{
        interruptID: "I want fiction books",
    },
})
```
