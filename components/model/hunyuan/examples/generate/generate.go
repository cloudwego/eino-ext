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

	"github.com/cloudwego/eino-ext/components/model/hunyuan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	secretId := os.Getenv("HUNYUAN_SECRET_ID")
	secretKey := os.Getenv("HUNYUAN_SECRET_KEY")

	if secretId == "" || secretKey == "" {
		log.Fatal("HUNYUAN_SECRET_ID and HUNYUAN_SECRET_KEY environment variables are required")
	}

	cm, err := hunyuan.NewChatModel(ctx, &hunyuan.ChatModelConfig{
		SecretId:  secretId,
		SecretKey: secretKey,
		Model:     "hunyuan-lite", // or "hunyuan-pro", "hunyuan-turbo"
	})
	if err != nil {
		log.Fatal(err)
	}

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一个有用的AI助手，请用中文回答用户的问题。",
		},
		{
			Role:    schema.User,
			Content: "请介绍一下腾讯云Hunyuan大模型的特点和优势。",
		},
	}

	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Printf("generate failed: %v", err)
		return
	}

	fmt.Printf("Assistant: %s\n", resp.Content)

	if resp.ReasoningContent != "" {
		fmt.Printf("\nReasoning: %s\n", resp.ReasoningContent)
	}

	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		usage := resp.ResponseMeta.Usage
		fmt.Printf("\nToken Usage:\n")
		fmt.Printf("  Prompt Tokens: %d\n", usage.PromptTokens)
		fmt.Printf("  Completion Tokens: %d\n", usage.CompletionTokens)
		fmt.Printf("  Total Tokens: %d\n", usage.TotalTokens)
	}
}
