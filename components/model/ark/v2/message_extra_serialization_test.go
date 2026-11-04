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
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func messagePersistenceCodecs() []metadataTestCodec {
	return append(responsesUsageJSONCodecs, metadataTestCodec{"encoding/json UseNumber", json.Marshal, func(raw []byte, value any) error {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		return decoder.Decode(value)
	}}, metadataTestCodec{"gob", func(value any) ([]byte, error) {
		var buffer bytes.Buffer
		err := gob.NewEncoder(&buffer).Encode(value)
		return buffer.Bytes(), err
	}, func(raw []byte, value any) error {
		return gob.NewDecoder(bytes.NewReader(raw)).Decode(value)
	}})
}

func equalMetadataJSON(t *testing.T, want, got any) {
	t.Helper()
	values := make([]any, 2)
	for i, input := range []any{want, got} {
		raw, err := json.Marshal(input)
		check(t, err)
		check(t, json.Unmarshal(raw, &values[i]))
	}
	equal(t, values[0], values[1])
}

func TestMessageExtraSurvivesSerialization(t *testing.T) {
	for _, mode := range []string{"chat", "chat stream", "responses", "responses stream"} {
		t.Run(mode, func(t *testing.T) {
			var msg *schema.Message
			var err error
			var expectedChatUsage chat.CompletionUsage
			chatAPI := mode == "chat" || mode == "chat stream"
			if chatAPI {
				var response map[string]json.RawMessage
				check(t, json.Unmarshal([]byte(responseJSON), &response))
				response["usage"] = json.RawMessage(`{"prompt_tokens":14,"completion_tokens":9,"total_tokens":23,"prompt_tokens_details":{"cached_tokens":2,"text_tokens":3,"image_tokens":4,"provisioned_tokens":5,"audio_tokens":6,"audio_cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":7,"provisioned_tokens":8}}`)
				check(t, json.Unmarshal(response["usage"], &expectedChatUsage))
				raw, marshalErr := json.Marshal(response)
				check(t, marshalErr)
				cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
					if decodeRequest(t, r)["stream"] == true {
						w.Header().Set("Content-Type", "text/event-stream")
						sseChunk(w, `{"id":"req-1","object":"chat.completion.chunk","created":1,"model":"actual-model","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"th","encrypted_content":"ci"},"finish_reason":null}],"usage":null}`)
						sseChunk(w, `{"id":"req-1","object":"chat.completion.chunk","created":1,"model":"actual-model","choices":[{"index":0,"delta":{"content":"done","reasoning_content":"ink","encrypted_content":"pher"},"finish_reason":"stop"}],"usage":null}`)
						usageChunk, encodeErr := json.Marshal(map[string]any{
							"id": "req-1", "object": "chat.completion.chunk", "created": 1, "model": "actual-model", "choices": []any{},
							"usage": response["usage"], "service_tier": response["service_tier"], "service_status": response["service_status"],
						})
						if encodeErr != nil {
							t.Error(encodeErr)
							return
						}
						sseChunk(w, string(usageChunk))
						sseChunk(w, "[DONE]")
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(raw)
				}, nil)
				if mode == "chat stream" {
					msg = callHistoryModel(t, cm, "stream", []*schema.Message{schema.UserMessage("hello")})
				} else {
					msg, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
				}
			} else {
				var response map[string]json.RawMessage
				check(t, json.Unmarshal([]byte(responsesJSON), &response))
				response["usage"] = json.RawMessage(detailedResponsesUsageJSON)
				var output []json.RawMessage
				check(t, json.Unmarshal(response["output"], &output))
				// Unknown SDK union variants retain their JSON in private fields.
				output = append(output, json.RawMessage(`{"type":"future_output","id":"future-1","payload":{"value":42}}`))
				response["output"], err = json.Marshal(output)
				check(t, err)
				raw, marshalErr := json.Marshal(response)
				check(t, marshalErr)
				cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("X-Request-Id", "request-1")
					if decodeRequest(t, r)["stream"] == true {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, responseEvent("response.reasoning_text.delta", map[string]any{"delta": "raw thought", "output_index": 0, "content_index": 0, "item_id": "rs-1"}))
						_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{"delta": "done", "output_index": 1, "content_index": 0, "item_id": "msg-1"}))
						_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(raw)}))
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(raw)
				}, &ResponsesAPIConfig{SessionCache: &SessionCacheConfig{EnableCache: true, TTL: 3600}})
				if mode == "responses stream" {
					reader, streamErr := cm.Stream(context.Background(), []*schema.Message{schema.UserMessage("hello")})
					check(t, streamErr)
					msg, err = readResponsesStream(t, reader)
				} else {
					msg, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
				}
			}
			check(t, err)
			for _, key := range []string{
				"ark-reasoning-content",
				keyUsageDetails + ".prompt_tokens", keyUsageDetails + ".completion_tokens", keyUsageDetails + ".total_tokens",
				keyUsageDetails + ".prompt_tokens_details.cached_tokens", keyUsageDetails + ".completion_tokens_details.reasoning_tokens",
				keyResponsesUsage + ".input_tokens", keyResponsesUsage + ".output_tokens", keyResponsesUsage + ".total_tokens",
				keyResponsesUsage + ".input_tokens_details.cached_tokens", keyResponsesUsage + ".output_tokens_details.reasoning_tokens",
			} {
				if _, exists := msg.Extra[key]; exists {
					t.Fatalf("common message field duplicated in Extra[%q]", key)
				}
			}
			for key, value := range msg.Extra {
				if strings.HasSuffix(key, ".set") {
					t.Fatalf("optional presence duplicated in Extra[%q]", key)
				}
				switch reflect.TypeOf(value).Kind() {
				case reflect.String, reflect.Bool, reflect.Int64:
				default:
					t.Fatalf("Extra[%q] has non-atomic type %T", key, value)
				}
			}
			for _, codec := range messagePersistenceCodecs() {
				t.Run(codec.name, func(t *testing.T) {
					raw, err := codec.marshal(msg)
					check(t, err)
					var restored schema.Message
					check(t, codec.unmarshal(raw, &restored))
					equalMetadataJSON(t, msg, &restored)
					equal(t, GetArkRequestID(msg), GetArkRequestID(&restored))
					equal(t, msg.ReasoningContent, restored.ReasoningContent)
					if chatAPI {
						usage, ok := GetTokenUsageDetails(&restored)
						equal(t, true, ok)
						equal(t, expectedChatUsage, usage)
						model, ok := GetModelName(&restored)
						equal(t, true, ok)
						equal(t, "actual-model", model)
						tier, ok := GetServiceTier(&restored)
						equal(t, true, ok)
						equal(t, "default", tier)
						cipher, ok := GetEncryptedContent(&restored)
						equal(t, true, ok)
						equal(t, "cipher", cipher)
						wantStatus, _ := GetServiceStatus(msg)
						status, ok := GetServiceStatus(&restored)
						equal(t, true, ok)
						equal(t, wantStatus, status)
					} else {
						wantUsage, _ := GetResponsesTokenUsageDetails(msg)
						usage, ok := GetResponsesTokenUsageDetails(&restored)
						equal(t, true, ok)
						equal(t, wantUsage, usage)
						wantOutput, _ := GetResponsesOutput(msg)
						output, ok := GetResponsesOutput(&restored)
						equal(t, true, ok)
						equalMetadataJSON(t, wantOutput, output)
						wantExpiration, _ := GetCacheExpiration(msg)
						expiration, ok := GetCacheExpiration(&restored)
						equal(t, true, ok)
						equal(t, wantExpiration, expiration)
						wantInput, err := toResponsesInput([]*schema.Message{msg}, true)
						check(t, err)
						input, err := toResponsesInput([]*schema.Message{&restored}, true)
						check(t, err)
						equalMetadataJSON(t, wantInput, input)
					}
				})
			}
		})
	}
}

func TestUsageConcatCombinesCommonCountsAndLastDetails(t *testing.T) {
	for _, api := range []string{"chat", "responses"} {
		t.Run(api, func(t *testing.T) {
			first := &schema.Message{ResponseMeta: &schema.ResponseMeta{}, Extra: map[string]any{}}
			middle := &schema.Message{ResponseMeta: &schema.ResponseMeta{}, Extra: map[string]any{}}
			last := &schema.Message{ResponseMeta: &schema.ResponseMeta{}, Extra: map[string]any{}}
			var chatUsage chat.CompletionUsage
			var responseUsage responses.ResponseUsage
			if api == "chat" {
				var response chat.ChatCompletionResponse
				check(t, json.Unmarshal([]byte(responseJSON), &response))
				setUsage(first, response.Usage.Value)
				var update chat.CompletionUsage
				update.PromptTokensDetails.AudioTokens = chat.NewOptInt32(0)
				setUsage(middle, update)
				setUsage(last, chatUsage)
				chatUsage = response.Usage.Value
				chatUsage.PromptTokensDetails.AudioTokens = chat.NewOptInt32(0)
				setServiceStatus(first, response.ServiceStatus.Value)
				setServiceStatus(last, chat.ChatCompletionServiceStatus{})
			} else {
				var usage responses.ResponseUsage
				check(t, json.Unmarshal([]byte(detailedResponsesUsageJSON), &usage))
				check(t, json.Unmarshal([]byte(detailedResponsesUsageJSON), &responseUsage))
				usage.InputTokensDetails.AudioTokens = responses.NewOptInt64(7)
				var err error
				first, err = responsesMetadata(&responses.Response{Usage: responses.NewOptResponseUsage(usage)}, &responses.ResponsesRequest{})
				check(t, err)
				var update responses.ResponseUsage
				update.InputTokensDetails.AudioTokens = responses.NewOptInt64(0)
				middle, err = responsesMetadata(&responses.Response{Usage: responses.NewOptResponseUsage(update)}, &responses.ResponsesRequest{})
				check(t, err)
				var lower responses.ResponseUsage
				check(t, json.Unmarshal([]byte(`{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}`), &lower))
				last, err = responsesMetadata(&responses.Response{Usage: responses.NewOptResponseUsage(lower)}, &responses.ResponsesRequest{})
				check(t, err)
			}
			// Explicit zero replaces a count; a later omission leaves it available.
			joined, err := schema.ConcatMessages([]*schema.Message{first, middle, last})
			check(t, err)
			for _, codec := range messagePersistenceCodecs() {
				t.Run(codec.name, func(t *testing.T) {
					raw, err := codec.marshal(joined)
					check(t, err)
					var restored schema.Message
					check(t, codec.unmarshal(raw, &restored))
					if api == "chat" {
						usage, ok := GetTokenUsageDetails(&restored)
						equal(t, true, ok)
						equal(t, chatUsage, usage)
						status, ok := GetServiceStatus(&restored)
						equal(t, true, ok)
						equal(t, chat.ChatCompletionServiceStatus{}, status)
					} else {
						usage, ok := GetResponsesTokenUsageDetails(&restored)
						equal(t, true, ok)
						equal(t, responseUsage, usage)
					}
				})
			}
		})
	}
}

func TestUsageDetailsFollowResponseMeta(t *testing.T) {
	var chatResponse chat.ChatCompletionResponse
	check(t, json.Unmarshal([]byte(responseJSON), &chatResponse))
	var response responses.Response
	check(t, json.Unmarshal([]byte(responsesJSON), &response))
	check(t, json.Unmarshal([]byte(detailedResponsesUsageJSON), &response.Usage.Value))
	for _, format := range []string{"current", "common only"} {
		t.Run(format, func(t *testing.T) {
			chatMessage, err := resolveChatResponse(&chatResponse)
			check(t, err)
			responseMessage, err := resolveResponsesResponse(&response, &responses.ResponsesRequest{})
			check(t, err)
			wantChat, wantResponse := chatResponse.Usage.Value, response.Usage.Value
			// Change all common counts, including zero values, as a modifier may do.
			wantChat.PromptTokens, wantChat.CompletionTokens, wantChat.TotalTokens = 0, 20, 20
			wantChat.PromptTokensDetails.CachedTokens, wantChat.CompletionTokensDetails.ReasoningTokens = 0, 10
			wantResponse.InputTokens, wantResponse.OutputTokens, wantResponse.TotalTokens = 0, 20, 20
			wantResponse.InputTokensDetails.CachedTokens, wantResponse.OutputTokensDetails.ReasoningTokens = 0, 10
			if format == "common only" {
				chatMessage.Extra, responseMessage.Extra = nil, nil
				wantChat.PromptTokensDetails.AudioTokens = chat.OptInt32{}
				wantResponse.InputTokensDetails.AudioTokens, wantResponse.InputTokensDetails.AudioCachedTokens = responses.OptInt64{}, responses.OptInt64{}
				wantResponse.ToolUsage, wantResponse.ToolUsageDetails = responses.OptToolUsage{}, responses.OptToolUsageDetails{}
			}
			for _, msg := range []*schema.Message{chatMessage, responseMessage} {
				msg.ResponseMeta.Usage = &schema.TokenUsage{CompletionTokens: 20, TotalTokens: 20,
					CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 10}}
			}
			for _, codec := range messagePersistenceCodecs() {
				t.Run(codec.name, func(t *testing.T) {
					raw, err := codec.marshal(chatMessage)
					check(t, err)
					var restored schema.Message
					check(t, codec.unmarshal(raw, &restored))
					chatUsage, ok := GetTokenUsageDetails(&restored)
					equal(t, true, ok)
					equal(t, wantChat, chatUsage)
					raw, err = codec.marshal(responseMessage)
					check(t, err)
					var restoredResponse schema.Message
					check(t, codec.unmarshal(raw, &restoredResponse))
					responseUsage, ok := GetResponsesTokenUsageDetails(&restoredResponse)
					equal(t, true, ok)
					equal(t, wantResponse, responseUsage)
				})
			}
		})
	}
}
