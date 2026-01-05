# ES9 Indexer

English | [中文](README_zh.md)

An Elasticsearch 9.x indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This enables seamless integration with Eino's vector storage and retrieval system for enhanced semantic search capabilities.

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
go get github.com/cloudwego/eino-ext/components/indexer/es9@latest
```

## Quick Start

Here's a quick example of how to use the indexer, you could read components/indexer/es9/examples/indexer/add_documents.go for more details:

```go
import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v9"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/es9"
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
	// es supports multiple ways to connect
	username := os.Getenv("ES_USERNAME")
	password := os.Getenv("ES_PASSWORD")

	// 1. Create ES client
	httpCACertPath := os.Getenv("ES_HTTP_CA_CERT_PATH")
	var cert []byte
	var err error
	if httpCACertPath != "" {
		cert, err = os.ReadFile(httpCACertPath)
		if err != nil {
			log.Fatalf("read file failed, err=%v", err)
		}
	}

	client, _ := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"https://localhost:9200"},
		Username:  username,
		Password:  password,
		CACert:    cert,
	})

	// 2. Create embedding component using ARK
	// Replace "ARK_API_KEY", "ARK_REGION", "ARK_MODEL" with your actual config
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 3. Prepare documents
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

	// 4. Create ES indexer component
	indexer, _ := es9.NewIndexer(ctx, &es9.IndexerConfig{
		Client:    client,
		Index:     indexName,
		BatchSize: 10,
		// DocumentToFields specifies how to map document fields to ES fields
		DocumentToFields: func(ctx context.Context, doc *schema.Document) (field2Value map[string]es9.FieldValue, err error) {
			return map[string]es9.FieldValue{
				fieldContent: {
					Value:    doc.Content,
					EmbedKey: fieldContentVector, // vectorize content and save to "content_vector"
				},
				fieldExtraLocation: {
					// Extra metadata field
					Value: doc.MetaData[docExtraLocation],
				},
			}, nil
		},
		// Provide the embedding component to use for vectorization
		Embedding: emb,
	})

	// 5. Index documents
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
    BatchSize int                // Optional: Max texts size for embedding (default: 5)

    // Required: Function to map Document fields to Elasticsearch fields
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

## Full Examples

- [Indexer Example](./examples/indexer)
- [Indexer with Sparse Vector Example](./examples/indexer_with_sparse_vector)

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Elasticsearch Go Client Documentation](https://github.com/elastic/go-elasticsearch)
