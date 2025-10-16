# Dify Retriever

English | [简体中文](README_zh.md)

A Dify retriever implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Retriever` interface. This enables seamless integration with Eino's retrieval system for retrieving relevant documents from Dify datasets.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever`
- Easy integration with Eino's retrieval system
- Support for configurable retrieval parameters
- Reranking support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/dify
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/retriever/dify"
)



func main() {
	APIKey := os.Getenv("DIFY_DATASET_API_KEY")
	Endpoint := os.Getenv("DIFY_ENDPOINT")
	DatasetID := os.Getenv("DIFY_DATASET_ID")
	ctx := context.Background()

	// create Dify Retriever
	ret, err := dify.NewRetriever(ctx, &dify.RetrieverConfig{
		APIKey:    APIKey,
		Endpoint:  Endpoint,
		DatasetID: DatasetID,
	})
	if err != nil {
		log.Fatalf("Failed to create retriever: %v", err)
	}

	// do search
	docs, err := ret.Retrieve(ctx, "test query")
	if err != nil {
		log.Fatalf("Failed to retrieve: %v", err)
	}

	// print docs
	for _, doc := range docs {
		fmt.Printf("doc id: %s\n", doc.ID)
		fmt.Printf("doc content: %s\n", doc.Content)
		fmt.Printf("score: %v\n\n", doc.Score())
	}
}
```

## Configuration

The retriever can be configured using the `RetrieverConfig` struct:

```go
type RetrieverConfig struct {
    APIKey string   // Dify Datasets API key
    Endpoint string // Endpoint of the Dify API, default: https://api.dify.ai/v1
    DatasetID string    // DatasetID of the Dify datasets
    RetrievalModel *RetrievalModel  // Retrieval model configuration 
    Timeout time.Duration   // HTTP connection timeout
}

type RetrievalModel struct {
    SearchMethod          SearchMethod    // Search method
    RerankingEnable      *bool           // Enable reranking
    RerankingMode        *string         // Reranking mode
    RerankingModel       *RerankingModel // Reranking model settings
    Weights              *float64        // Search weights
    TopK                 *int            // Number of documents to retrieve
    ScoreThresholdEnabled *bool          // Enable score threshold
    ScoreThreshold       *float64        // Minimum score threshold
}
```

## Document Metadata

The retriever adds the following metadata to retrieved documents:

- `orig_doc_id`: Original document ID in Dify
- `orig_doc_name`: Original document name in Dify
- `keywords`: Keywords extracted from the document

You can access these metadata using the helper functions:

```go
docID := dify.GetOrgDocID(doc)
docName := dify.GetOrgDocName(doc)
keywords := dify.GetKeywords(doc)
```

## For More Details

- [Dify API Documentation](https://github.com/langgenius/dify)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
