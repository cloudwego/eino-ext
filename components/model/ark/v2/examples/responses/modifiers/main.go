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
	"log"

	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	config.RequestPayloadModifier = func(_ context.Context, _ []*schema.Message, raw []byte) ([]byte, error) {
		var payload map[string]json.RawMessage
		if err := sonic.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		payload["max_output_tokens"] = json.RawMessage("256")
		return sonic.Marshal(payload)
	}
	config.ResponseMessageModifier = func(_ context.Context, message *schema.Message, raw []byte) (*schema.Message, error) {
		if message.Extra == nil {
			message.Extra = make(map[string]any)
		}
		message.Extra["example-raw-bytes"] = len(raw)
		return message, nil
	}
	config.ResponseChunkMessageModifier = func(_ context.Context, message *schema.Message, _ []byte, end bool) (*schema.Message, error) {
		if end {
			return &schema.Message{Extra: map[string]any{"example-complete": true}}, nil
		}
		return message, nil
	}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "Say hello in one sentence."
	}
	messages := []*schema.Message{schema.UserMessage(prompt)}
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
	return nil
}
