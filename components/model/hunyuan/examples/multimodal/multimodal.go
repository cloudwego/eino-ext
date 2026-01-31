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
		Model:     "hunyuan-vision",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 示例1: 图像描述
	fmt.Println("=== example 1: describe image ===")
	describeImageExample(ctx, cm)

	// 示例2: 图文问答
	fmt.Println("\n=== example 2: image question ===")
	imageQuestionExample(ctx, cm)
}

func describeImageExample(ctx context.Context, cm *hunyuan.ChatModel) {
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一个视觉AI助手，请详细描述用户提供的图像内容。",
		},
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "请描述这张图片的内容：",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							MIMEType:   "image/jpeg",
							Base64Data: toPtr("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="), // 示例base64图片数据
						},
					},
				},
			},
		},
	}

	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Printf("describe image failed: %v", err)
		return
	}

	fmt.Printf("image description result: %s\n", resp.Content)
}

func imageQuestionExample(ctx context.Context, cm *hunyuan.ChatModel) {
	// 构建图文问答消息
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一个视觉AI助手，请根据用户提供的图像回答问题。",
		},
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							MIMEType:   "image/jpeg",
							Base64Data: toPtr("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="),
						},
					},
				},
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "这张图片中有什么物体？请列出并描述它们。",
				},
			},
		},
	}

	resp, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Printf("image question failed: %v", err)
		return
	}

	fmt.Printf("image question result: %s\n", resp.Content)
}

func toPtr[T any](v T) *T {
	return &v
}
