# VikingDB Indexer

English | [中文](README_zh.md)

A VikingDB indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This component integrates with Volcengine VikingDB service to store documents with vector embeddings.

## Features

- Implements `github.com/cloudwego/eino/components/indexer.Indexer`
- Easy integration with Eino's indexer system
- Supports Volcengine VikingDB collections
- Multiple embedding modes:
  - Built-in VikingDB embedding (Embedding V2)
  - Custom embedding with your own embedder
  - Multi-modal support for platform-based vectorization
- Dense and sparse vector support
- Batch upsert operations for better performance
- Configurable TTL and custom fields support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb@latest
```

## Quick Start

### Using Built-in VikingDB Embedding

```go
import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// Create VikingDB indexer with built-in embedding
	indexer, _ := volc_vikingdb.NewIndexer(ctx, &volc_vikingdb.IndexerConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin: true,
			ModelName:  "bge-large-zh",  // Built-in embedding model
			UseSparse:  false,            // Set to true for sparse vectors
		},
		AddBatchSize: 5,
	})

	// Prepare documents
	docs := []*schema.Document{
		{
			ID:      "1",
			Content: "Eiffel Tower: Located in Paris, France.",
		},
		{
			ID:      "2",
			Content: "The Great Wall: Located in China.",
		},
	}

	// Index documents
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		fmt.Printf("index error: %v\n", err)
		return
	}
	fmt.Println("indexed ids:", ids)
}
```

### Using Custom Embedding

```go
import (
	"context"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// Create custom embedding component
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// Create VikingDB indexer with custom embedding
	indexer, _ := volc_vikingdb.NewIndexer(ctx, &volc_vikingdb.IndexerConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin: false,
			Embedding:  emb,  // Use custom embedding
		},
		AddBatchSize: 5,
	})

	docs := []*schema.Document{
		{ID: "1", Content: "Sample document"},
	}

	ids, _ := indexer.Store(ctx, docs)
	fmt.Println("indexed ids:", ids)
}
```

## Configuration

The indexer can be configured using the `IndexerConfig` struct:

```go
type IndexerConfig struct {
    // VikingDB connection settings
    Host              string  // Required: VikingDB service host
    Region            string  // Required: Service region (e.g., "cn-beijing")
    AK                string  // Required: Access Key
    SK                string  // Required: Secret Key
    Scheme            string  // Optional: "http" or "https" (default: "https")
    ConnectionTimeout int64   // Optional: Connection timeout in seconds

    // Collection settings
    Collection string  // Required: Collection name

    // Multi-modal support
    // Set to true if the dataset is vectorized on the platform
    // When true, no need to configure EmbeddingConfig
    WithMultiModal bool

    // Embedding configuration
    EmbeddingConfig EmbeddingConfig

    // Optional: Batch size for upsert operations (default: 5)
    AddBatchSize int
}

type EmbeddingConfig struct {
    // UseBuiltin determines whether to use VikingDB built-in embedding (Embedding V2)
    // When true, configure ModelName and UseSparse
    // When false, configure Embedding
    // See: https://www.volcengine.com/docs/84313/1254617
    UseBuiltin bool

    // ModelName specifies the built-in model name
    // Examples: "bge-large-zh", "text-embedding-ada-002"
    ModelName string

    // UseSparse determines whether to return sparse vectors
    // Models supporting sparse vectors: set true for dense+sparse, false for dense only
    // Models not supporting sparse vectors: setting true will cause an error
    UseSparse bool

    // Embedding is used when UseBuiltin is false
    // If provided (from here or indexer.Option), it takes precedence over built-in methods
    Embedding embedding.Embedder
}
```

## Advanced Features

### Custom Fields and TTL

You can add custom fields and TTL to documents using metadata:

```go
import "github.com/cloudwego/eino-ext/components/indexer/volc_vikingdb"

doc := &schema.Document{
    ID:      "1",
    Content: "Sample content",
    MetaData: map[string]any{
        volc_vikingdb.ExtraKeyVikingDBFields: map[string]any{
            "category": "technology",
            "author":   "John Doe",
        },
        volc_vikingdb.ExtraKeyVikingDBTTL: int64(86400), // 24 hours in seconds
    },
}

ids, _ := indexer.Store(ctx, []*schema.Document{doc})
```

### Dense and Sparse Vectors

When using built-in embedding with models that support sparse vectors:

```go
indexer, _ := volc_vikingdb.NewIndexer(ctx, &volc_vikingdb.IndexerConfig{
    // ... other configs
    EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
        UseBuiltin: true,
        ModelName:  "model-with-sparse-support",
        UseSparse:  true,  // Enable sparse vector extraction
    },
})
```

## Storage Format

Documents are stored in VikingDB with the following fields:
- `id`: Document ID
- `content`: Document content
- `vector`: Dense vector embedding
- `sparse_vector`: Sparse vector (if enabled)
- Additional custom fields from metadata

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [VikingDB Documentation](https://www.volcengine.com/docs/84313/1254617)
- [VikingDB Go SDK](https://github.com/volcengine/volc-sdk-golang)
## Examples

See the following examples for more usage:

- [Embedding Indexer](./examples/embed_indexer/)
- [Basic Indexer](./examples/indexer/)

