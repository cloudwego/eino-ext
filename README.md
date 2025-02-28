# Eino Extension

English | [中文](README.zh_CN.md)

## Overview

The EinoExt project hosts various extensions for the [Eino](https://github.com/cloudwego/eino) framework. Eino framework is a powerful and flexible framework for building LLM applications. The extensions include:

- **component implementations**: official implementations for Eino's component types.

| component type       | official implementations               |
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

For more details about component types, please refer to the [Eino component documentation.](https://www.cloudwego.io/zh/docs/eino/core_modules/components/)

For more details about component implementations, please refer to the [Eino ecosystem documentation.](https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/)

- **callback handlers**: official callback handlers implementing Eino's CallbackHandler interface, such as [Langfuse tracing](https://langfuse.com/docs/tracing) callback.
- **DevOps tools**: IDE plugin for Eino that enables visualized debugging, UI based graph editing and more. For more details, please refer to the  [Eino Dev tooling documentation.](https://www.cloudwego.io/zh/docs/eino/core_modules/devops/)

## Security

If you discover a potential security issue in this project, or think you may
have discovered a security issue, we ask that you notify Bytedance Security via
our [security center](https://security.bytedance.com/src) or [vulnerability reporting email](sec@bytedance.com).

Please do **not** create a public GitHub issue.

## License

This project is licensed under the [Apache-2.0 License](LICENSE.txt).
