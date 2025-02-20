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

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY environment variable is not set")
	}

	// 创建 Claude 模型
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:    apiKey,
		Model:     "deepseek-reasoner",
		MaxTokens: 2000,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\n=== Basic Chat ===")
	basicChat(ctx, cm)

	fmt.Println("\n=== Streaming Chat ===")
	streamingChat(ctx, cm)

	fmt.Println("\n=== Prefix ===")
	prefixChat(ctx, cm)
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

	reasoning, ok := deepseek.GetReasoningContent(resp)
	if !ok {
		fmt.Printf("Unexpected: non-reasoning")
	} else {
		fmt.Printf("Resoning Content: %s\n", reasoning)
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
		if reasoning, ok := deepseek.GetReasoningContent(resp); ok {
			fmt.Printf("Resoning Content: %s\n", reasoning)
		}
		if len(resp.Content) > 0 {
			fmt.Printf("Content: %s\n", resp.Content)
		}
		if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
			fmt.Printf("Tokens used: %d (prompt) + %d (completion) = %d (total)\n",
				resp.ResponseMeta.Usage.PromptTokens,
				resp.ResponseMeta.Usage.CompletionTokens,
				resp.ResponseMeta.Usage.TotalTokens)
		}
	}
}

func prefixChat(ctx context.Context, cm model.ChatModel) {
	messages := []*schema.Message{
		schema.UserMessage("Please write quick sort code"),
		schema.AssistantMessage("```python\n", nil),
	}
	deepseek.SetPrefix(messages[1])

	result, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Printf("Generate error: %v", err)
	}

	reasoningContent, ok := deepseek.GetReasoningContent(result)
	if !ok {
		fmt.Printf("No reasoning content")
	} else {
		fmt.Printf("Reasoning: %v\n", reasoningContent)
	}
	fmt.Printf("Content: %v\n", result)
}
