# DeepSeek 模型

一个针对 [Eino](https://github.com/cloudwego/eino) 的 DeepSeek 模型实现，实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

## 特性

- 实现了 `github.com/cloudwego/eino/components/model.Model`
- 轻松与 Eino 的模型系统集成
- 可配置的模型参数
- 支持聊天补全
- 支持流式响应
- 支持自定义响应解析
- 灵活的模型配置

## 安装

```bash
go get github.com/cloudwego/eino-ext/components/model/deepseek@latest
```

## 快速开始

以下是如何使用 DeepSeek 模型的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY environment variable is not set")
	}
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  apiKey,                          
		Model:    os.Getenv("MODEL_NAME"),           
		BaseURL: "https://api.deepseek.com/beta", 
	

	})
	if err != nil {
		log.Fatal(err)
	}

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful AI assistant. Be concise in your responses.",
		},
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	}

	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	reasoning, ok := deepseek.GetReasoningContent(resp)
	if !ok {
		fmt.Printf("Unexpected: non-reasoning")
	} else {
		fmt.Printf("Reasoning Content: %s \n", reasoning)
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total) \n",
			resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens,
			resp.ResponseMetaUsage.TotalTokens)
	}

}
```

## 配置

可以使用 `deepseek.ChatModelConfig` 结构体配置模型：

```go
type ChatModelConfig struct {
    // APIKey is your authentication key
    // Required
    APIKey string `json:"api_key"`
    
    // Timeout specifies the maximum duration to wait for API responses
    // Optional. Default: 5 minutes
    Timeout time.Duration `json:"timeout"`
    
    // HTTPClient specifies the client to send HTTP requests.
    // Optional. Default http.DefaultClient
    HTTPClient *http.Client `json:"http_client"`
    
    // BaseURL is your custom deepseek endpoint url
    // Optional. Default: https://api.deepseek.com/
    BaseURL string `json:"base_url"`
    
    // Path sets the path for the API request. Defaults to "chat/completions", if not set.
    // Example usages would be "/c/chat/" or any http after the baseURL extension
    // Path 用于设置 API 请求的路径。如果未设置，则默认为 "chat/completions"。
    // 用法示例可以是 "/c/chat/" 或 baseURL 之后的任何 http 路径。
    Path string `json:"path"`
    
    // The following fields correspond to DeepSeek's chat API parameters
    // Ref: https://api-docs.deepseek.com/api/create-chat-completion
    
    // Model specifies the ID of the model to use
    // Required
    Model string `json:"model"`
    
    // MaxTokens limits the maximum number of tokens that can be generated in the chat completion
    // Range: [1, 8192].
    // Optional. Default: 4096
    MaxTokens int `json:"max_tokens,omitempty"`
    
    // Temperature specifies what sampling temperature to use
    // Generally recommend altering this or TopP but not both.
    // Range: [0.0, 2.0]. Higher values make output more random
    // Optional. Default: 1.0
    Temperature float32 `json:"temperature,omitempty"`
    
    // TopP controls diversity via nucleus sampling
    // Generally recommend altering this or Temperature but not both.
    // Range: [0.0, 1.0]. Lower values make output more focused
    // Optional. Default: 1.0
    TopP float32 `json:"top_p,omitempty"`
    
    // Stop sequences where the API will stop generating further tokens
    // Optional. Example: []string{"\n", "User:"}
    Stop []string `json:"stop,omitempty"`
    
    // PresencePenalty prevents repetition by penalizing tokens based on presence
    // Range: [-2.0, 2.0]. Positive values increase likelihood of new topics
    // Optional. Default: 0
    PresencePenalty float32 `json:"presence_penalty,omitempty"`
    
    // ResponseFormat specifies the format of the model's response
    // Optional. Use for structured outputs
    ResponseFormatType ResponseFormatType `json:"response_format_type,omitempty"`
    
    // FrequencyPenalty prevents repetition by penalizing tokens based on frequency
    // Range: [-2.0, 2.0]. Positive values decrease likelihood of repetition
    // Optional. Default: 0
    FrequencyPenalty float32 `json:"frequency_penalty,omitempty"`
    
    // LogProbs specifies whether to return log probabilities of the output tokens.
    LogProbs bool `json:"log_probs"`
    
    // TopLogProbs specifies the number of most likely tokens to return at each token position, each with an associated log probability.
    TopLogProbs int `json:"top_log_probs"`
}
```

## 示例

查看以下示例了解更多用法：

- [基础生成](./examples/generate/)
- [前缀生成](./examples/generate_with_prefix/)
- [意图识别与工具调用](./examples/intent_tool/)
- [流式响应](./examples/stream/)
- [工具调用推理](./examples/tool_call_reasoning/)



## 更多信息

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [DeepSeek Documentation](https://api-docs.deepseek.com/api/create-chat-completion)
