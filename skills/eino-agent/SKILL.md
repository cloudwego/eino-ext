---
name: eino-agent
description: Eino ADK agent construction, middleware, and runner. Use when a user needs to build an AI Agent, configure ChatModelAgent with ReAct pattern, use middleware (filesystem, tool search, tool reduction, summarization, plan-task, skill, agents.md), set up the Runner for event-driven execution, implement human-in-the-loop with interrupt/resume, use Cancel/Retry/Failover for model resilience, build push-based multi-turn loops with TurnLoop, or wrap agents as tools. Covers ChatModelAgent, DeepAgents, and TurnLoop.
---

# Eino ADK Overview

Import: `github.com/cloudwego/eino/adk`

The Agent Development Kit (ADK) provides a framework for building agents in Go. The ADK is generically parameterized by `MessageType` to support both classic `*schema.Message` and the new `*schema.AgenticMessage`. Prefer `*schema.AgenticMessage` for new usage.

```go
type MessageType interface {
    *schema.Message | *schema.AgenticMessage
}

type TypedAgent[M MessageType] interface {
    Name(ctx context.Context) string
    Description(ctx context.Context) string
    Run(ctx context.Context, input *TypedAgentInput[M], options ...AgentRunOption) *AsyncIterator[*TypedAgentEvent[M]]
}

// Convenience aliases for classic message type
type Agent = TypedAgent[*schema.Message]
```

# Agent Types

| Type | Description | Decision |
|------|-------------|----------|
| ChatModelAgent | ReAct pattern: LLM reasons, calls tools, loops until done | Dynamic (LLM) |
| DeepAgent | Pre-built agent with planning, filesystem, sub-agents | Dynamic (LLM) |
| TurnLoop | Push-based event loop for multi-turn execution with preemption and lifecycle management | Runtime |
| Custom Agent | Implement the TypedAgent interface directly | Custom |

# ChatModelAgent Quick Start

```go
import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/components/tool/utils"
    "github.com/cloudwego/eino/compose"
)

func main() {
    ctx := context.Background()

    // 1. Create a tool
    searchTool, _ := utils.InferTool("search_book", "Search books by genre",
        func(ctx context.Context, input *struct {
            Genre string `json:"genre" jsonschema_description:"Book genre"`
        }) (string, error) {
            return `{"books": ["The Great Gatsby"]}`, nil
        })

    // 2. Create model (BaseModel[M], not ToolCallingChatModel)
    cm, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        APIKey: "your-key", Model: "gpt-4o",
    })

    // 3. Create agent
    agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:        "BookRecommender",
        Description: "Recommends books",
        Instruction: "You recommend books using the search_book tool.",
        Model:       cm,
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig: compose.ToolsNodeConfig{
                Tools: []tool.BaseTool{searchTool},
            },
        },
    })

    // 4. Run with Runner
    runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
    iter := runner.Query(ctx, "recommend a fiction book")
    for {
        event, ok := iter.Next()
        if !ok {
            break
        }
        if event.Err != nil {
            log.Fatal(event.Err)
        }
        if event.Output != nil && event.Output.MessageOutput != nil {
            msg, _ := event.Output.MessageOutput.GetMessage()
            fmt.Printf("Agent[%s]: %v\n", event.AgentName, msg)
        }
    }
}
```

# Cancel Mechanism

Cancel provides safe, controllable termination of agent execution.

```go
// Create a cancel function alongside the run
cancelOpt, cancelFn := adk.WithCancel()
iter := runner.Query(ctx, "do something", cancelOpt)

// ... iterate events ...

// Cancel at a safe point (cancelFn is non-blocking, Wait blocks until complete)
handle, ok := cancelFn(adk.WithAgentCancelMode(adk.CancelAfterChatModel))
if ok {
    handle.Wait()
}
```

**CancelMode** (bitmask):

| Mode | Behavior |
|------|----------|
| `CancelImmediate` (0) | Abort immediately, stream terminated |
| `CancelAfterChatModel` | Wait for current model call to finish |
| `CancelAfterToolCalls` | Wait for current tool calls to finish |

**Cancel options:**
- `WithAgentCancelMode(mode)` -- set safe point
- `WithAgentCancelTimeout(d)` -- escalate to immediate if safe point not reached in time
- `WithRecursive()` -- propagate cancel into nested AgentTool agents

Cancel produces a `CancelError` on the event stream with checkpoint data for later resumption.

# Model Retry

Output-based retry with full control over retry decisions.

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    ModelRetryConfig: &adk.ModelRetryConfig{
        MaxRetries: 3,
        ShouldRetry: func(ctx context.Context, retryCtx *adk.RetryContext) *adk.RetryDecision {
            // Retry based on output content (e.g., empty response, bad finish reason)
            if retryCtx.Err != nil {
                return &adk.RetryDecision{Retry: true, Backoff: time.Second}
            }
            if retryCtx.OutputMessage == nil || retryCtx.OutputMessage.Content == "" {
                return &adk.RetryDecision{Retry: true, Backoff: time.Second}
            }
            return &adk.RetryDecision{Retry: false}
        },
    },
})
```

**RetryContext** provides: `RetryAttempt`, `InputMessages`, `OutputMessage` (full concatenated response), `Err`.

**RetryDecision** controls: `Retry`, `ModifiedInputMessages`, `AdditionalOptions`, `Backoff`, `RejectReason`.

When streaming, a `WillRetryError` is emitted on the stream to signal retry is occurring.

# Model Failover

Dynamic model switching when primary model fails or produces unsatisfactory output.

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    ModelFailoverConfig: &adk.ModelFailoverConfig[*schema.Message]{
        MaxRetries: 2,
        ShouldFailover: func(ctx context.Context, outputMessage *schema.Message, outputErr error) bool {
            return outputErr != nil // failover on any error
        },
        GetFailoverModel: func(ctx context.Context, failoverCtx *adk.FailoverContext[*schema.Message]) (
            model.BaseModel[*schema.Message], []*schema.Message, error) {
            // Return a different model and optionally modified input
            return backupModel, failoverCtx.InputMessages, nil
        },
    },
})
```

Failover interacts with Retry: when both are configured, `FailoverContext.LastErr` will be a `*RetryExhaustedError` if retry was also exhausted.

# TurnLoop

Push-based event loop for multi-turn agent execution with preemption, idle timeout, and graceful shutdown.

```go
import "github.com/cloudwego/eino/adk"

loop := adk.NewTurnLoop(adk.TurnLoopConfig[string, *schema.Message]{
    GenInput: func(ctx context.Context, loop *adk.TurnLoop[string, *schema.Message], items []string) (*adk.GenInputResult[string, *schema.Message], error) {
        // Convert pushed items into agent input
        combined := strings.Join(items, "\n")
        return &adk.GenInputResult[string, *schema.Message]{
            RunCtx:   ctx,
            Input:    &adk.TypedAgentInput[*schema.Message]{Messages: []*schema.Message{schema.UserMessage(combined)}},
            Consumed: items,
        }, nil
    },
    PrepareAgent: func(ctx context.Context, loop *adk.TurnLoop[string, *schema.Message], consumed []string) (adk.Agent, error) {
        return myAgent, nil
    },
    OnAgentEvents: func(ctx context.Context, tc *adk.TurnContext[string, *schema.Message], events *adk.AsyncIterator[*adk.AgentEvent]) error {
        for {
            event, ok := events.Next()
            if !ok {
                break
            }
            if event.Err != nil {
                return event.Err
            }
            // Process events (e.g., send to client)
        }
        return nil
    },
})

// Start the loop
loop.Run(ctx)

// Push items (non-blocking, returns (ok bool, resolved <-chan struct{}))
loop.Push("user message 1")

// Preempt current turn with new input
loop.Push("urgent message", adk.WithPreempt[string, *schema.Message](adk.AfterChatModel))

// Stop gracefully
loop.Stop(adk.WithGraceful())

// Wait for exit and get final state
exitState := loop.Wait()
```

**Key concepts:**
- `Push()` queues items; the loop batches and processes them via `GenInput`
- Preemption: cancel current turn at a safe point and start new turn with pending items
- Idle timeout: `UntilIdleFor(d)` auto-stops after no items for duration `d`
- Graceful shutdown: `WithGraceful()`, `WithGracefulTimeout(d)`, `WithImmediate()`
- Exit state: `TurnLoopExitState` contains `ExitReason`, `UnhandledItems`, `InterruptedItems`

# Runner

The Runner manages agent lifecycle, context passing, and interrupt/resume:

```go
runner := adk.NewRunner(ctx, adk.RunnerConfig{
    Agent:           myAgent,
    EnableStreaming:  true,
    CheckPointStore: myStore,  // for interrupt/resume
})

// Query (convenience for single user message)
iter := runner.Query(ctx, "hello")

// Run (full control over input messages)
iter := runner.Run(ctx, []*schema.Message{schema.UserMessage("hello")})
```

# Middleware System

Middleware extends ChatModelAgent behavior. Configure via `Handlers` field:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    Handlers: []adk.ChatModelAgentMiddleware{fsMiddleware, summarizationMW},
})
```

Eight built-in middleware types (see reference/middleware.md for details):

| Middleware | Package | Purpose |
|-----------|---------|---------|
| FileSystem | `adk/middlewares/filesystem` | File ops (read/write/edit/glob/grep) + shell |
| ToolSearch | `adk/middlewares/dynamictool/toolsearch` | Dynamic tool selection via regex search |
| ToolReduction | `adk/middlewares/reduction` | Truncate/clear large tool results |
| Summarization | `adk/middlewares/summarization` | Compress long conversation history |
| PlanTask | `adk/middlewares/plantask` | Task creation and progress tracking |
| Skill | `adk/middlewares/skill` | Skill-based progressive disclosure |
| PatchToolCalls | `adk/middlewares/patchtoolcalls` | Fix dangling tool calls in history |
| Agents.md | `adk/middlewares/agentsmd` | Inject Agents.md instructions into model input |

# AgentAsTool

Wrap any Agent as a Tool for use by another agent:

```go
subAgent := createMySubAgent()
agentTool := adk.NewAgentTool(ctx, subAgent)

parentAgent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{
            Tools: []tool.BaseTool{agentTool},
        },
    },
})
```

# Human-in-the-Loop

ChatModelAgent supports interrupt and resume for human approval, clarification, and feedback. See reference/human-in-the-loop.md for details.

Key pattern: tool returns `compose.Interrupt(ctx, info)` to pause, then `runner.ResumeWithParams(ctx, checkpointID, params)` to continue.

# Instructions to Agent

- Default to `ChatModelAgent` for most use cases (single agent with tools)
- Use `Runner` to execute agents -- never call `agent.Run()` directly in production
- Middleware order matters: PatchToolCalls first, then Reduction, then Summarization
- Use `DeepAgent` (`adk/prebuilt/deep`) when you need built-in planning + filesystem + sub-agents
- Use `AgentAsTool` or DeepAgents' SubAgents when a sub-agent needs isolated context (no shared history)
- Use `TurnLoop` when building interactive applications that need preemption, idle management, or push-based input
- `Model` field accepts `model.BaseModel[M]` -- for classic path use any `BaseChatModel`, for agentic path use `AgenticModel`
- Cancel, Retry, and Failover can all be combined; failover wraps retry
- `WithCancel()` is a run option, not a config option -- create fresh per-run

## Reference Files

Read these files on-demand for detailed API, examples, and advanced usage:

- [reference/chat-model-agent.md](reference/chat-model-agent.md) -- ChatModelAgentConfig reference, ReAct pattern, ToolsConfig, streaming, Cancel/Retry/Failover details
- [reference/deep-agents.md](reference/deep-agents.md) -- DeepAgent concept, config, architecture, comparison with ChatModelAgent
- [reference/middleware.md](reference/middleware.md) -- All 8 middleware types with interface, config, and examples
- [reference/runner-and-events.md](reference/runner-and-events.md) -- Runner creation, AgentEvent/AgentOutput, event iteration patterns
- [reference/agent-as-tool.md](reference/agent-as-tool.md) -- Wrapping an Agent as a Tool for use by another agent
- [reference/human-in-the-loop.md](reference/human-in-the-loop.md) -- Interrupt APIs, ResumableAgent, CheckPointStore, resume patterns
- [reference/filesystem.md](reference/filesystem.md) -- Filesystem Backend interface, Local and AgentKit implementations, usage with DeepAgent
