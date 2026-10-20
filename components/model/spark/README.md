# Spark Model

English | [简体中文](README_zh.md)

An [iFLYTEK Spark](https://www.xfyun.cn/doc/spark/HTTP%E8%B0%83%E7%94%A8%E6%96%87%E6%A1%A3.html) (讯飞星火) model implementation for [Eino](https://github.com/cloudwego/eino) that implements the `ToolCallingChatModel` interface. This enables seamless integration with Eino's LLM capabilities for enhanced natural language processing and generation.

Spark exposes an OpenAI-compatible HTTP API, so this component is a thin wrapper over the shared OpenAI-compatible client, in the same spirit as the `qwen` and `deepseek` components.

## Features

- Implements `github.com/cloudwego/eino/components/model.ToolCallingChatModel`
- Easy integration with Eino's model system
- Configurable model parameters
- Support for chat completion and streaming responses
- Tool calling support
- Custom header / extra request field support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/model/spark@latest
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/spark"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	cm, err := spark.NewChatModel(ctx, &spark.ChatModelConfig{
		APIKey: os.Getenv("SPARK_API_KEY"), // the "API Password" from the iFLYTEK console
		Model:  "4.0Ultra",
	})
	if err != nil {
		log.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You are a helpful assistant."),
		schema.UserMessage("What is the capital of France?"),
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	log.Printf("Response: %s", resp.Content)
}
```

## Configuration

```go
type ChatModelConfig struct {
	// APIKey is the Spark HTTP service "API Password" — the single Bearer
	// credential shown as APIPassword in the iFLYTEK console. It is NOT the
	// legacy APPID / APIKey / APISecret triple used by the WebSocket API.
	// Required.
	APIKey string

	// Timeout specifies the maximum duration to wait for API responses.
	Timeout time.Duration

	// HTTPClient specifies the client to send HTTP requests.
	HTTPClient *http.Client

	// BaseURL specifies the Spark OpenAI-compatible endpoint URL.
	// Optional. Default: https://spark-api-open.xf-yun.com/v1
	BaseURL string

	// Model specifies the ID of the model to use.
	// Optional. Default: 4.0Ultra.
	Model string

	// Standard OpenAI-compatible parameters (all optional):
	MaxTokens        *int
	Temperature      *float32
	TopP             *float32
	Stop             []string
	PresencePenalty  *float32
	ResponseFormat   *openai.ChatCompletionResponseFormat
	Seed             *int
	FrequencyPenalty *float32
	LogitBias        map[string]int
	User             *string
}
```

### Credentials and endpoint

- **API Password (`APIKey`)**: obtain it from the [iFLYTEK Spark console](https://console.xfyun.cn/services/bmx1). It is the single `APIPassword` value, sent as the HTTP `Authorization: Bearer <APIPassword>` header — **not** the legacy `APPID` / `APIKey` / `APISecret` triple, which belongs to the WebSocket API only.
- **Endpoint**: this component targets the OpenAI-compatible HTTP endpoint `https://spark-api-open.xf-yun.com/v1`. Do not point `BaseURL` at the WebSocket host `spark-api.xf-yun.com`.

### Supported models (HTTP endpoint)

| Model        | Notes                          |
|--------------|--------------------------------|
| `4.0Ultra`   | Flagship, default              |
| `generalv3.5`| Spark Max                      |
| `max-32k`    | Spark Max, 32k context         |
| `generalv3`  | Spark Pro                      |
| `pro-128k`   | Spark Pro, 128k context        |
| `lite`       | Spark Lite                     |

## Streaming

```go
sr, err := cm.Stream(ctx, []*schema.Message{
	schema.UserMessage("Write a short poem about the moon."),
})
if err != nil {
	log.Fatalf("Stream failed: %v", err)
}
defer sr.Close()

for {
	chunk, err := sr.Recv()
	if err == io.EOF {
		break
	}
	if err != nil {
		log.Fatalf("Recv failed: %v", err)
	}
	log.Print(chunk.Content)
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [iFLYTEK Spark HTTP API Documentation](https://www.xfyun.cn/doc/spark/HTTP%E8%B0%83%E7%94%A8%E6%96%87%E6%A1%A3.html)
