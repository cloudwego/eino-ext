# Volcengine TLS Callbacks

English | [简体中文](README_zh.md)

`callbacks/tls` provides a TLS-based observability handler for [Eino](https://github.com/cloudwego/eino).

## Features

- Implements `github.com/cloudwego/eino/callbacks.Handler`
- Supports session-scoped tracing via `SetSession`
- Supports custom callback parsers through `WithCallbackDataParser`

## Installation

```bash
go get github.com/cloudwego/eino-ext/callbacks/tls
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/callbacks/tls"
	"github.com/cloudwego/eino/callbacks"
)

func main() {
	ctx := tls.SetSession(context.Background(),
		tls.WithSessionID("session-id"),
		tls.WithUserID("user-id"),
	)

	handler, shutdown, err := tls.NewTLSHandler(&tls.TLSConfig{
		TLSEndpoint:       "tls-cn-beijing.volces.com:4317",
		AppName:           "eino-app",
		TLSOTLPHeadersStr: "Authorization=Bearer <token>",
		Release:           "v0.0.1",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if shutdown != nil {
			_ = shutdown(ctx)
		}
	}()

	callbacks.AppendGlobalHandlers(handler)

	// Build and run your Eino graph or chain with ctx here.
}
```

## Configuration

```go
type TLSConfig struct {
	TLSEndpoint       string
	AppName           string
	TLSOTLPHeader     map[string]string
	TLSOTLPHeadersStr string
	Release           string
}
```

## Advanced Usage

```go
handler, shutdown, err := tls.NewTLSHandlerWithOptions(cfg,
	tls.WithCallbackDataParser(customParser),
	tls.WithAggrMessageOutput(true),
)
```

## References

- [Volcengine TLS Documentation](https://www.volcengine.com/docs/6431/69092)
- [Eino Documentation](https://github.com/cloudwego/eino)
