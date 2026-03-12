---
name: eino-guide
description: Eino framework overview, concepts, quick start, FAQ, and navigation. Use when a user asks general questions about Eino, needs help getting started, wants to understand the architecture, or is unsure which Eino skill to use. Eino is a Go framework for building LLM applications with components, orchestration graphs, and an agent development kit.
---

## What is Eino

Eino (pronounced "i know") is a Go framework for building LLM applications. It provides three layers:

1. **Components** -- Interfaces for ChatModel, Tool, Retriever, Indexer, Embedding, Prompt, Lambda, etc. Swap implementations without changing business code.
2. **Orchestration** -- Graph, Chain, and Workflow APIs that handle type checking, stream processing, concurrency, callbacks, and option assignment.
3. **ADK (Agent Development Kit)** -- High-level agent abstractions: ChatModelAgent, Runner, multi-agent patterns (Supervisor, Sequential, DeepAgent), interrupt/resume, and middleware.

## Repository Structure

| Repository | Description |
|---|---|
| `github.com/cloudwego/eino` | Core: interfaces, schema types, compose engine, ADK |
| `github.com/cloudwego/eino-ext` | Implementations: OpenAI, Ark, Ollama, Gemini, Milvus, etc. |
| `github.com/cloudwego/eino-examples` | Example applications and quickstart guides |

Key packages in `eino`:
- `schema` -- Message, Document, ToolInfo, StreamReader
- `components/model` -- ChatModel interface (Generate/Stream)
- `components/tool` -- Tool interfaces (BaseTool, InvokableTool, StreamableTool)
- `compose` -- Graph, Chain, Workflow orchestration
- `adk` -- Agent, Runner, ChatModelAgent, middleware, interrupt/resume
- `adk/prebuilt/deep` -- DeepAgent preset
- `callbacks` -- Callback handler framework

Key packages in `eino-ext`:
- `components/model/openai` -- OpenAI ChatModel
- `components/model/ark` -- Bytedance Ark ChatModel
- `components/model/ollama` -- Ollama ChatModel
- `components/model/gemini` -- Google Gemini ChatModel
- `adk/backend/local` -- Local filesystem Backend

## Quick Start

**Prerequisites:** Go 1.21+

```bash
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext/components/model/openai@latest
```

**Minimal ChatModel example:**

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino-ext/components/model/openai"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()
    model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
        Model:  "gpt-4o",
        APIKey: "your-api-key",
    })
    if err != nil {
        log.Fatal(err)
    }

    resp, err := model.Generate(ctx, []*schema.Message{
        schema.SystemMessage("You are a helpful assistant."),
        schema.UserMessage("What is Eino?"),
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.Content)
}
```

## Skill Routing Table

Use this decision tree to route questions to the right skill:

| User Intent | Skill |
|---|---|
| Component interfaces (ChatModel, Tool, Retriever, Embedding, etc.) | `/eino-component` |
| Graph, Chain, Workflow orchestration, streaming, callbacks | `/eino-compose` |
| Agent building, Runner, multi-agent, interrupt/resume, middleware | `/eino-agent` |
| General Eino questions, getting started, architecture overview | `/eino-guide` (this skill) |

## Common Patterns

**1. ChatModel + Tools (ReAct agent via ADK):**

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Model: chatModel,
    ToolsConfig: adk.ToolsConfig{
        ToolsNodeConfig: compose.ToolsNodeConfig{
            Tools: []tool.BaseTool{myTool},
        },
    },
})
runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
events := runner.Query(ctx, "What is the weather?")
```

**2. RAG pipeline (Graph):**

```go
graph := compose.NewGraph[string, *schema.Message]()
graph.AddRetrieverNode("retriever", myRetriever)
graph.AddChatTemplateNode("prompt", ragTemplate)
graph.AddChatModelNode("model", chatModel)
// wire: START -> retriever -> prompt -> model -> END
```

**3. Agent with middleware:**

```go
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Model:    chatModel,
    Handlers: []adk.ChatModelAgentMiddleware{summaryMW, reductionMW},
})
```

## Key Conventions

- **Options pattern**: Component methods accept variadic `...Option` for configuration (temperature, callbacks, etc.).
- **Context propagation**: Always pass `context.Context` through the call chain.
- **StreamReader must Close**: When using `Stream()`, always `defer stream.Close()` to avoid resource leaks.
- **Use ADK for agents**: Prefer `adk.NewChatModelAgent` over manually building ReAct loops with compose graphs.
- **ChatModelAgentMiddleware**: In v0.8+, use the new middleware interface instead of ChatModel/Tool Decorators.

## Reference Files

- `reference/concepts-and-architecture.md` -- Core types, component interfaces, Runnable abstraction, ADK layer
- `reference/quick-start.md` -- Three complete working examples (ChatModel, Agent+Runner, Graph)
- `reference/faq-and-troubleshooting.md` -- Common errors and solutions
- `reference/migration.md` -- Version migration notes (v0.4 through v0.8)

## Instructions to Agent

When answering Eino questions:

1. Always provide Go code examples using real import paths from `github.com/cloudwego/eino` and `github.com/cloudwego/eino-ext`.
2. Check user intent and route to the appropriate skill (`/eino-component`, `/eino-compose`, `/eino-agent`) when the question is specific.
3. For component implementation questions, check the `eino-ext` repository for available implementations.
4. Prefer ADK (ChatModelAgent + Runner) for agent use cases over raw graph composition.
5. When showing streaming code, always include `defer stream.Close()`.
6. For v0.8+ code, use `ChatModelAgentMiddleware` (Handlers field) instead of the older `AgentMiddleware` (Middlewares field).
