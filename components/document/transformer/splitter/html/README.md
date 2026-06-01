# HTML Header Splitter for Eino

English | [简体中文](README_zh.md)

## Introduction

This is an HTML header-based splitter component for [Eino](https://github.com/cloudwego/eino). It implements the `Transformer` interface and splits HTML documents based on header tags (h1, h2, h3, etc.), preserving header hierarchy as metadata.

## Features

- Implements `github.com/cloudwego/eino/components/document.Transformer` interface
- Split HTML content based on header tags (h1-h6)
- Preserve header hierarchy in document metadata
- Customizable header mapping to metadata keys
- Optional custom ID generator for split chunks
- Maintain parent-child relationships between headers
- Easy integration into Eino workflows

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/splitter/html
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/html"
)

func main() {
	ctx := context.Background()

	splitter, err := html.NewHeaderSplitter(ctx, &html.HeaderConfig{
		Headers: map[string]string{
			"h1": "Chapter",
			"h2": "Section",
		},
	})
	if err != nil {
		log.Fatalf("html.NewHeaderSplitter failed, err=%v", err)
	}

	htmlContent := `
		<h1>Chapter 1</h1>
		<p>Introduction text for chapter 1.</p>
		<h2>Section 1.1</h2>
		<p>Content for section 1.1.</p>
		<h2>Section 1.2</h2>
		<p>Content for section 1.2.</p>
		<h1>Chapter 2</h1>
		<p>Introduction text for chapter 2.</p>
	`

	docs := []*schema.Document{
		{
			Content: htmlContent,
			ID:      "doc-1",
		},
	}

	splits, err := splitter.Transform(ctx, docs)
	if err != nil {
		log.Fatalf("splitter.Transform failed, err=%v", err)
	}

	for i, split := range splits {
		log.Printf("Split %d:", i+1)
		log.Printf("  Content: %s", split.Content)
		log.Printf("  Metadata: %v", split.MetaData)
	}
}
```

## Configuration

The splitter can be configured through the `HeaderConfig` structure:

```go
type HeaderConfig struct {
    // Headers specify the headers to identify and their names in metadata
    // Key format: "h1", "h2", "h3", etc.
    // Value: the metadata key name for this header level
    // Example: {"h1": "Title", "h2": "Section", "h3": "Subsection"}
    Headers map[string]string
    
    // IDGenerator is an optional function to generate new IDs for split chunks
    // If nil, the original document ID will be used for all splits
    // Example: func(ctx context.Context, originalID string, splitIndex int) string {
    //     return fmt.Sprintf("%s_chunk_%d", originalID, splitIndex)
    // }
    IDGenerator IDGenerator
}
```

## Example Output

Given the HTML input:

```html
<h1>Chapter 1</h1>
<p>Introduction text</p>
<h2>Section 1.1</h2>
<p>Section content</p>
<h2>Section 1.2</h2>
<p>More content</p>
```

With config `Headers: {"h1": "Chapter", "h2": "Section"}`, you'll get:

**Split 1:**
```go
{
    Content: "Introduction text",
    MetaData: {
        "Chapter": "Chapter 1"
    }
}
```

**Split 2:**
```go
{
    Content: "Section content",
    MetaData: {
        "Chapter": "Chapter 1",
        "Section": "Section 1.1"
    }
}
```

**Split 3:**
```go
{
    Content: "More content",
    MetaData: {
        "Chapter": "Chapter 1",
        "Section": "Section 1.2"
    }
}
```

## Custom ID Generator

```go
idGenerator := func(ctx context.Context, originalID string, splitIndex int) string {
    return fmt.Sprintf("%s_split_%d", originalID, splitIndex)
}

splitter, err := html.NewHeaderSplitter(ctx, &html.HeaderConfig{
    Headers: map[string]string{
        "h1": "Title",
        "h2": "Section",
    },
    IDGenerator: idGenerator,
})
```

## Using in Chain

```go
import (
    "github.com/cloudwego/eino/compose"
    htmlSplitter "github.com/cloudwego/eino-ext/components/document/transformer/splitter/html"
)

splitter, _ := htmlSplitter.NewHeaderSplitter(ctx, &htmlSplitter.HeaderConfig{
    Headers: map[string]string{"h1": "Title", "h2": "Section"},
})

chain := compose.NewChain[[]*schema.Document, []*schema.Document]()
chain.AppendDocumentTransformer(splitter)

run, _ := chain.Compile(ctx)
splitDocs, _ := run.Invoke(ctx, docs)
```

## How It Works

1. **Parse HTML**: The splitter parses HTML content into a DOM tree
2. **Identify Headers**: It identifies headers specified in the config (e.g., h1, h2)
3. **Split Content**: When a header is found, content before it becomes a separate chunk
4. **Track Hierarchy**: Header text is stored as metadata, maintaining parent-child relationships
5. **Reset Hierarchy**: When a same or higher-level header is encountered, lower-level headers are cleared

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
