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

package otel

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type recordingExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *recordingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.spans = append(e.spans, spans...)
	return nil
}

func (e *recordingExporter) Shutdown(_ context.Context) error { return nil }

func (e *recordingExporter) lastSpan() sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.spans) == 0 {
		return nil
	}
	return e.spans[len(e.spans)-1]
}

func newTestHandler() (callbacks.Handler, *recordingExporter) {
	exporter := &recordingExporter{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	return NewHandler(tp), exporter
}

func spanAttrs(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	if span == nil {
		return nil
	}
	m := make(map[attribute.Key]attribute.Value, len(span.Attributes()))
	for _, kv := range span.Attributes() {
		m[kv.Key] = kv.Value
	}
	return m
}

func TestChatModelSpan(t *testing.T) {
	h, exporter := newTestHandler()
	ctx := context.Background()

	input := &model.CallbackInput{
		Config:   &model.Config{Model: "gpt-4o"},
		Messages: []*schema.Message{},
	}
	info := &callbacks.RunInfo{Name: "llm", Type: "OpenAI", Component: components.ComponentOfChatModel}

	ctx = h.OnStart(ctx, info, input)
	h.OnEnd(ctx, info, &model.CallbackOutput{
		Config:     &model.Config{Model: "gpt-4o"},
		TokenUsage: &model.TokenUsage{PromptTokens: 12, CompletionTokens: 7},
	})

	attrs := spanAttrs(exporter.lastSpan())
	if v, ok := attrs[attrOperationName]; !ok || v.AsString() != "chat" {
		t.Fatalf("gen_ai.operation.name = %v, want chat", v)
	}
	if v, ok := attrs[attrSystem]; !ok || v.AsString() != "openai" {
		t.Fatalf("gen_ai.system = %v, want openai", v)
	}
	if v, ok := attrs[attrRequestModel]; !ok || v.AsString() != "gpt-4o" {
		t.Fatalf("gen_ai.request.model = %v, want gpt-4o", v)
	}
	if v, ok := attrs[attrUsageInput]; !ok || v.AsInt64() != 12 {
		t.Fatalf("gen_ai.usage.input_tokens = %v, want 12", v)
	}
	if v, ok := attrs[attrUsageOutput]; !ok || v.AsInt64() != 7 {
		t.Fatalf("gen_ai.usage.output_tokens = %v, want 7", v)
	}
}

func TestToolSpanOperationName(t *testing.T) {
	h, exporter := newTestHandler()
	ctx := context.Background()

	info := &callbacks.RunInfo{Name: "search", Type: "SearchTool", Component: components.ComponentOfTool}
	ctx = h.OnStart(ctx, info, nil)
	h.OnEnd(ctx, info, nil)

	attrs := spanAttrs(exporter.lastSpan())
	if v, ok := attrs[attrOperationName]; !ok || v.AsString() != "execute_tool" {
		t.Fatalf("gen_ai.operation.name = %v, want execute_tool", v)
	}
}

func TestOnErrorRecordsStatus(t *testing.T) {
	h, exporter := newTestHandler()
	ctx := context.Background()

	info := &callbacks.RunInfo{Name: "llm", Type: "OpenAI", Component: components.ComponentOfChatModel}
	ctx = h.OnStart(ctx, info, nil)
	h.OnError(ctx, info, errors.New("boom"))

	span := exporter.lastSpan()
	if span == nil {
		t.Fatal("expected an exported span")
	}
	if span.Status().Code != codes.Error {
		t.Fatalf("span status code = %v, want error", span.Status().Code)
	}
}

func TestNilRunInfoIsIgnored(t *testing.T) {
	h, _ := newTestHandler()
	ctx := context.Background()

	ctx = h.OnStart(ctx, nil, nil)
	h.OnEnd(ctx, nil, nil)
	h.OnError(ctx, nil, errors.New("boom"))
	// Reaching here without panicking is the assertion.
}
