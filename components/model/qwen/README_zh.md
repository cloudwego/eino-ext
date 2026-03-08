# Qwen 模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的 Qwen 模型实现，实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

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
go get github.com/cloudwego/eino-ext/components/model/qwen@latest
```

## 快速开始

以下是如何使用 Qwen 模型的快速示例：

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    
    "github.com/cloudwego/eino-ext/components/model/qwen"
    "github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://help.aliyun.com/zh/model-studio/developer-reference/get-api-key?spm=a2c4g.11186623.help-menu-2400256.d_3_0.1ebc47bb0ClCgF
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qwen failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("as a machine, how do you answer user's question?"),
	})
	if err != nil {
		log.Fatalf("Generate of qwen failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)

}

func of[T any](t T) *T {
	return &t
}
```

## 配置

可以使用 `qwen.ChatModelConfig` 结构体配置模型：
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

// BaseURL specifies the QWen endpoint URL
// Required. Example: https://dashscope.aliyuncs.com/compatible-mode/v1
BaseURL string `json:"base_url"`

// The following fields correspond to OpenAI's chat completion API parameters
// Ref: https://platform.openai.com/docs/api-reference/chat/create

// Model specifies the ID of the model to use
// Required
Model string `json:"model"`

// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
// Optional. Default: model's maximum
MaxTokens *int `json:"max_tokens,omitempty"`

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

// ResponseFormat specifies the format of the model's response
// Optional. Use for structured outputs
ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

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

// EnableThinking enables thinking mode
// https://help.aliyun.com/zh/model-studio/deep-thinking
// Optional. Default: base on the Model
EnableThinking *bool `json:"enable_thinking,omitempty"`
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
