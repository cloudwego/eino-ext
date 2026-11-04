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
	"testing"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

const sparseResponsesUsageJSON = `{"input_tokens":8,"output_tokens":3,"total_tokens":11,"input_tokens_details":{"cached_tokens":2},"output_tokens_details":{"reasoning_tokens":1}}`
const detailedResponsesUsageJSON = `{"input_tokens":8,"output_tokens":3,"total_tokens":11,"input_tokens_details":{"cached_tokens":2,"audio_tokens":0,"audio_cached_tokens":1},"output_tokens_details":{"reasoning_tokens":1},"tool_usage":{"web_search":2,"mcp":0},"tool_usage_details":{"web_search":{"calls":2,"future_detail":"kept"}}}`

type metadataTestCodec struct {
	name      string
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
}

var responsesUsageJSONCodecs = []metadataTestCodec{
	{"encoding/json", json.Marshal, json.Unmarshal},
	{"sonic", sonic.Marshal, sonic.Unmarshal},
}

func TestResponsesMessageJSONRoundTrip(t *testing.T) {
	response := `{"id":"resp-json","created_at":1,"object":"response","model":"test","status":"completed","output":[{"type":"message","id":"msg-json","role":"assistant","status":"completed","content":[{"type":"output_text","text":"done","annotations":[]}]}],"usage":` + sparseResponsesUsageJSON + `}`
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			cm := testResponsesModel(t, func(w http.ResponseWriter, r *http.Request) {
				if decodeRequest(t, r)["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = io.WriteString(w, responseEvent("response.output_text.delta", map[string]any{
						"delta": "done", "output_index": 0, "content_index": 0, "item_id": "msg-json",
					}))
					_, _ = io.WriteString(w, responseEvent("response.completed", map[string]any{"response": json.RawMessage(response)}))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, response)
			}, nil)
			input := []*schema.Message{schema.UserMessage("hello")}
			var msg *schema.Message
			var err error
			if mode == "stream" {
				reader, streamErr := cm.Stream(context.Background(), input)
				check(t, streamErr)
				msg, err = readResponsesStream(t, reader)
			} else {
				msg, err = cm.Generate(context.Background(), input)
			}
			check(t, err)
			equal(t, "done", msg.Content)
			wantUsage, ok := GetResponsesTokenUsageDetails(msg)
			if !ok {
				t.Fatal("missing Responses usage")
			}
			for _, codec := range responsesUsageJSONCodecs {
				t.Run(codec.name, func(t *testing.T) {
					raw, err := codec.marshal(msg)
					check(t, err)
					var restored schema.Message
					check(t, codec.unmarshal(raw, &restored))
					equal(t, msg.Content, restored.Content)
					equal(t, msg.ResponseMeta.Usage, restored.ResponseMeta.Usage)
					usage, ok := GetResponsesTokenUsageDetails(&restored)
					equal(t, true, ok)
					equal(t, wantUsage, usage)
				})
			}
		})
	}
}
