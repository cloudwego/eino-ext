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
	TLSExporterEnabled bool
	TLSLogEndpoint    string
	TLSLogRegion      string
	TLSLogTopicID     string
	TLSLogAPIKey      string
	TLSLogAccessKeyID string
	TLSLogAccessKeySecret string
}
```

### TLS exporter, dashboard, and LogApp transport

Set `TLSExporterEnabled` to use the TLS trace schema. Eino callbacks
then emit `agent.turn`, `llm.request`, and `tool.call` spans; they include
`tls.app.type=eino`, `session.id`, standard model token fields, and tool
input/output fields. This is the schema consumed by the Eino LogApp dashboard.

Set the `TLSLog*` fields to write directly to the Trace Topic through TLS
Producer `SendLogs`. Use either an API key or an AK/SK pair, never hard-code
either in source code.

```go
handler, shutdown, err := tls.NewTLSHandler(&tls.TLSConfig{
	AppName:               "Eino",
	TLSExporterEnabled:    true,
	TLSLogEndpoint:        os.Getenv("TLS_LOG_ENDPOINT"),
	TLSLogRegion:          os.Getenv("TLS_LOG_REGION"),
	TLSLogTopicID:         os.Getenv("TLS_LOG_TRACE_TOPIC_ID"),
	TLSLogAPIKey:          os.Getenv("TLS_LOG_API_KEY"), // or TLSLogAccessKeyID/TLSLogAccessKeySecret
})
```

The equivalent environment-based configuration is `TLS_EXPORTER_ENABLED=true`
together with `TLS_APP_NAME`, `TLS_LOG_ENDPOINT`, `TLS_LOG_REGION`,
`TLS_LOG_TRACE_TOPIC_ID`, and either `TLS_LOG_API_KEY` or `TLS_LOG_AK` plus
`TLS_LOG_SK`.

## Advanced Usage

```go
handler, shutdown, err := tls.NewTLSHandlerWithOptions(cfg,
	tls.WithCallbackDataParser(customParser),
	tls.WithAggrMessageOutput(true),
)
```

### Manual Spans

Use manual spans for work that does not emit Eino callbacks, such as an ACP protocol request.

```go
ctx = handler.StartSpanWithKind(ctx, "session/prompt", trace.SpanKindServer, map[string]any{
	"rpc.system.name": "jsonrpc",
	"rpc.method":      "session/prompt",
})
defer func() {
	handler.FinishSpanWithError(ctx, nil, err)
}()
```

## References

- [Volcengine TLS Documentation](https://www.volcengine.com/docs/6431/69092)
- [Eino Documentation](https://github.com/cloudwego/eino)
