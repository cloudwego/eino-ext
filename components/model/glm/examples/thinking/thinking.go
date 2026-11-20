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

	"github.com/cloudwego/eino-ext/components/model/glm"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://docs.bigmodel.cn/cn/guide/start/model-overview
	apiKey := os.Getenv("ZHIPU_API_KEY")
	// Use a reasoning-capable model, e.g. glm-4.6 or glm-4.5-flash.
	modelName := os.Getenv("MODEL_NAME")
	cm, err := glm.NewChatModel(ctx, &glm.ChatModelConfig{
		APIKey:         apiKey,
		Model:          modelName,
		EnableThinking: of(true),
	})
	if err != nil {
		log.Fatalf("NewChatModel of glm failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("一个数的平方是144，这个数可能是多少？请简要说明。"),
	}, glm.WithThinking(true))
	if err != nil {
		log.Fatalf("Generate of glm failed, err=%v", err)
	}

	fmt.Printf("reasoning:\n%s\n\n", resp.ReasoningContent)
	fmt.Printf("content:\n%s\n", resp.Content)
}

func of[T any](t T) *T {
	return &t
}
