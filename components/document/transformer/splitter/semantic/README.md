# Semantic Splitter for Eino

English | [简体中文](README_zh.md)

## Introduction

This is a semantic-based document splitter component for [Eino](https://github.com/cloudwego/eino). It implements the `Transformer` interface and splits documents based on semantic similarity using embeddings, ensuring that semantically related content stays together.

## Features

- Implements `github.com/cloudwego/eino/components/document.Transformer` interface
- Split documents based on semantic similarity using embeddings
- Configurable buffer size for context-aware splitting
- Customizable separators for initial text segmentation
- Minimum chunk size enforcement
- Percentile-based threshold for determining split points
- Optional custom ID generator for split chunks
- Easy integration into Eino workflows

## How It Works

1. **Initial Split**: Text is split using separators (newlines, periods, etc.)
2. **Context Buffering**: Adjacent sentences are combined with buffer to create context-rich chunks
3. **Embedding**: Each buffered chunk is embedded using the provided embedder
4. **Similarity Calculation**: Cosine similarity is calculated between adjacent embeddings
5. **Threshold Determination**: A percentile-based threshold determines where to split
6. **Final Split**: Text is split at points where similarity drops below the threshold
7. **Merge Small Chunks**: Chunks smaller than minimum size are merged with adjacent chunks

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic"
	"github.com/cloudwego/eino-ext/components/embedding/openai"
)

func main() {
	ctx := context.Background()

	embedder, err := openai.NewEmbedder(ctx, &openai.Config{
		APIKey: "your-api-key",
	})
	if err != nil {
		log.Fatalf("openai.NewEmbedder failed, err=%v", err)
	}

	splitter, err := semantic.NewSplitter(ctx, &semantic.Config{
		Embedding:    embedder,
		BufferSize:   1,
		MinChunkSize: 50,
		Percentile:   0.9,
	})
	if err != nil {
		log.Fatalf("semantic.NewSplitter failed, err=%v", err)
	}

	docs := []*schema.Document{
		{
			ID: "doc-1",
			Content: `Introduction to AI. Artificial intelligence is transforming industries.
			Machine learning is a subset of AI. It learns from data.
			Weather forecast for tomorrow. It will be sunny and warm.`,
		},
	}

	splits, err := splitter.Transform(ctx, docs)
	if err != nil {
		log.Fatalf("splitter.Transform failed, err=%v", err)
	}

	for i, split := range splits {
		log.Printf("Split %d: %s", i+1, split.Content)
	}
}
```

## Configuration

The splitter can be configured through the `Config` structure:

```go
type Config struct {
    // Embedding is used to generate vectors for similarity calculation (Required)
    Embedding embedding.Embedder
    
    // BufferSize specifies how many chunks to concatenate before and after
    // each chunk during embedding (Optional)
    // This allows chunks to carry more context information
    // Default: 0
    // Example: 1 means each chunk includes 1 sentence before and after
    BufferSize int
    
    // MinChunkSize specifies the minimum chunk size (Optional)
    // Chunks smaller than this will be concatenated to adjacent chunks
    // Default: 0 (no minimum)
    // Example: 50 characters
    MinChunkSize int
    
    // Separators are sequentially used to split text (Optional)
    // Default: ["\n", ".", "?", "!"]
    // Example: ["\n\n", "\n", ". "]
    Separators []string
    
    // LenFunc is used to calculate string length (Optional)
    // Default: len() function
    // Example: func(s string) int { return utf8.RuneCountInString(s) }
    LenFunc func(s string) int
    
    // Percentile specifies the split threshold (Optional)
    // If the similarity difference between two chunks is greater than
    // the X percentile, they will be split
    // Default: 0.9 (90th percentile)
    // Example: 0.95 for stricter splitting
    Percentile float64
    
    // IDGenerator is an optional function to generate new IDs (Optional)
    // Default: keeps original document ID
    IDGenerator IDGenerator
}
```

## Example with Custom Settings

```go
splitter, err := semantic.NewSplitter(ctx, &semantic.Config{
    Embedding:    embedder,
    BufferSize:   2,
    MinChunkSize: 100,
    Separators:   []string{"\n\n", "\n", ". ", "! ", "? "},
    Percentile:   0.95,
    IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
        return fmt.Sprintf("%s_semantic_%d", originalID, splitIndex)
    },
})
```

## Understanding BufferSize

The `BufferSize` parameter is crucial for semantic splitting:

- **BufferSize = 0**: Each sentence is embedded independently
- **BufferSize = 1**: Each sentence is embedded with 1 sentence before and after
- **BufferSize = 2**: Each sentence is embedded with 2 sentences before and after

Higher buffer sizes provide more context but increase embedding costs.

## Understanding Percentile

The `Percentile` parameter controls splitting sensitivity:

- **Percentile = 0.9** (90%): Split at points with lower similarity (more chunks)
- **Percentile = 0.95** (95%): Split only at points with very low similarity (fewer chunks)
- **Percentile = 0.5** (50%): Very aggressive splitting (many small chunks)

## Using in Chain

```go
import (
    "github.com/cloudwego/eino/compose"
    semanticSplitter "github.com/cloudwego/eino-ext/components/document/transformer/splitter/semantic"
)

splitter, _ := semanticSplitter.NewSplitter(ctx, &semanticSplitter.Config{
    Embedding:    embedder,
    BufferSize:   1,
    MinChunkSize: 50,
})

chain := compose.NewChain[[]*schema.Document, []*schema.Document]()
chain.AppendDocumentTransformer(splitter)

run, _ := chain.Compile(ctx)
splitDocs, _ := run.Invoke(ctx, docs)
```

## Performance Considerations

- **Embedding Cost**: Each sentence (with buffer) requires an embedding call. For long documents, this can be expensive.
- **Buffer Trade-off**: Larger buffers provide better context but increase embedding size and cost.
- **Chunk Size**: Setting a minimum chunk size helps avoid creating too many tiny chunks.

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
