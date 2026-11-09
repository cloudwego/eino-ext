# Eino DevOps

[English](README.md) | 中文

## 详细文档
EinoExt/Devops 项目为 [Eino](https://github.com/cloudwego/eino) 提供可视化调试能力, 请参阅 [Eino Dev 插件调试使用文档.](https://www.cloudwego.io/zh/docs/eino/core_modules/devops/visual_debug_plugin_guide/)

## 自定义文本类型

对于兼容实现了 `encoding.TextMarshaler` 和 `encoding.TextUnmarshaler` 的 Go
类型，Eino Dev 会将其作为 JSON 字符串处理。遵循这一标准接口约定的 UUID
类型只需使用普通的 `json` 字段标签，无需添加 Eino 专用标签。实现了自定义
JSON 编解码接口的类型仍沿用原有的 Schema 处理逻辑。

## 安全

如果你在该项目中发现潜在的安全问题，或你认为可能发现了安全问题，请通过我们的[安全中心](https://security.bytedance.com/src)或[漏洞报告邮箱](mailto:sec@bytedance.com)通知字节跳动安全团队。

请**不要**创建公开的 GitHub Issue。

## 开源许可证

本项目依据 [Apache-2.0 许可证](../LICENSE) 授权。
