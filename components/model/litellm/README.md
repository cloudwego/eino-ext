# LiteLLM ChatModel

A ChatModel component for the [Eino](https://github.com/cloudwego/eino) framework that connects to a [LiteLLM](https://github.com/BerriAI/litellm) proxy.

LiteLLM is an AI gateway that provides a unified OpenAI-compatible API to 100+ LLM providers including OpenAI, Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, Mistral, Cohere, and more.

## Prerequisites

A running LiteLLM proxy server. See [LiteLLM Proxy Quick Start](https://docs.litellm.ai/docs/proxy/quick_start).

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/model/litellm"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	chatModel, err := litellm.NewChatModel(ctx, &litellm.Config{
		BaseURL: "http://localhost:4000",
		APIKey:  "sk-your-litellm-key",
		Model:   "openai/gpt-4o",
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: "What is 2+2?"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Content)
}
```

## Streaming

```go
stream, err := chatModel.Stream(ctx, []*schema.Message{
	{Role: schema.User, Content: "Tell me a joke"},
})
if err != nil {
	log.Fatal(err)
}
defer stream.Close()

for {
	msg, err := stream.Recv()
	if err != nil {
		break
	}
	fmt.Print(msg.Content)
}
```

## Tool Calling

```go
modelWithTools, err := chatModel.WithTools([]*schema.ToolInfo{
	{
		Name: "get_weather",
		Desc: "Get current weather for a city",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {Type: "string", Desc: "City name"},
		}),
	},
})
```

## LiteLLM-Specific Features

Pass LiteLLM-specific parameters via `ExtraFields`:

```go
chatModel, err := litellm.NewChatModel(ctx, &litellm.Config{
	BaseURL: "http://localhost:4000",
	APIKey:  "sk-your-litellm-key",
	Model:   "anthropic/claude-sonnet-4-20250514",
	ExtraFields: map[string]any{
		"drop_params": true,
		"metadata":    map[string]string{"team": "engineering"},
	},
})
```

Or per-request via options:

```go
resp, err := chatModel.Generate(ctx, messages,
	litellm.WithExtraFields(map[string]any{"drop_params": true}),
	litellm.WithExtraHeader(map[string]string{"X-LiteLLM-Tag": "prod"}),
)
```

## Model Naming

LiteLLM uses a `provider/model` naming convention:

| Provider | Example Model |
|---|---|
| OpenAI | `openai/gpt-4o` |
| Anthropic | `anthropic/claude-sonnet-4-20250514` |
| Google Gemini | `gemini/gemini-2.5-flash` |
| AWS Bedrock | `bedrock/anthropic.claude-3-haiku-20240307-v1:0` |
| Azure OpenAI | `azure/gpt-4o` |
| Mistral | `mistral/mistral-large-latest` |

See the full list at [LiteLLM Providers](https://docs.litellm.ai/docs/providers).
