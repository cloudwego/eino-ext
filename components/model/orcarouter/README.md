# OrcaRouter

An [OrcaRouter](https://www.orcarouter.ai) implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

OrcaRouter is an AI gateway that serves OpenAI-compatible (`/v1/chat/completions`), Anthropic-compatible (`/v1/messages`) and embedding endpoints from one base URL, with namespaced model ids such as `anthropic/claude-sonnet-5` and `anthropic/claude-haiku-4.5`. It also runs gateway-level, zero-trust security for AI agents on the same endpoint — screening every prompt/response and governing every tool call on a default-deny basis, with no application code changes.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Support for tool calling
- Thinking-model `reasoning_content` passthrough for Anthropic models served by OrcaRouter

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/orcarouter@latest
```

## Quick start

Here's a quick example of how to use the OrcaRouter model:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/orcarouter"
)

func main() {
	ctx := context.Background()

	chatModel, err := orcarouter.NewChatModel(ctx, &orcarouter.Config{
		APIKey: os.Getenv("ORCAROUTER_API_KEY"),
		Model:  os.Getenv("ORCAROUTER_MODEL"), // e.g. anthropic/claude-sonnet-5
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

## Configuration

The model can be configured using the `orcarouter.Config` struct:

```go
type Config struct {
    APIKey string
    // Timeout specifies the maximum duration to wait for API responses.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default: no timeout
    Timeout time.Duration `json:"timeout"`

    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // BaseURL specifies the OrcaRouter endpoint URL.
    // Optional. Default: https://api.orcarouter.ai/v1
    BaseURL string `json:"base_url"`

    // Model specifies the ID of the model to use.
    // OrcaRouter serves namespaced model ids (e.g. anthropic/claude-sonnet-5,
    // anthropic/claude-haiku-4.5) over one OpenAI-compatible endpoint.
    // Optional.
    Model string `json:"model,omitempty"`

    // MaxTokens represents the maximum number of tokens that can be generated in the chat completion.
    // Optional. Default: model's maximum
    MaxTokens *int `json:"max_tokens,omitempty"`

    // MaxCompletionTokens represents the total number of tokens in the model's output,
    // including both the final output and any tokens generated during the thinking process.
    MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`

    // Temperature specifies what sampling temperature to use.
    // Generally recommend altering this or TopP but not both.
    // Range: 0.0 to 2.0. Higher values make output more random.
    // Optional. Default: 1.0
    Temperature *float32 `json:"temperature,omitempty"`

    // TopP controls diversity via nucleus sampling.
    // Generally recommend altering this or Temperature but not both.
    // Range: 0.0 to 1.0. Lower values make output more focused.
    // Optional. Default: 1.0
    TopP *float32 `json:"top_p,omitempty"`

    // Stop sequences where the API will stop generating further tokens.
    // Optional. Example: []string{"\n", "User:"}
    Stop []string `json:"stop,omitempty"`

    // PresencePenalty prevents repetition by penalizing tokens based on presence.
    // Range: -2.0 to 2.0. Positive values increase likelihood of new topics.
    // Optional. Default: 0
    PresencePenalty *float32 `json:"presence_penalty,omitempty"`

    // FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
    // Range: -2.0 to 2.0. Positive values decrease likelihood of repetition.
    // Optional. Default: 0
    FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

    // Seed enables deterministic sampling for consistent outputs.
    // Optional. Set for reproducible results.
    Seed *int `json:"seed,omitempty"`

    // User unique identifier representing end-user.
    // Optional.
    User *string `json:"user,omitempty"`

    // LogitBias modifies the likelihood of specified tokens appearing in the completion.
    // Optional. Map token IDs to bias values from -100 to 100.
    LogitBias map[string]int `json:"logit_bias,omitempty"`

    // LogProbs specifies whether to return log probabilities of the output tokens.
    LogProbs bool `json:"log_probs"`

    // TopLogProbs is an integer between 0 and 20 specifying the number of most likely tokens
    // to return at each token position, each with an associated log probability.
    // logprobs must be set to true if this parameter is used.
    TopLogProbs int `json:"top_logprobs"`

    // ResponseFormat specifies the format of the model's response.
    // Optional. Use for structured outputs.
    ResponseFormat *openai.ChatCompletionResponseFormat `json:"response_format,omitempty"`

    // ExtraFields will override any existing fields with the same key.
    // Optional. Useful for experimental features not yet officially supported.
    ExtraFields map[string]any `json:"extra_fields,omitempty"`
}
```

## Examples

See the following examples for more usage:

- [Basic Chat Completion](./examples/chat/)



## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [OrcaRouter](https://www.orcarouter.ai)
