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
	"io"
	"log"
	"time"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.Run(2*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	config.RequestPayloadModifier = func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
		// RawMessage preserves unknown fields and large integer precision.
		var payload map[string]json.RawMessage
		if err := sonic.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		payload["user"] = json.RawMessage(`"eino-example"`)
		var stream bool
		if err := sonic.Unmarshal(payload["stream"], &stream); err != nil {
			return nil, err
		}
		if stream {
			// chunk_include_usage is not exposed by the generated SDK struct.
			payload["stream_options"] = json.RawMessage(`{"include_usage":true,"chunk_include_usage":true}`)
		}
		return sonic.Marshal(payload)
	}
	config.ResponseMessageModifier = func(_ context.Context, msg *schema.Message, raw []byte) (*schema.Message, error) {
		if msg.Extra == nil {
			msg.Extra = make(map[string]any)
		}
		msg.Extra["example-raw-bytes"] = len(raw)
		return msg, nil
	}
	config.ResponseChunkMessageModifier = func(_ context.Context, msg *schema.Message, _ []byte, end bool) (*schema.Message, error) {
		if end {
			// This final marker is emitted only after [DONE], never on errors
			// or premature EOF. It carries application metadata, not model text.
			return &schema.Message{Extra: map[string]any{"example-complete": true}}, nil
		}
		return msg, nil
	}
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	messages := []*schema.Message{{Role: schema.User, Content: "Say hello in one sentence."}}
	reply, err := cm.Generate(ctx, messages)
	if err != nil {
		return err
	}
	fmt.Printf("Generate: %s (raw bytes: %v)\n", reply.Content, reply.Extra["example-raw-bytes"])
	reader, err := cm.Stream(ctx, messages)
	if err != nil {
		return err
	}
	defer reader.Close()
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		fmt.Print(chunk.Content)
		if chunk.Extra["example-complete"] == true {
			fmt.Println("\nStream completed")
		}
	}
	// Per-call WithRequestPayloadModifier(nil), WithResponseMessageModifier(nil),
	// or WithResponseChunkMessageModifier(nil) disable configured hooks.
	return nil
}
