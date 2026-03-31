# Claude 模型

一个为 [Eino](https://github.com/cloudwego/eino) 实现的 Claude 模型，它实现了 `ToolCallingChatModel` 接口。这使得能够与 Eino 的 LLM 功能无缝集成，以增强自然语言处理和生成能力。

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
go get github.com/cloudwego/eino-ext/components/model/claude@latest
```

## 快速开始

以下是如何使用 Claude 模型的快速示例：

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/claude"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// Create a Claude model
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
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

	resp, err := cm.Generate(ctx, messages, claude.WithThinking(&claude.Thinking{
		Enable:       true,
		BudgetTokens: 1024,
	}))
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	thinking, ok := claude.GetThinking(resp)
	fmt.Printf("Thinking(have: %v): %s\n", ok, thinking)
	fmt.Printf("Assistant: %s\n", resp.Content)
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total)\n",
			resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens,
			resp.ResponseMeta.Usage.TotalTokens)
	}
}
```

## 配置

可以使用 `claude.ChatModelConfig` 结构体配置模型：

```go
type Config struct {
    // ByBedrock indicates whether to use Bedrock Service
    // Required for Bedrock
    ByBedrock bool
    
    // AccessKey is your Bedrock API Access key
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    AccessKey string
    
    // SecretAccessKey is your Bedrock API Secret Access key
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    SecretAccessKey string
    
    // SessionToken is your Bedrock API Session Token
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    SessionToken string
    
    // Profile is your Bedrock API AWS profile
    // This parameter is ignored if AccessKey and SecretAccessKey are provided
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    Profile string
    
    // Region is your Bedrock API region
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    Region string
    
    // BaseURL is the custom API endpoint URL
    // Use this to specify a different API endpoint, e.g., for proxies or enterprise setups
    // Optional. Example: "https://custom-claude-api.example.com"
    BaseURL *string
    
    // APIKey is your Anthropic API key
    // Obtain from: https://console.anthropic.com/account/keys
    // Required
    APIKey string
    
    // Model specifies which Claude model to use
    // Required
    Model string
    
    // MaxTokens limits the maximum number of tokens in the response
    // Range: 1 to model's context length
    // Required. Example: 2000 for a medium-length response
    MaxTokens int
    
    // Temperature controls randomness in responses
    // Range: [0.0, 1.0], where 0.0 is more focused and 1.0 is more creative
    // Optional. Example: float32(0.7)
    Temperature *float32
    
    // TopP controls diversity via nucleus sampling
    // Range: [0.0, 1.0], where 1.0 disables nucleus sampling
    // Optional. Example: float32(0.95)
    TopP *float32
    
    // TopK controls diversity by limiting the top K tokens to sample from
    // Optional. Example: int32(40)
    TopK *int32
    
    // StopSequences specifies custom stop sequences
    // The model will stop generating when it encounters any of these sequences
    // Optional. Example: []string{"\n\nHuman:", "\n\nAssistant:"}
    StopSequences []string
    
    Thinking *Thinking
    
    // HTTPClient specifies the client to send HTTP requests.
    HTTPClient *http.Client `json:"http_client"`
    
    DisableParallelToolUse *bool `json:"disable_parallel_tool_use"`
}
```

## 示例

查看以下示例了解更多用法：

- [提示缓存](./examples/claude_prompt_cache/)
- [基础生成](./examples/generate/)
- [图像输入](./examples/generate_with_image/)
- [意图识别与工具调用](./examples/intent_tool/)
- [流式响应](./examples/stream/)



## 更多信息

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Claude Documentation](https://docs.claude.com/en/api/messages)
