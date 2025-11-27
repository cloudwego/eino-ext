# Volcengine Ark Model

A Volcengine Ark model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

This package provides two distinct models:
- **ChatModel**: For text-based and multi-modal chat completions.
- **ImageGenerationModel**: For generating images from text prompts or image.
- **ResponseAPI**: Contains methods and other services that help with interacting with the openai API.

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


## examples

### generate

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

### generate_with_image

```go

package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
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

	multiModalMsg := schema.UserMessage("")
	image, err := os.ReadFile("./examples/generate_with_image/eino.png")
	if err != nil {
		log.Fatalf("os.ReadFile failed, err=%v\n", err)
	}

	imageStr := base64.StdEncoding.EncodeToString(image)

	multiModalMsg.UserInputMultiContent = []schema.MessageInputPart{
		{
			Type: schema.ChatMessagePartTypeText,
			Text: "What do you see in this image?",
		},
		{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &imageStr,
					MIMEType:   "image/png",
				},
				Detail: schema.ImageURLDetailAuto,
			},
		},
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		multiModalMsg,
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("Ark ChatModel output: \n%v", resp)
}

```

### stream

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v", err)
		return
	}

	streamMsgs, err := chatModel.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})

	if err != nil {
		log.Printf("Generate failed, err=%v", err)
		return
	}

	defer streamMsgs.Close() // do not forget to close the stream

	msgs := make([]*schema.Message, 0)

	log.Printf("typewriter output:")
	for {
		msg, err := streamMsgs.Recv()
		if err == io.EOF {
			break
		}
		msgs = append(msgs, msg)
		if err != nil {
			log.Printf("\nstream.Recv failed, err=%v", err)
			return
		}
		fmt.Print(msg.Content)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Printf("ConcatMessages failed, err=%v", err)
		return
	}

	log.Printf("output: %s\n", msg.Content)
}

```

### intent_tool

```go

package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v", err)
		return
	}

	err = chatModel.BindTools([]*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "Query the user's company and position information based on their name and email",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "The user's name",
					},
					"email": {
						Type: "string",
						Desc: "The user's email",
					},
				}),
		},
		{
			Name: "user_salary",
			Desc: "Query the user's salary information based on their name and email",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "The user's name",
					},
					"email": {
						Type: "string",
						Desc: "The user's email",
					},
				}),
		},
	})
	if err != nil {
		log.Printf("BindForcedTools failed, err=%v", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a real estate agent. Use the user_company and user_salary APIs to provide relevant property information based on the user's salary and job. Email is required",
		},
		{
			Role:    schema.User,
			Content: "My name is zhangsan, and my email is zhangsan@bytedance.com. Please recommend some suitable houses for me.",
		},
	})

	if err != nil {
		log.Printf("Generate failed, err=%v", err)
		return
	}

	log.Printf("output: \n%v", resp)
}

```

### image_generate

```go

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
)

func ptr[T any](v T) *T {
	return &v
}

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	imageGenerationModel, err := ark.NewImageGenerationModel(ctx, &ark.ImageGenerationConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),

		// Control the size of image generated by the model.
		Size: "1K",

		// Control whether to generate a set of images.
		SequentialImageGeneration: ark.SequentialImageGenerationAuto,

		// Control the maximum number of images to generate
		SequentialImageGenerationOption: &model.SequentialImageGenerationOptions{
			MaxImages: ptr(2),
		},

		// Control the format of the generated jpeg image.
		ResponseFormat: ark.ImageResponseFormatURL,

		// Control whether to add a watermark to the generated image
		DisableWatermark: false,
	})

	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	inMsgs := []*schema.Message{
		{
			Role:    schema.User,
			Content: "generate two images of a cat",
		},
	}

	// Use ImageGeneration API
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

### prefixcache-contextapi

```go

package main

import (
	"context"
	"encoding/json"
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

	info, err := chatModel.CreatePrefixCache(ctx, []*schema.Message{
		schema.UserMessage("my name is megumin"),
	}, 3600)
	if err != nil {
		log.Fatalf("CreatePrefix failed, err=%v", err)
	}

	inMsgs := []*schema.Message{
		{
			Role:    schema.User,
			Content: "what is my name?",
		},
	}

	msg, err := chatModel.Generate(ctx, inMsgs, ark.WithCache(&ark.CacheOption{
		APIType:   ark.ContextAPI,
		ContextID: &info.ContextID,
	}))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

	outStreamReader, err := chatModel.Stream(ctx, inMsgs, ark.WithCache(&ark.CacheOption{
		APIType:   ark.ContextAPI,
		ContextID: &info.ContextID,
	}))
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	var msgs []*schema.Message
	for {
		item, e := outStreamReader.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			log.Fatal(e)
		}

		msgs = append(msgs, item)
	}
	msg, err = schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}
	log.Printf("\nstream output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ = json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}

```

### prefixcache-responsesapi

```go

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

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

	chatModelWithTools, err := chatModel.WithTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the current weather in a given location",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "The city and state, e.g. San Francisco, CA",
					},
				},
			),
		},
	})
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
	}

	thinking := &arkModel.Thinking{
		Type: arkModel.ThinkingTypeDisabled,
	}
	cacheOpt := &ark.CacheOption{
		APIType: ark.ResponsesAPI,
		SessionCache: &ark.SessionCacheConfig{
			EnableCache: true,
			TTL:         86400,
		},
	}

	outMsg, err := chatModelWithTools.Generate(ctx, []*schema.Message{
		schema.UserMessage("my name is megumin"),
	}, ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	respID, ok := ark.GetResponseID(outMsg)
	if !ok {
		log.Fatalf("not found response id in message")
	}

	msg, err := chatModelWithTools.Generate(ctx, []*schema.Message{
		schema.UserMessage("what is my name?"),
	}, ark.WithThinking(thinking),
		ark.WithCache(&ark.CacheOption{
			APIType:                ark.ResponsesAPI,
			HeadPreviousResponseID: &respID,
		}),
	)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}

```

### sessioncache-contextapi

```go

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
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

	instructions := []*schema.Message{
		schema.SystemMessage("Your name is superman"),
	}

	cacheInfo, err := chatModel.CreateSessionCache(ctx, instructions, 86400, nil)
	if err != nil {
		log.Fatalf("CreateSessionCache failed, err=%v", err)
	}

	thinking := &arkModel.Thinking{
		Type: arkModel.ThinkingTypeDisabled,
	}

	cacheOpt := &ark.CacheOption{
		APIType:   ark.ContextAPI,
		ContextID: &cacheInfo.ContextID,
		SessionCache: &ark.SessionCacheConfig{
			EnableCache: true,
			TTL:         86400,
		},
	}

	msg, err := chatModel.Generate(ctx, instructions,
		ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	<-time.After(500 * time.Millisecond)

	msg, err = chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What's your name?",
		},
	},
		ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	fmt.Printf("\ngenerate output: \n")
	fmt.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	fmt.Printf("  body: %s\n", string(respBody))

	outStreamReader, err := chatModel.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What do I ask you last time?",
		},
	},
		ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	fmt.Println("\ntypewriter output:")
	var msgs []*schema.Message
	for {
		item, e := outStreamReader.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			log.Fatal(e)
		}

		fmt.Print(item.Content)
		msgs = append(msgs, item)
	}

	msg, err = schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}
	fmt.Print("\n\nstream output: \n")
	fmt.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ = json.MarshalIndent(msg, "  ", "  ")
	fmt.Printf("  body: %s\n", string(respBody))
}

```

### sessioncache-responsesapi

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
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

	thinking := &arkModel.Thinking{
		Type: arkModel.ThinkingTypeDisabled,
	}
	cacheOpt := &ark.CacheOption{
		APIType: ark.ResponsesAPI,
		SessionCache: &ark.SessionCacheConfig{
			EnableCache: true,
			TTL:         86400,
		},
	}

	useMsgs := []*schema.Message{
		schema.UserMessage("Your name is superman"),
		schema.UserMessage("What's your name?"),
		schema.UserMessage("What do I ask you last time?"),
	}

	var input []*schema.Message
	for _, msg := range useMsgs {
		input = append(input, msg)

		streamResp, err := chatModel.Stream(ctx, input,
			ark.WithThinking(thinking),
			ark.WithCache(cacheOpt))
		if err != nil {
			log.Fatalf("Stream failed, err=%v", err)
		}

		var messages []*schema.Message
		for {
			chunk, err := streamResp.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Recv of streamResp failed, err=%v", err)
			}
			messages = append(messages, chunk)
		}

		resp, err := schema.ConcatMessages(messages)
		if err != nil {
			log.Fatalf("ConcatMessages of ark failed, err=%v", err)
		}

		fmt.Printf("stream output: \n%v\n\n", resp)

		input = append(input, resp)
	}
}

```



## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Volcengine Ark Model Documentation](https://www.volcengine.com/docs/82379/1263272)
