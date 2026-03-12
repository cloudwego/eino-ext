---
name: eino-component
description: Eino component selection, configuration, and usage. Use when a user needs to choose or configure a ChatModel, Embedding, Retriever, Indexer, Tool, Document loader/parser/transformer, Prompt template, or Callback handler. Covers all component interfaces and their implementations in eino-ext including OpenAI, Claude, Gemini, Ollama, Milvus, Elasticsearch, Redis, MCP tools, and more.
---

# Eino Component Guide

## Component Selection Guide

### ChatModel -- LLM inference

| Provider | Package | Tool Calling | Streaming | Notes |
|----------|---------|:---:|:---:|-------|
| OpenAI | `model/openai` | Yes | Yes | Also supports Azure via `ByAzure: true` |
| Claude | `model/claude` | Yes | Yes | Also supports AWS Bedrock via `ByBedrock: true` |
| Gemini | `model/gemini` | Yes | Yes | Requires `genai.Client` |
| Ark (Volcengine) | `model/ark` | Yes | Yes | Doubao models |
| Ollama | `model/ollama` | Yes | Yes | Local models |
| DeepSeek | `model/deepseek` | Yes | Yes | Reasoning support |
| Qwen | `model/qwen` | Yes | Yes | Alibaba DashScope API |
| Qianfan | `model/qianfan` | Yes | Yes | Baidu ERNIE models |
| OpenRouter | `model/openrouter` | Yes | Yes | Multi-provider routing |

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

// Document: Loader, Transformer
// Tool: InvokableTool (Info + InvokableRun)
// Prompt: ChatTemplate (Format)
```

## Common Configuration Pattern

All components follow `NewXxx(ctx, &XxxConfig{...})`:

```go
chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4o",
})

embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
    APIKey: "your-key",
    Model:  "text-embedding-3-small",
})

retriever, err := redis.NewRetriever(ctx, &redis.RetrieverConfig{
    Client: redisClient,
    Index:  "my_index",
})
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

- Always check the component README in `eino-ext/components/{type}/{impl}/` for the full Config struct and examples.
- Provide the complete Config struct with required fields filled in.
- Use `ToolCallingChatModel` (not deprecated `ChatModel`) for tool binding.
- For RAG, ensure the same Embedder model is used for both indexing and retrieval.
- See reference files for detailed per-component documentation.

## Reference Files

Read these files on-demand for detailed API, examples, and advanced usage:

- [reference/chat-model.md](reference/chat-model.md) -- All ChatModel implementations with config fields and examples
- [reference/embedding-and-retrieval.md](reference/embedding-and-retrieval.md) -- Embedder and Retriever interfaces, implementations, RAG example
- [reference/indexer.md](reference/indexer.md) -- Indexer interface, implementations, indexing pipeline
- [reference/document-pipeline.md](reference/document-pipeline.md) -- Loader, Parser, Transformer interfaces and implementations
- [reference/tool.md](reference/tool.md) -- Tool interfaces, MCP integration, search/utility tools, custom tool creation
- [reference/prompt-and-callback.md](reference/prompt-and-callback.md) -- ChatTemplate, FString templates, callback handlers
