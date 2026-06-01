# VikingDB Retriever

English | [中文](README_zh.md)

A VikingDB retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever` interface. This component integrates with Volcengine VikingDB service to retrieve documents based on semantic similarity.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever`
- Easy integration with Eino's retriever system
- Supports Volcengine VikingDB indexes
- Multiple embedding modes:
  - Built-in VikingDB embedding (Embedding V2)
  - Custom embedding with your own embedder
  - Multi-modal search for platform-based vectorization
- Dense and sparse vector search
- Configurable top-k results
- Score threshold filtering
- DSL-based filtering support
- Partition (sub-index) support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb@latest
```

## Quick Start

### Using Built-in VikingDB Embedding

```go
import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// Create VikingDB retriever with built-in embedding
	retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		Index:      "your_index_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin:  true,
			ModelName:   "bge-large-zh",
			UseSparse:   false,
			DenseWeight: 0.5,  // For hybrid search
		},
		TopK: ptrOf(10),
	})

	// Retrieve documents
	docs, err := retriever.Retrieve(ctx, "search query")
	if err != nil {
		fmt.Printf("retrieve error: %v\n", err)
		return
	}

	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s, Score: %f\n", 
			doc.ID, doc.Content, doc.Score())
	}
}

func ptrOf[T any](v T) *T {
	return &v
}
```

### Using Custom Embedding

```go
import (
	"context"
	"os"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb"
)

func main() {
	ctx := context.Background()

	// Create custom embedding component
	emb, _ := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Region: os.Getenv("ARK_REGION"),
		Model:  os.Getenv("ARK_MODEL"),
	})

	// Create VikingDB retriever with custom embedding
	retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
		Host:       "api-vikingdb.volces.com",
		Region:     "cn-beijing",
		AK:         "your-access-key",
		SK:         "your-secret-key",
		Scheme:     "https",
		Collection: "your_collection_name",
		Index:      "your_index_name",
		EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
			UseBuiltin: false,
			Embedding:  emb,
		},
	})

	docs, _ := retriever.Retrieve(ctx, "search query")
	for _, doc := range docs {
		fmt.Printf("ID: %s, Content: %s\n", doc.ID, doc.Content)
	}
}
```

### Using Multi-Modal Search

For datasets with platform-based vectorization:

```go
retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	Host:           "api-vikingdb.volces.com",
	Region:         "cn-beijing",
	AK:             "your-access-key",
	SK:             "your-secret-key",
	Collection:     "your_collection_name",
	Index:          "your_index_name",
	WithMultiModal: true,  // Enable multi-modal search
})

// Search with text query directly
docs, _ := retriever.Retrieve(ctx, "search query")
```

## Configuration

```go
type RetrieverConfig struct {
    // VikingDB connection settings
    Host              string  // Required: VikingDB service host
    Region            string  // Required: Service region (e.g., "cn-beijing")
    AK                string  // Required: Access Key
    SK                string  // Required: Secret Key
    Scheme            string  // Optional: "http" or "https" (default: "https")
    ConnectionTimeout int64   // Optional: Connection timeout in seconds

    // Collection and index settings
    Collection string  // Required: Collection name
    Index      string  // Required: Index name

    // Multi-modal support
    // Set to true if the dataset is vectorized on the platform
    // When true, no need to configure EmbeddingConfig
    WithMultiModal bool

    // Embedding configuration
    EmbeddingConfig EmbeddingConfig

    // Optional: Partition (sub-index) field (default: "default")
    Partition string

    // Optional: Number of results to return (default: 100)
    TopK *int

    // Optional: Minimum score threshold for results
    ScoreThreshold *float64

    // Optional: DSL filter expression
    // See: https://www.volcengine.com/docs/84313/1254609
    FilterDSL map[string]any
}

type EmbeddingConfig struct {
    // UseBuiltin determines whether to use VikingDB built-in embedding (Embedding V2)
    // When true, configure ModelName and UseSparse
    // When false, configure Embedding
    // See: https://www.volcengine.com/docs/84313/1254568
    UseBuiltin bool

    // ModelName specifies the built-in model name
    ModelName string

    // UseSparse determines whether to return sparse vectors
    // For hybrid index search with sparse vectors
    UseSparse bool

    // DenseWeight controls dense vector weight in hybrid search
    // Range: [0.2, 1.0], default: 0.5
    // Only effective for hybrid indexes
    DenseWeight float64

    // Embedding is used when UseBuiltin is false
    Embedding embedding.Embedder
}
```

## Advanced Features

### Score Threshold Filtering

Filter results by minimum score:

```go
threshold := 0.8

retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... other configs
	ScoreThreshold: &threshold,
})

docs, _ := retriever.Retrieve(ctx, "search query")
```

### DSL Filtering

Use DSL expressions for advanced filtering:

```go
import "github.com/cloudwego/eino-ext/components/retriever/volc_vikingdb"

filterDSL := map[string]any{
	"op": "and",
	"conditions": []map[string]any{
		{
			"op":    "range",
			"field": "price",
			"gte":   100,
			"lt":    500,
		},
		{
			"op":    "term",
			"field": "category",
			"value": "electronics",
		},
	},
}

retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... other configs
	FilterDSL: filterDSL,
})

docs, _ := retriever.Retrieve(ctx, "search query")
```

### Partition Support

Search within specific partitions (sub-indexes):

```go
retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... other configs
	Partition: "partition_2023",
})
```

### Hybrid Search with Sparse Vectors

For indexes supporting sparse vectors:

```go
retriever, _ := volc_vikingdb.NewRetriever(ctx, &volc_vikingdb.RetrieverConfig{
	// ... other configs
	EmbeddingConfig: volc_vikingdb.EmbeddingConfig{
		UseBuiltin:  true,
		ModelName:   "model-with-sparse-support",
		UseSparse:   true,
		DenseWeight: 0.7,  // 70% weight on dense vectors
	},
})
```

## Runtime Options

Override configuration at retrieval time:

```go
import "github.com/cloudwego/eino/components/retriever"

docs, _ := retriever.Retrieve(ctx, "search query",
	retriever.WithTopK(20),
	retriever.WithScoreThreshold(0.9),
)
```

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [VikingDB Documentation](https://www.volcengine.com/docs/84313/1254568)
- [VikingDB DSL Filtering](https://www.volcengine.com/docs/84313/1254609)
- [VikingDB Go SDK](https://github.com/volcengine/volc-sdk-golang)
## Examples

See the following examples for more usage:

- [Embedding Retriever](./examples/embed_retriever/)
- [Basic Retriever](./examples/retriever/)

