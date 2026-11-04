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
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func TestResponsesMediaWire(t *testing.T) {
	for _, role := range []schema.RoleType{schema.User, schema.Tool} {
		for _, kind := range []string{"image", "audio", "video", "file"} {
			for _, source := range []string{"url", "base64", "file_id", "all", "base64_and_id"} {
				t.Run(string(role)+"/"+kind+"/"+source, func(t *testing.T) {
					common := schema.MessagePartCommon{MIMEType: map[string]string{"image": "image/png", "audio": "audio/mpeg", "video": "video/mp4", "file": "application/pdf"}[kind]}
					switch source {
					case "url":
						common.URL = ptr("https://example.com/media")
					case "base64":
						common.Base64Data = ptr("aGVsbG8=")
					case "file_id":
						common.FileID = ptr("file-1")
					case "all":
						common.URL, common.Base64Data, common.FileID = ptr("https://example.com/media"), ptr("aGVsbG8="), ptr("file-1")
					case "base64_and_id":
						common.Base64Data, common.FileID = ptr("aGVsbG8="), ptr("file-1")
					}
					part := schema.MessageInputPart{}
					sourceKey := ""
					switch kind {
					case "image":
						part.Type = schema.ChatMessagePartTypeImageURL
						part.Image = &schema.MessageInputImage{MessagePartCommon: common, Detail: schema.ImageURLDetailHigh}
						sourceKey = "image_url"
					case "audio":
						part.Type = schema.ChatMessagePartTypeAudioURL
						part.Audio = &schema.MessageInputAudio{MessagePartCommon: common}
						sourceKey = "audio_url"
					case "video":
						part.Type = schema.ChatMessagePartTypeVideoURL
						part.Video = &schema.MessageInputVideo{MessagePartCommon: common}
						SetInputVideoFPS(part.Video, 0.5)
						sourceKey = "video_url"
					case "file":
						part.Type = schema.ChatMessagePartTypeFileURL
						part.File = &schema.MessageInputFile{MessagePartCommon: common, Name: "report.pdf"}
						sourceKey = "file_url"
					}
					cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
						item := decodeRequest(t, r)["input"].([]any)[0].(map[string]any)
						field := "content"
						if role == schema.Tool {
							field = "output"
							equal(t, "call-1", item["call_id"])
						}
						parts := item[field].([]any)
						media := parts[1].(map[string]any)
						equal(t, "input_"+kind, media["type"])
						want := "https://example.com/media"
						if source == "base64" || source == "base64_and_id" {
							want = "data:" + common.MIMEType + ";base64,aGVsbG8="
							if kind == "file" {
								sourceKey = "file_data"
							}
						}
						if source == "file_id" {
							sourceKey = "file_id"
							want = "file-1"
						}
						equal(t, want, media[sourceKey])
						if kind == "file" {
							if sourceKey == "file_data" {
								equal(t, "report.pdf", media["filename"])
							} else if _, ok := media["filename"]; ok {
								t.Error("filename is only valid with file_data")
							}
						}
						if kind == "video" {
							equal(t, float64(0.5), media["fps"])
						}
						if kind == "image" {
							equal(t, "high", media["detail"])
						}
						writeResponses(w)
					}, nil)
					_, err := cm.Generate(context.Background(), []*schema.Message{{Role: role, ToolCallID: "call-1", UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "describe"}, part}}})
					check(t, err)
				})
			}
		}
	}
}

func TestResponsesInputValidation(t *testing.T) {
	cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) { t.Error("invalid input reached HTTP"); writeResponses(w) }, nil)
	for _, messages := range [][]*schema.Message{
		{nil},
		{{Role: "unknown"}},
		{{Role: schema.Tool, Content: "result"}},
		{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeImageURL}}}},
		{{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: ptr("YQ==")}}}}}},
	} {
		if _, err := cm.Generate(context.Background(), messages); err == nil {
			t.Errorf("expected validation error: %+v", messages)
		}
	}
	_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}, model.WithTools([]*schema.ToolInfo{toolInfo("inspect")}), model.WithToolChoice(schema.ToolChoiceForced, "missing"))
	if err == nil {
		t.Fatal("expected unknown tool error")
	}
}

func TestResponsesPayloadValidation(t *testing.T) {
	cm := testResponsesModel(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("invalid payload reached HTTP")
		writeResponses(w)
	}, nil)
	for _, raw := range []string{`null`, `[]`, `{}`, `{"model":null,"input":[]}`, `{"model":"m","input":null}`, `{"model":"m","input":42}`} {
		_, err := cm.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")}, WithRequestPayloadModifier(func(context.Context, []*schema.Message, []byte) ([]byte, error) { return []byte(raw), nil }))
		if err == nil {
			t.Errorf("expected invalid payload: %s", raw)
		}
	}
}

func TestResponsesMetadataAndCacheValidation(t *testing.T) {
	now := time.Now().Unix()
	msg := &schema.Message{Extra: map[string]any{keyResponseID: metadataString("cached"), keyCacheExpiration: metadataInt64(now + 100)}}
	req := &responses.ResponsesRequest{}
	remaining, err := populateResponsesCache([]*schema.Message{schema.UserMessage("old"), msg, schema.UserMessage("new")}, req, &CacheOption{SessionCache: &SessionCacheConfig{EnableCache: true}, HeadPreviousResponseID: ptr("prefix")})
	check(t, err)
	equal(t, 1, len(remaining))
	equal(t, "cached", req.PreviousResponseID.Or(""))
	_, err = populateResponsesCache([]*schema.Message{msg}, req, &CacheOption{SessionCache: &SessionCacheConfig{EnableCache: true}})
	if err == nil {
		t.Fatal("expected missing incremental input")
	}
	var decoded responses.Response
	check(t, sonic.UnmarshalString(responsesJSON, &decoded))
	out, err := resolveResponsesResponse(&decoded, &responses.ResponsesRequest{})
	check(t, err)
	items, err := toResponsesInput([]*schema.Message{out}, true)
	check(t, err)
	raw, err := sonic.Marshal(items)
	check(t, err)
	if !strings.Contains(string(raw), `"encrypted_content":"cipher"`) {
		t.Fatal("reasoning did not survive passback")
	}
	var restored schema.Message
	var buffer bytes.Buffer
	check(t, gob.NewEncoder(&buffer).Encode(out))
	check(t, gob.NewDecoder(&buffer).Decode(&restored))
	items, err = toResponsesInput([]*schema.Message{&restored}, true)
	check(t, err)
	raw, err = sonic.Marshal(items)
	check(t, err)
	if !strings.Contains(string(raw), `"encrypted_content":"cipher"`) {
		t.Fatal("reasoning did not survive checkpoint")
	}
}

func TestResponsesPrefixCache(t *testing.T) {
	cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeRequest(t, r)
		equal(t, true, body["store"])
		equal(t, true, body["caching"].(map[string]any)["prefix"])
		equal(t, false, body["stream"])
		var response map[string]json.RawMessage
		check(t, sonic.UnmarshalString(responsesJSON, &response))
		response["output"] = json.RawMessage("[]")
		raw, err := sonic.Marshal(response)
		check(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}, nil)
	info, err := cm.CreatePrefixCache(context.Background(), []*schema.Message{schema.SystemMessage("prefix")}, 3600)
	check(t, err)
	equal(t, "resp-1", info.ResponseID)
	equal(t, 11, info.Usage.TotalTokens)
}
