# S3 Loader for Eino

English | [简体中文](README_zh.md)

## Introduction

This is an AWS S3 loader component for [Eino](https://github.com/cloudwego/eino). It implements the `Loader` interface and can be seamlessly integrated into Eino's document processing workflow to load documents from AWS S3 buckets.

## Features

- Implements `github.com/cloudwego/eino/components/document.Loader` interface
- Load documents from AWS S3 buckets
- Support for AWS authentication with access key/secret key
- Customizable parser configuration
- Built-in callback support
- Easy integration into Eino workflows

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/loader/s3
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino-ext/components/document/loader/s3"
)

func main() {
	ctx := context.Background()

	region := "us-east-1"
	accessKey := "your-access-key"
	secretKey := "your-secret-key"

	loader, err := s3.NewS3Loader(ctx, &s3.LoaderConfig{
		Region:           &region,
		AWSAccessKey:     &accessKey,
		AWSSecretKey:     &secretKey,
		UseObjectKeyAsID: true,
	})
	if err != nil {
		log.Fatalf("s3.NewS3Loader failed, err=%v", err)
	}

	docs, err := loader.Load(ctx, document.Source{
		URI: "s3://my-bucket/path/to/document.txt",
	})
	if err != nil {
		log.Fatalf("loader.Load failed, err=%v", err)
	}

	log.Printf("Loaded %d documents", len(docs))
	log.Printf("Document content: %v", docs[0].Content)
}
```

## Configuration

The loader can be configured through the `LoaderConfig` structure:

```go
type LoaderConfig struct {
    // Region is the AWS region of the bucket (Optional)
    // Example: "us-east-1"
    Region *string
    
    // AWSAccessKey is the AWS access key for authentication (Optional)
    // If not provided, uses default AWS credentials
    AWSAccessKey *string
    
    // AWSSecretKey is the AWS secret key for authentication (Optional)
    // Must be provided together with AWSAccessKey
    AWSSecretKey *string
    
    // UseObjectKeyAsID uses the S3 object key as document ID (Optional)
    // Default: false
    UseObjectKeyAsID bool
    
    // Parser specifies the parser to use for file content (Optional)
    // Default: TextParser
    Parser parser.Parser
}
```

## URI Format

The loader expects S3 URIs in the following format:

```
s3://bucket-name/object-key
```

For example:
- `s3://my-bucket/documents/file.txt`
- `s3://data-bucket/reports/2024/report.pdf`

**Note**: Batch loading with prefix (e.g., `s3://bucket/prefix/`) is not currently supported.

## Authentication

The loader supports two authentication methods:

1. **Explicit credentials**: Provide `AWSAccessKey` and `AWSSecretKey` in the config
2. **Default AWS credentials**: If keys are not provided, the loader will use the default AWS credentials chain (environment variables, shared credentials file, IAM role, etc.)

## Using in Chain

```go
chain := compose.NewChain[document.Source, []*schema.Document]()
chain.AppendLoader(loader)

run, err := chain.Compile(ctx)
if err != nil {
    log.Fatalf("chain.Compile failed, err=%v", err)
}

docs, err := run.Invoke(ctx, document.Source{URI: "s3://my-bucket/file.txt"})
```

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
