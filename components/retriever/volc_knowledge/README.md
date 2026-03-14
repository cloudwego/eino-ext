# Volcengine Knowledge Retriever for Eino

English | [简体中文](README_zh.md)

## Introduction

This is a Volcengine Knowledge Base retriever component for [Eino](https://github.com/cloudwego/eino). It implements the `Retriever` interface and integrates with Volcengine's Knowledge Base service to retrieve relevant documents based on query text.

## Features

- Implements `github.com/cloudwego/eino/components/retriever.Retriever` interface
- Integration with Volcengine Knowledge Base service
- Support for dense retrieval with configurable weights
- Query preprocessing (rewriting, instruction generation)
- Post-processing options (reranking, chunk diffusion, grouping)
- Document filtering capabilities
- Metadata extraction (doc ID, doc name, chunk ID, attachments, tables)
- Token usage tracking
- Easy integration into Eino workflows

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/retriever/volc_knowledge
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	knowledge "github.com/cloudwego/eino-ext/components/retriever/volc_knowledge"
)

func main() {
	ctx := context.Background()

	retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
		AK:         "your-access-key",
		SK:         "your-secret-key",
		AccountID:  "your-account-id",
		Name:       "your-knowledge-base-name",
		Project:    "default",
		Limit:      10,
	})
	if err != nil {
		log.Fatalf("knowledge.NewRetriever failed, err=%v", err)
	}

	docs, err := retriever.Retrieve(ctx, "What is machine learning?")
	if err != nil {
		log.Fatalf("retriever.Retrieve failed, err=%v", err)
	}

	for i, doc := range docs {
		log.Printf("Document %d:", i+1)
		log.Printf("  ID: %s", doc.ID)
		log.Printf("  Content: %s", doc.Content)
		log.Printf("  Doc ID: %s", knowledge.GetDocID(doc))
		log.Printf("  Doc Name: %s", knowledge.GetDocName(doc))
	}
}
```

## Configuration

The retriever can be configured through the `Config` structure:

```go
type Config struct {
    // Authentication (Required)
    AK        string // Access key
    SK        string // Secret key
    AccountID string // Volcengine account ID
    
    // Connection Settings (Optional)
    Timeout time.Duration // Request timeout (default: no timeout)
    BaseURL string        // Base URL (default: "api-knowledgebase.mlp.cn-beijing.volces.com")
    
    // Knowledge Base Identification (Required: either Name+Project OR ResourceID)
    Name       string // Knowledge base name
    Project    string // Project identifier (default: "default")
    ResourceID string // Resource identifier (alternative to Name+Project)
    
    // Retrieval Settings (Optional)
    Limit       int32          // Max documents to return (1-200, default: 10)
    DocFilter   map[string]any // Document filters
    DenseWeight float64        // Dense retrieval weight (default: 0.5)
    
    // Preprocessing (Optional)
    NeedInstruction  bool              // Include instructions
    Rewrite          bool              // Enable query rewriting
    ReturnTokenUsage bool              // Return token usage info
    Messages         []*schema.Message // Conversation history for rewriting
    
    // Post-processing (Optional)
    RerankSwitch        bool   // Enable reranking
    RetrieveCount       int32  // Docs to retrieve for reranking (≥Limit, default: 25)
    ChunkDiffusionCount int32  // Chunk diffusion count
    ChunkGroup          bool   // Group chunks
    RerankModel         string // Rerank model name
    RerankOnlyChunk     bool   // Rerank using only chunk content
    GetAttachmentLink   bool   // Include attachment links
}
```

## Advanced Usage

### With Reranking

```go
retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:            "your-access-key",
    SK:            "your-secret-key",
    AccountID:     "your-account-id",
    Name:          "your-knowledge-base-name",
    Limit:         10,
    RerankSwitch:  true,
    RetrieveCount: 50,
    RerankModel:   "bge-reranker-v2-m3",
})
```

### With Query Rewriting

```go
retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:        "your-access-key",
    SK:        "your-secret-key",
    AccountID: "your-account-id",
    Name:      "your-knowledge-base-name",
    Rewrite:   true,
    Messages: []*schema.Message{
        {Role: schema.User, Content: "Tell me about AI"},
        {Role: schema.Assistant, Content: "AI is..."},
        {Role: schema.User, Content: "How does it work?"},
    },
})
```

### With Document Filtering

```go
retriever, err := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:        "your-access-key",
    SK:        "your-secret-key",
    AccountID: "your-account-id",
    Name:      "your-knowledge-base-name",
    DocFilter: map[string]any{
        "doc_type": "pdf",
        "year":     2024,
    },
})
```

## Metadata Helpers

The package provides helper functions to extract metadata:

```go
docID := knowledge.GetDocID(doc)           // Get document ID
docName := knowledge.GetDocName(doc)       // Get document name
chunkID := knowledge.GetChunkID(doc)       // Get chunk ID
attachments := knowledge.GetAttachments(doc) // Get attachments
tables := knowledge.GetTableChunks(doc)    // Get table chunks
```

## Using in Chain

```go
import (
    "github.com/cloudwego/eino/compose"
    knowledge "github.com/cloudwego/eino-ext/components/retriever/volc_knowledge"
)

retriever, _ := knowledge.NewRetriever(ctx, &knowledge.Config{
    AK:        "your-access-key",
    SK:        "your-secret-key",
    AccountID: "your-account-id",
    Name:      "your-knowledge-base-name",
})

chain := compose.NewChain[string, []*schema.Document]()
chain.AppendRetriever(retriever)

run, _ := chain.Compile(ctx)
docs, _ := run.Invoke(ctx, "query text")
```

## For More Details

- [Volcengine Knowledge Base API Documentation](https://www.volcengine.com/docs/84313/1350012)
- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
