# File Loader for Eino

## Introduction

This is a file loader component for [Eino](https://github.com/cloudwego/eino). It implements the `Loader` interface and can be seamlessly integrated into Eino's document processing workflow to load local files.

## Features

- Implements `github.com/cloudwego/eino/components/document.Loader` interface
- Easy integration into Eino workflows
- Supports automatic file parsing based on extension
- Customizable parser configuration
- Built-in callback support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/loader/file
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino-ext/components/document/loader/file"
)

func main() {
	ctx := context.Background()

	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		UseNameAsID: true,
	})
	if err != nil {
		log.Fatalf("file.NewFileLoader failed, err=%v", err)
	}

	filePath := "./document.txt"
	docs, err := loader.Load(ctx, document.Source{
		URI: filePath,
	})
	if err != nil {
		log.Fatalf("loader.Load failed, err=%v", err)
	}

	log.Printf("doc content: %v", docs[0].Content)
	log.Printf("Extension: %s\n", docs[0].MetaData[file.MetaKeyExtension])
	log.Printf("Source: %s\n", docs[0].MetaData[file.MetaKeySource])
}
```

## Configuration

The loader can be configured through the `FileLoaderConfig` structure:

```go
type FileLoaderConfig struct {
    // UseNameAsID uses the file name as the document ID
    // Optional. Default: false
    UseNameAsID bool
    
    // Parser specifies the parser to use for file content
    // Optional. Default: ExtParser with TextParser fallback
    Parser parser.Parser
}
```

## Metadata

The loader automatically adds the following metadata to loaded documents:

- `_file_name`: File name (basename)
- `_extension`: File extension
- `_source`: File path (URI)

## Using in Chain

```go
chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, err := chain.Compile(ctx)
if err != nil {
    log.Fatalf("chain.Compile failed, err=%v", err)
}

docs, err := run.Invoke(ctx, document.Source{URI: filePath})
```

## Examples

For more examples, please refer to the [examples](./examples) directory:

- [fileloader](./examples/fileloader) - Basic file loading usage
- [customloader](./examples/customloader) - Custom loader implementation

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
