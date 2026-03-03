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
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("DEEPSEEK_API_KEY environment variable is not set")
	}
	cm, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:  apiKey,
		Model:   os.Getenv("MODEL_NAME"),
		BaseURL: "https://api.deepseek.com/beta",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

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
			fmt.Printf("Reasoning Content: %s\n", reasoning)
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
