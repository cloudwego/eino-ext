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

package tls

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sem_ai "github.com/cloudwego/eino-ext/callbacks/tls/semconv"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volc-sdk-golang/service/tls/pb"
	"github.com/volcengine/volc-sdk-golang/service/tls/producer"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type fakeTLSLogProducer struct {
	result *producer.Result
}

func (p *fakeTLSLogProducer) SendLog(_ string, _ string, _ string, _ string, _ *pb.Log, callback producer.CallBack) error {
	if p.result != nil && !p.result.SuccessFlag {
		callback.Fail(p.result)
		return nil
	}
	callback.Success(&producer.Result{SuccessFlag: true})
	return nil
}

func (p *fakeTLSLogProducer) Close() {}

func TestTLSExporterSpanNames(t *testing.T) {
	if got := tlsSpanName(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfChatModel}); got != tlsModelSpanName {
		t.Fatalf("model span name = %q, want %q", got, tlsModelSpanName)
	}
	if got := tlsSpanName(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfTool}); got != tlsToolSpanName {
		t.Fatalf("tool span name = %q, want %q", got, tlsToolSpanName)
	}
	if got := tlsSpanName(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfRetriever}); got != tlsRootSpanName {
		t.Fatalf("root span name = %q, want %q", got, tlsRootSpanName)
	}
	ctx := context.WithValue(context.Background(), TLSStateKey{}, &TLSState{})
	if got := tlsSpanName(ctx, &callbacks.RunInfo{Name: "retriever", Component: components.ComponentOfRetriever}); got != "retriever" {
		t.Fatalf("nested span name = %q, want retriever", got)
	}
}

func TestTLSExporterSpanKinds(t *testing.T) {
	handler := &TLSCallbackHandler{TLSExporterEnabled: true}
	if got := handler.spanKind(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfRetriever}); got != trace.SpanKindServer {
		t.Fatalf("root span kind = %s, want server", got)
	}
	if got := handler.spanKind(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfChatModel}); got != trace.SpanKindClient {
		t.Fatalf("model span kind = %s, want client", got)
	}
}

func TestTLSModelTokenAliases(t *testing.T) {
	tags := make(spanTags)
	setModelConfigAndTokenUsage(tags, &model.CallbackOutput{TokenUsage: &model.TokenUsage{
		PromptTokens:     11,
		CompletionTokens: 7,
		TotalTokens:      18,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: 3,
		},
		CompletionTokensDetails: model.CompletionTokensDetails{
			ReasoningTokens: 4,
		},
	}}, false)

	for key, want := range map[string]int{
		sem_ai.GEN_AI_USAGE_INPUT_TOKENS:                11,
		sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS:               7,
		sem_ai.GEN_AI_USAGE_TOTAL_TOKENS:                18,
		sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS_V2:  3,
		sem_ai.GEN_AI_USAGE_CACHED_TOKENS:               3,
		sem_ai.GEN_AI_USAGE_CACHE_CREATE_INPUT_TOKENS:   0,
		sem_ai.GEN_AI_USAGE_CACHE_CREATION_INPUT_TOKENS: 0,
		sem_ai.GEN_AI_USAGE_REASONING_OUTPUT_TOKENS:     4,
	} {
		if got, ok := tags[key].(int); !ok || got != want {
			t.Errorf("%s = %#v, want %d", key, tags[key], want)
		}
	}
}

func TestSpanToTLSLog(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	resource := sdkresource.NewWithAttributes("", attribute.String("service.name", "Eino"))
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource),
		sdktrace.WithSpanProcessor(recorder),
	)
	defer func() { _ = provider.Shutdown(context.Background()) }()

	_, span := provider.Tracer("test").Start(context.Background(), tlsModelSpanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithTimestamp(time.Unix(1_700_000_000, 0)),
	)
	span.SetAttributes(
		attribute.String(sem_ai.TLS_APP_TYPE, tlsAppType),
		attribute.String(sem_ai.SESSION_ID, "session-1"),
		attribute.Int(sem_ai.GEN_AI_USAGE_INPUT_TOKENS, 11),
	)
	span.End(trace.WithTimestamp(time.Unix(1_700_000_001, 0)))

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	log, err := spanToTLSLog(spans[0])
	if err != nil {
		t.Fatalf("spanToTLSLog: %v", err)
	}
	contents := make(map[string]string, len(log.Contents))
	for _, item := range log.Contents {
		contents[item.Key] = item.Value
	}
	if contents["Name"] != tlsModelSpanName || contents["Kind"] != "client" || contents["StatusCode"] != "OK" {
		t.Fatalf("unexpected envelope identity: %#v", contents)
	}
	if contents["ParentSpanID"] != "" {
		t.Fatalf("root ParentSpanID = %q, want empty", contents["ParentSpanID"])
	}
	var attrs map[string]any
	if err := json.Unmarshal([]byte(contents["Attributes"]), &attrs); err != nil {
		t.Fatalf("decode Attributes: %v", err)
	}
	if attrs[sem_ai.TLS_APP_TYPE] != tlsAppType || attrs[sem_ai.SESSION_ID] != "session-1" || attrs[sem_ai.GEN_AI_USAGE_INPUT_TOKENS] != float64(11) {
		t.Fatalf("unexpected TLS exporter attributes: %#v", attrs)
	}
	if contents["ServiceName"] != "Eino" {
		t.Fatalf("ServiceName = %q, want Eino", contents["ServiceName"])
	}
}

func TestTLSLogExporterWaitsForProducerCallback(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	_, span := provider.Tracer("test").Start(context.Background(), tlsRootSpanName)
	span.End()
	spans := recorder.Ended()

	exporter := &tlsLogExporter{producer: &fakeTLSLogProducer{}, topicID: "topic", source: "source"}
	if err := exporter.ExportSpans(context.Background(), spans); err != nil {
		t.Fatalf("ExportSpans() error = %v", err)
	}
}

func TestTLSLogExporterReturnsProducerCallbackFailure(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	_, span := provider.Tracer("test").Start(context.Background(), tlsRootSpanName)
	span.End()
	spans := recorder.Ended()

	exporter := &tlsLogExporter{producer: &fakeTLSLogProducer{result: &producer.Result{
		Attempts: []*producer.Attempt{{ErrorCode: "Forbidden", ErrorMessage: "denied"}},
	}}, topicID: "topic", source: "source"}
	if err := exporter.ExportSpans(context.Background(), spans); err == nil {
		t.Fatal("ExportSpans() succeeded after a Producer callback failure")
	}
}

func TestTLSRootStateAggregatesModelUsage(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	_, rootSpan := provider.Tracer("test").Start(context.Background(), tlsRootSpanName)
	root := &tlsRootState{span: rootSpan}
	root.addModelUsage(map[string]any{
		sem_ai.GEN_AI_REQUEST_MODEL:                    "model-a",
		sem_ai.GEN_AI_USAGE_INPUT_TOKENS:               17,
		sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS:              8,
		sem_ai.GEN_AI_USAGE_TOTAL_TOKENS:               25,
		sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS_V2: 3,
		sem_ai.GEN_AI_USAGE_REASONING_OUTPUT_TOKENS:    4,
	})
	rootSpan.End()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	attrs := otelAttributesToMap(spans[0].Attributes())
	for key, want := range map[string]any{
		sem_ai.GEN_AI_REQUEST_MODEL:                    "model-a",
		sem_ai.GEN_AI_USAGE_INPUT_TOKENS:               int64(17),
		sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS:              int64(8),
		sem_ai.GEN_AI_USAGE_TOTAL_TOKENS:               int64(25),
		sem_ai.GEN_AI_USAGE_CACHE_READ_INPUT_TOKENS_V2: int64(3),
		sem_ai.GEN_AI_USAGE_REASONING_OUTPUT_TOKENS:    int64(4),
	} {
		if got := attrs[key]; got != want {
			t.Errorf("%s = %#v, want %#v", key, got, want)
		}
	}
}

func TestTLSExporterCreatesTraceRootForDirectModel(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	handler := &TLSCallbackHandler{
		Tracer:             provider.Tracer(scopeName),
		dataParser:         NewDefaultDataParser(false),
		TLSExporterEnabled: true,
		AppName:            "Eino",
	}
	ctx := SetSession(context.Background(), WithSessionID("trace-session"))
	ctx = handler.OnStart(ctx, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackInput{
		Messages: []*schema.Message{{Role: schema.User, Content: "hello"}},
		Config:   &model.Config{Model: "demo-model"},
	})
	handler.OnEnd(ctx, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackOutput{
		Message: &schema.Message{Role: schema.Assistant, Content: "world"},
		TokenUsage: &model.TokenUsage{
			PromptTokens:     3,
			CompletionTokens: 2,
			TotalTokens:      5,
		},
	})

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want 2", len(spans))
	}
	byName := make(map[string]sdktrace.ReadOnlySpan, len(spans))
	for _, span := range spans {
		byName[span.Name()] = span
	}
	root := byName[tlsRootSpanName]
	modelSpan := byName[tlsModelSpanName]
	if root == nil || modelSpan == nil {
		t.Fatalf("expected %q and %q spans, got %#v", tlsRootSpanName, tlsModelSpanName, byName)
	}
	if root.Parent().IsValid() {
		t.Fatalf("root unexpectedly has parent %s", root.Parent().SpanID())
	}
	if modelSpan.Parent().SpanID() != root.SpanContext().SpanID() {
		t.Fatalf("model parent = %s, want root %s", modelSpan.Parent().SpanID(), root.SpanContext().SpanID())
	}
	attrs := otelAttributesToMap(root.Attributes())
	for key, want := range map[string]any{
		sem_ai.GEN_AI_SPAN_KIND:           sem_ai.GEN_AI_SPAN_KIND_CHAIN,
		sem_ai.GEN_AI_INPUT:               "hello",
		sem_ai.GEN_AI_OUTPUT:              "world",
		sem_ai.GEN_AI_REQUEST_MODEL:       "demo-model",
		sem_ai.GEN_AI_USAGE_INPUT_TOKENS:  int64(3),
		sem_ai.GEN_AI_USAGE_OUTPUT_TOKENS: int64(2),
		sem_ai.GEN_AI_USAGE_TOTAL_TOKENS:  int64(5),
		sem_ai.GEN_AI_SESSION_ID:          "trace-session",
		sem_ai.GEN_AI_RESPONSE_MODEL:      "demo-model",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("root %s = %#v, want %#v", key, got, want)
		}
	}
	modelAttrs := otelAttributesToMap(modelSpan.Attributes())
	if got := modelAttrs[sem_ai.GEN_AI_REQUEST_MODEL]; got != "demo-model" {
		t.Errorf("model %s = %#v, want demo-model", sem_ai.GEN_AI_REQUEST_MODEL, got)
	}
	if got := modelAttrs[sem_ai.GEN_AI_RESPONSE_MODEL]; got != "demo-model" {
		t.Errorf("model %s = %#v, want demo-model", sem_ai.GEN_AI_RESPONSE_MODEL, got)
	}
	if _, ok := modelAttrs[sem_ai.GEN_AI_REQUEST_DURATION_MS].(int64); !ok {
		t.Errorf("model is missing numeric %s: %#v", sem_ai.GEN_AI_REQUEST_DURATION_MS, modelAttrs[sem_ai.GEN_AI_REQUEST_DURATION_MS])
	}
	inputMessagesJSON, ok := attrs[sem_ai.GEN_AI_INPUT_MESSAGES].(string)
	if !ok {
		t.Errorf("root is missing %s", sem_ai.GEN_AI_INPUT_MESSAGES)
	} else {
		var messages []struct {
			Role  string `json:"role"`
			Parts []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"parts"`
		}
		if err := json.Unmarshal([]byte(inputMessagesJSON), &messages); err != nil {
			t.Fatalf("decode %s: %v", sem_ai.GEN_AI_INPUT_MESSAGES, err)
		}
		if len(messages) != 1 || messages[0].Role != "user" || len(messages[0].Parts) != 1 || messages[0].Parts[0].Type != "text" || messages[0].Parts[0].Content != "hello" {
			t.Fatalf("unexpected normalized input messages: %s", inputMessagesJSON)
		}
	}
}

func TestLatestUserMessageContent(t *testing.T) {
	tags := map[string]any{
		sem_ai.GEN_AI_PROMPT + ".0.role":    "system",
		sem_ai.GEN_AI_PROMPT + ".0.content": "system prompt",
		sem_ai.GEN_AI_PROMPT + ".1.role":    "user",
		sem_ai.GEN_AI_PROMPT + ".1.content": "first question",
		sem_ai.GEN_AI_PROMPT + ".2.role":    "assistant",
		sem_ai.GEN_AI_PROMPT + ".2.content": "first answer",
		sem_ai.GEN_AI_PROMPT + ".3.role":    "user",
		sem_ai.GEN_AI_PROMPT + ".3.content": "latest question",
	}
	if got := latestUserMessageContent(tags); got != "latest question" {
		t.Fatalf("latest user message = %q, want latest question", got)
	}
}

func TestTLSExporterParentsToolsUnderLatestModel(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	handler := &TLSCallbackHandler{
		Tracer:             provider.Tracer(scopeName),
		dataParser:         NewDefaultDataParser(false),
		TLSExporterEnabled: true,
	}
	rootInfo := &callbacks.RunInfo{Component: compose.ComponentOfLambda, Name: "eino-agent"}
	rootCtx := handler.OnStart(context.Background(), rootInfo, "input")
	modelInfo := &callbacks.RunInfo{Component: components.ComponentOfChatModel}
	modelCtx := handler.OnStart(rootCtx, modelInfo, &model.CallbackInput{
		Messages: []*schema.Message{{Role: schema.User, Content: "hello"}},
		Config:   &model.Config{Model: "demo-model"},
	})
	handler.OnEnd(modelCtx, modelInfo, &model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "tool call"}})

	toolInfo := &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "weather"}
	toolCtx := handler.OnStart(rootCtx, toolInfo, &tool.CallbackInput{})
	handler.OnEnd(toolCtx, toolInfo, &tool.CallbackOutput{})
	handler.OnEnd(rootCtx, rootInfo, "done")

	spans := recorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("ended spans = %d, want 3", len(spans))
	}
	byName := make(map[string]sdktrace.ReadOnlySpan, len(spans))
	for _, span := range spans {
		byName[span.Name()] = span
	}
	root := byName[tlsRootSpanName]
	modelSpan := byName[tlsModelSpanName]
	toolSpan := byName[tlsToolSpanName]
	if root == nil || modelSpan == nil || toolSpan == nil {
		t.Fatalf("expected root/model/tool spans, got %#v", byName)
	}
	if modelSpan.Parent().SpanID() != root.SpanContext().SpanID() {
		t.Fatalf("model parent = %s, want root %s", modelSpan.Parent().SpanID(), root.SpanContext().SpanID())
	}
	if toolSpan.Parent().SpanID() != modelSpan.SpanContext().SpanID() {
		t.Fatalf("tool parent = %s, want model %s", toolSpan.Parent().SpanID(), modelSpan.SpanContext().SpanID())
	}
	toolAttrs := otelAttributesToMap(toolSpan.Attributes())
	if got := toolAttrs[sem_ai.TOOL_SUCCESS]; got != true {
		t.Errorf("tool success = %#v, want true", got)
	}
	if got := toolAttrs[sem_ai.TOOL_ERROR]; got != false {
		t.Errorf("tool error = %#v, want false", got)
	}
}

func TestTLSExporterParentsToolsByToolCallID(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	tracer := provider.Tracer(scopeName)
	rootCtx, rootSpan := tracer.Start(context.Background(), tlsRootSpanName)
	root := &tlsRootState{span: rootSpan}

	_, firstModel := tracer.Start(rootCtx, tlsModelSpanName)
	root.rememberModelSpan(firstModel, "call-weather")
	firstModel.End()
	_, secondModel := tracer.Start(rootCtx, tlsModelSpanName)
	root.rememberModelSpan(secondModel, "call-search")
	secondModel.End()

	handler := &TLSCallbackHandler{TLSExporterEnabled: true}
	toolCtx := handler.contextWithToolCallParent(rootCtx, &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "weather"}, root, "call-weather")
	_, toolSpan := tracer.Start(toolCtx, tlsToolSpanName)
	toolSpan.End()
	rootSpan.End()

	if toolSpan.SpanContext().TraceID() != firstModel.SpanContext().TraceID() {
		t.Fatal("tool span did not preserve the model trace")
	}
	foundTool := false
	for _, span := range recorder.Ended() {
		if span.Name() == tlsToolSpanName && span.Parent().SpanID() != firstModel.SpanContext().SpanID() {
			t.Fatalf("tool parent = %s, want first model %s", span.Parent().SpanID(), firstModel.SpanContext().SpanID())
		} else if span.Name() == tlsToolSpanName {
			foundTool = true
		}
	}
	if !foundTool {
		t.Fatal("tool span was not exported")
	}
}

func TestWaitStreamInputStopsOnCancellationAndTimeout(t *testing.T) {
	stopCh := make(TLSStreamInputAsyncVal)
	canceledCtx, cancel := context.WithCancel(context.WithValue(context.Background(), TLSStreamInputAsyncKey{}, stopCh))
	cancel()
	start := time.Now()
	waitStreamInput(canceledCtx, time.Second)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("canceled stream wait took %s", elapsed)
	}

	timeoutCtx := context.WithValue(context.Background(), TLSStreamInputAsyncKey{}, stopCh)
	start = time.Now()
	waitStreamInput(timeoutCtx, 10*time.Millisecond)
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("timed stream wait took %s", elapsed)
	}
}

func TestTLSApplicationAttributes(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer func() { _ = provider.Shutdown(context.Background()) }()
	_, span := provider.Tracer("test").Start(context.Background(), tlsRootSpanName)
	(&TLSCallbackHandler{AppName: "Eino", Release: "v1", TLSExporterEnabled: true}).setTLSApplicationAttributes(
		span,
		&callbacks.RunInfo{Name: "eino-agent"},
	)
	span.End()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	attrs := otelAttributesToMap(spans[0].Attributes())
	for key, want := range map[string]any{
		"app.name":           "Eino",
		"agent.name":         "eino-agent",
		"tls.plugin.version": "v1",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("%s = %#v, want %#v", key, got, want)
		}
	}
}

func TestValidateTLSLogExporterConfig(t *testing.T) {
	cfg := &TLSConfig{
		AppName:            "Eino",
		TLSExporterEnabled: true,
		TLSLogEndpoint:     "tls-cn-guilin-boe.volces.com",
		TLSLogRegion:       "cn-guilin-boe",
		TLSLogTopicID:      "topic-id",
		TLSLogAPIKey:       "api-key",
	}
	if err := ValidateTLSConfig(cfg); err != nil {
		t.Fatalf("ValidateTLSConfig() error = %v", err)
	}
	cfg.TLSLogEndpoint = ""
	if err := ValidateTLSConfig(cfg); err == nil {
		t.Fatal("ValidateTLSConfig() accepted incomplete TLS log exporter config")
	}
}

func TestLoadTLSConfigFromEnvForTLSExporter(t *testing.T) {
	t.Setenv("TLS_EXPORTER_ENABLED", "true")
	t.Setenv("TLS_APP_NAME", "Eino")
	t.Setenv("TLS_LOG_ENDPOINT", "tls-cn-guilin-boe.volces.com")
	t.Setenv("TLS_LOG_REGION", "cn-guilin-boe")
	t.Setenv("TLS_LOG_TRACE_TOPIC_ID", "topic-id")
	t.Setenv("TLS_LOG_API_KEY", "api-key")

	cfg, err := LoadTLSConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadTLSConfigFromEnv() error = %v", err)
	}
	if !cfg.TLSExporterEnabled {
		t.Fatal("TLSExporterEnabled = false, want true")
	}
}

func TestNormalizeTLSProducerEndpoint(t *testing.T) {
	for input, want := range map[string]string{
		"tls-cn-guilin-boe.volces.com":          "https://tls-cn-guilin-boe.volces.com",
		"https://tls-cn-guilin-boe.volces.com/": "https://tls-cn-guilin-boe.volces.com",
		"http://localhost:8080/":                "http://localhost:8080",
		"":                                      "",
	} {
		if got := normalizeTLSProducerEndpoint(input); got != want {
			t.Errorf("normalizeTLSProducerEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTLSExporterPreservesCurrentEinoMultimodalInputAndOutput(t *testing.T) {
	imageURL := "https://example.com/test-image.png"
	videoURL := "https://example.com/test-video.mp4"
	input := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "describe the attached media"},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{URL: &imageURL, MIMEType: "image/png"},
			}},
			{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{
				MessagePartCommon: schema.MessagePartCommon{URL: &videoURL, MIMEType: "video/mp4"},
			}},
		},
	}
	output := &schema.Message{
		Role: schema.Assistant,
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "the video shows a test scene"},
			{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{
				MessagePartCommon: schema.MessagePartCommon{URL: &videoURL, MIMEType: "video/mp4"},
			}},
		},
	}

	parser := defaultDataParser{}
	inputTags, err := parser.ParseInput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackInput{
		Messages: []*schema.Message{input},
	})
	if err != nil {
		t.Fatalf("parse multimodal input: %v", err)
	}
	inputMessages := modelMessagesFromTag(t, inputTags[sem_ai.GEN_AI_INPUT_MESSAGES])
	lensInput := toTLSLensMessages(inputMessages)
	if len(lensInput) != 1 || len(lensInput[0].Parts) != 3 {
		t.Fatalf("unexpected lens input: %#v", lensInput)
	}
	if lensInput[0].Parts[0].Type != "text" || lensInput[0].Parts[0].Content != "describe the attached media" {
		t.Fatalf("text input part was lost: %#v", lensInput[0].Parts[0])
	}
	if lensInput[0].Parts[1].Type != "image_url" || mediaURL(lensInput[0].Parts[1].ImageURL) != imageURL {
		t.Fatalf("image input part was lost: %#v", lensInput[0].Parts[1])
	}
	if lensInput[0].Parts[2].Type != "video_url" || mediaURL(lensInput[0].Parts[2].VideoURL) != videoURL {
		t.Fatalf("video input part was lost: %#v", lensInput[0].Parts[2])
	}
	normalizeTLSLensMessages(inputTags)
	assertTLSLensValuePayload(t, inputTags[sem_ai.GEN_AI_INPUT], "user", "describe the attached media")
	if got := latestUserMessageContent(inputTags); got != "describe the attached media [image: "+imageURL+"] [video: "+videoURL+"]" {
		t.Fatalf("session input summary = %q", got)
	}

	outputTags, err := parser.ParseOutput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackOutput{Message: output})
	if err != nil {
		t.Fatalf("parse multimodal output: %v", err)
	}
	outputMessages := modelMessagesFromTag(t, outputTags[sem_ai.GEN_AI_OUTPUT_MESSAGES])
	lensOutput := toTLSLensMessages(outputMessages)
	if len(lensOutput) != 1 || len(lensOutput[0].Parts) != 2 {
		t.Fatalf("unexpected lens output: %#v", lensOutput)
	}
	if lensOutput[0].Parts[0].Content != "the video shows a test scene" || mediaURL(lensOutput[0].Parts[1].VideoURL) != videoURL {
		t.Fatalf("multimodal model output was lost: %#v", lensOutput[0].Parts)
	}
	normalizeTLSLensMessages(outputTags)
	assertTLSLensValuePayload(t, outputTags[sem_ai.GEN_AI_OUTPUT], "assistant", "the video shows a test scene")
}

func TestTLSExporterRedactsInlineMediaPayloads(t *testing.T) {
	imageData := "image-inline-payload"
	audioData := "audio-inline-payload"
	videoData := "video-inline-payload"
	fileData := "file-inline-payload"
	input := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &imageData, MIMEType: "image/png"}}},
			{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &audioData, MIMEType: "audio/wav"}}},
			{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &videoData, MIMEType: "video/mp4"}}},
			{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{Name: "private.pdf", MessagePartCommon: schema.MessagePartCommon{Base64Data: &fileData, MIMEType: "application/pdf"}}},
		},
	}
	output := &schema.Message{
		Role: schema.Assistant,
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &imageData, MIMEType: "image/png"}}},
			{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: &audioData, MIMEType: "audio/wav"}}},
			{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{Base64Data: &videoData, MIMEType: "video/mp4"}}},
		},
	}

	for name, message := range map[string]*schema.Message{"input": input, "output": output} {
		encoded, err := json.Marshal(convertModelMessage(message))
		if err != nil {
			t.Fatalf("marshal %s message: %v", name, err)
		}
		for _, payload := range []string{imageData, audioData, videoData, fileData, "base64_data"} {
			if strings.Contains(string(encoded), payload) {
				t.Fatalf("%s telemetry payload leaked %q: %s", name, payload, encoded)
			}
		}
	}
}

func TestTLSExporterPreservesMultimodalAndToolPayloadsInStreams(t *testing.T) {
	imageURL := "https://example.com/stream-image.png"
	videoURL := "https://example.com/stream-video.mp4"
	parser := defaultDataParser{}

	inputReader, inputWriter := schema.Pipe[callbacks.CallbackInput](1)
	inputWriter.Send(&model.CallbackInput{Messages: []*schema.Message{{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "stream this media"},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &imageURL}}},
			{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: schema.MessagePartCommon{URL: &videoURL}}},
		},
	}}}, nil)
	inputWriter.Close()
	inputTags, err := parser.ParseStreamInput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfChatModel}, inputReader)
	if err != nil {
		t.Fatalf("parse stream multimodal input: %v", err)
	}
	streamInput := modelMessagesFromTag(t, inputTags[sem_ai.GEN_AI_INPUT_MESSAGES])
	if len(streamInput) != 1 {
		t.Fatalf("unexpected stream input messages: %#v", inputTags[sem_ai.GEN_AI_INPUT_MESSAGES])
	}
	lensInput := toTLSLensMessages(streamInput)
	if len(lensInput[0].Parts) != 3 || mediaURL(lensInput[0].Parts[1].ImageURL) != imageURL || mediaURL(lensInput[0].Parts[2].VideoURL) != videoURL {
		t.Fatalf("stream multimodal parts were lost: %#v", lensInput)
	}
	if inputTags[sem_ai.GEN_AI_REQUEST_IS_STREAM] != true || inputTags[sem_ai.GEN_AI_IS_STREAMING] != true {
		t.Fatalf("stream flags missing from input: %#v", inputTags)
	}
	normalizeTLSLensMessages(inputTags)
	assertTLSLensValuePayload(t, inputTags[sem_ai.GEN_AI_INPUT], "user", "stream this media")

	modelOutputReader, modelOutputWriter := schema.Pipe[callbacks.CallbackOutput](1)
	modelOutputWriter.Send(&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant, Content: "stream response"}}, nil)
	modelOutputWriter.Close()
	modelOutputTags, err := parser.ParseStreamOutput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfChatModel}, modelOutputReader)
	if err != nil {
		t.Fatalf("parse stream model output: %v", err)
	}
	normalizeTLSLensMessages(modelOutputTags)
	assertTLSLensValuePayload(t, modelOutputTags[sem_ai.GEN_AI_OUTPUT], "assistant", "stream response")

	toolInputReader, toolInputWriter := schema.Pipe[callbacks.CallbackInput](2)
	toolInputWriter.Send(&tool.CallbackInput{ArgumentsInJSON: `{"city":`}, nil)
	toolInputWriter.Send(&tool.CallbackInput{ArgumentsInJSON: `"Guilin"}`}, nil)
	toolInputWriter.Close()
	toolInputTags, err := parser.ParseStreamInput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "weather"}, toolInputReader)
	if err != nil {
		t.Fatalf("parse stream tool input: %v", err)
	}
	if got := toolInputTags[sem_ai.GEN_AI_TOOL_CALL_ARGUMENTS]; got != `{"city":"Guilin"}` {
		t.Fatalf("stream tool arguments = %#v", got)
	}

	toolOutputReader, toolOutputWriter := schema.Pipe[callbacks.CallbackOutput](2)
	toolOutputWriter.Send(&tool.CallbackOutput{Response: `{"temperature":`}, nil)
	toolOutputWriter.Send(&tool.CallbackOutput{Response: `25}`}, nil)
	toolOutputWriter.Close()
	toolOutputTags, err := parser.ParseStreamOutput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "weather"}, toolOutputReader)
	if err != nil {
		t.Fatalf("parse stream tool output: %v", err)
	}
	if got := toolOutputTags[sem_ai.GEN_AI_TOOL_CALL_RESULT]; got != `{"temperature":25}` {
		t.Fatalf("stream tool result = %#v", got)
	}
}

func TestTLSExporterUsesRawToolPayloadsAndLensToolParts(t *testing.T) {
	parser := defaultDataParser{}
	inputTags, err := parser.ParseInput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "weather"}, &tool.CallbackInput{
		ArgumentsInJSON: `{"city":"Guilin"}`,
	})
	if err != nil {
		t.Fatalf("parse tool input: %v", err)
	}
	if got := inputTags[sem_ai.GEN_AI_TOOL_CALL_ARGUMENTS]; got != `{"city":"Guilin"}` {
		t.Fatalf("tool arguments = %#v", got)
	}
	outputTags, err := parser.ParseOutput(context.Background(), &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "weather"}, &tool.CallbackOutput{
		Response: `{"temperature":25}`,
	})
	if err != nil {
		t.Fatalf("parse tool output: %v", err)
	}
	if got := outputTags[sem_ai.GEN_AI_TOOL_CALL_RESULT]; got != `{"temperature":25}` {
		t.Fatalf("tool result = %#v", got)
	}

	lensMessages := toTLSLensMessages([]*sem_ai.ModelMessage{
		{
			Role: "assistant",
			ToolCalls: []*sem_ai.ModelToolCall{{
				ID:       "call-weather",
				Function: &sem_ai.ModelToolCallFunction{Name: "weather", Arguments: `{"city":"Guilin"}`},
			}},
		},
		{Role: "tool", ToolCallID: "call-weather", Content: `{"temperature":25}`},
	})
	if len(lensMessages) != 2 || len(lensMessages[0].Parts) != 1 || len(lensMessages[1].Parts) != 1 {
		t.Fatalf("unexpected lens tool messages: %#v", lensMessages)
	}
	if call := lensMessages[0].Parts[0]; call.Type != "tool_call" || call.ID != "call-weather" || call.Name != "weather" {
		t.Fatalf("tool call part = %#v", call)
	}
	if response := lensMessages[1].Parts[0]; response.Type != "tool_call_response" || response.ID != "call-weather" || response.Result != `{"temperature":25}` {
		t.Fatalf("tool response part = %#v", response)
	}
}

func TestTLSLensMessagesCoalescesStreamedContentParts(t *testing.T) {
	messages := toTLSLensMessages([]*sem_ai.ModelMessage{{
		Role:    "assistant",
		Content: "streamed text",
		Parts: []*sem_ai.ModelMessagePart{
			{Type: sem_ai.ModelMessagePartTypeText, Text: "streamed"},
			{Type: sem_ai.ModelMessagePartTypeText, Text: " text"},
			{Type: sem_ai.ModelMessagePartTypeReasoning, Text: "reason"},
			{Type: sem_ai.ModelMessagePartTypeReasoning, Text: "ing"},
		},
	}})
	if len(messages) != 1 || len(messages[0].Parts) != 2 {
		t.Fatalf("stream chunks should be coalesced, got %#v", messages)
	}
	if got := messages[0].Parts[0]; got.Type != "text" || got.Content != "streamed text" {
		t.Fatalf("stream text = %#v, want one content part", got)
	}
	if got := messages[0].Parts[1]; got.Type != "reasoning" || got.Content != "reasoning" {
		t.Fatalf("stream reasoning = %#v, want one content part", got)
	}
}

func mediaURL(raw any) string {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	var media struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(encoded, &media) != nil {
		return ""
	}
	return media.URL
}

func assertTLSLensValuePayload(t *testing.T, raw any, wantRole, wantContent string) {
	t.Helper()
	payload, ok := raw.(tlsLensMessagesPayload)
	if !ok {
		t.Fatalf("trace detail value type = %T, want tlsLensMessagesPayload", raw)
	}
	if len(payload.Messages) != 1 || payload.Messages[0].Role != wantRole || len(payload.Messages[0].Parts) == 0 {
		t.Fatalf("unexpected trace detail payload: %#v", payload)
	}
	if got := payload.Messages[0].Parts[0].Content; got != wantContent {
		t.Fatalf("trace detail content = %#v, want %q", got, wantContent)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal trace detail payload: %v", err)
	}
	if strings.Contains(string(encoded), `"stream"`) || strings.Contains(string(encoded), `"choices"`) {
		t.Fatalf("trace detail payload leaked callback wrapper: %s", encoded)
	}
}

func modelMessagesFromTag(t *testing.T, raw any) []*sem_ai.ModelMessage {
	t.Helper()
	if messages, ok := raw.([]*sem_ai.ModelMessage); ok {
		return messages
	}
	encoded, ok := raw.(string)
	if !ok {
		t.Fatalf("message tag type = %T, want JSON string", raw)
	}
	var messages []*sem_ai.ModelMessage
	if err := json.Unmarshal([]byte(encoded), &messages); err != nil {
		t.Fatalf("decode message tag: %v; raw=%s", err, encoded)
	}
	return messages
}
