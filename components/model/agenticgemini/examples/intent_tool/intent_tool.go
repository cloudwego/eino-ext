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
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/components/model"
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

	var cm model.AgenticModel
	cm, err = agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
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

	stream, err := cm.Stream(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("Recommend business books with minimum 4.3 rating and max 350 pages"),
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}

	var messages []*schema.AgenticMessage
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Recv error: %v", err)
		}
		messages = append(messages, msg)
		fmt.Printf("frame: %s\n", msg.String())
	}
	resp, err := schema.ConcatAgenticMessages(messages)
	if err != nil {
		log.Fatalf("Concat error: %v", err)
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

	stream, err = cm.Stream(ctx, []*schema.AgenticMessage{
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
	messages = []*schema.AgenticMessage{}
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Recv error: %v", err)
		}
		messages = append(messages, msg)
		fmt.Printf("frame: %s\n", msg.String())
	}
	resp, err = schema.ConcatAgenticMessages(messages)
	if err != nil {
		log.Fatalf("Concat error: %v", err)
	}

	fmt.Printf("second response:\n%s\n", resp.String())
}
