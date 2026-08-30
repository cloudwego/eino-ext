# Eino OpenTelemetry Callback

A backend-agnostic OpenTelemetry callback handler for [Eino](https://github.com/cloudwego/eino)
that reports component invocations as spans following the OpenTelemetry
[GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/).

Unlike the dedicated Langfuse / Cozeloop integrations, this handler emits
standard spans to whatever exporter is configured on your `TracerProvider`,
so it works with any OTLP collector or APM backend.

## Usage

```go
import (
    "go.opentelemetry.io/otel"
    einootel "github.com/cloudwego/eino-ext/callbacks/otel"
    "github.com/cloudwego/eino/callbacks"
)

// 1. Configure an OTel SDK TracerProvider with your exporter as usual.
// 2. Register the handler globally:
callbacks.AppendGlobalHandlers(einootel.NewHandler(otel.GetTracerProvider()))
```

## Reported attributes

| Attribute | Source |
| --- | --- |
| `gen_ai.operation.name` | Mapped from the component kind: `chat`, `embeddings`, `retrieve`, `execute_tool`, `invoke_agent` |
| `gen_ai.system` | Component implementation type (lowercased), e.g. `openai` |
| `gen_ai.request.model` | Model name from chat model / embedding callbacks |
| `gen_ai.usage.input_tokens` | Prompt tokens from chat model / embedding callbacks |
| `gen_ai.usage.output_tokens` | Completion tokens from chat model callbacks |

Streaming invocations are reported as spans covering the lifetime of the
stream; usage attributes are only populated for non-streaming calls.
