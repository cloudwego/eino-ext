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
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/zhipu"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://open.bigmodel.cn/usercenter/apikeys
	apiKey := os.Getenv("ZHIPU_API_KEY")
	chatModel, err := zhipu.NewChatModel(ctx, &zhipu.ChatModelConfig{
		BaseURL:     "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       "glm-4.6v", // Use vision model
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.95)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of zhipu failed, err=%v", err)
	}

	image, err := os.ReadFile("./examples/vision/test.jpg")
	if err != nil {
		log.Fatalf("os.ReadFile failed, err=%v\n", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "这张图片里有什么？请详细描述。",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: of(base64.StdEncoding.EncodeToString(image)),
							MIMEType:   "image/jpeg",
						},
						Detail: schema.ImageURLDetailAuto,
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Generate error: %v", err)
		return
	}
	fmt.Printf("Assistant: %s\n", resp.Content)

}

func of[T any](t T) *T {
	return &t
}
