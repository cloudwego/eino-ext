# Milvus Retriever

English | [简体中文](README_zh.md)

An Milvus 2.x retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever`
interface. This enables seamless integration
with Eino's vector storage and retrieval system for enhanced semantic search capabilities.

> **Note**: This package supports **Milvus 2.4.x**. For Milvus 2.5+ features (BM25, server-side functions, hybrid search), use the [`milvus2`](../milvus2) package instead.

## Quick Start

### Installation

```bash
go get github.com/milvus-io/milvus-sdk-go/v2@2.4.2
go get github.com/cloudwego/eino-ext/retriever/milvus@latest
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
	"github.com/milvus-io/milvus-sdk-go/v2/client"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
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
	cli, err := client.NewClient(ctx, client.Config{
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
	retriever, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      cli,
		Collection:  "",
		Partition:   nil,
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
		Sp:                nil,
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
	// Required
	Client client.Client

	// Default Retriever config
	// Collection is the collection name in the milvus database
	// Optional, and the default value is "eino_collection"
	Collection string
	// Partition is the collection partition name
	// Optional, and the default value is empty
	Partition []string
	// VectorField is the vector field name in the collection
	// Optional, and the default value is "vector"
	VectorField string
	// OutputFields is the fields to be returned
	// Optional, and the default value is empty
	OutputFields []string
	// DocumentConverter is the function to convert the search result to s.Document
	// Optional, and the default value is defaultDocumentConverter
	DocumentConverter func(ctx context.Context, doc client.SearchResult) ([]*s.Document, error)
	// MetricType is the metric type for vector
	// Optional, and the default value is "HAMMING"
	MetricType entity.MetricType
	// TopK is the top k results to be returned
	// Optional, and the default value is 5
	TopK int
	// ScoreThreshold is the threshold for the search result
	// Optional, and the default value is 0
	ScoreThreshold float64
	// SearchParams
	// Optional, and the default value is entity.IndexAUTOINDEXSearchParam, and the level is 1
	Sp entity.SearchParam

	// Embedding is the embedding vectorization method for values needs to be embedded from s.Document's content.
	// Required
	Embedding embedding.Embedder
}
```