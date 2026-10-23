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
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"net/http"
	"testing"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func TestInvalidInputsDoNotSendRequests(t *testing.T) {
	cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid input reached HTTP transport")
		writeResponse(w)
	}, nil)
	cases := map[string][]*schema.Message{
		"nil":                      {nil},
		"unknown role":             {{Role: "unknown", Content: "text"}},
		"mixed":                    {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText}}, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText}}}},
		"wrong input role":         {{Role: schema.Assistant, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText}}}},
		"nil image":                {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL}}}},
		"nil audio":                {{Role: schema.Tool, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeAudioURL}}}},
		"nil video":                {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeVideoURL}}}},
		"nil file":                 {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL}}}},
		"no source":                {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{}}}}},
		"data URL in base64":       {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("data:image/png;base64,eA=="), MIMEType: "image/png", FileID: ptr("fallback")}}}}}},
		"base64 without MIME":      {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("eA=="), FileID: ptr("fallback")}}}}}},
		"file without name":        {{Role: schema.Tool, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("eA=="), MIMEType: "application/pdf"}}}}}},
		"file base64 without MIME": {{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{Name: "fixture.pdf", MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("eA=="), FileID: ptr("fallback")}}}}}},
	}
	for name, messages := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := cm.Generate(context.Background(), messages)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	_, err := cm.Generate(context.Background(), nil, fmodel.WithMaxTokens(1<<32))
	if err == nil {
		t.Fatal("expected integer overflow error")
	}
	_, err = cm.Generate(context.Background(), nil, WithMaxCompletionTokens(1<<32))
	if err == nil {
		t.Fatal("expected integer overflow error")
	}
}

func TestLegacyContentAndAssistantHistory(t *testing.T) {
	video := &schema.ChatMessageVideoURL{URL: "https://example.com/video"}
	SetFPS(video, 0.25)
	output := &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("video-file"), Extra: map[string]any{videoURLFPS: 0.5}}}
	cases := []struct {
		msg        *schema.Message
		field, key string
		want       any
	}{
		{&schema.Message{Role: schema.User, MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeVideoURL, VideoURL: video}}}, "video_url", "fps", 0.25},
		{&schema.Message{Role: schema.User, MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image", Detail: schema.ImageURLDetailLow}}}}, "image_url", "detail", "low"},
		{&schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeVideoURL, Video: output}}}, "video_url", "file_id", "video-file"},
		{&schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("image-file")}}}}}, "image_url", "file_id", "image-file"},
	}
	for _, tc := range cases {
		msg, err := toArkMessage(tc.msg)
		check(t, err)
		data, err := json.Marshal(&msg)
		check(t, err)
		var body map[string]any
		check(t, json.Unmarshal(data, &body))
		equal(t, tc.want, body["content"].([]any)[0].(map[string]any)[tc.field].(map[string]any)[tc.key])
	}
	equal(t, 0.5, *GetOutputVideoFPS(output))
	equal(t, 0.25, *GetFPS(video))
}

func TestToolChoiceAndBinding(t *testing.T) {
	cm, err := NewChatModel(context.Background(), &ChatModelConfig{APIKey: "test"})
	check(t, err)
	if _, err = cm.WithTools(nil); err == nil {
		t.Fatal("expected empty tools error")
	}
	if err = cm.BindTools([]*schema.ToolInfo{nil}); err == nil {
		t.Fatal("expected nil tool error")
	}
	check(t, cm.BindForcedTools([]*schema.ToolInfo{toolInfo("a"), toolInfo("b")}))
	for _, tc := range []struct {
		choice schema.ToolChoice
		names  []string
		want   string
		fail   bool
	}{
		{schema.ToolChoiceAllowed, nil, `"auto"`, false},
		{schema.ToolChoiceForbidden, nil, `"none"`, false},
		{schema.ToolChoiceForced, nil, `"required"`, false},
		{schema.ToolChoiceForced, []string{"b"}, `{"type":"function","function":{"name":"b"}}`, false},
		{schema.ToolChoiceForced, []string{"a", "b"}, `"required"`, false},
		{schema.ToolChoiceForced, []string{"absent"}, "", true},
	} {
		req := &chat.ChatCompletionRequest{Tools: cm.tools}
		err := populateToolChoice(req, &tc.choice, tc.names)
		if tc.fail {
			if err == nil {
				t.Fatal("expected tool choice error")
			}
			continue
		}
		check(t, err)
		data, err := json.Marshal(&req.ToolChoice)
		check(t, err)
		equal(t, tc.want, string(data))
	}
	req := &chat.ChatCompletionRequest{}
	err = populateToolChoice(req, ptr(schema.ToolChoiceForced), nil)
	if err == nil {
		t.Fatal("expected no tools error")
	}
	for _, config := range []*ChatModelConfig{
		{BatchChat: &BatchChatConfig{EnableBatchChat: true}},
		{BatchChat: &BatchChatConfig{EnableBatchChat: true, BatchChatAsyncRetryTimeout: 1, BatchMaxParallel: ptr(0)}},
	} {
		if _, err = NewChatModel(context.Background(), config); err == nil {
			t.Fatal("expected batch config error")
		}
	}
}

func TestStreamCompatibilityDecoder(t *testing.T) {
	// Omitted usage and nullable optional deltas are accepted; other malformed
	// values must still fail the SDK decoder.
	payload := `{"id":"x","model":"m","object":"chat.completion.chunk","created":1,"choices":[{"index":0,"delta":{"content":null,"reasoning_content":null,"tool_calls":null},"finish_reason":null,"logprobs":null}]}`
	var native chat.ChatCompletionStreamResponse
	if err := json.Unmarshal([]byte(payload), &native); err == nil {
		t.Fatal("SDK behavior changed; reevaluate compatibility decoder")
	}
	check(t, (chatStreamUnmarshaler{}).Unmarshal([]byte(payload), &native))
	equal(t, true, native.Usage.Null)
	if err := (chatStreamUnmarshaler{}).Unmarshal([]byte(`{"choices":"invalid"}`), &native); err == nil {
		t.Fatal("expected malformed stream error")
	}
	if err := (chatStreamUnmarshaler{}).Unmarshal([]byte(`null`), &native); err == nil {
		t.Fatal("expected null stream error")
	}
}

func TestMetadataSurvivesCheckpoint(t *testing.T) {
	var resp chat.ChatCompletionResponse
	check(t, json.Unmarshal([]byte(responseJSON), &resp))
	msg, err := resolveChatResponse(&resp)
	check(t, err)
	var buffer bytes.Buffer
	check(t, gob.NewEncoder(&buffer).Encode(msg))
	var restored schema.Message
	check(t, gob.NewDecoder(&buffer).Decode(&restored))
	equal(t, "req-1", GetArkRequestID(&restored))
	status, ok := GetServiceStatus(&restored)
	equal(t, true, ok)
	equal(t, true, status.ModelFallback.Value.FallbackTriggered)
	usage, ok := GetTokenUsageDetails(&restored)
	equal(t, true, ok)
	equal(t, int32(11), usage.TotalTokens)
	// Legacy metadata uses defined string types; plain JSON uses strings.
	type legacyName string
	name, ok := GetModelName(&schema.Message{Extra: map[string]any{keyModelName: legacyName("legacy")}})
	equal(t, true, ok)
	equal(t, "legacy", name)
}
