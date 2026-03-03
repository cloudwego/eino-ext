# Markdown Header Splitter

English | [简体中文](README_zh.md)

A Markdown document splitter that splits documents based on header levels. This transformer is designed for [Eino](https://github.com/cloudwego/eino) and helps organize Markdown content by splitting at header boundaries while preserving header hierarchy as metadata.

## Features

- Splits Markdown documents based on configurable header levels (e.g., `#`, `##`, `###`)
- Preserves header hierarchy as document metadata
- Handles code blocks correctly (does not split within code blocks)
- Optional header trimming from split content
- Customizable ID generation for split documents

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/cloudwego/eino/schema"
    "github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
)

func main() {
    ctx := context.Background()
    
    transformer, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
        Headers: map[string]string{
            "#":   "h1",
            "##":  "h2",
            "###": "h3",
        },
        TrimHeaders: true,
    })
    if err != nil {
        log.Fatalf("failed to create splitter: %v", err)
    }
    
    doc := &schema.Document{
        Content: "# Title\nIntro content\n## Section 1\nSection content",
    }
    
    splitDocs, err := transformer.Transform(ctx, []*schema.Document{doc})
    if err != nil {
        log.Fatalf("failed to transform: %v", err)
    }
    
    for _, doc := range splitDocs {
        log.Printf("Content: %s, Metadata: %v\n", doc.Content, doc.MetaData)
    }
}
```

## Configuration

The splitter can be configured using the `HeaderConfig` struct:

```go
type HeaderConfig struct {
    // Headers specify the headers to be identified and their names in document metadata.
    // Headers can only consist of '#'.
    // Key: header pattern (e.g., "##")
    // Value: metadata key name
    Headers map[string]string
    
    // TrimHeaders specifies if results contain header lines.
    // If true, header lines are removed from the split content.
    // If false, header lines are included in the split content.
    TrimHeaders bool
    
    // IDGenerator is an optional function to generate new IDs for split chunks.
    // If nil, the original document ID will be used for all splits.
    IDGenerator IDGenerator
}

type IDGenerator func(ctx context.Context, originalID string, splitIndex int) string
```

### Configuration Options

- **Headers** (required): A map defining which header levels to split on and what to call them in metadata
  - Keys must only contain `#` characters
  - Example: `{"#": "Title", "##": "Section", "###": "Subsection"}`

- **TrimHeaders** (optional, default: `false`): Controls whether header lines are included in split content
  - `true`: Remove header lines from the content
  - `false`: Keep header lines in the content

- **IDGenerator** (optional): Custom function to generate document IDs for splits
  - Default behavior: Uses the original document ID for all splits

## Examples

See the following examples for more usage:

- [Header Splitter](./examples/headersplitter/)

## How It Works

1. **Header Detection**: The splitter scans through the document line by line, looking for lines that start with the configured header patterns.

2. **Code Block Handling**: Code blocks (enclosed by ` ``` ` or `~~~`) are properly handled - the splitter does not split content within code blocks.

3. **Header Hierarchy**: When a header is encountered, it's added to the metadata. If a new header of the same or higher level is found, previous headers at that level or lower are replaced in the metadata.

4. **Content Splitting**: Content is split at header boundaries. Each split document contains:
   - The content between headers
   - Metadata containing all active headers in the hierarchy
   - An ID (original or generated)

## For More Details

- [Eino Documentation](https://www.cloudwego.io/zh/docs/eino/)
- [Example Code](examples/headersplitter/main.go)
