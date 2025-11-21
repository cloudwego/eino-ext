# Milvus New Retriever

English | [简体中文](README_zh.md)

An Milvus 2.6+ retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever`
interface. This enables seamless integration
with Eino's vector storage and retrieval system for enhanced semantic search capabilities.
Compared to the old version, this version is based on the new Milvus Client V2 API, with better performance and type safety.

## Quick Start

### Installation

It requires the milvus/client/v2 of version 2.6+

```bash
go get github.com/milvus-io/milvus/client/v2@v2.6.1
go get github.com/cloudwego/eino-ext/components/retriever/milvus_new@latest
```

### Create the Milvus Retriever

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	"github.com/cloudwego/eino-ext/components/retriever/milvus_new"
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

	// Create a retriever
	retriever, err := milvus_new.NewRetriever(ctx, &milvus_new.RetrieverConfig{
		Client:      cli,
		Collection:  "",
		Partition:   "",
		VectorField: "",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		DocumentConverter: nil,
		MetricType:        "",
		TopK:              0,
		ScoreThreshold:    5,
		Embedding:         emb,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
		return
	}

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "milvus")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
		return
	}
	
	// Print the documents
	for i, doc := range documents {
		fmt.Printf("Document %d:\n", i)
		fmt.Printf("title: %s\n", doc.ID)
		fmt.Printf("content: %s\n", doc.Content)
		fmt.Printf("metadata: %v\n", doc.MetaData)
	}
}
```

## Configuration

```go
type RetrieverConfig struct {
	// Client is the milvus client to be called
	// It uses the new milvus/client/v2/milvusclient
	// Required
	Client *milvusclient.Client

	// Default Retriever config
	// Collection is the collection name in the milvus database
	// Optional, and the default value is "eino_collection"
	Collection string
	// Partition is the collection partition name
	// Optional, and the default value is empty
	Partition string
	// VectorField is the vector field name in the collection
	// Optional, and the default value is "vector"
	VectorField string
	// OutputFields is the fields to be returned
	// Optional, and the default value is all fields except vector
	OutputFields []string
	// DocumentConverter is the function to convert the search result to schema.Document
	// Optional, and the default value is defaultDocumentConverter
	DocumentConverter func(ctx context.Context, columns []column.Column, scores []float32) ([]*schema.Document, error)
	// VectorConverter is the function to convert the vectors to binary vector bytes
	// Deprecated: This field is no longer used for float vectors. Float vectors are handled directly.
	VectorConverter func(ctx context.Context, vectors [][]float64) ([][]byte, error)
	// MetricType is the metric type for vector
	// Optional, and the default value is "COSINE" for float vectors
	MetricType MetricType
	// TopK is the top k results to be returned
	// Optional, and the default value is 5
	TopK int
	// ScoreThreshold is the threshold for the search result
	// Optional, and the default value is 0
	ScoreThreshold float64

	// Embedding is the embedding vectorization method for values needs to be embedded from schema.Document's content.
	// Required
	Embedding embedding.Embedder
}
```

## Options

### WithFilter

Set filter expression for search

```go
docs, err := retriever.Retrieve(ctx, "query", 
    milvus_new.WithFilter("year > 2020"))
```

### WithPartition

Specify partition name for search

```go
docs, err := retriever.Retrieve(ctx, "query", 
    milvus_new.WithPartition("partition_2024"))
```

## Key Differences from the Old Version

1. **Client API**: Uses the new `milvus/client/v2/milvusclient` instead of `milvus-sdk-go/v2/client`
2. **Data Format**: Uses column-based data format instead of SearchResult for better performance
3. **Type Safety**: Better type safety with specific types instead of interfaces
4. **Configuration**: Some configuration parameters have changed to match the new API
5. **Options**: Uses new option pattern with `milvus_new.WithFilter()` and `milvus_new.WithPartition()` instead of parameters in Retrieve method