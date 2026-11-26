/*
 * Copyright 2024 CloudWeGo Authors
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

package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
	openai "github.com/meguminnnnnnnnn/go-openai"
	"github.com/stretchr/testify/require"
)

func TestAssistantEmptyTextParts(t *testing.T) {
	empty := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeText}
	signed := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeText, Extra: map[string]any{"gemini_thought_signature": []byte("signature")}}
	text := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeText, Text: "answer"}
	audio := &schema.MessageOutputAudio{}
	setMessageOutputAudioID(audio, "audio-1")
	calls := []schema.ToolCall{{ID: "call-1", Type: "function", Function: schema.FunctionCall{Name: "lookup", Arguments: "{}"}}}
	for _, tc := range []struct {
		name  string
		input *schema.Message
		want  []openai.ChatCompletionMessage
	}{
		{"text and signatures", &schema.Message{Role: schema.Assistant, Content: "answer", AssistantGenMultiContent: []schema.MessageOutputPart{text, signed, empty}}, []openai.ChatCompletionMessage{{Role: "assistant", MultiContent: []openai.ChatMessagePart{{Type: "text", Text: "answer"}}}}},
		{"signature only", &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{signed}}, nil},
		{"unsigned empty", &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{empty}}, nil},
		{"fallback content", &schema.Message{Role: schema.Assistant, Content: "fallback", AssistantGenMultiContent: []schema.MessageOutputPart{signed}}, []openai.ChatCompletionMessage{{Role: "assistant", Content: "fallback"}}},
		{"whitespace", &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{empty, {Type: schema.ChatMessagePartTypeText, Text: " "}}}, []openai.ChatCompletionMessage{{Role: "assistant", MultiContent: []openai.ChatMessagePart{{Type: "text", Text: " "}}}}},
		{"tool calls", &schema.Message{Role: schema.Assistant, ToolCalls: calls, AssistantGenMultiContent: []schema.MessageOutputPart{signed}}, []openai.ChatCompletionMessage{{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "call-1", Type: "function", Function: openai.FunctionCall{Name: "lookup", Arguments: "{}"}}}}}},
		{"reasoning", &schema.Message{Role: schema.Assistant, ReasoningContent: "reason", AssistantGenMultiContent: []schema.MessageOutputPart{signed}}, []openai.ChatCompletionMessage{{Role: "assistant", ReasoningContent: "reason"}}},
		{"audio", &schema.Message{Role: schema.Assistant, Content: "transcript", AssistantGenMultiContent: []schema.MessageOutputPart{empty, {Type: schema.ChatMessagePartTypeAudioURL, Audio: audio}, signed}}, []openai.ChatCompletionMessage{{Role: "assistant", Audio: &openai.Audio{ID: "audio-1"}}}},
		{"empty tool result", &schema.Message{Role: schema.Tool, ToolCallID: "call-1"}, []openai.ChatCompletionMessage{{Role: "tool", ToolCallID: "call-1"}}},
		{"legacy empty assistant", &schema.Message{Role: schema.Assistant}, []openai.ChatCompletionMessage{{Role: "assistant"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := json.Marshal(tc.input)
			require.NoError(t, err)
			input := []*schema.Message{schema.UserMessage("question"), tc.input, schema.UserMessage("continue")}
			c := &Client{config: &Config{Model: "test"}}
			req, cb, _, _, err := c.genRequest(context.Background(), input)
			require.NoError(t, err)
			want := []openai.ChatCompletionMessage{{Role: "user", Content: "question"}}
			want = append(want, tc.want...)
			want = append(want, openai.ChatCompletionMessage{Role: "user", Content: "continue"})
			require.Equal(t, want, req.Messages)
			require.Equal(t, input, cb.Messages)
			after, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.Equal(t, before, after, "source history must retain provider metadata")
		})
	}
}

func TestAssistantEmptyTextPartsWire(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%v", streaming), func(t *testing.T) {
			requests := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				requests <- body
				w.Header().Set("Content-Type", "application/json")
				if streaming {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"done\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				} else {
					_, _ = fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`)
				}
			}))
			defer server.Close()
			c, err := NewClient(context.Background(), &Config{BaseURL: server.URL, APIKey: "test", Model: "test"})
			require.NoError(t, err)
			signed := schema.MessageOutputPart{Type: schema.ChatMessagePartTypeText, Extra: map[string]any{"gemini_thought_signature": []byte("sig")}}
			input := []*schema.Message{
				schema.UserMessage("question"),
				{Role: schema.Assistant, Content: "answer", AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "answer"}, signed}},
				{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{signed}},
				{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "call-1", Function: schema.FunctionCall{Name: "lookup", Arguments: "{}"}}}, AssistantGenMultiContent: []schema.MessageOutputPart{signed}},
				{Role: schema.Tool, ToolCallID: "call-1", Content: "result"},
				schema.UserMessage("continue"),
			}
			before, err := json.Marshal(input)
			require.NoError(t, err)
			if streaming {
				stream, err := c.Stream(context.Background(), input)
				require.NoError(t, err)
				defer stream.Close()
				var content string
				for {
					m, err := stream.Recv()
					if err == io.EOF {
						break
					}
					require.NoError(t, err)
					content += m.Content
				}
				require.Equal(t, "done", content)
			} else {
				m, err := c.Generate(context.Background(), input)
				require.NoError(t, err)
				require.Equal(t, "done", m.Content)
			}
			select {
			case body := <-requests:
				messages := body["messages"].([]any)
				require.Len(t, messages, 5)
				assistant := messages[1].(map[string]any)
				require.Equal(t, []any{map[string]any{"type": "text", "text": "answer"}}, assistant["content"])
				toolCall := messages[2].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
				require.Equal(t, "call-1", toolCall["id"])
				require.Equal(t, "call-1", messages[3].(map[string]any)["tool_call_id"])
			default:
				t.Fatal("no request")
			}
			after, err := json.Marshal(input)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}
