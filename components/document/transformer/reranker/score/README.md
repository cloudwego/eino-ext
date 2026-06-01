# Score-Based Reranker for Eino

English | [简体中文](README_zh.md)

## Introduction

This is a score-based document reranker component for [Eino](https://github.com/cloudwego/eino). It implements the `Transformer` interface and reorganizes documents based on their scores to optimize LLM context processing.

## Features

- Implements `github.com/cloudwego/eino/components/document.Transformer` interface
- Reorders documents based on scores using the "primacy and recency effect"
- Places high-score documents at the beginning and end of the array
- Places low-score documents in the middle
- Supports custom score field from document metadata
- Optimized for LLM context window performance

## Why This Reranking Strategy?

This reranker is based on research showing that LLMs exhibit better performance when relevant information appears at the beginning or end of the input context, known as the "primacy and recency effect". 

The strategy places:
- **High-scoring documents** → at the beginning and end (where LLMs pay more attention)
- **Low-scoring documents** → in the middle (where LLMs pay less attention)

Reference: [Lost in the Middle: How Language Models Use Long Contexts](https://arxiv.org/abs/2307.03172)

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/reranker/score
```

## Quick Start

### Using default document scores

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/document/transformer/reranker/score"
)

func main() {
	ctx := context.Background()

	reranker, err := score.NewReranker(ctx, &score.Config{})
	if err != nil {
		log.Fatalf("score.NewReranker failed, err=%v", err)
	}

	docs := []*schema.Document{
		{Content: "Doc 1", DocumentScore: 0.5},
		{Content: "Doc 2", DocumentScore: 0.9},
		{Content: "Doc 3", DocumentScore: 0.3},
		{Content: "Doc 4", DocumentScore: 0.8},
		{Content: "Doc 5", DocumentScore: 0.1},
	}

	reranked, err := reranker.Transform(ctx, docs)
	if err != nil {
		log.Fatalf("reranker.Transform failed, err=%v", err)
	}

	for i, doc := range reranked {
		log.Printf("Position %d: %s (score: %.1f)", i, doc.Content, doc.Score())
	}
}
```

### Using custom score field

```go
scoreField := "custom_score"
reranker, err := score.NewReranker(ctx, &score.Config{
	ScoreFieldKey: &scoreField,
})

docs := []*schema.Document{
	{
		Content: "Doc 1",
		MetaData: map[string]any{"custom_score": 0.5},
	},
	{
		Content: "Doc 2",
		MetaData: map[string]any{"custom_score": 0.9},
	},
}

reranked, err := reranker.Transform(ctx, docs)
```

## Configuration

The reranker can be configured through the `Config` structure:

```go
type Config struct {
    // ScoreFieldKey specifies the key in metadata that stores the document score
    // If nil, uses Document.Score() method (default)
    // Example: &"relevance_score"
    ScoreFieldKey *string
}
```

## Example Reranking

Input documents with scores: `[0.5, 0.9, 0.3, 0.8, 0.1]`

After sorting by score (descending): `[0.9, 0.8, 0.5, 0.3, 0.1]`

After reranking (alternating high/low):
```
Position 0: 0.9 (highest)     ← Start: High attention
Position 1: 0.5 (middle-high)
Position 2: 0.3 (middle-low)
Position 3: 0.1 (lowest)
Position 4: 0.8 (second highest) ← End: High attention
```

## Using in Chain

```go
import (
    "github.com/cloudwego/eino/compose"
    scoreReranker "github.com/cloudwego/eino-ext/components/document/transformer/reranker/score"
)

reranker, _ := scoreReranker.NewReranker(ctx, &scoreReranker.Config{})

chain := compose.NewChain[[]*schema.Document, []*schema.Document]()
chain.AppendDocumentTransformer(reranker)

run, _ := chain.Compile(ctx)
rerankedDocs, _ := run.Invoke(ctx, docs)
```

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
