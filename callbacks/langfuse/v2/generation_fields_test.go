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
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestGenerationPreservesCallbackFields(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		name := "generate"
		if streaming {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			handler, err := NewHandler(context.Background(), &Config{SpanExporter: exporter})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handler.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			choice := schema.ToolChoiceForced
			parameters := &jsonschema.Schema{}
			if err := json.Unmarshal([]byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`), parameters); err != nil {
				t.Fatal(err)
			}
			input := &model.CallbackInput{
				Messages: []*schema.Message{schema.UserMessage("weather")},
				Config:   &model.Config{Model: "test-model", MaxTokens: 64},
				Tools: []*schema.ToolInfo{
					{Name: "weather", Desc: "Find the weather", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(parameters)},
					nil,
					{Name: "clock"},
				},
				ToolChoice: &choice,
			}
			output := &model.CallbackOutput{
				Config: &model.Config{Model: "test-model", MaxTokens: 64},
				Message: &schema.Message{Role: schema.Assistant, Content: "sunny", ResponseMeta: &schema.ResponseMeta{
					FinishReason: "stop",
					Usage: &schema.TokenUsage{PromptTokens: 100, CompletionTokens: 30, TotalTokens: 130,
						PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: 20, CacheWriteTokens: 10},
						CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 5}},
				}},
			}
			before, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			info := &callbacks.RunInfo{Name: name, Component: components.ComponentOfChatModel}
			if streaming {
				ctx := handler.OnStartWithStreamInput(context.Background(), info,
					schema.StreamReaderFromArray([]callbacks.CallbackInput{input, &model.CallbackInput{}}))
				handler.OnEndWithStreamOutput(ctx, info, schema.StreamReaderFromArray([]callbacks.CallbackOutput{
					&model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant}}, output,
				}))
			} else {
				ctx := handler.OnStart(context.Background(), info, input)
				handler.OnEnd(ctx, info, output)
			}
			if err := handler.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			attrs := attributesByKey(spanByName(t, exporter.GetSpans(), name).Attributes)
			var tools []map[string]any
			raw := attrs["gen_ai.tool.definitions"].Value.AsString()
			if err := json.Unmarshal([]byte(raw), &tools); err != nil {
				t.Fatalf("tool definitions %q: %v", raw, err)
			}
			if len(tools) != 2 || tools[0]["name"] != "weather" || tools[0]["description"] != "Find the weather" || tools[1]["name"] != "clock" {
				t.Fatalf("tool definitions = %#v", tools)
			}
			var wantSchema any
			if err := json.Unmarshal([]byte(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`), &wantSchema); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tools[0]["parameters"], wantSchema) {
				t.Fatalf("parameters = %#v", tools[0]["parameters"])
			}
			assertStringAttribute(t, attrs, "gen_ai.request.tool_choice", "forced")
			var modelParameters map[string]any
			if err := json.Unmarshal([]byte(attrs["langfuse.observation.model.parameters"].Value.AsString()), &modelParameters); err != nil {
				t.Fatal(err)
			}
			if modelParameters["tool_choice"] != "forced" || modelParameters["max_tokens"] != float64(64) {
				t.Fatalf("model parameters = %#v", modelParameters)
			}
			if got := attrs["gen_ai.response.finish_reasons"].Value.AsStringSlice(); !reflect.DeepEqual(got, []string{"stop"}) {
				t.Fatalf("finish reasons = %#v", got)
			}
			assertInt64Attribute(t, attrs, "gen_ai.usage.total_tokens", 130)
			assertInt64Attribute(t, attrs, "gen_ai.usage.cache_creation.input_tokens", 10)
			assertInt64Attribute(t, attrs, "gen_ai.usage.input_tokens", 100)
			assertInt64Attribute(t, attrs, "gen_ai.usage.cache_read.input_tokens", 20)
			assertInt64Attribute(t, attrs, "gen_ai.usage.reasoning.output_tokens", 5)
			after, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("callback input was mutated")
			}
		})
	}
}

func TestGenerationToolDefinitionsHonorMaskAndLimits(t *testing.T) {
	for _, test := range []struct {
		name              string
		maxValue, maxSpan int
	}{
		{"mask", -1, -1}, {"value limit", 80, -1}, {"span limit", -1, 180},
	} {
		t.Run(test.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			handler, err := NewHandler(context.Background(), &Config{
				SpanExporter: exporter, MaxAttributeValueLength: test.maxValue, MaxSpanAttributeBytes: test.maxSpan,
				MaskFunc: func(s string) string { return strings.ReplaceAll(s, "secret", "masked") },
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handler.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			info := &callbacks.RunInfo{Name: test.name, Component: components.ComponentOfChatModel}
			ctx := handler.OnStart(context.Background(), info, &model.CallbackInput{
				Tools: []*schema.ToolInfo{{Name: "lookup", Desc: strings.Repeat("secret ", 100)}},
			})
			handler.OnEnd(ctx, info, &model.CallbackOutput{})
			if err := handler.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			attrs := attributesByKey(spanByName(t, exporter.GetSpans(), test.name).Attributes)
			raw := attrs["gen_ai.tool.definitions"].Value.AsString()
			if strings.Contains(raw, "secret") {
				t.Fatalf("unmasked tool definition: %s", raw)
			}
			if raw != "" && !json.Valid([]byte(raw)) {
				t.Fatalf("invalid JSON: %q", raw)
			}
			if test.name == "mask" && !strings.Contains(raw, "masked") {
				t.Fatalf("missing masked definition: %q", raw)
			}
			if test.maxValue > 0 && len(raw) > test.maxValue {
				t.Fatalf("value exceeds limit: %d", len(raw))
			}
			if test.maxSpan > 0 {
				size := 0
				for key, attr := range attrs {
					size += len(key) + attributeValueSize(attr.Value)
				}
				if size > test.maxSpan {
					t.Fatalf("span attributes %d exceed budget %d", size, test.maxSpan)
				}
			}
		})
	}
}

func TestGenerationUsageFieldsFromCallbackOrMessage(t *testing.T) {
	for _, fromCallback := range []bool{false, true} {
		name := "message"
		if fromCallback {
			name = "callback"
		}
		t.Run(name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			handler, err := NewHandler(context.Background(), &Config{SpanExporter: exporter})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handler.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			output := &model.CallbackOutput{Message: &schema.Message{Role: schema.Assistant,
				ResponseMeta: &schema.ResponseMeta{FinishReason: "length", Usage: &schema.TokenUsage{
					PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15,
					PromptTokenDetails: schema.PromptTokenDetails{CacheWriteTokens: 6},
				}},
			}}
			total, cacheWrite := int64(15), int64(6)
			if fromCallback {
				output.TokenUsage = &model.TokenUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30,
					PromptTokenDetails: model.PromptTokenDetails{CacheWriteTokens: 8}}
				total, cacheWrite = 30, 8
			}
			info := &callbacks.RunInfo{Name: name, Component: components.ComponentOfChatModel}
			ctx := handler.OnStart(context.Background(), info, &model.CallbackInput{})
			handler.OnEnd(ctx, info, output)
			if err := handler.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			attrs := attributesByKey(spanByName(t, exporter.GetSpans(), name).Attributes)
			assertInt64Attribute(t, attrs, "gen_ai.usage.total_tokens", total)
			assertInt64Attribute(t, attrs, "gen_ai.usage.cache_creation.input_tokens", cacheWrite)
			if got := attrs["gen_ai.response.finish_reasons"].Value.AsStringSlice(); !reflect.DeepEqual(got, []string{"length"}) {
				t.Fatalf("finish reasons = %#v", got)
			}
			if _, ok := attrs["gen_ai.tool.definitions"]; ok {
				t.Fatal("tool definitions emitted without tools")
			}
			if _, ok := attrs["gen_ai.request.tool_choice"]; ok {
				t.Fatal("tool choice emitted without a choice")
			}
		})
	}
}

func TestGenerationToolChoiceWithoutConfig(t *testing.T) {
	for _, choice := range []schema.ToolChoice{schema.ToolChoiceAllowed, schema.ToolChoiceForbidden, schema.ToolChoiceForced} {
		t.Run(string(choice), func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			handler, err := NewHandler(context.Background(), &Config{SpanExporter: exporter})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handler.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			info := &callbacks.RunInfo{Name: string(choice), Component: components.ComponentOfChatModel}
			ctx := handler.OnStart(context.Background(), info, &model.CallbackInput{ToolChoice: &choice})
			handler.OnEnd(ctx, info, &model.CallbackOutput{})
			if err := handler.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			attrs := attributesByKey(spanByName(t, exporter.GetSpans(), string(choice)).Attributes)
			assertStringAttribute(t, attrs, "gen_ai.request.tool_choice", string(choice))
			var parameters map[string]string
			if err := json.Unmarshal([]byte(attrs["langfuse.observation.model.parameters"].Value.AsString()), &parameters); err != nil {
				t.Fatal(err)
			}
			if parameters["tool_choice"] != string(choice) {
				t.Fatalf("parameters = %#v", parameters)
			}
		})
	}
}
