# OpenAI 模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的 OpenAI 模型实现，实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

## 特性

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Custom response parsing support
- Flexible model configuration

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/openai@latest
```

## 快速开始

以下是如何使用 OpenAI 模型的快速示例：

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		// 如果您想使用 Azure OpenAI 服务，请设置这两个字段。
		// BaseURL: "https://{RESOURCE_NAME}.openai.azure.com",
		// ByAzure: true,
		// APIVersion: "2024-06-01",
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		ByAzure: func() bool {
			if os.Getenv("OPENAI_BY_AZURE") == "true" {
				return true
			}
			return false
		}(),
		ReasoningEffort: openai.ReasoningEffortLevelHigh,
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}
	fmt.Printf("output: \n%v", resp)

}


```

## 使用 schema.Message 调用 Responses API

`NewChatModel` 继续使用 Chat Completions。如果现有 `schema.Message` /
`ToolCallingChatModel` 流程需要 `/v1/responses`，可使用 `NewResponsesChatModel`，
无需将消息和中间件迁移到 `schema.AgenticMessage`。

```go
cm, err := openai.NewResponsesChatModel(ctx, &openai.ResponsesChatModelConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model: os.Getenv("OPENAI_MODEL"),
    BaseURL: os.Getenv("OPENAI_BASE_URL"), // 可选
    ReasoningEffort: openai.ResponsesReasoningEffortHigh,
})
```

该模型支持文本和图片输入、文本输出、自定义函数工具（`WithTools` / `BindTools`）、
工具选择、流式输出、回调和 token 用量。加密 reasoning 保存在 `Message.Extra` 中并随
历史回传，保存会话时需保留完整消息及 `Extra`。正文和工具调用仍通过 Eino 消息转换。
服务端内置工具及任意原生 Responses 内容块不在此适配器范围内；需要 `AgenticMessage`
接口时，请使用 `agenticopenai`。

通用参数继续使用 `model.WithMaxTokens`、`model.WithTemperature` 等选项。Responses
专用参数使用 `WithResponsesReasoningEffort`、`WithResponsesStore`、
`WithResponsesPromptCacheKey` 和 `WithResponsesPromptCacheRetention`；现有 Chat
Completions 专用选项不会应用于 Responses 模型。

默认回传完整历史，response 存储行为遵循供应商默认值。只有明确需要通过已存储的
response ID 续接时才使用 `WithResponsesUseResponseID(true)`，并确保供应商已存储并
支持该 response，例如使用 `WithResponsesStore(true)`。SDK 内部重试关闭，由应用或
ADK 控制重试。

### 代理商扩展流事件

通过 `ResponsesChatModelConfig.StreamEventHandler` 处理 SSE 扩展事件，额外字段可从
`event.RawJSON()` 读取。回调返回 `ResponsesStreamEventResult`：

- `Error`：立即结束流，原样返回错误，保留 `errors.Is` / `errors.As` 判断，供重试和 failover 使用。
- `IncompleteError`：仅在流缺少终止响应便结束时使用。终止响应或 SDK/网络错误优先，
  适合处理可能伴随成功响应出现的限额通知。
- 返回零值或不配置回调时，标准事件处理行为不变。

每条流内部按顺序调用回调，不同请求可能并发调用同一回调。适配器为每条流独立保存
`IncompleteError`，接入方不需要共享可变状态来追踪限流。代理事件名称、错误解析与
错误标识由接入方维护，不内置于通用适配器中。

此实现依赖官方 OpenAI Go SDK，模块最低 Go 版本为 1.22。
[完整示例](examples/responses/responses.go) 展示了生成、历史回传和流式调用。

## 配置

可以使用 `openai.ChatModelConfig` 结构体配置模型：

```go

type ChatModelConfig struct {
// APIKey is your authentication key
// Use OpenAI API key or Azure API key depending on the service
// Required
APIKey string `json:"api_key"`

// Timeout specifies the maximum duration to wait for API responses
// If HTTPClient is set, Timeout will not be used.
// Optional. Default: no timeout
Timeout time.Duration `json:"timeout"`

// HTTPClient specifies the client to send HTTP requests.
// If HTTPClient is set, Timeout will not be used.
// Optional. Default &http.Client{Timeout: Timeout}
HTTPClient *http.Client `json:"http_client"`

// The following three fields are only required when using Azure OpenAI Service, otherwise they can be ignored.
// For more details, see: https://learn.microsoft.com/en-us/azure/ai-services/openai/

// ByAzure indicates whether to use Azure OpenAI Service
// Required for Azure
ByAzure bool `json:"by_azure"`

// AzureModelMapperFunc is used to map the model name to the deployment name for Azure OpenAI Service.
// This is useful when the model name is different from the deployment name.
// Optional for Azure, remove [,:] from the model name by default.
AzureModelMapperFunc func(model string) string

// BaseURL is the Azure OpenAI endpoint URL
// Format: https://{YOUR_RESOURCE_NAME}.openai.azure.com. YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
// Required for Azure
BaseURL string `json:"base_url"`

// APIVersion specifies the Azure OpenAI API version
// Required for Azure
APIVersion string `json:"api_version"`

// The following fields correspond to OpenAI's chat completion API parameters
// Ref: https://platform.openai.com/docs/api-reference/chat/create

// Model specifies the ID of the model to use
// Required
Model string `json:"model"`

// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
// Optional. Default: model's maximum
// Deprecated: use MaxCompletionTokens. Not compatible with o1-series models.
// refs: https://platform.openai.com/docs/api-reference/chat/create#chat-create-max_tokens
MaxTokens *int `json:"max_tokens,omitempty"`

// MaxCompletionTokens specifies an upper bound for the number of tokens that can be generated for a completion, including visible output tokens and reasoning tokens.
MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

// Temperature specifies what sampling temperature to use
// Generally recommend altering this or TopP but not both.
// Range: 0.0 to 2.0. Higher values make output more random
// Optional. Default: 1.0
Temperature *float32 `json:"temperature,omitempty"`

// TopP controls diversity via nucleus sampling
// Generally recommend altering this or Temperature but not both.
// Range: 0.0 to 1.0. Lower values make output more focused
// Optional. Default: 1.0
TopP *float32 `json:"top_p,omitempty"`

// Stop sequences where the API will stop generating further tokens
// Optional. Example: []string{"\n", "User:"}
Stop []string `json:"stop,omitempty"`

// PresencePenalty prevents repetition by penalizing tokens based on presence
// Range: -2.0 to 2.0. Positive values increase likelihood of new topics
// Optional. Default: 0
PresencePenalty *float32 `json:"presence_penalty,omitempty"`

// ResponseFormat 指定模型响应的格式
// 可选。用于结构化输出
ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

// Seed enables deterministic sampling for consistent outputs
// Optional. Set for reproducible results
Seed *int `json:"seed,omitempty"`

// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
// Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
// Optional. Default: 0
FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

// LogitBias modifies likelihood of specific tokens appearing in completion
// Optional. Map token IDs to bias values from -100 to 100
LogitBias map[string]int `json:"logit_bias,omitempty"`

// User unique identifier representing end-user
// Optional. Helps OpenAI monitor and detect abuse
User *string `json:"user,omitempty"`

// ExtraFields will override any existing fields with the same key.
// Optional. Useful for experimental features not yet officially supported.
ExtraFields map[string]any `json:"extra_fields,omitempty"`

// ReasoningEffort will override the default reasoning level of "medium"
// Optional. Useful for fine tuning response latency vs. accuracy
ReasoningEffort ReasoningEffortLevel

// Modalities are output types that you would like the model to generate. Most models are capable of generating text, which is the default: ["text"]
// The gpt-4o-audio-preview model can also be used to generate audio. To request that this model generate both text and audio responses, you can use: ["text", "audio"]
Modalities []Modality `json:"modalities,omitempty"`

// Audio parameters for audio output. Required when audio output is requested with modalities: ["audio"]
Audio *Audio `json:"audio,omitempty"`
}
```


## 示例

查看以下示例了解更多用法：

- [音频生成](./examples/audio_generate/)
- [基础生成](./examples/generate/)
- [图像输入](./examples/generate_with_image/)
- [意图识别与工具调用](./examples/intent_tool/)
- [流式响应](./examples/stream/)
- [结构化输出](./examples/structured/)
- [超时处理](./examples/timeout/)



## 更多信息

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [OpenAI Documentation](https://platform.openai.com/docs/api-reference/chat/create)
