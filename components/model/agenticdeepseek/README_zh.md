# Agentic DeepSeek 模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的 Agentic DeepSeek 模型实现，实现了 `AgenticModel` 接口。通过 `AgenticMessage` 实现与 Eino agentic 能力的无缝集成，以增强自然语言处理和生成能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/model.AgenticModel`
- 使用 `AgenticMessage` 和结构化 `ContentBlock` 支持丰富的内容类型
- 轻松与 Eino 的 agentic 模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 灵活的模型配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticdeepseek@latest
```

## 快速开始

以下是如何使用 Agentic DeepSeek 模型的快速示例：

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    
    "github.com/cloudwego/eino-ext/components/model/agenticdeepseek"
    "github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	m, err := agenticdeepseek.New(ctx, &agenticdeepseek.Config{
		BaseURL:     "https://api.deepseek.com",
		APIKey:      apiKey,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})

	if err != nil {
		log.Fatalf("New agenticdeepseek model failed, err=%v", err)
	}

	resp, err := m.Generate(ctx, []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "What is the capital of France?"}),
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate of agenticdeepseek failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)
}

func of[T any](t T) *T {
	return &t
}
```

## 配置

可以使用 `agenticdeepseek.Config` 结构体配置模型：

```go
type Config struct {
	// APIKey is your authentication key.
	// Required.
	APIKey string `json:"api_key"`

	// Timeout specifies the maximum duration to wait for API responses.
	// If HTTPClient is set, Timeout will not be used.
	// Optional.
	Timeout time.Duration `json:"timeout"`

	// HTTPClient specifies the client to send HTTP requests.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: &http.Client{Timeout: Timeout}
	HTTPClient *http.Client `json:"http_client"`

	// BaseURL is your custom deepseek endpoint url.
	// Optional. Default: https://api.deepseek.com
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use.
	// Required.
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	TopP *float32 `json:"top_p,omitempty"`

	// Stop sequences where the API will stop generating further tokens.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty prevents repetition by penalizing tokens based on presence.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// ResponseFormatType specifies the format of the model's response.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	ResponseFormatType ResponseFormatType `json:"response_format_type,omitempty"`

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogProbs specifies whether to return log probabilities of the output tokens.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	LogProbs *bool `json:"log_probs,omitempty"`

	// TopLogProbs specifies the number of most likely tokens to return at each token position.
	// Optional. Default see: https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
	TopLogProbs *int `json:"top_log_probs,omitempty"`
}
```

## 示例

查看以下示例了解更多用法：

- [基础生成](./examples/generate/)
- [流式响应](./examples/stream/)
- [意图识别与工具调用](./examples/intent_tool/)

## 更多信息
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [DeepSeek Documentation](https://api-docs.deepseek.com/api/create-chat-completion)
