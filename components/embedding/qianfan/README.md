# QianFan Embedding for Eino

English | [简体中文](./README_zh.md)

## Introduction

This is a QianFan (Baidu Qianfan) Embedding component implementation for [Eino](https://github.com/cloudwego/eino), which implements the `Embedder` interface and seamlessly integrates into Eino's embedding system to provide text vectorization capabilities using Baidu's Qianfan platform.

## Features

- Implements `github.com/cloudwego/eino/components/embedding.Embedder` interface
- Easy integration into Eino workflows
- Support for Baidu Qianfan embedding models
- Built-in callback support in Eino
- Configurable retry mechanism (retry count, timeout, backoff factor)
- Token usage tracking

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/embedding/qianfan
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/qianfan"
)

func main() {
	ctx := context.Background()

	qcfg := qianfan.GetQianfanSingletonConfig()
	qcfg.AccessKey = os.Getenv("QIANFAN_ACCESS_KEY")
	qcfg.SecretKey = os.Getenv("QIANFAN_SECRET_KEY")

	embedder, err := qianfan.NewEmbedder(ctx, &qianfan.EmbeddingConfig{
		Model: "Embedding-V1",
	})
	if err != nil {
		log.Fatalf("NewEmbedder of qianfan error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello world", "bye world"})
	if err != nil {
		log.Fatalf("EmbedStrings of QianFan failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

## Configuration

### Authentication Configuration

QianFan uses a singleton configuration pattern for authentication. You need to configure authentication before creating the embedder:

```go
qcfg := qianfan.GetQianfanSingletonConfig()
qcfg.AccessKey = "your-access-key"
qcfg.SecretKey = "your-secret-key"
```

You can also set authentication via environment variables:
- `QIANFAN_ACCESS_KEY`: Your IAM Access Key
- `QIANFAN_SECRET_KEY`: Your IAM Secret Key

### Embedder Configuration

The embedder can be configured through the `EmbeddingConfig` structure:

```go
type EmbeddingConfig struct {
    // Model specifies which Qianfan embedding model to use
    // Required
    Model string

    // LLMRetryCount specifies the number of retry attempts for failed API calls
    // Optional
    LLMRetryCount *int

    // LLMRetryTimeout specifies the timeout duration for retry attempts (in seconds)
    // Optional
    LLMRetryTimeout *float32

    // LLMRetryBackoffFactor specifies the backoff multiplier for retry attempts
    // Optional
    LLMRetryBackoffFactor *float32
}
```

### Example with Retry Configuration

```go
retryCount := 3
retryTimeout := float32(10.0)
backoffFactor := float32(2.0)

embedder, err := qianfan.NewEmbedder(ctx, &qianfan.EmbeddingConfig{
    Model:                 "Embedding-V1",
    LLMRetryCount:         &retryCount,
    LLMRetryTimeout:       &retryTimeout,
    LLMRetryBackoffFactor: &backoffFactor,
})
```

## Available Models

QianFan supports various embedding models. Common models include:

- `Embedding-V1`: Baidu's standard embedding model

For the complete list of available models and their specifications, please refer to the [Baidu Qianfan documentation](https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Nlks5zkzu).

## Authentication

To use the QianFan API, you need to obtain IAM credentials:

1. Sign up for Baidu Cloud account
2. Create credentials in the IAM console to get AccessKey and SecretKey
3. Set the credentials either via code or environment variables

For more details about authentication, please refer to the [Baidu Qianfan Authentication Guide](https://cloud.baidu.com/doc/WENXINWORKSHOP/s/Ilkkrb0i5).

## Token Usage

The embedder automatically tracks token usage and returns it through callbacks:

- `PromptTokens`: Number of tokens in the input
- `CompletionTokens`: Number of tokens in the output (if applicable)
- `TotalTokens`: Total number of tokens used

## Examples

See the [examples](./examples/) directory for complete usage examples.

## License

This component is licensed under the Apache License 2.0. See the LICENSE file for details.
