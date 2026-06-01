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

	"github.com/cloudwego/eino-ext/components/model/openrouter"
)

func main() {
	ctx := context.Background()
	cm, err := openrouter.NewChatModel(ctx, &openrouter.Config{
		APIKey:  os.Getenv("API_KEY"),
		Model:   os.Getenv("MODEL"), // model should support image generate
		BaseURL: os.Getenv("BASE_URL"),
		Reasoning: &openrouter.Reasoning{
			Effort: openrouter.EffortOfMedium,
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
	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "Generate an image of a cat",
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	log.Printf("\ngenerate output: \n")
	respBody, _ := json.MarshalIndent(resp, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

}
