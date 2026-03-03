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
