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
	modelName := os.Getenv("MODEL_NAME") // e.g. glm-4.6, glm-4.5-flash, glm-4-flash-250414
	chatModel, err := glm.NewChatModel(ctx, &glm.ChatModelConfig{
		APIKey:      apiKey,
		Model:       modelName,
		Temperature: of(float32(0.7)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of glm failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("as a machine, how do you answer user's question?"),
	})
	if err != nil {
		log.Fatalf("Generate of glm failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)
}

func of[T any](t T) *T {
	return &t
}
