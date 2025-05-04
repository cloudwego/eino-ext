/*
 * Copyright 2024 CloudWeGo Authors
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
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/cloudwego/eino-ext/components/model/grok"
)

// Helper function to create pointers
func ptrOf[T any](v T) *T {
	return &v
}

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GROK_API_KEY")
	if apiKey == "" {
		log.Fatal("GROK_API_KEY environment variable is not set")
	}

	// Create a new Grok model
	cm, err := grok.NewChatModel(ctx, &grok.Config{
		APIKey:    apiKey,
		Model:     "grok-3",
		MaxTokens: ptrOf(2000),
	})
	if err != nil {
		log.Fatalf("NewChatModel of grok failed, err=%v", err)
	}

	fmt.Println("\n=== Basic Chat ===")
	basicChat(ctx, cm)

	fmt.Println("\n=== Streaming Chat ===")
	streamingChat(ctx, cm)

	fmt.Println("\n=== Function Calling ===")
	functionCalling(ctx, cm)

	fmt.Println("\n=== Advanced Options ===")
	advancedOptions(ctx, cm)
}

func basicChat(ctx context.Context, cm model.ChatModel) {
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful AI assistant. Be concise in your responses.",
		},
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	}

	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	fmt.Printf("Assistant: %s\n", resp.Content)
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total)\n",
			resp.ResponseMeta.Usage.PromptTokens,
			resp.ResponseMeta.Usage.CompletionTokens,
			resp.ResponseMeta.Usage.TotalTokens)
	}
}

func streamingChat(ctx context.Context, cm model.ChatModel) {
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Write a short poem about spring, word by word.",
		},
	}

	stream, err := cm.Stream(ctx, messages)
	if err != nil {
		log.Printf("Stream error: %v", err)
		return
	}

	fmt.Print("Assistant: ")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			// 正常结束，不需要报错
			break
		}
		if err != nil {
			log.Printf("Stream receive error: %v", err)
			return
		}
		fmt.Print(resp.Content)
	}
	fmt.Println()
}

func functionCalling(ctx context.Context, cm model.ChatModel) {
	// Bind tools to the model
	err := cm.BindTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get current weather information for a city",
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
				Type: "object",
				Properties: map[string]*openapi3.SchemaRef{
					"city": {
						Value: &openapi3.Schema{
							Type:        "string",
							Description: "The city name",
						},
					},
					"unit": {
						Value: &openapi3.Schema{
							Type: "string",
							Enum: []interface{}{"celsius", "fahrenheit"},
						},
					},
				},
				Required: []string{"city"},
			}),
		},
	})
	if err != nil {
		log.Printf("Bind tools error: %v", err)
		return
	}

	// Stream the response with a function call
	streamResp, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What's the weather like in Paris today? Please use Celsius.",
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	msgs := make([]*schema.Message, 0)
	for {
		msg, err := streamResp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Stream receive error: %v", err)
			return
		}
		msgs = append(msgs, msg)
	}
	resp, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Printf("Concat error: %v", err)
		return
	}

	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Function called: %s\n", resp.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.ToolCalls[0].Function.Arguments)

		// Handle the function call with a mock response
		weatherResp, err := cm.Generate(ctx, []*schema.Message{
			{
				Role:    schema.User,
				Content: "What's the weather like in Paris today? Please use Celsius.",
			},
			resp,
			{
				Role:       schema.Tool,
				ToolCallID: resp.ToolCalls[0].ID,
				Content:    `{"temperature": 18, "condition": "sunny"}`,
			},
		})
		if err != nil {
			log.Printf("Generate error: %v", err)
			return
		}
		fmt.Printf("Final response: %s\n", weatherResp.Content)
	} else {
		fmt.Printf("No function was called. Response: %s\n", resp.Content)
	}
}

// Advanced example showing TopK parameter usage
func advancedOptions(ctx context.Context, cm model.ChatModel) {
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "Generate 5 creative business ideas.",
		},
	}

	// Using TopK parameter to control diversity of tokens
	resp, err := cm.Generate(ctx, messages, grok.WithTopK(50))
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	fmt.Printf("Assistant (with TopK=50): %s\n", resp.Content)
}
