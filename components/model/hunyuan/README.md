# Hunyuan Model

A Tencent Cloud Hunyuan model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Custom response parsing support
- Flexible model configuration
- Support for multimodal content (images, videos)
- Tool calling (Function Calling) support
- Reasoning content support
- Complete error handling
- Callback mechanism support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/hunyuan@latest
```

## Quick Start

Here's a quick example of how to use the Hunyuan model:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/hunyuan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	
	// Get authentication from environment variables
	secretId := os.Getenv("HUNYUAN_SECRET_ID")
	secretKey := os.Getenv("HUNYUAN_SECRET_KEY")
	
	if secretId == "" || secretKey == "" {
		log.Fatal("HUNYUAN_SECRET_ID and HUNYUAN_SECRET_KEY environment variables are required")
	}

	// Create Hunyuan chat model
	cm, err := hunyuan.NewChatModel(ctx, &hunyuan.ChatModelConfig{
		SecretId:  secretId,
		SecretKey: secretKey,
		Model:     "hunyuan-lite", // Options: hunyuan-lite, hunyuan-pro, hunyuan-turbo
		Timeout:   30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Build conversation messages
	messages := []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Please answer user questions in Chinese."),
		schema.UserMessage("Please introduce the features and advantages of Tencent Cloud Hunyuan large model."),
	}

	// Call model to generate response
	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Assistant: %s\n", resp.Content)
	
	if resp.ReasoningContent != "" {
		fmt.Printf("\nReasoning: %s\n", resp.ReasoningContent)
	}
	
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		usage := resp.ResponseMeta.Usage
		fmt.Printf("\nToken Usage:\n")
		fmt.Printf("  Prompt Tokens: %d\n", usage.PromptTokens)
		fmt.Printf("  Completion Tokens: %d\n", usage.CompletionTokens)
		fmt.Printf("  Total Tokens: %d\n", usage.TotalTokens)
	}
}
```

## Configuration

The model can be configured using the `hunyuan.ChatModelConfig` struct:

```go
type ChatModelConfig struct {
	// SecretId is your Tencent Cloud API Secret ID
	// Required
	SecretId string

	// SecretKey is your Tencent Cloud API Secret Key
	// Required
	SecretKey string

	// Model specifies the ID of the model to use
	// Required. Options: hunyuan-lite, hunyuan-pro, hunyuan-turbo
	Model string

	// Region specifies the service region
	// Optional. Default: ap-guangzhou
	Region string

	// Timeout specifies the maximum duration to wait for API responses
	// Optional. Default: 30 seconds
	Timeout time.Duration

	// Temperature controls the randomness of the output
	// Range: [0.0, 2.0]. Higher values make output more random
	// Optional. Default: 1.0
	Temperature float32

	// TopP controls diversity via nucleus sampling
	// Range: [0.0, 1.0]. Lower values make output more focused
	// Optional. Default: 1.0
	TopP float32

	// Stop sequences where the API will stop generating further tokens
	// Optional
	Stop []string

	// PresencePenalty prevents repetition by penalizing tokens based on presence
	// Range: [-2.0, 2.0]. Positive values increase likelihood of new topics
	// Optional. Default: 0
	PresencePenalty float32

	// FrequencyPenalty prevents repetition by penalizing tokens based on frequency
	// Range: [-2.0, 2.0]. Positive values decrease likelihood of repetition
	// Optional. Default: 0
	FrequencyPenalty float32
}
```

## Examples

### Basic Text Generation

Reference: [examples/generate/generate.go](./examples/generate/generate.go)

```bash
cd examples/generate
go run generate.go
```

### Streaming Response

Reference: [examples/stream/stream.go](./examples/stream/stream.go)

```bash
cd examples/stream
go run stream.go
```

### Tool Calling

Reference: [examples/tool_call/tool_call.go](./examples/tool_call/tool_call.go)

```bash
cd examples/tool_call
go run tool_call.go
```

### Multimodal Processing

Reference: [examples/multimodal/multimodal.go](./examples/multimodal/multimodal.go)

```bash
cd examples/multimodal
go run multimodal.go
```

## Tool Calling

### Defining Tools

```go
tools := []*schema.ToolInfo{
	{
		Name: "get_weather",
		Desc: "Query city weather information",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {
				Type: schema.String,
				Desc: "City name",
			},
		}),
	},
}

cmWithTools, err := cm.WithTools(tools)
```

### Handling Tool Calls

```go
resp, err := cmWithTools.Generate(ctx, messages)
if len(resp.ToolCalls) > 0 {
	for _, toolCall := range resp.ToolCalls {
		switch toolCall.Function.Name {
		case "get_weather":
			// Execute tool call
			result, _ := getWeather(city)
			
			// Add result to conversation
			messages = append(messages, &schema.Message{
				Role:       schema.Tool,
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}
	}
	
	// Continue conversation
	finalResp, err := cmWithTools.Generate(ctx, messages)
}
```

## Multimodal Content Processing

### Image Processing

```go
messages := []*schema.Message{
	{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "Please describe the content of this image:",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						MIMEType: "image/jpeg",
						Base64Data: toPtr("base64-image-data"),
					},
				},
			},
		},
	},
}
```

### Video Processing

```go
messages := []*schema.Message{
	{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeVideoURL,
				Video: &schema.MessageInputVideo{
					MessagePartCommon: schema.MessagePartCommon{
						MIMEType: "video/mp4",
						Base64Data: toPtr("base64-video-data"),
					},
				},
			},
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "Please analyze the main content of this video.",
			},
		},
	},
}
```

## Environment Variables

| Variable Name | Description | Required |
|---------------|-------------|----------|
| HUNYUAN_SECRET_ID | Tencent Cloud API Secret ID | Yes |
| HUNYUAN_SECRET_KEY | Tencent Cloud API Secret Key | Yes |

## Testing

Run unit tests:

```bash
go test -v ./...
```

## Related Links

- [Tencent Cloud Hunyuan Documentation](https://cloud.tencent.com/document/product/1729)
- [CloudWeGo Eino Framework](https://github.com/cloudwego/eino)