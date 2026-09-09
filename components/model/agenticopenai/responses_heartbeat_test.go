/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the real SDK decoder and Responses converter together. Mocking
// Stream.Next would hide comment-only blocks being decoded as empty JSON.
// Keep this regression when upgrading openai-go: the adapter must inherit
// the SDK's no-data dispatch fix (openai/openai-go#621).
func TestResponsesStreamIgnoresNoDataBlocks(t *testing.T) {
	for name, keepAlive := range map[string]string{
		"comment":  ": keep-alive\n\n",
		"retry":    "retry: 3000\n\n",
		"crlf":     ": keep-alive\r\n\r\n",
		"repeated": strings.Repeat(": keep-alive\n\n", 1000),
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, keepAlive)
				w.(http.Flusher).Flush()
				_, _ = fmt.Fprint(w, "event: response.created\ndata: ", `{"type":"response.created","response":{"id":"resp_test","status":"in_progress"}}`, "\n\n")
				_, _ = fmt.Fprint(w, "event: response.output_item.added\ndata: ", `{"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_test","role":"assistant","status":"in_progress","content":[]}}`, "\n\n")
				_, _ = fmt.Fprint(w, "event: response.content_part.added\ndata: ", `{"type":"response.content_part.added","item_id":"msg_test","output_index":0,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`, "\n\n")
				_, _ = io.WriteString(w, keepAlive)
				_, _ = fmt.Fprint(w, "event: response.output_text.delta\ndata: ", `{"type":"response.output_text.delta","item_id":"msg_test","output_index":0,"content_index":0,"delta":"ok"}`, "\n\n")
				_, _ = io.WriteString(w, keepAlive)
				_, _ = fmt.Fprint(w, "event: response.completed\ndata: ", `{"type":"response.completed","response":{"id":"resp_test","status":"completed","usage":{"input_tokens":20,"input_tokens_details":{"cached_tokens":5},"output_tokens":1,"total_tokens":21}}}`, "\n\n")
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			m, err := NewResponsesModel(ctx, &ResponsesConfig{APIKey: "test", BaseURL: server.URL, Model: "test", HTTPClient: server.Client()})
			require.NoError(t, err)
			stream, err := m.Stream(ctx, []*schema.AgenticMessage{schema.UserAgenticMessage("test")})
			require.NoError(t, err)
			defer stream.Close()
			var chunks []*schema.AgenticMessage
			for {
				chunk, err := stream.Recv()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				if chunk != nil {
					chunks = append(chunks, chunk)
				}
			}
			msg, err := schema.ConcatAgenticMessages(chunks)
			require.NoError(t, err)
			require.NotNil(t, msg)
			var text strings.Builder
			for _, block := range msg.ContentBlocks {
				if block.AssistantGenText != nil {
					text.WriteString(block.AssistantGenText.Text)
				}
			}
			assert.Equal(t, "ok", text.String())
			require.NotNil(t, msg.ResponseMeta)
			require.NotNil(t, msg.ResponseMeta.TokenUsage)
			assert.Equal(t, 5, msg.ResponseMeta.TokenUsage.PromptTokenDetails.CachedTokens)
			require.NotNil(t, msg.ResponseMeta.OpenAIExtension)
			assert.Equal(t, "resp_test", msg.ResponseMeta.OpenAIExtension.ID)
			assert.Equal(t, "completed", string(msg.ResponseMeta.OpenAIExtension.Status))
		})
	}
}
