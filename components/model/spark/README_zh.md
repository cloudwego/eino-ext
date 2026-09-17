# Spark Model

[English](README.md) | 简体中文

[讯飞星火](https://www.xfyun.cn/doc/spark/HTTP%E8%B0%83%E7%94%A8%E6%96%87%E6%A1%A3.html)（iFLYTEK Spark）大模型的 [Eino](https://github.com/cloudwego/eino) 实现，实现了 `ToolCallingChatModel` 接口，可与 Eino 的模型能力无缝集成，用于自然语言处理与生成。

星火提供 OpenAI 兼容的 HTTP API，因此本组件是对共享 OpenAI 兼容客户端的一层薄封装，与 `qwen`、`deepseek` 组件同源。

## 特性

- 实现 `github.com/cloudwego/eino/components/model.ToolCallingChatModel` 接口
- 与 Eino 模型系统轻松集成
- 模型参数可配置
- 支持对话补全与流式响应
- 支持工具调用（Tool Calling）
- 支持自定义请求头 / 额外请求字段

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/spark@latest
```

## 快速开始

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/spark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	cm, err := spark.NewChatModel(ctx, &spark.ChatModelConfig{
		APIKey: os.Getenv("SPARK_API_KEY"), // 讯飞控制台中的「API Password」
		Model:  "4.0Ultra",
	})
	if err != nil {
		log.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage("你是一个乐于助人的助手。"),
		schema.UserMessage("法国的首都是哪里？"),
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	log.Printf("Response: %s", resp.Content)
}
```

## 配置说明

```go
type ChatModelConfig struct {
	// APIKey 是星火 HTTP 服务的「API Password」——讯飞控制台中展示为 APIPassword 的
	// 单一 Bearer 凭据；不是 WebSocket API 使用的 APPID / APIKey / APISecret 三元组。
	// 必填。
	APIKey string

	// Timeout 指定等待 API 响应的最大时长。
	Timeout time.Duration

	// HTTPClient 指定发送 HTTP 请求的客户端。
	HTTPClient *http.Client

	// BaseURL 指定星火 OpenAI 兼容端点。
	// 可选。默认：https://spark-api-open.xf-yun.com/v1
	BaseURL string

	// Model 指定使用的模型 ID。
	// 可选。默认：4.0Ultra。
	Model string

	// 标准 OpenAI 兼容参数（均为可选）：
	MaxTokens        *int
	Temperature      *float32
	TopP             *float32
	Stop             []string
	PresencePenalty  *float32
	ResponseFormat   *openai.ChatCompletionResponseFormat
	Seed             *int
	FrequencyPenalty *float32
	LogitBias        map[string]int
	User             *string
}
```

### 凭据与端点

- **API Password（`APIKey`）**：在[讯飞星火控制台](https://console.xfyun.cn/services/bmx1)获取，即单一的 `APIPassword`，通过 HTTP 请求头 `Authorization: Bearer <APIPassword>` 发送——**不是** WebSocket API 的 `APPID` / `APIKey` / `APISecret` 三元组。
- **端点**：本组件对接 OpenAI 兼容的 HTTP 端点 `https://spark-api-open.xf-yun.com/v1`，请勿把 `BaseURL` 指向 WebSocket 主机 `spark-api.xf-yun.com`。

### 支持的模型（HTTP 端点）

| 模型          | 说明                    |
|---------------|-------------------------|
| `4.0Ultra`    | 旗舰，默认              |
| `generalv3.5` | Spark Max              |
| `max-32k`     | Spark Max，32k 上下文  |
| `generalv3`   | Spark Pro              |
| `pro-128k`    | Spark Pro，128k 上下文 |
| `lite`        | Spark Lite             |

## 流式调用

```go
sr, err := cm.Stream(ctx, []*schema.Message{
	schema.UserMessage("写一首关于月亮的短诗。"),
})
if err != nil {
	log.Fatalf("Stream failed: %v", err)
}
defer sr.Close()

for {
	chunk, err := sr.Recv()
	if err == io.EOF {
		break
	}
	if err != nil {
		log.Fatalf("Recv failed: %v", err)
	}
	log.Print(chunk.Content)
}
```

## 更多信息

- [Eino 文档](https://github.com/cloudwego/eino)
- [讯飞星火 HTTP API 文档](https://www.xfyun.cn/doc/spark/HTTP%E8%B0%83%E7%94%A8%E6%96%87%E6%A1%A3.html)
