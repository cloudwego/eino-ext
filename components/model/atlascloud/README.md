# Atlas Cloud ChatModel

An Atlas Cloud ChatModel implementation for [Eino](https://github.com/cloudwego/eino).
It wraps the OpenAI-compatible model client and defaults the LLM endpoint to `https://api.atlascloud.ai/v1`.

## Features

- Implements `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- Uses Atlas Cloud's OpenAI-compatible chat completions API
- Supports streaming responses
- Supports tool calling
- Supports request/response modifiers from the OpenAI-compatible client

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/atlascloud@latest
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/atlascloud"
)

func main() {
	ctx := context.Background()

	chatModel, err := atlascloud.NewChatModel(ctx, &atlascloud.ChatModelConfig{
		APIKey: os.Getenv("ATLASCLOUD_API_KEY"),
		Model:  os.Getenv("ATLASCLOUD_MODEL"), // e.g. deepseek-ai/DeepSeek-V3-0324
		// BaseURL is optional. Default: https://api.atlascloud.ai/v1
	})
	if err != nil {
		log.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("Explain Eino in one sentence."),
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	fmt.Println(resp.Content)
}
```

## Configuration

`atlascloud.ChatModelConfig` is an alias of `openai.ChatModelConfig`, so you can use the same OpenAI-compatible fields:

- `APIKey`: required
- `Model`: required
- `BaseURL`: optional, defaults to `https://api.atlascloud.ai/v1`
- `Timeout`, `HTTPClient`
- `Temperature`, `TopP`, `Stop`
- `MaxCompletionTokens`, `ResponseFormat`, `ReasoningEffort`
- `ExtraFields`, request/response modifiers, and other OpenAI-compatible options

## Notes

- Atlas Cloud's chat API is OpenAI-compatible.
- The `/v1` suffix is required in the base URL.
- Model names should use Atlas Cloud's model library IDs, such as `deepseek-ai/DeepSeek-V3-0324`.

## Example

See [examples/basic](./examples/basic/).
