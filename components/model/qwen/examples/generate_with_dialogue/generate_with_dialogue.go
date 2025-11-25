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
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/schema"
	"log"
	"os"
)

func main() {
	ctx := context.Background()
	// get api key: https://help.aliyun.com/zh/model-studio/developer-reference/get-api-key?spm=a2c4g.11186623.help-menu-2400256.d_3_0.1ebc47bb0ClCgF
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qwen failed, err=%v", err)
	}

	// 1. ask a first question and cache the response from the assistant.
	prompt1 := "Please recommend a classic love-themed movie."
	resp1, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage(prompt1),
	})
	if err != nil {
		log.Fatalf("Generate of qwen failed, err=%v", err)
	}

	fmt.Printf("output resp1: \n%v\n", resp1)

	// 2. continue ask the following question with the previous questions and responses.
	prompt2 := "What is the name of the main character in this movie?"
	resp2, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage(prompt1),
		schema.AssistantMessage(resp1.Content, nil),
		schema.UserMessage(prompt2),
	})
	if err != nil {
		log.Fatalf("Generate of qwen failed, err=%v", err)
	}

	fmt.Printf("output resp2: \n%v\n", resp2)
}

func of[T any](t T) *T {
	return &t
}
