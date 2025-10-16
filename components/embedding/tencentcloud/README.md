# Tencent Cloud Hunyuan Embedding

English | [简体中文](README_zh.md)

A Tencent Cloud Hunyuan embedding implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Embedder` interface. This enables seamless integration with Eino's embedding system for text embedding capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/embedding.Embedder`
- Easy integration with Eino's rag workflow
- Built-in token usage tracking
- Automatic batch processing for large text arrays
- Built-in callback support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/embedding/tencentcloud
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/cloudwego/eino-ext/components/embedding/tencentcloud"
)

func main() {
    ctx := context.Background()
    
    // Create embedder config
    cfg := &tencentcloud.EmbeddingConfig{
        SecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
        SecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
        Region:    "ap-guangzhou",
    }

    // Create the embedder
    embedder, err := tencentcloud.NewEmbedder(ctx, cfg)
    if err != nil {
        panic(err)
    }

    // Get embeddings for texts
    embeddings, err := embedder.EmbedStrings(ctx, []string{"hello world", "bye world"})
    if err != nil {
        panic(err)
    }

    fmt.Printf("Embeddings: %v\n", embeddings)
}
```

## Configuration

The embedder can be configured using the `EmbeddingConfig` struct:

```go
type EmbeddingConfig struct {
    SecretID  string // Tencent Cloud Secret ID
    SecretKey string // Tencent Cloud Secret Key
    Region    string // Tencent Cloud Region (e.g. "ap-hongkong")
}
```

## Features Details

### Automatic Batch Processing

The embedder automatically handles batch processing for large text arrays. According to Tencent Cloud's API limitations, each request can process up to 200 texts. The embedder will automatically split larger arrays into appropriate batches.

### Token Usage Tracking

The embedder tracks token usage through Eino's callback system. Token usage information includes:
- Prompt tokens
- Total tokens

### Callbacks Support

The embedder fully supports Eino's callback system, enabling:
- Error tracking
- Start/End event monitoring
- Token usage statistics

## For More Details

- [Tencent Cloud Hunyuan API Documentation](https://cloud.tencent.com/document/product/1729/102832)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
