# DeepSeek Model

A DeepSeek model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Model` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Custom response parsing support
- Flexible model configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/deepseek@latest
```

## Quick Start

Here's a quick example of how to use the DeepSeek model:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY environment variable is not set")
	}

	// 创建 deepseek 模型
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:    apiKey,
		Model:     "deepseek-reasoner",
		MaxTokens: 2000,
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
		fmt.Printf("Resoning Content: %s\n", reasoning)
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total)\n",
			resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens,
			resp.ResponseMeta.Usage.TotalTokens)
	}
}
```

## Configuration

The model can be configured using the `deepseek.ChatModelConfig` struct:

```go
type ChatModelConfig struct {
// APIKey is your authentication key
// Required
APIKey string `json:"api_key"`

// Timeout specifies the maximum duration to wait for API responses
// Optional. Default: 5 minutes
Timeout time.Duration `json:"timeout"`

// BaseURL is your custom deepseek endpoint url
// Optional. Default: https://api.deepseek.com/
BaseURL string `json:"base_url"`

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
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [DeepSeek Documentation](https://api-docs.deepseek.com/api/create-chat-completion)
