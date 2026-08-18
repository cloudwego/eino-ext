# Eino callback for Langfuse

English | [简体中文](README_zh.md)

A [CloudWeGo Eino](https://github.com/cloudwego/eino) callback that exports traces to [Langfuse](https://langfuse.com) through native OpenTelemetry OTLP/HTTP ingestion.

This package targets Langfuse v4's observations-first data model. It sends one complete immutable OTEL span per Eino operation to `/api/public/otel/v1/traces`, uses Basic Auth, and includes `x-langfuse-ingestion-version: 4` for real-time ingestion. It does not use the deprecated `/api/public/ingestion` event API.

## Features

- Eino chat models become Langfuse `generation` observations.
- Eino ADK agents, tools, retrievers, embeddings, and prompts receive specific Langfuse observation types.
- Model name, parameters, token usage, streaming output, and completion start time are captured.
- ADK async event iterators remain open until the agent has actually completed.
- Parent/child relationships use standard OpenTelemetry context propagation.
- Same-name Eino ChatModelAgent implementation Chains can be collapsed with an opt-in setting.
- Trace name, user, session, tags, release, environment, version, and metadata are propagated to child observations for Langfuse v4.
- Input/output masking and size limits are configurable.
- OTLP batching, sampling, custom exporters, custom HTTP clients, flush, and graceful shutdown are supported.

## Install

```bash
go get github.com/cloudwego/eino-ext/callbacks/langfuse/v2
```

The callback uses Go 1.23+, Eino 0.9.14, and OpenTelemetry Go 1.38.

## Usage

```go
package main

import (
	"context"
	"os"

	"github.com/cloudwego/eino/callbacks"
	langfuse "github.com/cloudwego/eino-ext/callbacks/langfuse/v2"
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
		langfuse.WithInput("Hello"),
		langfuse.WithObservationType(langfuse.ObservationTypeAgent),
	)

	// Run Eino components with traceCtx. Their callbacks become child spans.

	handler.EndTrace(traceCtx, "Hello! How can I help?")
}
```

`Config.Host` accepts a Langfuse base URL, a URL ending in `/api/public/otel`, or the full `/api/public/otel/v1/traces` endpoint.

`WithName` sets both the Langfuse trace name and the application root
observation's span name.

Discarded telemetry is never silent. Sampling, a full asynchronous queue,
callbacks received after processor shutdown, callback and OTel attribute limits,
metadata/payload serialization failures, and OTel event/link limits are
aggregated into one warning per minute:

```text
langfuse callback discarded telemetry since previous report (interval 1m0s): queue_full_spans=37 value_limit_truncations=2
```

Set `Config.DropLogInterval` to change the reporting interval. A negative value
logs each discard immediately. Export failures are always logged immediately
with the affected batch size because the processor will not retry the batch
after the exporter returns its final error.

The batch processor does not add a shorter export deadline. For the built-in
OTLP exporter, `Config.Timeout` limits each HTTP request and the OpenTelemetry
exporter's retry policy controls the total retry window. Explicit `ForceFlush`
operations pass the caller's context to the exporter and remain cancellable.

These processor-level diagnostics apply when the callback creates its own OTel
tracer provider. With a caller-owned `TracerProvider`, queueing and exporting are
owned by that provider and must be diagnosed by its span processors/exporters;
callback-level truncation and serialization diagnostics still apply.

Call `EndTrace` when the root operation completes. It records the final output and waits for active Eino child callbacks to finish before ending the root. `StartTrace` also watches the supplied context: when it is cancelled or reaches its deadline, the root waits for active child callbacks and then ends automatically. A non-cancellable context such as `context.Background()` requires `EndTrace`; `Shutdown` closes any roots that remain active during process shutdown.

User-initiated `context.Canceled` callbacks are exported with the default Langfuse level, a `cancelled` status message, and cancellation metadata instead of being counted as errors. `context.DeadlineExceeded` and other callback failures remain errors.

Resumable Eino tool, graph, subgraph, and ADK business interrupts are exported
with the default Langfuse level and an `interrupted` status instead of `ERROR`.
The interrupt cause remains available in structured observation output and
metadata. OTel instrumentation scope includes the callback release version so
traces can distinguish callback upgrades from the application `release`.

Process resource attributes are excluded by default to keep observation metadata
small and avoid exposing local owners, executable paths, or command arguments.
Set `Config.IncludeProcessResourceAttributes` only when those diagnostics are
needed. Service name, OTel SDK identity, and instrumentation scope remain present.

Set `Config.CollapseAgentInternalSpans` to remove the same-name internal Chain
created beneath an Eino ChatModelAgent's Agent observation, its internal `ReAct`
Graph and `Init` Lambda, and anonymous or default-named Lambda wrappers. It is
disabled by default to preserve complete framework traces. These names are only
collapsed beneath an active Agent; matching components elsewhere and named
business Lambdas are retained. Descendant generations, tools, and sub-agents
remain attached to the nearest retained parent through standard OTel context
propagation.

`MaxAttributeValueLength` applies a JSON-safe per-value limit. `MaxSpanAttributeBytes` limits the combined callback-owned attribute keys and values on each span. JSON inputs and outputs remain valid after either limit is applied.

Use `MaskFunc` before exporting production inputs and outputs that may contain secrets or personal data.

## Migrating from v1

The v1 callback remains available at `github.com/cloudwego/eino-ext/callbacks/langfuse`.
The v2 module uses Langfuse's OTLP ingestion API and intentionally replaces the
legacy `NewLangfuseHandler`, `SetTrace`, and `UpdateTraceOutput` lifecycle with
`NewHandler`, `StartTrace`, `EndTrace`, `Flush`, and `Shutdown`. OTLP spans are
immutable after export, so retain the context returned by `StartTrace` and pass
the final output to `EndTrace`.

## Compatibility notes

Langfuse supports OTLP over HTTP/protobuf and HTTP/JSON, but not OTLP/gRPC. This package uses HTTP/protobuf with gzip. The tracing transport is standard OTLP; the `langfuse.*` span attributes are Langfuse's vendor-specific semantic mapping.

This repository focuses on runtime Eino instrumentation. Langfuse management APIs such as prompts, datasets, scores, projects, and API keys are outside its scope.

## License

Apache-2.0
