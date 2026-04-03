# Minimax ChatModel

This module provides a Minimax ChatModel implementation for the Eino framework.

## Supported Models

- MiniMax-M2.7
- MiniMax-M2.7-highspeed
- MiniMax-M2.5
- MiniMax-M2.5-highspeed
- MiniMax-M2.1
- MiniMax-M2.1-highspeed
- MiniMax-M2

## Requirements

- Go 1.22+
- `github.com/cloudwego/eino` v0.7.13+

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/minimax
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/cloudwego/eino-ext/components/model/minax"
)

func main() {
    ctx := context.Background()
    
    model, err := minimax.NewChatModel(ctx, &minimax.Config{
        APIKey:    "your-api-key",
        Model:     "MiniMax-M2.7",
        MaxTokens: 1024,
    })
    if err != nil {
        panic(err)
    }
    
    resp, err := model.Generate(ctx, []*schema.Message{
        schema.UserMessage("Hello, who are you?"),
    })
    if err != nil {
        panic(err)
    }
    
    fmt.Println(resp.Content)
}
```

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| APIKey | string | Yes | Your Minimax API key |
| Model | string | Yes | Model name (e.g., "MiniMax-M2.7") |
| MaxTokens | int | Yes | Maximum tokens to generate |
| BaseURL | *string | No | Custom API endpoint |
| Temperature | *float32 | No | Sampling temperature (0.0-1.0) |
| TopP | *float32 | No | Nucleus sampling parameter |
| HTTPClient | *http.Client | No | Custom HTTP client |
| AdditionalHeaderFields | map[string]string | No | Extra HTTP headers |
| AdditionalRequestFields | map[string]any | No | Extra request fields |

## Tool Calling

```go
model, err := minimax.NewChatModel(ctx, &minimax.Config{
    APIKey:    "your-api-key",
    Model:     "MiniMax-M2.7",
    MaxTokens: 1024,
})

toolModel, err := model.WithTools([]*schema.ToolInfo{
    {
        Name: "get_weather",
        Desc: "Get current weather for a location",
        ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
            Type: "object",
            Properties: orderedmap.New[string, *jsonschema.Schema](
                orderedmap.Pair[string, *jsonschema.Schema]{
                    Key: "location",
                    Value: &jsonschema.Schema{Type: "string"},
                },
            ),
            Required: []string{"location"},
        }),
    },
})
```

## Streaming

```go
stream, err := model.Stream(ctx, []*schema.Message{
    schema.UserMessage("Write a story"),
})

for {
    chunk, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        panic(err)
    }
    fmt.Print(chunk.Content)
}
```

## License

Apache 2.0
