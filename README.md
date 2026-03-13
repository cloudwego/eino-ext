# Eino Extension

English | [中文](README.zh_CN.md)

## Overview

The EinoExt project hosts various extensions for the [Eino](https://github.com/cloudwego/eino) framework. Eino framework is a powerful and flexible framework for building LLM applications. The extensions include:

- **component implementations**: official implementations for Eino's component types.

### Component Summary

| Component Type | Count | Official Implementations |
|----------------|-------|--------------------------|
| **ChatModel** | 10 | OpenAI, Claude, Gemini, Ark, ArkBot, Ollama, Qwen, Qianfan, DeepSeek, OpenRouter |
| **Embedding** | 8 | OpenAI, Ark, Gemini, Ollama, Qianfan, DashScope, TencentCloud, Cache |
| **Indexer** | 10 | Elasticsearch (7/8/9), OpenSearch (2/3), Milvus, Milvus2, Qdrant, Redis, Volc VikingDB |
| **Retriever** | 12 | Elasticsearch (7/8/9), OpenSearch (2/3), Milvus, Milvus2, Qdrant, Redis, Dify, Volc VikingDB, Volc Knowledge |
| **Tool** | 10 | Bing Search, DuckDuckGo, Google Search, Wikipedia, SearXNG, BrowserUse, Command Line, HTTP Request, MCP, Sequential Thinking |
| **Prompt** | 2 | CozeLoop, MCP |
| **Document** | 12 | File/URL/S3 Loaders, HTML/PDF/DOCX/XLSX Parsers, Recursive/HTML/Markdown/Semantic Splitters, Score Reranker |
| **Callback Handler** | 4 | APMPlus, CozeLoop, Langfuse, Langsmith |

📋 **For detailed component information (import paths, descriptions, features, and GitHub links), see [components.md](components.md)**

For more details about component types, please refer to the [Eino component documentation.](https://www.cloudwego.io/zh/docs/eino/core_modules/components/)

For more details about component implementations, please refer to the [Eino ecosystem documentation.](https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/)

- **DevOps tools**: IDE plugin for Eino that enables visualized debugging, UI based graph editing and more. For more details, please refer to the  [Eino Dev tooling documentation.](https://www.cloudwego.io/zh/docs/eino/core_modules/devops/)

## Security

If you discover a potential security issue in this project, or think you may
have discovered a security issue, we ask that you notify Bytedance Security via
our [security center](https://security.bytedance.com/src) or [vulnerability reporting email](sec@bytedance.com).

Please do **not** create a public GitHub issue.

## License

This project is licensed under the [Apache-2.0 License](LICENSE.txt).
