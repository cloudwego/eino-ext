# Milvus New Indexer

English | [简体中文](README_zh.md)

An Milvus 2.6+ indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer`
interface. This enables seamless integration
with Eino's vector storage and retrieval system for enhanced semantic search capabilities.
Compared to the old version, this version is based on the new Milvus Client V2 API, with better performance and type safety.

## Quick Start

### Installation

It requires the milvus/client/v2 of version 2.6+

```bash
go get github.com/milvus-io/milvus/client/v2@v2.6.1
go get github.com/cloudwego/eino-ext/components/indexer/milvus_new@latest
```

### Create the Milvus Indexer

```go
package main

import (
	"context"
	"log"
	"os"
	
	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	
	"github.com/cloudwego/eino-ext/components/indexer/milvus_new"
)

func main() {
	// Get the environment variables
	addr := os.Getenv("MILVUS_ADDR")
	username := os.Getenv("MILVUS_USERNAME")
	password := os.Getenv("MILVUS_PASSWORD")
	arkApiKey := os.Getenv("ARK_API_KEY")
	arkModel := os.Getenv("ARK_MODEL")
	
	// Create a client
	ctx := context.Background()
	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address:  addr,
		Username: username,
		Password: password,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
		return
	}
	defer cli.Close()
	
	// Create an embedding model
	emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey: arkApiKey,
		Model:  arkModel,
	})
	if err != nil {
		log.Fatalf("Failed to create embedding: %v", err)
		return
	}
	
	// Create an indexer
	indexer, err := milvus_new.NewIndexer(ctx, &milvus_new.IndexerConfig{
		Client:    cli,
		Embedding: emb,
	})
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
		return
	}
	log.Printf("Indexer created success")
	
	// Store documents
	docs := []*schema.Document{
		{
			ID:      "milvus-1",
			Content: "milvus is an open-source vector database",
			MetaData: map[string]any{
				"h1": "milvus",
				"h2": "open-source",
				"h3": "vector database",
			},
		},
		{
			ID:      "milvus-2",
			Content: "milvus is a distributed vector database",
		},
	}
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("Failed to store: %v", err)
		return
	}
	log.Printf("Store success, ids: %v", ids)
}
```

## Configuration

```go
type IndexerConfig struct {
    // Client is the milvus client to be called
    // It uses the new milvus/client/v2/milvusclient
    // Required
    Client *milvusclient.Client

    // Default Collection config
    // Collection is the collection name in milvus database
    // Optional, and the default value is "eino_collection"
    Collection string
    // Description is the description for collection
    // Optional, and the default value is "the collection for eino"
    Description string
    // PartitionNum is the collection partition number
    // Optional, and the default value is 0(disable)
    // If the partition number is larger than 1, it means use partition and must have a partition key in Fields
    PartitionNum int64
    // PartitionName is the collection partition name
    // Optional, and the default value is empty
    PartitionName string
    // Fields is the collection fields
    // Optional, and the default value is the default fields
    Fields       []*entity.Field
    // SharedNum is the milvus required param to create collection
    // Optional, and the default value is 1
    SharedNum int32
    // ConsistencyLevel is the milvus collection consistency tactics
    // Optional, and the default level is ClBounded(bounded consistency level with default tolerance of 5 seconds)
    ConsistencyLevel ConsistencyLevel
    // EnableDynamicSchema is means the collection is enabled to dynamic schema
    // Optional, and the default value is false
    // Enable to dynamic schema it could affect milvus performance
    EnableDynamicSchema bool

    // DocumentConverter is the function to convert the schema.Document to the row data
    // Optional, and the default value is defaultDocumentConverter
    DocumentConverter func(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]column.Column, error)

    // Index config to the vector column
    // MetricType the metric type for vector
    // Optional and default type is HAMMING
    MetricType MetricType

    // Embedding vectorization method for values needs to be embedded from schema.Document's content.
    // Required
    Embedding embedding.Embedder
}
```

## Options

### WithPartition

Specify partition name for storage

```go
ids, err := indexer.Store(ctx, docs, 
    milvus_new.WithPartition("partition_2024"))
```

## Default Collection Schema

| Field    | Type           | DataBase Type | Index Type                 | Description             | Remark             |
|----------|----------------|---------------|----------------------------|-------------------------|--------------------|
| id       | string         | varchar       |                            | Document ID             | Max Length: 255    |
| content  | string         | varchar       |                            | Document content        | Max Length: 1024   |
| vector   | []float32      | float array   | HAMMING(default) / JACCARD | Document content vector | Default Dim: 768   |
| metadata | map[string]any | json          |                            | Document meta data      |                    |

## Key Differences from the Old Version

1. **Client API**: Uses the new `milvus/client/v2/milvusclient` instead of `milvus-sdk-go/v2/client`
2. **Data Format**: Uses column-based data format instead of row-based for better performance
3. **Type Safety**: Better type safety with specific types instead of interfaces
4. **Configuration**: Some configuration parameters have changed to match the new API

## How to determine the dim parameter

The conversion relationship is `dim = embedding model output`

In the new version, we directly use float vectors instead of converting float64 to bytes, so the dimension is directly determined by the embedding model output.