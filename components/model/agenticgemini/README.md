# Google Gemini

A Google Gemini implementation for [Eino](https://github.com/cloudwego/eino) that implements the `model.AgentModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

## Features

- Implements `github.com/cloudwego/eino/components/model.AgentModel`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion
- Support for streaming responses
- Custom response parsing support
- Flexible model configuration

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/agenticgemini@latest
```

## Quick start

Here's a quick example of how to use the Gemini agentic model:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/agentic/gemini"
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

	cm, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
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

	resp, err := cm.Generate(ctx, []*schema.AgenticMessage{schema.UserAgenticMessage("What's the capital of France")})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("\n%s\n\n\n", resp.String())

	resp, err = cm.Generate(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("What's the capital of France"),
		resp,
		schema.UserAgenticMessage("What's the capital of England"),
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("\n%s\n\n\n", resp.String())
}


```

## Configuration

The model can be configured using the `agenticgemini.Config` struct:

```go
// Config contains the configuration options for the Gemini agentic model
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

// ResponseJSONSchema defines the structure for JSON responses
// Optional. Used when you want structured output in JSON format
ResponseJSONSchema *jsonschema.Schema

// EnableCodeExecution allows the model to use the server tool CodeExecution
// Optional.
EnableCodeExecution *genai.ToolCodeExecution
// EnableGoogleSearch allows the model to use the server tool GoogleSearch
// Optional.
EnableGoogleSearch *genai.GoogleSearch
// EnableGoogleSearchRetrieval allows the model to use the server tool GoogleSearchRetrieval
// Optional.
EnableGoogleSearchRetrieval *genai.GoogleSearchRetrieval
// EnableComputerUse allows the model to use the server tool ComputerUse
// Optional.
EnableComputerUse *genai.ComputerUse
// EnableURLContext allows the model to use the server tool URLContext
// Optional.
EnableURLContext *genai.URLContext
// EnableFileSearch allows the model to use the server tool FileSearch
// Optional.
EnableFileSearch *genai.FileSearch
// EnableGoogleMaps allows the model to use the server tool GoogleMaps
// Optional.
EnableGoogleMaps *genai.GoogleMaps

// SafetySettings configures content filtering for different harm categories
// Controls the model's filtering behavior for potentially harmful content
// Optional.
SafetySettings []*genai.SafetySetting

ThinkingConfig *genai.ThinkingConfig

// ResponseModalities specifies the modalities the model can return.
// Optional.
ResponseModalities []ResponseModality

MediaResolution genai.MediaResolution

// Cache controls prefix cache settings for the model.
// Optional. used to CreatePrefixCache for reused inputs.
Cache *CacheConfig
}
```


## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Gemini API Documentation](https://ai.google.dev/api/generate-content?hl=zh-cn#v1beta.GenerateContentResponse)
