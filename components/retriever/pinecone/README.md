# Pinecone Search

[English](README.md) | [简体中文](README_zh.md)

This is a Pinecone-based vector search implementation that provides a storage solution compatible with the `Retriever` interface for [Eino](https://github.com/cloudwego/eino). The component can be seamlessly integrated into Eino's vector storage and retrieval system to enhance semantic search capabilities.

## Quick Start

### Installation

Requires pinecone-io/go-pinecone/v3 client version 3.x.x

```bash
go get github.com/eino-project/eino/retriever/pinecone@latest
```

### Create a Pinecone Retriever

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/retriever/pinecone"
	"github.com/cloudwego/eino/components/embedding"
	pc "github.com/pinecone-io/go-pinecone/v3/pinecone"
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

	// Create Pinecone retriever config
	config := pinecone.RetrieverConfig{
		Client:    client,
		Embedding: &mockEmbedding{},
	}

	ctx := context.Background()
	retriever, err := pinecone.NewRetriever(ctx, &config)
	if err != nil {
		log.Fatalf("Failed to create Pinecone retriever: %v", err)
	}
	log.Println("Retriever created successfully")

	// Retrieve documents
	documents, err := retriever.Retrieve(ctx, "pinecone")
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

## 配置

```go
type RetrieverConfig struct {
	// Client is the Pinecone client instance used for all API operations.
	// Required. Must be initialized before use.
	Client *pc.Client

	// IndexName is the name of the Pinecone index to search against.
	// Optional. Default is "eino-index".
	IndexName string

	// Namespace is the logical namespace within the index, used for multi-tenant or data isolation scenarios.
	// Optional. Default is "".
	Namespace string

	// MetricType specifies the similarity metric used for vector search (e.g., cosine, dotproduct, euclidean).
	// Optional. Default is pc.IndexMetricCosine.
	MetricType pc.IndexMetric

	// Field specifies the document field to associate with vector data, used for mapping between Pinecone vectors and application documents.
	// Optional. Default is "". Set if you want to map a specific document field.
	Field string

	// VectorConverter is a function to convert float64 vectors (from embedding models) to float32 as required by Pinecone API.
	// Optional. If nil, a default conversion will be used.
	VectorConverter func(ctx context.Context, vector []float64) ([]float32, error)

	// DocumentConverter is a function to convert Pinecone vector results to schema.Document objects for downstream consumption.
	// Optional. If nil, a default converter will be used.
	DocumentConverter func(ctx context.Context, vector *pc.Vector, field string) (*schema.Document, error)

	// TopK specifies the number of top results to return for each query.
	// Optional. Default is 10.
	TopK int

	// ScoreThreshold is the minimum similarity score for a result to be returned.
	// Optional. Default is 0. Used to filter out low-relevance matches.
	ScoreThreshold float64

	// Embedding is the embedding model or service used to convert queries into vector representations.
	// Required for semantic search.
	Embedding embedding.Embedder
}
```