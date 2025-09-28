# Volcengine Ark Model

A Volcengine Ark model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Model` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

This package provides two distinct models:
- **ChatModel**: For text-based and multi-modal chat completions.
- **ImageGenerationModel**: For generating images from text prompts or image.

## Features

- Implements `github.com/cloudwego/eino/components/model.Model`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for both chat completion and image generation
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

	log.Printf("generate output: \n")
	log.Printf("  request_id: %s\n")
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
	log.Printf("  request_id: %s\n")
	respBody, _ = json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
```

### Configuration

The `ChatModel` can be configured using the `ark.ChatModelConfig` struct:

```go
type ChatModelConfig struct {
    // Timeout specifies the maximum duration to wait for API responses
    // Optional. Default: 10 minutes
    Timeout *time.Duration `json:"timeout"`
    
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
    APIKey    string `json:"api_key"`
    AccessKey string `json:"access_key"`
    SecretKey string `json:"secret_key"`
    
    // The following fields correspond to Ark's chat completion API parameters
    // Ref: https://www.volcengine.com/docs/82379/1298454
    
    // Model specifies the ID of endpoint on ark platform
    // Required
    Model string `json:"model"`
    
    // MaxTokens limits the maximum number of tokens that can be generated in the chat completion and the range of values is [0, 4096]
    // Optional. Default: 4096
    MaxTokens *int `json:"max_tokens,omitempty"`
    
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
    // --- Authentication and basic connection settings ---
    // (Timeout, HTTPClient, RetryTimes, BaseURL, Region, APIKey)
    // ...

    // --- Image Generation Specific Parameters ---
    // Ref: https://www.volcengine.com/docs/82379/1541523

    // Model specifies the ID of the image generation endpoint on the Ark platform.
    // Required.
    Model string `json:"model"`

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
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Volcengine Ark Model Documentation](https://www.volcengine.com/docs/82379/1263272)
