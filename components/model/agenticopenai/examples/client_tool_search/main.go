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
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/responses"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
)

// This example demonstrates client-executed tool search.
// The model emits a tool_search_call, and your code resolves it by
// searching for matching tools and supplying a tool_search_output back in the next turn.
// This requires a two-turn conversation where your code resolves the search between turns.
func main() {
	ctx := context.Background()

	am, err := agenticopenai.New(ctx, &agenticopenai.Config{
		BaseURL: "https://api.openai.com/v1",
		Model:   os.Getenv("OPENAI_MODEL_ID"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Reasoning: &responses.ReasoningParam{
			Effort:  responses.ReasoningEffortLow,
			Summary: responses.ReasoningSummaryDetailed,
		},
	})
	if err != nil {
		log.Fatalf("failed to create agentic model, err=%v", err)
	}

	// Define the tool search tool itself. This is a custom function tool
	// that the model will call to request tool search. The parameters describe
	// what the model should provide when requesting a search.
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

	// === First turn: the model will emit a tool_search_call ===
	firstInput := []*schema.AgenticMessage{
		schema.UserAgenticMessage("I want to check the shipping ETA for order_42"),
	}

	sResp, err := am.Stream(ctx, firstInput, opts...)
	if err != nil {
		log.Fatalf("failed to stream, err: %v", err)
	}

	var msgs []*schema.AgenticMessage
	for {
		msg, err := sResp.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", err)
		}
		msgs = append(msgs, msg)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	// Find the tool search call (it appears as a FunctionToolCall with ToolSearchToolCall extra set)
	var toolSearchCallBlock *schema.ContentBlock
	for _, block := range concatenated.ContentBlocks {
		if block.FunctionToolCall != nil && agenticopenai.GetToolSearchToolCall(block) {
			toolSearchCallBlock = block
			log.Printf("tool_search_call: name=%s, args=%s\n",
				block.FunctionToolCall.Name, block.FunctionToolCall.Arguments)
			break
		}
	}

	if toolSearchCallBlock == nil {
		log.Fatalf("expected a tool search call in the response")
	}

	// === Client resolves the tool search ===
	// In a real application, you would search your tool registry based on the
	// model's search arguments. Here we simulate finding a matching tool.
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

	// Build the tool search result message
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

	// === Second turn: send back the tool search result, model will call the discovered tool ===
	secondInput := append(firstInput, concatenated, toolSearchResultMsg)

	sResp2, err := am.Stream(ctx, secondInput, opts...)
	if err != nil {
		log.Fatalf("failed to stream second turn, err: %v", err)
	}

	var msgs2 []*schema.AgenticMessage
	for {
		msg, err := sResp2.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("failed to receive stream response, err: %v", err)
		}
		msgs2 = append(msgs2, msg)
	}

	concatenated2, err := schema.ConcatAgenticMessages(msgs2)
	if err != nil {
		log.Fatalf("failed to concat agentic messages, err: %v", err)
	}

	// The model should now call the discovered tool (e.g., get_shipping_eta)
	for _, block := range concatenated2.ContentBlocks {
		if block.FunctionToolCall != nil {
			log.Printf("function_call: name=%s, args=%s\n",
				block.FunctionToolCall.Name, block.FunctionToolCall.Arguments)
		}
	}

	meta := concatenated2.ResponseMeta.OpenAIExtension
	log.Printf("request_id: %s\n", meta.ID)

	respBody, _ := sonic.MarshalIndent(concatenated2, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
