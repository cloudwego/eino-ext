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
	"encoding/json"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"

	"github.com/cloudwego/eino-ext/components/model/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  modelName,
		ResponseModalities: []gemini.GeminiResponseModality{
			gemini.GeminiResponseModalityText,
			gemini.GeminiResponseModalityImage,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	/*
		The generated multimodal content is stored in the `AssistantGenMultiContent` field.
		For this example, the resulting message will have a structure similar to this:

		resp := &schema.Message{
			Role: schema.Assistant,
			AssistantGenMultiContent: []schema.MessageOutputPart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageOutputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: &base64String, // The base64 encoded image data
							MIMEType:   "image/png",
						},
					},
				},
			},
		}
	*/
	firstUserMessage := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "Generate an image of a cat",
			},
		},
	}

	resp, err := cm.Generate(ctx, []*schema.Message{firstUserMessage})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	log.Printf("\ngenerate output: \n")
	respBody, _ := json.MarshalIndent(resp, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

	secondUserMessage := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "Generate another image based on the previous image, but the cat is wearing a hat",
			},
		},
	}

	resp2, err := cm.Generate(ctx, []*schema.Message{firstUserMessage, resp, secondUserMessage})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	log.Printf("\ngenerate output (round 2): \n")
	respBody2, _ := json.MarshalIndent(resp2, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody2))
}
