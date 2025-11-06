# CozeLoop Component for Eino

This is a CozeLoop component implementation for [Eino](https://github.com/cloudwego/eino) that integrates with [CozeLoop](https://github.com/coze-dev/cozeloop-go). The component implements the `ChatTemplate` interface and allows you to fetch and format prompts from CozeLoop's prompt management service.

## Features

- Implements `github.com/cloudwego/eino/components/prompt.ChatTemplate` interface
- Integrates with CozeLoop SDK for prompt management
- Supports prompt versioning
- Automatic prompt formatting with variable substitution
- Callback support for observability and monitoring

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/prompt/cozeloop
```

## Prerequisites

Before using this component, you need to:

1. Set up a CozeLoop account and workspace
2. Create prompts in the CozeLoop prompt management system
3. Obtain authentication credentials (API token or JWT OAuth credentials)

## Configuration

Set the following environment variables:

```bash
# Required
export COZELOOP_WORKSPACE_ID=your-workspace-id

# Option 1: Using API Token (recommended for testing only)
export COZELOOP_API_TOKEN=your-api-token

# Option 2: Using JWT OAuth (recommended for production)
export COZELOOP_JWT_OAUTH_CLIENT_ID=your-client-id
export COZELOOP_JWT_OAUTH_PRIVATE_KEY=your-private-key
export COZELOOP_JWT_OAUTH_PUBLIC_KEY_ID=your-public-key-id
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/prompt/cozeloop"
	cozeloopgo "github.com/coze-dev/cozeloop-go"
)

func main() {
	ctx := context.Background()

	// Initialize CozeLoop client
	client, err := cozeloopgo.NewClient()
	if err != nil {
		log.Fatalf("Failed to create CozeLoop client: %v", err)
	}
	defer client.Close(ctx)

	// Create PromptHub component
	ph, err := cozeloop.NewPromptHub(ctx, &cozeloop.Config{
		Key:            "your.prompt.key", // Your prompt key from CozeLoop
		Version:        "",                // Empty for latest version
		CozeLoopClient: client,
	})
	if err != nil {
		log.Fatalf("Failed to create PromptHub: %v", err)
	}

	// Format the prompt with variables
	variables := map[string]any{
		"user_name": "John Doe",
		"topic":     "example topic",
	}

	messages, err := ph.Format(ctx, variables)
	if err != nil {
		log.Fatalf("Failed to format prompt: %v", err)
	}

	// Use the formatted messages
	for _, msg := range messages {
		fmt.Printf("Role: %s, Content: %s\n", msg.Role, msg.Content)
	}
}
```

### Multi-Modal Content Support

The component supports multi-modal content (text + images) in prompt variables. You can pass `[]*entity.ContentPart` as variable values:

```go
import "github.com/coze-dev/cozeloop-go/entity"

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}

// Create multi-modal content with text and images
multiModalContent := []*entity.ContentPart{
	{
		Type: entity.ContentTypeText,
		Text: strPtr("Describe this image: "),
	},
	{
		Type:     entity.ContentTypeImageURL,
		ImageURL: strPtr("https://example.com/image.png"),
	},
	{
		Type: entity.ContentTypeText,
		Text: strPtr(" in detail."),
	},
}

variables := map[string]any{
	"user_input": multiModalContent,
}

messages, err := ph.Format(ctx, variables)
// The formatted messages will contain UserInputMultiContent or AssistantGenMultiContent
// based on the message role in the prompt template
```

**Supported Content Types:**
- `entity.ContentTypeText` - Plain text content
- `entity.ContentTypeImageURL` - Image referenced by URL
- `entity.ContentTypeBase64Data` - Image encoded in base64

**Note:** Multi-modal content is supported for both User messages and Assistant messages (e.g., for few-shot examples in prompt templates).

## API Reference

### NewPromptHub

Creates a new PromptHub instance.

```go
func NewPromptHub(ctx context.Context, conf *Config) (prompt.ChatTemplate, error)
```

**Config fields:**
- `Key` (string, required): The prompt key from CozeLoop
- `Version` (string, optional): Specific version of the prompt. Empty string gets the latest version
- `CozeLoopClient` (cozeloop.Client, required): Initialized CozeLoop client

### Format

Formats the prompt with the provided variables.

```go
func (p *promptHub) Format(ctx context.Context, vs map[string]any, opts ...prompt.Option) ([]*schema.Message, error)
```

**Parameters:**
- `ctx`: Context for the operation
- `vs`: Map of variable names to values for prompt substitution
- `opts`: Optional prompt formatting options

**Returns:**
- Slice of formatted messages
- Error if formatting fails

### GetType

Returns the component type identifier.

```go
func (p *promptHub) GetType() string
```

**Returns:** "PromptHub"

## Integration with Eino

This component can be seamlessly integrated into Eino workflows:

```go
import (
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/components/prompt/cozeloop"
)

// Use in a chain
chain := compose.NewChain[map[string]any, []*schema.Message]()
chain.AppendChatTemplate(ph)
```

## Error Handling

The component returns errors in the following cases:
- Failed to create CozeLoop client
- Invalid configuration (nil config or client)
- Prompt not found or empty
- Prompt formatting failure
- Network or API errors

## Observability

The component supports Eino's callback system for observability:

```go
import (
	"github.com/cloudwego/eino/callbacks"
	ccb "github.com/cloudwego/eino-ext/callbacks/cozeloop"
)

// Add CozeLoop callback handler
client, _ := cozeloop.NewClient()
handler := ccb.NewLoopHandler(client)
callbacks.AppendGlobalHandlers(handler)
```

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## Related Projects

- [Eino](https://github.com/cloudwego/eino) - The main Eino framework
- [CozeLoop Go SDK](https://github.com/coze-dev/cozeloop-go) - Official CozeLoop Go SDK
- [Eino Extensions](https://github.com/cloudwego/eino-ext) - Collection of Eino extensions
