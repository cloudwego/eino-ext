---
name: eino-component
description: Eino component selection, configuration, and usage. Use when a user needs to choose or configure a ChatModel, Embedding, Retriever, Indexer, Tool, Document loader/parser/transformer, Prompt template, or Callback handler. Covers all component interfaces and their implementations in eino-ext including OpenAI, Claude, Gemini, Ollama, Milvus, Elasticsearch, Redis, MCP tools, and more.
---

# Eino Component Guide

## Component Selection Guide

### ChatModel -- LLM inference

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
// ChatModel
type BaseChatModel interface {
    Generate(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.Message, error)
    Stream(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.StreamReader[*schema.Message], error)
}
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
type InvokableTool interface {
    Info(ctx context.Context) (*schema.ToolInfo, error)
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
go get github.com/cloudwego/eino-ext/components/retriever/milvus2@latest
go get github.com/cloudwego/eino-ext/components/tool/mcp@latest
```

## ChatModel Usage

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
- Use `ToolCallingChatModel` (not deprecated `ChatModel`) for tool binding.
- For RAG, ensure the same Embedder model is used for both indexing and retrieval.
- See reference files for detailed per-component documentation.

## Reference Files

Read files on-demand for detailed API, config, and examples. Each `{type}/` directory contains an `overview.md` (interfaces + common patterns) and per-implementation files:

- `reference/model/*.md` -- ChatModel interfaces, tool binding, streaming, and per-provider config (openai, claude, gemini, ark, ollama, deepseek, qwen, qianfan, openrouter)
- `reference/embedding/*.md` -- Embedder interface and per-provider config (openai, ark, ollama, etc.)
- `reference/retriever/*.md` -- Retriever interface, RAG example, and per-backend config (redis, milvus2, es8)
- `reference/indexer/*.md` -- Indexer interface, indexing pipeline, and per-backend config (redis, milvus2, es8, qdrant)
- `reference/tool/*.md` -- Tool interfaces, custom tool creation, MCP integration, search tools, utility tools
- `reference/document/pipeline.md` -- Loader, Parser, Transformer interfaces and full pipeline example
- `reference/prompt.md` -- ChatTemplate, FString/GoTemplate/Jinja2 formats, message helpers
- `reference/callback/*.md` -- Callback handler interface, registration patterns, and per-provider config (cozeloop, apmplus, langfuse, langsmith)
