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
	modelName := os.Getenv("GEMINI_MODEL")

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
	})
	if err != nil {
		log.Fatalf("NewChatModel of gemini failed, err=%v", err)
	}

	image, err := os.ReadFile("./examples/generate_with_image/test.jpg")
	if err != nil {
		log.Fatalf("os.ReadFile failed, err=%v\n", err)
	}

	imageStr := base64.StdEncoding.EncodeToString(image)

	resp, err := cm.Generate(ctx, []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				{Type: schema.ContentBlockTypeUserInputText, UserInputText: &schema.UserInputText{Text: "What do you see in this image?"}},
				{Type: schema.ContentBlockTypeUserInputImage, UserInputImage: &schema.UserInputImage{Base64Data: imageStr, MIMEType: "image/jpeg"}},
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate error: %v", err)
	}
	fmt.Printf("Assistant: %s\n", resp.String())
}
