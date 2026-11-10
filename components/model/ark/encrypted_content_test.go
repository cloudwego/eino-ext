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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino/schema"
)

type encryptedContentTransport struct {
	requests []map[string]any
}

func (tr *encryptedContentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	defer req.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return nil, err
	}
	tr.requests = append(tr.requests, body)
	response := `{"id":"fixture","model":"fixture","choices":[{"index":0,"message":{"role":"assistant","content":"","reasoning_content":"summary","encrypted_content":"opaque-marker","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`
	contentType := "application/json"
	if body["stream"] == true {
		contentType = "text/event-stream"
		response = ""
		for _, chunk := range []string{
			`{"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"summary"}}]}`,
			`{"choices":[{"index":0,"delta":{"encrypted_content":"opaque-marker"}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			`[DONE]`,
		} {
			response += "data: " + chunk + "\n\n"
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(response)),
		Request:    req,
	}, nil
}

func TestChatCompletionEncryptedContentRoundTrip(t *testing.T) {
	for _, mode := range []string{"generate", "stream"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			transport := &encryptedContentTransport{}
			cm, err := NewChatModel(ctx, &ChatModelConfig{
				APIKey:     "fixture",
				Model:      "fixture",
				BaseURL:    "https://example.invalid",
				HTTPClient: &http.Client{Transport: transport},
				RetryTimes: ptrOf(0),
			})
			require.NoError(t, err)
			input := []*schema.Message{schema.UserMessage("Use lookup.")}
			var output *schema.Message
			if mode == "stream" {
				sr, streamErr := cm.Stream(ctx, input)
				require.NoError(t, streamErr)
				output, err = schema.ConcatMessageStream(sr)
			} else {
				output, err = cm.Generate(ctx, input)
			}
			require.NoError(t, err)
			assert.Equal(t, "summary", output.ReasoningContent)
			require.Len(t, output.ToolCalls, 1)
			assert.Equal(t, "call_1", output.ToolCalls[0].ID)

			// History persistence must preserve the opaque value too.
			encoded, err := json.Marshal(output)
			require.NoError(t, err)
			assert.Contains(t, string(encoded), "opaque-marker")
			var restored schema.Message
			require.NoError(t, json.Unmarshal(encoded, &restored))
			for _, assistant := range []*schema.Message{output, &restored} {
				_, err = cm.Generate(ctx, append(input, assistant, schema.ToolMessage("ok", "call_1")))
				require.NoError(t, err)
				request := transport.requests[len(transport.requests)-1]
				messages := request["messages"].([]any)
				sent := messages[1].(map[string]any)
				assert.Equal(t, "opaque-marker", sent["encrypted_content"])
				assert.Equal(t, "summary", sent["reasoning_content"])
			}
		})
	}
}
