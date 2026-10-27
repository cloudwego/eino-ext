# GLM 模型

GLM（智谱 AI / BigModel）在 [Eino](https://github.com/cloudwego/eino) 中的 ChatModel 实现，实现了 `ToolCallingChatModel` 接口。GLM 提供 OpenAI 兼容的对话补全 API，因此本模块是对共享的 OpenAI 兼容传输层的轻量封装，遵循与 `qwen`、`deepseek` 模块一致的模式。

## 特性

- 实现 `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- 对话补全与流式响应
- 工具 / 函数调用
- 推理模型（如 `glm-4.6`、`glm-4.5-flash`）的深度思考模式，通过 `schema.Message.ReasoningContent` 返回思维链
- 内置联网搜索选项

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/glm@latest
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/glm"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// 获取 api key: https://docs.bigmodel.cn/cn/guide/start/model-overview
	chatModel, err := glm.NewChatModel(ctx, &glm.ChatModelConfig{
		APIKey: os.Getenv("ZHIPU_API_KEY"),
		Model:  "glm-4.5-flash",
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("as a machine, how do you answer user's question?"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)
}
```

## 配置

通过 `glm.ChatModelConfig` 配置。`APIKey` 与 `Model` 为必填；`BaseURL` 默认为 `https://open.bigmodel.cn/api/paas/v4`。其余字段（`Temperature`、`TopP`、`MaxTokens`、`Stop`、`PresencePenalty`、`FrequencyPenalty`、`ResponseFormat`、`User`）对应标准的 OpenAI 对话补全参数。

### 推理（深度思考）

可在配置上开启（`EnableThinking`）或按次调用开启（`glm.WithThinking(true)`）。思维链通过 `schema.Message.ReasoningContent` 返回（流式下以 `reasoning_content` 增量返回）。仅在推理模型（如 `glm-4.6`、`glm-4.5-flash`）上生效。

### 联网搜索

`glm.WithWebSearch(true)` 开启 GLM 内置的联网搜索工具。由于 GLM 通过请求的 `tools` 数组下发联网搜索，请勿在同一次调用中与 eino 的函数工具（`BindTools`/`WithTools`）同时使用。

## 示例

- [基础生成](./examples/generate/)
- [流式响应](./examples/stream/)
- [工具调用](./examples/tool/)
- [推理 / 思考](./examples/thinking/)

## 更多信息

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [GLM / BigModel 文档](https://docs.bigmodel.cn/cn/guide/start/model-overview)
