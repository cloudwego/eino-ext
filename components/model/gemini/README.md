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

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
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

## examples

### generate

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
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  modelName,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

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

### generate_with_image

```go

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/schema"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  modelName,
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	image, err := os.ReadFile("./examples/generate_with_image/test.jpg")
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
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
}

```

### generate_with_prefix_cache

```go

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
)

func main() {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Fatalf("genai.NewClient failed: %v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Model:  os.Getenv("GEMINI_MODEL"),
		Client: client,
	})
	if err != nil {
		log.Fatalf("gemini.NewChatModel failed: %v", err)
	}

	type toolCallInput struct {
		Answer int `json:"answer" jsonschema_description:"the answer of the question"`
	}
	answerTool, err := utils.InferTool("answer_to_user",
		"answer to user",
		func(ctx context.Context, in *toolCallInput) (string, error) {
			return fmt.Sprintf("answer: %v", in.Answer), nil
		})
	if err != nil {
		log.Fatalf("utils.InferTool failed: %v", err)
	}

	info, err := answerTool.Info(ctx)
	if err != nil {
		log.Fatalf("get tool info failed: %v", err)
	}

	// this file is from gemini cache usage example
	fileData, err := os.ReadFile("./a11.test.txt")
	if err != nil {
		log.Fatalf("os.ReadFile failed: %v", err)
	}

	txtFileBase64 := base64.StdEncoding.EncodeToString(fileData)
	cacheInfo, err := cm.CreatePrefixCache(ctx, []*schema.Message{
		{
			Role: schema.System,
			Content: `You are an expert at analyzing transcripts.
answer the question with the tool "answer_to_user"
always include the start_time and end_time of the transcript in the output`,
		},
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeFileURL,
					File: &schema.MessageInputFile{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &txtFileBase64,
							MIMEType:   "text/plain",
						},
					},
				},
			},
		},
	}, model.WithTools([]*schema.ToolInfo{info}), model.WithToolChoice(schema.ToolChoiceForced))
	if err != nil {
		log.Fatalf("CreatePrefixCache failed: %v", err)
	}

	data, _ := sonic.MarshalIndent(cacheInfo, "", "  ")
	log.Printf("prefix cache info:\n%v\n", string(data))

	msg, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "give a very short summary about this transcript",
		},
	}, gemini.WithCachedContentName(cacheInfo.Name),
		model.WithTools([]*schema.ToolInfo{info}),
		model.WithToolChoice(schema.ToolChoiceForced))
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}
	msgData, _ := sonic.MarshalIndent(msg, "", "  ")
	log.Printf("model output:\n%v\n", string(msgData))
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

	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/schema"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  modelName,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
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
	"log"
	"os"

	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/schema"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  modelName,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}
	err = cm.BindTools([]*schema.ToolInfo{
		{
			Name: "book_recommender",
			Desc: "Recommends books based on user preferences and provides purchase links",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"genre": {
					Type: "string",
					Desc: "Preferred book genre",
					Enum: []string{"fiction", "sci-fi", "mystery", "biography", "business"},
				},
				"max_pages": {
					Type: "integer",
					Desc: "Maximum page length (0 for no limit)",
				},
				"min_rating": {
					Type: "number",
					Desc: "Minimum user rating (0-5 scale)",
				},
			}),
		},
	})
	if err != nil {
		log.Fatalf("Bind tools error: %v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Recommend business books with minimum 4.3 rating and max 350 pages",
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Function called: \n")
		if len(resp.ReasoningContent) > 0 {
			fmt.Printf("ReasoningContent: %s\n", resp.ReasoningContent)
		}
		fmt.Println("Name: ", resp.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.ToolCalls[0].Function.Arguments)
	} else {
		log.Printf("Function called without tool calls: %s\n", resp.Content)
	}

	resp, err = cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Recommend business books with minimum 4.3 rating and max 350 pages",
		},
		resp,
		{
			Role:       schema.Tool,
			ToolCallID: resp.ToolCalls[0].ID,
			Content:    "{\"book name\":\"Microeconomics for Managers\"}",
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	fmt.Printf("Function call final result: %s\n", resp.Content)
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

	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/schema"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  modelName,
		// you can set the necessary parameters for image generation
		ImageConfig: &genai.ImageConfig{
			AspectRatio: "16:9",
			ImageSize:   "1K",
		},
		ResponseModalities: []gemini.GeminiResponseModality{
			gemini.GeminiResponseModalityText,
			gemini.GeminiResponseModalityImage,
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

### react

```go

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
)

func main() {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
	})
	if err != nil {
		log.Fatalf("genai.NewClient failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Model:  os.Getenv("GEMINI_MODEL"),
		Client: client,
	})
	if err != nil {
		log.Fatalf("gemini.NewChatModel failed, err=%v", err)
	}

	type toolCallInput struct {
		LastCount int `json:"last_count" jsonschema_description:"the last count"`
	}
	countsTool, err := utils.InferTool("count_tool_call",
		"count the number of tool calls",
		func(ctx context.Context, in *toolCallInput) (string, error) {
			counts := in.LastCount + 1
			return fmt.Sprintf("tool call counts: %v", counts), nil
		})
	if err != nil {
		log.Fatalf("utils.InferTool failed, err=%v", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "react_agent",
		Description: "react_agent",
		Instruction: `call count_tool_call 5 times, then say 'done'`,
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{
					countsTool,
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("adk.NewChatModelAgent failed, err=%v", err)
	}

	iter := agent.Run(ctx, &adk.AgentInput{
		Messages: []adk.Message{
			{
				Role:    schema.User,
				Content: "start to count",
			},
		},
	})
	idx := 0
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Fatalf("agent.Run failed, err=%v", event.Err)
		}

		msg, err_ := event.Output.MessageOutput.GetMessage()
		if err_ != nil {
			log.Fatalf("GetMessage failed, err=%v", err_)
		}

		idx++
		msgData, _ := sonic.MarshalIndent(msg, "", "  ")
		log.Printf("\nmessage %v:\n%v\n", idx, string(msgData))
	}
}

```



## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Gemini API Documentation](https://ai.google.dev/api/generate-content?hl=zh-cn#v1beta.GenerateContentResponse)
