# tls-eino-demo

A minimal, runnable Eino demo that ships traces to **Volcengine TLS** through the
[`callbacks/tls`](../../callbacks/tls) handler.

It builds a real `compose.Chain` (chat prompt template → chat model) and invokes
it both non-streaming and streaming. The TLS handler is registered as a **global
Eino callback**, so the `agent.turn` / `llm.request` spans are produced by the
framework itself — this is what a production integration looks like. A mock
`ChatModel` stands in for a real provider so the demo runs without LLM
credentials; swap it for `components/model/ark` and the spans stay identical.

## Run

```bash
export TLS_EXPORTER_ENABLED=true
export TLS_APP_NAME=tls-eino-demo
export TLS_LOG_ENDPOINT=tls-cn-beijing2.volces.com   # optional; derived from region if empty
export TLS_LOG_REGION=cn-beijing2
export TLS_LOG_TRACE_TOPIC_ID=<your-trace-topic-id>
export TLS_LOG_API_KEY=<your-tls-api-key>            # or TLS_LOG_AK + TLS_LOG_SK

go run .
```

On success the traces appear in the TLS LLM dashboard, grouped by the demo's
`session_id`. If the TLS service rejects the credentials you'll see a
`403` from the producer (e.g. `InvalidAccessKeyId` / API-key errors) — that means
the pipeline works and only the credentials need fixing.

## What it does

1. Resolves `TLSConfig` from the environment (defaulting the endpoint from the
   region when `TLS_LOG_ENDPOINT` is empty).
2. Creates the TLS handler and registers it via `callbacks.AppendGlobalHandlers`.
3. Attaches a session via `tls.SetSession` so traces group per session.
4. Runs `Chain.Invoke` and `Chain.Stream`; the framework fires callbacks that the
   handler converts to TLS spans and flushes on shutdown.
