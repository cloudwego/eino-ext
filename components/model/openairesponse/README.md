# OpenAI Responses API Model

An [OpenAI Responses API](https://platform.openai.com/docs/api-reference/responses) model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This component uses the `/v1/responses` endpoint (backed by `github.com/openai/openai-go/v3`) and supports tool calling, streaming, multi-modal input, and built-in web search.

## Features

- Implements `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- Supports both `Generate` (non-streaming) and `Stream` (streaming) invocation
- Tool calling via `WithTools` with full `ToolChoice` control (auto / none / required / specific tool)
- Built-in web search tool support (`EnableWebSearch`)
- Multi-modal input: text, image URL, image base64, file URL, file base64
- Response format: `text`, `json_object`, `json_schema`
- Multi-turn conversation with assistant messages and tool result messages
- Fully integrated with Eino's callback system

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/openairesponse@latest
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openairesponse"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	chatModel, err := openairesponse.NewResponsesAPIChatModel(ctx, &openairesponse.ResponsesAPIConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  "gpt-4o",
	})
	if err != nil {
		log.Fatalf("NewResponsesAPIChatModel failed: %v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Hello, who are you?",
		},
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}
	fmt.Println(resp.Content)
}
```

## Streaming

```go
stream, err := chatModel.Stream(ctx, []*schema.Message{
	{Role: schema.User, Content: "Tell me a story."},
})
if err != nil {
	log.Fatalf("Stream failed: %v", err)
}
defer stream.Close()

for {
	chunk, err := stream.Recv()
	if err != nil {
		break
	}
	fmt.Print(chunk.Content)
}
```

## Tool Calling

```go
tools := []*schema.ToolInfo{
	{
		Name: "get_weather",
		Desc: "Get current weather for a city",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {Type: schema.String, Required: true},
		}),
	},
}

modelWithTools, err := chatModel.WithTools(tools)
if err != nil {
	log.Fatalf("WithTools failed: %v", err)
}

resp, err := modelWithTools.Generate(ctx, []*schema.Message{
	{Role: schema.User, Content: "What's the weather in Beijing?"},
})
```

## Built-in Web Search

Enable via config:

```go
chatModel, err := openairesponse.NewResponsesAPIChatModel(ctx, &openairesponse.ResponsesAPIConfig{
	APIKey:          os.Getenv("OPENAI_API_KEY"),
	Model:           "gpt-4o",
	EnableWebSearch: true,
})
```

Or enable per-call via option:

```go
import "github.com/cloudwego/eino/components/model"

resp, err := chatModel.Generate(ctx, messages,
	model.WithImplSpecificOptions(&openairesponse.responsesAPIOptions{
		EnableWebSearch: true,
	}),
)
```

## Multi-modal Input

```go
resp, err := chatModel.Generate(ctx, []*schema.Message{
	{
		Role: schema.User,
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "What's in this image?"},
			{
				Type:     schema.ChatMessagePartTypeImageURL,
				ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image.png"},
			},
		},
	},
})
```

## Configuration

```go
type ResponsesAPIConfig struct {
	// APIKey is the OpenAI API key.
	// Required.
	APIKey string `json:"api_key"`

	// BaseURL overrides the default OpenAI API base URL.
	// Optional. Default: "https://api.openai.com/v1"
	BaseURL string `json:"base_url,omitempty"`

	// Timeout specifies the HTTP client timeout.
	// If HTTPClient is set, Timeout will not be used.
	// Optional. Default: no timeout
	Timeout time.Duration `json:"timeout,omitempty"`

	// HTTPClient specifies a custom HTTP client.
	// If set, Timeout will not be used.
	// Optional.
	HTTPClient *http.Client `json:"-"`

	// Model specifies the ID of the model to use.
	// Required.
	Model string `json:"model"`

	// MaxOutputTokens sets an upper bound on the number of tokens the model may generate.
	// Optional.
	MaxOutputTokens *int `json:"max_output_tokens,omitempty"`

	// ResponseFormat configures the output format (text, json_object, or json_schema).
	// Optional.
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

	// BuiltinTools specifies built-in tools (e.g. file_search) to attach at the model level.
	// Optional.
	BuiltinTools []*schema.ToolInfo `json:"tools,omitempty"`

	// EnableWebSearch enables OpenAI's built-in web_search tool.
	// Optional. Default: false
	EnableWebSearch bool `json:"enable_web_search,omitempty"`
}
```

### Response Format Types

| Constant | Value | Description |
|---|---|---|
| `ChatCompletionResponseFormatTypeText` | `"text"` | Plain text output (default) |
| `ChatCompletionResponseFormatTypeJSONObject` | `"json_object"` | JSON object output |
| `ChatCompletionResponseFormatTypeJSONSchema` | `"json_schema"` | Structured output with JSON Schema |

## Use with Eino ADK (ChatModelAgent)

`ResponsesAPIChatModel` can be used directly as the model for `adk.NewChatModelAgent`.
The same model instance can also be reused inside middlewares (e.g. `summarization`).

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino-ext/components/model/openairesponse"
)

func main() {
	ctx := context.Background()

	maxTokens := 16000
	cm, err := openairesponse.NewResponsesAPIChatModel(ctx, &openairesponse.ResponsesAPIConfig{
		APIKey:          "your-api-key",
		Model:           "gpt-4o",
		BaseURL:         "https://your-custom-endpoint/v1", // optional, override default
		EnableWebSearch: true,
		MaxOutputTokens: &maxTokens,
	})
	if err != nil {
		log.Fatalf("NewResponsesAPIChatModel failed: %v", err)
	}

	// Reuse the same model instance for the summarization middleware
	summaryMw, err := summarization.New(ctx, &summarization.Config{
		Model: cm,
		Trigger: &summarization.TriggerCondition{
			ContextTokens: 100000,
		},
	})
	if err != nil {
		log.Fatalf("summarization.New failed: %v", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "MyAgent",
		Description:   "An agent powered by OpenAI Responses API",
		Model:         cm,
		Handlers:      []adk.ChatModelAgentMiddleware{summaryMw},
		MaxIterations: 30,
	})
	if err != nil {
		log.Fatalf("NewChatModelAgent failed: %v", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})
	_ = runner
	fmt.Println("Agent runner created successfully")
}
```

> **Note**: `ResponsesAPIChatModel` implements `model.ToolCallingChatModel`, which satisfies the model interface required by `adk.ChatModelAgentConfig.Model`. The model instance is safe to share between the agent and middlewares like `summarization`.

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [OpenAI Responses API Reference](https://platform.openai.com/docs/api-reference/responses)
- [openai-go SDK](https://github.com/openai/openai-go)
![1775101436513](image/README/1775101436513.png)