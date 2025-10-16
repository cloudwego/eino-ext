# Claude Model

A Claude model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Model` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

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
go get github.com/cloudwego/eino-ext/components/model/claude@latest
```

## Quick Start

Here's a quick example of how to use the Claude model:

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

## Configuration

The model can be configured using the `claude.ChatModelConfig` struct:

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




## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Claude Documentation](https://docs.claude.com/en/api/messages)
