# Redis Indexer

English | [中文](README_zh.md)

A Redis indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This component uses Redis Hashes to store documents with vector embeddings, enabling vector similarity search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/indexer.Indexer`
- Easy integration with Eino's indexer system
- Uses Redis Hashes for data storage
- Supports document vectorization with configurable embedding
- Batch embedding operations for better performance
- Custom field mapping support
- Flexible document to hash conversion

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/redis@latest
```

## Quick Start

Here's a quick example of how to use the indexer:

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/redis"
)

func main() {
	ctx := context.Background()

	// 1. Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// 2. Create embedding component
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// 3. Prepare documents
	docs := []*schema.Document{
		{
			ID:      "1",
			Content: "Eiffel Tower: Located in Paris, France.",
			MetaData: map[string]any{
				"location": "France",
			},
		},
		{
			ID:      "2",
			Content: "The Great Wall: Located in China.",
			MetaData: map[string]any{
				"location": "China",
			},
		},
	}

	// 4. Create Redis indexer
	indexer, _ := redis.NewIndexer(ctx, &redis.IndexerConfig{
		Client:    client,
		KeyPrefix: "doc:",
		BatchSize: 10,
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
    // Required: Redis client instance
    Client *redis.Client

    // Optional: Prefix for each key, hset key would be KeyPrefix+Hashes.Key
    // If not set, make sure each key from DocumentToHashes contains same prefix
    KeyPrefix string

    // Optional: Customize key, field and value for redis hash
    // Default: defaultDocumentToFields (uses doc.ID as key, content as field, and vectorizes content)
    DocumentToHashes func(ctx context.Context, doc *schema.Document) (*Hashes, error)

    // Optional: Max texts size for batch embedding (default: 10)
    BatchSize int

    // Required: Embedding method for text vectorization
    Embedding embedding.Embedder
}

// Hashes defines the structure for Redis hash storage
type Hashes struct {
    Key         string                   // Redis hash key
    Field2Value map[string]FieldValue    // Field to value mappings
}

// FieldValue defines how a field should be stored and vectorized
type FieldValue struct {
    Value     any                             // Original value to store
    EmbedKey  string                          // If set, Value will be vectorized and stored under this key
    Stringify func(val any) (string, error)  // Optional: custom string conversion for embedding
}
```

## Custom Document Mapping

You can customize how documents are mapped to Redis hashes:

```go
indexer, _ := redis.NewIndexer(ctx, &redis.IndexerConfig{
    Client:    client,
    KeyPrefix: "doc:",
    DocumentToHashes: func(ctx context.Context, doc *schema.Document) (*redis.Hashes, error) {
        return &redis.Hashes{
            Key: doc.ID,
            Field2Value: map[string]redis.FieldValue{
                "content": {
                    Value:    doc.Content,
                    EmbedKey: "content_vector",  // Vectorize content and save to "content_vector"
                },
                "title": {
                    Value:    doc.MetaData["title"],
                    EmbedKey: "title_vector",    // Vectorize title and save to "title_vector"
                },
                "category": {
                    Value: doc.MetaData["category"],  // Store category without vectorization
                },
            },
        }, nil
    },
    BatchSize: 10,
    Embedding: emb,
})
```

## How It Works

1. **Document Processing**: The indexer converts documents to Redis hash structures using `DocumentToHashes`
2. **Batch Embedding**: Texts marked for vectorization (with `EmbedKey` set) are batched and embedded together
3. **Pipeline Execution**: Uses Redis pipeline for efficient bulk insertion
4. **Storage Format**: Each document is stored as a Redis hash with the pattern `KeyPrefix+key`

## Default Behavior

By default (when `DocumentToHashes` is not provided):
- Uses `doc.ID` as the hash key
- Stores `doc.Content` in the "content" field
- Vectorizes content and stores in "content_vector" field
- Includes all `doc.MetaData` fields as-is

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Redis Go Client Documentation](https://github.com/redis/go-redis)
- [Redis Vector Search](https://redis.io/docs/latest/develop/interact/search-and-query/advanced-concepts/vectors/)
## Examples

See the following examples for more usage:

- [Customized Indexer](./examples/customized_indexer/)
- [Default Indexer](./examples/default_indexer/)

