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
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/cloudwego/eino-ext/components/model/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}
	defer func() {
		err = client.Close()
		if err != nil {
			log.Printf("close client error: %v", err)
		}
	}()

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  "gemini-pro",
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	fmt.Println("\n=== Basic Chat ===")
	basicChat(ctx, cm)

	fmt.Println("\n=== Streaming Chat ===")
	streamingChat(ctx, cm)

	fmt.Println("\n=== Function Calling ===")
	functionCalling(ctx, cm)

	fmt.Println("\n=== Image Processing ===")
	imageProcessing(ctx, client)
}

func basicChat(ctx context.Context, cm model.ChatModel) {
	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What is the capital of France?",
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp.Content)
}

func streamingChat(ctx context.Context, cm model.ChatModel) {
	stream, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Write a short poem about spring.",
		},
	})
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
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(
				&openapi3.Schema{
					Type: "object",
					Properties: map[string]*openapi3.SchemaRef{
						"city": {
							Value: &openapi3.Schema{
								Type:        "string",
								Description: "The city name",
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

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What's the weather like in Paris today?",
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Function called: %s\n", resp.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.ToolCalls[0].Function.Arguments)
	} else {
		log.Printf("Function called without tool calls: %s\n", resp.Content)
	}
}

func imageProcessing(ctx context.Context, client *genai.Client) {
	file, err := client.UploadFileFromPath(ctx, "examples/test.jpg", &genai.UploadFileOptions{
		DisplayName: "test",
		MIMEType:    "image/jpeg",
	})
	if err != nil {
		log.Printf("Upload file error: %v", err)
		return
	}
	defer func() {
		err = client.DeleteFile(ctx, file.Name)
		if err != nil {
			log.Printf("Delete file error: %v", err)
		}
	}()

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  "gemini-1.5-flash",
	})
	if err != nil {
		log.Printf("NewChatModel error: %v", err)
		return
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
						URI:      file.URI,
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
