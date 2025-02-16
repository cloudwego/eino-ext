# Eino Extension

[English](README.md) | 中文

## 详细文档

EinoExt 项目为 [Eino](https://github.com/cloudwego/eino) 框架提供了各种扩展。Eino 框架是一个功能强大且灵活的用于构建大语言模型（LLM）应用程序的框架。这些扩展包括：

- **组件实现**: Eino 组件类型的官方实现。

| 组件类型                 | 官方组件实现                                 |
|----------------------|----------------------------------------|
| ChatModel            | OpenAI, Claude, Gemini, Ark, Ollama... |
| Tool                 | Google Search, Duck Duck Go...         |
| Retriever            | Elastic Search, Volc VikingDB...       |
| ChatTemplate         | DefaultChatTemplate...                 |
| Document Loader      | WebURL, Amazon S3, File...             |
| Document Transformer | HTMLSplitter, ScoreReranker...         |
| Indexer              | Elastic Search, Volc VikingDB...       |
| Embedding            | OpenAI, Ark...                         |
| Lambda               | JSONMessageParser...                   |


有关组件类型的更多详细信息，请参阅 [Eino 组件文档.](https://www.cloudwego.io/zh/docs/eino/core_modules/components/)

有关组件实现的更多详细信息，请参阅 [Eino 生态系统文档.](https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/)

- **callback handlers**: 实现 Eino 的 callbacks.Handler 接口的官方 callback handler，例如[Langfuse tracing](https://langfuse.com/docs/tracing) 回调.
- **DevOps 工具**: 用于 Eino 的 IDE 插件，支持可视化调试、基于 UI 的图形编辑等功能。更多详细信息，请参阅 [Eino Dev 工具文档.](https://www.cloudwego.io/zh/docs/eino/core_modules/devops/)

## 安全

如果你在该项目中发现潜在的安全问题，或你认为可能发现了安全问题，请通过我们的[安全中心](https://security.bytedance.com/src)或[漏洞报告邮箱](sec@bytedance.com)通知字节跳动安全团队。

请**不要**创建公开的 GitHub Issue。

## 开源许可证

本项目依据 [Apache-2.0 许可证](LICENSE.txt) 授权。
