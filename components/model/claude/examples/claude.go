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
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" {
		log.Fatal("CLAUDE_API_KEY environment variable is not set")
	}

	// 创建 Claude 模型
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		APIKey:    apiKey,
		Model:     "claude-3-opus-20240229",
		MaxTokens: 2000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of claude failed, err=%v", err)
	}

	fmt.Println("\n=== Basic Chat ===")
	basicChat(ctx, cm)

	fmt.Println("\n=== Streaming Chat ===")
	streamingChat(ctx, cm)

	fmt.Println("\n=== Function Calling ===")
	functionCalling(ctx, cm)

	fmt.Println("\n=== Image Processing ===")
	imageProcessing(ctx, cm)
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
			log.Fatalf("Stream receive error: %v", err)
		}
		msgs = append(msgs, msg)
	}
	resp, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("Concat error: %v", err)
	}

	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Function called: %s\n", resp.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.ToolCalls[0].Function.Arguments)

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
	}
}

func imageProcessing(ctx context.Context, cm model.ChatModel) {
	imageBinary, err := os.ReadFile("./examples/test.jpg")
	if err != nil {
		log.Fatalf("read file failed, err=%v", err)
	}
	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "What do you see in this image?",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					ImageURL: &schema.ChatMessageImageURL{
						URL:      "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imageBinary),
						MIMEType: "image/jpeg",
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
}
