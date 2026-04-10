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
}
```

## 进阶用法

```go
handler, shutdown, err := tls.NewTLSHandlerWithOptions(cfg,
	tls.WithCallbackDataParser(customParser),
	tls.WithAggrMessageOutput(true),
)
```

## 参考资料

- [火山引擎 TLS 文档](https://www.volcengine.com/docs/6431/69092)
- [Eino 文档](https://github.com/cloudwego/eino)
