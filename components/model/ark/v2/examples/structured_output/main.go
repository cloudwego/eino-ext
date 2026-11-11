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
	"fmt"
	"log"
	"time"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func main() {
	if err := exampleutil.Run(2*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	config.Thinking = &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)}
	config.ResponseFormat = &chat.ChatCompletionResponseFormat{
		Type: chat.NewOptResponseFormatType(chat.ResponseFormatTypeJSONSchema),
		JSONSchema: chat.NewOptChatCompletionResponseFormatJsonSchema(chat.ChatCompletionResponseFormatJsonSchema{
			Name: "trip", Strict: chat.NewOptBool(true),
			Schema: []byte(`{"type":"object","properties":{"city":{"type":"string"},"days":{"type":"integer"}},"required":["city","days"],"additionalProperties":false}`),
		}),
	}
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	reply, err := cm.Generate(ctx, []*schema.Message{{Role: schema.User, Content: "Extract the city and number of days from: I will visit Singapore for 3 days."}})
	if err != nil {
		return err
	}
	if reply.ResponseMeta != nil && reply.ResponseMeta.FinishReason == "length" {
		return fmt.Errorf("structured output was truncated")
	}
	var trip struct {
		City string `json:"city"`
		Days int    `json:"days"`
	}
	if err := sonic.UnmarshalString(reply.Content, &trip); err != nil {
		return fmt.Errorf("decode structured output: %w", err)
	}
	if trip.City == "" || trip.Days <= 0 {
		return fmt.Errorf("invalid trip: %+v", trip)
	}
	fmt.Printf("city=%s days=%d\n", trip.City, trip.Days)
	return nil
}
