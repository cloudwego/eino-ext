/*
 * Copyright 2026 CloudWeGo Authors
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

	"github.com/cloudwego/eino-ext/components/model/atlascloud"
	"github.com/cloudwego/eino/schema"
)

func main() {
	apiKey := os.Getenv("ATLASCLOUD_API_KEY")
	modelName := os.Getenv("ATLASCLOUD_MODEL")
	if modelName == "" {
		modelName = "deepseek-ai/DeepSeek-V3-0324"
	}

	if apiKey == "" {
		log.Fatal("ATLASCLOUD_API_KEY is required")
	}

	ctx := context.Background()
	chatModel, err := atlascloud.NewChatModel(ctx, &atlascloud.ChatModelConfig{
		APIKey: apiKey,
		Model:  modelName,
	})
	if err != nil {
		log.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage("You are a concise assistant."),
		schema.UserMessage("Reply with exactly one short sentence introducing Atlas Cloud in Chinese."),
	})
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	fmt.Println(resp.Content)
}
