# MiniMax - ChatModel

English | [中文](README_zh.md)

## Overview

MiniMax ChatModel component for the [Eino](https://github.com/cloudwego/eino) framework, providing access to [MiniMax](https://www.minimaxi.com/) large language models via the OpenAI-compatible API.

## Supported Models

| Model ID | Description |
|----------|-------------|
| `MiniMax-M2.7` | Peak Performance. Ultimate Value. Master the Complex. (default) |
| `MiniMax-M2.7-highspeed` | Same performance, faster and more agile |

Both models support 204,800 tokens context window and up to 192K output tokens.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cloudwego/eino-ext/components/model/minimax"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()

    model, err := minimax.NewChatModel(ctx, &minimax.Config{
        APIKey: "your-minimax-api-key", // or set MINIMAX_API_KEY env var
        Model:  "MiniMax-M2.7",
    })
    if err != nil {
        log.Fatal(err)
    }

    result, err := model.Generate(ctx, []*schema.Message{
        schema.SystemMessage("You are a helpful assistant."),
        {Role: schema.User, Content: "Hello!"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result.Content)
}
```

## Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `APIKey` | string | Yes | - | MiniMax API key |
| `Model` | string | No | `MiniMax-M2.7` | Model ID |
| `BaseURL` | string | No | `https://api.minimax.io/v1` | API endpoint. Use `https://api.minimaxi.com/v1` for mainland China |
| `Temperature` | *float32 | No | `1.0` | Sampling temperature, range (0.0, 1.0] |
| `TopP` | *float32 | No | `1.0` | Nucleus sampling parameter |
| `MaxTokens` | *int | No | model's max | Maximum output tokens |
| `Stop` | []string | No | - | Stop sequences |
| `Timeout` | time.Duration | No | no timeout | Request timeout |

## Features

- Chat completion (Generate)
- Streaming (Stream)
- Tool calling / Function calling
- Callback integration
- Temperature clamping (auto-clamps to MiniMax's valid range)

## Links

- [MiniMax Platform](https://platform.minimax.io/)
- [MiniMax API Reference](https://platform.minimax.io/docs/api-reference/text-openai-api)
- [Eino Framework](https://github.com/cloudwego/eino)
