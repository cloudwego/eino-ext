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
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/minimax"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("MINIMAX_API_KEY")
	modelName := os.Getenv("MINIMAX_MODEL")
	baseURL := os.Getenv("MINIMAX_BASE_URL")
	if apiKey == "" {
		log.Fatal("MINIMAX_API_KEY environment variable is not set")
	}

	if modelName == "" {
		modelName = "MiniMax-M2.7"
	}

	var baseURLPtr *string = nil
	if len(baseURL) > 0 {
		baseURLPtr = &baseURL
	}

	cm, err := minimax.NewChatModel(ctx, &minimax.Config{
		APIKey:    apiKey,
		Model:     modelName,
		BaseURL:   baseURLPtr,
		MaxTokens: 3000,
	})
	if err != nil {
		log.Fatalf("NewChatModel of minimax failed, err=%v", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage("You are a helpful AI assistant. Be concise in your responses."),
		{
			Role:    schema.User,
			Content: "Write a short poem about spring, word by word.",
		},
	}

	stream, err := cm.Stream(ctx, messages, minimax.WithThinking(&minimax.Thinking{
		Enable:       true,
		BudgetTokens: 1024,
	}))
	if err != nil {
		log.Printf("Stream error: %v", err)
		return
	}
	isFirstThinking := false
	isFirstContent := false

	fmt.Print("Assistant: ----------\n")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Stream receive error: %v", err)
			return
		}

		thinkingContent, ok := minimax.GetThinking(resp)
		if ok && thinkingContent != "" {
			if !isFirstThinking {
				isFirstThinking = true
				fmt.Print("\nThinking: ----------\n")
			}
			fmt.Print(thinkingContent)
		}

		if len(resp.Content) > 0 {
			if !isFirstContent {
				isFirstContent = true
				fmt.Print("\nContent: ----------\n")
			}
			fmt.Print(resp.Content)
		}
	}
	fmt.Println("\n----------")
}
