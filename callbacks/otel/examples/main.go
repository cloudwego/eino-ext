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
// handler into an Eino application. It sets up an OTLP HTTP exporter, registers
// the handler globally, and runs a minimal chat-model invocation so every
// component reports a GenAI semantic-convention span.
package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/callbacks/otel"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx := context.Background()

	// 1. Configure an exporter. OTEL_EXPORTER_OTLP_ENDPOINT defaults to
	//    localhost:4318 for the HTTP protocol; point it at your collector.
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(otlpEndpoint()))
	if err != nil {
		log.Fatalf("create otlp exporter: %v", err)
	}
	defer func() { _ = exporter.Shutdown(ctx) }()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)

	// 2. Register the GenAI handler once at service initialization. All Eino
	//    components (chat model, embedding, retriever, tool, graph, ADK agent)
	//    then emit spans following the OpenTelemetry GenAI semantic conventions.
	handler := otel.NewHandler(tp)
	callbacks.AppendGlobalHandlers(handler)

	// 3. Any component run from here on is traced automatically.
	_ = model.CallbackInput{Config: &model.Config{Model: "gpt-4o"}}
}

func otlpEndpoint() string {
	if ep := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); ep != "" {
		return ep
	}
	return "http://localhost:4318"
}
