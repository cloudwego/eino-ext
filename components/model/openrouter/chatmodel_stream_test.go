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

package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

// contentChunk is one SSE frame carrying a content delta, optionally with a
// finish reason on the same choice.
func contentChunk(content string, finishReason any) map[string]any {
	choice := map[string]any{
		"index": 0,
		"delta": map[string]any{"role": "assistant", "content": content},
	}
	if finishReason != nil {
		choice["finish_reason"] = finishReason
	}
	return map[string]any{
		"id": "chatcmpl-1", "object": "chat.completion.chunk", "created": 1, "model": "a-model",
		"choices": []any{choice},
	}
}

// emptyFinalChunk is the ending OpenAI and OpenRouter send: an empty delta with
// the finish reason and the usage.
func emptyFinalChunk() map[string]any {
	return map[string]any{
		"id": "chatcmpl-1", "object": "chat.completion.chunk", "created": 1, "model": "a-model",
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{}, "finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 7, "completion_tokens": 4, "total_tokens": 11},
	}
}

func streamingServer(t *testing.T, chunks []map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("the test server cannot flush, so this would not be a stream")
			return
		}
		for _, chunk := range chunks {
			b, err := json.Marshal(chunk)
			if err != nil {
				t.Errorf("marshalling a chunk: %v", err)
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestStreamEndings covers the endings that leave the stream loop with no last
// empty message, so the response chunk modifier is called with a nil one — which
// ResponseChunkMessageModifier documents for end == true. Before the nil guard in
// buildResponseChunkMessageModifier, the last two cases panicked and the reply was
// dropped after it had already been received in full.
func TestStreamEndings(t *testing.T) {
	tests := []struct {
		name   string
		chunks []map[string]any
	}{
		{
			name:   "empty final chunk",
			chunks: []map[string]any{contentChunk("a", nil), contentChunk("b", nil), emptyFinalChunk()},
		},
		{
			name:   "finish reason on the last content chunk",
			chunks: []map[string]any{contentChunk("a", nil), contentChunk("b", "stop")},
		},
		{
			name:   "no chunk after the last content chunk",
			chunks: []map[string]any{contentChunk("a", nil), contentChunk("b", nil)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := streamingServer(t, tt.chunks)

			ctx := context.Background()
			cm, err := NewChatModel(ctx, &Config{
				APIKey:  "test-api-key",
				Model:   "a-model",
				BaseURL: srv.URL + "/v1",
			})
			assert.NoError(t, err)

			sr, err := cm.Stream(ctx, []*schema.Message{schema.UserMessage("say it")})
			assert.NoError(t, err)
			defer sr.Close()

			var content string
			for {
				msg, recvErr := sr.Recv()
				if recvErr == io.EOF {
					break
				}
				assert.NoError(t, recvErr)
				if recvErr != nil {
					return
				}
				content += msg.Content
			}

			assert.Equal(t, "ab", content)
		})
	}
}
