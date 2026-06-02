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
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func main() {
	ctx := context.Background()

	m, err := agenticopenai.NewChatModel(ctx, &agenticopenai.ChatConfig{
		BaseURL: "https://api.openai.com/v1",
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("failed to create model: %v", err)
	}

	functionTools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "get the weather in a city",
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
				Type: "object",
				Properties: orderedmap.New[string, *jsonschema.Schema](
					orderedmap.WithInitialData(
						orderedmap.Pair[string, *jsonschema.Schema]{
							Key: "city",
							Value: &jsonschema.Schema{
								Type:        "string",
								Description: "the city to get the weather",
							},
						},
					),
				),
				Required: []string{"city"},
			}),
		},
	}

	allowedTools := []*schema.AllowedTool{
		{
			FunctionName: "get_weather",
		},
	}

	opts := []model.Option{
		model.WithAgenticToolChoice(&schema.AgenticToolChoice{
			Type: schema.ToolChoiceForced,
			Forced: &schema.AgenticForcedToolChoice{
				Tools: allowedTools,
			},
		}),
		model.WithTools(functionTools),
	}

	firstInput := []*schema.AgenticMessage{
		schema.UserAgenticMessage("what's the weather like in Beijing today"),
	}

	// Stream with tool calling
	stream, err := m.Stream(ctx, firstInput, opts...)
	if err != nil {
		log.Fatalf("failed to stream: %v", err)
	}

	var msgs []*schema.AgenticMessage
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("failed to receive: %v", err)
		}
		msgs = append(msgs, chunk)
	}

	concatenated, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat messages: %v", err)
	}

	respBody, _ := sonic.MarshalIndent(concatenated, "", "  ")
	log.Printf("first response: %s\n", string(respBody))

	// Build tool result and do second turn
	lastBlock := concatenated.ContentBlocks[len(concatenated.ContentBlocks)-1]
	if lastBlock.Type != schema.ContentBlockTypeFunctionToolCall {
		log.Fatalf("expected function tool call, got: %s", lastBlock.Type)
	}

	toolCall := lastBlock.FunctionToolCall
	toolResultMsg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolResult{
				CallID: toolCall.CallID,
				Name:   toolCall.Name,
				Content: []*schema.FunctionToolResultContentBlock{
					{Type: schema.FunctionToolResultContentBlockTypeText, Text: &schema.UserInputText{Text: "20 degrees, sunny"}},
				},
			}),
		},
	}

	secondInput := append(firstInput, concatenated, toolResultMsg)

	// Stream the second turn
	stream, err = m.Stream(ctx, secondInput)
	if err != nil {
		log.Fatalf("failed to stream second turn: %v", err)
	}

	msgs = nil
	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Fatalf("failed to receive: %v", err)
		}
		msgs = append(msgs, chunk)
	}

	concatenated, err = schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("failed to concat messages: %v", err)
	}

	finalBody, _ := sonic.MarshalIndent(concatenated, "", "  ")
	log.Printf("final response: %s\n", string(finalBody))
}
