# Eino Extension

[English](README.md) | 中文

## 详细文档

EinoExt 项目为 [Eino](https://github.com/cloudwego/eino) 框架提供了各种扩展。Eino 框架是一个功能强大且灵活的用于构建大语言模型（LLM）应用程序的框架。这些扩展包括：

- **组件实现**: Eino 组件类型的官方实现。

### 组件概览

| 组件类型 | 数量 | 官方实现 |
|---------|------|---------|
| **ChatModel** | 10 | OpenAI, Claude, Gemini, Ark, ArkBot, Ollama, Qwen, Qianfan, DeepSeek, OpenRouter |
| **Embedding** | 8 | OpenAI, Ark, Gemini, Ollama, Qianfan, DashScope, TencentCloud, Cache |
| **Indexer** | 10 | Elasticsearch (7/8/9), OpenSearch (2/3), Milvus, Milvus2, Qdrant, Redis, Volc VikingDB |
| **Retriever** | 12 | Elasticsearch (7/8/9), OpenSearch (2/3), Milvus, Milvus2, Qdrant, Redis, Dify, Volc VikingDB, Volc Knowledge |
| **Tool** | 10 | Bing Search, DuckDuckGo, Google Search, Wikipedia, SearXNG, BrowserUse, Command Line, HTTP Request, MCP, Sequential Thinking |
| **Prompt** | 2 | CozeLoop, MCP |
| **Document** | 12 | File/URL/S3 加载器, HTML/PDF/DOCX/XLSX 解析器, Recursive/HTML/Markdown/Semantic 分割器, Score Reranker |
| **Callback Handler** | 4 | APMPlus, CozeLoop, Langfuse, Langsmith |

📋 **查看详细的组件信息（导入路径、描述、功能特性和 GitHub 链接），请参阅 [components.md](components.md)**

有关组件类型的更多详细信息，请参阅 [Eino 组件文档.](https://www.cloudwego.io/zh/docs/eino/core_modules/components/)

有关组件实现的更多详细信息，请参阅 [Eino 生态系统文档.](https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/)

- **DevOps 工具**: 用于 Eino 的 IDE 插件，支持可视化调试、基于 UI 的图形编辑等功能。更多详细信息，请参阅 [Eino Dev 工具文档.](https://www.cloudwego.io/zh/docs/eino/core_modules/devops/)

## 安全

如果你在该项目中发现潜在的安全问题，或你认为可能发现了安全问题，请通过我们的[安全中心](https://security.bytedance.com/src)或[漏洞报告邮箱](sec@bytedance.com)通知字节跳动安全团队。

请**不要**创建公开的 GitHub Issue。

## 开源许可证

本项目依据 [Apache-2.0 许可证](LICENSE.txt) 授权。
