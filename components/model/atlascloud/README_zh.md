# Atlas Cloud ChatModel

这是一个面向 [Eino](https://github.com/cloudwego/eino) 的 Atlas Cloud ChatModel 实现。
它基于 OpenAI 兼容客户端封装，并将 LLM 默认地址设为 `https://api.atlascloud.ai/v1`。

## 特性

- 实现 `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- 使用 Atlas Cloud 的 OpenAI 兼容 Chat Completions API
- 支持流式输出
- 支持工具调用
- 支持复用 OpenAI 兼容客户端的请求/响应扩展能力

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/atlascloud@latest
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/atlascloud"
)

func main() {
	ctx := context.Background()

	chatModel, err := atlascloud.NewChatModel(ctx, &atlascloud.ChatModelConfig{
		APIKey: os.Getenv("ATLASCLOUD_API_KEY"),
		Model:  os.Getenv("ATLASCLOUD_MODEL"), // 例如 deepseek-ai/DeepSeek-V3-0324
		// BaseURL 可选，默认是 https://api.atlascloud.ai/v1
	})
	if err != nil {
		log.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("请用一句话介绍 Eino。"),
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	fmt.Println(resp.Content)
}
```

## 配置说明

`atlascloud.ChatModelConfig` 是 `openai.ChatModelConfig` 的别名，因此可直接使用同一套 OpenAI 兼容配置字段：

- `APIKey`：必填
- `Model`：必填
- `BaseURL`：可选，默认 `https://api.atlascloud.ai/v1`
- `Timeout`、`HTTPClient`
- `Temperature`、`TopP`、`Stop`
- `MaxCompletionTokens`、`ResponseFormat`、`ReasoningEffort`
- `ExtraFields`、请求/响应 modifier 等 OpenAI 兼容能力

## 说明

- Atlas Cloud 的 Chat API 完全兼容 OpenAI。
- `BaseURL` 必须带上 `/v1` 后缀。
- 模型名请使用 Atlas Cloud 模型库中的模型 ID，例如 `deepseek-ai/DeepSeek-V3-0324`。

## 示例

参考 [examples/basic](./examples/basic/)。
