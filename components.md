# Eino-Ext Components

This document provides a comprehensive list of all component implementations in the Eino-Ext project, categorized by type.

## Table of Contents

- [ChatModel Components](#chatmodel-components)
- [Embedding Components](#embedding-components)
- [Indexer Components](#indexer-components)
- [Retriever Components](#retriever-components)
- [Tool Components](#tool-components)
- [Prompt Components](#prompt-components)
- [Document Components](#document-components)
- [Callback Handlers](#callback-handlers)

---

## ChatModel Components

ChatModel components provide integrations with various Large Language Model (LLM) providers for chat-based interactions.

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| OpenAI | `github.com/cloudwego/eino-ext/components/model/openai` | OpenAI API integration for GPT models, providing access to GPT-4, GPT-3.5-turbo, and other OpenAI models. | • Support for GPT-4, GPT-3.5-turbo<br>• Streaming and non-streaming responses<br>• Function/tool calling<br>• Vision capabilities (GPT-4V)<br>• Configurable parameters<br>• Custom API base URL support | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/openai) |
| Claude | `github.com/cloudwego/eino-ext/components/model/claude` | Anthropic Claude model integration for accessing Claude 3 family models. | • Support for Claude 3 family (Opus, Sonnet, Haiku)<br>• Streaming responses<br>• Tool use capabilities<br>• Vision support<br>• Long context windows (up to 200K tokens)<br>• System prompts support | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/claude) |
| Gemini | `github.com/cloudwego/eino-ext/components/model/gemini` | Google Gemini model integration for Eino framework. | • Support for Gemini Pro and Gemini Pro Vision<br>• Streaming and non-streaming responses<br>• Tool/function calling support<br>• Multi-modal capabilities (text and images)<br>• Configurable temperature, top-p, top-k<br>• Safety settings configuration | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/gemini) |
| Ark | `github.com/cloudwego/eino-ext/components/model/ark` | ByteDance Ark platform model integration for accessing ByteDance's model platform. | • Access to ByteDance's model platform<br>• Streaming and non-streaming chat<br>• Tool calling support<br>• Multi-modal capabilities<br>• Image generation support<br>• Enterprise-grade reliability | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/ark) |
| ArkBot | `github.com/cloudwego/eino-ext/components/model/arkbot` | ByteDance ArkBot integration for conversational AI applications. | • Bot-specific API integration<br>• Streaming responses<br>• Context management<br>• Tool integration<br>• Conversation history handling | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/arkbot) |
| Ollama | `github.com/cloudwego/eino-ext/components/model/ollama` | Local Ollama model integration for running LLMs locally without API dependencies. | • Run models locally without API keys<br>• Support for various open-source models (Llama, Mistral, etc.)<br>• Streaming responses<br>• Tool calling support<br>• Customizable model parameters<br>• Multi-modal capabilities | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/ollama) |
| Qwen | `github.com/cloudwego/eino-ext/components/model/qwen` | Alibaba Qwen (Tongyi Qianwen) model integration. | • Support for multiple Qwen model variants<br>• Streaming and non-streaming chat<br>• Tool calling support<br>• Configurable generation parameters<br>• Multi-modal support | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/qwen) |
| Qianfan | `github.com/cloudwego/eino-ext/components/model/qianfan` | Baidu Qianfan platform model integration for accessing Baidu's ERNIE models. | • Support for Baidu's ERNIE models<br>• Streaming responses<br>• Function calling<br>• Configurable API endpoints<br>• Authentication via API key and secret key | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/qianfan) |
| DeepSeek | `github.com/cloudwego/eino-ext/components/model/deepseek` | DeepSeek AI model integration for cost-effective inference. | • Support for DeepSeek-V2 and other variants<br>• Streaming chat completions<br>• Function calling<br>• Configurable generation parameters<br>• Cost-effective inference | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/deepseek) |
| OpenRouter | `github.com/cloudwego/eino-ext/components/model/openrouter` | OpenRouter API integration for accessing multiple LLM providers through a unified interface. | • Access to 100+ models from various providers<br>• Unified API interface<br>• Streaming support<br>• Tool calling capabilities<br>• Cost tracking and model routing | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/model/openrouter) |

---

## Embedding Components

Embedding components provide text embedding generation capabilities for semantic search and similarity tasks.

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| OpenAI Embedding | `github.com/cloudwego/eino-ext/components/embedding/openai` | OpenAI embedding service integration for generating text embeddings using OpenAI's embedding models. | • Support for text-embedding-3-small, text-embedding-3-large, and ada-002<br>• Batch processing support<br>• High-quality embeddings<br>• Configurable dimensions | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/openai) |
| Ark Embedding | `github.com/cloudwego/eino-ext/components/embedding/ark` | ByteDance Ark platform embedding service integration. | • ByteDance embedding models<br>• Batch processing<br>• Enterprise support<br>• High performance | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/ark) |
| Gemini Embedding | `github.com/cloudwego/eino-ext/components/embedding/gemini` | Google Gemini embedding service integration. | • Google's embedding models<br>• Multi-language support<br>• Batch processing<br>• High-quality embeddings | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/gemini) |
| Ollama Embedding | `github.com/cloudwego/eino-ext/components/embedding/ollama` | Local embedding generation using Ollama for privacy-focused applications. | • Run embeddings locally<br>• No API key required<br>• Support for various embedding models<br>• Privacy-focused (data stays local)<br>• Batch embedding support | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/ollama) |
| Qianfan Embedding | `github.com/cloudwego/eino-ext/components/embedding/qianfan` | Baidu Qianfan platform embedding service integration. | • Baidu embedding models<br>• Chinese language optimization<br>• Batch processing<br>• API-based service | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/qianfan) |
| DashScope Embedding | `github.com/cloudwego/eino-ext/components/embedding/dashscope` | Alibaba DashScope embedding service integration. | • Alibaba Cloud embedding models<br>• Multi-language support<br>• Batch processing<br>• Cloud-based service | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/dashscope) |
| TencentCloud Embedding | `github.com/cloudwego/eino-ext/components/embedding/tencentcloud` | Tencent Cloud embedding service integration. | • High-quality text embeddings<br>• Batch processing support<br>• Multiple embedding models<br>• Scalable cloud infrastructure<br>• Chinese language optimization | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/tencentcloud) |
| Cache Embedding | `github.com/cloudwego/eino-ext/components/embedding/cache` | Cache embedder for storing and retrieving embeddings efficiently to speed up the embedding process. | • Cache embeddings to avoid recomputation<br>• Support for different caching backends (Redis)<br>• Customizable key generation (hash-based)<br>• Transparent caching layer<br>• Performance optimization | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/embedding/cache) |

---

## Indexer Components

Indexer components provide vector database indexing capabilities for storing and managing embeddings.

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| Elasticsearch 7 | `github.com/cloudwego/eino-ext/components/indexer/es7` | Elasticsearch 7.x indexer integration for full-text search capabilities. | • Elasticsearch 7.x support<br>• Traditional full-text search<br>• Aggregations<br>• Index management<br>• Query DSL | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/es7) |
| Elasticsearch 8 | `github.com/cloudwego/eino-ext/components/indexer/es8` | Elasticsearch 8.x indexer integration with vector search support. | • Elasticsearch 8.x compatibility<br>• Vector search support<br>• Full-text capabilities<br>• Index lifecycle management<br>• Security features | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/es8) |
| Elasticsearch 9 | `github.com/cloudwego/eino-ext/components/indexer/es9` | Elasticsearch 9.x indexer integration with latest features. | • Latest Elasticsearch features<br>• Dense vector search<br>• Sparse vector support<br>• Full-text search<br>• Advanced analytics<br>• Scalable indexing | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/es9) |
| OpenSearch 2 | `github.com/cloudwego/eino-ext/components/indexer/opensearch2` | OpenSearch 2.x indexer integration. | • OpenSearch 2.x support<br>• Vector search capabilities<br>• Full-text search<br>• Index management<br>• Query optimization | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/opensearch2) |
| OpenSearch 3 | `github.com/cloudwego/eino-ext/components/indexer/opensearch3` | OpenSearch 3.x indexer integration. | • Full-text and vector search<br>• OpenSearch 3.x compatibility<br>• Advanced query DSL<br>• Aggregations support<br>• Distributed search | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/opensearch3) |
| Milvus | `github.com/cloudwego/eino-ext/components/indexer/milvus` | Milvus vector database indexer integration. | • Scalable vector database<br>• Multiple index types (IVF, HNSW, etc.)<br>• Hybrid search support<br>• Distributed architecture<br>• High throughput | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/milvus) |
| Milvus2 | `github.com/cloudwego/eino-ext/components/indexer/milvus2` | Milvus 2.x version indexer integration with enhanced features. | • Updated Milvus 2.x API<br>• Improved performance<br>• Enhanced features<br>• Better scalability<br>• Collection management<br>• Multiple index building strategies (HNSW, Auto, Hybrid, etc.) | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/milvus2) |
| Qdrant | `github.com/cloudwego/eino-ext/components/indexer/qdrant` | Qdrant vector database indexer integration. | • High-performance vector storage<br>• HNSW algorithm for fast search<br>• Filtering and payload support<br>• Distributed deployment<br>• REST and gRPC APIs | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/qdrant) |
| Redis | `github.com/cloudwego/eino-ext/components/indexer/redis` | Redis vector database indexer integration. | • Redis-based vector storage<br>• Fast in-memory operations<br>• Vector similarity search<br>• Hybrid search capabilities<br>• Scalable architecture | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/redis) |
| Volc VikingDB | `github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb` | ByteDance Volcengine VikingDB indexer integration. | • Volcengine cloud vector database<br>• High-performance indexing<br>• Scalable infrastructure<br>• Enterprise support<br>• Advanced filtering | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/indexer/volc_vikingdb) |

---

## Retriever Components

Retriever components provide document retrieval capabilities from various vector databases and knowledge bases.

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| Elasticsearch 7 | `github.com/cloudwego/eino-ext/components/retriever/es7` | Elasticsearch 7.x retriever integration for document retrieval. | • ES 7.x support<br>• Traditional search<br>• Query DSL<br>• Aggregations<br>• Relevance tuning<br>• Multiple search modes (exact match, raw string) | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/es7) |
| Elasticsearch 8 | `github.com/cloudwego/eino-ext/components/retriever/es8` | Elasticsearch 8.x retriever integration with vector search. | • ES 8.x compatibility<br>• Vector search<br>• Full-text retrieval<br>• Query optimization<br>• Score boosting<br>• Dense vector similarity search<br>• Sparse vector search | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/es8) |
| Elasticsearch 9 | `github.com/cloudwego/eino-ext/components/retriever/es9` | Elasticsearch 9.x retriever integration with latest features. | • Latest ES features<br>• Dense vector retrieval<br>• Sparse vector retrieval<br>• Full-text search<br>• Hybrid retrieval<br>• Advanced ranking | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/es9) |
| OpenSearch 2 | `github.com/cloudwego/eino-ext/components/retriever/opensearch2` | OpenSearch 2.x retriever integration. | • OpenSearch 2.x compatibility<br>• Vector similarity search<br>• Full-text search<br>• Combined retrieval strategies<br>• Relevance scoring | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/opensearch2) |
| OpenSearch 3 | `github.com/cloudwego/eino-ext/components/retriever/opensearch3` | OpenSearch 3.x retriever integration. | • Vector and full-text retrieval<br>• Hybrid search<br>• Query DSL support<br>• Aggregation-based retrieval<br>• Score normalization | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/opensearch3) |
| Milvus | `github.com/cloudwego/eino-ext/components/retriever/milvus` | Milvus vector database retriever. | • High-performance vector search<br>• Multiple distance metrics<br>• Hybrid search support<br>• Filtering expressions<br>• Top-K retrieval | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/milvus) |
| Milvus2 | `github.com/cloudwego/eino-ext/components/retriever/milvus2` | Milvus 2.x retriever integration with advanced search modes. | • Milvus 2.x API support<br>• Enhanced search performance<br>• Advanced filtering<br>• Partition support<br>• Dynamic field filtering<br>• Multiple search modes (hybrid, iterator, range, scalar, sparse) | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/milvus2) |
| Qdrant | `github.com/cloudwego/eino-ext/components/retriever/qdrant` | Qdrant vector database retriever. | • Fast similarity search<br>• Filtering capabilities<br>• Payload-based retrieval<br>• Score threshold filtering<br>• Batch retrieval | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/qdrant) |
| Redis | `github.com/cloudwego/eino-ext/components/retriever/redis` | Redis vector database retriever. | • Fast in-memory retrieval<br>• Vector similarity search<br>• Hybrid search<br>• Filtering support<br>• High performance | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/redis) |
| Dify | `github.com/cloudwego/eino-ext/components/retriever/dify` | Dify platform retriever integration for knowledge base retrieval. | • Dify knowledge base integration<br>• API-based retrieval<br>• Multi-source retrieval<br>• Relevance scoring<br>• Context management | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/dify) |
| Volc VikingDB | `github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb` | ByteDance Volcengine VikingDB retriever integration. | • Volcengine cloud vector database retrieval<br>• High-performance search<br>• Advanced filtering<br>• Enterprise support<br>• Scalable architecture | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/volc_vikingdb) |
| Volc Knowledge | `github.com/cloudwego/eino-ext/components/retriever/volc_knowledge` | ByteDance Volcengine Knowledge Base retriever integration. | • Volcengine knowledge base integration<br>• Semantic search<br>• Context-aware retrieval<br>• Metadata support<br>• Enterprise features | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/retriever/volc_knowledge) |

---

## Tool Components

Tool components provide various utilities and integrations for extending LLM capabilities.

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| Bing Search | `github.com/cloudwego/eino-ext/components/tool/bingsearch` | Microsoft Bing search engine integration for web search capabilities. | • Bing Web Search API<br>• Rich search results<br>• Image and video search<br>• News search<br>• Entity recognition<br>• Market-specific results | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/bingsearch) |
| DuckDuckGo | `github.com/cloudwego/eino-ext/components/tool/duckduckgo` | DuckDuckGo search engine integration for privacy-focused web search. | • Privacy-focused search<br>• Web search capabilities<br>• No tracking<br>• Instant answers<br>• Safe search | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/duckduckgo) |
| Google Search | `github.com/cloudwego/eino-ext/components/tool/googlesearch` | Google Custom Search API integration. | • Google Custom Search API<br>• Programmable search<br>• Rich search results<br>• Customizable search parameters<br>• Safe search options | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/googlesearch) |
| Wikipedia | `github.com/cloudwego/eino-ext/components/tool/wikipedia` | Wikipedia search and content retrieval tool. | • Search Wikipedia articles<br>• Retrieve article content<br>• Summary extraction<br>• Multi-language support<br>• Structured data extraction | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/wikipedia) |
| SearXNG | `github.com/cloudwego/eino-ext/components/tool/searxng` | SearXNG metasearch engine integration for privacy-focused search. | • Privacy-focused search<br>• Multiple search engine aggregation<br>• Customizable search sources<br>• No tracking<br>• Self-hostable | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/searxng) |
| BrowserUse | `github.com/cloudwego/eino-ext/components/tool/browseruse` | Browser automation and web interaction tool for automated web browsing. | • Browser automation<br>• Web page interaction<br>• Element selection<br>• Screenshot capture<br>• Navigation control<br>• JavaScript execution | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/browseruse) |
| Command Line | `github.com/cloudwego/eino-ext/components/tool/commandline` | Execute command line operations with security constraints. | • Execute shell commands<br>• Python code execution<br>• File editing capabilities<br>• Capture stdout/stderr<br>• Working directory configuration<br>• Environment variables support<br>• Timeout control<br>• Security constraints | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/commandline) |
| HTTP Request | `github.com/cloudwego/eino-ext/components/tool/httprequest` | Generic HTTP request tool for API calls. | • Make HTTP requests (GET, POST, PUT, DELETE, etc.)<br>• Custom headers support<br>• Request body configuration<br>• Response parsing<br>• Timeout configuration<br>• Authentication support | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/httprequest) |
| MCP (Model Context Protocol) | `github.com/cloudwego/eino-ext/components/tool/mcp` | Model Context Protocol tool integration for standardized tool communication. | • Standardized tool protocol<br>• Dynamic tool discovery<br>• Context management<br>• Tool chaining<br>• Protocol-based communication | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/mcp) |
| Sequential Thinking | `github.com/cloudwego/eino-ext/components/tool/sequentialthinking` | Tool for structured sequential reasoning and thinking processes. | • Step-by-step reasoning<br>• Thought chain management<br>• Structured thinking process<br>• Reasoning transparency<br>• Decision tracking | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/tool/sequentialthinking) |

---

## Prompt Components

Prompt components provide prompt management and template capabilities.

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| CozeLoop Prompt | `github.com/cloudwego/eino-ext/components/prompt/cozeloop` | CozeLoop platform prompt integration for prompt template management. | • CozeLoop prompt templates<br>• Variable substitution<br>• Prompt optimization<br>• Template management<br>• Multi-language support | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/prompt/cozeloop) |
| MCP Prompt | `github.com/cloudwego/eino-ext/components/prompt/mcp` | Model Context Protocol prompt management. | • MCP-compliant prompts<br>• Dynamic prompt templates<br>• Context injection<br>• Prompt versioning<br>• Structured prompt format | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/prompt/mcp) |

---

## Document Components

Document components provide document loading, parsing, and transformation capabilities.

### Document Loaders

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| File Loader | `github.com/cloudwego/eino-ext/components/document/loader/file` | Load documents from local file system. | • Load files from local filesystem<br>• Support for various file formats<br>• Batch loading<br>• Metadata extraction | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/loader/file) |
| URL Loader | `github.com/cloudwego/eino-ext/components/document/loader/url` | Load documents from web URLs. | • Load content from URLs<br>• HTTP/HTTPS support<br>• Authentication support<br>• Proxy configuration<br>• Custom headers<br>• Timeout configuration | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/loader/url) |
| S3 Loader | `github.com/cloudwego/eino-ext/components/document/loader/s3` | Load documents from Amazon S3 or S3-compatible storage. | • AWS S3 integration<br>• S3-compatible storage support<br>• Batch loading<br>• Credential management<br>• Prefix-based filtering | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/loader/s3) |

### Document Parsers

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| HTML Parser | `github.com/cloudwego/eino-ext/components/document/parser/html` | Parse HTML documents and extract text content. | • HTML parsing<br>• Text extraction<br>• Tag filtering<br>• Structure preservation<br>• Metadata extraction | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/parser/html) |
| PDF Parser | `github.com/cloudwego/eino-ext/components/document/parser/pdf` | Parse PDF documents and extract text content. | • PDF text extraction<br>• Multi-page support<br>• Layout preservation<br>• Metadata extraction<br>• Image handling | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/parser/pdf) |
| DOCX Parser | `github.com/cloudwego/eino-ext/components/document/parser/docx` | Parse Microsoft Word (DOCX) documents. | • Parse DOCX files<br>• Extract text content<br>• Preserve formatting information<br>• Extract tables and images<br>• Metadata extraction<br>• Style preservation | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/parser/docx) |
| XLSX Parser | `github.com/cloudwego/eino-ext/components/document/parser/xlsx` | Parse Excel (XLSX) files for table data extraction. | • Support for Excel files with or without headers<br>• Select specific worksheets to process<br>• Custom document ID prefixes<br>• Automatic conversion of table data to document format<br>• Preservation of complete row data as metadata<br>• Support for additional metadata injection | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/parser/xlsx) |

### Document Transformers

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| Recursive Splitter | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive` | Recursive text splitter for chunking long documents. | • Split text into chunks recursively<br>• Configurable chunk size<br>• Overlap size configuration to maintain context<br>• Useful for processing long documents<br>• Preserves context between chunks | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/transformer/splitter/recursive) |
| HTML Splitter | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/html` | HTML-aware text splitter that respects HTML structure. | • HTML-aware splitting<br>• Preserve HTML structure<br>• Tag-based chunking<br>• Configurable chunk size<br>• Semantic splitting | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/transformer/splitter/html) |
| Markdown Splitter | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown` | Markdown-aware text splitter that respects Markdown structure. | • Markdown-aware splitting<br>• Header-based chunking<br>• Structure preservation<br>• Configurable chunk size<br>• Semantic splitting | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/transformer/splitter/markdown) |
| Semantic Splitter | `github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic` | Semantic text splitter that uses embeddings for intelligent chunking. | • Semantic-aware splitting<br>• Embedding-based chunking<br>• Intelligent boundary detection<br>• Context preservation<br>• Configurable similarity threshold | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/transformer/splitter/semantic) |
| Score Reranker | `github.com/cloudwego/eino-ext/components/document/transformer/reranker/score` | Score-based document reranker for improving retrieval results. | • Score-based reranking<br>• Relevance optimization<br>• Configurable scoring methods<br>• Result filtering<br>• Top-K selection | [Link](https://github.com/cloudwego/eino-ext/tree/main/components/document/transformer/reranker/score) |

---

## Callback Handlers

Callback handlers provide observability and tracing capabilities for Eino applications.

| Name | Import Path | Description | Key Features | GitHub URL |
|------|-------------|-------------|--------------|------------|
| APMPlus | `github.com/cloudwego/eino-ext/callbacks/apmplus` | Volcengine APMPlus callback implementation for enhanced observability. | • Implements Eino Handler interface<br>• Session functionality for request association<br>• Easy integration with Eino applications<br>• Trace and metrics reporting<br>• Enterprise monitoring | [Link](https://github.com/cloudwego/eino-ext/tree/main/callbacks/apmplus) |
| CozeLoop | `github.com/cloudwego/eino-ext/callbacks/cozeloop` | CozeLoop callback implementation for Eino observability. | • Implements Eino Handler interface<br>• Easy integration with Eino applications<br>• CozeLoop platform integration<br>• Trace collection<br>• Performance monitoring | [Link](https://github.com/cloudwego/eino-ext/tree/main/callbacks/cozeloop) |
| Langfuse | `github.com/cloudwego/eino-ext/callbacks/langfuse` | Langfuse tracing callback for LLM observability. | • Implements Eino Handler interface<br>• Langfuse platform integration<br>• Trace collection<br>• Cost tracking<br>• Performance analytics | [Link](https://github.com/cloudwego/eino-ext/tree/main/callbacks/langfuse) |
| Langsmith | `github.com/cloudwego/eino-ext/callbacks/langsmith` | Langsmith tracing callback for LLM observability and debugging. | • Implements Eino Handler interface<br>• Langsmith platform integration<br>• Trace collection<br>• Session management<br>• Debugging support | [Link](https://github.com/cloudwego/eino-ext/tree/main/callbacks/langsmith) |

---

## Summary

This document cataloged **64 component implementations** across the following categories:

- **10 ChatModel** implementations (OpenAI, Claude, Gemini, Ark, ArkBot, Ollama, Qwen, Qianfan, DeepSeek, OpenRouter)
- **8 Embedding** implementations (OpenAI, Ark, Gemini, Ollama, Qianfan, DashScope, TencentCloud, Cache)
- **10 Indexer** implementations (ES7, ES8, ES9, OpenSearch2, OpenSearch3, Milvus, Milvus2, Qdrant, Redis, Volc VikingDB)
- **12 Retriever** implementations (ES7, ES8, ES9, OpenSearch2, OpenSearch3, Milvus, Milvus2, Qdrant, Redis, Dify, Volc VikingDB, Volc Knowledge)
- **10 Tool** implementations (Bing Search, DuckDuckGo, Google Search, Wikipedia, SearXNG, BrowserUse, Command Line, HTTP Request, MCP, Sequential Thinking)
- **2 Prompt** implementations (CozeLoop, MCP)
- **12 Document** implementations (3 loaders, 4 parsers, 5 transformers)
- **4 Callback** handlers (APMPlus, CozeLoop, Langfuse, Langsmith)

All components are designed to work seamlessly within the [Eino framework](https://github.com/cloudwego/eino) ecosystem and provide standardized interfaces for their respective functionalities.

---

**Project Repository:** https://github.com/cloudwego/eino-ext

**Documentation:** https://www.cloudwego.io/docs/eino/
