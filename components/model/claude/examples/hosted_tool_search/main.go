/*
 * Copyright 2025 CloudWeGo Authors
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
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/cloudwego/eino-ext/components/model/claude"
)

// This example demonstrates hosted (server-side) tool search with Claude.
// When using hosted tool search, tools are registered with defer_loading=true,
// and the model's built-in tool search (BM25 or Regex) discovers and loads them on demand.
// The search and resolution happen automatically server-side in one API call.
func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	modelName := os.Getenv("CLAUDE_MODEL")
	baseURL := os.Getenv("CLAUDE_BASE_URL")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	var baseURLPtr *string
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	// Create a Claude model with BM25 tool search algorithm (default when deferred tools are used).
	// You can also set ToolSearchAlgorithm to claude.ToolSearchAlgorithmRegex for regex-based search.
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		APIKey:    apiKey,
		BaseURL:   baseURLPtr,
		Model:     modelName,
		MaxTokens: 4096,
		// ToolSearchAlgorithm: claude.ToolSearchAlgorithmRegex, // optional: use regex instead of bm25
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	// Define deferred tools. These tools are registered with defer_loading=true,
	// meaning their full schemas are sent to the server but only loaded into context
	// when the model's built-in tool search finds them relevant.
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
		{
			Name: "cancel_order",
			Desc: "Cancel a specific order",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](
					orderedmap.WithInitialData(
						orderedmap.Pair[string, *jsonschema.Schema]{
							Key: "order_id",
							Value: &jsonschema.Schema{
								Type:        "string",
								Description: "the order id to cancel",
							},
						},
					),
				),
				Required: []string{"order_id"},
			}),
		},
	}

	// Use model.WithDeferredTools to pass deferred tools.
	// The built-in tool search tool (BM25 by default) is automatically added.
	opts := []model.Option{
		model.WithDeferredTools(deferredTools),
	}

	messages := []*schema.Message{
		schema.SystemMessage("You are a helpful customer service assistant."),
		{
			Role:    schema.User,
			Content: "Can you check the open orders for customer_42?",
		},
	}

	// Stream the response. The server-side tool search will automatically
	// discover relevant tools from the deferred tool list.
	stream, err := cm.Stream(ctx, messages, opts...)
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	fmt.Println("Assistant: ----------")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Stream receive error: %v", err)
		}

		// Print text content
		if len(resp.Content) > 0 {
			fmt.Print(resp.Content)
		}

		// Print tool calls (the model will call discovered deferred tools)
		for _, tc := range resp.ToolCalls {
			fmt.Printf("\n[Tool Call] %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
		}
	}
	fmt.Println("\n----------")
}
