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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino/schema"
)

func runModifiers(ctx context.Context, cm *ark.ResponsesAPIChatModel, messages []*schema.Message) (*schema.Message, error) {
	requestCalls, responseCalls, events, finalized := 0, 0, 0, 0
	request := ark.WithRequestPayloadModifier(func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
		var body map[string]json.RawMessage
		if err := sonic.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		// RawMessage preserves unknown fields and JSON numbers. Set an SDK field
		// through the payload hook; Generate and Stream still control stream.
		body["max_output_tokens"] = json.RawMessage("256")
		requestCalls++
		return sonic.Marshal(body)
	})
	generated, err := cm.Generate(ctx, messages, request, ark.WithResponseMessageModifier(func(_ context.Context, msg *schema.Message, raw []byte) (*schema.Message, error) {
		if len(raw) == 0 {
			return nil, errors.New("missing original response JSON")
		}
		responseCalls++
		if msg.Extra == nil {
			msg.Extra = make(map[string]any)
		}
		msg.Extra["example-raw-bytes"] = len(raw)
		return msg, nil
	}))
	if err != nil {
		return nil, err
	}
	fmt.Printf("Generate: %s (raw bytes: %v)\n", generated.Content, generated.Extra["example-raw-bytes"])
	reader, err := cm.Stream(ctx, messages, request, ark.WithResponseChunkMessageModifier(func(_ context.Context, msg *schema.Message, raw []byte, end bool) (*schema.Message, error) {
		if end {
			finalized++
			return &schema.Message{Extra: map[string]any{"example-complete": true}}, nil
		}
		if len(raw) == 0 {
			return nil, errors.New("missing original SSE event JSON")
		}
		events++
		return msg, nil
	}))
	if err != nil {
		return nil, err
	}
	answer, err := collect(reader, true)
	if err != nil {
		return nil, err
	}
	if requestCalls != 2 || responseCalls != 1 || events == 0 || finalized != 1 || answer.Extra["example-complete"] != true {
		return nil, fmt.Errorf("modifier verification failed: requests=%d responses=%d events=%d finalized=%d", requestCalls, responseCalls, events, finalized)
	}
	fmt.Printf("Verified modifiers: requests=%d responses=%d events=%d finalized=%d\n", requestCalls, responseCalls, events, finalized)
	return answer, nil
}
