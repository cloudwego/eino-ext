/*
 * Copyright 2026 CloudWeGo Authors
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

	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/agentic/gemini"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL") // gemini-2.5-flash-image

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("NewClient of gemini failed, err=%v", err)
	}

	cm, err := agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
		Client: client,
		Model:  modelName,
		ResponseModalities: []agenticgemini.ResponseModality{
			agenticgemini.ResponseModalityText,
			agenticgemini.ResponseModalityImage,
		},
	})
	if err != nil {
		log.Fatalf("NewAgenticModel of gemini failed, err=%v", err)
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
	resp, err := cm.Generate(ctx, []*schema.AgenticMessage{
		schema.UserAgenticMessage("A close-up, photorealistic image of an adorable fluffy ginger and white cat sitting comfortably on a soft wool blanket. The lighting is warm and natural, highlighting the texture of the cat's fur and its bright, curious green eyes. The background is slightly blurred (bokeh effect), showing a cozy living room setting with a bookshelf and a plant.\\\\\\\" }\\\",\\n  \\\"thought\\\": \\\"The user wants an image of a cat. Since they didn't specify a style, I will generate a high-quality, photorealistic image of a cute, fluffy ginger and white cat in a cozy setting, as this is generally universally appealing."),
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	fmt.Printf("\n%s\n", resp.String())

	for i, b := range resp.ContentBlocks {
		if b.Type == schema.ContentBlockTypeAssistantGenImage && b.AssistantGenImage != nil && len(b.AssistantGenImage.Base64Data) > 0 {
			bin, err := base64.StdEncoding.DecodeString(b.AssistantGenImage.Base64Data)
			if err != nil {
				panic(fmt.Errorf("decode base64 error: %w", err))
			}
			filePath := fmt.Sprintf("./examples/image_generate/image_%d.png", i)
			err = os.WriteFile(filePath, bin, os.ModePerm)
			if err != nil {
				panic(fmt.Errorf("write file error: %w", err))
			}
			fmt.Printf("\nimage has been written to %s\n", filePath)
		}
	}
}
