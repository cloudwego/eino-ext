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

package openaigo

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

func TestBuildParams_UsesConfigAndOverrides(t *testing.T) {
	maxOut := 50
	cfgTopP := float32(0.7)
	cfgTemp := float32(0.3)
	store := true
	cm := &ChatModel{
		model:       "gpt-4o-mini",
		maxOutTok:   &maxOut,
		topP:        &cfgTopP,
		temperature: &cfgTemp,
		reasoning: &Reasoning{
			Effort:  ReasoningEffortMedium,
			Summary: ReasoningSummaryConcise,
		},
		store:       &store,
		metadata:    map[string]string{"scope": "config"},
		extraFields: map[string]any{"base_only": true},
		tools:       []responses.ToolUnionParam{responses.ToolParamOfFunction("default_tool", map[string]any{"type": "object"}, true)},
		rawTools:    []*schema.ToolInfo{makeWeatherTool()},
	}

	requestTool := makeLookupTool()
	forcedMax := 99
	overrideReasoning := &Reasoning{Effort: ReasoningEffortHigh, Summary: ReasoningSummaryDetailed}
	params, cbIn, err := cm.buildParams([]*schema.Message{{Role: schema.User, Content: "hello"}}, true,
		model.WithModel("gpt-test"),
		model.WithTemperature(0.9),
		model.WithTopP(0.4),
		WithMaxOutputTokens(forcedMax),
		WithReasoning(overrideReasoning),
		WithStore(false),
		WithMetadata(map[string]string{"scope": "request"}),
		WithExtraFields(map[string]any{"exp": "beta"}),
		model.WithTools([]*schema.ToolInfo{requestTool}),
		model.WithToolChoice(schema.ToolChoiceForced, requestTool.Name),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := string(params.Model); got != "gpt-test" {
		t.Fatalf("expected model override, got %q", got)
	}
	if got := optInt64(params.MaxOutputTokens); got != int64(forcedMax) {
		t.Fatalf("expected max output tokens %d, got %d", forcedMax, got)
	}
	if got := optFloat64(params.Temperature); got < 0.899 || got > 0.901 {
		t.Fatalf("expected temperature 0.9, got %v", got)
	}
	if got := optFloat64(params.TopP); got < 0.399 || got > 0.401 {
		t.Fatalf("expected top_p 0.4, got %v", got)
	}
	if !params.Store.Valid() || params.Store.Value != false {
		t.Fatalf("expected store override to false, got %#v", params.Store)
	}
	if !params.StreamOptions.IncludeObfuscation.Valid() || params.StreamOptions.IncludeObfuscation.Value != false {
		t.Fatalf("expected stream options for streaming request, got %#v", params.StreamOptions)
	}
	if got := params.Metadata["scope"]; got != "request" {
		t.Fatalf("expected request metadata override, got %q", got)
	}
	if got := string(params.Reasoning.Effort); got != "high" {
		t.Fatalf("expected reasoning effort high, got %q", got)
	}
	if got := string(params.Reasoning.Summary); got != "detailed" {
		t.Fatalf("expected reasoning summary detailed, got %q", got)
	}
	if len(params.Tools) != 1 || params.Tools[0].OfFunction == nil || params.Tools[0].OfFunction.Name != requestTool.Name {
		t.Fatalf("expected request tool to be used, got %#v", params.Tools)
	}
	if params.ToolChoice.OfFunctionTool == nil || params.ToolChoice.OfFunctionTool.Name != requestTool.Name {
		t.Fatalf("expected forced function tool choice, got %#v", params.ToolChoice)
	}
	if got := params.Input.OfInputItemList; len(got) != 1 || got[0].OfMessage == nil {
		t.Fatalf("expected one input message, got %#v", got)
	}

	body, err := params.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	jsonBody := string(body)
	if !strings.Contains(jsonBody, `"exp":"beta"`) {
		t.Fatalf("expected extra fields in marshaled request, got %s", jsonBody)
	}

	if cbIn == nil || cbIn.Config == nil {
		t.Fatalf("expected callback input config")
	}
	if cbIn.Config.Model != "gpt-test" || cbIn.Config.MaxTokens != forcedMax {
		t.Fatalf("unexpected callback config: %#v", cbIn.Config)
	}
	if len(cbIn.Tools) != 1 || cbIn.Tools[0] != requestTool {
		t.Fatalf("expected callback tools to use request tools, got %#v", cbIn.Tools)
	}
	if cbIn.ToolChoice == nil || *cbIn.ToolChoice != schema.ToolChoiceForced {
		t.Fatalf("unexpected callback tool choice: %#v", cbIn.ToolChoice)
	}
}

func TestBuildParams_ErrorCases(t *testing.T) {
	cm := &ChatModel{model: "gpt-4o-mini"}

	_, _, err := cm.buildParams([]*schema.Message{{Role: schema.User, Content: "hi"}}, false,
		model.WithToolChoice(schema.ToolChoiceForced),
	)
	if err == nil || !strings.Contains(err.Error(), "no tools") {
		t.Fatalf("expected forced tool choice error, got %v", err)
	}

	_, _, err = cm.buildParams([]*schema.Message{{Role: schema.User, Content: "hi"}}, false,
		model.WithTools([]*schema.ToolInfo{nil}),
	)
	if err == nil || !strings.Contains(err.Error(), "tool info cannot be nil") {
		t.Fatalf("expected nil tool error, got %v", err)
	}
}

func TestToInputItems_MixedMessages(t *testing.T) {
	imgURL := "https://example.com/cat.png"
	fileURL := "https://example.com/report.pdf"
	fileData := "cGRm"
	items, err := toInputItems([]*schema.Message{
		nil,
		{Role: schema.System, Content: "You are helpful"},
		{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "see image"},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{URL: &imgURL},
				Detail:            schema.ImageURLDetailHigh,
			}},
			{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{
				MessagePartCommon: schema.MessagePartCommon{URL: &fileURL},
				Name:              "report.pdf",
			}},
		}},
		{Role: schema.Assistant, Content: "working", ToolCalls: []schema.ToolCall{{
			ID:       "call_1",
			Type:     "function",
			Function: schema.FunctionCall{Name: "lookup_weather", Arguments: `{"city":"beijing"}`},
		}}},
		{Role: schema.Tool, ToolCallID: "call_1", UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "sunny"},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{URL: &imgURL},
			}},
			{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{
				MessagePartCommon: schema.MessagePartCommon{Base64Data: &fileData, MIMEType: "application/pdf"},
				Name:              "inline.pdf",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 5 {
		t.Fatalf("expected 5 input items, got %d", len(items))
	}
	if items[0].OfMessage == nil || items[0].OfMessage.Role != responses.EasyInputMessageRoleSystem {
		t.Fatalf("expected first item to be a system message, got %#v", items[0])
	}
	if items[1].OfMessage == nil || items[1].OfMessage.Role != responses.EasyInputMessageRoleUser {
		t.Fatalf("expected second item to be a user message, got %#v", items[1])
	}
	if items[2].OfMessage == nil || items[2].OfMessage.Role != responses.EasyInputMessageRoleAssistant {
		t.Fatalf("expected third item to be an assistant history message, got %#v", items[2])
	}
	if items[3].OfFunctionCall == nil || items[3].OfFunctionCall.CallID != "call_1" {
		t.Fatalf("expected fourth item to be the assistant tool call, got %#v", items[3])
	}
	if items[4].OfFunctionCallOutput == nil || items[4].OfFunctionCallOutput.CallID != "call_1" {
		t.Fatalf("expected fifth item to be tool output, got %#v", items[4])
	}
}

func TestToInputItems_ErrorCases(t *testing.T) {
	tests := []struct {
		name string
		msg  *schema.Message
		want string
	}{
		{name: "tool missing call id", msg: &schema.Message{Role: schema.Tool, Content: "oops"}, want: "ToolCallID"},
		{name: "tool image nil", msg: &schema.Message{Role: schema.Tool, ToolCallID: "call", UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL}}}, want: "image field must not be nil"},
		{name: "unknown role", msg: &schema.Message{Role: schema.RoleType("mystery"), Content: "hi"}, want: "unknown role"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := toInputItems([]*schema.Message{tt.msg})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestToInputContentFromMessage(t *testing.T) {
	imgURL := "https://example.com/img.png"
	fileData := "aGVsbG8="

	t.Run("user multimodal", func(t *testing.T) {
		parts, err := toInputContentFromMessage(&schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "hello"},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &imgURL}}},
			{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &fileData, MIMEType: "text/plain"}, Name: "note.txt"}},
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(parts) != 3 {
			t.Fatalf("expected 3 parts, got %d", len(parts))
		}
	})

	t.Run("assistant output text parts", func(t *testing.T) {
		parts, err := toInputContentFromMessage(&schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "one"}, {Type: schema.ChatMessagePartTypeText, Text: "two"}}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(parts) != 2 {
			t.Fatalf("expected 2 parts, got %d", len(parts))
		}
	})

	t.Run("deprecated multi content", func(t *testing.T) {
		parts, err := toInputContentFromMessage(&schema.Message{Role: schema.User, MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeText, Text: "legacy"}, {Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: imgURL}}}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(parts) != 2 {
			t.Fatalf("expected 2 parts, got %d", len(parts))
		}
	})

	t.Run("empty assistant content allowed", func(t *testing.T) {
		parts, err := toInputContentFromMessage(&schema.Message{Role: schema.Assistant})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(parts) != 0 {
			t.Fatalf("expected empty parts, got %d", len(parts))
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name string
			msg  *schema.Message
			want string
		}{
			{name: "mixed content fields", msg: &schema.Message{UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "u"}}, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "a"}}}, want: "cannot contain both"},
			{name: "assistant non text output part", msg: &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeImageURL}}}, want: "unsupported assistant output part type"},
			{name: "deprecated unsupported part", msg: &schema.Message{Role: schema.User, MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeFileURL}}}, want: "unsupported deprecated MultiContent"},
			{name: "empty user content", msg: &schema.Message{Role: schema.User}, want: "message content is empty"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := toInputContentFromMessage(tt.msg)
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected error containing %q, got %v", tt.want, err)
				}
			})
		}
	})
}

func TestToInputContentPartFromInputPart(t *testing.T) {
	imgURL := "https://example.com/img.png"
	fileData := "YmFzZTY0"

	if _, err := toInputContentPartFromInputPart(schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: "ok"}); err != nil {
		t.Fatalf("text part should succeed: %v", err)
	}
	if _, err := toInputContentPartFromInputPart(schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &imgURL}, Detail: schema.ImageURLDetailLow}}); err != nil {
		t.Fatalf("image part should succeed: %v", err)
	}
	if _, err := toInputContentPartFromInputPart(schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &fileData, MIMEType: "text/plain"}, Name: "x.txt"}}); err != nil {
		t.Fatalf("file part should succeed: %v", err)
	}

	tests := []struct {
		name string
		part schema.MessageInputPart
		want string
	}{
		{name: "nil image", part: schema.MessageInputPart{Type: schema.ChatMessagePartTypeImageURL}, want: "image field must not be nil"},
		{name: "nil file", part: schema.MessageInputPart{Type: schema.ChatMessagePartTypeFileURL}, want: "file field must not be nil"},
		{name: "unsupported type", part: schema.MessageInputPart{Type: schema.ChatMessagePartTypeAudioURL}, want: "unsupported content type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := toInputContentPartFromInputPart(tt.part)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestToFunctionCallOutputFromToolMessageDeprecated(t *testing.T) {
	if _, err := toFunctionCallOutputFromToolMessage(&schema.Message{}); err == nil || !strings.Contains(err.Error(), "deprecated") {
		t.Fatalf("expected deprecated error, got %v", err)
	}
}

func TestPopulateToolChoice(t *testing.T) {
	tools := []responses.ToolUnionParam{
		responses.ToolParamOfFunction("lookup_weather", map[string]any{"type": "object"}, true),
		responses.ToolParamOfFunction("lookup_stock", map[string]any{"type": "object"}, true),
	}

	t.Run("nil tool choice", func(t *testing.T) {
		params := responses.ResponseNewParams{}
		if err := populateToolChoice(&params, nil, nil, tools); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !param.IsOmitted(params.ToolChoice.OfToolChoiceMode) || params.ToolChoice.OfFunctionTool != nil {
			t.Fatalf("expected omitted tool choice, got %#v", params.ToolChoice)
		}
	})

	t.Run("forbidden", func(t *testing.T) {
		params := responses.ResponseNewParams{}
		choice := schema.ToolChoiceForbidden
		if err := populateToolChoice(&params, &choice, nil, tools); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode := params.ToolChoice.OfToolChoiceMode.Value; mode != responses.ToolChoiceOptionsNone {
			t.Fatalf("expected none mode, got %q", mode)
		}
	})

	t.Run("allowed", func(t *testing.T) {
		params := responses.ResponseNewParams{}
		choice := schema.ToolChoiceAllowed
		if err := populateToolChoice(&params, &choice, nil, tools); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode := params.ToolChoice.OfToolChoiceMode.Value; mode != responses.ToolChoiceOptionsAuto {
			t.Fatalf("expected auto mode, got %q", mode)
		}
	})

	t.Run("forced single allowed tool", func(t *testing.T) {
		params := responses.ResponseNewParams{}
		choice := schema.ToolChoiceForced
		if err := populateToolChoice(&params, &choice, []string{"lookup_stock"}, tools); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if params.ToolChoice.OfFunctionTool == nil || params.ToolChoice.OfFunctionTool.Name != "lookup_stock" {
			t.Fatalf("expected specific function tool, got %#v", params.ToolChoice)
		}
	})

	t.Run("forced single tool fallback", func(t *testing.T) {
		params := responses.ResponseNewParams{}
		choice := schema.ToolChoiceForced
		if err := populateToolChoice(&params, &choice, nil, tools[:1]); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if params.ToolChoice.OfFunctionTool == nil || params.ToolChoice.OfFunctionTool.Name != "lookup_weather" {
			t.Fatalf("expected only tool to be forced, got %#v", params.ToolChoice)
		}
	})

	t.Run("forced required mode", func(t *testing.T) {
		params := responses.ResponseNewParams{}
		choice := schema.ToolChoiceForced
		if err := populateToolChoice(&params, &choice, nil, tools); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode := params.ToolChoice.OfToolChoiceMode.Value; mode != responses.ToolChoiceOptionsRequired {
			t.Fatalf("expected required mode, got %q", mode)
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name    string
			choice  schema.ToolChoice
			allowed []string
			tools   []responses.ToolUnionParam
			want    string
		}{
			{name: "forced no tools", choice: schema.ToolChoiceForced, want: "no tools"},
			{name: "multiple allowed names", choice: schema.ToolChoiceForced, tools: tools, allowed: []string{"a", "b"}, want: "only one allowed tool name"},
			{name: "allowed tool missing", choice: schema.ToolChoiceForced, tools: tools, allowed: []string{"missing"}, want: "not found"},
			{name: "unknown choice", choice: schema.ToolChoice("mystery"), tools: tools, want: "unknown tool choice"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				params := responses.ResponseNewParams{}
				err := populateToolChoice(&params, &tt.choice, tt.allowed, tt.tools)
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected error containing %q, got %v", tt.want, err)
				}
			})
		}
	})
}

func TestToOpenAITools(t *testing.T) {
	weatherTool := makeWeatherTool()
	tools, rawTools, err := toOpenAITools([]*schema.ToolInfo{weatherTool})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || len(rawTools) != 1 {
		t.Fatalf("expected one tool, got tools=%d rawTools=%d", len(tools), len(rawTools))
	}
	if rawTools[0] != weatherTool {
		t.Fatalf("expected raw tool to be preserved")
	}
	fn := tools[0].OfFunction
	if fn == nil {
		t.Fatalf("expected function tool")
	}
	if fn.Name != weatherTool.Name || !fn.Strict.Value {
		t.Fatalf("unexpected function tool: %#v", fn)
	}
	props, ok := fn.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map in parameters: %#v", fn.Parameters)
	}
	if _, ok := props["city"]; !ok {
		t.Fatalf("expected city property, got %#v", props)
	}
	if fn.Parameters["type"] != "object" || fn.Parameters["additionalProperties"] != false {
		t.Fatalf("expected strict object schema, got %#v", fn.Parameters)
	}
	if required, ok := fn.Parameters["required"].([]any); !ok || len(required) == 0 {
		t.Fatalf("expected required fields, got %#v", fn.Parameters["required"])
	}

	if _, _, err := toOpenAITools([]*schema.ToolInfo{nil}); err == nil || !strings.Contains(err.Error(), "cannot be nil") {
		t.Fatalf("expected nil tool error, got %v", err)
	}
}

func TestToSDKImageDetail(t *testing.T) {
	if got := toSDKImageDetail(schema.ImageURLDetailHigh); got != responses.ResponseInputImageDetailHigh {
		t.Fatalf("expected high detail, got %q", got)
	}
	if got := toSDKImageDetail(schema.ImageURLDetailLow); got != responses.ResponseInputImageDetailLow {
		t.Fatalf("expected low detail, got %q", got)
	}
	if got := toSDKImageDetail(schema.ImageURLDetailAuto); got != responses.ResponseInputImageDetailAuto {
		t.Fatalf("expected auto detail, got %q", got)
	}
	if got := toSDKImageDetail(schema.ImageURLDetail("unknown")); got != responses.ResponseInputImageDetailAuto {
		t.Fatalf("expected unknown detail to default to auto, got %q", got)
	}
}

func TestConvertResponseToMessage(t *testing.T) {
	resp := &responses.Response{
		Status: responses.ResponseStatusCompleted,
		Usage: responses.ResponseUsage{
			InputTokens:         11,
			OutputTokens:        7,
			TotalTokens:         18,
			InputTokensDetails:  responses.ResponseUsageInputTokensDetails{CachedTokens: 3},
			OutputTokensDetails: responses.ResponseUsageOutputTokensDetails{ReasoningTokens: 2},
		},
		Output: []responses.ResponseOutputItemUnion{
			mustJSON[responses.ResponseOutputItemUnion](t, map[string]any{
				"type":   "message",
				"id":     "msg_1",
				"role":   "assistant",
				"status": "completed",
				"content": []map[string]any{{
					"type":        "output_text",
					"text":        "Hello world",
					"annotations": []any{},
				}},
			}),
			mustJSON[responses.ResponseOutputItemUnion](t, map[string]any{
				"type":      "function_call",
				"id":        "item_fc_1",
				"call_id":   "call_1",
				"name":      "lookup_weather",
				"arguments": `{"city":"shanghai"}`,
			}),
			mustJSON[responses.ResponseOutputItemUnion](t, map[string]any{
				"type":   "image_generation_call",
				"id":     "img_1",
				"status": "completed",
				"result": "ZmFrZS1pbWFnZQ==",
			}),
			mustJSON[responses.ResponseOutputItemUnion](t, map[string]any{
				"type": "reasoning",
				"id":   "reason_1",
				"summary": []map[string]any{{
					"type": "summary_text",
					"text": "summary text",
				}},
			}),
		},
	}

	msg, err := (&ChatModel{}).convertResponseToMessage(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Role != schema.Assistant || msg.Content != "Hello world" {
		t.Fatalf("unexpected message: %#v", msg)
	}
	if msg.ResponseMeta == nil || msg.ResponseMeta.FinishReason != string(responses.ResponseStatusCompleted) {
		t.Fatalf("unexpected response meta: %#v", msg.ResponseMeta)
	}
	if msg.ResponseMeta.Usage == nil || msg.ResponseMeta.Usage.TotalTokens != 18 {
		t.Fatalf("unexpected token usage: %#v", msg.ResponseMeta.Usage)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call_1" || msg.ToolCalls[0].Function.Name != "lookup_weather" {
		t.Fatalf("unexpected tool calls: %#v", msg.ToolCalls)
	}
	if msg.ReasoningContent != "summary text" {
		t.Fatalf("unexpected reasoning content: %q", msg.ReasoningContent)
	}
	if len(msg.AssistantGenMultiContent) != 2 {
		t.Fatalf("expected text and image output parts, got %#v", msg.AssistantGenMultiContent)
	}
	if msg.AssistantGenMultiContent[0].Type != schema.ChatMessagePartTypeText || msg.AssistantGenMultiContent[0].Text != "Hello world" {
		t.Fatalf("unexpected first output part: %#v", msg.AssistantGenMultiContent[0])
	}
	if msg.AssistantGenMultiContent[1].Type != schema.ChatMessagePartTypeImageURL || msg.AssistantGenMultiContent[1].Image == nil || msg.AssistantGenMultiContent[1].Image.Base64Data == nil {
		t.Fatalf("unexpected image output part: %#v", msg.AssistantGenMultiContent[1])
	}
	if got := *msg.AssistantGenMultiContent[1].Image.Base64Data; got != "ZmFrZS1pbWFnZQ==" {
		t.Fatalf("unexpected image result %q", got)
	}

	msg, err = (&ChatModel{}).convertResponseToMessage(&responses.Response{Status: responses.ResponseStatusFailed})
	if err != nil {
		t.Fatalf("unexpected error for minimal response: %v", err)
	}
	if msg.ResponseMeta == nil || msg.ResponseMeta.FinishReason != string(responses.ResponseStatusFailed) {
		t.Fatalf("unexpected finish reason: %#v", msg.ResponseMeta)
	}

	if _, err := (&ChatModel{}).convertResponseToMessage(nil); err == nil || !strings.Contains(err.Error(), "nil response") {
		t.Fatalf("expected nil response error, got %v", err)
	}
}

func TestTokenUsageConversions(t *testing.T) {
	if got := toEinoTokenUsage(responses.ResponseUsage{}); got != nil {
		t.Fatalf("expected nil usage for zero-value response usage, got %#v", got)
	}

	usage := responses.ResponseUsage{
		InputTokens:         4,
		OutputTokens:        6,
		TotalTokens:         10,
		InputTokensDetails:  responses.ResponseUsageInputTokensDetails{CachedTokens: 1},
		OutputTokensDetails: responses.ResponseUsageOutputTokensDetails{ReasoningTokens: 2},
	}
	converted := toEinoTokenUsage(usage)
	if converted == nil || converted.PromptTokens != 4 || converted.CompletionTokens != 6 || converted.TotalTokens != 10 {
		t.Fatalf("unexpected converted usage: %#v", converted)
	}
	modelUsage := toModelTokenUsage(&schema.ResponseMeta{Usage: converted})
	if modelUsage == nil || modelUsage.PromptTokens != 4 || modelUsage.CompletionTokensDetails.ReasoningTokens != 2 {
		t.Fatalf("unexpected model token usage: %#v", modelUsage)
	}
	if got := toModelTokenUsage(nil); got != nil {
		t.Fatalf("expected nil model token usage when meta is nil, got %#v", got)
	}
	if got := toModelTokenUsage(&schema.ResponseMeta{}); got != nil {
		t.Fatalf("expected nil model token usage when usage is nil, got %#v", got)
	}
}

func TestOptionHelpers(t *testing.T) {
	got := model.GetImplSpecificOptions(&options{},
		WithMaxOutputTokens(123),
		WithReasoning(&Reasoning{Effort: ReasoningEffortMinimal, Summary: ReasoningSummaryAuto}),
		WithStore(true),
		WithMetadata(map[string]string{"k": "v"}),
		WithExtraFields(map[string]any{"x": 1}),
	)
	if got.MaxOutputTokens == nil || *got.MaxOutputTokens != 123 {
		t.Fatalf("unexpected max output tokens: %#v", got.MaxOutputTokens)
	}
	if got.Reasoning == nil || got.Reasoning.Effort != ReasoningEffortMinimal {
		t.Fatalf("unexpected reasoning option: %#v", got.Reasoning)
	}
	if got.Store == nil || *got.Store != true {
		t.Fatalf("unexpected store option: %#v", got.Store)
	}
	if got.Metadata["k"] != "v" || got.ExtraFields["x"] != 1 {
		t.Fatalf("unexpected option maps: %#v %#v", got.Metadata, got.ExtraFields)
	}

	meta := map[string]string{"k": "v"}
	extra := map[string]any{"x": 1}
	got = model.GetImplSpecificOptions(&options{}, WithMetadata(meta), WithExtraFields(extra))
	meta["k"] = "changed"
	extra["x"] = 2
	if got.Metadata["k"] != "v" || got.ExtraFields["x"] != 1 {
		t.Fatalf("expected cloned maps, got %#v %#v", got.Metadata, got.ExtraFields)
	}
}

func TestReasoningToSDK(t *testing.T) {
	if got := (*Reasoning)(nil).toSDK(); got.Effort != "" || got.Summary != "" {
		t.Fatalf("expected zero value reasoning param, got %#v", got)
	}
	got := (&Reasoning{Effort: ReasoningEffortXHigh, Summary: ReasoningSummaryDetailed}).toSDK()
	if string(got.Effort) != "xhigh" || string(got.Summary) != "detailed" {
		t.Fatalf("unexpected sdk reasoning: %#v", got)
	}
}

func makeWeatherTool() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "lookup_weather",
		Desc: "Look up weather by city",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {Type: schema.String, Desc: "City name", Required: true},
			"unit": {Type: schema.String, Desc: "Temperature unit", Enum: []string{"celsius", "fahrenheit"}},
		}),
	}
}

func makeLookupTool() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "lookup_stock",
		Desc: "Look up stock prices",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"ticker": {Type: schema.String, Required: true},
		}),
	}
}

func mustJSON[T any](t *testing.T, v any) T {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal into target: %v", err)
	}
	return out
}
