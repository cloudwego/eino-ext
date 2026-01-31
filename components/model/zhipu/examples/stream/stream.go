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

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("ZHIPU_API_KEY")
	if apiKey == "" {
		log.Fatal("ZHIPU_API_KEY environment variable not set")
	}

	// Create chat model
	config := &zhipu.ChatModelConfig{
		APIKey: apiKey,
		Model:  "glm-4.7-flash", // Free model
	}

	chatModel, err := zhipu.NewChatModel(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create chat model: %v", err)
	}

	// Prepare messages
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: "请写一首关于人工智能的五言绝句。",
		},
	}

	// Stream response
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to stream: %v", err)
	}

	fmt.Println("Streaming response:")
	fmt.Println("---")

	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}

		// Print content as it arrives
		if msg.Content != "" {
			fmt.Print(msg.Content)
		}

		// Print token usage if available (usually in the last message)
		if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
			usage := msg.ResponseMeta.Usage
			if usage.TotalTokens > 0 {
				fmt.Printf("\n\nToken Usage:\n")
				fmt.Printf("  Prompt tokens: %d\n", usage.PromptTokens)
				fmt.Printf("  Completion tokens: %d\n", usage.CompletionTokens)
				fmt.Printf("  Total tokens: %d\n", usage.TotalTokens)
			}
		}
	}

	fmt.Println("\n---")
}
