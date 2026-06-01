# Volcengine Ark Model

A Volcengine Ark model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

This package provides two distinct models:
- **ChatModel**: For text-based and multi-modal chat completions.
- **ImageGenerationModel**: For generating images from text prompts or image.
- **ResponsesAPIChatModel**: Contains methods and other services that help with interacting with ResponsesAPI.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for both chat completion, image generation and response api
- Support for streaming responses
- Custom response parsing support
- Flexible model configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/ark@latest
```

---

## Chat Completion

This model is used for standard chat and text generation tasks.

### Quick Start

Here's a quick example of how to use the `ChatModel`:

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

func main() {
	ctx := context.Background()

	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
	})

	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	inMsgs := []*schema.Message{
		{
			Role:    schema.User,
			Content: "how do you generate answer for user question as a machine, please answer in short?",
		},
	}

	msg, err := chatModel.Generate(ctx, inMsgs)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

	sr, err := chatModel.Stream(ctx, inMsgs)
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	chunks := make([]*schema.Message, 0, 1024)
	for {
		msgChunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatalf("Stream Recv failed, err=%v", err)
		}

		chunks = append(chunks, msgChunk)
	}

	msg, err = schema.ConcatMessages(chunks)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	log.Printf("stream final output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ = json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```

### Configuration

The `ChatModel` can be configured using the `ark.ChatModelConfig` struct:

```go
type ChatModelConfig struct {
    // Timeout specifies the maximum duration to wait for API responses
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default: 10 minutes
    Timeout *time.Duration `json:"timeout"`
    
    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`
    
    // RetryTimes specifies the number of retry attempts for failed API calls
    // Optional. Default: 2
    RetryTimes *int `json:"retry_times"`
    
    // BaseURL specifies the base URL for Ark service
    // Optional. Default: "https://ark.cn-beijing.volces.com/api/v3"
    BaseURL string `json:"base_url"`
    
    // Region specifies the region where Ark service is located
    // Optional. Default: "cn-beijing"
    Region string `json:"region"`
    
    // The following three fields are about authentication - either APIKey or AccessKey/SecretKey pair is required
    // For authentication details, see: https://www.volcengine.com/docs/82379/1298459
    // APIKey takes precedence if both are provided
    APIKey string `json:"api_key"`
    
    AccessKey string `json:"access_key"`
    
    SecretKey string `json:"secret_key"`
    
    // The following fields correspond to Ark's chat completion API parameters
    // Ref: https://www.volcengine.com/docs/82379/1298454
    
    // Model specifies the ID of endpoint on ark platform
    // Required
    Model string `json:"model"`
    
    // MaxTokens limits the maximum number of output tokens in the Chat Completion API.
    // In Responses API, corresponds to `max_output_tokens`, representing both the model output and reasoning output.
    // Optional. In chat completion, default: 4096, in Responses API, no default.
    MaxTokens *int `json:"max_tokens,omitempty"`
    
    // MaxCompletionTokens specifies the maximum tokens in the Chat Completion API,
    // representing both the model output and reasoning output.
    // Range: 0 to 65,536 tokens. Exceeding the maximum threshold will result in an error.
    // Note: In chat completion, MaxCompletionTokens and MaxTokens cannot both be set, in Responses API, this field is ignored; use MaxTokens.
    // Optional.
    MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`
    
    // Temperature specifies what sampling temperature to use
    // Generally recommend altering this or TopP but not both
    // Range: 0.0 to 1.0. Higher values make output more random
    // Optional. Default: 1.0
    Temperature *float32 `json:"temperature,omitempty"`
    
    // TopP controls diversity via nucleus sampling
    // Generally recommend altering this or Temperature but not both
    // Range: 0.0 to 1.0. Lower values make output more focused
    // Optional. Default: 0.7
    TopP *float32 `json:"top_p,omitempty"`
    
    // Stop sequences where the API will stop generating further tokens
    // Optional. Example: []string{"\n", "User:"}
    Stop []string `json:"stop,omitempty"`
    
    // FrequencyPenalty prevents repetition by penalizing tokens based on frequency
    // Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
    // Optional. Default: 0
    FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
    
    // LogitBias modifies likelihood of specific tokens appearing in completion
    // Optional. Map token IDs to bias values from -100 to 100
    LogitBias map[string]int `json:"logit_bias,omitempty"`
    
    // PresencePenalty prevents repetition by penalizing tokens based on presence
    // Range: -2.0 to 2.0. Positive values increase likelihood of new topics
    // Optional. Default: 0
    PresencePenalty *float32 `json:"presence_penalty,omitempty"`
    
    // CustomHeader the http header passed to model when requesting model
    CustomHeader map[string]string `json:"custom_header"`
    
    // LogProbs specifies whether to return log probabilities of the output tokens.
    LogProbs bool `json:"log_probs"`
    
    // TopLogProbs specifies the number of most likely tokens to return at each token position, each with an associated log probability.
    TopLogProbs int `json:"top_log_probs"`
    
    // ResponseFormat specifies the format that the model must output.
    ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
    
    // Thinking controls whether the model is set to activate the deep thinking mode.
    // It is set to be enabled by default.
    Thinking *model.Thinking `json:"thinking,omitempty"`
    
    // ServiceTier specifies whether to use the TPM guarantee package. The effective target has purchased the inference access point for the guarantee package.
    ServiceTier *string `json:"service_tier"`
    
    // ReasoningEffort specifies the reasoning effort of the model.
    // Optional.
    ReasoningEffort *model.ReasoningEffort `json:"reasoning_effort,omitempty"`
    
    // BatchChat ark batch chat config
    // Optional.
    BatchChat *BatchChatConfig `json:"batch_chat,omitempty"`
    
    Cache *CacheConfig `json:"cache,omitempty"`
}
```

### Request Options

The `ChatModel` supports various request options to customize the behavior of API calls. Here are the available options:

```go
// WithCustomHeader sets custom headers for a single request
// the headers will override all the headers given in ChatModelConfig.CustomHeader
func WithCustomHeader(m map[string]string) model.Option {}
```

---

## Image Generation

This model is used specifically for generating images from text prompts.

### Quick Start

Here's a quick example of how to use the `ImageGenerationModel`:

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/ark"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and an image generation model ID
	imageGenerationModel, err := ark.NewImageGenerationModel(ctx, &ark.ImageGenerationConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_IMAGE_MODEL_ID"), // Use an appropriate image model ID
	})

	if err != nil {
		log.Fatalf("NewImageGenerationModel failed, err=%v", err)
	}

	inMsgs := []*schema.Message{
		{
			Role:    schema.User,
			Content: "a photo of a cat sitting on a table",
		},
	}

	msg, err := imageGenerationModel.Generate(ctx, inMsgs)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("\ngenerate output: \n")
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

	sr, err := imageGenerationModel.Stream(ctx, inMsgs)
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	log.Printf("stream output: \n")
	index := 0
	for {
		msgChunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatalf("Stream Recv failed, err=%v", err)
		}

		respBody, _ = json.MarshalIndent(msgChunk, "  ", "  ")
		log.Printf("stream chunk %d: body: %s\n", index, string(respBody))
		index++
	}
}
```

### Configuration

The `ImageGenerationModel` can be configured using the `ark.ImageGenerationConfig` struct:

```go

type ImageGenerationConfig struct {
    // For authentication, APIKey is required as the image generation API only supports API Key authentication.
    // For authentication details, see: https://www.volcengine.com/docs/82379/1298459
    // Required
    APIKey string `json:"api_key"`
    
    // Model specifies the ID of endpoint on ark platform
    // Required
    Model string `json:"model"`
    
    // Timeout specifies the maximum duration to wait for API responses
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default: 10 minutes
    Timeout *time.Duration `json:"timeout"`
    
    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`
    
    // RetryTimes specifies the number of retry attempts for failed API calls
    // Optional. Default: 2
    RetryTimes *int `json:"retry_times"`
    
    // BaseURL specifies the base URL for Ark service
    // Optional. Default: "https://ark.cn-beijing.volces.com/api/v3"
    BaseURL string `json:"base_url"`
    
    // Region specifies the region where Ark service is located
    // Optional. Default: "cn-beijing"
    Region string `json:"region"`
    
    // The following fields correspond to Ark's image generation API parameters
    // Ref: https://www.volcengine.com/docs/82379/1541523
    
    // Size specifies the dimensions of the generated image.
    // It can be a resolution keyword (e.g., "1K", "2K", "4K") or a custom resolution
    // in "{width}x{height}" format (e.g., "1920x1080").
    // When using custom resolutions, the total pixels must be between 1280x720 and 4096x4096,
    // and the aspect ratio (width/height) must be between 1/16 and 16.
    // Optional. Defaults to "2048x2048".
    Size string `json:"size"`
    
    // SequentialImageGeneration determines if the model should generate a sequence of images.
    // Possible values:
    //  - "auto": The model decides whether to generate multiple images based on the prompt.
    //  - "disabled": Only a single image is generated.
    // Optional. Defaults to "disabled".
    SequentialImageGeneration SequentialImageGeneration `json:"sequential_image_generation"`
    
    // SequentialImageGenerationOption sets the maximum number of images to generate when
    // SequentialImageGeneration is set to "auto".
    // The value must be between 1 and 15.
    // Optional. Defaults to 15.
    SequentialImageGenerationOption *model.SequentialImageGenerationOptions `json:"sequential_image_generation_option"`
    
    // ResponseFormat specifies how the generated image data is returned.
    // Possible values:
    //  - "url": A temporary URL to download the image (valid for 24 hours).
    //  - "b64_json": The image data encoded as a Base64 string in the response.
    // Optional. Defaults to "url".
    ResponseFormat ImageResponseFormat `json:"response_format"`
    
    // DisableWatermark, if set to true, removes the "AI Generated" watermark
    // from the bottom-right corner of the image.
    // Optional. Defaults to false.
    DisableWatermark bool `json:"disable_watermark"`

    // 	BatchMaxParallel specifies the maximum number of parallel requests to send to the chat completion API.
    //	Optional. Default: 3000.
    BatchMaxParallel *int `json:"batch_max_parallel,omitempty"`

    // BatchChat ark batch chat config
    // Optional.
    BatchChat *BatchChatConfig `json:"batch_chat,omitempty"`

    Cache *CacheConfig `json:"cache,omitempty"`
}
```

## Examples

See the following examples for more usage:

- [Basic Generation](./examples/generate/)
- [Batch Chat](./examples/generate_batch_chat/)
- [Image Input](./examples/generate_with_image/)
- [Image Generation](./examples/image_generate/)
- [Intent & Tool Calling](./examples/intent_tool/)
- [Prefix Caching](./examples/prefixcache/)
- [Session Caching](./examples/sessioncache/)
- [Streaming Response](./examples/stream/)

### Chat Completion Examples

- **[generate](./examples/generate)** - Basic text generation with both Generate() and Stream() methods
  - Shows how to use the chat model for standard text generation
  - Demonstrates both non-streaming and streaming responses
  - Includes request ID tracking

- **[generate_with_image](./examples/generate_with_image)** - Multi-modal chat with image input
  - Demonstrates processing images alongside text
  - Shows how to encode and send images in base64 format
  - Example of using ChatTemplate with images

- **[stream](./examples/stream)** - Streaming response with reasoning effort
  - Shows typewriter-style streaming output
  - Demonstrates using reasoning effort options
  - Example of proper stream handling and cleanup

- **[intent_tool](./examples/intent_tool)** - Tool calling and function execution
  - Demonstrates function/tool calling capabilities
  - Shows how to define and register tools
  - Example of handling tool calls and responses

- **[generate_batch_chat](./examples/generate_batch_chat)** - Batch chat completions
  - Shows how to process multiple chat conversations in batch
  - Demonstrates batch configuration options
  - Example of handling batch results

### Caching Examples

- **[prefixcache/contextapi](./examples/prefixcache/contextapi)** - Prefix caching with Context API
  - Demonstrates prefix caching to improve performance
  - Shows cache hit/miss tracking
  - Context API integration example

- **[prefixcache/responsesapi](./examples/prefixcache/responsesapi)** - Prefix caching with Responses API
  - Same as above but using Responses API
  - Shows different API integration approach

- **[sessioncache/contextapi](./examples/sessioncache/contextapi)** - Session caching with Context API
  - Demonstrates session-level caching
  - Shows multi-turn conversation caching

- **[sessioncache/responsesapi](./examples/sessioncache/responsesapi)** - Session caching with Responses API
  - Same as above but using Responses API

### Image Generation Example

- **[image_generate](./examples/image_generate)** - Text-to-image generation
  - Demonstrates using the ImageGenerationModel
  - Shows how to configure image generation parameters
  - Example of saving generated images

## Quick Example

Here's a minimal example to get started (for complete examples, see above):

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
	})

	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	inMsgs := []*schema.Message{
		{
			Role:    schema.User,
			Content: "how do you generate answer for user question as a machine, please answer in short?",
		},
	}

	msg, err := chatModel.Generate(ctx, inMsgs)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

	sr, err := chatModel.Stream(ctx, inMsgs)
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	chunks := make([]*schema.Message, 0, 1024)
	for {
		msgChunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Fatalf("Stream Recv failed, err=%v", err)
		}

		chunks = append(chunks, msgChunk)
	}

	msg, err = schema.ConcatMessages(chunks)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	log.Printf("stream final output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ = json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}

```


- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Volcengine Ark Model Documentation](https://www.volcengine.com/docs/82379/1263272)
