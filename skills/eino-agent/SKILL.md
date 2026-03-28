---
name: eino-agent
description: Eino ADK agent construction, middleware, and runner. Use when a user needs to build an AI Agent, configure ChatModelAgent with ReAct pattern, use middleware (filesystem, tool search, tool reduction, summarization, plan-task, skill), set up the Runner for event-driven execution, implement human-in-the-loop with interrupt/resume, or wrap agents as tools. Covers ChatModelAgent and DeepAgents.
---

# Eino ADK Overview

Import: `github.com/cloudwego/eino/adk`

The Agent Development Kit (ADK) provides a framework for building agents in Go. Core interface:

```go
type Agent interface {
    Name(ctx context.Context) string
    Description(ctx context.Context) string
    Run(ctx context.Context, input *AgentInput, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]
}
```

# Agent Types

| Type | Description | Decision |
|------|-------------|----------|
| ChatModelAgent | ReAct pattern: LLM reasons, calls tools, loops until done | Dynamic (LLM) |
| DeepAgent | Pre-built agent with planning, filesystem, sub-agents | Dynamic (LLM) |
| Custom Agent | Implement the Agent interface directly | Custom |

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

    // 2. Create model
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
        msg, _ := event.Output.MessageOutput.GetMessage()
        fmt.Printf("Agent[%s]: %v\n", event.AgentName, msg)
    }
}
```

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
iter := runner.Run(ctx, []adk.Message{schema.UserMessage("hello")})
```

# Middleware System

Middleware extends ChatModelAgent behavior. Configure via `Handlers` field:

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    // ...
    Handlers: []adk.ChatModelAgentMiddleware{fsMiddleware, summarizationMW},
})
```

Seven built-in middleware types (see reference/middleware.md for details):

| Middleware | Package | Purpose |
|-----------|---------|---------|
| FileSystem | `adk/middlewares/filesystem` | File ops (read/write/edit/glob/grep) + shell |
| ToolSearch | `adk/middlewares/dynamictool/toolsearch` | Dynamic tool selection via regex search |
| ToolReduction | `adk/middlewares/reduction` | Truncate/clear large tool results |
| Summarization | `adk/middlewares/summarization` | Compress long conversation history |
| PlanTask | `adk/middlewares/plantask` | Task creation and progress tracking |
| Skill | `adk/middlewares/skill` | Skill-based progressive disclosure |
| PatchToolCalls | `adk/middlewares/patchtoolcalls` | Fix dangling tool calls in history |

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

Key pattern: tool returns `compose.NewInterruptAndRerunErr(info)` to pause, then `runner.ResumeWithParams(ctx, checkpointID, params)` to continue.

# Instructions to Agent

- Default to `ChatModelAgent` for most use cases (single agent with tools)
- Use `Runner` to execute agents -- never call `agent.Run()` directly in production
- Middleware order matters: PatchToolCalls first, then Summarization, then Reduction
- Use `DeepAgent` (`adk/prebuilt/deep`) when you need built-in planning + filesystem + sub-agents
- Use `AgentAsTool` or DeepAgents' SubAgents when a sub-agent needs isolated context (no shared history)

## Reference Files

Read these files on-demand for detailed API, examples, and advanced usage:

- [reference/chat-model-agent.md](reference/chat-model-agent.md) -- ChatModelAgentConfig reference, ReAct pattern, ToolsConfig, streaming example
- [reference/deep-agents.md](reference/deep-agents.md) -- DeepAgent concept, config, architecture, comparison with ChatModelAgent
- [reference/middleware.md](reference/middleware.md) -- All 7 middleware types with interface, config, and examples
- [reference/runner-and-events.md](reference/runner-and-events.md) -- Runner creation, AgentEvent/AgentOutput, event iteration patterns
- [reference/agent-as-tool.md](reference/agent-as-tool.md) -- Wrapping an Agent as a Tool for use by another agent
- [reference/human-in-the-loop.md](reference/human-in-the-loop.md) -- Interrupt APIs, ResumableAgent, CheckPointStore, resume patterns
- [reference/filesystem.md](reference/filesystem.md) -- Filesystem Backend interface, Local and AgentKit implementations, usage with DeepAgent
