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
	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}
	defer func() {
		if err != nil {
			log.Printf("close client error: %v", err)
		}
	}()

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  "gemini-2.5-flash",
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  nil,
		},
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

	fmt.Println("\n=== Image Generation ===")
	generateImage(ctx, client)
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
	if len(resp.ReasoningContent) > 0 {
		fmt.Printf("ReasoningContent: %s\n", resp.ReasoningContent)
	}
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

	fmt.Println("Assistant: ")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Stream receive error: %v", err)
			return
		}

		fmt.Println("frame: ")
		if len(resp.Content) > 0 {
			fmt.Println("content: ", resp.Content)
		}
		if len(resp.ReasoningContent) > 0 {
			fmt.Printf("ReasoningContent: %s\n", resp.ReasoningContent)
		}
	}
	fmt.Println()
}

func functionCalling(ctx context.Context, cm model.ChatModel) {
	err := cm.BindTools([]*schema.ToolInfo{
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
		log.Printf("Bind tools error: %v", err)
		return
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Recommend business books with minimum 4.3 rating and max 350 pages",
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}

	if len(resp.ToolCalls) > 0 {
		fmt.Printf("Function called: \n")
		if len(resp.ReasoningContent) > 0 {
			fmt.Printf("ReasoningContent: %s\n", resp.ReasoningContent)
		}
		fmt.Println("Name: ", resp.ToolCalls[0].Function.Name)
		fmt.Printf("Arguments: %s\n", resp.ToolCalls[0].Function.Arguments)
	} else {
		log.Printf("Function called without tool calls: %s\n", resp.Content)
	}

	resp, err = cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "Recommend business books with minimum 4.3 rating and max 350 pages",
		},
		resp,
		{
			Role:       schema.Tool,
			ToolCallID: resp.ToolCalls[0].ID,
			Content:    "{\"book name\":\"Microeconomics for Managers\"}",
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Function call final result: %s\n", resp.Content)
}

func imageProcessing(ctx context.Context, client *genai.Client) {
	file, err := client.Files.UploadFromPath(ctx, "examples/test.jpg", &genai.UploadFileConfig{
		DisplayName: "test",
		MIMEType:    "image/jpeg",
	})
	if err != nil {
		log.Printf("Upload file error: %v", err)
		return
	}
	defer func() {
		_, err = client.Files.Delete(ctx, file.Name, &genai.DeleteFileConfig{})
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

func generateImage(ctx context.Context, client *genai.Client) {
	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  "gemini-2.5-flash-image-preview",
		ResponseModalities: []gemini.GeminiResponseModality{
			gemini.GeminiResponseModalityText,
			gemini.GeminiResponseModalityImage,
		},
	})
	if err != nil {
		log.Printf("NewChatModel error: %v", err)
		return
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "Generate an image of a cat",
				},
			},
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp)
}
