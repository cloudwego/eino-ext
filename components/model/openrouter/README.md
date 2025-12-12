# OpenRouter 

A OpenRouter implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

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
go get github.com/cloudwego/eino-ext/components/model/openrouter@latest
```

## Quick start

Here's a quick example of how to use the OpenRouter model:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"


	"github.com/cloudwego/eino/schema"
	
	"github.com/cloudwego/eino-ext/components/model/openrouter"
	
)

func main() {
	
	ctx := context.Background()
	cm, err := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey:  os.Getenv("API_KEY"),
		Model:   os.Getenv("MODEL"), // model should support image generate
		BaseURL: os.Getenv("BASE_URL"),
		Reasoning: &openrouter.Reasoning{
			Effort: openrouter.EffortOfMedium,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}


	// If you are using a model that supports image understanding (e.g., google/gemini-2.5-flash-image-preview),
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

## Configuration

The model can be configured using the `openrouter.Config` struct:

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
    
    // BaseURL specifies the OpenRouter endpoint URL
    // Optional. Default: https://openrouter.ai/api/v1
    BaseURL string `json:"base_url"`
    
    // Model specifies the ID of the model to use.
    // Optional.
    Model string `json:"model,omitempty"`
    
    // Models parameter lets you automatically try other models if the primary model’s providers are down,
    // rate-limited, or refuse to reply due to content moderation.
    // Optional.
    Models []string `json:"models,omitempty"`

    // MaxCompletionTokens represents the total number of tokens in the model's output, including both the final output and any tokens generated during the thinking process.
    MaxCompletionTokens *int `json:"max_completion_tokens,omitempty"`
    
    // MaxTokens represents only the final output of the model, excluding any tokens from the thinking process.
    MaxTokens *int `json:"max_tokens,omitempty"`
    
    // Seed enables deterministic sampling for consistent outputs.
    // Optional. Set for reproducible results
    Seed *int `json:"seed,omitempty"`
    
    // Stop sequences where the API will stop generating further tokens.
    // Optional. Example: []string{"\n", "User:"}
    Stop []string `json:"stop,omitempty"`
    
    // TopP controls diversity via nucleus sampling
    // Generally recommend altering this or Temperature but not both.
    // Range: 0.0 to 1.0. Lower values make output more focused
    // Optional. Default: 1.0
    TopP *float32 `json:"top_p,omitempty"`
    
    // Temperature specifies what sampling temperature to use.
    // Generally recommend altering this or TopP but not both.
    // Range: 0.0 to 2.0. Higher values make output more random.
    // Optional. Default: 1.0
    Temperature *float32 `json:"temperature,omitempty"`
    
    // ResponseFormat specifies the format of the model's response.
    // Optional. Use for structured outputs
    ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitempty"`
    
    // PresencePenalty prevents repetition by penalizing tokens based on presence.
    // Range: -2.0 to 2.0. Positive values increase likelihood of new topics.
    // Optional. Default: 0
    PresencePenalty *float32 `json:"presence_penalty,omitempty"`
    
    // FrequencyPenalty prevents repetition by penalizing tokens based on frequency.
    // Range: -2.0 to 2.0. Positive values decrease likelihood of repetition
    // Optional. Default: 0
    FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
    
    // LogitBias modifies likelihood of specific tokens appearing in completion.
    // Optional. Map token IDs to bias values from -100 to 100
    LogitBias map[string]int `json:"logit_bias,omitempty"`
    
    // LogProbs specifies whether to return log probabilities of the output tokens.
    LogProbs bool `json:"log_probs"`
    
    // TopLogProbs is an integer between 0 and 20 specifying the number of most likely tokens to return at each
    // token position, each with an associated log probability.
    // logprobs must be set to true if this parameter is used.
    TopLogProbs int `json:"top_logprobs"`
    
    // Reasoning supports advanced reasoning capabilities, allowing models to show their internal reasoning process with configurable effort、summary fields levels
    Reasoning *Reasoning `json:"reasoning,omitempty"`
    
    // User unique identifier representing end-user
    // Optional.
    User *string `json:"user,omitempty"`
    
    // Metadata Set of 16 key-value pairs that can be attached to an object.
    // This can be useful for storing additional information about the object in a structured format, and querying for objects via API or the dashboard.
    // Keys are strings with a maximum length of 64 characters. Values are strings with a maximum length of 512 characters.
    Metadata map[string]string `json:"metadata,omitempty"`
    
    // ExtraFields will override any existing fields with the same key.
    // Optional. Useful for experimental features not yet officially supported.
    ExtraFields map[string]any `json:"extra_fields,omitempty"`
}


    
type ChatCompletionResponseFormatType string
    
const (
    ChatCompletionResponseFormatTypeJSONObject ChatCompletionResponseFormatType = "json_object"
    ChatCompletionResponseFormatTypeJSONSchema ChatCompletionResponseFormatType = "json_schema"
    ChatCompletionResponseFormatTypeText       ChatCompletionResponseFormatType = "text"
)
    
type ChatCompletionResponseFormat struct {
    Type       ChatCompletionResponseFormatType        `json:"type,omitempty"`
    JSONSchema *ChatCompletionResponseFormatJSONSchema `json:"json_schema,omitempty"`
}
    
type ChatCompletionResponseFormatJSONSchema struct {
    Name        string             `json:"name"`
    Description string             `json:"description,omitempty"`
    JSONSchema  *jsonschema.Schema `json:"schema"`
    Strict      bool               `json:"strict"`
}
    
type Effort string
    
const (
    EffortOfNone    Effort = "none"
    EffortOfMinimal Effort = "minimal"
    EffortOfLow     Effort = "low"
    EffortOfMedium  Effort = "medium"
    EffortOfHigh    Effort = "high"
)
    
type Summary string
    
const (
    SummaryOfAuto     Summary = "auto"
    SummaryOfConcise  Summary = "concise"
    SummaryOfDetailed Summary = "detailed"
)


// Reasoning configures reasoning capabilities across different models.
// See documentation for each field to understand model support and behavior differences.
// Reference: https://openrouter.ai/docs/guides/best-practices/reasoning-tokens
type Reasoning struct {
    // Effort controls the reasoning strength level.
    Effort Effort `json:"effort,omitempty"`
    // Summary specifies whether and how reasoning should be summarized.
    Summary Summary `json:"summary,omitempty"`
    
    // MaxTokens directly specifies the maximum tokens to allocate for reasoning.
    // For models that only support effort-based reasoning, this value determines
    // the appropriate effort level. See: https://openrouter.ai/docs/guides/best-practices/reasoning-tokens
    MaxTokens int `json:"maxTokens,omitempty"`
    
    // Exclude indicates whether reasoning should occur internally but not appear
    // in the response. When true, reasoning tokens appear in the "reasoning"
    // field of each message.
    Exclude bool `json:"exclude,omitempty"`
    
    // Enabled explicitly enables or disables reasoning capabilities.
    Enabled *bool `json:"enabled,omitempty"`
}

```
## examples
### generate

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openrouter"
)

func main() {
	ctx := context.Background()

	chatModel, err := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey:  os.Getenv("API_KEY"),
		Model:   os.Getenv("MODEL"),
		BaseURL: os.Getenv("BASE_URL"),
		Reasoning: &openrouter.Reasoning{
			Effort: openrouter.EffortOfMedium,
		},
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

### generate_with_image

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openrouter"
)

func main() {
	ctx := context.Background()
	chatModel, err := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey:  os.Getenv("API_KEY"),
		Model:   os.Getenv("MODEL"), // model support image generate example: google/gemini-2.5-flash-image
		BaseURL: os.Getenv("BASE_URL"),
		Reasoning: &openrouter.Reasoning{
			Effort: openrouter.EffortOfMedium,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)

	}

	multiModalMsg := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{

			{
				Type: schema.ChatMessagePartTypeText,
				Text: "this picture is a landscape photo, what's the picture's content",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL: of("https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcT11qEDxU4X_MVKYQVU5qiAVFidA58f8GG0bQ&s"),
					},
					Detail: schema.ImageURLDetailAuto,
				},
			},
		},
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		multiModalMsg,
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)
}

func of[T any](a T) *T {
	return &a
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

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openrouter"
)

func main() {

	ctx := context.Background()

	cm, err := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey:  os.Getenv("API_KEY"),
		Model:   os.Getenv("MODEL"),
		BaseURL: os.Getenv("BASE_URL"),
		Reasoning: &openrouter.Reasoning{
			Effort: openrouter.EffortOfMedium,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}
	stream, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Write a short poem about spring.",
		},
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	fmt.Println("Assistant: ")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream receive error: %v", err)
		}

		fmt.Println("frame: ")
		if len(resp.Content) > 0 {
			fmt.Println("content: ", resp.Content)
		}
		if len(resp.ReasoningContent) > 0 {
			fmt.Printf("ReasoningContent: %s\n", resp.ReasoningContent)
		}
	}
	fmt.Println()
}

```

### intent_tool

```go

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openrouter"
)

package main

import (
"context"
"fmt"
"io"
"log"
"os"

"github.com/cloudwego/eino/schema"

"github.com/cloudwego/eino-ext/components/model/openrouter"
)

func main() {
	ctx := context.Background()
	chatModel, err := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey:  os.Getenv("API_KEY"),
		Model:   os.Getenv("MODEL"),
		BaseURL: os.Getenv("BASE_URL"),
		Reasoning: &openrouter.Reasoning{
			Effort: openrouter.EffortOfMedium,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of openai failed, err=%v", err)
	}
	cm, err := chatModel.WithTools([]*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "Retrieve the user's company and position based on their name and email.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name":  {Type: "string", Desc: "user's name"},
					"email": {Type: "string", Desc: "user's email"}}),
		}, {
			Name: "user_salary",
			Desc: "Retrieve the user's salary based on their name and email.\n",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name":  {Type: "string", Desc: "user's name"},
					"email": {Type: "string", Desc: "user's email"},
				}),
		}})
	if err != nil {
		log.Fatalf("BindForcedTools of openai failed, err=%v", err)
	}
	resp, err := cm.Generate(ctx, []*schema.Message{{
		Role:    schema.System,
		Content: "As a real estate agent, provide relevant property information based on the user's salary and job using the user_company and user_salary APIs. An email address is required.",
	}, {
		Role:    schema.User,
		Content: "My name is John and my email is john@abc.com，Please recommend some houses that suit me.",
	}})
	if err != nil {
		log.Fatalf("Generate of openai failed, err=%v", err)
	}
	fmt.Printf("output: \n%v", resp)

	streamResp, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "As a real estate agent, provide relevant property information based on the user's salary and job using the user_company and user_salary APIs. An email address is required.",
		}, {
			Role:    schema.User,
			Content: "My name is John and my email is john@abc.com，Please recommend some houses that suit me.",
		},
	})
	if err != nil {
		log.Fatalf("Stream of openai failed, err=%v", err)
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
	resp, err = schema.ConcatMessages(messages)
	if err != nil {
		log.Fatalf("ConcatMessages of openai failed, err=%v", err)
	}
	fmt.Printf("stream output: \n%v", resp)
}


```

### image_generate

```go

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/openrouter"
)

func main() {
	ctx := context.Background()
	cm, err := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey:  os.Getenv("API_KEY"),
		Model:   os.Getenv("MODEL"), // model should support image generate
		BaseURL: os.Getenv("BASE_URL"),
		Reasoning: &openrouter.Reasoning{
			Effort: openrouter.EffortOfMedium,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	/*
		The generated multimodal content is stored in the `AssistantGenMultiContent` field.
		For this example, the resulting message will have a structure similar to this:

		resp := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageOutputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &base64String, // The base64 encoded image data
							MIMEType:   "image/png",
						},
					},
				},
			},
		}
	*/
	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "Generate an image of a cat",
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	log.Printf("\ngenerate output: \n")
	respBody, _ := json.MarshalIndent(resp, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

}

```



## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [OpenRouter API Documentation](https://openrouter.ai/docs/api/reference/overview)
