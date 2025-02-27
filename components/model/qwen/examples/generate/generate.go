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
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://help.aliyun.com/zh/model-studio/developer-reference/get-api-key?spm=a2c4g.11186623.help-menu-2400256.d_3_0.1ebc47bb0ClCgF
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	cm, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       "qwen-max",
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of qwen failed, err=%v", err)
	}

	ir, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("你好"),
	})
	if err != nil {
		log.Fatalf("Generate of qwen failed, err=%v", err)
	}

	fmt.Println(ir)
	// assistant: 你好！有什么我可以帮助你的吗？
	// finish_reason: stop
	// usage: &{9 8 17}
}

func of[T any](t T) *T {
	return &t
}
