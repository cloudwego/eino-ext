---
name: eino-component
description: Eino component selection, configuration, and usage. Use when a user needs to choose or configure a ChatModel, AgenticModel, Embedding, Retriever, Indexer, Tool, Document loader/parser/transformer, Prompt template, or Callback handler. Covers all component interfaces and their implementations in eino-ext including OpenAI, Claude, Gemini, Ark, Ollama, Milvus, Elasticsearch, Redis, MCP tools, and more.
---

# Eino Component Guide

## Component Selection Guide

### ChatModel -- LLM inference (classic Message path)

| Provider | Package | Notes |
|----------|---------|-------|
| OpenAI | `model/openai` | Also supports Azure via `ByAzure: true` |
| Claude | `model/claude` | Also supports AWS Bedrock via `ByBedrock: true` |
| Gemini | `model/gemini` | Requires `genai.Client` |
| Ark (Volcengine) | `model/ark` | Doubao models |
| Ollama | `model/ollama` | Local models |
| DeepSeek | `model/deepseek` | Reasoning support |
| Qwen | `model/qwen` | Alibaba DashScope API |
| Qianfan | `model/qianfan` | Baidu ERNIE models |
| OpenRouter | `model/openrouter` | Multi-provider routing |

### AgenticModel -- LLM inference (AgenticMessage path)

AgenticModel operates on `*schema.AgenticMessage` with block-based content (reasoning, text, images, audio, video, tool calls/results). Tools are always passed at call time via `model.WithTools` option (no `WithTools` method).

| Provider | Package | Notes |
|----------|---------|-------|
| OpenAI | `model/agenticopenai` | GPT-4o, o1, o3 series |
| Gemini | `model/agenticgemini` | Gemini 2.x models |
| DeepSeek | `model/agenticdeepseek` | DeepSeek-R1 with reasoning |
| Ark (Volcengine) | `model/agenticark` | Doubao models (agentic path) |
| Qwen | `model/agenticqwen` | Qwen series via DashScope |

Detailed configuration references:
- `reference/model/agenticopenai.md`
- `reference/model/agenticgemini.md`
- `reference/model/agenticdeepseek.md`
- `reference/model/agenticark.md`
- `reference/model/agenticqwen.md`

### Embedding -- text to vector

| Provider | Package | Notes |
|----------|---------|-------|
| OpenAI | `embedding/openai` | text-embedding-3-small/large, ada-002 |
| Ark | `embedding/ark` | Volcengine embedding models |
| Gemini | `embedding/gemini` | Google embedding models |
| DashScope | `embedding/dashscope` | Alibaba embedding |
| Ollama | `embedding/ollama` | Local embedding models |
| Qianfan | `embedding/qianfan` | Baidu embedding |

### Retriever -- vector/keyword search

| Backend | Package | Notes |
|---------|---------|-------|
| Redis | `retriever/redis` | KNN and range vector search |
| Milvus 2.x | `retriever/milvus2` | Dense + sparse hybrid, BM25 |
| Elasticsearch 8 | `retriever/es8` | Approximate vector search |
| Qdrant | `retriever/qdrant` | Vector similarity search |

### Indexer -- store documents with vectors

| Backend | Package |
|---------|---------|
| Redis | `indexer/redis` |
| Milvus 2.x | `indexer/milvus2` |
| Elasticsearch 8 | `indexer/es8` |
| Qdrant | `indexer/qdrant` |

### Tools -- model-callable functions

| Tool | Package | Notes |
|------|---------|-------|
| MCP | `tool/mcp` | Model Context Protocol tools |
| Google Search | `tool/googlesearch` | Custom Search JSON API |
| DuckDuckGo | `tool/duckduckgo` | Web search (use v2) |
| Bing Search | `tool/bingsearch` | Bing Web Search API |
| HTTP Request | `tool/httprequest` | Generic HTTP calls |
| Command Line | `tool/commandline` | Shell command execution |
| Browser Use | `tool/browseruse` | Browser automation |

## Interface Quick Reference

```go
// BaseModel (generic)
type BaseModel[M any] interface {
    Generate(ctx context.Context, input []M, opts ...Option) (M, error)
    Stream(ctx context.Context, input []M, opts ...Option) (*schema.StreamReader[M], error)
}

// Type aliases
type BaseChatModel = BaseModel[*schema.Message]       // classic path
type AgenticModel = BaseModel[*schema.AgenticMessage] // agentic path

// ToolCallingChatModel (classic path, adds WithTools)
type ToolCallingChatModel interface {
    BaseChatModel
    WithTools(tools []*schema.ToolInfo) (ToolCallingChatModel, error)
}

// Embedding
type Embedder interface {
    EmbedStrings(ctx context.Context, texts []string, opts ...Option) ([][]float64, error)
}

// Retriever
type Retriever interface {
    Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error)
}

// Indexer
type Indexer interface {
    Store(ctx context.Context, docs []*schema.Document, opts ...Option) (ids []string, err error)
}

// Document
type Loader interface {
    Load(ctx context.Context, src Source, opts ...LoaderOption) ([]*schema.Document, error)
}
type Transformer interface {
    Transform(ctx context.Context, src []*schema.Document, opts ...TransformerOption) ([]*schema.Document, error)
}

// Tool
type BaseTool interface {
    Info(ctx context.Context) (*schema.ToolInfo, error)
}

type InvokableTool interface {
    BaseTool
    InvokableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (string, error)
}

// Prompt
type ChatTemplate interface {
    Format(ctx context.Context, vs map[string]any, opts ...Option) ([]*schema.Message, error)
}
```

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/{type}/{impl}@latest
# Examples:
go get github.com/cloudwego/eino-ext/components/model/openai@latest
go get github.com/cloudwego/eino-ext/components/model/agenticopenai@latest
go get github.com/cloudwego/eino-ext/components/retriever/milvus2@latest
go get github.com/cloudwego/eino-ext/components/tool/mcp@latest
```

## ChatModel Usage (Classic Path)

### Generate

```go
resp, err := chatModel.Generate(ctx, []*schema.Message{
    {Role: schema.User, Content: "Hello"},
})
fmt.Println(resp.Content)
```

### Stream

```go
reader, err := chatModel.Stream(ctx, messages)
defer reader.Close()
for {
    chunk, err := reader.Recv()
    if errors.Is(err, io.EOF) { break }
    if err != nil { return err }
    fmt.Print(chunk.Content)
}
```

### Tool Calling

```go
withTools, err := chatModel.WithTools([]*schema.ToolInfo{toolInfo})
resp, err := withTools.Generate(ctx, messages)
// resp.ToolCalls contains model's tool invocations
```

## AgenticModel Usage

```go
import (
    "github.com/cloudwego/eino-ext/components/model/agenticopenai"
    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/schema"
)

// Create agentic model
am, _ := agenticopenai.New(ctx, &agenticopenai.Config{
    Model:  "gpt-4o",
    APIKey: "your-key",
})

// Tools passed at call time via option for AgenticModel-interface code
resp, err := am.Generate(ctx,
    []*schema.AgenticMessage{schema.UserAgenticMessage("Search for Go tutorials")},
    model.WithTools(toolInfos),
)

// Response contains typed ContentBlocks
for _, block := range resp.ContentBlocks {
    switch block.Type {
    case schema.ContentBlockTypeAssistantGenText:
        fmt.Println(block.AssistantGenText.Text)
    case schema.ContentBlockTypeFunctionToolCall:
        fmt.Printf("Tool call: %s(%s)\n", block.FunctionToolCall.Name, block.FunctionToolCall.Arguments)
    case schema.ContentBlockTypeReasoning:
        fmt.Printf("Reasoning: %s\n", block.Reasoning.Text)
    }
}
```

## RAG Components

Embedding + Indexer + Retriever form the RAG pipeline:

```go
// 1. Embed and store documents
indexer, _ := redisIndexer.NewIndexer(ctx, &redisIndexer.IndexerConfig{
    Client: redisClient, KeyPrefix: "doc:", Embedding: embedder,
})
ids, _ := indexer.Store(ctx, docs)

// 2. Retrieve relevant documents
retriever, _ := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
    Client: redisClient, Index: "my_index", Embedding: embedder,
})
docs, _ := retriever.Retrieve(ctx, "user query", retriever.WithTopK(5))
```

## Tool Usage

### MCP Tools

```go
import mcpp "github.com/cloudwego/eino-ext/components/tool/mcp"

tools, err := mcpp.GetTools(ctx, &mcpp.Config{Cli: mcpClient})
```

### Custom InvokableTool

Implement `Info()` and `InvokableRun()` to create a custom tool.

## Instructions to Agent

- Constructor signatures and Config struct names vary across implementations. Always read the provider's reference file in `reference/{type}/{impl}.md` before generating initialization code.
- Use `BaseChatModel` (classic path) or `AgenticModel` (agentic path) based on the user's needs.
- `model.AgenticModel` does not add a `WithTools` method to the interface. Prefer `model.WithTools(...)` at call time for interface-oriented code.
- For ADK agents, the `ChatModelAgentConfig.Model` field accepts `model.BaseModel[M]` -- both paths work seamlessly.
- For RAG, ensure the same Embedder model is used for both indexing and retrieval.
- See reference files for detailed per-component documentation.

## Reference Files

Read files on-demand for detailed API, config, and examples. Each `{type}/` directory contains an `overview.md` (interfaces + common patterns) and per-implementation files:

- `reference/model/*.md` -- ChatModel and AgenticModel interfaces, tool binding, streaming, and per-provider config (openai, claude, gemini, ark, ollama, deepseek, qwen, qianfan, openrouter)
- `reference/embedding/*.md` -- Embedder interface and per-provider config (openai, ark, ollama, etc.)
- `reference/retriever/*.md` -- Retriever interface, RAG example, and per-backend config (redis, milvus2, es8)
- `reference/indexer/*.md` -- Indexer interface, indexing pipeline, and per-backend config (redis, milvus2, es8, qdrant)
- `reference/tool/*.md` -- Tool interfaces, custom tool creation, MCP integration, search tools, utility tools
- `reference/document/pipeline.md` -- Loader, Parser, Transformer interfaces and full pipeline example
- `reference/prompt.md` -- ChatTemplate, FString/GoTemplate/Jinja2 formats, message helpers
- `reference/callback/*.md` -- Callback handler interface, registration patterns, and per-provider config (cozeloop, apmplus, langfuse, langsmith)
