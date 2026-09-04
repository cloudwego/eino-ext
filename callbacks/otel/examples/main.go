/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Command otel-example demonstrates wiring the OpenTelemetry GenAI callback
// handler into an Eino application. It registers the handler globally so every
// component run emits a span following the OpenTelemetry GenAI semantic
// conventions.
//
// In a real service, replace otel.GetTracerProvider() with the SDK
// TracerProvider already configured with your OTLP exporter.
package main

import (
	"context"
	"log"

	otelcallback "github.com/cloudwego/eino-ext/callbacks/otel"
	"github.com/cloudwego/eino/callbacks"

	"go.opentelemetry.io/otel"
)

func main() {
	// Use the globally-configured TracerProvider. Set it once at startup with
	// your OTLP exporter, e.g.:
	//
	//   tp := sdktrace.NewTracerProvider(
	//       sdktrace.WithBatcher(otlptracehttp.New(ctx)),
	//   )
	//   otel.SetTracerProvider(tp)
	tp := otel.GetTracerProvider()

	// Register the GenAI handler once. All Eino components (chat model,
	// embedding, retriever, tool, graph, ADK agent) then emit GenAI
	// semantic-convention spans.
	handler := otelcallback.NewHandler(tp)
	callbacks.AppendGlobalHandlers(handler)

	log.Println("OpenTelemetry GenAI callback handler registered")
	_ = context.Background()
}
