# DashScope Embedding for Eino

English | [简体中文](./README_zh.md)

## Introduction

This is a DashScope Embedding component implementation for [Eino](https://github.com/cloudwego/eino), which implements the `Embedder` interface and seamlessly integrates into Eino's embedding system to provide text vectorization capabilities. It uses Alibaba Cloud's DashScope text embedding API through OpenAI-compatible endpoints.

## Features

- Implements `github.com/cloudwego/eino/components/embedding.Embedder` interface
- Easy integration into Eino workflows
- Support for multiple DashScope embedding models (text-embedding-v1, text-embedding-v2, text-embedding-v3)
- Configurable output dimensions for text-embedding-v3 model
- Built-in callback support in Eino
- Configurable timeout and HTTP client settings

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/embedding/dashscope
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
)

func main() {
	ctx := context.Background()

	embedder, err := dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		APIKey:  "your-dashscope-api-key",
		Model:   "text-embedding-v3",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of dashscope error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of DashScope failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

## Configuration

The embedder can be configured through the `EmbeddingConfig` structure:

```go
type EmbeddingConfig struct {
    // APIKey is your DashScope API key
    // Required
    APIKey string `json:"api_key"`

    // Timeout specifies the http request timeout.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default: no timeout
    Timeout time.Duration `json:"timeout"`

    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // Model specifies which embedding model to use
    // Available models: text-embedding-v1, text-embedding-v2, text-embedding-v3
    // Async embedding models are not supported
    // Required
    Model string `json:"model"`

    // Dimensions specify output vector dimension
    // Only applicable to text-embedding-v3 model
    // Can only be selected between three values: 1024, 768, and 512
    // Optional. Default: 1024
    Dimensions *int `json:"dimensions,omitempty"`
}
```

## Available Models

DashScope supports the following text embedding models:

- `text-embedding-v1`: Basic text embedding model
- `text-embedding-v2`: Enhanced text embedding model  
- `text-embedding-v3`: Latest text embedding model with configurable dimensions (512, 768, or 1024)

Note: Async embedding models are not supported.

## Examples

See the following examples for more usage:

- [Text Embedding](./examples/embedding/)

## API Reference

For more details about DashScope's text embedding API, please refer to:
- [DashScope Text Embedding API Documentation](https://help.aliyun.com/zh/model-studio/developer-reference/text-embedding-synchronous-api)

## License

This component is licensed under the Apache License 2.0. See the LICENSE file for details.
