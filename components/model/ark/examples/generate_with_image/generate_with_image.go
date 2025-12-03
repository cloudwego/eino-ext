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
	"encoding/base64"
	"log"
	"os"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	multiModalMsg := schema.UserMessage("")
	image, err := os.ReadFile("./examples/generate_with_image/eino.png")
	if err != nil {
		log.Fatalf("os.ReadFile failed, err=%v\n", err)
	}

	imageStr := base64.StdEncoding.EncodeToString(image)

	multiModalMsg.UserInputMultiContent = []schema.MessageInputPart{
		{
			Type: schema.ChatMessagePartTypeText,
			Text: "What do you see in this image?",
		},
		{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &imageStr,
					MIMEType:   "image/png",
				},
				Detail: schema.ImageURLDetailAuto,
			},
		},
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		multiModalMsg,
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("Ark ChatModel output: \n%v", resp)

	// demonstrate how to use ChatTemplate to generate with image
	imgPlaceholder := "{img}"
	ctx = context.Background()
	chain := compose.NewChain[map[string]any, *schema.Message]()
	_ = chain.AppendChatTemplate(prompt.FromMessages(schema.FString,
		&schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "What do you see in this image?",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &imgPlaceholder,
							MIMEType:   "image/png",
						},
						Detail: schema.ImageURLDetailAuto,
					},
				},
			},
		}))
	_ = chain.AppendChatModel(chatModel)
	r, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("Compile failed, err=%v", err)
	}

	resp, err = r.Invoke(ctx, map[string]any{
		"img": imageStr,
	})
	if err != nil {
		log.Fatalf("Run failed, err=%v", err)
	}

	log.Printf("Ark ChatModel output with ChatTemplate: \n%v", resp)
}
