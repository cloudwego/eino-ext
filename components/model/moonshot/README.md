# Moonshot (Kimi) Model

A Moonshot AI (Kimi) model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface, built on top of the OpenAI-compatible API exposed at `https://api.moonshot.cn/v1`.

## Features

- Implements `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- Chat completion and streaming
- Tool / function calling
- JSON mode via `ResponseFormat`
- `WithExtraFields` escape hatch for Moonshot-specific parameters such as `partial` (prefix completion) and `context_cache`

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/moonshot@latest
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/cloudwego/eino-ext/components/model/moonshot"
    "github.com/cloudwego/eino/schema"
)

func main() {
    ctx := context.Background()
    // Get an API key at https://platform.moonshot.cn/console/api-keys
    chatModel, err := moonshot.NewChatModel(ctx, &moonshot.ChatModelConfig{
        APIKey:      os.Getenv("MOONSHOT_API_KEY"),
        Model:       "kimi-k2-0905-preview",
        MaxTokens:   of(2048),
        Temperature: of(float32(0.3)),
    })
    if err != nil {
        log.Fatalf("NewChatModel failed: %v", err)
    }

    resp, err := chatModel.Generate(ctx, []*schema.Message{
        schema.UserMessage("introduce yourself in one sentence"),
    })
    if err != nil {
        log.Fatalf("Generate failed: %v", err)
    }

    fmt.Println(resp.Content)
}

func of[T any](t T) *T { return &t }
```

`BaseURL` defaults to `https://api.moonshot.cn/v1` and only needs to be set when pointing at a proxy or a self-hosted gateway.

## Moonshot-specific parameters

Moonshot exposes a few non-OpenAI parameters. Pass them through `WithExtraFields`:

```go
resp, err := chatModel.Generate(ctx, msgs,
    moonshot.WithExtraFields(map[string]any{
        // Prefix completion: https://platform.moonshot.cn/docs/api/partial
        "partial": true,
    }),
)
```

## For More Details
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Moonshot API Documentation](https://platform.moonshot.cn/docs/api/chat)
