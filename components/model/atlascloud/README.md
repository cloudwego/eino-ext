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

## Validated Atlas LLM Pool

Recommended validated Atlas Cloud model IDs for `ATLASCLOUD_MODEL`:

- `deepseek-ai/DeepSeek-V3-0324`, `deepseek-ai/deepseek-r1-0528`, `moonshotai/Kimi-K2-Instruct`, `Qwen/Qwen3-Coder`, `Qwen/Qwen3-235B-A22B-Instruct-2507`, `deepseek-ai/DeepSeek-V3.1`, `moonshotai/Kimi-K2-Instruct-0905`, `Qwen/Qwen3-Next-80B-A3B-Instruct`, `Qwen/Qwen3-Next-80B-A3B-Thinking`, `Qwen/Qwen3-30B-A3B-Instruct-2507`
- `deepseek-ai/DeepSeek-V3.1-Terminus`, `deepseek-ai/DeepSeek-V3.2-Exp`, `zai-org/GLM-4.6`, `MiniMaxAI/MiniMax-M2`, `Qwen/Qwen3-VL-235B-A22B-Instruct`, `moonshotai/Kimi-K2-Thinking`, `google/gemini-2.5-flash`, `google/gemini-2.5-flash-lite`, `openai/gpt-5.1`, `openai/gpt-5.1-chat`
- `openai/gpt-4o`, `openai/gpt-4o-mini`, `openai/gpt-4.1`, `openai/gpt-4.1-mini`, `openai/gpt-4.1-nano`, `openai/o1`, `openai/o3`, `openai/o3-mini`, `openai/o4-mini`, `anthropic/claude-sonnet-4.5-20250929`
- `deepseek-ai/deepseek-v3.2`, `openai/gpt-5`, `openai/gpt-5-chat`, `openai/gpt-5-mini`, `openai/gpt-5-nano`, `openai/gpt-5.2`, `openai/gpt-5.2-chat`, `google/gemini-2.5-pro`, `anthropic/claude-opus-4.5-20251101`, `google/gemini-3-flash-preview`
- `zai-org/glm-4.7`, `minimaxai/minimax-m2.1`, `google/gemini-2.0-flash`, `qwen/qwen3-8b`, `qwen/qwen3-235b-a22b-thinking-2507`, `qwen/qwen3-vl-235b-a22b-thinking`, `qwen/qwen3-30b-a3b`, `qwen/qwen3-30b-a3b-thinking-2507`, `deepseek-ai/deepseek-ocr`, `xai/grok-4-0709`

## Example

See [examples/basic](./examples/basic/).
