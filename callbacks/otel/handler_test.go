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
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
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

func waitForSpan(t *testing.T, exporter *recordingExporter) sdktrace.ReadOnlySpan {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s := exporter.lastSpan(); s != nil {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for an exported span")
	return nil
}

func TestWithTracerName(t *testing.T) {
	exporter := &recordingExporter{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	h := NewHandler(tp, WithTracerName("custom/instrumentation"))

	info := &callbacks.RunInfo{Name: "llm", Component: components.ComponentOfChatModel}
	ctx := h.OnStart(context.Background(), info, nil)
	h.OnEnd(ctx, info, nil)

	span := exporter.lastSpan()
	if span == nil {
		t.Fatal("expected an exported span")
	}
	if got := span.InstrumentationScope().Name; got != "custom/instrumentation" {
		t.Fatalf("instrumentation scope name = %q, want custom/instrumentation", got)
	}
}

func TestOnEndWithoutStart(t *testing.T) {
	h, exporter := newTestHandler()
	info := &callbacks.RunInfo{Name: "llm", Component: components.ComponentOfChatModel}
	ctx := h.OnEnd(context.Background(), info, nil)
	_ = ctx
	if exporter.lastSpan() != nil {
		t.Fatal("expected no span to be exported")
	}
}

func TestOnErrorWithoutStart(t *testing.T) {
	h, exporter := newTestHandler()
	info := &callbacks.RunInfo{Name: "llm", Component: components.ComponentOfChatModel}
	ctx := h.OnError(context.Background(), info, errors.New("boom"))
	_ = ctx
	if exporter.lastSpan() != nil {
		t.Fatal("expected no span to be exported")
	}
}

func TestStreamInputDrainsSpan(t *testing.T) {
	h, exporter := newTestHandler()
	info := &callbacks.RunInfo{Name: "llm", Type: "OpenAI", Component: components.ComponentOfChatModel}
	stream := schema.StreamReaderFromArray([]callbacks.CallbackInput{
		&model.CallbackInput{Config: &model.Config{Model: "gpt-4o"}},
		nil,
	})

	ctx := h.OnStartWithStreamInput(context.Background(), info, stream)

	span := waitForSpan(t, exporter)
	attrs := spanAttrs(span)
	if v, ok := attrs[attrOperationName]; !ok || v.AsString() != "chat" {
		t.Fatalf("gen_ai.operation.name = %v, want chat", v)
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestStreamInputNilInfoClosesInput(t *testing.T) {
	h, _ := newTestHandler()
	stream := schema.StreamReaderFromArray([]callbacks.CallbackInput{nil})

	h.OnStartWithStreamInput(context.Background(), nil, stream)

	// The reader copy must have been closed by the nil-info branch.
	closed := make(chan struct{})
	go func() {
		for {
			if _, err := stream.Recv(); err != nil {
				close(closed)
				return
			}
		}
	}()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("expected stream input to be closed")
	}
}

func TestStreamOutputDrainsSpan(t *testing.T) {
	h, exporter := newTestHandler()
	info := &callbacks.RunInfo{Name: "llm", Type: "OpenAI", Component: components.ComponentOfChatModel}
	stream := schema.StreamReaderFromArray([]callbacks.CallbackOutput{
		&model.CallbackOutput{TokenUsage: &model.TokenUsage{PromptTokens: 3, CompletionTokens: 4}},
	})

	h.OnEndWithStreamOutput(context.Background(), info, stream)

	span := waitForSpan(t, exporter)
	if span == nil {
		t.Fatal("expected an exported span")
	}
}

func TestStreamOutputNilInfoClosesOutput(t *testing.T) {
	h, _ := newTestHandler()
	stream := schema.StreamReaderFromArray([]callbacks.CallbackOutput{nil})

	h.OnEndWithStreamOutput(context.Background(), nil, stream)

	closed := make(chan struct{})
	go func() {
		for {
			if _, err := stream.Recv(); err != nil {
				close(closed)
				return
			}
		}
	}()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("expected stream output to be closed")
	}
}

func TestStreamInputNilReaderEndsSpan(t *testing.T) {
	h, exporter := newTestHandler()
	info := &callbacks.RunInfo{Name: "llm", Component: components.ComponentOfChatModel}

	// A nil stream reader ends the span synchronously via drain's nil branch.
	h.OnStartWithStreamInput(context.Background(), info, nil)

	if exporter.lastSpan() == nil {
		t.Fatal("expected a span to be exported")
	}
}

func TestSpanNameFallback(t *testing.T) {
	if got := spanName(&callbacks.RunInfo{Component: components.ComponentOfTool}); got != string(components.ComponentOfTool) {
		t.Fatalf("spanName fallback = %q, want component name", got)
	}
}

func TestEmbeddingSpan(t *testing.T) {
	h, exporter := newTestHandler()
	ctx := context.Background()

	input := &embedding.CallbackInput{Config: &embedding.Config{Model: "text-embedding-3-small"}}
	info := &callbacks.RunInfo{Name: "embed", Type: "OpenAI", Component: components.ComponentOfEmbedding}

	ctx = h.OnStart(ctx, info, input)
	h.OnEnd(ctx, info, &embedding.CallbackOutput{
		Config:     &embedding.Config{Model: "text-embedding-3-small"},
		TokenUsage: &embedding.TokenUsage{PromptTokens: 5},
	})

	attrs := spanAttrs(exporter.lastSpan())
	if v, ok := attrs[attrOperationName]; !ok || v.AsString() != "embeddings" {
		t.Fatalf("gen_ai.operation.name = %v, want embeddings", v)
	}
	if v, ok := attrs[attrRequestModel]; !ok || v.AsString() != "text-embedding-3-small" {
		t.Fatalf("gen_ai.request.model = %v, want text-embedding-3-small", v)
	}
	if v, ok := attrs[attrUsageInput]; !ok || v.AsInt64() != 5 {
		t.Fatalf("gen_ai.usage.input_tokens = %v, want 5", v)
	}
}

func TestOperationNames(t *testing.T) {
	tests := []struct {
		component components.Component
		want      string
		ok        bool
	}{
		{components.ComponentOfChatModel, "chat", true},
		{components.ComponentOfAgenticModel, "chat", true},
		{components.ComponentOfEmbedding, "embeddings", true},
		{components.ComponentOfIndexer, "embeddings", true},
		{components.ComponentOfRetriever, "retrieve", true},
		{components.ComponentOfTool, "execute_tool", true},
		{compose.ComponentOfToolsNode, "execute_tool", true},
		{compose.ComponentOfAgenticToolsNode, "execute_tool", true},
		{adk.ComponentOfAgent, "invoke_agent", true},
		{adk.ComponentOfAgenticAgent, "invoke_agent", true},
		{compose.ComponentOfGraph, "invoke_agent", true},
		{compose.ComponentOfWorkflow, "invoke_agent", true},
		{compose.ComponentOfChain, "invoke_agent", true},
		{components.ComponentOfPrompt, "", false},
	}
	for _, tt := range tests {
		got, ok := operationName(tt.component)
		if ok != tt.ok || got != tt.want {
			t.Errorf("operationName(%q) = (%q, %v), want (%q, %v)", tt.component, got, ok, tt.want, tt.ok)
		}
	}
}
