# ES8 Indexer

English

An Elasticsearch 8.x indexer implementation for [Eino](https://github.com/cloudwego/eino) that implements the `Indexer` interface. This enables seamless integration with Eino's vector storage and retrieval system for enhanced semantic search capabilities.

## Features

- Implements `github.com/cloudwego/eino/components/indexer.Indexer`
- Easy integration with Eino's indexer system
- Configurable Elasticsearch parameters
- Support for vector similarity search
- Bulk indexing operations
- Custom field mapping support
- Flexible document vectorization

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/indexer/es8
```

## Quick Start
```go
package main

import (
    "context"
    "log"

    "github.com/cloudwego/eino-ext/components/indexer/es8"
    "github.com/elastic/go-elasticsearch/v8"
    "github.com/cloudwego/eino/schema"
)

func main() {
    // Create ES client
    esClient, err := elasticsearch.NewClient(elasticsearch.Config{
        Addresses: []string{"http://localhost:9200"},
        Username:  "elastic",
        Password:  "your_password",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create indexer config
    cfg := &es8.IndexerConfig{
        Client: esClient,
        Index:  "your_index_name",
        BatchSize: 5,  // Optional: controls max texts size for embedding
        DocumentToFields: func(ctx context.Context, doc *schema.Document) (map[string]es8.FieldValue, error) {
            // Define how document fields should be mapped to Elasticsearch fields
            return map[string]es8.FieldValue{
                "content": {
                    Value:    doc.Content,
                    EmbedKey: "content_vector", // Field will be vectorized
                },
                "metadata": {
                    Value: doc.Metadata,
                },
            }, nil
        },
        // Optional: provide embedder if vectorization is needed
        Embedding: yourEmbedder,
    }

    // Create the ES8 indexer
    indexer, err := es8.NewIndexer(context.Background(), cfg)
    if err != nil {
        log.Fatal(err)
    }

    // Use with Eino's system
    // ... configure and use with Eino
}
```

## Configuration

The indexer can be configured using the `IndexerConfig` struct:

```go
type IndexerConfig struct {
    Client *elasticsearch.Client // Required: Elasticsearch client instance
    Index  string               // Required: Index name to store documents
    BatchSize int               // Optional: Max texts size for embedding (default: 5)
    
    // Required: Function to map Document fields to Elasticsearch fields
    DocumentToFields func(ctx context.Context, doc *schema.Document) (map[string]FieldValue, error)
    
    // Optional: Required only if vectorization is needed
    Embedding embedding.Embedder
}

// FieldValue defines how a field should be stored and vectorized
type FieldValue struct {
    Value     any    // Original value to store
    EmbedKey  string // If set, Value will be vectorized and saved
    Stringify func(val any) (string, error) // Optional: custom string conversion
}
```

## For More Details

- [Eino Documentation](https://github.com/cloudwego/eino)
- [Elasticsearch Go Client Documentation](https://github.com/elastic/go-elasticsearch)