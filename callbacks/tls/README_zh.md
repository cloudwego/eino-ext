# 火山引擎 TLS 回调

[English](README.md) | 简体中文

`callbacks/tls` 为 [Eino](https://github.com/cloudwego/eino) 提供基于 TLS 的可观测回调实现。

## 特性

- 实现 `github.com/cloudwego/eino/callbacks.Handler`
- 支持通过 `SetSession` 传递会话级 tracing 信息
- 支持通过 `WithCallbackDataParser` 自定义 callback 数据解析逻辑

## 安装

```bash
go get github.com/cloudwego/eino-ext/callbacks/tls
```

## 快速开始

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

	// 在这里使用带有 ctx 的 Eino graph 或 chain。
}
```

## 配置

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

### TLS exporter、大盘与 LogApp 写入

设置 `TLSExporterEnabled` 后，Eino 回调会使用 TLS Trace Schema：
`agent.turn`、`llm.request`、`tool.call` 三类 span，并补齐
`tls.app.type=eino`、`session.id`、标准 Token 字段和工具入参/结果字段；这些
字段可直接被 Eino LogApp 大盘消费。

同时设置 `TLSLog*` 字段会通过 TLS Producer 的 `SendLogs` 直接写入 Trace
Topic。认证使用 API Key 或 AK/SK 二选一，不要将任何凭证写入源码。

```go
handler, shutdown, err := tls.NewTLSHandler(&tls.TLSConfig{
	AppName:               "Eino",
	TLSExporterEnabled:    true,
	TLSLogEndpoint:        os.Getenv("TLS_LOG_ENDPOINT"),
	TLSLogRegion:          os.Getenv("TLS_LOG_REGION"),
	TLSLogTopicID:         os.Getenv("TLS_LOG_TRACE_TOPIC_ID"),
	TLSLogAPIKey:          os.Getenv("TLS_LOG_API_KEY"), // 或 TLSLogAccessKeyID/TLSLogAccessKeySecret
})
```

环境变量方式对应 `TLS_EXPORTER_ENABLED=true`，以及 `TLS_APP_NAME`、
`TLS_LOG_ENDPOINT`、`TLS_LOG_REGION`、`TLS_LOG_TRACE_TOPIC_ID` 和
`TLS_LOG_API_KEY`，或 `TLS_LOG_AK` + `TLS_LOG_SK`。

## 进阶用法

```go
handler, shutdown, err := tls.NewTLSHandlerWithOptions(cfg,
	tls.WithCallbackDataParser(customParser),
	tls.WithAggrMessageOutput(true),
)
```

### 手动 Span

对于 ACP 协议请求等不会触发 Eino callback 的工作，可使用手动 Span。

```go
ctx = handler.StartSpanWithKind(ctx, "session/prompt", trace.SpanKindServer, map[string]any{
	"rpc.system.name": "jsonrpc",
	"rpc.method":      "session/prompt",
})
defer func() {
	handler.FinishSpanWithError(ctx, nil, err)
}()
```

## 参考资料

- [火山引擎 TLS 文档](https://www.volcengine.com/docs/6431/69092)
- [Eino 文档](https://github.com/cloudwego/eino)
