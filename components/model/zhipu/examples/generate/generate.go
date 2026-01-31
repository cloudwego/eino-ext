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
			Role:    schema.System,
			Content: "你是一个有用的AI助手。",
		},
		{
			Role:    schema.User,
			Content: "请用一句话介绍智谱AI的GLM模型。",
		},
	}

	// Generate response
	resp, err := chatModel.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to generate: %v", err)
	}

	fmt.Printf("Response: %s\n", resp.Content)

	// Print token usage if available
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		usage := resp.ResponseMeta.Usage
		fmt.Printf("\nToken Usage:\n")
		fmt.Printf("  Prompt tokens: %d\n", usage.PromptTokens)
		fmt.Printf("  Completion tokens: %d\n", usage.CompletionTokens)
		fmt.Printf("  Total tokens: %d\n", usage.TotalTokens)
	}
}
