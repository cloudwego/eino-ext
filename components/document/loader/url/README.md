# URL Loader for Eino

## Introduction

This is a URL loader component for [Eino](https://github.com/cloudwego/eino). It implements the `Loader` interface and can be seamlessly integrated into Eino's document processing workflow to load documents from URLs.

## Features

- Implements `github.com/cloudwego/eino/components/document.Loader` interface
- Easy integration into Eino workflows
- Supports loading documents from HTTP/HTTPS URLs
- Customizable HTTP client and request builder
- Built-in HTML parser with configurable selectors
- Built-in callback support

## Installation

```bash
go get github.com/cloudwego/eino-ext/components/document/loader/url
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino-ext/components/document/loader/url"
)

func main() {
	ctx := context.Background()

	loader, err := url.NewLoader(ctx, &url.LoaderConfig{})
	if err != nil {
		log.Fatalf("NewLoader failed, err=%v", err)
	}

	docs, err := loader.Load(ctx, document.Source{
		URI: "https://example.com/page.html",
	})
	if err != nil {
		log.Fatalf("Load failed, err=%v", err)
	}

	for _, doc := range docs {
		log.Printf("Content: %s\n", doc.Content)
	}
}
```

## Configuration

The loader can be configured through the `LoaderConfig` structure:

```go
type LoaderConfig struct {
    // Parser specifies the parser to use for response content
    // Optional. Default: HTML parser with body selector
    Parser parser.Parser
    
    // Client specifies the HTTP client to use
    // Optional. Default: http.DefaultClient
    Client *http.Client
    
    // RequestBuilder customizes the HTTP request
    // Optional. Default: GET request builder
    RequestBuilder func(ctx context.Context, source document.Source, opts ...document.LoaderOption) (*http.Request, error)
}
```

## Advanced Usage

### Custom HTTP Client with Proxy

```go
proxyURL, _ := url.Parse("http://proxy.example.com:8080")
client := &http.Client{
    Transport: &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    },
}

loader, err := url.NewLoader(ctx, &url.LoaderConfig{
    Client: client,
})
```

### Custom Request Builder with Authentication

```go
requestBuilder := func(ctx context.Context, source document.Source, opts ...document.LoaderOption) (*http.Request, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", source.URI, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer YOUR_TOKEN")
    return req, nil
}

loader, err := url.NewLoader(ctx, &url.LoaderConfig{
    RequestBuilder: requestBuilder,
})
```

### Custom Parser

```go
customParser, err := html.NewParser(ctx, &html.Config{
    Selector: &html.ArticleSelector,
})

loader, err := url.NewLoader(ctx, &url.LoaderConfig{
    Parser: customParser,
})
```

## Examples

See the following examples for more usage:

- [Authentication](./examples/auth/)
- [Directory Path](./examples/dirpath/)
- [HTML Loading](./examples/html/)
- [Proxy Configuration](./examples/proxy/)
- [Test Data Example](./examples/testdata/)

## License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.
