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
