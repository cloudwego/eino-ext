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
	if value, ok := fields["model"]; !ok || sonic.Unmarshal(value, &model) != nil || model == "" {
		return nil, nil, fmt.Errorf("modified request payload must contain a non-empty string model")
	}
	var input []json.RawMessage
	messagesRaw, ok := fields["messages"]
	if !ok || bytes.Equal(bytes.TrimSpace(messagesRaw), []byte("null")) || sonic.Unmarshal(messagesRaw, &input) != nil {
		return nil, nil, fmt.Errorf("modified request payload must contain a messages array")
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
	// including fields unknown to generated models. Start with an empty typed
	// request so deleting an optional field does not restore its configured value.
	wire := &chat.ChatCompletionRequest{Model: model, Stream: chat.NewOptBool(stream)}
	// Decode only callback configuration. The original JSON remains the wire
	// source, including unknown fields and exact numbers.
	for key, target := range map[string]interface{}{
		"max_tokens": &wire.MaxTokens, "max_completion_tokens": &wire.MaxCompletionTokens,
		"temperature": &wire.Temperature, "top_p": &wire.TopP, "stop": &wire.Stop, "thinking": &wire.Thinking,
	} {
		if value, exists := fields[key]; exists && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if err := sonic.Unmarshal(value, target); err != nil {
				return nil, nil, fmt.Errorf("modified %s: %w", key, err)
			}
		}
	}
	return wire, extra, nil
}
