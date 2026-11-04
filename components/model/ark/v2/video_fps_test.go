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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestVideoFPSOuterExtraWire(t *testing.T) {
	for _, api := range []string{"chat", "responses"} {
		for _, mode := range []string{"generate", "stream"} {
			for _, role := range []schema.RoleType{schema.User, schema.Tool, schema.Assistant} {
				for _, codec := range messagePersistenceCodecs() {
					for _, location := range []string{"outer", "deprecated only"} {
						t.Run(api+"/"+mode+"/"+string(role)+"/"+codec.name+"/"+location, func(t *testing.T) {
							outer := map[string]any{"application": "preserved"}
							if location == "outer" && role == schema.Assistant {
								outer[keyVideoFPS] = float64(0.25)
							}
							// Conflicting deprecated metadata must never override or
							// supply the value configured on the outer part.
							media := schema.MessagePartCommon{URL: ptr("https://example.com/video.mp4"), Extra: map[string]any{keyVideoFPS: float64(9)}}
							message := &schema.Message{Role: role}
							if role == schema.Assistant {
								message.AssistantGenMultiContent = []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageOutputVideo{MessagePartCommon: media}, Extra: outer}}
							} else {
								part := schema.MessageInputPart{Type: schema.ChatMessagePartTypeVideoURL, Video: &schema.MessageInputVideo{MessagePartCommon: media}, Extra: outer}
								if location == "outer" {
									SetInputVideoFPS(&part, 0.25)
								}
								message.UserInputMultiContent = []schema.MessageInputPart{part}
								if role == schema.Tool {
									message.ToolCallID = "call-1"
								}
							}
							history := roundTripHistory(t, codec, []*schema.Message{message})
							before, err := json.Marshal(history)
							check(t, err)
							var cm model.BaseChatModel
							handler := func(w http.ResponseWriter, r *http.Request) {
								body := decodeRequest(t, r)
								list, field := "messages", "content"
								if api == "responses" {
									list = "input"
									if role == schema.Tool {
										field = "output"
									}
								}
								item := body[list].([]any)[0].(map[string]any)
								video := item[field].([]any)[0].(map[string]any)
								if api == "chat" {
									video = video["video_url"].(map[string]any)
								}
								if location == "outer" {
									equal(t, float64(0.25), video["fps"])
								} else if _, exists := video["fps"]; exists {
									t.Error("deprecated inner FPS was sent")
								}
								if api == "chat" {
									writeChatHistoryReply(w, false, body["stream"] == true)
								} else {
									writeResponsesHistoryReply(w, false, body["stream"] == true)
								}
							}
							if api == "chat" {
								cm = testModel(t, handler, nil)
							} else {
								cm = testResponsesModel(t, handler, nil)
							}
							callHistoryModel(t, cm, mode, history)
							after, err := json.Marshal(history)
							check(t, err)
							equal(t, string(before), string(after))
						})
					}
				}
			}
		}
	}
}
