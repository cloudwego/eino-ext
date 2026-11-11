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
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func callHistoryModel(t *testing.T, cm model.BaseChatModel, mode string, history []*schema.Message) *schema.Message {
	t.Helper()
	if mode == "stream" {
		reader, err := cm.Stream(context.Background(), history)
		check(t, err)
		msg, err := schema.ConcatMessageStream(reader)
		check(t, err)
		return msg
	}
	msg, err := cm.Generate(context.Background(), history)
	check(t, err)
	return msg
}

func roundTripHistory(t *testing.T, codec metadataTestCodec, history []*schema.Message) []*schema.Message {
	t.Helper()
	raw, err := codec.marshal(history)
	check(t, err)
	var restored []*schema.Message
	check(t, codec.unmarshal(raw, &restored))
	return restored
}

func historyWithToolResult(reply *schema.Message) []*schema.Message {
	return []*schema.Message{
		schema.UserMessage("hello"), reply,
		{Role: schema.Tool, ToolCallID: "call-1", Content: "tool-result"},
		schema.UserMessage("follow-up"),
	}
}

func writeChatHistoryReply(w http.ResponseWriter, first, stream bool) {
	if !stream {
		if first {
			writeResponse(w)
		} else {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"req-2","object":"chat.completion","created":1,"model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"continued"},"finish_reason":"stop","logprobs":null}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`)
		}
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	if first {
		sseChunk(w, `{"id":"req-1","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{"role":"assistant","content":"done","reasoning_content":"think","encrypted_content":"cipher","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"inspect","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":11,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens_details":{"reasoning_tokens":1}}}`)
	} else {
		sseChunk(w, `{"id":"req-2","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{"role":"assistant","content":"continued"},"finish_reason":"stop"}],"usage":null}`)
	}
	sseChunk(w, "[DONE]")
}

func TestChatModelCallsAfterHistorySerialization(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, codec := range messagePersistenceCodecs() {
			t.Run(mode+"/"+codec.name, func(t *testing.T) {
				var calls atomic.Int32
				requests := make(chan map[string]any, 2)
				cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					requests <- body
					writeChatHistoryReply(w, calls.Add(1) == 1, body["stream"] == true)
				}, &ChatModelConfig{Model: "history-model"})
				first := callHistoryModel(t, cm, mode, []*schema.Message{schema.UserMessage("hello")})
				<-requests
				restored := roundTripHistory(t, codec, historyWithToolResult(first))
				reply := callHistoryModel(t, cm, mode, restored)
				equal(t, "continued", reply.Content)
				body := <-requests
				equal(t, "history-model", body["model"])
				messages := body["messages"].([]any)
				equal(t, 4, len(messages))
				assistant := messages[1].(map[string]any)
				equal(t, "assistant", assistant["role"])
				equal(t, "done", assistant["content"])
				equal(t, "think", assistant["reasoning_content"])
				equal(t, "cipher", assistant["encrypted_content"])
				tools := assistant["tool_calls"].([]any)
				equal(t, 1, len(tools))
				tool := tools[0].(map[string]any)
				equal(t, "call-1", tool["id"])
				equal(t, map[string]any{"name": "inspect", "arguments": "{}"}, tool["function"])
				result := messages[2].(map[string]any)
				equal(t, "tool", result["role"])
				equal(t, "call-1", result["tool_call_id"])
				equal(t, "tool-result", result["content"])
				equal(t, "follow-up", messages[3].(map[string]any)["content"])
				equal(t, int32(2), calls.Load())
			})
		}
	}
}

func writeResponsesHistoryReply(w http.ResponseWriter, first, stream bool) {
	response := responsesJSON
	if !first {
		response = `{"id":"resp-2","created_at":1,"object":"response","model":"test","status":"completed","output":[{"type":"message","id":"msg-2","role":"assistant","status":"completed","content":[{"type":"output_text","text":"continued","annotations":[]}]}],"usage":` + sparseResponsesUsageJSON + `}`
	}
	if !stream {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	text := "continued"
	outputIndex, itemID := 0, "msg-2"
	if first {
		outputIndex, itemID = 1, "msg-1"
		text = "done"
		_, _ = io.WriteString(w, responseEvent("response.reasoning_text.delta", map[string]any{"output_index": 0, "content_index": 0, "item_id": "rs-1", "delta": "raw thought"}))
		_, _ = io.WriteString(w, responseEvent("response.output_item.added", map[string]any{
			"output_index": 2, "item": map[string]any{"type": "function_call", "id": "fc-1", "call_id": "call-1", "name": "inspect", "arguments": ""},
		}))
		_, _ = io.WriteString(w, responseEvent("response.function_call_arguments.delta", map[string]any{"output_index": 2, "item_id": "fc-1", "delta": "{}"}))
	}
	_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{"output_index": outputIndex, "content_index": 0, "item_id": itemID, "delta": text}))
	_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(response)}))
}

func TestResponsesCallsAfterHistorySerialization(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, codec := range messagePersistenceCodecs() {
			t.Run(mode+"/"+codec.name, func(t *testing.T) {
				var calls atomic.Int32
				requests := make(chan map[string]any, 3)
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					requests <- body
					if rejectCachedTools(w, body) {
						return
					}
					writeResponsesHistoryReply(w, calls.Add(1) == 1, body["stream"] == true)
				}, &ResponsesAPIConfig{ToolStrict: ptr(true), SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}})
				check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
				first := callHistoryModel(t, cm, mode, []*schema.Message{schema.UserMessage("hello")})
				firstRequest := <-requests
				equal(t, true, firstRequest["tools"].([]any)[0].(map[string]any)["strict"])
				restored := roundTripHistory(t, codec, historyWithToolResult(first))
				reply := callHistoryModel(t, cm, mode, restored)
				equal(t, "continued", reply.Content)
				body := <-requests
				equal(t, "resp-1", body["previous_response_id"])
				input := body["input"].([]any)
				equal(t, 2, len(input))
				result := input[0].(map[string]any)
				equal(t, "function_call_output", result["type"])
				equal(t, "call-1", result["call_id"])
				equal(t, "tool-result", result["output"])
				equal(t, "follow-up", input[1].(map[string]any)["content"])

				// After invalidation and another persistence cycle, send the full
				// conversation with the original encrypted reasoning and tool IDs.
				check(t, InvalidateMessageCaches(restored))
				restored = roundTripHistory(t, codec, restored)
				reply = callHistoryModel(t, cm, mode, restored)
				equal(t, "continued", reply.Content)
				body = <-requests
				if _, exists := body["previous_response_id"]; exists {
					t.Fatal("invalidated cache was reused")
				}
				equal(t, true, body["tools"].([]any)[0].(map[string]any)["strict"])
				input = body["input"].([]any)
				equal(t, 6, len(input))
				reasoning := input[1].(map[string]any)
				equal(t, "reasoning", reasoning["type"])
				equal(t, "rs-1", reasoning["id"])
				equal(t, "cipher", reasoning["encrypted_content"])
				equal(t, "msg-1", input[2].(map[string]any)["id"])
				equal(t, "completed", reasoning["status"])
				if _, exists := input[2].(map[string]any)["status"]; exists {
					t.Fatal("output-only message status was sent as input")
				}
				text := input[2].(map[string]any)["content"].([]any)[0].(map[string]any)
				equal(t, "output_text", text["type"])
				equal(t, "done", text["text"])
				tool := input[3].(map[string]any)
				equal(t, "function_call", tool["type"])
				equal(t, "call-1", tool["call_id"])
				equal(t, "inspect", tool["name"])
				equal(t, "{}", tool["arguments"])
				result = input[4].(map[string]any)
				equal(t, "call-1", result["call_id"])
				equal(t, "tool-result", result["output"])
				equal(t, "follow-up", input[5].(map[string]any)["content"])
				equal(t, int32(3), calls.Load())
			})
		}
	}
}
