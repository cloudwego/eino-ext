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

package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceEndpoint(t *testing.T) {
	tests := map[string]string{
		"https://us.cloud.langfuse.com":                           "https://us.cloud.langfuse.com/api/public/otel/v1/traces",
		"https://us.cloud.langfuse.com/api/public/otel":           "https://us.cloud.langfuse.com/api/public/otel/v1/traces",
		"https://us.cloud.langfuse.com/api/public/otel/v1/traces": "https://us.cloud.langfuse.com/api/public/otel/v1/traces",
	}
	for input, expected := range tests {
		actual, err := traceEndpoint(input)
		if err != nil {
			t.Fatalf("traceEndpoint(%q): %v", input, err)
		}
		if actual != expected {
			t.Fatalf("traceEndpoint(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestHandlerUsesLangfuseOTLPHTTPContract(t *testing.T) {
	type requestSnapshot struct {
		Path             string
		Authorization    string
		IngestionVersion string
		ContentType      string
		ContentEncoding  string
		BodyBytes        int
	}
	requests := make(chan requestSnapshot, 1)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		requests <- requestSnapshot{
			Path:             request.URL.Path,
			Authorization:    request.Header.Get("Authorization"),
			IngestionVersion: request.Header.Get("x-langfuse-ingestion-version"),
			ContentType:      request.Header.Get("Content-Type"),
			ContentEncoding:  request.Header.Get("Content-Encoding"),
			BodyBytes:        len(body),
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/x-protobuf"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})}

	handler, err := NewHandler(context.Background(), &Config{
		Host:       "https://langfuse.example",
		PublicKey:  "pk-test",
		SecretKey:  "sk-test",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := handler.StartTrace(context.Background(), WithName("otlp-contract"))
	handler.EndTrace(ctx, "done")
	flushCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := handler.Flush(flushCtx); err != nil {
		t.Fatal(err)
	}

	select {
	case request := <-requests:
		if request.Path != "/api/public/otel/v1/traces" {
			t.Fatalf("request path = %q", request.Path)
		}
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-test:sk-test"))
		if request.Authorization != expectedAuth {
			t.Fatalf("authorization = %q, want %q", request.Authorization, expectedAuth)
		}
		if request.IngestionVersion != "4" {
			t.Fatalf("ingestion version = %q", request.IngestionVersion)
		}
		if request.ContentType != "application/x-protobuf" {
			t.Fatalf("content type = %q", request.ContentType)
		}
		if request.ContentEncoding != "gzip" {
			t.Fatalf("content encoding = %q", request.ContentEncoding)
		}
		if request.BodyBytes == 0 {
			t.Fatal("OTLP request body is empty")
		}
	case <-flushCtx.Done():
		t.Fatal("OTLP request was not received")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestHandlerExportsLangfuseV4RootAndGeneration(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{
		SpanExporter:            exporter,
		MaxAttributeValueLength: -1,
		Environment:             "test",
		Release:                 "release-1",
		Version:                 "version-1",
		Tags:                    []string{"service:eino"},
	})
	if err != nil {
		t.Fatal(err)
	}

	const traceID = "0123456789abcdef0123456789abcdef"
	rootCtx := handler.StartTrace(context.Background(),
		WithID(traceID),
		WithName("project-agent"),
		WithObservationType(ObservationTypeAgent),
		WithInput("write a chapter"),
		WithUserID("user-1"),
		WithSessionID("project-1_user-1"),
		WithMetadata(map[string]string{"project_id": "project-1"}),
		WithMetadataValues(map[string]any{"attempt": 2}),
		WithTags("feature:project-agent"),
	)
	info := &callbacks.RunInfo{Name: "QwenChatModel", Component: components.ComponentOfChatModel}
	generationCtx := handler.OnStart(rootCtx, info, &model.CallbackInput{
		Messages: []*schema.Message{schema.UserMessage("write a chapter")},
		Config: &model.Config{
			Model:       "qwen-plus",
			MaxTokens:   1024,
			Temperature: 0.7,
		},
	})
	handler.OnEnd(generationCtx, info, &model.CallbackOutput{
		Message: &schema.Message{
			Role:    schema.Assistant,
			Content: "chapter",
			ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: 2,
				},
				CompletionTokensDetails: schema.CompletionTokensDetails{
					ReasoningTokens: 5,
				},
			}},
		},
	})
	handler.EndTrace(rootCtx, "chapter")
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("exported %d spans, want 2", len(spans))
	}
	root := spanByName(t, spans, "project-agent")
	generation := spanByName(t, spans, "QwenChatModel")
	if root.SpanContext.TraceID().String() != traceID {
		t.Fatalf("root trace ID = %s, want %s", root.SpanContext.TraceID(), traceID)
	}
	if root.Parent.IsValid() {
		t.Fatalf("root observation unexpectedly has parent %s", root.Parent.SpanID())
	}
	if generation.SpanContext.TraceID() != root.SpanContext.TraceID() {
		t.Fatal("generation and root have different trace IDs")
	}
	if generation.Parent.SpanID() != root.SpanContext.SpanID() {
		t.Fatalf("generation parent = %s, want %s", generation.Parent.SpanID(), root.SpanContext.SpanID())
	}

	rootAttrs := attributesByKey(root.Attributes)
	assertStringAttribute(t, rootAttrs, "langfuse.observation.type", string(ObservationTypeAgent))
	assertStringAttribute(t, rootAttrs, "langfuse.observation.input", "write a chapter")
	assertStringAttribute(t, rootAttrs, "langfuse.observation.output", "chapter")
	assertStringAttribute(t, rootAttrs, "langfuse.trace.name", "project-agent")

	generationAttrs := attributesByKey(generation.Attributes)
	assertStringAttribute(t, generationAttrs, "langfuse.observation.type", string(ObservationTypeGeneration))
	assertStringAttribute(t, generationAttrs, "langfuse.trace.name", "project-agent")
	assertStringAttribute(t, generationAttrs, "langfuse.user.id", "user-1")
	assertStringAttribute(t, generationAttrs, "langfuse.session.id", "project-1_user-1")
	assertStringAttribute(t, generationAttrs, "langfuse.trace.metadata.project_id", "project-1")
	assertStringAttribute(t, generationAttrs, "langfuse.trace.metadata.attempt", "2")
	assertStringAttribute(t, generationAttrs, "langfuse.environment", "test")
	assertStringAttribute(t, generationAttrs, "langfuse.release", "release-1")
	assertStringAttribute(t, generationAttrs, "langfuse.version", "version-1")
	assertStringAttribute(t, generationAttrs, "langfuse.observation.model.name", "qwen-plus")
	if tags := generationAttrs["langfuse.trace.tags"].Value.AsStringSlice(); len(tags) != 2 || tags[0] != "service:eino" || tags[1] != "feature:project-agent" {
		t.Fatalf("unexpected trace tags: %#v", tags)
	}

	assertInt64Attribute(t, generationAttrs, "gen_ai.usage.input_tokens", 10)
	assertInt64Attribute(t, generationAttrs, "gen_ai.usage.output_tokens", 20)
	assertInt64Attribute(t, generationAttrs, "gen_ai.usage.cache_read.input_tokens", 2)
	assertInt64Attribute(t, generationAttrs, "gen_ai.usage.reasoning.output_tokens", 5)
}

func TestHandlerCollectsStreamingGenerationBeforeExport(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{
		SpanExporter:            exporter,
		MaxAttributeValueLength: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	rootCtx := handler.StartTrace(context.Background(), WithName("stream-trace"))
	info := &callbacks.RunInfo{Name: "stream-model", Component: components.ComponentOfChatModel}
	input := schema.StreamReaderFromArray([]callbacks.CallbackInput{
		&model.CallbackInput{Messages: []*schema.Message{{Role: schema.User, Content: "hello "}}},
		&model.CallbackInput{Messages: []*schema.Message{{Role: schema.User, Content: "world"}}, Config: &model.Config{Model: "qwen-plus"}},
	})
	generationCtx := handler.OnStartWithStreamInput(rootCtx, info, input)
	output := schema.StreamReaderFromArray([]callbacks.CallbackOutput{
		&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "good "}},
		&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "day", ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 2, CompletionTokens: 2, TotalTokens: 4}}}},
	})
	handler.OnEndWithStreamOutput(generationCtx, info, output)
	handler.EndTrace(rootCtx, "good day")

	flushCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := handler.Flush(flushCtx); err != nil {
		t.Fatal(err)
	}
	generation := spanByName(t, exporter.GetSpans(), "stream-model")
	attrs := attributesByKey(generation.Attributes)
	assertStringAttribute(t, attrs, "langfuse.observation.model.name", "qwen-plus")
	if attrs["langfuse.observation.completion_start_time"].Value.AsString() == "" {
		t.Fatal("streaming generation has no completion start time")
	}
	if got := attrs["langfuse.observation.output"].Value.AsString(); got == "" || !json.Valid([]byte(got)) {
		t.Fatalf("streaming output is not valid JSON: %q", got)
	}
}

func TestHandlerCollectsADKAgentIterator(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{
		SpanExporter:            exporter,
		MaxAttributeValueLength: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	rootCtx := handler.StartTrace(context.Background(), WithName("agent-trace"))
	info := &callbacks.RunInfo{Name: "writer-agent", Component: adk.ComponentOfAgent}
	agentCtx := handler.OnStart(rootCtx, info, &adk.AgentCallbackInput{
		Input: &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage("write")}},
	})
	events, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	handler.OnEnd(agentCtx, info, &adk.AgentCallbackOutput{Events: events})
	generator.Send(&adk.AgentEvent{Output: &adk.AgentOutput{
		MessageOutput: &adk.MessageVariant{Message: schema.AssistantMessage("done", nil)},
	}})
	generator.Close()
	handler.EndTrace(rootCtx, "done")

	flushCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := handler.Flush(flushCtx); err != nil {
		t.Fatal(err)
	}
	agent := spanByName(t, exporter.GetSpans(), "writer-agent")
	attrs := attributesByKey(agent.Attributes)
	assertStringAttribute(t, attrs, "langfuse.observation.type", string(ObservationTypeAgent))
	if got := attrs["langfuse.observation.output"].Value.AsString(); got == "" || !json.Valid([]byte(got)) {
		t.Fatalf("agent output is not valid JSON: %q", got)
	}
}

func TestCollapseAgentInternalSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{
		SpanExporter:               exporter,
		CollapseAgentInternalSpans: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	agentInfo := &callbacks.RunInfo{Name: "writer-agent", Component: adk.ComponentOfAgent}
	agentCtx := handler.OnStart(context.Background(), agentInfo, &adk.AgentCallbackInput{})
	t.Cleanup(func() {
		handler.OnEnd(agentCtx, agentInfo, nil)
		if err := handler.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})

	if handler.Needed(agentCtx, &callbacks.RunInfo{Name: "writer-agent", Component: compose.ComponentOfChain}, callbacks.TimingOnStart) {
		t.Fatal("expected the active agent's same-name internal Chain to be collapsed")
	}
	if !handler.Needed(agentCtx, &callbacks.RunInfo{Name: "business-chain", Component: compose.ComponentOfChain}, callbacks.TimingOnStart) {
		t.Fatal("expected a differently named Chain to be retained")
	}
	if !handler.Needed(agentCtx, &callbacks.RunInfo{Name: "writer-agent", Component: compose.ComponentOfGraph}, callbacks.TimingOnStart) {
		t.Fatal("expected a same-name non-Chain component to be retained")
	}
	if handler.Needed(agentCtx, &callbacks.RunInfo{Name: internalReActGraphName, Component: compose.ComponentOfGraph}, callbacks.TimingOnStart) {
		t.Fatal("expected the Agent's internal ReAct Graph to be collapsed")
	}
	if handler.Needed(agentCtx, &callbacks.RunInfo{Component: compose.ComponentOfLambda}, callbacks.TimingOnStart) {
		t.Fatal("expected an unnamed Lambda wrapper to be collapsed")
	}
	if handler.Needed(agentCtx, &callbacks.RunInfo{Name: string(compose.ComponentOfLambda), Component: compose.ComponentOfLambda}, callbacks.TimingOnStart) {
		t.Fatal("expected a default-named Lambda wrapper to be collapsed")
	}
	if handler.Needed(agentCtx, &callbacks.RunInfo{Name: internalInitLambdaName, Component: compose.ComponentOfLambda}, callbacks.TimingOnStart) {
		t.Fatal("expected the ReAct Init Lambda to be collapsed")
	}
	if !handler.Needed(agentCtx, &callbacks.RunInfo{Name: "prepare-prompt", Component: compose.ComponentOfLambda}, callbacks.TimingOnStart) {
		t.Fatal("expected a named business Lambda to be retained")
	}
	if !handler.Needed(context.Background(), &callbacks.RunInfo{Name: internalReActGraphName, Component: compose.ComponentOfGraph}, callbacks.TimingOnStart) {
		t.Fatal("expected a ReAct-named Graph outside an Agent to be retained")
	}
	if !handler.Needed(context.Background(), &callbacks.RunInfo{Name: internalInitLambdaName, Component: compose.ComponentOfLambda}, callbacks.TimingOnStart) {
		t.Fatal("expected an Init-named Lambda outside an Agent to be retained")
	}
	if !handler.Needed(context.Background(), &callbacks.RunInfo{Component: compose.ComponentOfLambda}, callbacks.TimingOnStart) {
		t.Fatal("expected an anonymous Lambda outside an Agent to be retained")
	}
}

func TestCollapseAgentInternalSpansDisabledByDefault(t *testing.T) {
	handler := &CallbackHandler{}
	ctx := context.WithValue(context.Background(), activeAgentNameKey{}, "writer-agent")
	if !handler.Needed(ctx, &callbacks.RunInfo{Name: "writer-agent", Component: compose.ComponentOfChain}, callbacks.TimingOnStart) {
		t.Fatal("expected the internal Chain to be retained by default")
	}
	if !handler.Needed(ctx, &callbacks.RunInfo{Component: compose.ComponentOfLambda}, callbacks.TimingOnStart) {
		t.Fatal("expected the anonymous Lambda to be retained by default")
	}
}

func TestTraceEndsWhenContextIsCancelledWithoutChild(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{SpanExporter: exporter})
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("stopped by user")
	ctx, cancel := context.WithCancelCause(context.Background())
	rootCtx := handler.StartTrace(ctx, WithName("cancelled-without-child"))
	run, _ := rootCtx.Value(traceRunKey{}).(*traceRun)
	cancel(cause)
	select {
	case <-run.done:
	case <-time.After(time.Second):
		t.Fatal("trace did not end after its context was cancelled")
	}
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	span := spanByName(t, exporter.GetSpans(), "cancelled-without-child")
	attrs := attributesByKey(span.Attributes)
	assertStringAttribute(t, attrs, "langfuse.observation.metadata.eino_callback_cancelled", "true")
	if output := attrs["langfuse.observation.output"].Value.AsString(); !strings.Contains(output, cause.Error()) {
		t.Fatalf("root output = %q, want cancellation cause", output)
	}
}

func TestTraceDeadlineIsReportedAsError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{SpanExporter: exporter})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	rootCtx := handler.StartTrace(ctx, WithName("deadline-without-child"))
	run, _ := rootCtx.Value(traceRunKey{}).(*traceRun)
	select {
	case <-run.done:
	case <-time.After(time.Second):
		t.Fatal("trace did not end after its context deadline")
	}
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	span := spanByName(t, exporter.GetSpans(), "deadline-without-child")
	if span.Status.Code != codes.Error {
		t.Fatalf("root status = %v, want error", span.Status.Code)
	}
	attrs := attributesByKey(span.Attributes)
	assertStringAttribute(t, attrs, "langfuse.observation.level", "ERROR")
	if output := attrs["langfuse.observation.output"].Value.AsString(); !strings.Contains(output, context.DeadlineExceeded.Error()) {
		t.Fatalf("root output = %q, want deadline error", output)
	}
}

func TestCancelledTraceWaitsForActiveChild(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{SpanExporter: exporter})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rootCtx := handler.StartTrace(ctx, WithName("active-child"))
	run, _ := rootCtx.Value(traceRunKey{}).(*traceRun)

	info := &callbacks.RunInfo{Name: "first-child", Component: components.ComponentOfTool}
	childCtx := handler.OnStart(rootCtx, info, "input")
	cancel()
	time.Sleep(20 * time.Millisecond)
	if spans := exporter.GetSpans(); len(spans) != 0 {
		t.Fatalf("cancelled trace ended while its child was active: %#v", spans)
	}

	handler.OnEnd(childCtx, info, "output")
	select {
	case <-run.done:
	case <-time.After(time.Second):
		t.Fatal("trace did not end after its active child finished")
	}
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if spans := exporter.GetSpans(); len(spans) != 2 {
		t.Fatalf("exported %d spans after child finished, want 2", len(spans))
	}
	root := spanByName(t, exporter.GetSpans(), "active-child")
	attrs := attributesByKey(root.Attributes)
	if _, cancelled := attrs["langfuse.observation.metadata.eino_callback_cancelled"]; cancelled {
		t.Fatal("plain context cancellation was reported as a user stop")
	}
	if output := attrs["langfuse.observation.output"].Value.AsString(); !strings.Contains(output, "output") {
		t.Fatalf("root output = %q, want last child output", output)
	}
}

func TestEndTraceWaitsForActiveChild(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	handler, err := NewHandler(context.Background(), &Config{TracerProvider: provider})
	if err != nil {
		t.Fatal(err)
	}
	rootCtx := handler.StartTrace(context.Background(), WithName("explicit-end-active-child"))
	run, _ := rootCtx.Value(traceRunKey{}).(*traceRun)
	info := &callbacks.RunInfo{Name: "active-tool", Component: components.ComponentOfTool}
	childCtx := handler.OnStart(rootCtx, info, "input")

	handler.EndTrace(rootCtx, "explicit root output")
	if spans := exporter.GetSpans(); len(spans) != 0 {
		t.Fatalf("EndTrace exported while its child was active: %#v", spans)
	}
	select {
	case <-run.done:
		t.Fatal("EndTrace completed while its child was active")
	default:
	}

	handler.OnEnd(childCtx, info, "child output")
	select {
	case <-run.done:
	case <-time.After(time.Second):
		t.Fatal("trace did not end after its active child finished")
	}
	if spans := exporter.GetSpans(); len(spans) != 2 {
		t.Fatalf("exported %d spans after child finished, want 2", len(spans))
	}
	root := spanByName(t, exporter.GetSpans(), "explicit-end-active-child")
	attrs := attributesByKey(root.Attributes)
	assertStringAttribute(t, attrs, "langfuse.observation.output", "explicit root output")
}

func TestReceiveAllRecordsFirstChunkTime(t *testing.T) {
	reader, writer := schema.Pipe[string](0)
	started := time.Now()
	go func() {
		time.Sleep(30 * time.Millisecond)
		writer.Send("first", nil)
		writer.Close()
	}()
	values, first, err := receiveAllWithFirst(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != "first" {
		t.Fatalf("unexpected stream values: %#v", values)
	}
	if first.Sub(started) < 20*time.Millisecond {
		t.Fatalf("first chunk time %s was recorded before the chunk arrived", first.Sub(started))
	}
}

func TestValueLimitsPreserveValidJSON(t *testing.T) {
	input := `[{"role":"user","content":"` + strings.Repeat("x", 500) + `"}]`
	prepare := newValuePreparer(func(value string) string {
		return value[:150]
	}, 120, nil)
	got := prepare(input)
	if len(got) > 120 {
		t.Fatalf("prepared value has %d bytes, want at most 120", len(got))
	}
	if !json.Valid([]byte(got)) {
		t.Fatalf("prepared JSON is invalid: %q", got)
	}
}

func TestSpanAttributeBudgetIsEnforced(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	handler, err := NewHandler(context.Background(), &Config{
		SpanExporter:            exporter,
		MaxAttributeValueLength: -1,
		MaxSpanAttributeBytes:   500,
	})
	if err != nil {
		t.Fatal(err)
	}
	rootCtx := handler.StartTrace(context.Background(),
		WithName("budgeted-trace"),
		WithInput(strings.Repeat("root", 400)),
		WithMetadataValues(map[string]any{"large": strings.Repeat("metadata", 200)}),
	)
	info := &callbacks.RunInfo{Name: "budgeted-tool", Component: components.ComponentOfTool}
	childCtx := handler.OnStart(rootCtx, info, strings.Repeat("input", 400))
	handler.OnEnd(childCtx, info, strings.Repeat("output", 400))
	handler.EndTrace(rootCtx, strings.Repeat("result", 400))
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, span := range exporter.GetSpans() {
		total := 0
		for _, attr := range span.Attributes {
			total += len(attr.Key) + attributeValueSize(attr.Value)
			if value := attr.Value; value.Type() == attribute.STRING && strings.Contains(string(attr.Key), "observation.") && strings.Contains(string(attr.Key), "input") {
				if got := value.AsString(); got != "" && !json.Valid([]byte(got)) {
					t.Fatalf("span %q contains invalid limited JSON: %q", span.Name, got)
				}
			}
		}
		if total > 500 {
			t.Fatalf("span %q has %d callback attribute bytes, want at most 500", span.Name, total)
		}
	}
}

func TestAgentEventErrorMarksAgentAndRoot(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	handler, err := NewHandler(context.Background(), &Config{TracerProvider: provider})
	if err != nil {
		t.Fatal(err)
	}
	rootCtx := handler.StartTrace(context.Background(), WithName("failed-agent-trace"))
	info := &callbacks.RunInfo{Name: "failed-agent", Component: adk.ComponentOfAgent}
	agentCtx := handler.OnStart(rootCtx, info, &adk.AgentCallbackInput{})
	events, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	handler.OnEnd(agentCtx, info, &adk.AgentCallbackOutput{Events: events})
	generator.Send(&adk.AgentEvent{Err: errors.New("agent failed")})
	generator.Close()
	if err := handler.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"failed-agent", "failed-agent-trace"} {
		span := spanByName(t, exporter.GetSpans(), name)
		if span.Status.Code != codes.Error {
			t.Fatalf("span %q status = %v, want error", name, span.Status.Code)
		}
		attrs := attributesByKey(span.Attributes)
		assertStringAttribute(t, attrs, "langfuse.observation.level", "ERROR")
	}
}

func TestContextCancellationDoesNotMarkTraceAsError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	handler, err := NewHandler(context.Background(), &Config{TracerProvider: provider})
	if err != nil {
		t.Fatal(err)
	}

	cause := errors.New("generation stopped by user")
	ctx, cancel := context.WithCancelCause(context.Background())
	rootCtx := handler.StartTrace(ctx, WithName("stopped-trace"))
	info := &callbacks.RunInfo{Name: "stopped-generation", Component: components.ComponentOfChatModel}
	generationCtx := handler.OnStart(rootCtx, info, &model.CallbackInput{})
	cancel(cause)
	handler.OnError(generationCtx, info, context.Canceled)
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler.EndTrace(rootCtx, "")

	for _, name := range []string{"stopped-generation", "stopped-trace"} {
		span := spanByName(t, exporter.GetSpans(), name)
		if span.Status.Code == codes.Error {
			t.Fatalf("span %q was marked as an error", name)
		}
		attrs := attributesByKey(span.Attributes)
		assertStringAttribute(t, attrs, "langfuse.observation.metadata.eino_callback_cancelled", "true")
		assertStringAttribute(t, attrs, "langfuse.observation.metadata.eino_cancellation_cause", cause.Error())
		output := attrs["langfuse.observation.output"].Value.AsString()
		if !strings.Contains(output, `"status":"cancelled"`) || !strings.Contains(output, cause.Error()) {
			t.Fatalf("span %q output = %q, want structured cancellation", name, output)
		}
		if status := attrs["langfuse.observation.status_message"].Value.AsString(); !strings.Contains(status, "cancelled") {
			t.Fatalf("span %q status message = %q", name, status)
		}
		if level, ok := attrs["langfuse.observation.level"]; ok && level.Value.AsString() == "ERROR" {
			t.Fatalf("span %q has ERROR level", name)
		}
	}
}

func TestEinoInterruptDoesNotMarkTraceAsError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	handler, err := NewHandler(context.Background(), &Config{TracerProvider: provider})
	if err != nil {
		t.Fatal(err)
	}

	rootCtx := handler.StartTrace(context.Background(), WithName("interrupted-trace"))
	info := &callbacks.RunInfo{Name: "interrupted-tool", Component: components.ComponentOfTool}
	toolCtx := handler.OnStart(rootCtx, info, map[string]any{"chapter": 4})
	interruptErr := compose.Interrupt(toolCtx, map[string]any{"reason": "awaiting confirmation"})
	handler.OnError(toolCtx, info, fmt.Errorf("invoke tool: %w", interruptErr))
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler.EndTrace(rootCtx, "")

	for _, name := range []string{"interrupted-tool", "interrupted-trace"} {
		span := spanByName(t, exporter.GetSpans(), name)
		if span.Status.Code == codes.Error {
			t.Fatalf("span %q was marked as an error", name)
		}
		attrs := attributesByKey(span.Attributes)
		assertStringAttribute(t, attrs, "langfuse.observation.metadata.eino_callback_interrupted", "true")
		if status := attrs["langfuse.observation.status_message"].Value.AsString(); !strings.Contains(status, "interrupted") {
			t.Fatalf("span %q status message = %q", name, status)
		}
		output := attrs["langfuse.observation.output"].Value.AsString()
		if !strings.Contains(output, `"status":"interrupted"`) || !strings.Contains(output, "awaiting confirmation") {
			t.Fatalf("span %q output = %q, want structured interrupt", name, output)
		}
		if level, ok := attrs["langfuse.observation.level"]; ok && level.Value.AsString() == "ERROR" {
			t.Fatalf("span %q has ERROR level", name)
		}
	}
}

func TestTracerIncludesCallbackInstrumentationVersion(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	handler, err := NewHandler(context.Background(), &Config{TracerProvider: provider})
	if err != nil {
		t.Fatal(err)
	}

	ctx := handler.StartTrace(context.Background(), WithName("versioned-trace"))
	handler.EndTrace(ctx, "done")
	span := spanByName(t, exporter.GetSpans(), "versioned-trace")
	if span.InstrumentationScope.Name != instrumentationName {
		t.Fatalf("instrumentation name = %q, want %q", span.InstrumentationScope.Name, instrumentationName)
	}
	if span.InstrumentationScope.Version != instrumentationVersion {
		t.Fatalf("instrumentation version = %q, want %q", span.InstrumentationScope.Version, instrumentationVersion)
	}
}

func TestProcessResourceAttributesAreOptIn(t *testing.T) {
	for _, test := range []struct {
		name           string
		includeProcess bool
		wantProcess    bool
	}{
		{name: "excluded by default"},
		{name: "included explicitly", includeProcess: true, wantProcess: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			handler, err := NewHandler(context.Background(), &Config{
				SpanExporter:                     exporter,
				IncludeProcessResourceAttributes: test.includeProcess,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer handler.Shutdown(context.Background())

			ctx := handler.StartTrace(context.Background(), WithName("resource-trace"))
			handler.EndTrace(ctx, "done")
			if err := handler.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			span := spanByName(t, exporter.GetSpans(), "resource-trace")
			hasProcess := false
			for _, attr := range span.Resource.Attributes() {
				if strings.HasPrefix(string(attr.Key), "process.") {
					hasProcess = true
					break
				}
			}
			if hasProcess != test.wantProcess {
				t.Fatalf("has process resource attributes = %v, want %v", hasProcess, test.wantProcess)
			}
		})
	}
}

func TestDeadlineExceededStillMarksTraceAsError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	handler, err := NewHandler(context.Background(), &Config{TracerProvider: provider})
	if err != nil {
		t.Fatal(err)
	}

	rootCtx := handler.StartTrace(context.Background(), WithName("timed-out-trace"))
	info := &callbacks.RunInfo{Name: "timed-out-generation", Component: components.ComponentOfChatModel}
	generationCtx := handler.OnStart(rootCtx, info, &model.CallbackInput{})
	handler.OnError(generationCtx, info, context.DeadlineExceeded)
	if err := handler.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler.EndTrace(rootCtx, "")

	for _, name := range []string{"timed-out-generation", "timed-out-trace"} {
		span := spanByName(t, exporter.GetSpans(), name)
		if span.Status.Code != codes.Error {
			t.Fatalf("span %q status = %v, want error", name, span.Status.Code)
		}
		attrs := attributesByKey(span.Attributes)
		assertStringAttribute(t, attrs, "langfuse.observation.level", "ERROR")
		if output := attrs["langfuse.observation.output"].Value.AsString(); !strings.Contains(output, context.DeadlineExceeded.Error()) {
			t.Fatalf("span %q output = %q, want structured error", name, output)
		}
	}
}

func TestCancellationCauseRecognizesCustomCauseOnly(t *testing.T) {
	cause := errors.New("generation stopped by user")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)

	got, cancelled := cancellationCause(ctx, fmt.Errorf("agent stopped: %w", cause))
	if !cancelled || !errors.Is(got, cause) {
		t.Fatalf("custom cancellation cause = (%v, %v)", got, cancelled)
	}
	if got, cancelled := cancellationCause(ctx, errors.New("model failed")); cancelled || got != nil {
		t.Fatalf("unrelated error was treated as cancellation: (%v, %v)", got, cancelled)
	}
}

func TestShutdownWaitsForAgentOutputBeforeEndingRoot(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer provider.Shutdown(context.Background())
	handler, err := NewHandler(context.Background(), &Config{TracerProvider: provider})
	if err != nil {
		t.Fatal(err)
	}
	rootCtx := handler.StartTrace(context.Background(), WithName("shutdown-trace"))
	info := &callbacks.RunInfo{Name: "shutdown-agent", Component: adk.ComponentOfAgent}
	agentCtx := handler.OnStart(rootCtx, info, &adk.AgentCallbackInput{})
	events, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	handler.OnEnd(agentCtx, info, &adk.AgentCallbackOutput{Events: events})

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- handler.Shutdown(context.Background()) }()
	time.Sleep(20 * time.Millisecond)
	generator.Send(&adk.AgentEvent{Output: &adk.AgentOutput{
		MessageOutput: &adk.MessageVariant{Message: schema.AssistantMessage("final", nil)},
	}})
	generator.Close()
	if err := <-shutdownDone; err != nil {
		t.Fatal(err)
	}
	root := spanByName(t, exporter.GetSpans(), "shutdown-trace")
	output := attributesByKey(root.Attributes)["langfuse.observation.output"].Value.AsString()
	if !strings.Contains(output, "final") {
		t.Fatalf("root output %q does not contain final agent output", output)
	}
}

func spanByName(t *testing.T, spans tracetest.SpanStubs, name string) tracetest.SpanStub {
	t.Helper()
	for _, span := range spans {
		if span.Name == name {
			return span
		}
	}
	t.Fatalf("span %q not found in %#v", name, spans)
	return tracetest.SpanStub{}
}

func attributesByKey(attrs []attribute.KeyValue) map[string]attribute.KeyValue {
	result := make(map[string]attribute.KeyValue, len(attrs))
	for _, attr := range attrs {
		result[string(attr.Key)] = attr
	}
	return result
}

func assertStringAttribute(t *testing.T, attrs map[string]attribute.KeyValue, key, expected string) {
	t.Helper()
	attr, ok := attrs[key]
	if !ok {
		t.Fatalf("attribute %q is missing", key)
	}
	if actual := attr.Value.AsString(); actual != expected {
		t.Fatalf("attribute %q = %q, want %q", key, actual, expected)
	}
}

func assertInt64Attribute(t *testing.T, attrs map[string]attribute.KeyValue, key string, expected int64) {
	t.Helper()
	attr, ok := attrs[key]
	if !ok {
		t.Fatalf("attribute %q is missing", key)
	}
	if actual := attr.Value.AsInt64(); actual != expected {
		t.Fatalf("attribute %q = %d, want %d", key, actual, expected)
	}
}
