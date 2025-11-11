# Milvus Indexer

## Overview

Milvus Indexer is a vector database indexing component based on the CloudWeGo Eino framework. It supports embedding text
documents into vectors and storing them in Milvus, enabling efficient vector search and semantic retrieval scenarios.

## Version Requirements

- Milvus client library: github.com/milvus-io/milvus/client/v2 v2.6.x

## Installation

Add the dependency in your go.mod:

```shell
go get -u github.com/milvus-io/milvus/client/v2
go get -u github.com/cloudwego/eino-ext/components/indexer/milvus
```

## Quick Start

Refer to `examples/main.go` to create a Milvus client and Indexer:

```go
package main

import (
	"context"
	
	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func main() {
	ctx := context.Background()
	client, _ := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: "your-milvus-address",
		APIKey:  "your-api-key",
	})
	indexer, _ := milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:    client,
		Dim:       2560,
		Embedding: yourEmbedder, // Implements embedding.Embedder interface
	})
	// Store documents
	ids, err := indexer.Store(ctx, docs)
}
```

## Configuration

Key parameters in `IndexerConfig`:

- `Client`: Milvus client instance (required)
- `Collection`: Collection name (auto-created by default)
- `Partition`: Partition name (optional)
- `Schema`: Collection schema (auto-inferred or custom)
- `Dim`: Vector dimension (required)
- `Embedder`: Embedder, implements embedding.Embedder interface
- `DocumentConverter`: Document converter (default implementation provided)