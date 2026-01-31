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

	// Create chat model with thinking mode enabled
	temp := float32(0.9)
	config := &zhipu.ChatModelConfig{
		APIKey:      apiKey,
		Model:       "glm-4.7-flash", // Free model with thinking mode support
		Temperature: &temp,
		Thinking: &zhipu.Thinking{
			Type: zhipu.ThinkingEnabled,
		},
	}

	chatModel, err := zhipu.NewChatModel(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create chat model: %v", err)
	}

	// Prepare messages with a complex reasoning question
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful assistant that can solve complex problems.",
		},
		{
			Role:    schema.User,
			Content: "What is the revolution of large language models? Please analyze from multiple perspectives.",
		},
	}

	// Stream response to see both reasoning and final answer
	stream, err := chatModel.Stream(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to stream: %v", err)
	}

	fmt.Println("Thinking Mode Response:")
	fmt.Println("=" + "=" + "=" + "=")

	hasReasoning := false
	for {
		msg, err := stream.Recv()
		if err != nil {
			break
		}

		// Print reasoning content (the model's thought process)
		if msg.ReasoningContent != "" {
			if !hasReasoning {
				fmt.Println("\n[Reasoning Process]")
				fmt.Println("---")
				hasReasoning = true
			}
			fmt.Print(msg.ReasoningContent)
		}

		// Print final answer content
		if msg.Content != "" {
			if hasReasoning {
				fmt.Println("\n---")
				fmt.Println("\n[Final Answer]")
				hasReasoning = false
			}
			fmt.Print(msg.Content)
		}
	}

	fmt.Println("\n" + "=" + "=" + "=" + "=")
}
