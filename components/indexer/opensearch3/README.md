# OpenSearch Indexer

English | [简体中文](README_zh.md)

An OpenSearch indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This enables seamless integration with Eino's vector storage and retrieval system for enhanced semantic search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/indexer.Indexer`
- Easy integration with Eino's indexer system
- Configurable OpenSearch parameters
- Support for vector similarity search
- Bulk indexing operations
- Custom field mapping support
- Flexible document vectorization

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/opensearch3@latest
```

## Quick Start

Here's a quick example of how to use the indexer, you could read components/indexer/opensearch3/examples/indexer/main.go for more details:

```go
package main

import (
	"context"
	"fmt"
	"log"
	
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/cloudwego/eino-ext/components/indexer/opensearch3"
)

func main() {
	ctx := context.Background()

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{"http://localhost:9200"},
			// ... auth config
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// create embedding component
	emb := createYourEmbedding()

	// create opensearch indexer component
	indexer, _ := opensearch3.NewIndexer(ctx, &opensearch3.IndexerConfig{
		Client:    client,
		Index:     "your_index_name",
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]opensearch3.FieldValue, error) {
			return map[string]opensearch3.FieldValue{
				"content": {
					Value:    doc.Content,
					EmbedKey: "content_vector",
				},
			}, nil
		},
		Embedding: emb,
	})

	docs := []*schema.Document{
		{ID: "1", Content: "example content"},
	}

	ids, _ := indexer.Store(ctx, docs)
	fmt.Println(ids)
}
```

## Configuration

The indexer can be configured using the `IndexerConfig` struct:

```go
type IndexerConfig struct {
    Client *opensearchapi.Client // Required: OpenSearch client instance
    Index  string             // Required: Index name to store documents
    BatchSize int             // Optional: Max texts size for embedding (default: 5)

    // Required: Function to map Document fields to OpenSearch fields
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // Optional: Required only if vectorization is needed
    Embedding embedding.Embedder
}

// FieldValue defines how a field should be stored and vectorized
type FieldValue struct {
    Value     any    // Original value to store
    EmbedKey  string // If set, Value will be vectorized and saved
    Stringify func(val any) (string, error) // Optional: custom string conversion
}
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [OpenSearch Go Client Documentation](https://github.com/opensearch-project/opensearch-go)
