# OpenSearch 3 Indexer

English | [简体中文](README_zh.md)

An OpenSearch 3 indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This enables seamless integration with Eino's vector storage and retrieval system for enhanced semantic search capabilities.

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
	"os"
	
	"github.com/cloudwego/eino/schema"
	opensearch "github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/opensearch3"
)

const (
	indexName          = "eino_example"
	fieldContent       = "content"
	fieldContentVector = "content_vector"
	fieldExtraLocation = "location"
	docExtraLocation   = "location"
)

func main() {
	ctx := context.Background()
	username := os.Getenv("OPENSEARCH_USERNAME")
	password := os.Getenv("OPENSEARCH_PASSWORD")

	// 1. Create OpenSearch client
	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{"http://localhost:9200"},
			Username:  username,
			Password:  password,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. Define Index Specification (Optional: automatically creates index if it doesn't exist)
	indexSpec := &opensearch3.IndexSpec{
		Settings: map[string]any{
			"number_of_shards": 1,
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				fieldContentVector: map[string]any{
					"type":      "knn_vector",
					"dimension": 1536,
					"method": map[string]any{
						"name":       "hnsw",
						"engine":     "nmslib",
						"space_type": "l2",
					},
				},
			},
		},
	}

	// 3. Create embedding component using ARK
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 4. Create opensearch indexer component
	indexer, _ := opensearch3.NewIndexer(ctx, &opensearch3.IndexerConfig{
		Client:    client,
		Index:     indexName,
		IndexSpec: indexSpec, // Add this to enable automatic index creation
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]opensearch3.FieldValue, error) {
			return map[string]opensearch3.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentVector, // vectorize content and save to "content_vector"
				},
				fieldExtraLocation: {
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		Embedding: emb,
	})

	// 5. Prepare documents
	// Documents usually contain at least an ID and Content.
	// You can also add extra metadata for filtering or other purposes.
	docs := []*schema.Document{
		{
			ID:      "1",
			Content: "Eiffel Tower: Located in Paris, France.",
			MetaData: map[string]any{
				docExtraLocation: "France",
			},
		},
		{
			ID:      "2",
			Content: "The Great Wall: Located in China.",
			MetaData: map[string]any{
				docExtraLocation: "China",
			},
		},
	}

	// 6. Index documents
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		fmt.Printf("index error: %v\n", err)
		return
	}
	fmt.Println("indexed ids:", ids)
}
```

## Configuration

The indexer can be configured using the `IndexerConfig` struct:

```go
type IndexerConfig struct {
    Client *opensearchapi.Client // Required: OpenSearch client instance
    Index  string             // Required: Index name to store documents
    IndexSpec *IndexSpec       // Optional: Settings and mappings for automatic index creation.
                               // If provided, the indexer will check if the index exists during initialization (NewIndexer).
                               // If it doesn't exist, it will be created with the provided specification.
                               // If it already exists, no action is taken.
    BatchSize int             // Optional: Max texts size for embedding (default: 5)

    // Required: Function to map Document fields to OpenSearch fields
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)

    // Optional: Required only if vectorization is needed
    Embedding embedding.Embedder
}

// IndexSpec defines the settings and mappings for the index
type IndexSpec struct {
    Settings map[string]any `json:"settings,omitempty"`
    Mappings map[string]any `json:"mappings,omitempty"`
    Aliases  map[string]any `json:"aliases,omitempty"`
}

// FieldValue defines how a field should be stored and vectorized
type FieldValue struct {
    Value     any    // Original value to store
    EmbedKey  string // If set, Value will be vectorized and saved
    Stringify func(val any) (string, error) // Optional: custom string conversion
}
```

## Full Examples

- [Indexer Example](./examples/indexer)

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [OpenSearch Go Client Documentation](https://github.com/opensearch-project/opensearch-go)
