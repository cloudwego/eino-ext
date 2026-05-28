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

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// This example demonstrates client-executed tool search.
// The model emits a tool search function call, and your code resolves it by
// searching for matching tools and supplying a tool search result back in the next turn.
// This requires a two-turn conversation where your code resolves the search between turns.
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

	toolSearchTool := &schema.ToolInfo{
		Name: "search_tools",
		Desc: "Search for available tools based on a goal description",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](
				orderedmap.WithInitialData(
					orderedmap.Pair[string, *jsonschema.Schema]{
						Key: "goal",
						Value: &jsonschema.Schema{
							Type:        "string",
							Description: "The goal or intent behind the tool search",
						},
					},
				),
			),
			Required: []string{"goal"},
		}),
	}

	opts := []model.Option{
		model.WithToolSearchTool(toolSearchTool),
	}

	firstInput := []*schema.AgenticMessage{
		schema.UserAgenticMessage("I want to check the shipping ETA for order_42"),
	}

	sResp, err := am.Stream(ctx, firstInput, opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	var msgs []*schema.AgenticMessage
	for {
		msg, recvErr := sResp.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", recvErr)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	var toolSearchCallBlock *schema.ContentBlock
	for _, block := range concatenated.ContentBlocks {
		if block.FunctionToolCall != nil && block.FunctionToolCall.Name == toolSearchTool.Name {
			toolSearchCallBlock = block
			log.Printf("tool_search_call: name=%s, args=%s\n",
				block.FunctionToolCall.Name, block.FunctionToolCall.Arguments)
			break
		}
	}

	if toolSearchCallBlock == nil {
		log.Fatalf("expected a tool search call in the response")
	}

	resolvedTools := []*schema.ToolInfo{
		{
			Name: "get_shipping_eta",
			Desc: "Get the estimated shipping arrival time for an order",
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

	toolSearchResultMsg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.ToolSearchFunctionToolResult{
				CallID: toolSearchCallBlock.FunctionToolCall.CallID,
				Name:   toolSearchCallBlock.FunctionToolCall.Name,
				Result: &schema.ToolSearchResult{
					Tools: resolvedTools,
				},
			}),
		},
	}

	secondInput := append(firstInput, concatenated, toolSearchResultMsg)

	sResp2, err := am.Stream(ctx, secondInput, opts...)
	if err != nil {
		log.Fatalf("failed to stream second turn, err: %v", err)
	}

	var msgs2 []*schema.AgenticMessage
	for {
		msg, recvErr := sResp2.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", recvErr)
		}
		msgs2 = append(msgs2, msg)
	}

	concatenated2, err := schema.ConcatAgenticMessages(msgs2)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	for _, block := range concatenated2.ContentBlocks {
		if block.FunctionToolCall != nil {
			log.Printf("function_call: name=%s, args=%s\n",
				block.FunctionToolCall.Name, block.FunctionToolCall.Arguments)
		}
	}

	if meta := concatenated2.ResponseMeta.ClaudeExtension; meta != nil {
		log.Printf("request_id: %s\n", meta.ID)
	}

	respBody, _ := sonic.MarshalIndent(concatenated2, "  ", "  ")
	log.Printf("body: %s\n", string(respBody))
}
