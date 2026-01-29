# ES7 Indexer

English | [中文](README_zh.md)

An Elasticsearch 7.x indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface.

## Features

- Implements `github.com/cloudwego/eino/components/indexer.Indexer`
- Easy integration with Eino's indexer system
- Configurable Elasticsearch parameters
- Support for vector similarity search
- Bulk indexing operations
- Custom field mapping support
- Flexible document vectorization

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/es7@latest
```

## Quick Start

Here's a quick example of how to use the indexer:

```go
import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v7"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/es7"
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

	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	// 1. Create ES client
	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  username,
		Password:  password,
	})

	// 2. Define Index Specification (Optional: automatically creates index if it doesn't exist)
	indexSpec := &es7.IndexSpec{
		Settings: map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				fieldContentVector: map[string]any{
					"type": "dense_vector",
					"dims": 1536,
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

	// 4. Prepare documents
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

	// 5. Create ES indexer component
	indexer, _ := es7.NewIndexer(ctx, &es7.IndexerConfig{
		Client:    client,
		Index:     indexName,
		IndexSpec: indexSpec, // Add this to enable automatic index creation
		BatchSize: 10,
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]es7.FieldValue, err error) {
			return map[string]es7.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentVector, // vectorize doc content and save vector to field "content_vector"
				},
				fieldExtraLocation: {
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		Embedding: emb,
	})

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
    Client *elasticsearch.Client // Required: Elasticsearch client instance
    Index  string                // Required: Index name to store documents
    IndexSpec *IndexSpec         // Optional: Settings and mappings for automatic index creation.
                                 // If provided, the indexer will check if the index exists during initialization (NewIndexer).
                                 // If it doesn't exist, it will be created with the provided specification.
                                 // If it already exists, no action is taken.
    BatchSize int                // Optional: Max texts size for embedding (default: 5)

    // Required: Function to map Document fields to Elasticsearch fields
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
- [Elasticsearch Go Client Documentation](https://github.com/elastic/go-elasticsearch)
