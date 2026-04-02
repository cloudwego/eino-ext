# OpenAI Responses API 模型

基于 [OpenAI Responses API](https://platform.openai.com/docs/api-reference/responses) 的 [Eino](https://github.com/cloudwego/eino) 模型实现，实现了 `ToolCallingChatModel` 接口。本组件基于 `/v1/responses` 端点（底层使用 `github.com/openai/openai-go/v3`），支持工具调用、流式输出、多模态输入及内置网络搜索。

## 功能特性

- 实现 `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- 支持 `Generate`（非流式）和 `Stream`（流式）两种调用方式
- 通过 `WithTools` 绑定工具，完整支持 `ToolChoice`（auto / none / required / 指定工具）
- 内置网络搜索工具支持（`EnableWebSearch`）
- 多模态输入：文本、图片 URL、图片 base64、文件 URL、文件 base64
- 输出格式：`text`、`json_object`、`json_schema`
- 支持多轮对话，正确处理 assistant 消息和工具结果消息
- 完整集成 Eino 回调体系

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/openairesponse@latest
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openairesponse"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	chatModel, err := openairesponse.NewResponsesAPIChatModel(ctx, &openairesponse.ResponsesAPIConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o",
	})
	if err != nil {
		log.Fatalf("NewResponsesAPIChatModel 失败: %v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "你好，你是谁？",
		},
	})
	if err != nil {
		log.Fatalf("Generate 失败: %v", err)
	}
	fmt.Println(resp.Content)
}
```

## 流式输出

```go
stream, err := chatModel.Stream(ctx, []*schema.Message{
	{Role: schema.User, Content: "给我讲一个故事。"},
})
if err != nil {
	log.Fatalf("Stream 失败: %v", err)
}
defer stream.Close()

for {
	chunk, err := stream.Recv()
	if err != nil {
		break
	}
	fmt.Print(chunk.Content)
}
```

## 工具调用

```go
tools := []*schema.ToolInfo{
	{
		Name: "get_weather",
		Desc: "获取某城市的当前天气",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {Type: schema.String, Required: true},
		}),
	},
}

modelWithTools, err := chatModel.WithTools(tools)
if err != nil {
	log.Fatalf("WithTools 失败: %v", err)
}

resp, err := modelWithTools.Generate(ctx, []*schema.Message{
	{Role: schema.User, Content: "北京今天天气怎么样？"},
})
```

## 内置网络搜索

通过配置开启：

```go
chatModel, err := openairesponse.NewResponsesAPIChatModel(ctx, &openairesponse.ResponsesAPIConfig{
	APIKey:          os.Getenv("OPENAI_API_KEY"),
	Model:           "gpt-4o",
	EnableWebSearch: true,
})
```

或通过单次调用 Option 开启：

```go
import "github.com/cloudwego/eino/components/model"

resp, err := chatModel.Generate(ctx, messages,
	model.WithImplSpecificOptions(&openairesponse.responsesAPIOptions{
		EnableWebSearch: true,
	}),
)
```

## 多模态输入

```go
resp, err := chatModel.Generate(ctx, []*schema.Message{
	{
		Role: schema.User,
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "这张图片里有什么？"},
			{
				Type:     schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image.png"},
			},
		},
	},
})
```

## 配置说明

```go
type ResponsesAPIConfig struct {
	// APIKey 是 OpenAI API 密钥。
	// 必填。
	APIKey string `json:"api_key"`

	// BaseURL 覆盖默认的 OpenAI API 地址。
	// 可选。默认值："https://api.openai.com/v1"
	BaseURL string `json:"base_url,omitempty"`

	// Timeout 指定 HTTP 客户端超时时间。
	// 若设置了 HTTPClient，则此字段无效。
	// 可选。默认不超时。
	Timeout time.Duration `json:"timeout,omitempty"`

	// HTTPClient 指定自定义 HTTP 客户端。
	// 若设置了此字段，则 Timeout 无效。
	// 可选。
	HTTPClient *http.Client `json:"-"`

	// Model 指定使用的模型 ID。
	// 必填。
	Model string `json:"model"`

	// MaxOutputTokens 设置模型最大输出 token 数上限。
	// 可选。
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`

	// ResponseFormat 配置输出格式（text、json_object 或 json_schema）。
	// 可选。
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// BuiltinTools 指定模型级内置工具（如 file_search）。
	// 可选。
	BuiltinTools []*schema.ToolInfo `json:"tools,omitempty"`

	// EnableWebSearch 开启 OpenAI 内置的 web_search 工具。
	// 可选。默认：false
	EnableWebSearch bool `json:"enable_web_search,omitempty"`
}
```

### 输出格式类型

| 常量 | 值 | 说明 |
|---|---|---|
| `ChatCompletionResponseFormatTypeText` | `"text"` | 纯文本输出（默认） |
| `ChatCompletionResponseFormatTypeJSONObject` | `"json_object"` | JSON 对象输出 |
| `ChatCompletionResponseFormatTypeJSONSchema` | `"json_schema"` | 基于 JSON Schema 的结构化输出 |

## 与 Eino ADK 集成（ChatModelAgent）

`ResponsesAPIChatModel` 可直接作为 `adk.NewChatModelAgent` 的模型参数使用。
同一个模型实例也可以在中间件（如 `summarization`）中复用。

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino-ext/components/model/openairesponse"
)

func main() {
	ctx := context.Background()

	maxTokens := 16000
	cm, err := openairesponse.NewResponsesAPIChatModel(ctx, &openairesponse.ResponsesAPIConfig{
		APIKey:          "your-api-key",
		Model:           "gpt-4o",
		BaseURL:         "https://your-custom-endpoint/v1", // 可选，覆盖默认地址
		EnableWebSearch: true,
		MaxOutputTokens: &maxTokens,
	})
	if err != nil {
		log.Fatalf("NewResponsesAPIChatModel 失败: %v", err)
	}

	// 将同一个模型实例复用于 summarization 中间件
	summaryMw, err := summarization.New(ctx, &summarization.Config{
		Model: cm,
		Trigger: &summarization.TriggerCondition{
			ContextTokens: 100000, // 上下文超过 100k tokens 时触发摘要
		},
	})
	if err != nil {
		log.Fatalf("summarization.New 失败: %v", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "MyAgent",
		Description:   "由 OpenAI Responses API 驱动的 Agent",
		Model:         cm,
		Handlers:      []adk.ChatModelAgentMiddleware{summaryMw},
		MaxIterations: 30,
	})
	if err != nil {
		log.Fatalf("NewChatModelAgent 失败: %v", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})
	_ = runner
	fmt.Println("Agent Runner 创建成功")
}
```

> **说明**：`ResponsesAPIChatModel` 实现了 `model.ToolCallingChatModel` 接口，满足 `adk.ChatModelAgentConfig.Model` 所需的接口约束。同一个模型实例可以安全地在 Agent 和 `summarization` 等中间件之间共享。

## 更多资料

- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)
- [OpenAI Responses API 参考](https://platform.openai.com/docs/api-reference/responses)
- [openai-go SDK](https://github.com/openai/openai-go)
