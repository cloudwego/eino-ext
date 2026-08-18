# Eino Langfuse 回调

[English](README.md) | 简体中文

这是一个 [CloudWeGo Eino](https://github.com/cloudwego/eino) 回调，通过原生 OpenTelemetry OTLP/HTTP 将 trace 导出到 [Langfuse](https://langfuse.com)。

该包面向 Langfuse v4 的 observation-first 数据模型。每个 Eino 操作会上报一个完整且不可变的 OTel span 到 `/api/public/otel/v1/traces`，使用 Basic Auth，并携带 `x-langfuse-ingestion-version: 4` 以支持实时摄取。它不再使用已弃用的 `/api/public/ingestion` 事件接口。

## 功能

- 将 Eino ChatModel 映射为 Langfuse `generation` observation。
- 为 Eino ADK Agent、Tool、Retriever、Embedding 和 Prompt 设置对应的 observation 类型。
- 采集模型名称、参数、token usage、流式输出以及首 token 时间。
- 在 ADK 异步事件迭代器真正结束后再结束 Agent observation。
- 使用标准 OpenTelemetry context 传播父子关系。
- 可选择折叠 Eino Agent 产生的框架内部 Chain、ReAct Graph 和 Lambda 包装层。
- 将 trace 名称、用户、会话、标签、release、environment、version 和 metadata 传播到子 observation。
- 支持 input/output 脱敏、大小限制、OTLP 批处理、采样、自定义 exporter、自定义 HTTP client、flush 和优雅关闭。

## 安装

```bash
go get github.com/cloudwego/eino-ext/callbacks/langfuse/v2
```

当前实现使用 Go 1.23+、Eino 0.9.14 和 OpenTelemetry Go 1.38。

## 使用方法

```go
package main

import (
	"context"
	"os"

	"github.com/cloudwego/eino-ext/callbacks/langfuse/v2"
	"github.com/cloudwego/eino/callbacks"
)

func main() {
	ctx := context.Background()
	handler, err := langfuse.NewHandler(ctx, &langfuse.Config{
		Host:        os.Getenv("LANGFUSE_HOST"),
		PublicKey:   os.Getenv("LANGFUSE_PUBLIC_KEY"),
		SecretKey:   os.Getenv("LANGFUSE_SECRET_KEY"),
		ServiceName: "my-eino-service",
		Environment: "production",
	})
	if err != nil {
		panic(err)
	}
	defer handler.Shutdown(ctx)

	callbacks.AppendGlobalHandlers(handler)

	traceCtx := handler.StartTrace(ctx,
		langfuse.WithName("chat-response"),
		langfuse.WithUserID("user-123"),
		langfuse.WithSessionID("conversation-456"),
		langfuse.WithInput("你好"),
		langfuse.WithObservationType(langfuse.ObservationTypeAgent),
	)

	// 使用 traceCtx 运行 Eino 组件，其 callback 会成为 root 的子 span。

	handler.EndTrace(traceCtx, "你好！有什么可以帮你？")
}
```

`Config.Host` 可以是 Langfuse 基础地址、以 `/api/public/otel` 结尾的 OTLP 地址，或完整的 `/api/public/otel/v1/traces` 地址。

`WithName` 会同时设置 Langfuse trace 名称和应用 root observation 的 span 名称。

## 生命周期

应用根操作结束时调用 `EndTrace`。它会记录最终 output，并等待所有仍活跃的 Eino child callback 完成后再结束 root。`StartTrace` 也会监听传入 context：context 被取消或超时时，root 同样会等待活跃 child，然后自动结束。对于 `context.Background()` 等不可取消 context，必须调用 `EndTrace`；进程退出时，`Shutdown` 会关闭仍然活跃的 root。

用户主动触发的 `context.Canceled` 会记录为 `cancelled`，可恢复的 Eino Tool、Graph、SubGraph 和 ADK interrupt 会记录为 `interrupted`，两者都不会统一标记成 `ERROR`。真正的超时和 callback 失败仍会标记为错误。

## Trace 精简

设置 `Config.CollapseAgentInternalSpans` 可折叠 Eino Agent 下同名的内部 Chain、内部 `ReAct` Graph、`Init` Lambda，以及匿名或默认命名的 Lambda 包装层。默认关闭，以保留完整的框架 trace。Generation、Tool 和 Sub-Agent 会通过标准 OTel context 继续挂在最近的保留父节点下。

默认不携带 `process.*` resource attribute，以减少 metadata 噪声并避免暴露本地用户名、可执行文件路径和命令参数。只有确实需要这些诊断信息时才启用 `Config.IncludeProcessResourceAttributes`。

## 丢弃和错误日志

采样、队列已满、processor 已关闭、callback/OTel 属性限制以及序列化失败不会静默发生。默认每分钟汇总一次：

```text
langfuse callback discarded telemetry since previous report (interval 1m0s): queue_full_spans=37 value_limit_truncations=2
```

使用 `Config.DropLogInterval` 调整汇总间隔；设置为负数会立即记录每次丢弃。exporter 最终返回错误时，会立即打印失败 batch 的 span 数量，因为该 batch 不会再次重试。

当 callback 使用调用方传入的 `TracerProvider` 时，队列和导出由该 provider 管理，应通过其 span processor/exporter 诊断；callback 自身的截断和序列化诊断仍然有效。

## 大小限制和脱敏

`MaxAttributeValueLength` 限制单个 JSON 属性值；`MaxSpanAttributeBytes` 限制 callback 写入一个 span 的属性键值总大小。任一限制生效后，JSON input/output 仍会保持合法。设置为负数可禁用对应限制。

生产环境建议配置 `MaskFunc`，在导出前处理可能包含密钥或个人数据的 input/output。

## 从 v1 迁移

v1 callback 继续保留在 `github.com/cloudwego/eino-ext/callbacks/langfuse`。v2 module 改用 Langfuse OTLP ingestion API，并有意使用 `NewHandler`、`StartTrace`、`EndTrace`、`Flush` 和 `Shutdown` 取代旧的 `NewLangfuseHandler`、`SetTrace` 和 `UpdateTraceOutput` 生命周期。OTLP span 导出后不可修改，请保留 `StartTrace` 返回的 context，并通过 `EndTrace` 提交最终 output。

## 兼容性

Langfuse 支持 OTLP over HTTP/protobuf 和 HTTP/JSON，但不支持 OTLP/gRPC。本包使用带 gzip 的 HTTP/protobuf。传输协议是标准 OTLP，`langfuse.*` span attribute 是 Langfuse 提供的厂商语义映射。

该包只负责运行时 Eino instrumentation。Prompt、Dataset、Score、Project 和 API Key 等 Langfuse 管理接口不在其范围内。

## License

Apache-2.0
