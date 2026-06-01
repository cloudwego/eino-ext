# OpenAI ACL (API Client Library)

English | [简体中文](README_zh.md)

## Introduction

This is a low-level OpenAI API client library for Go. It provides direct access to OpenAI's Chat Completion and Embedding APIs, with support for both OpenAI and Azure OpenAI services. This library is used internally by higher-level component packages.

**Note**: For most use cases with Eino, you should use the higher-level component packages like `components/model/openai` and `components/embedding/openai` instead, which provide simpler interfaces integrated with Eino's component system.

## Features

- Direct OpenAI API client implementation
- Support for both OpenAI and Azure OpenAI services
- Chat completion with streaming support
- Embedding generation
- Tool/function calling support
- JSON schema response formatting
- Configurable HTTP client
- Thread-safe operations

## Installation

```bash
go get github.com/cloudwego/eino-ext/libs/acl/openai
```

## Chat Completion

### Basic Usage

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	client, err := openai.NewChatModel(ctx, &openai.Config{
		APIKey: "your-api-key",
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: "Hello, how are you?"},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println(resp.Message.Content)
}
```

### With Azure OpenAI

```go
client, err := openai.NewChatModel(ctx, &openai.Config{
	APIKey:     "your-azure-api-key",
	ByAzure:    true,
	BaseURL:    "https://your-resource.openai.azure.com",
	APIVersion: "2024-02-01",
})
```

### Streaming

```go
stream, err := client.Stream(ctx, []*schema.Message{
	{Role: schema.User, Content: "Tell me a story"},
})
if err != nil {
	log.Fatal(err)
}
defer stream.Close()

for {
	chunk, err := stream.Recv()
	if err == io.EOF {
		break
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(chunk.Message.Content)
}
```

## Embedding

### Basic Usage

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
)

func main() {
	ctx := context.Background()

	client, err := openai.NewEmbeddingClient(ctx, &openai.EmbeddingConfig{
		APIKey: "your-api-key",
		Model:  "text-embedding-3-small",
	})
	if err != nil {
		log.Fatal(err)
	}

	embeddings, err := client.EmbedStrings(ctx, []string{
		"Hello world",
		"How are you?",
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Generated %d embeddings", len(embeddings))
}
```

## Configuration

### Chat Model Config

```go
type Config struct {
    // Authentication (Required)
    APIKey string
    
    // HTTP Client (Optional)
    HTTPClient *http.Client
    
    // Azure OpenAI (Optional)
    ByAzure              bool
    BaseURL              string
    APIVersion           string
    AzureModelMapperFunc func(model string) string
    
    // OpenAI Base URL Override (Optional)
    BaseURL string
    
    // Chat Completion Parameters
    Model             string
    Temperature       *float32
    TopP              *float32
    N                 *int
    Stop              []string
    MaxTokens         *int
    PresencePenalty   *float32
    ResponseFormat    *ChatCompletionResponseFormat
    Seed              *int
    FrequencyPenalty  *float32
    LogitBias         map[string]int
    LogProbs          *bool
    TopLogProbs       *int
    User              *string
    Modalities        []Modality
    Audio             *Audio
    StreamOptions     *StreamOptions
}
```

### Embedding Config

```go
type EmbeddingConfig struct {
    // Authentication (Required)
    APIKey string
    
    // HTTP Client (Optional)
    HTTPClient *http.Client
    
    // Azure OpenAI (Optional)
    ByAzure    bool
    BaseURL    string
    APIVersion string
    
    // Embedding Parameters
    Model          string
    EncodingFormat *EmbeddingEncodingFormat
    Dimensions     *int
    User           *string
}
```

## Advanced Features

### Tool/Function Calling

```go
tools := []*schema.ToolInfo{
	{
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"location": {
				Type:        "string",
				Description: "The city name",
			},
		}),
	},
}

resp, err := client.Generate(ctx, messages, 
	model.WithTools(tools),
)
```

### JSON Schema Response

```go
jsonSchema := &openai.ChatCompletionResponseFormat{
	Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
	JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
		Name:   "weather_response",
		Schema: mySchema,
		Strict: true,
	},
}

resp, err := client.Generate(ctx, messages,
	model.WithGenOption(openai.WithResponseFormat(jsonSchema)),
)
```

### Call Options

Available call options:

```go
// Temperature control
openai.WithTemperature(0.7)

// Max tokens
openai.WithMaxTokens(1000)

// Top P sampling
openai.WithTopP(0.9)

// Frequency penalty
openai.WithFrequencyPenalty(0.5)

// Presence penalty
openai.WithPresencePenalty(0.5)

// Stop sequences
openai.WithStop([]string{"\n", "END"})

// Response format
openai.WithResponseFormat(format)

// Stream options
openai.WithStreamOptions(&openai.StreamOptions{
	IncludeUsage: true,
})
```

## Use Cases

This library is typically used:

1. **Internal Implementation**: As the underlying client for higher-level component packages
2. **Direct Integration**: When you need fine-grained control over OpenAI API calls
3. **Custom Solutions**: When building custom LLM integrations

For most Eino integrations, use the higher-level component packages instead:
- [components/model/openai](../../components/model/openai) for chat models
- [components/embedding/openai](../../components/embedding/openai) for embeddings

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
