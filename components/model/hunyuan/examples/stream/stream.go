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
		Model:     "hunyuan-lite",
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
			Content: "请详细介绍一下人工智能的发展历程。",
		},
	}

	stream, err := cm.Stream(ctx, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("start streaming response:")
	fmt.Println("========================================")

	var fullContent string
	var tokenCount int

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Printf("stream error: %v", err)
			break
		}

		if chunk != nil {
			content := chunk.Content
			if content != "" {
				fmt.Print(content)
				fullContent += content
				tokenCount += len(content)
			}
		}
	}

	fmt.Println("\n========================================")
	fmt.Printf("\ntoken count: %d\n", tokenCount)
	fmt.Printf("content length: %d\n", len(fullContent))
}
