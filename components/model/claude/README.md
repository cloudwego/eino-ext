# Claude Model

A Claude model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

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
go get github.com/cloudwego/eino-ext/components/model/claude@latest
```

## Quick Start

Here's a quick example of how to use the Claude model:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/claude"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// Create a Claude model
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful AI assistant. Be concise in your responses.",
		},
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	}

	resp, err := cm.Generate(ctx, messages, claude.WithThinking(&claude.Thinking{
		Enable:       true,
		BudgetTokens: 1024,
	}))
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	thinking, ok := claude.GetThinking(resp)
	fmt.Printf("Thinking(have: %v): %s\n", ok, thinking)
	fmt.Printf("Assistant: %s\n", resp.Content)
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total)\n",
			resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens,
			resp.ResponseMeta.Usage.TotalTokens)
	}
}

```

## Configuration

The model can be configured using the `claude.ChatModelConfig` struct:

```go
type Config struct {
    // ByBedrock indicates whether to use Bedrock Service
    // Required for Bedrock
    ByBedrock bool
    
    // AccessKey is your Bedrock API Access key
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    AccessKey string
    
    // SecretAccessKey is your Bedrock API Secret Access key
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    SecretAccessKey string
    
    // SessionToken is your Bedrock API Session Token
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    SessionToken string
    
    // Profile is your Bedrock API AWS profile
    // This parameter is ignored if AccessKey and SecretAccessKey are provided
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    Profile string
    
    // Region is your Bedrock API region
    // Obtain from: https://docs.aws.amazon.com/bedrock/latest/userguide/getting-started.html
    // Optional for Bedrock
    Region string
    
    // BaseURL is the custom API endpoint URL
    // Use this to specify a different API endpoint, e.g., for proxies or enterprise setups
    // Optional. Example: "https://custom-claude-api.example.com"
    BaseURL *string
    
    // APIKey is your Anthropic API key
    // Obtain from: https://console.anthropic.com/account/keys
    // Required
    APIKey string
    
    // Model specifies which Claude model to use
    // Required
    Model string
    
    // MaxTokens limits the maximum number of tokens in the response
    // Range: 1 to model's context length
    // Required. Example: 2000 for a medium-length response
    MaxTokens int
    
    // Temperature controls randomness in responses
    // Range: [0.0, 1.0], where 0.0 is more focused and 1.0 is more creative
    // Optional. Example: float32(0.7)
    Temperature *float32
    
    // TopP controls diversity via nucleus sampling
    // Range: [0.0, 1.0], where 1.0 disables nucleus sampling
    // Optional. Example: float32(0.95)
    TopP *float32
    
    // TopK controls diversity by limiting the top K tokens to sample from
    // Optional. Example: int32(40)
    TopK *int32
    
    // StopSequences specifies custom stop sequences
    // The model will stop generating when it encounters any of these sequences
    // Optional. Example: []string{"\n\nHuman:", "\n\nAssistant:"}
    StopSequences []string
    
    Thinking *Thinking
    
    // HTTPClient specifies the client to send HTTP requests.
    HTTPClient *http.Client `json:"http_client"`
    
    DisableParallelToolUse *bool `json:"disable_parallel_tool_use"`
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

	"github.com/cloudwego/eino-ext/components/model/claude"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// Create a Claude model
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful AI assistant. Be concise in your responses.",
		},
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	}

	resp, err := cm.Generate(ctx, messages, claude.WithThinking(&claude.Thinking{
		Enable:       true,
		BudgetTokens: 1024,
	}))
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	thinking, ok := claude.GetThinking(resp)
	fmt.Printf("Thinking(have: %v): %s\n", ok, thinking)
	fmt.Printf("Assistant: %s\n", resp.Content)
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total)\n",
			resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens,
			resp.ResponseMeta.Usage.TotalTokens)
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

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/claude"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// Create a Claude model
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	imageBinary, err := os.ReadFile("examples/test.jpg")
	if err != nil {
		log.Fatalf("read file failed, err=%v", err)
	}
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
							Base64Data: of(base64.StdEncoding.EncodeToString(imageBinary)),
							MIMEType:   "image/jpeg",
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
}

func of[T any](v T) *T {
	return &v
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

	"github.com/cloudwego/eino-ext/components/model/claude"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// Create a Claude model
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Be concise in your responses."),
		{
			Role:    schema.User,
			Content: "Write a short poem about spring, word by word.",
		},
	}

	stream, err := cm.Stream(ctx, messages, claude.WithThinking(&claude.Thinking{
		Enable:       true,
		BudgetTokens: 1024,
	}))
	if err != nil {
		log.Printf("Stream error: %v", err)
		return
	}
	isFirstThinking := false
	isFirstContent := false

	fmt.Print("Assistant: ----------\n")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Stream receive error: %v", err)
			return
		}

		thinkingContent, ok := claude.GetThinking(resp)
		if ok {
			if !isFirstThinking {
				isFirstThinking = true
				fmt.Print("\nThinking: ----------\n")
			}
			fmt.Print(thinkingContent)
		}

		if len(resp.Content) > 0 {
			if !isFirstContent {
				isFirstContent = true
				fmt.Print("\nContent: ----------\n")
			}
			fmt.Print(resp.Content)
		}
	}
	fmt.Println("\n----------")
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

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// Create a Claude model
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	_, err = cm.WithTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get current weather information for a city",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "city",
						Value: &jsonschema.Schema{
							Type:        "string",
							Description: "The city name",
						},
					},
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "unit",
						Value: &jsonschema.Schema{
							Type: "string",
							Enum: []interface{}{"celsius", "fahrenheit"},
						},
					},
				)),
				Required: []string{"city"},
			}),
		},
	})
	if err != nil {
		log.Printf("Bind tools error: %v", err)
		return
	}

	streamResp, err := cm.Stream(ctx, []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Be concise in your responses."),
		schema.UserMessage("call 'get_weather' to query what's the weather like in Paris today? Please use Celsius."),
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	msgs := make([]*schema.Message, 0)
	for {
		msg, err := streamResp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream receive error: %v", err)
		}
		msgs = append(msgs, msg)
	}
	resp, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("Concat error: %v", err)
	}

	fmt.Printf("assistant content:\n  %v\n----------\n", resp.Content)
	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Function called: %s\n", resp.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.ToolCalls[0].Function.Arguments)

		weatherResp, err := cm.Generate(ctx, []*schema.Message{
			schema.UserMessage("What's the weather like in Paris today? Please use Celsius."),
			resp,
			schema.ToolMessage(`{"temperature": 18, "condition": "sunny"}`, resp.ToolCalls[0].ID),
		})
		if err != nil {
			log.Printf("Generate error: %v", err)
			return
		}
		fmt.Printf("Final response: %s\n", weatherResp.Content)
	}
}

```

### claude_prompt_cache

```go

package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	systemCache()
	//toolInfoCache()
	//sessionCache()
}

func systemCache() {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")

	ctx := context.Background()

	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   &baseURL,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	breakpoint := claude.SetMessageBreakpoint(&schema.Message{
		Role:    schema.System,
		Content: "The film Dongji Rescue, based on a true historical event, tells the story of Chinese fishermen braving turbulent seas to save strangers — a testament to the nation's capacity to create miracles in the face of adversity.\n\nEighty-three years ago, in 1942, the Japanese military seized the Lisbon Maru ship to transport 1,816 British prisoners of war from Hong Kong to Japan. Passing through the waters near Zhoushan, Zhejiang province, it was torpedoed by a US submarine. Fishermen from Zhoushan rescued 384 prisoners of war and hid them from Japanese search parties.\n\nActor Zhu Yilong, in an exclusive interview with China Daily, said he was deeply moved by the humanity shown in such extreme conditions when he accepted his role. \"This historical event proves that in dire circumstances, Chinese people can extend their goodness to protect both themselves and others,\" he said.\n\nLast Friday, a themed event for Dongji Rescue was held in Zhoushan, a city close to where the Lisbon Maru sank. Descendants of the British POWs and the rescuing fishermen gathered to watch the film and share their reflections.\n\nIn the film, the British POWs are aided by a group of fishermen from Dongji Island, whose courage and compassion cut a path through treacherous waves. After the screening, many descendants were visibly moved.\n\n\"I want to express my deepest gratitude to the Chinese people and the descendants of Chinese fishermen. When the film is released in the UK, I will bring my family and friends to watch it. Heroic acts like this deserve to be known worldwide,\" said Denise Wynne, a descendant of a British POW.\n\n\"I felt the profound friendship between the Chinese and British people through the film,\" said Li Hui, a descendant of a rescuer. Many audience members were brought to tears — some by the fishermen's bravery, others by the enduring spirit of \"never abandoning those in peril\".\n\n\"In times of sea peril, rescue is a must,\" said Wu Buwei, another rescuer's descendant, noting that this value has long been a part of Dongji fishermen's traditions.\n\nCoincidentally, on the morning of Aug 5, a real-life rescue unfolded on the shores of Dongfushan in Dongji town. Two tourists from Shanghai fell into the sea and were swept away by powerful waves. Without hesitation, two descendants of Dongji fishermen — Wang Yubin and Yang Xiaoping — leaped in to save them, braving a wind force of 9 to 10 before pulling them to safety.\n\n\"Our forebearers once rescued many British POWs here. As their descendants, we should carry forward their bravery,\" Wang said. Both rescuers later joined the film's cast and crew at the Zhoushan event, sharing the stage with actors who portrayed the fishermen in a symbolic \"reunion across 83 years\".\n\nChinese actor Wu Lei, who plays A Dang in the film, expressed his respect for the two rescuing fishermen on-site. In the film, A Dang saves a British POW named Thomas Newman. Though they share no common language, they manage to connect. In an interview with China Daily, Wu said, \"They connected through a small globe. A Dang understood when Newman pointed out his home, the UK, on the globe.\"\n\nThe film's director Fei Zhenxiang noted that the Eastern Front in World War II is often overlooked internationally, despite China's 14 years of resistance tying down large numbers of Japanese troops. \"Every Chinese person is a hero, and their righteousness should not be forgotten,\" he said.\n\nGuan Hu, codirector, said, \"We hope that through this film, silent kindness can be seen by the world\", marking the 80th anniversary of victory in the Chinese People's War of Resistance Against Japanese Aggression (1931-45) and the World Anti-Fascist War.",
	})

	for i := 0; i < 2; i++ {
		now := time.Now()

		resp, err := cm.Generate(ctx, []*schema.Message{
			{
				Role: schema.System,
				Content: "You are an AI assistant tasked with analyzing literary works. " +
					"Your goal is to provide insightful commentary on themes.",
			},
			breakpoint,
			{
				Role:    schema.User,
				Content: "Analyze the major themes in the content.",
			},
		})
		if err != nil {
			log.Fatalf("Generate failed, err=%v", err)
		}

		log.Printf("time_consume=%f, output: \n%v", time.Now().Sub(now).Seconds(), resp)
	}
}

func toolInfoCache() {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")

	ctx := context.Background()

	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   &baseURL,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	breakpoint := claude.SetToolInfoBreakpoint(&schema.ToolInfo{
		Name: "get_time",
		Desc: "Get the current time in a given time zone",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"timezone": {
					Required: true,
					Type:     "string",
					Desc:     "The IANA time zone name, e.g. America/Los_Angeles",
				},
			},
		)},
	)

	mockTools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the current weather in a given location",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "The city and state, e.g. San Francisco, CA",
					},
					"unit": {
						Type: "string",
						Enum: []string{"celsius", "fahrenheit"},
						Desc: "The unit of temperature, either celsius or fahrenheit",
					},
				},
			),
		},
		{
			Name: "mock_tool_1",
			Desc: "The film Dongji Rescue, based on a true historical event, tells the story of Chinese fishermen braving turbulent seas to save strangers — a testament to the nation's capacity to create miracles in the face of adversity.\n\nEighty-three years ago, in 1942, the Japanese military seized the Lisbon Maru ship to transport 1,816 British prisoners of war from Hong Kong to Japan. Passing through the waters near Zhoushan, Zhejiang province, it was torpedoed by a US submarine. Fishermen from Zhoushan rescued 384 prisoners of war and hid them from Japanese search parties.\n\nActor Zhu Yilong, in an exclusive interview with China Daily, said he was deeply moved by the humanity shown in such extreme conditions when he accepted his role. \"This historical event proves that in dire circumstances, Chinese people can extend their goodness to protect both themselves and others,\" he said.\n\nLast Friday, a themed event for Dongji Rescue was held in Zhoushan, a city close to where the Lisbon Maru sank. Descendants of the British POWs and the rescuing fishermen gathered to watch the film and share their reflections.\n\nIn the film, the British POWs are aided by a group of fishermen from Dongji Island, whose courage and compassion cut a path through treacherous waves. After the screening, many descendants were visibly moved.\n\n\"I want to express my deepest gratitude to the Chinese people and the descendants of Chinese fishermen. When the film is released in the UK, I will bring my family and friends to watch it. Heroic acts like this deserve to be known worldwide,\" said Denise Wynne, a descendant of a British POW.\n\n\"I felt the profound friendship between the Chinese and British people through the film,\" said Li Hui, a descendant of a rescuer. Many audience members were brought to tears — some by the fishermen's bravery, others by the enduring spirit of \"never abandoning those in peril\".\n\n\"In times of sea peril, rescue is a must,\" said Wu Buwei, another rescuer's descendant, noting that this value has long been a part of Dongji fishermen's traditions.\n\nCoincidentally, on the morning of Aug 5, a real-life rescue unfolded on the shores of Dongfushan in Dongji town. Two tourists from Shanghai fell into the sea and were swept away by powerful waves. Without hesitation, two descendants of Dongji fishermen — Wang Yubin and Yang Xiaoping — leaped in to save them, braving a wind force of 9 to 10 before pulling them to safety.\n\n\"Our forebearers once rescued many British POWs here. As their descendants, we should carry forward their bravery,\" Wang said. Both rescuers later joined the film's cast and crew at the Zhoushan event, sharing the stage with actors who portrayed the fishermen in a symbolic \"reunion across 83 years\".\n\nChinese actor Wu Lei, who plays A Dang in the film, expressed his respect for the two rescuing fishermen on-site. In the film, A Dang saves a British POW named Thomas Newman. Though they share no common language, they manage to connect. In an interview with China Daily, Wu said, \"They connected through a small globe. A Dang understood when Newman pointed out his home, the UK, on the globe.\"\n\nThe film's director Fei Zhenxiang noted that the Eastern Front in World War II is often overlooked internationally, despite China's 14 years of resistance tying down large numbers of Japanese troops. \"Every Chinese person is a hero, and their righteousness should not be forgotten,\" he said.\n\nGuan Hu, codirector, said, \"We hope that through this film, silent kindness can be seen by the world\", marking the 80th anniversary of victory in the Chinese People's War of Resistance Against Japanese Aggression (1931-45) and the World Anti-Fascist War.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"mock_param": {
						Required: true,
						Type:     "string",
						Desc:     "mock parameter",
					},
				},
			),
		},
		{
			Name: "mock_tool_2",
			Desc: "For the administration, the biggest gain was Japan's commitment to investing $550 billion in the United States, particularly with funds to be directed to sectors deemed strategic, ranging from semiconductors and pharmaceuticals to energy infrastructure and critical minerals.The White House highlighted that the deal represents what it called the \"single largest foreign investment commitment ever secured by any country.\" It touted the agreement as a \"strategic realignment of the U.S.-Japan economic relationship\" that will advance the mutual interests of both countries.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"mock_param": {
						Required: true,
						Type:     "string",
						Desc:     "Bessent said it will check the extent of Japan's compliance every quarter and \"if the president is unhappy, then they will boomerang back to the 25 percent tariff rates, both on cars and the rest of their products.\"",
					},
				},
			),
		},
		{
			Name: "mock_tool_3",
			Desc: "Of the amount, up to 100,000 tons is imported for direct consumption. Most of the remainder is used for animal feed and processing into other food products. Any rice imported to Japan beyond the special quota is subject to a tariff of 341 yen ($2.3) per kilogram.\n\n\n In fiscal 2024, the United States shipped roughly 346,000 tons of rice to Japan, compared with 286,000 tons exported by Thailand and 70,000 tons by Australia, under the quota system, according to Japan's farm ministry.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"mock_param": {
						Required: true,
						Type:     "string",
						Desc:     "Currently, Japan imports about 770,000 tons of tariff-free rice a year under the WTO framework, with the United States the No. 1 exporter, accounting for nearly half of the total, followed by Thailand and Australia.",
					},
				},
			),
		},
		breakpoint,
	}

	chatModelWithTools, err := cm.WithTools(mockTools)
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
	}

	for i := 0; i < 2; i++ {
		now := time.Now()

		resp, err := chatModelWithTools.Generate(ctx, []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a tool calling assistant.",
			},
			{
				Role:    schema.User,
				Content: "Mock the get_weather tool input yourself, and then call get_weather",
			},
		})
		if err != nil {
			log.Fatalf("Generate failed, err=%v", err)
		}

		log.Printf("time_consume=%f, output: \n%v", time.Now().Sub(now).Seconds(), resp)
	}
}

func sessionCache() {
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")

	ctx := context.Background()

	cm, err := claude.NewChatModel(ctx, &claude.Config{
		// if you want to use Aws Bedrock Service, set these four field.
		// ByBedrock:       true,
		// AccessKey:       "",
		// SecretAccessKey: "",
		// Region:          "us-west-2",
		APIKey: apiKey,
		// Model:     "claude-3-5-sonnet-20240620",
		BaseURL:   &baseURL,
		Model:     modelName,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	opts := []model.Option{
		claude.WithEnableAutoCache(true),
	}

	mockTools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the current weather in a given location",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "The city and state, e.g. San Francisco, CA",
					},
					"unit": {
						Type: "string",
						Enum: []string{"celsius", "fahrenheit"},
						Desc: "The unit of temperature, either celsius or fahrenheit",
					},
				},
			),
		},
	}

	chatModelWithTools, err := cm.WithTools(mockTools)
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
		return
	}

	input := []*schema.Message{
		schema.SystemMessage("You are a tool calling assistant."),
		schema.UserMessage("What tools can you call?"),
	}

	resp, err := chatModelWithTools.Generate(ctx, input, opts...)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}
	log.Printf("output_1: \n%v\n\n", resp)

	input = append(input, resp, schema.UserMessage("What am I asking last time?"))

	resp, err = chatModelWithTools.Generate(ctx, input, opts...)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}

	log.Printf("output_2: \n%v\n\n", resp)
}

```



## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Claude Documentation](https://docs.claude.com/en/api/messages)
