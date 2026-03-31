# Google Gemini

A Google Gemini implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Custom response parsing support
- Flexible model configuration
- Caching support for generated responses
- Automatic handling of duplicate tool call IDs

## Important Notes

### Tool Call ID Handling

Gemini's API does not provide tool call IDs in its responses. To ensure compatibility with the Eino framework and enable proper tool execution tracking, this implementation automatically generates a unique UUID (v4) for each tool call.

**ID Generation:**
- Each tool call receives a freshly generated UUID
- UUIDs are globally unique across all responses and sessions
- Format: Standard UUID v4 (e.g., `550e8400-e29b-41d4-a716-446655440000`)

**Example:**
```go
// If Gemini returns multiple calls to "get_weather" for different cities:
// Tool Call 1: ID = "550e8400-e29b-41d4-a716-446655440000", Args = {"city": "Paris"}
// Tool Call 2: ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8", Args = {"city": "London"}
// Tool Call 3: ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7", Args = {"city": "Tokyo"}
```

**Benefits:**
- **Session-wide uniqueness**: UUIDs prevent ID collisions across multiple model calls
- **Standard format**: Compatible with industry-standard tool tracking systems
- **Simplified implementation**: No need to maintain state between calls

This ensures that each tool call has a globally unique identifier, which is essential for proper tool execution tracking and response handling in complex agent workflows with multiple model interactions.

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/gemini@latest
```

## Quick start

Here's a quick example of how to use the Gemini model:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/schema"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	baseURL := os.Getenv("GEMINI_BASE_URL")

	ctx := context.Background()
	clientConfig := &genai.ClientConfig{
		APIKey: apiKey,
	}
	// Optional: route requests to a custom Gemini-compatible endpoint.
	if baseURL != "" {
		clientConfig.HTTPOptions = genai.HTTPOptions{
			BaseURL: baseURL,
		}
	}
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  "gemini-2.5-flash",
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	// If you are using a model that supports image understanding (e.g., gemini-2.5-flash-image-preview),
	// you can provide both image and text input like this:
	/*
		image, err := os.ReadFile("./path/to/your/image.jpg")
		if err != nil {
			log.Fatalf("os.ReadFile failed, err=%v\n", err)
		}

		imageStr := base64.StdEncoding.EncodeToString(image)

		resp, err := cm.Generate(ctx, []*schema.Message{
			{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "What do you see in this image?",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{
							MessagePartCommon: schema.MessagePartCommon{
								Base64Data: &imageStr,
								MIMEType:   "image/jpeg",
							},
							Detail: schema.ImageURLDetailAuto,
						},
					},
				},
			},
		})
	*/

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("Assistant: %s\n", resp.Content)
	if len(resp.ReasoningContent) > 0 {
		fmt.Printf("ReasoningContent: %s\n", resp.ReasoningContent)
	}
}
```

Set `GEMINI_BASE_URL` if you need to route request to custom Gemini-compatible endpoint

## Configuration

The model can be configured using the `gemini.Config` struct:

```go
type Config struct {
	// Client is the Gemini API client instance
	// Required for making API calls to Gemini
	Client *genai.Client

	// Model specifies which Gemini model to use
	// Examples: "gemini-pro", "gemini-pro-vision", "gemini-1.5-flash"
	Model string

	// MaxTokens limits the maximum number of tokens in the response
	// Optional. Example: maxTokens := 100
	MaxTokens *int

	// Temperature controls randomness in responses
	// Range: [0.0, 1.0], where 0.0 is more focused and 1.0 is more creative
	// Optional. Example: temperature := float32(0.7)
	Temperature *float32

	// TopP controls diversity via nucleus sampling
	// Range: [0.0, 1.0], where 1.0 disables nucleus sampling
	// Optional. Example: topP := float32(0.95)
	TopP *float32

	// TopK controls diversity by limiting the top K tokens to sample from
	// Optional. Example: topK := int32(40)
	TopK *int32

	// ResponseSchema defines the structure for JSON responses
	// Optional. Used when you want structured output in JSON format
	ResponseSchema *openapi3.Schema

	// EnableCodeExecution allows the model to execute code
	// Warning: Be cautious with code execution in production
	// Optional. Default: false
	EnableCodeExecution bool

	// SafetySettings configures content filtering for different harm categories
	// Controls the model's filtering behavior for potentially harmful content
	// Optional.
	SafetySettings []*genai.SafetySetting

	ThinkingConfig *genai.ThinkingConfig

	// ResponseModalities specifies the modalities the model can return.
	// Optional.
	ResponseModalities []
	
	MediaResolution genai.MediaResolution

	// Cache controls prefix cache settings for the model.
	// Optional. used to CreatePrefixCache for reused inputs.
	Cache *CacheConfig
}

// CacheConfig controls prefix cache settings for the model.
type CacheConfig struct {
	// TTL specifies how long cached resources remain valid (now + TTL).
	TTL time.Duration `json:"ttl,omitempty"`
	// ExpireTime sets the absolute expiration timestamp for cached resources.
	ExpireTime time.Time `json:"expireTime,omitempty"`
}
```

## Caching

This component supports two caching strategies to improve latency and reduce API calls:

- Explicit caching (prefix cache): Build a reusable context from the system instruction, tools, and messages. Use `CreatePrefixCache` to create the cache and pass its name with `gemini.WithCachedContentName(...)` in subsequent requests. Configure TTL and absolute expiry via `CacheConfig` (`TTL`, `ExpireTime`). When a cached content is used, the request omits system instruction and tools and relies on the cached prefix.
- Implicit caching: Managed by Gemini itself. The service may reuse prior requests or responses automatically. Expiry and reuse are controlled by Gemini and cannot be configured.

```
toolInfoList := []*schema.ToolInfo{
	{
		Name:        "tool_a",
		Desc:        "desc",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	},
}
cacheInfo, _ := cm.CreatePrefixCache(ctx, []*schema.Message{
		{
			Role: schema.System,
			Content: `aaa`,
		},
		{
			Role: schema.User,
			Content: `bbb`,
		},
	}, model.WithTools(toolInfoList))


msg, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "give a very short summary about this transcript",
		},
	}, gemini.WithCachedContentName(cacheInfo.Name))
```

The example above shows how to create a prefix cache and reuse it in a follow-up call.

## Examples

See the following examples for more usage:

- [Basic Generation](./examples/generate/)
- [Image Input](./examples/generate_with_image/)
- [Prefix Cache](./examples/generate_with_prefix_cache/)
- [Image Generation](./examples/image_generate/)
- [Intent & Tool Calling](./examples/intent_tool/)
- [ReAct Pattern](./examples/react/)
- [Streaming Response](./examples/stream/)



## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Gemini API Documentation](https://ai.google.dev/api/generate-content?hl=zh-cn#v1beta.GenerateContentResponse)
