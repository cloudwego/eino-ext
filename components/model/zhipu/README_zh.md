# 智谱AI GLM 模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的智谱AI GLM 模型实现，实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/model.Model`
- 轻松与 Eino 的模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 支持工具调用（Function Calling）
- 支持思考模式（Thinking Mode）
- 支持图像理解（多模态模型）
- 灵活的模型配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/zhipu@latest
```

## 快速开始

以下是如何使用智谱AI GLM 模型的快速示例：

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/cloudwego/eino-ext/components/model/zhipu"
    "github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// 获取 API Key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("你好，请介绍一下智谱AI。"),
	})
	if err != nil {
		log.Fatalf("Generate of zhipu failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)

}

func of[T any](t T) *T {
	return &t
}
```

## 配置

可以使用 `zhipu.ChatModelConfig` 结构体配置模型：

```go
type ChatModelConfig struct {

// APIKey 是您的身份验证密钥
// 必填
APIKey string `json:"api_key"`

// Timeout 指定等待 API 响应的最长时间
// 如果设置了 HTTPClient，则 Timeout 不会被使用
// 可选。默认：无超时
Timeout time.Duration `json:"timeout"`

// HTTPClient 指定用于发送 HTTP 请求的客户端
// 如果设置了 HTTPClient，则 Timeout 不会被使用
// 可选。默认：&http.Client{Timeout: Timeout}
HTTPClient *http.Client `json:"http_client"`

// BaseURL 指定智谱AI 端点 URL
// 必填。示例：https://open.bigmodel.cn/api/paas/v4/
BaseURL string `json:"base_url"`

// 以下字段对应 OpenAI 的聊天补全 API 参数
// 参考：https://platform.openai.com/docs/api-reference/chat/create

// Model 指定要使用的模型 ID
// 必填
Model string `json:"model"`

// MaxTokens 限制聊天补全中可以生成的最大 token 数量
// 可选。默认：模型的最大值
MaxTokens *int `json:"max_tokens,omitempty"`

// Temperature 指定使用的采样温度
// 通常建议更改此项或 TopP，但不要同时更改两者
// 范围：0.0 到 1.0。较高的值使输出更随机
// 可选。默认：0.6
Temperature *float32 `json:"temperature,omitempty"`

// TopP 通过核采样控制多样性
// 通常建议更改此项或 Temperature，但不要同时更改两者
// 范围：0.0 到 1.0。较低的值使输出更集中
// 可选。默认：0.95
TopP *float32 `json:"top_p,omitempty"`

// Stop API 将停止生成更多 token 的序列
// 可选。示例：[]string{"\n", "User:"}
Stop []string `json:"stop,omitempty"`

// PresencePenalty 通过基于存在来惩罚 token 来防止重复
// 范围：-2.0 到 2.0。正值增加新主题的可能性
// 可选。默认：0
PresencePenalty *float32 `json:"presence_penalty,omitempty"`

// ResponseFormat 指定模型响应的格式
// 可选。用于结构化输出
ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

// Seed 启用确定性采样以获得一致的输出
// 可选。设置以获得可重现的结果
Seed *int `json:"seed,omitempty"`

// FrequencyPenalty 通过基于频率惩罚 token 来防止重复
// 范围：-2.0 到 2.0。正值降低重复的可能性
// 可选。默认：0
FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

// LogitBias 修改特定 token 在补全中出现的可能性
// 可选。将 token ID 映射到 -100 到 100 的偏置值
LogitBias map[string]int `json:"logit_bias,omitempty"`

// User 代表最终用户的唯一标识符
// 可选。帮助监控和检测滥用
User *string `json:"user,omitempty"`

// Thinking 思考模式配置
// 可选。用于复杂推理问题
Thinking *Thinking `json:"thinking,omitempty"`
}

```

## 示例

### 文本生成

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// 获取 API Key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("你好，请介绍一下智谱AI。"),
	})
	if err != nil {
		log.Fatalf("Generate of zhipu failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)

}

func of[T any](t T) *T {
	return &t
}

```

### 流式生成

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// 获取 API Key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	cm, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	sr, err := cm.Stream(ctx, []*schema.Message{
		schema.UserMessage("你好"),
	})
	if err != nil {
		log.Fatalf("Stream of zhipu failed, err=%v", err)
	}

	var msgs []*schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatalf("Stream of zhipu failed, err=%v", err)
		}

		fmt.Println(msg)
		// assistant: 你好
		// finish_reason:
		// : ！
		// finish_reason:
		// : 有什么
		// finish_reason:
		// : 可以帮助
		// finish_reason:
		// : 你的吗？
		// finish_reason:
		// :
		// finish_reason: stop
		// usage: &{9 7 16}
		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	fmt.Println(msg)
	// assistant: 你好！有什么可以帮助你的吗？
	// finish_reason: stop
	// usage: &{9 7 16}
}

func of[T any](t T) *T {
	return &t
}

```

### 思考模式（Thinking Mode）

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// 获取 API Key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	temp := float32(0.9)

	cm, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       "glm-4.7",
		Temperature: &temp,
		Thinking: &zhipu.Thinking{
			Type: zhipu.ThinkingEnabled,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	sr, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "you are a helpful assistant",
		},
		{
			Role:    schema.User,
			Content: "what is the revolution of llm?",
		},
	})
	if err != nil {
		log.Fatalf("Stream of zhipu failed, err=%v", err)
	}

	var msgs []*schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatalf("Stream of zhipu failed, err=%v", err)
		}

		// 思考内容
		if msg.ReasoningContent != "" {
			fmt.Printf("思考: %s\n", msg.ReasoningContent)
		}

		// 回答内容
		if msg.Content != "" {
			fmt.Printf("回答: %s\n", msg.Content)
		}

		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	fmt.Println("\n完整响应:")
	fmt.Printf("思考内容: %s\n", msg.ReasoningContent)
	fmt.Printf("回答内容: %s\n", msg.Content)
}

```

### 工具调用

```go

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// 获取 API Key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	cm, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	err = cm.BindTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "获取指定城市的天气信息",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "城市名称，例如：北京、上海",
					},
				}),
		},
	})
	if err != nil {
		log.Fatalf("BindTools of zhipu failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "北京今天天气怎么样？",
		},
	})

	if err != nil {
		log.Fatalf("Generate of zhipu failed, err=%v", err)
	}

	fmt.Println(resp)
	// assistant:
	// tool_calls: [{0x14000275930 call_xxx function {get_weather {"location": "北京"}} map[]}]
	// finish_reason: tool_calls
	// usage: &{100 20 120}

	// ==========================
	// using stream
	fmt.Printf("\n\n======== Stream ========\n")
	sr, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "上海今天天气怎么样？",
		},
	})
	if err != nil {
		log.Fatalf("Stream of zhipu failed, err=%v", err)
	}

	msgs := make([]*schema.Message, 0)
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			log.Fatalf("json.Marshal failed, err=%v", err)
		}
		fmt.Printf("%s\n", jsonMsg)
		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("json.Marshal failed, err=%v", err)
	}
	fmt.Printf("final: %s\n", jsonMsg)
}

func of[T any](t T) *T {
	return &t
}

```



## 更多信息
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [智谱AI 文档](https://docs.bigmodel.cn)
