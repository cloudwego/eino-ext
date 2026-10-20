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

// Package otel provides a backend-agnostic OpenTelemetry callback handler for
// Eino that reports component invocations as spans following the OpenTelemetry
// GenAI semantic conventions.
//
// It maps each component kind to gen_ai.operation.name and attaches
// gen_ai.system, gen_ai.request.model, and gen_ai.usage.* attributes when the
// corresponding component payload carries that information (chat models and
// embeddings today).
//
// Usage:
//
//	import (
//	    "go.opentelemetry.io/otel"
//	    einootel "github.com/cloudwego/eino-ext/callbacks/otel"
//	)
//
//	handler := einootel.NewHandler(otel.GetTracerProvider())
//	callbacks.AppendGlobalHandlers(handler)
//
// Span output is sent to whatever exporter is configured on the shared
// TracerProvider, so it works with any OTLP collector or APM backend.
package otel
