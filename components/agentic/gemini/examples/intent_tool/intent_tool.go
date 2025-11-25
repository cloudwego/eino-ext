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
	"log"
	"os"

	"github.com/cloudwego/eino/components/agentic"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/agentic/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	var cm agentic.Model
	cm, err = gemini.NewAgenticModel(ctx, &gemini.Config{
		Client: client,
		Model:  modelName,
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}
	cm, err = cm.WithTools([]*schema.ToolInfo{
		{
			Name: "book_recommender",
			Desc: "Recommends books based on user preferences and provides purchase links",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"genre": {
					Type: "string",
					Desc: "Preferred book genre",
					Enum: []string{"fiction", "sci-fi", "mystery", "biography", "business"},
				},
				"max_pages": {
					Type: "integer",
					Desc: "Maximum page length (0 for no limit)",
				},
				"min_rating": {
					Type: "number",
					Desc: "Minimum user rating (0-5 scale)",
				},
			}),
		},
	})
	if err != nil {
		log.Fatalf("Bind tools error: %v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Recommend business books with minimum 4.3 rating and max 350 pages"),
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	fmt.Printf("first response:\n%s\n", resp.String())

	callID := ""
	toolName := ""
	haveToolCall := false
	for _, b := range resp.ContentBlocks {
		if b.Type == schema.ContentBlockTypeFunctionToolCall && b.FunctionToolCall != nil {
			haveToolCall = true
			callID = b.FunctionToolCall.CallID
			toolName = b.FunctionToolCall.Name
			break
		}
	}
	if !haveToolCall {
		log.Fatalf("Tool call not found in response")
	}

	resp, err = cm.Generate(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Recommend business books with minimum 4.3 rating and max 350 pages"),
		resp,
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.FunctionToolResult{
					CallID: callID,
					Name:   toolName,
					Result: "{\"book name\":\"Microeconomics for Managers\"}",
				}),
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	fmt.Printf("second response:\n%s\n", resp.String())
}
