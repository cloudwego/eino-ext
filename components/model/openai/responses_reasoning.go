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

package openai

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
)

const extraKeyReasoning = "_eino_openai_responses_reasoning"

// Keep only encrypted reasoning items in Extra. A JSON string survives both
// session JSON and checkpoint gob without registering SDK union types.
func saveReasoning(msg *schema.Message, output []responses.ResponseOutputItemUnion) error {
	var items []json.RawMessage
	for _, item := range output {
		if item.Type == "reasoning" && item.AsReasoning().EncryptedContent != "" {
			items = append(items, json.RawMessage(item.RawJSON()))
		}
	}
	if len(items) == 0 {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("encode reasoning: %w", err)
	}
	if msg.Extra == nil {
		msg.Extra = make(map[string]any)
	}
	msg.Extra[extraKeyReasoning] = string(raw)
	return nil
}

func reasoningInput(msg *schema.Message) (responses.ResponseInputParam, error) {
	raw, _ := msg.Extra[extraKeyReasoning].(string)
	if raw == "" {
		return nil, nil
	}
	var reasoning []responses.ResponseInputItemUnion
	if err := json.Unmarshal([]byte(raw), &reasoning); err != nil {
		return nil, fmt.Errorf("decode reasoning: %w", err)
	}
	items := make(responses.ResponseInputParam, 0, len(reasoning))
	for _, item := range reasoning {
		items = append(items, item.ToParam())
	}
	return items, nil
}
