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

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {
	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v\n", err)
		return
	}

	multiModalMsg := &schema.Message{
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "this picture is a landscape photo, what's the picture's content",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL: of("https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcT11qEDxU4X_MVKYQVU5qiAVFidA58f8GG0bQ&s"),
					},
					Detail: schema.ImageURLDetailAuto,
				},
			},
		},
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		multiModalMsg,
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)
}

func of[T any](a T) *T {
	return &a
}
