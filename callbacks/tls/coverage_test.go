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
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestConvertCallbacksToTLSData(t *testing.T) {
	toolInfo := &schema.ToolInfo{
		Name: "weather",
		Desc: "returns the weather for a city",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {Type: schema.String, Required: true},
		}),
	}
	convertedTool := convertTool(toolInfo)
	if convertedTool == nil || convertedTool.Function == nil {
		t.Fatal("convertTool returned an incomplete tool")
	}
	if convertedTool.Type != toolTypeFunction || convertedTool.Function.Name != "weather" {
		t.Fatalf("unexpected converted tool: %#v", convertedTool)
	}
	if !strings.Contains(string(convertedTool.Function.Parameters), "city") {
		t.Fatalf("tool parameters did not preserve the JSON schema: %s", convertedTool.Function.Parameters)
	}
	if convertTool(nil) != nil {
		t.Fatal("convertTool(nil) must return nil")
	}

	config := &model.Config{Temperature: 0.7, MaxTokens: 128, TopP: 0.9}
	modelOptions := convertModelCallOption(config)
	if modelOptions == nil || modelOptions.MaxTokens != 128 || modelOptions.Temperature != 0.7 || modelOptions.TopP != 0.9 {
		t.Fatalf("unexpected model call options: %#v", modelOptions)
	}
	if convertModelCallOption(nil) != nil {
		t.Fatal("convertModelCallOption(nil) must return nil")
	}

	promptInput := convertPromptInput(&prompt.CallbackInput{
		Templates: []schema.MessagesTemplate{
			&schema.Message{Role: schema.System, Content: "be concise"},
			schema.MessagesPlaceholder("history", true),
		},
		Variables: map[string]any{"city": "Guilin", "days": 3},
	})
	if promptInput == nil || len(promptInput.Templates) != 2 || promptInput.Templates[0].Content != "be concise" || promptInput.Templates[1] != nil {
		t.Fatalf("unexpected prompt input: %#v", promptInput)
	}
	if len(promptInput.Arguments) != 2 {
		t.Fatalf("prompt arguments = %#v, want two arguments", promptInput.Arguments)
	}
	if convertPromptInput(nil) != nil || convertPromptOutput(nil) != nil || convertTemplate(nil) != nil || convertPromptArguments(nil) != nil {
		t.Fatal("nil prompt values must remain nil")
	}

	promptOutput := convertPromptOutput(&prompt.CallbackOutput{Result: []*schema.Message{{Role: schema.Assistant, Content: "sunny"}}})
	if promptOutput == nil || len(promptOutput.Prompts) != 1 || promptOutput.Prompts[0].Content != "sunny" {
		t.Fatalf("unexpected prompt output: %#v", promptOutput)
	}

	document := (&schema.Document{ID: "doc-1", Content: "TLS callback docs"}).WithScore(0.95).WithDenseVector([]float64{1, 2})
	retrieverOutput := convertRetrieverOutput(&retriever.CallbackOutput{Docs: []*schema.Document{document, nil}})
	if retrieverOutput == nil || len(retrieverOutput.Documents) != 2 || retrieverOutput.Documents[0].Score != 0.95 || !reflect.DeepEqual(retrieverOutput.Documents[0].Vector, []float64{1, 2}) || retrieverOutput.Documents[1] != nil {
		t.Fatalf("unexpected retriever output: %#v", retrieverOutput)
	}
	threshold := 0.8
	retrieverOptions := convertRetrieverCallOption(&retriever.CallbackInput{TopK: 5, Filter: "lang=zh", ScoreThreshold: &threshold})
	if retrieverOptions == nil || retrieverOptions.TopK != 5 || retrieverOptions.Filter != "lang=zh" || retrieverOptions.MinScore == nil || *retrieverOptions.MinScore != threshold {
		t.Fatalf("unexpected retriever options: %#v", retrieverOptions)
	}
	if convertRetrieverOutput(nil) != nil || convertRetrieverCallOption(nil) != nil || convertDocument(nil) != nil {
		t.Fatal("nil retriever values must remain nil")
	}
}

func TestOptionsAndLegacyMessageSerialization(t *testing.T) {
	parser := NewDefaultDataParser(false)
	options := newOptions(
		WithCallbackDataParser(parser),
		WithConcatFunction(func(chunks []string) (string, error) { return strings.Join(chunks, ""), nil }),
		WithAggrMessageOutput(false),
	)
	if options.parser != parser || options.enableAggrOutput {
		t.Fatalf("unexpected options: %#v", options)
	}
	if _, ok := options.concatFuncs[reflect.TypeOf("")]; !ok {
		t.Fatalf("string concat function was not registered: %#v", options.concatFuncs)
	}
	if _, ok := newDefaultDataParserWithConcatFuncs(nil, true).(*defaultDataParser); !ok {
		t.Fatal("nil concat functions must use the default parser")
	}
	if custom, ok := newDefaultDataParserWithConcatFuncs(options.concatFuncs, false).(*defaultDataParser); !ok || custom.enableAggrMessageOutput {
		t.Fatalf("custom concat parser did not preserve options: %#v", custom)
	}

	if got := _promptTemplateInput(&prompt.CallbackInput{}); got != nil {
		t.Fatalf("empty prompt template attributes = %#v, want nil", got)
	}
	if got := _promptTemplateInput(&prompt.CallbackInput{Templates: []schema.MessagesTemplate{&schema.Message{Role: schema.User, Content: "hello"}}}); len(got) != 0 {
		t.Fatalf("prompt template attributes = %#v, want empty", got)
	}

	message := &schema.Message{
		Role:    schema.User,
		Content: "describe these media items",
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "caption"},
			{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image.png"}},
			{Type: schema.ChatMessagePartTypeAudioURL, AudioURL: &schema.ChatMessageAudioURL{URL: "https://example.com/audio.wav"}},
			{Type: schema.ChatMessagePartTypeVideoURL, VideoURL: &schema.ChatMessageVideoURL{URL: "https://example.com/video.mp4"}},
			{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URL: "https://example.com/spec.pdf"}},
		},
	}
	parts, err := convertMultiContentToJSON(message)
	if err != nil || len(parts) != 5 {
		t.Fatalf("convertMultiContentToJSON() = %#v, %v; want five parts", parts, err)
	}
	if _, err := convertMultiContentToJSON(nil); err == nil {
		t.Fatal("convertMultiContentToJSON(nil) must return an error")
	}

	inputAttrs := _llmModelInput(&model.CallbackInput{
		Messages: []*schema.Message{message},
		Tools:    []*schema.ToolInfo{{Name: "weather", Desc: "looks up weather"}},
		Config:   &model.Config{Model: "ep-test", Temperature: 0.2, TopP: 0.4},
	})
	if len(inputAttrs) < 7 {
		t.Fatalf("model input attributes = %#v, want prompt, multimodal, tool, and config attributes", inputAttrs)
	}
	if _llmModelInput(nil) != nil || _llmModelInput(&model.CallbackInput{}) != nil {
		t.Fatal("empty model input must not create attributes")
	}

	outputAttrs := _llmModelOutput(&model.CallbackOutput{
		Message:    &schema.Message{ReasoningContent: "checked forecast"},
		TokenUsage: &model.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
	})
	if len(outputAttrs) != 4 {
		t.Fatalf("model output attributes = %#v, want token and reasoning attributes", outputAttrs)
	}
	if _llmModelOutput(nil) != nil || _llmModelOutput(&model.CallbackOutput{}) != nil {
		t.Fatal("empty model output must not create attributes")
	}
	if _toolsOutput(nil) != nil || _toolsOutput(&tool.CallbackOutput{}) != nil {
		t.Fatal("empty tool output must not create attributes")
	}
	if got := _toolsOutput(&tool.CallbackOutput{Response: "sunny"}); len(got) != 0 {
		t.Fatalf("tool output attributes = %#v, want empty until a tool result semantic convention is emitted", got)
	}

	tags := spanTags{}
	tags.set("string", "value").set("string", "overwritten").set("object", map[string]any{"ok": true})
	tags.setIfNotZero("zero", 0)
	tags.setFromExtraIfNotZero("extra", map[string]any{"extra": "set", "empty": ""})
	if tags["string"] != "value" || !strings.Contains(tags["object"].(string), "ok") || tags["zero"] != nil || tags["extra"] != "set" {
		t.Fatalf("unexpected span tags: %#v", tags)
	}
}

func TestTLSConstructorsAndErrorCallback(t *testing.T) {
	cfg := &TLSConfig{
		TLSEndpoint:       "127.0.0.1:4317",
		AppName:           "tls-coverage-test",
		TLSOTLPHeadersStr: "authorization=Bearer test, x-scope = test",
	}
	if err := ValidateTLSConfig(cfg); err != nil {
		t.Fatalf("ValidateTLSConfig() error = %v", err)
	}
	if cfg.TLSOTLPHeader["authorization"] != "Bearer test" || cfg.TLSOTLPHeader["x-scope"] != "test" {
		t.Fatalf("TLS OTLP headers were not parsed: %#v", cfg.TLSOTLPHeader)
	}
	if got := parseOTLPHeaders("invalid, a = one, b= two=three"); got["a"] != "one" || got["b"] != "two=three" || len(got) != 2 {
		t.Fatalf("parseOTLPHeaders() = %#v", got)
	}
	t.Setenv("TLS_COVERAGE_VALUE", "configured")
	if got := getEnvOrDefault("TLS_COVERAGE_VALUE", "fallback"); got != "configured" {
		t.Fatalf("getEnvOrDefault() = %q, want configured", got)
	}
	if got := getEnvOrDefault("TLS_COVERAGE_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("getEnvOrDefault() = %q, want fallback", got)
	}
	if resolved, err := resolveTLSConfig(cfg); err != nil || resolved != cfg {
		t.Fatalf("resolveTLSConfig() = %#v, %v", resolved, err)
	}

	constructors := []struct {
		name string
		new  func() (func(context.Context) error, error)
	}{
		{
			name: "callback handler",
			new: func() (func(context.Context) error, error) {
				_, shutdown, err := NewTLSCallbackHandler(cfg)
				return shutdown, err
			},
		},
		{
			name: "callback handler with options",
			new: func() (func(context.Context) error, error) {
				_, shutdown, err := NewTLSCallbackHandlerWithOptions(cfg, WithAggrMessageOutput(false))
				return shutdown, err
			},
		},
		{
			name: "callback handler from env compatible API",
			new: func() (func(context.Context) error, error) {
				_, shutdown, err := NewTLSCallbackHandlerFromEnv(cfg)
				return shutdown, err
			},
		},
		{
			name: "TLS handler",
			new: func() (func(context.Context) error, error) {
				_, shutdown, err := NewTLSHandler(cfg)
				return shutdown, err
			},
		},
		{
			name: "TLS handler with options",
			new: func() (func(context.Context) error, error) {
				_, shutdown, err := NewTLSHandlerWithOptions(cfg, WithAggrMessageOutput(false))
				return shutdown, err
			},
		},
		{
			name: "TLS handler from env compatible API",
			new: func() (func(context.Context) error, error) {
				_, shutdown, err := NewTLSHandlerFromEnv(cfg)
				return shutdown, err
			},
		},
	}
	for _, tt := range constructors {
		t.Run(tt.name, func(t *testing.T) {
			shutdown, err := tt.new()
			if err != nil {
				t.Fatalf("constructor returned error: %v", err)
			}
			if shutdown == nil {
				t.Fatal("constructor returned nil shutdown")
			}
			if err := shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown returned error: %v", err)
			}
		})
	}

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	handler := &TLSCallbackHandler{
		Tracer:             provider.Tracer(scopeName),
		dataParser:         NewDefaultDataParser(true),
		TLSExporterEnabled: true,
	}
	info := &callbacks.RunInfo{Name: "weather", Type: "coverage", Component: components.ComponentOfTool}
	ctx := handler.OnStart(context.Background(), info, nil)
	handler.OnError(ctx, info, errors.New("tool request failed"))

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want synthetic root plus tool span", len(spans))
	}
	for _, span := range spans {
		if span.Status().Code != codes.Error {
			t.Fatalf("span %q status = %s, want error", span.Name(), span.Status().Code)
		}
	}
}
