# Agentic Qwen 模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的 Agentic Qwen 模型实现，实现了 `AgenticModel` 接口。这使得能够与 Eino 的 Agentic 功能无缝集成，使用 `AgenticMessage` 增强自然语言处理和生成能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/model.AgenticModel`
- 使用 `AgenticMessage` 和结构化 `ContentBlock` 支持丰富的内容类型
- 轻松与 Eino 的 Agentic 模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 灵活的模型配置
- 支持思考模式

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticqwen@latest
```

## 快速开始

以下是如何使用 Agentic Qwen 模型的快速示例：

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    
    "github.com/cloudwego/eino-ext/components/model/agenticqwen"
    "github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	m, err := agenticqwen.New(ctx, &agenticqwen.Config{
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})

	if err != nil {
		log.Fatalf("New agenticqwen model failed, err=%v", err)
	}

	resp, err := m.Generate(ctx, []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "as a machine, how do you answer user's question?"}),
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate of agenticqwen failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)
}

func of[T any](t T) *T {
	return &t
}
```

## 配置

可以使用 `agenticqwen.Config` 结构体配置模型：

```go
type Config struct {

	// APIKey is your authentication key
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

	// BaseURL specifies the QWen endpoint URL
	// Optional. Default: https://dashscope-intl.aliyuncs.com/compatible-mode/v1
	BaseURL string `json:"base_url"`

	// Model specifies the ID of the model to use
	// Required
	Model string `json:"model"`

	// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
	// Optional. Default: model's maximum
	MaxTokens *int `json:"max_tokens,omitempty"`

	// Temperature specifies what sampling temperature to use
	// Range: 0.0 to 2.0. Higher values make output more random
	// Optional. Default: 1.0
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP controls diversity via nucleus sampling
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
	// Optional. Helps monitor and detect abuse
	User *string `json:"user,omitempty"`

	// EnableThinking enables thinking mode
	// https://help.aliyun.com/zh/model-studio/deep-thinking
	// Optional. Default: base on the Model
	EnableThinking *bool `json:"enable_thinking,omitempty"`

	// PreserveThinking preserves thinking content in multi-turn conversations.
	// https://help.aliyun.com/zh/model-studio/deep-thinking
	// Optional. Default: false
	PreserveThinking *bool `json:"preserve_thinking,omitempty"`

	// Modalities specifies the output data modalities and is only supported by the Qwen-Omni model.
	// Possible values are:
	// - ["text", "audio"]: Output text and audio.
	// - ["text"]: Output text (default).
	Modalities []Modality `json:"modalities,omitempty"`

	// Audio parameters for audio output. Required when modalities includes "audio".
	Audio *AudioConfig `json:"audio,omitempty"`
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
- [Qwen 文档](https://help.aliyun.com/zh/model-studio/use-qwen-by-calling-api)
