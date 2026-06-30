# Pinecone Indexer

English | [简体中文](README_zh.md)

A Pinecone indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This enables seamless integration with Eino's vector storage and retrieval system for enhanced semantic search capabilities.

## Quick Start

### Installation

It requires the go-pinecone client of version v3.x:

```bash
go get github.com/pinecone-io/go-pinecone/v3@latest
go get github.com/cloudwego/eino-ext/components/indexer/pinecone@latest
```

### Create the Pinecone Indexer

```go
package main

import (
	"context"
	"log"
	"os"

	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
	"github.com/cloudwego/eino-ext/components/indexer/pinecone"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

func main() {
	// Load configuration from environment variables
	apiKey := os.Getenv("PINECONE_APIKEY")
	if apiKey == "" {
		log.Fatal("PINECONE_APIKEY environment variable is required")
	}

	// Initialize Pinecone client
	client, err := pc.NewClient(pc.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to create Pinecone client: %v", err)
	}

	// Create Pinecone indexer config
	config := pinecone.IndexerConfig{
		Client:    client,
		Dimension: 2560, // Set according to your embedding model
		Embedding: &mockEmbedding{},
	}

	// Create an indexer
	ctx := context.Background()
	indexer, err := pinecone.NewIndexer(ctx, &config)
	if err != nil {
		log.Fatalf("Failed to create Pinecone indexer: %v", err)
	}
	log.Println("Indexer created successfully")

	// Store documents
	docs := []*schema.Document{
		{
			ID:      "pinecone-1",
			Content: "pinecone is a vector database",
			MetaData: map[string]any{
				"tag1": "pinecone",
				"tag2": "vector",
				"tag3": "database",
			},
		},
		{
			ID:      "pinecone-2",
			Content: "Pinecone is a vector database for building accurate and performant AI applications.",
		},
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store documents: %v", err)
		return
	}
	log.Printf("Stored document ids: %v", ids)
}

// mockEmbedding is a placeholder for your embedding implementation
// Replace with your actual embedding model

// type mockEmbedding struct{}
// func (m *mockEmbedding) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
// 	// Implement your embedding logic here
// }
```

## Configuration

The following configuration options are available in `IndexerConfig`:

| Field               | Type                    | Description                                                      | Default         |
|---------------------|-------------------------|------------------------------------------------------------------|-----------------|
| Client              | *pinecone.Client        | Pinecone client instance (required)                              | -               |
| IndexName           | string                  | Name of the Pinecone index                                       | "eino-index"    |
| Cloud               | pinecone.Cloud          | Cloud provider (e.g., "aws")                                     | "aws"           |
| Region              | string                  | Cloud region (e.g., "us-east-1")                                 | "us-east-1"     |
| Metric              | pinecone.IndexMetric    | Distance metric: "cosine", "euclidean", "dotproduct"            | "cosine"        |
| Dimension           | int32                   | Vector dimension                                                 | 2560            |
| VectorType          | string                  | Type of vectors (e.g., "float32")                                | "float32"       |
| Namespace           | string                  | Namespace within the index                                       | (default)       |
| Field               | string                  | Field to store content text                                      | (default)       |
| Tags                | *pinecone.IndexTags     | Metadata tags                                                    | (optional)      |
| DeletionProtection  | pinecone.DeletionProtection | Deletion protection                                        | (optional)      |
| DocumentConverter   | func                    | Custom document converter                                        | (optional)      |
| BatchSize           | int                     | Batch size for upserts                                           | 100             |
| MaxConcurrency      | int                     | Max concurrency for upserts                                      | 10              |
| Embedding           | embedding.Embedder      | Embedding model instance                                         | (required)      |

## License

Apache 2.0. See [LICENSE](../../LICENSE) for details.
