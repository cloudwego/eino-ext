# 千帆模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的千帆模型实现，实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/model.Model`
- 轻松与 Eino 的模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 自定义响应解析支持
- 灵活的模型配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/qianfan@latest
```

## 快速开始

以下是如何使用千帆模型的快速示例：


```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"
	modelName := os.Getenv("MODEL_NAME")
	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               modelName,
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
		Seed:                of(0),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	ir, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})

	if err != nil {
		log.Fatalf("Generate of qianfan failed, err=%v", err)
	}

	fmt.Println(ir)
	// assistant: 你好！我是文心一言，很高兴与你交流。请问你有什么想问我的吗？无论是关于知识、创作还是其他任何问题，我都会尽力回答你。
}

func of[T any](t T) *T {
	return &t
}
```

## 配置

可以使用 `qianfan.ChatModelConfig` 结构体配置模型：


```go
// ChatModelConfig config for qianfan chat completion
// see: https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Wm3fhy2vb
type ChatModelConfig struct {
	// Model is the model to use for the chat completion.
	Model string

	// LLMRetryCount is the number of times to retry a failed request.
	LLMRetryCount *int

	// LLMRetryTimeout is the timeout for each retry attempt.
	LLMRetryTimeout *float32

	// LLMRetryBackoffFactor is the backoff factor for retries.
	LLMRetryBackoffFactor *float32

	// Temperature controls the randomness of the output. A higher value makes the output more random, while a lower value makes it more focused and deterministic. Default is 0.95, range (0, 1.0].
	Temperature *float32

	// TopP controls the diversity of the output. A higher value increases the diversity of the generated text. Default is 0.7, range [0, 1.0].
	TopP *float32

	// PenaltyScore reduces the generation of repetitive tokens by adding a penalty. A higher value means a larger penalty. Range: [1.0, 2.0].
	PenaltyScore *float64

	// MaxCompletionTokens is the maximum number of tokens to generate in the completion. Range [2, 2048].
	MaxCompletionTokens *int

	// Seed is the random seed for generation. Range (0, 2147483647).
	Seed *int

	// Stop is a list of strings that will stop the generation when the model generates a token that is a suffix of one of the strings.
	Stop []string

	// User is a unique identifier representing the end-user.
	User *string

	// FrequencyPenalty specifies the frequency penalty to control the repetition of generated text. Range [-2.0, 2.0].
	FrequencyPenalty *float64

	// PresencePenalty specifies the presence penalty to control the repetition of generated text. Range [-2.0, 2.0].
	PresencePenalty *float64

	// ParallelToolCalls specifies whether to call tools in parallel. Defaults to true.
	ParallelToolCalls *bool

	// ResponseFormat specifies the format of the response.
	ResponseFormat *ResponseFormat
}
```








## 示例

查看以下示例了解更多用法：

- [基础生成](./examples/generate/)
- [图像输入](./examples/generate_with_image/)
- [流式响应](./examples/stream/)
- [工具调用](./examples/tool/)



## 更多信息

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [千帆文档](https://cloud.baidu.com/doc/qianfan-api/s/rm7u7qdiq)
