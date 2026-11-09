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
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestChatCacheUsesOnlyChatCompletions(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			requests := make(chan map[string]any, 1)
			cm := testModel(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				equal(t, http.MethodPost, r.Method)
				equal(t, "/api/v3/chat/completions", r.URL.Path)
				requests <- decodeRequest(t, r)
				writeChatHistoryReply(w, true, mode == "stream")
			}, nil)
			// Responses cache metadata must not switch APIs or truncate Chat history.
			history := []*schema.Message{
				schema.UserMessage("original question"),
				{
					Role: schema.Assistant, Content: "previous answer",
					Extra: map[string]any{
						keyResponseID:      metadataString("resp-cached"),
						keyCacheExpiration: metadataInt64(time.Now().Add(time.Hour).Unix()),
					},
				},
				schema.UserMessage("follow-up"),
			}
			reply := callHistoryModel(t, cm, mode, history)
			equal(t, 2, reply.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
			equal(t, int32(1), calls.Load())
			body := <-requests
			messages := body["messages"].([]any)
			equal(t, len(history), len(messages))
			for i, message := range history {
				got := messages[i].(map[string]any)
				equal(t, string(message.Role), got["role"])
				equal(t, message.Content, got["content"])
			}
			for _, key := range []string{"previous_response_id", "caching", "context_id"} {
				if _, exists := body[key]; exists {
					t.Errorf("Chat Completions request contains %s", key)
				}
			}
		})
	}
}
