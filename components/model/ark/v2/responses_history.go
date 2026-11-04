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
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Only new inputs need an SDK input view for file:// preprocessing. Native
// output is inserted before the indexed input (or after the last input), without
// converting it to the SDK's less expressive input union.
type responsesInput struct {
	items  []responses.InputItem
	output map[int][]json.RawMessage
}

func (in *responsesInput) MarshalJSON() ([]byte, error) {
	raw := make([]json.RawMessage, 0, len(in.items))
	for i := 0; i <= len(in.items); i++ {
		raw = append(raw, in.output[i]...)
		if i == len(in.items) {
			break
		}
		value, err := sonic.Marshal(&in.items[i])
		if err != nil {
			return nil, err
		}
		raw = append(raw, value)
	}
	return sonic.Marshal(raw)
}

// Save the original output from a JSON response or a terminal SSE event, rather
// than re-encoding generated SDK structs that may discard unknown fields.
func retainResponsesOutput(msg *schema.Message, raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	var envelope struct {
		Output   json.RawMessage `json:"output"`
		Response *struct {
			Output json.RawMessage `json:"output"`
		} `json:"response"`
	}
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode Responses output: %w", err)
	}
	if envelope.Response != nil {
		envelope.Output = envelope.Response.Output
	}
	if len(envelope.Output) > 0 {
		msg.Extra[keyResponsesOutput] = metadataString(envelope.Output)
	}
	return nil
}

func responsesOutputForReplay(msg *schema.Message) ([]json.RawMessage, bool, error) {
	raw, exists := extraString(msg, keyResponsesOutput)
	if !exists {
		return nil, false, nil
	}
	var output []responses.OutputItem
	if err := sonic.UnmarshalString(raw, &output); err != nil {
		return nil, false, fmt.Errorf("decode saved Responses output: %w", err)
	}
	// Common fields are projections for applications. If callers edit them,
	// construct input from those edits instead of silently restoring stale data.
	projection := &schema.Message{}
	projectResponsesOutput(projection, output)
	reasoningMatches := msg.ReasoningContent == projection.ReasoningContent
	if digest, ok := extraString(msg, keyResponsesStreamReasoningDigest); ok {
		// Stream deltas may contain several summary parts or both summary and
		// raw reasoning. Compare against what was streamed, not a different
		// textual projection of the terminal output. The digest avoids storing
		// another copy of the reasoning text in Extra.
		reasoningMatches = digest == fmt.Sprintf("%x", sha256.Sum256([]byte(msg.ReasoningContent)))
	}
	if msg.Content != projection.Content || !reasoningMatches ||
		len(msg.ToolCalls) != len(projection.ToolCalls) || len(msg.AssistantGenMultiContent) > 0 || len(msg.MultiContent) > 0 {
		return nil, false, nil
	}
	for i, call := range msg.ToolCalls {
		expected := projection.ToolCalls[i]
		if call.ID != expected.ID || call.Function != expected.Function {
			return nil, false, nil
		}
	}
	var items []json.RawMessage
	if err := sonic.UnmarshalString(raw, &items); err != nil {
		return nil, false, err
	}
	return items, true, nil
}
