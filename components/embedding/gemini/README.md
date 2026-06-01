# Gemini Embedding for Eino

English | [简体中文](./README_zh.md)

## Introduction

This is a Gemini Embedding component implementation for [Eino](https://github.com/cloudwego/eino), which implements the `Embedder` interface and seamlessly integrates into Eino's embedding system to provide text vectorization capabilities using Google's Gemini embedding models.

## Features

- Implements `github.com/cloudwego/eino/components/embedding.Embedder` interface
- Easy integration into Eino workflows
- Support for multiple Gemini embedding models
- Configurable task types and output dimensions
- Built-in callback support in Eino
- Support for auto-truncation and custom MIME types

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/embedding/gemini
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/gemini"
	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()

	cli, err := genai.NewClient(ctx, &genai.ClientConfig{})
	if err != nil {
		log.Fatal("create genai client error: ", err)
	}

	embedder, err := gemini.NewEmbedder(ctx, &gemini.EmbeddingConfig{
		Client:   cli,
		Model:    "gemini-embedding-001",
		TaskType: "RETRIEVAL_QUERY",
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return
	}

	embedding, err := embedder.EmbedStrings(ctx, []string{"hello world", "你好世界"})
	if err != nil {
		log.Printf("embedding error: %v\n", err)
		return
	}

	log.Printf("embedding: %v\n", embedding)
}
```

## Configuration

The embedder can be configured through the `EmbeddingConfig` structure:

```go
type EmbeddingConfig struct {
    // Client is the Gemini API client instance
    // Required for making API calls to Gemini
    Client *genai.Client

    // Model specifies which Gemini embedding model to use
    // Examples: "gemini-embedding-001", "text-embedding-004"
    // Required
    Model string

    // TaskType specifies the type of task for which the embedding will be used
    // Optional
    TaskType string

    // Title for the text
    // Only applicable when TaskType is RETRIEVAL_DOCUMENT
    // Optional
    Title string

    // OutputDimensionality specifies reduced dimension for the output embedding
    // If set, excessive values in the output embedding are truncated from the end
    // Supported by newer models since 2024 only
    // You cannot set this value if using the earlier model (models/embedding-001)
    // Optional
    OutputDimensionality *int32

    // MIMEType specifies the MIME type of the input (Vertex API only)
    // Optional
    MIMEType string

    // AutoTruncate determines whether to silently truncate inputs longer than the max sequence length
    // If set to false, oversized inputs will lead to an INVALID_ARGUMENT error
    // Vertex API only
    // Optional. Default: false
    AutoTruncate bool
}
```

## Task Types

The `TaskType` parameter allows you to specify how the embeddings will be used:

- `RETRIEVAL_QUERY`: Use this when the embedding will be used for search/retrieval queries
- `RETRIEVAL_DOCUMENT`: Use this when the embedding will be used for documents in a corpus
- `SEMANTIC_SIMILARITY`: Use this for semantic similarity tasks
- `CLASSIFICATION`: Use this for classification tasks
- `CLUSTERING`: Use this for clustering tasks

## Available Models

Gemini supports several embedding models:

- `gemini-embedding-001`: Earlier generation embedding model
- `text-embedding-004`: Newer embedding model with support for output dimensionality configuration

Note: The `OutputDimensionality` parameter is only supported by newer models (2024 and later).

## Authentication

To use the Gemini API, you need to set up authentication:

1. Set the `GOOGLE_API_KEY` or `GEMINI_API_KEY` environment variable with your API key
2. Create a Gemini client using `genai.NewClient(ctx, &genai.ClientConfig{})`
3. Pass the client to the embedder configuration

For more details about Gemini API authentication, please refer to the [Google AI documentation](https://ai.google.dev/docs).

## Examples

See the [examples](./examples/) directory for complete usage examples.

## License

This component is licensed under the Apache License 2.0. See the LICENSE file for details.
