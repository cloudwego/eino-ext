/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
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
	"errors"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// This example demonstrates hosted (server-side) tool search.
// When using hosted tool search, tools are registered with defer_loading=true,
// and the model's built-in tool search discovers and loads them on demand.
// The search and resolution happen automatically server-side in one API call.
func main() {
	ctx := context.Background()

	am, err := agenticclaude.New(ctx, &agenticclaude.Config{
		BaseURL:   os.Getenv("CLAUDE_BASE_URL"),
		Model:     os.Getenv("CLAUDE_MODEL"),
		APIKey:    os.Getenv("CLAUDE_API_KEY"),
		MaxTokens: 4096,
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	deferredTools := []*schema.ToolInfo{
		{
			Name: "list_open_orders",
			Desc: "List all open orders for a customer",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](
					orderedmap.WithInitialData(
						orderedmap.Pair[string, *jsonschema.Schema]{
							Key: "customer_id",
							Value: &jsonschema.Schema{
								Type:        "string",
								Description: "the customer id",
							},
						},
					),
				),
				Required: []string{"customer_id"},
			}),
		},
		{
			Name: "get_order_details",
			Desc: "Get details of a specific order",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](
					orderedmap.WithInitialData(
						orderedmap.Pair[string, *jsonschema.Schema]{
							Key: "order_id",
							Value: &jsonschema.Schema{
								Type:        "string",
								Description: "the order id",
							},
						},
					),
				),
				Required: []string{"order_id"},
			}),
		},
	}

	opts := []model.Option{
		model.WithDeferredTools(deferredTools),
	}

	input := []*schema.AgenticMessage{
		schema.UserAgenticMessage("Can you check the open orders for customer_42?"),
	}

	resp, err := am.Stream(ctx, input, opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	callID := ""
	var msgs []*schema.AgenticMessage
	for {
		msg, recvErr := resp.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", recvErr)
		}
		for _, cb := range msg.ContentBlocks {
			if cb.FunctionToolCall != nil && cb.FunctionToolCall.CallID != "" {
				callID = cb.FunctionToolCall.CallID
			}
		}
		msgs = append(msgs, msg)
	}

	if callID == "" {
		log.Fatalf("expect a function call in response")
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	log.Printf("\n%s\n", concatenated.String())

	resp, err = am.Stream(ctx, append(input, concatenated, &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolResult{
				CallID: callID,
				Name:   "list_open_orders",
				Content: []*schema.FunctionToolResultContentBlock{
					{Type: schema.FunctionToolResultContentBlockTypeText, Text: &schema.UserInputText{Text: "[noodles,soap]"}},
				},
			}),
		},
	}), opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	msgs = []*schema.AgenticMessage{}
	for {
		msg, recvErr := resp.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", recvErr)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err = schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	log.Printf("\n%s\n", concatenated.String())
}
