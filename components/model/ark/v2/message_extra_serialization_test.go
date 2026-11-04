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
	for _, mode := range []string{"chat", "responses", "responses stream"} {
		t.Run(mode, func(t *testing.T) {
			var msg *schema.Message
			var err error
			if mode == "chat" {
				var response map[string]json.RawMessage
				check(t, json.Unmarshal([]byte(responseJSON), &response))
				response["usage"] = json.RawMessage(`{"prompt_tokens":14,"completion_tokens":9,"total_tokens":23,"prompt_tokens_details":{"cached_tokens":2,"text_tokens":3,"image_tokens":4,"provisioned_tokens":5,"audio_tokens":6,"audio_cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":7,"provisioned_tokens":8}}`)
				raw, marshalErr := json.Marshal(response)
				check(t, marshalErr)
				cm := testModel(t, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(raw)
				}, nil)
				msg, err = cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")})
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
			for key, value := range msg.Extra {
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
					if mode == "chat" {
						wantUsage, _ := GetTokenUsageDetails(msg)
						usage, ok := GetTokenUsageDetails(&restored)
						equal(t, true, ok)
						equal(t, wantUsage, usage)
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

func TestLegacyMessageExtraSurvivesSerialization(t *testing.T) {
	var chatResponse chat.ChatCompletionResponse
	check(t, json.Unmarshal([]byte(responseJSON), &chatResponse))
	var response responses.Response
	check(t, json.Unmarshal([]byte(responsesJSON), &response))
	msg := &schema.Message{Extra: map[string]any{
		keyServiceStatus:  serviceStatus(chatResponse.ServiceStatus.Value),
		keyUsageDetails:   tokenUsageDetails(chatResponse.Usage.Value),
		keyResponsesUsage: responsesUsage(response.Usage.Value),
		keyResponsesOutput: responsesOutput{
			response.Output[2],
		},
	}}
	for _, codec := range messagePersistenceCodecs() {
		t.Run(codec.name, func(t *testing.T) {
			raw, err := codec.marshal(msg)
			check(t, err)
			var restored schema.Message
			check(t, codec.unmarshal(raw, &restored))
			equalMetadataJSON(t, msg, &restored)
			_, ok := GetServiceStatus(&restored)
			equal(t, true, ok)
			_, ok = GetTokenUsageDetails(&restored)
			equal(t, true, ok)
			_, ok = GetResponsesTokenUsageDetails(&restored)
			equal(t, true, ok)
			_, ok = GetResponsesOutput(&restored)
			equal(t, true, ok)
		})
	}
}

func TestEmptyLegacyMessageExtraJSON(t *testing.T) {
	msg := &schema.Message{Extra: map[string]any{
		keyServiceStatus: serviceStatus{}, keyUsageDetails: tokenUsageDetails{}, keyResponsesUsage: responsesUsage{},
	}}
	for _, codec := range responsesUsageJSONCodecs {
		t.Run(codec.name, func(t *testing.T) {
			raw, err := codec.marshal(msg)
			check(t, err)
			var restored schema.Message
			check(t, codec.unmarshal(raw, &restored))
			_, ok := GetServiceStatus(&restored)
			equal(t, true, ok)
			_, ok = GetTokenUsageDetails(&restored)
			equal(t, true, ok)
			_, ok = GetResponsesTokenUsageDetails(&restored)
			equal(t, true, ok)
		})
	}
}

func TestLegacyReasoningSurvivesSerialization(t *testing.T) {
	var response responses.Response
	check(t, json.Unmarshal([]byte(responsesJSON), &response))
	reasoning := response.Output[0].OneOf.ItemReasoning
	msg := &schema.Message{Role: schema.Assistant, ReasoningContent: responsesReasoningText([]responses.ItemReasoning{reasoning}), Extra: map[string]any{
		keyResponsesReasoning: responsesReasoning{reasoning},
	}}
	for _, codec := range messagePersistenceCodecs() {
		t.Run(codec.name, func(t *testing.T) {
			raw, err := codec.marshal(msg)
			check(t, err)
			var restored schema.Message
			check(t, codec.unmarshal(raw, &restored))
			items, ok := getResponsesReasoning(&restored)
			equal(t, true, ok)
			equal(t, "cipher", items[0].EncryptedContent.Or(""))
			equal(t, msg.ReasoningContent, responsesReasoningText(items))
		})
	}
}

func TestAtomicMetadataConcatUsesLastUsage(t *testing.T) {
	for _, api := range []string{"chat", "responses"} {
		t.Run(api, func(t *testing.T) {
			first := &schema.Message{Extra: map[string]any{}}
			last := &schema.Message{Extra: map[string]any{}}
			var chatUsage chat.CompletionUsage
			var responseUsage responses.ResponseUsage
			if api == "chat" {
				var response chat.ChatCompletionResponse
				check(t, json.Unmarshal([]byte(responseJSON), &response))
				setChatUsage(first, response.Usage.Value)
				setChatUsage(last, chatUsage)
				setServiceStatus(first, response.ServiceStatus.Value)
				setServiceStatus(last, chat.ChatCompletionServiceStatus{})
			} else {
				var usage responses.ResponseUsage
				check(t, json.Unmarshal([]byte(detailedResponsesUsageJSON), &usage))
				check(t, json.Unmarshal([]byte(sparseResponsesUsageJSON), &responseUsage))
				check(t, setResponsesUsage(first, usage))
				check(t, setResponsesUsage(last, responseUsage))
			}
			joined, err := schema.ConcatMessages([]*schema.Message{first, last})
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
