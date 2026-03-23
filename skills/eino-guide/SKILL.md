---
name: eino-guide
description: Eino framework overview, concepts, and navigation. Use when a user asks general questions about Eino, needs help getting started, wants to understand the architecture, or is unsure which Eino skill to use. Eino is a Go framework for building LLM applications with components, orchestration graphs, and an agent development kit.
---

# Eino Framework Guide

Eino (pronounced "i know") is a Go framework for building LLM applications.

## Core Concepts

### Component

Standardized interfaces for AI capabilities. Each interface has multiple interchangeable implementations.

| Component | What It Does | Key Interface |
|-----------|-------------|---------------|
| ChatModel | LLM inference (generate / stream) | `model.BaseChatModel`, `model.ToolCallingChatModel` |
| Tool | Functions the model can call | `tool.InvokableTool`, `tool.EnhancedInvokableTool` |
| Embedding | Text to vector | `embedding.Embedder` |
| Retriever | Vector/keyword search | `retriever.Retriever` |
| Indexer | Store documents with vectors | `indexer.Indexer` |
| ChatTemplate | Prompt formatting with variables | `prompt.ChatTemplate` |
| Document Loader/Transformer | Load and process documents | `document.Loader`, `document.Transformer` |
| Callback Handler | Observability and tracing | `callbacks.Handler` |

Implementations live in `eino-ext` (OpenAI, Claude, Gemini, Ark, Ollama, Milvus, Redis, Elasticsearch, etc.).

-> Use `/eino-component` for selecting, configuring, and using components.

### Orchestration (Compose)

Three APIs for wiring components into executable pipelines. All compile to a `Runnable[I, O]` with four execution modes (Invoke, Stream, Collect, Transform).

| API | Topology | When to Use |
|-----|----------|-------------|
| **Graph** | Directed graph, supports cycles | Complex flows with branching, loops (e.g., ReAct pattern) |
| **Chain** | Linear sequential | Simple pipelines (e.g., template -> model) |
| **Workflow** | DAG with field-level mapping | Parallel branches with struct field routing |

The compose layer handles type checking, stream conversion between nodes, concurrency, callback injection, and option distribution automatically.

-> Use `/eino-compose` for building graphs, chains, workflows, streaming, callbacks, and state management.

### ADK (Agent Development Kit)

High-level abstractions for building AI agents. Encapsulates the model-tool-loop pattern.

| Concept | What It Does |
|---------|-------------|
| **ChatModelAgent** | ReAct-style agent: model generates, calls tools, loops until done |
| **DeepAgent** | Pre-built agent with filesystem backend, tool search, summarization |
| **Runner** | Executes agents, manages checkpoints, emits event streams |
| **Middleware (Handlers)** | Intercept and extend agent behavior (filesystem, summarization, plan-task, etc.) |
| **Interrupt/Resume** | Human-in-the-loop: pause agent, get user input, resume from checkpoint |
| **AgentAsTool** | Wrap an agent as a tool callable by another agent |

-> Use `/eino-agent` for building agents, configuring middleware, runners, and human-in-the-loop.

### Schema

Shared data types used across all layers:

- `schema.Message` -- Conversation message (system/user/assistant/tool roles, content, tool calls)
- `schema.Document` -- Document with content, metadata, and vector embeddings
- `schema.ToolInfo` -- Tool description with JSON schema parameters
- `schema.StreamReader[T]` -- Generic streaming reader (always `defer stream.Close()`)

## Repositories

| Repository | Role |
|---|---|
| `github.com/cloudwego/eino` | Core: interfaces, schema, compose engine, ADK, callbacks |
| `github.com/cloudwego/eino-ext` | Implementations: model providers, vector stores, tools, callback handlers |

## Packages at a Glance

**eino (core):**

| Package | Contains |
|---------|----------|
| `schema` | Message, Document, ToolInfo, StreamReader |
| `components/model` | ChatModel interfaces |
| `components/tool` | Tool interfaces (BaseTool, InvokableTool, StreamableTool, Enhanced variants) |
| `components/embedding` | Embedder interface |
| `components/retriever` | Retriever interface |
| `components/indexer` | Indexer interface |
| `components/document` | Loader, Transformer interfaces |
| `components/prompt` | ChatTemplate interface |
| `compose` | Graph, Chain, Workflow, ToolsNode, Runnable, state, checkpoint |
| `callbacks` | Handler interface, global/per-run registration |
| `adk` | Agent, Runner, ChatModelAgent, middleware, interrupt/resume |
| `adk/prebuilt/deep` | DeepAgent preset |

**eino-ext (implementations):**

| Package | Contains |
|---------|----------|
| `components/model/{provider}` | ChatModel implementations (openai, claude, gemini, ark, ollama, deepseek, qwen, etc.) |
| `components/embedding/{provider}` | Embedding implementations (openai, ark, ollama, etc.) |
| `components/retriever/{backend}` | Retriever implementations (redis, milvus2, es8, qdrant) |
| `components/indexer/{backend}` | Indexer implementations (redis, milvus2, es8, qdrant) |
| `components/tool/{type}` | Tool implementations (mcp, googlesearch, duckduckgo, bingsearch, etc.) |
| `callbacks/{provider}` | Callback handlers (cozeloop, apmplus, langfuse, langsmith) |
| `adk/backend/local` | Local filesystem Backend for DeepAgent |

## Choosing Your Approach

| Scenario | Approach | Skill |
|----------|----------|-------|
| Single model call (generate or stream) | Use ChatModel directly | `/eino-component` |
| Multi-turn agent with tools | ChatModelAgent + Runner | `/eino-agent` |
| Production agent with filesystem, tool search | DeepAgent | `/eino-agent` |
| Linear pipeline (template -> model) | Chain | `/eino-compose` |
| Complex flow with branching or loops | Graph | `/eino-compose` |
| Parallel branches with field mapping | Workflow | `/eino-compose` |
| RAG (embed + index + retrieve) | Indexer + Retriever + Embedding | `/eino-component` |
| Agent with human approval | Interrupt/Resume + Runner | `/eino-agent` |
| Observability and tracing | Callback handlers | `/eino-component` |

## Reference Files

- `reference/schema.md` -- Core data types shared across all layers: Message, Document, ToolInfo, StreamReader
- `reference/runnable.md` -- Runnable[I, O] interface, four execution modes, runtime options
- `reference/quick-start.md` -- Three complete working examples (ChatModel, Agent+Runner, Chain)

## Instructions to Agent

1. Route to the appropriate skill (`/eino-component`, `/eino-compose`, `/eino-agent`) when the question is specific. Consult the "Choosing Your Approach" table.
2. Always provide Go code examples using real import paths from `github.com/cloudwego/eino` and `github.com/cloudwego/eino-ext`.
3. For component implementation details, always read the provider's reference file before generating code. Do not assume constructor or config naming conventions.
4. Prefer ADK (ChatModelAgent + Runner) for agent use cases over manually building ReAct loops with compose graphs.
5. When showing streaming code, always include `defer stream.Close()`.
