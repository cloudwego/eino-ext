# OpenAI Model

A OpenAI model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

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
go get github.com/cloudwego/eino-ext/components/model/openai@latest
```

## Quick Start

Here's a quick example of how to use the OpenAI model:

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		// if you want to use Azure OpenAI Service, set these two field.
		// BaseURL: "https://{RESOURCE_NAME}.openai.azure.com",
		// ByAzure: true,
		// APIVersion: "2024-06-01",
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		ByAzure: func() bool {
			if os.Getenv("OPENAI_BY_AZURE") == "true" {
				return true
			}
			return false
		}(),
		ReasoningEffort: openai.ReasoningEffortLevelHigh,
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}
	fmt.Printf("output: \n%v", resp)

}


```

## Responses API with schema.Message

`NewChatModel` uses Chat Completions. Use `NewResponsesChatModel` when an existing
`schema.Message` / `ToolCallingChatModel` workflow needs `/v1/responses` without
migrating its messages and middleware to `schema.AgenticMessage`.

```go
cm, err := openai.NewResponsesChatModel(ctx, &openai.ResponsesChatModelConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model: os.Getenv("OPENAI_MODEL"),
    BaseURL: os.Getenv("OPENAI_BASE_URL"), // optional
    ReasoningEffort: openai.ResponsesReasoningEffortHigh,
})
if err != nil {
    return err
}
history := []*schema.Message{schema.UserMessage("Explain binary search.")}
answer, err := cm.Generate(ctx, history)
if err != nil {
    return err
}
history = append(history, answer, schema.UserMessage("What are its edge cases?"))
stream, err := cm.Stream(ctx, history)
if err != nil {
    return err
}
followup, err := schema.ConcatMessageStream(stream)
```

The Responses model supports text and image inputs, text output, custom function
calling (`WithTools` / `BindTools`), tool-choice options, callbacks, streaming,
usage accounting, and encrypted reasoning carried through `Message.Extra`.
Retain the returned message, including `Extra`, when saving conversation history.
Normal user/assistant messages and tool results still use the standard Eino
message constructors. Provider-hosted tools and arbitrary native Responses
content blocks are outside this message-oriented adapter; use `agenticopenai`
for the `AgenticMessage` interface.

Use `model.WithMaxTokens`, `model.WithTemperature`, and other common model options
as usual. Responses-specific per-call overrides are `WithResponsesReasoningEffort`,
`WithResponsesStore`, `WithResponsesPromptCacheKey`, and
`WithResponsesPromptCacheRetention`; the existing Chat Completions-specific
options do not configure the Responses model.

By default, the complete message history is sent and the provider decides whether
to store responses. `WithResponsesUseResponseID(true)` instead continues from the
latest assistant response ID in `Extra`; use it only when the provider stores and
supports retrieving that response (for example with `WithResponsesStore(true)`).
SDK retries are disabled so application or ADK retry policies control retries.

### Provider-specific stream events

Set `ResponsesChatModelConfig.StreamEventHandler` to inspect SSE events, including
extension fields available through `event.RawJSON()`. The handler returns a
`ResponsesStreamEventResult`:

- `Error` terminates the stream immediately and reaches the reader unchanged,
  preserving `errors.Is` / `errors.As` for retry and failover policies.
- `IncompleteError` supplies an error only if the stream ends without a terminal
  response. A terminal response or native SDK/transport error takes precedence.
  This is useful for quota notices that can accompany otherwise successful work.
- A zero result leaves normal event processing unchanged. A nil handler preserves
  the default behavior.

For example, an application can classify its gateway's events:

```go
StreamEventHandler: func(ctx context.Context, event responses.ResponseStreamEventUnion) openai.ResponsesStreamEventResult {
    switch event.Type {
    case "gateway.upstream_error":
        return openai.ResponsesStreamEventResult{Error: errGatewayUpstream}
    case "gateway.quota_notice":
        return openai.ResponsesStreamEventResult{IncompleteError: errGatewayQuota}
    }
    return openai.ResponsesStreamEventResult{}
},
```

Here `responses` is `github.com/openai/openai-go/v3/responses`; the error values
are application-owned sentinels. In practice, inspect the extension payload to
determine whether the event indicates a failure. Events are processed sequentially
within each stream, but the same handler can run concurrently for different
requests. Incomplete-stream errors are kept separately for each stream by the
adapter; handlers do not need shared mutable state for this purpose.

The Responses implementation uses the official OpenAI Go SDK and requires Go 1.22
or later. See [the runnable example](examples/responses/responses.go).

## Configuration

The model can be configured using the `openai.ChatModelConfig` struct:

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

// The following three fields are only required when using Azure OpenAI Service, otherwise they can be ignored.
// For more details, see: https://learn.microsoft.com/en-us/azure/ai-services/openai/

// ByAzure indicates whether to use Azure OpenAI Service
// Required for Azure
ByAzure bool `json:"by_azure"`

// AzureModelMapperFunc is used to map the model name to the deployment name for Azure OpenAI Service.
// This is useful when the model name is different from the deployment name.
// Optional for Azure, remove [,:] from the model name by default.
AzureModelMapperFunc func(model string) string

// BaseURL is the Azure OpenAI endpoint URL
// Format: https://{YOUR_RESOURCE_NAME}.openai.azure.com. YOUR_RESOURCE_NAME is the name of your resource that you have created on Azure.
// Required for Azure
BaseURL string `json:"base_url"`

// APIVersion specifies the Azure OpenAI API version
// Required for Azure
APIVersion string `json:"api_version"`

// The following fields correspond to OpenAI's chat completion API parameters
// Ref: https://platform.openai.com/docs/api-reference/chat/create

// Model specifies the ID of the model to use
// Required
Model string `json:"model"`

// MaxTokens limits the maximum number of tokens that can be generated in the chat completion
// Optional. Default: model's maximum
// Deprecated: use MaxCompletionTokens. Not compatible with o1-series models.
// refs: https://platform.openai.com/docs/api-reference/chat/create#chat-create-max_tokens
MaxTokens *int `json:"max_tokens,omitempty"`

// MaxCompletionTokens specifies an upper bound for the number of tokens that can be generated for a completion, including visible output tokens and reasoning tokens.
MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

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
ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`

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

// ExtraFields will override any existing fields with the same key.
// Optional. Useful for experimental features not yet officially supported.
ExtraFields map[string]any `json:"extra_fields,omitempty"`

// ReasoningEffort will override the default reasoning level of "medium"
// Optional. Useful for fine tuning response latency vs. accuracy
ReasoningEffort ReasoningEffortLevel

// Modalities are output types that you would like the model to generate. Most models are capable of generating text, which is the default: ["text"]
// The gpt-4o-audio-preview model can also be used to generate audio. To request that this model generate both text and audio responses, you can use: ["text", "audio"]
Modalities []Modality `json:"modalities,omitempty"`

// Audio parameters for audio output. Required when audio output is requested with modalities: ["audio"]
Audio *Audio `json:"audio,omitempty"`
}
```


## Examples

See the following examples for more usage:

- [Audio Generation](./examples/audio_generate/)
- [Basic Generation](./examples/generate/)
- [Image Input](./examples/generate_with_image/)
- [Intent & Tool Calling](./examples/intent_tool/)
- [Streaming Response](./examples/stream/)
- [Structured Output](./examples/structured/)
- [Timeout Handling](./examples/timeout/)



## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [OpenAI Documentation](https://platform.openai.com/docs/api-reference/chat/create)
