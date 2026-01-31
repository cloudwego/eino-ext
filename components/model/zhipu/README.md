# Zhipu AI GLM Model

A Zhipu AI GLM model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Support for function calling (tool use)
- Support for thinking mode
- Support for vision understanding (multimodal models)
- Flexible model configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/zhipu@latest
```

## Quick Start

Here's a quick example of how to use the Zhipu AI GLM model:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/cloudwego/eino-ext/components/model/zhipu"
    "github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("Hello, please introduce Zhipu AI."),
	})
	if err != nil {
		log.Fatalf("Generate of zhipu failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)

}

func of[T any](t T) *T {
	return &t
}
```

## Configuration

The model can be configured using the `zhipu.ChatModelConfig` struct:

```go
type ChatModelConfig struct {

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

// BaseURL specifies the Zhipu AI endpoint URL
// Required. Example: https://open.bigmodel.cn/api/paas/v4/
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
// Range: 0.0 to 1.0. Higher values make output more random
// Optional. Default: 0.6
Temperature *float32 `json:"temperature,omitempty"`

// TopP controls diversity via nucleus sampling
// Generally recommend altering this or Temperature but not both.
// Range: 0.0 to 1.0. Lower values make output more focused
// Optional. Default: 0.95
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
// Optional. Helps monitor and detect abuse
User *string `json:"user,omitempty"`

// Thinking mode configuration
// Optional. For complex reasoning problems
Thinking *Thinking `json:"thinking,omitempty"`
}

```

## Examples

### Generate

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("Hello, please introduce Zhipu AI."),
	})
	if err != nil {
		log.Fatalf("Generate of zhipu failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)

}

func of[T any](t T) *T {
	return &t
}

```

### Stream

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	cm, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	sr, err := cm.Stream(ctx, []*schema.Message{
		schema.UserMessage("Hello"),
	})
	if err != nil {
		log.Fatalf("Stream of zhipu failed, err=%v", err)
	}

	var msgs []*schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatalf("Stream of zhipu failed, err=%v", err)
		}

		fmt.Println(msg)
		// assistant: Hello
		// finish_reason:
		// : !
		// finish_reason:
		// : How
		// finish_reason:
		// : can I
		// finish_reason:
		// : help you?
		// finish_reason:
		// :
		// finish_reason: stop
		// usage: &{9 7 16}
		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	fmt.Println(msg)
	// assistant: Hello! How can I help you?
	// finish_reason: stop
	// usage: &{9 7 16}
}

func of[T any](t T) *T {
	return &t
}

```

### Thinking Mode

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	temp := float32(0.9)

	cm, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       "glm-4.7",
		Temperature: &temp,
		Thinking: &zhipu.Thinking{
			Type: zhipu.ThinkingEnabled,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	sr, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "you are a helpful assistant",
		},
		{
			Role:    schema.User,
			Content: "what is the revolution of llm?",
		},
	})
	if err != nil {
		log.Fatalf("Stream of zhipu failed, err=%v", err)
	}

	var msgs []*schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatalf("Stream of zhipu failed, err=%v", err)
		}

		// Reasoning content
		if msg.ReasoningContent != "" {
			fmt.Printf("Reasoning: %s\n", msg.ReasoningContent)
		}

		// Answer content
		if msg.Content != "" {
			fmt.Printf("Answer: %s\n", msg.Content)
		}

		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	fmt.Println("\nFull response:")
	fmt.Printf("Reasoning: %s\n", msg.ReasoningContent)
	fmt.Printf("Answer: %s\n", msg.Content)
}

```

### Tool Calling

```go

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	cm, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	err = cm.BindTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get weather information for a specified city",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "City name, e.g., Beijing, Shanghai",
					},
				}),
		},
	})
	if err != nil {
		log.Fatalf("BindTools of zhipu failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What's the weather like in Beijing today?",
		},
	})

	if err != nil {
		log.Fatalf("Generate of zhipu failed, err=%v", err)
	}

	fmt.Println(resp)
	// assistant:
	// tool_calls: [{0x14000275930 call_xxx function {get_weather {"location": "Beijing"}} map[]}]
	// finish_reason: tool_calls
	// usage: &{100 20 120}

	// ==========================
	// using stream
	fmt.Printf("\n\n======== Stream ========\n")
	sr, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What's the weather like in Shanghai today?",
		},
	})
	if err != nil {
		log.Fatalf("Stream of zhipu failed, err=%v", err)
	}

	msgs := make([]*schema.Message, 0)
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			log.Fatalf("json.Marshal failed, err=%v", err)
		}
		fmt.Printf("%s\n", jsonMsg)
		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("json.Marshal failed, err=%v", err)
	}
	fmt.Printf("final: %s\n", jsonMsg)
}

func of[T any](t T) *T {
	return &t
}

```



## For More Details
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Zhipu AI Documentation](https://docs.bigmodel.cn)
