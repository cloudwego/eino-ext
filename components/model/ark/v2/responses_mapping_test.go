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

package ark

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func TestResponsesAssistantMediaHistory(t *testing.T) {
	requests := make(chan map[string]any, 1)
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		requests <- decodeRequest(t, r)
		writeResponses(w)
	}, nil)
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "caption"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("image-1")}}},
		{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{URL: ptr("https://example.com/video")}}, Extra: map[string]any{keyVideoFPS: 0.25}},
	}}})
	check(t, err)
	item := (<-requests)["input"].([]any)[0].(map[string]any)
	equal(t, "assistant", item["role"])
	equal(t, []any{
		map[string]any{"type": "input_text", "text": "caption"},
		map[string]any{"type": "input_image", "file_id": "image-1"},
		map[string]any{"type": "input_video", "video_url": "https://example.com/video", "fps": 0.25},
	}, item["content"])
}

func TestResponsesNativeCallOptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		thinking *responses.Thinking
		want     any
	}{
		{"disabled", &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)}, map[string]any{"type": "disabled"}},
		{"unset type", &responses.Thinking{}, map[string]any{}},
		{"omitted", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &ResponsesAPIConfig{Thinking: &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeEnabled)}, ReasoningEffort: ptr(responses.ReasoningEffortLow)}
			requests := make(chan map[string]any, 1)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) { requests <- decodeRequest(t, r); writeResponses(w) }, config)
			var observed any
			handler := callbacks.NewHandlerBuilder().OnStartFn(func(ctx context.Context, _ *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
				observed = input.(*model.CallbackInput).Extra["thinking"]
				return ctx
			}).Build()
			ctx := callbacks.InitCallbacks(context.Background(), nil, handler)
			_, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage("hello")}, WithResponsesThinking(tc.thinking), WithResponsesReasoningEffort(responses.ReasoningEffortHigh))
			check(t, err)
			body := <-requests
			equal(t, tc.want, body["thinking"])
			equal(t, "high", body["reasoning"].(map[string]any)["effort"])
			thinking, ok := observed.(*responses.Thinking)
			equal(t, true, ok)
			equal(t, tc.thinking, thinking)
			equal(t, responses.ThinkingModeEnabled, config.Thinking.Type.Value)
			equal(t, responses.ReasoningEffortLow, *config.ReasoningEffort)
		})
	}
}

func TestResponsesNativeToolChoice(t *testing.T) {
	for _, tc := range []struct {
		name   string
		choice *schema.ToolChoice
		names  []string
		want   any
		count  int
	}{
		{"default", nil, nil, nil, 2},
		{"none", ptr(schema.ToolChoiceForbidden), nil, "none", 2},
		{"auto subset", ptr(schema.ToolChoiceAllowed), []string{"inspect"}, "auto", 1},
		{"required", ptr(schema.ToolChoiceForced), nil, "required", 2},
		{"named", ptr(schema.ToolChoiceForced), []string{"inspect"}, map[string]any{"type": "function", "name": "inspect"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := make(chan map[string]any, 2)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) { requests <- decodeRequest(t, r); writeResponses(w) }, nil)
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect"), toolInfo("other")}))
			var opts []model.Option
			if tc.choice != nil {
				opts = append(opts, model.WithToolChoice(*tc.choice, tc.names...))
			}
			_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}, opts...)
			check(t, err)
			body := <-requests
			equal(t, tc.want, body["tool_choice"])
			equal(t, tc.count, len(body["tools"].([]any)))
			for _, value := range body["tools"].([]any) {
				function := value.(map[string]any)
				equal(t, "function", function["type"])
				if _, nested := function["function"]; nested {
					t.Fatal("Responses function uses the Chat envelope")
				}
				if _, strict := function["strict"]; strict {
					t.Fatal("unset strict must be omitted")
				}
			}
			if len(tc.names) > 0 {
				_, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("next")})
				check(t, err)
				equal(t, 2, len((<-requests)["tools"].([]any)))
			}
		})
	}
}
