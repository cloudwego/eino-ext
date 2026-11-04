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
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func rejectCachedTools(w http.ResponseWriter, body map[string]any) bool {
	if body["previous_response_id"] != nil {
		if _, present := body["tools"]; present {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"code":"InvalidParameter","message":"can not set tools when previous response cached","type":"BadRequest"}}`)
			return true
		}
	}
	return false
}

func callResponsesWithOptions(t *testing.T, cm *ResponsesAPIChatModel, mode string, history []*schema.Message, opts ...model.Option) *schema.Message {
	t.Helper()
	if mode == "stream" {
		reader, err := cm.Stream(context.Background(), history, opts...)
		check(t, err)
		msg, err := schema.ConcatMessageStream(reader)
		check(t, err)
		return msg
	}
	msg, err := cm.Generate(context.Background(), history, opts...)
	check(t, err)
	return msg
}

func TestResponsesCachedRequestOmitsTools(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		for _, kind := range []string{"no tools", "bound", "strict", "call tools", "web search", "modifier"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				config := &ResponsesAPIConfig{}
				if kind == "strict" || kind == "modifier" {
					config.ToolStrict = ptr(true)
				}
				if kind == "web search" {
					config.EnableToolWebSearch = &responses.WebSearchTool{}
				}
				requests := make(chan map[string]any, 1)
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					body := decodeRequest(t, r)
					requests <- body
					if rejectCachedTools(w, body) {
						return
					}
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, config)
				if kind != "no tools" && kind != "call tools" && kind != "web search" {
					check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
				}
				opts := []model.Option{WithCache(&CacheOption{HeadPreviousResponseID: ptr("cached-response")})}
				if kind == "call tools" {
					opts = append(opts, model.WithTools([]*schema.ToolInfo{WithToolStrict(toolInfo("inspect"), false)}))
				}
				if kind == "modifier" {
					opts = append(opts, WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) { return raw, nil }))
				}
				reply := callResponsesWithOptions(t, cm, mode, []*schema.Message{schema.UserMessage("second turn")}, opts...)
				equal(t, "continued", reply.Content)
				body := <-requests
				equal(t, "cached-response", body["previous_response_id"])
				if _, present := body["tools"]; present {
					t.Fatal("cached request resent tools")
				}
			})
		}
	}
}

func TestResponsesCachedImageToolContinuation(t *testing.T) {
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
					if calls.Add(1) == 1 {
						w.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(w, `{"id":"image-cache","created_at":1,"object":"response","model":"test","status":"completed","expire_at":4102444800,"output":[{"type":"function_call","id":"fc-image","call_id":"call-image","name":"view_image","arguments":"{}","status":"completed"}]}`)
						return
					}
					writeResponsesHistoryReply(w, false, body["stream"] == true)
				}, &ResponsesAPIConfig{ToolStrict: ptr(true), SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}})
				check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("view_image")}))
				user := schema.UserMessage("view the image")
				first, err := cm.Generate(context.Background(), []*schema.Message{user})
				check(t, err)
				body := <-requests
				if _, present := body["previous_response_id"]; present {
					t.Fatal("first call reused a cache")
				}
				equal(t, true, body["tools"].([]any)[0].(map[string]any)["strict"])
				tool := &schema.Message{Role: schema.Tool, ToolCallID: "call-image", UserInputMultiContent: []schema.MessageInputPart{
					{Type: schema.ChatMessagePartTypeText, Text: "image result"},
					{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("aGVsbG8="), MIMEType: "image/png"}}},
				}}
				history := roundTripHistory(t, codec, []*schema.Message{user, first, tool})
				reply := callResponsesWithOptions(t, cm, mode, history)
				equal(t, "continued", reply.Content)
				body = <-requests
				equal(t, "image-cache", body["previous_response_id"])
				input := body["input"].([]any)
				equal(t, 1, len(input))
				output := input[0].(map[string]any)
				equal(t, "function_call_output", output["type"])
				equal(t, "call-image", output["call_id"])
				equal(t, "data:image/png;base64,aGVsbG8=", output["output"].([]any)[1].(map[string]any)["image_url"])
				check(t, InvalidateMessageCaches(history))
				reply = callResponsesWithOptions(t, cm, mode, roundTripHistory(t, codec, history))
				equal(t, "continued", reply.Content)
				body = <-requests
				if _, present := body["previous_response_id"]; present {
					t.Fatal("invalidated cache was reused")
				}
				equal(t, true, body["tools"].([]any)[0].(map[string]any)["strict"])
				input = body["input"].([]any)
				equal(t, 3, len(input))
				equal(t, "user", input[0].(map[string]any)["role"])
				equal(t, "view_image", input[1].(map[string]any)["name"])
				equal(t, "call-image", input[2].(map[string]any)["call_id"])
				equal(t, "data:image/png;base64,aGVsbG8=", input[2].(map[string]any)["output"].([]any)[1].(map[string]any)["image_url"])
			})
		}
	}
}

func TestResponsesCachedTextConversation(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			requests := make(chan map[string]any, 2)
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				body := decodeRequest(t, r)
				requests <- body
				if rejectCachedTools(w, body) {
					return
				}
				writeResponsesHistoryReply(w, false, body["stream"] == true)
			}, &ResponsesAPIConfig{ToolStrict: ptr(true), SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}})
			check(t, cm.BindTools([]*schema.ToolInfo{toolInfo("inspect")}))
			user := schema.UserMessage("first turn")
			first := callResponsesWithOptions(t, cm, mode, []*schema.Message{user})
			firstRequest := <-requests
			equal(t, true, firstRequest["tools"].([]any)[0].(map[string]any)["strict"])
			id, ok := GetResponseID(first)
			equal(t, true, ok)
			reply := callResponsesWithOptions(t, cm, mode, []*schema.Message{user, first, schema.UserMessage("second turn")})
			equal(t, "continued", reply.Content)
			body := <-requests
			equal(t, id, body["previous_response_id"])
			input := body["input"].([]any)
			equal(t, 1, len(input))
			equal(t, "second turn", input[0].(map[string]any)["content"])
		})
	}
}
