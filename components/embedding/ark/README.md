# Ark Embedding for Eino

English | [简体中文](./README_zh.md)

## Introduction

This is an Ark Embedding component implementation for [Eino](https://github.com/cloudwego/eino), which implements the `Embedder` interface and seamlessly integrates into Eino's embedding system to provide text vectorization capabilities. It supports both text embedding and multi-modal embedding APIs provided by VolcEngine Ark.

## Features

- Implements `github.com/cloudwego/eino/components/embedding.Embedder` interface
- Easy integration into Eino workflows
- Supports both text and multi-modal embedding APIs
- Supports multiple authentication methods (API Key or AccessKey/SecretKey)
- Built-in callback support in Eino
- Configurable retry mechanism and timeout settings
- Concurrent request support for multi-modal embedding API

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/embedding/ark
```

## Quick Start

### Text Embedding API

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
)

func main() {
	ctx := context.Background()

	embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: "your-api-key",
		Model:  "your-model-endpoint-id",
	})
	if err != nil {
		log.Fatalf("NewEmbedder of ark error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of Ark failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

### Multi-Modal Embedding API

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
)

func main() {
	ctx := context.Background()

	apiType := ark.APITypeMultiModal
	embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey:  "your-api-key",
		Model:   "your-model-endpoint-id",
		APIType: &apiType,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of ark error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of Ark failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

## Configuration

The embedder can be configured through the `EmbeddingConfig` structure:

```go
type EmbeddingConfig struct {
    // Timeout specifies the maximum duration to wait for API responses
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default: 10 minutes
    Timeout *time.Duration `json:"timeout"`

    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // RetryTimes specifies the number of retry attempts for failed API calls
    // Optional. Default: 2
    RetryTimes *int `json:"retry_times"`

    // BaseURL specifies the base URL for Ark service
    // Optional. Default: "https://ark.cn-beijing.volces.com/api/v3"
    BaseURL string `json:"base_url"`

    // Region specifies the region where Ark service is located
    // Optional. Default: "cn-beijing"
    Region string `json:"region"`

    // APIKey is the API key for authentication
    // APIKey takes precedence if both APIKey and AccessKey/SecretKey are provided
    // For authentication details, see: https://www.volcengine.com/docs/82379/1298459
    APIKey string `json:"api_key"`

    // AccessKey and SecretKey for authentication
    // Either APIKey or AccessKey/SecretKey pair is required
    AccessKey string `json:"access_key"`
    SecretKey string `json:"secret_key"`

    // Model specifies the ID of endpoint on Ark platform
    // Required
    Model string `json:"model"`

    // APIType specifies which API to use: text or multi-modal
    // Optional. Default: APITypeText
    APIType *APIType `json:"api_type,omitempty"`

    // MaxConcurrentRequests specifies the maximum number of concurrent multi-modal embedding API calls
    // Only applicable when APIType is APITypeMultiModal
    // Optional. Default: 5
    MaxConcurrentRequests *int `json:"max_concurrent_requests"`
}
```

### API Types

- `APITypeText`: Uses `/embeddings` text embedding API
  - API Reference: https://www.volcengine.com/docs/82379/1521766
  - BaseURL: https://ark.cn-beijing.volces.com/api/v3

- `APITypeMultiModal`: Uses `/embeddings/multimodal` multi-modal embedding API
  - API Reference: https://www.volcengine.com/docs/82379/1523520
  - BaseURL: https://ark.cn-beijing.volces.com/api/v3

## Authentication

The embedder supports two authentication methods:

1. **API Key**: Provide `APIKey` in the configuration
2. **Access Key / Secret Key**: Provide both `AccessKey` and `SecretKey` in the configuration

For more details about authentication, see the [VolcEngine documentation](https://www.volcengine.com/docs/82379/1298459).

## Examples

See the following examples for more usage:

- [Text Embedding](./examples/embedding/)

## License

This component is licensed under the Apache License 2.0. See the LICENSE file for details.
