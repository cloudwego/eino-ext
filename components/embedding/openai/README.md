# OpenAI Embedding for Eino

English | [简体中文](./README_zh.md)

## Introduction

This is an OpenAI Embedding component implementation for [Eino](https://github.com/cloudwego/eino), which implements the `Embedder` interface and seamlessly integrates into Eino's embedding system to provide text vectorization capabilities. It supports both OpenAI API and Azure OpenAI Service.

## Features

- Implements `github.com/cloudwego/eino/components/embedding.Embedder` interface
- Easy integration into Eino workflows
- Support for both OpenAI and Azure OpenAI Service
- Support for multiple OpenAI embedding models
- Configurable encoding format (float or base64)
- Configurable output dimensions for text-embedding-3 models
- Built-in callback support in Eino
- Configurable timeout and HTTP client settings

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/embedding/openai
```

## Quick Start

### Using OpenAI API

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	ctx := context.Background()

	embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
		APIKey:  "your-openai-api-key",
		Model:   "text-embedding-3-small",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of openai error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of OpenAI failed, err=%v", err)
	}

	log.Printf("vectors : %v", vectors)
}
```

### Using Azure OpenAI Service

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	ctx := context.Background()

	embedder, err := openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
		APIKey:     "your-azure-api-key",
		ByAzure:    true,
		BaseURL:    "https://{YOUR_RESOURCE_NAME}.openai.azure.com",
		APIVersion: "2024-02-01",
		Model:      "text-embedding-3-small",
		Timeout:    30 * time.Second,
	})
	if err != nil {
		log.Fatalf("NewEmbedder of openai error: %v", err)
		return
	}

	vectors, err := embedder.EmbedStrings(ctx, []string{"hello", "how are you"})
	if err != nil {
		log.Fatalf("EmbedStrings of OpenAI failed, err=%v", err)
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
    // Optional. Default: no timeout
    Timeout time.Duration `json:"timeout"`

    // HTTPClient specifies the client to send HTTP requests.
    // If HTTPClient is set, Timeout will not be used.
    // Optional. Default &http.Client{Timeout: Timeout}
    HTTPClient *http.Client `json:"http_client"`

    // APIKey is your authentication key
    // Use OpenAI API key or Azure API key depending on the service
    // Required
    APIKey string `json:"api_key"`

    // ByAzure indicates whether to use Azure OpenAI Service
    // Optional. Default: false
    ByAzure bool `json:"by_azure"`

    // BaseURL is the Azure OpenAI endpoint URL (only for Azure)
    // Format: https://{YOUR_RESOURCE_NAME}.openai.azure.com
    // Required for Azure
    BaseURL string `json:"base_url"`

    // APIVersion specifies the Azure OpenAI API version (only for Azure)
    // Required for Azure
    APIVersion string `json:"api_version"`

    // Model specifies the ID of the model to use for embedding generation
    // Required
    Model string `json:"model"`

    // EncodingFormat specifies the format of the embeddings output
    // Optional. Default: EmbeddingEncodingFormatFloat
    EncodingFormat *EmbeddingEncodingFormat `json:"encoding_format,omitempty"`

    // Dimensions specifies the number of dimensions the resulting output embeddings should have
    // Only supported in text-embedding-3 and later models
    // Optional
    Dimensions *int `json:"dimensions,omitempty"`

    // User is a unique identifier representing your end-user
    // Optional. Helps OpenAI monitor and detect abuse
    User *string `json:"user,omitempty"`
}
```

### Encoding Formats

The embedder supports two encoding formats:

- `EmbeddingEncodingFormatFloat`: Returns embeddings as floating-point arrays (default)
- `EmbeddingEncodingFormatBase64`: Returns embeddings as base64-encoded strings

## Available Models

OpenAI supports several embedding models:

- `text-embedding-3-small`: Latest small embedding model with improved performance
- `text-embedding-3-large`: Latest large embedding model with highest quality
- `text-embedding-ada-002`: Previous generation embedding model

The `text-embedding-3-small` and `text-embedding-3-large` models support the `Dimensions` parameter for configurable output dimensions.

## Azure OpenAI Service

To use Azure OpenAI Service, you need to:

1. Set `ByAzure` to `true`
2. Provide your Azure resource URL in `BaseURL` (format: `https://{YOUR_RESOURCE_NAME}.openai.azure.com`)
3. Specify the API version in `APIVersion`
4. Use your Azure API key in `APIKey`

For more details about Azure OpenAI Service, see the [Azure documentation](https://learn.microsoft.com/en-us/azure/ai-services/openai/).

## Examples

See the following examples for more usage:

- [Text Embedding](./examples/embedding/)

## API Reference

For more details about OpenAI's embedding API, please refer to:
- [OpenAI Embeddings API Documentation](https://platform.openai.com/docs/api-reference/embeddings/create)

## License

This component is licensed under the Apache License 2.0. See the LICENSE file for details.
