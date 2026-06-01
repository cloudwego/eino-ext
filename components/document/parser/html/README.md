# HTML Parser for Eino

English | [简体中文](README_zh.md)

## Introduction

This is an HTML parser component for [Eino](https://github.com/cloudwego/eino). It implements the `Parser` interface and can be seamlessly integrated into Eino's document processing workflow to parse HTML content into structured documents.

## Features

- Implements `github.com/cloudwego/eino/components/document/parser.Parser` interface
- Parse HTML content to plain text
- Extract metadata from HTML (title, description, language, charset)
- Customizable content selector using CSS selector syntax
- HTML sanitization using bluemonday
- Easy integration into Eino workflows

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/parser/html
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"strings"

	"github.com/cloudwego/eino-ext/components/document/parser/html"
)

func main() {
	ctx := context.Background()

	parser, err := html.NewParser(ctx, &html.Config{})
	if err != nil {
		log.Fatalf("html.NewParser failed, err=%v", err)
	}

	htmlContent := `
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="description" content="Sample page">
			<title>Sample Page</title>
		</head>
		<body>
			<h1>Hello World</h1>
			<p>This is a sample HTML document.</p>
		</body>
		</html>
	`

	reader := strings.NewReader(htmlContent)
	docs, err := parser.Parse(ctx, reader)
	if err != nil {
		log.Fatalf("parser.Parse failed, err=%v", err)
	}

	log.Printf("Content: %s", docs[0].Content)
	log.Printf("Title: %s", docs[0].MetaData[html.MetaKeyTitle])
	log.Printf("Description: %s", docs[0].MetaData[html.MetaKeyDesc])
	log.Printf("Language: %s", docs[0].MetaData[html.MetaKeyLang])
}
```

## Configuration

The parser can be configured through the `Config` structure:

```go
type Config struct {
    // Selector is a CSS selector to extract specific content (Optional)
    // Examples: "body" for <body>, "#content" for <div id="content">
    // Default: entire document
    Selector *string
}
```

## Metadata

The parser automatically extracts and adds the following metadata to parsed documents:

- `_title`: Document title from `<title>` tag
- `_description`: Description from `<meta name="description">` tag
- `_language`: Language from `<html lang="">` attribute
- `_charset`: Character encoding from `<meta charset="">` tag
- `_source`: Source URI (if provided via parser options)

## Using Custom Selector

You can extract content from specific parts of the HTML:

```go
bodySelector := "body"
parser, err := html.NewParser(ctx, &html.Config{
    Selector: &bodySelector,
})

contentSelector := "#main-content"
parser, err := html.NewParser(ctx, &html.Config{
    Selector: &contentSelector,
})
```

## Using in Chain

```go
import (
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/components/document"
    htmlParser "github.com/cloudwego/eino-ext/components/document/parser/html"
)

parser, _ := htmlParser.NewParser(ctx, &htmlParser.Config{})
loader, _ := urlLoader.NewURLLoader(ctx, &urlLoader.LoaderConfig{
    Parser: parser,
})

chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, _ := chain.Compile(ctx)
docs, _ := run.Invoke(ctx, document.Source{URI: "https://example.com"})
```

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
