# Valkey Indexer

A Valkey indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This component uses Valkey Hashes to store documents with vector embeddings via the [valkey-glide](https://github.com/valkey-io/valkey-glide) client, enabling vector similarity search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/indexer.Indexer`
- Uses Valkey Hashes (HSET) or JSON (JSON.SET) for document storage
- Pipeline batch execution for high throughput
- Automatic embedding generation with configurable batch size
- Custom field mapping via `DocumentToHashes` or `DocumentToJSON` function
- Configurable key prefix for index partitioning
- Full Eino callback integration (OnStart, OnEnd, OnError)

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/valkey@latest
```

## Prerequisites

- Valkey 9.1+ with the Search module (e.g., `valkey/valkey-bundle`)
- [valkey-glide](https://github.com/valkey-io/valkey-glide) Go client (requires CGO and Rust core library)

## Quick Start

```go
import (
	"context"
	"fmt"

	glide "github.com/valkey-io/valkey-glide/go/v2"
	"github.com/valkey-io/valkey-glide/go/v2/config"
	"github.com/cloudwego/eino/schema"

	valkeyIndexer "github.com/cloudwego/eino-ext/components/indexer/valkey"
)

func main() {
	ctx := context.Background()

	// 1. Create Valkey GLIDE client
	cfg := config.NewClientConfiguration().
		WithAddress(&config.NodeAddress{Host: "localhost", Port: 6379})
	client, _ := glide.NewClient(cfg)

	// 2. Create embedding component (use your preferred embedder)
	emb := yourEmbedder()

	// 3. Create Valkey indexer
	indexer, _ := valkeyIndexer.NewIndexer(ctx, &valkeyIndexer.IndexerConfig{
		Client:    client,
		KeyPrefix: "doc:",
		BatchSize: 10,
		Embedding: emb,
	})

	// 4. Store documents
	docs := []*schema.Document{
		{ID: "1", Content: "Valkey is a high-performance key-value store."},
		{ID: "2", Content: "Vector search enables semantic similarity queries."},
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		fmt.Printf("store error: %v\n", err)
		return
	}
	fmt.Printf("stored document IDs: %v\n", ids)
}
```

## Configuration

```go
type IndexerConfig struct {
    // Required: Valkey GLIDE client (must implement Exec for pipeline batching)
    Client BatchClient

    // Optional: Key prefix prepended to each hash key
    // Should match the prefix used in FT.CREATE
    KeyPrefix string

    // Optional: Storage format - DocumentTypeHash (default) or DocumentTypeJSON
    DocumentType DocumentType

    // Optional: Custom document-to-hash conversion (Hash mode only)
    // Default: stores Content with embedding, plus all MetaData fields
    DocumentToHashes func(ctx context.Context, doc *schema.Document) (*Hashes, error)

    // Optional: Custom document-to-JSON conversion (JSON mode only)
    // Default: stores content, vector, and metadata fields
    DocumentToJSON func(ctx context.Context, doc *schema.Document, vector []float64) (map[string]any, error)

    // Optional: Max texts per embedding batch call (default: 10)
    BatchSize int

    // Required: Embedding method for vectorizing document content
    Embedding embedding.Embedder
}
```

## JSON Document Storage

For JSON storage, use `DocumentTypeJSON`. This requires the Valkey JSON module and an index created with `ON JSON`:

```go
indexer, _ := valkeyIndexer.NewIndexer(ctx, &valkeyIndexer.IndexerConfig{
    Client:       client,
    KeyPrefix:    "jdoc:",
    DocumentType: valkeyIndexer.DocumentTypeJSON,
    Embedding:    emb,
})
```

Create the index with JSONPath field identifiers:
```
FT.CREATE my_json_index ON JSON PREFIX 1 jdoc: SCHEMA
  $.content AS content TEXT
  $.vector_content AS vector_content VECTOR HNSW 6 TYPE FLOAT32 DIM 1024 DISTANCE_METRIC COSINE
```

## Custom Field Mapping

```go
indexer, _ := valkeyIndexer.NewIndexer(ctx, &valkeyIndexer.IndexerConfig{
    Client:    client,
    KeyPrefix: "doc:",
    DocumentToHashes: func(ctx context.Context, doc *schema.Document) (*valkeyIndexer.Hashes, error) {
        return &valkeyIndexer.Hashes{
            Key: doc.ID,
            Field2Value: map[string]valkeyIndexer.FieldValue{
                "title": {Value: doc.MetaData["title"]},
                "body":  {Value: doc.Content, EmbedKey: "body_vector"},
            },
        }, nil
    },
    Embedding: emb,
})
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Valkey GLIDE Go Client](https://github.com/valkey-io/valkey-glide)
- [Valkey Search](https://valkey.io/topics/search/)
