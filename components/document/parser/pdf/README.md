# PDF Parser for Eino

English | [简体中文](README_zh.md)

## Introduction

This is a PDF parser component for [Eino](https://github.com/cloudwego/eino). It implements the `Parser` interface and can be seamlessly integrated into Eino's document processing workflow to parse PDF files into plain text documents.

## Features

- Implements `github.com/cloudwego/eino/components/document/parser.Parser` interface
- Parse PDF content to plain text
- Support for page-by-page parsing or full document parsing
- Font caching for improved performance
- Easy integration into Eino workflows

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/parser/pdf
```

## Quick Start

### Parse entire PDF as single document

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/document/parser/pdf"
)

func main() {
	ctx := context.Background()

	parser, err := pdf.NewPDFParser(ctx, &pdf.Config{})
	if err != nil {
		log.Fatalf("pdf.NewPDFParser failed, err=%v", err)
	}

	file, err := os.Open("document.pdf")
	if err != nil {
		log.Fatalf("os.Open failed, err=%v", err)
	}
	defer file.Close()

	docs, err := parser.Parse(ctx, file)
	if err != nil {
		log.Fatalf("parser.Parse failed, err=%v", err)
	}

	log.Printf("Parsed %d document(s)", len(docs))
	log.Printf("Content: %s", docs[0].Content)
}
```

### Parse PDF page-by-page

```go
parser, err := pdf.NewPDFParser(ctx, &pdf.Config{
	ToPages: true,
})

docs, err := parser.Parse(ctx, file)

for i, doc := range docs {
	log.Printf("Page %d: %s", i+1, doc.Content)
}
```

## Configuration

The parser can be configured through the `Config` structure:

```go
type Config struct {
    // ToPages determines whether to parse PDF page-by-page (Optional)
    // If true, each page becomes a separate document
    // If false, entire PDF is parsed as a single document
    // Default: false
    ToPages bool
}
```

## Parser Options

You can also configure the parser behavior using parser options:

```go
docs, err := parser.Parse(ctx, file, 
    pdf.WithToPages(true),
)
```

## Using in Chain

```go
import (
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/components/document"
    pdfParser "github.com/cloudwego/eino-ext/components/document/parser/pdf"
)

parser, _ := pdfParser.NewPDFParser(ctx, &pdfParser.Config{})
loader, _ := fileLoader.NewFileLoader(ctx, &fileLoader.FileLoaderConfig{
    Parser: parser,
})

chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, _ := chain.Compile(ctx)
docs, _ := run.Invoke(ctx, document.Source{URI: "document.pdf"})
```

## Important Notes

⚠️ **Alpha Stage**: This parser is in alpha stage and may not support all PDF use cases perfectly. 

Current Limitations:
- May not preserve whitespace and newlines in all cases
- Complex PDF layouts may not be parsed optimally
- Some PDF features may not be fully supported

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
