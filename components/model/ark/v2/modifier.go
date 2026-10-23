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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func prepareRequestPayload(ctx context.Context, messages []*schema.Message, req *chat.ChatCompletionRequest, modifier RequestPayloadModifier) (*chat.ChatCompletionRequest, map[string]interface{}, error) {
	if modifier == nil {
		return req, nil, nil
	}
	raw, err := sonic.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request payload: %w", err)
	}
	raw, err = modifier(ctx, messages, raw)
	if err != nil {
		return nil, nil, fmt.Errorf("modify request payload: %w", err)
	}
	var fields map[string]json.RawMessage
	if err = sonic.Unmarshal(raw, &fields); err != nil {
		return nil, nil, fmt.Errorf("decode modified request payload: %w", err)
	}
	if fields == nil {
		return nil, nil, fmt.Errorf("modified request payload must be a JSON object")
	}
	var model string
	if value, ok := fields["model"]; !ok || sonic.Unmarshal(value, &model) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, nil, fmt.Errorf("modified request payload must contain a string model")
	}
	if _, ok := fields["messages"]; !ok {
		return nil, nil, fmt.Errorf("modified request payload must contain messages")
	}
	if value, ok := fields["n"]; ok && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		var n int
		if sonic.Unmarshal(value, &n) != nil || n != 1 {
			return nil, nil, fmt.Errorf("Eino ChatModel supports one completion: n must be omitted or 1")
		}
	}
	stream := req.Stream.Or(false)
	fields["stream"], _ = sonic.Marshal(stream)
	if stream {
		var options map[string]json.RawMessage
		if value, ok := fields["stream_options"]; ok {
			if err = sonic.Unmarshal(value, &options); err != nil {
				return nil, nil, fmt.Errorf("modified stream_options must be an object: %w", err)
			}
		}
		if options == nil {
			options = make(map[string]json.RawMessage)
		}
		options["include_usage"] = json.RawMessage("true")
		fields["stream_options"], _ = sonic.Marshal(options)
	}
	extra := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		extra[key] = value
	}
	// The SDK's public WithExtraBody option sends every modified JSON field,
	// including fields unknown to generated models. Keep only transport routing
	// in the typed request so deleting an optional field does not restore it.
	return &chat.ChatCompletionRequest{Model: model, Stream: chat.NewOptBool(stream)}, extra, nil
}

type responseCaptureKey struct{}
type responseCapture struct {
	rawBody []byte
}

type responseCaptureTransport struct {
	next http.RoundTripper
}

func (t *responseCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	capture, _ := req.Context().Value(responseCaptureKey{}).(*responseCapture)
	if capture == nil || !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return t.next.RoundTrip(req)
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, err
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response payload: %w", err)
	}
	capture.rawBody = raw
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	return resp, nil
}

type streamPayloadDecoder struct {
	rawBody []byte
	capture bool
}

func (d *streamPayloadDecoder) Unmarshal(raw []byte, value interface{}) error {
	if _, ok := value.(*chat.ChatCompletionStreamResponse); ok && d.capture {
		d.rawBody = append([]byte(nil), raw...)
	}
	return (chatStreamUnmarshaler{}).Unmarshal(raw, value)
}
