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
	"encoding/json"

	"github.com/bytedance/sonic"

	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

// v0.12.0 models optional stream fields as non-nullable Opt values. The API
// also sends null for them, and can omit usage until the last chunk. Normalize
// only those known differences; keep the generated decoder for all other fields.
// This is installed through the SDK's exported stream Unmarshaler hook.
type chatStreamUnmarshaler struct{}

func (chatStreamUnmarshaler) Unmarshal(data []byte, target interface{}) error {
	response, ok := target.(*chat.ChatCompletionStreamResponse)
	if !ok {
		return sonic.Unmarshal(data, target)
	}
	decodeErr := sonic.Unmarshal(data, response)
	if decodeErr == nil {
		return nil
	}
	var body map[string]json.RawMessage
	if err := sonic.Unmarshal(data, &body); err != nil {
		return err
	}
	if body == nil {
		return decodeErr
	}
	if _, ok := body["usage"]; !ok {
		body["usage"] = json.RawMessage("null")
	}
	choices := []map[string]json.RawMessage{}
	if err := sonic.Unmarshal(body["choices"], &choices); err != nil {
		return err
	}
	for _, choice := range choices {
		omitNull(choice, "finish_reason", "logprobs", "moderation_hit_type")
		if raw, ok := choice["delta"]; ok {
			var delta map[string]json.RawMessage
			if err := sonic.Unmarshal(raw, &delta); err != nil {
				return err
			}
			omitNull(delta, "role", "content", "reasoning_content", "encrypted_content", "tool_calls")
			raw, err := sonic.Marshal(delta)
			if err != nil {
				return err
			}
			choice["delta"] = raw
		}
	}
	raw, err := sonic.Marshal(choices)
	if err != nil {
		return err
	}
	body["choices"] = raw
	raw, err = sonic.Marshal(body)
	if err != nil {
		return err
	}
	*response = chat.ChatCompletionStreamResponse{}
	return sonic.Unmarshal(raw, response)
}

func omitNull(fields map[string]json.RawMessage, names ...string) {
	for _, name := range names {
		if bytes.Equal(bytes.TrimSpace(fields[name]), []byte("null")) {
			delete(fields, name)
		}
	}
}
