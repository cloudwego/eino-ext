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

	"github.com/cloudwego/eino-ext/components/model/agenticgemini"
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

	cm, err := agenticgemini.New(ctx, &agenticgemini.Config{
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
    
    // SafetySettings configures content filtering for different harm categories
    // Controls the model's filtering behavior for potentially harmful content
    // Optional.
    SafetySettings []*genai.SafetySetting
    
    ThinkingConfig *genai.ThinkingConfig

    // ImageConfig is the image generation configuration.
    // Note: an error will be returned if this field is set for a model that does not support the configuration options.
    // Optional.
    ImageConfig *genai.ImageConfig
    
    // ResponseModalities specifies the modalities the model can return.
    // Optional.
    ResponseModalities []genai.Modality
    
    MediaResolution genai.MediaResolution
    
    // CacheExpiration configures the expiration policy for prefix cache resources.
    // Optional.
    CacheExpiration *CacheExpiration
}
```


## Extension Fields

Several fields in the Eino agentic schema are typed as `any` so that each model implementation can attach
provider-specific data. When you consume responses produced by this package, you must type-assert these `any`
fields to the concrete types defined here before you can read them. This section documents every such field and
the exact type it carries.

### ResponseMeta

Each returned `*schema.AgenticMessage` carries a `ResponseMeta *schema.AgenticResponseMeta`. This package
populates the strongly-typed `GeminiExtension` field (no assertion required); the generic `Extension any` field
is left unused.

```go
type AgenticResponseMeta struct {
    // TokenUsage is populated with prompt / completion / total token counts.
    TokenUsage *TokenUsage

    // GeminiExtension is populated by this package (strongly typed, no assertion needed).
    GeminiExtension *gemini.ResponseMetaExtension

    // OpenAIExtension / ClaudeExtension / Extension are unused by this package.
}
```

`GeminiExtension` is `*github.com/cloudwego/eino/schema/gemini.ResponseMetaExtension`. This package fills in the
finish reason and, when grounding (Google Search) is used, the grounding metadata:

```go
type ResponseMetaExtension struct {
    ID            string             // response ID (concatenated from stream chunks)
    FinishReason  string             // e.g. "STOP", "MAX_TOKENS", "SAFETY"
    GroundingMeta *GroundingMetadata // non-nil only when grounding/search is used
}
```

```go
ext := msg.ResponseMeta.GeminiExtension // strongly typed, no assertion needed
```

### ServerToolCall & ServerToolResult

When the model uses the built-in code execution server tool, the resulting content blocks carry
`*schema.ServerToolCall` and `*schema.ServerToolResult`. Both wrap their payload in an `any` field, which this
package always populates with its own concrete types. The `Name` field is `agenticgemini.ServerToolNameCodeExecution`.

```go
type ServerToolCall struct {
    Name      string // "CodeExecution" (agenticgemini.ServerToolNameCodeExecution)
    CallID    string
    Arguments any    // concrete type: *agenticgemini.ServerToolCallArguments
}

type ServerToolResult struct {
    Name    string
    CallID  string
    Content any    // concrete type: *agenticgemini.ServerToolCallResult
}
```

#### `ServerToolCall.Arguments` (`any`)

Assert to `*agenticgemini.ServerToolCallArguments`. It carries the executable code the model produced:

```go
type ServerToolCallArguments struct {
    ExecutableCode *ExecutableCode // Code string + Language (e.g. agenticgemini.LanguagePython)
}
```

```go
// The concrete type is always *agenticgemini.ServerToolCallArguments.
args, ok := msg.ContentBlocks[i].ServerToolCall.Arguments.(*agenticgemini.ServerToolCallArguments)
```

#### `ServerToolResult.Content` (`any`)

Assert to `*agenticgemini.ServerToolCallResult`. It carries the outcome and output of the executed code:

```go
type ServerToolCallResult struct {
    CodeExecutionResult *CodeExecutionResult // Outcome (e.g. agenticgemini.OutcomeOK) + Output string
}
```

```go
// The concrete type is always *agenticgemini.ServerToolCallResult.
result, ok := msg.ContentBlocks[i].ServerToolResult.Content.(*agenticgemini.ServerToolCallResult)
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Gemini API Documentation](https://ai.google.dev/api/generate-content?hl=zh-cn#v1beta.GenerateContentResponse)
