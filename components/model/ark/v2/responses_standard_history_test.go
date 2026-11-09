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
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestResponsesStandardAssistantHistory(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  *schema.Message
		want string
	}{
		{
			name: "flat reasoning and tools",
			msg: &schema.Message{Role: schema.Assistant, Content: "answer", ReasoningContent: "thought", ToolCalls: []schema.ToolCall{
				{Index: ptr(8), ID: "first", Function: schema.FunctionCall{Name: "inspect", Arguments: `{"number":9007199254740993}`}},
				{Index: ptr(1), ID: "second", Type: "function", Function: schema.FunctionCall{Name: "finish", Arguments: `{}`}},
			}},
			want: `[{"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"thought"}]},
			 {"type":"message","role":"assistant","content":"answer"},
			 {"type":"function_call","call_id":"first","name":"inspect","arguments":"{\"number\":9007199254740993}"},
			 {"type":"function_call","call_id":"second","name":"finish","arguments":"{}"}]`,
		},
		{
			name: "ordered reasoning parts",
			msg: &schema.Message{Role: schema.Assistant, Content: "duplicate answer", ReasoningContent: "duplicate reasoning", AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "before", StreamingMeta: &schema.MessageStreamingMeta{Index: 9}},
				{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "first thought", Signature: "first signature"}},
				{Type: schema.ChatMessagePartTypeText, Text: "between"},
				{Type: schema.ChatMessagePartTypeReasoning, Reasoning: &schema.MessageOutputReasoning{Text: "second thought", Signature: "second signature"}},
				{Type: schema.ChatMessagePartTypeText, Text: "after", StreamingMeta: &schema.MessageStreamingMeta{Index: 0}},
			}},
			want: `[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"before"}]},
			 {"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"first thought"}],"encrypted_content":"first signature"},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"between"}]},
			 {"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"second thought"}],"encrypted_content":"second signature"},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"after"}]}]`,
		},
		{
			name: "media with separate reasoning",
			msg: &schema.Message{Role: schema.Assistant, ReasoningContent: "media thought", AssistantGenMultiContent: []schema.MessageOutputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "caption"},
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{FileID: ptr("image-1")}}},
				{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageOutputAudio{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("YXVkaW8="), MIMEType: "audio/wav"}}},
				{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: schema.MessagePartCommon{URL: ptr("https://example.com/video")}}},
			}},
			want: `[{"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"media thought"}]},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"caption"},
			 {"type":"input_image","file_id":"image-1"},
			 {"type":"input_audio","audio_url":"data:audio/wav;base64,YXVkaW8="},
			 {"type":"input_video","video_url":"https://example.com/video"}]}]`,
		},
		{
			name: "legacy content",
			msg: &schema.Message{Role: schema.Assistant, Content: "duplicate answer", ReasoningContent: "legacy thought", MultiContent: []schema.ChatMessagePart{
				{Type: schema.ChatMessagePartTypeText, Text: "caption"},
				{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "https://example.com/image", Detail: schema.ImageURLDetailHigh}},
			}},
			want: `[{"type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"legacy thought"}]},
			 {"type":"message","role":"assistant","content":[{"type":"output_text","text":"caption"},{"type":"input_image","image_url":"https://example.com/image","detail":"high"}]}]`,
		},
		{
			name: "whitespace content",
			msg:  &schema.Message{Role: schema.Assistant, Content: " \n"},
			want: `[{"type":"message","role":"assistant","content":" \n"}]`,
		},
	} {
		for _, mode := range []string{"generate", "stream"} {
			for _, codec := range messagePersistenceCodecs() {
				t.Run(tc.name+"/"+mode+"/"+codec.name, func(t *testing.T) {
					messages := roundTripHistory(t, codec, []*schema.Message{tc.msg})
					msg := messages[0]
					// Arbitrary model-specific state must not influence the standard
					// mapping, even if a field resembles a target provider setting.
					extra := map[string]any{"id": "foreign-id", "status": "foreign-status", "signature": "foreign signature", keyVideoFPS: float64(9)}
					msg.Extra = extra
					for i := range msg.AssistantGenMultiContent {
						msg.AssistantGenMultiContent[i].Extra = extra
					}
					for i := range msg.ToolCalls {
						msg.ToolCalls[i].Extra = extra
					}
					before, err := json.Marshal(messages)
					check(t, err)
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						var body struct {
							Input  json.RawMessage `json:"input"`
							Stream bool            `json:"stream"`
						}
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							return
						}
						equalRawJSON(t, []byte(tc.want), body.Input)
						writeResponsesHistoryReply(w, false, body.Stream)
					}, nil)
					callResponsesWithOptions(t, cm, mode, messages)
					after, err := json.Marshal(messages)
					check(t, err)
					equal(t, string(before), string(after))
				})
			}
		}
	}
}

func TestResponsesStandardAssistantInvalidToolType(t *testing.T) {
	cm := testResponsesModel(t, func(http.ResponseWriter, *http.Request) { t.Error("unsupported tool type reached HTTP") }, nil)
	_, err := cm.Generate(context.Background(), []*schema.Message{{Role: schema.Assistant, ToolCalls: []schema.ToolCall{
		{ID: "call", Type: "custom", Function: schema.FunctionCall{Name: "inspect", Arguments: `{}`}},
	}}})
	if err == nil || !strings.Contains(err.Error(), "unsupported tool call type") {
		t.Fatalf("expected unsupported tool type error, got %v", err)
	}
}
