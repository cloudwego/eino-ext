/*
 * Copyright 2024 CloudWeGo Authors
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
	"log"

	"github.com/cloudwego/eino/schema"
	ollamaapi "github.com/eino-contrib/ollama/api"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {
	ctx := context.Background()

	thinking := ollamaapi.ThinkValue{Value: true}
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL:  "http://localhost:11434",
		Model:    "qwen3:8b",
		Thinking: &thinking,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v\n", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Printf("Generate failed, err=%v\n", err)
		return
	}

	log.Printf("output thinking: \n%v\n", resp.ReasoningContent)
	log.Printf("output content: \n%v\n", resp.Content)
}
