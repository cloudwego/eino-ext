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

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		// if you want to use Azure OpenAI Service, set these two field.
		// BaseURL: "https://{RESOURCE_NAME}.openai.azure.com",
		// ByAzure: true,
		// APIVersion: "2024-06-01",
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		ByAzure: func() bool {
			if os.Getenv("OPENAI_BY_AZURE") == "true" {
				return true
			}
			return false
		}(),
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	// single text modality processing
	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "as a machine, how do you answer user's question?",
		},
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}
	fmt.Printf("output: \n%v", resp)

	/***
	Multi-modal input processing

	resp, err = chatModel.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "analyze the content of the image below for me"},
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{URL: of("https://img0.baidu.com/it/u=4078387433,1356951957&fm=253&fmt=auto&app=138&f=JPEG?w=800&h=1034")},
				}},
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}


	For multi-modal output, such as when the openai gpt-4o-audio-preview model supports speech generation, you need to set Modalities and Audio when initializing the chat model:

	chatModel, err = openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		ByAzure: func() bool {
			if os.Getenv("OPENAI_BY_AZURE") == "true" {
				return true
			}
			return false
		}(),

		Modalities: []openai.Modality{openai.TextModality, openai.AudioModality},
		Audio: &openai.Audio{
			Format: openai.AudioFormatFlac,
			Voice:  openai.AudioVoiceAlloy,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	resp, err = chatModel.Generate(ctx, []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeText, Text: "help me convert the following text to speech"},
				{Type: schema.ChatMessagePartTypeText, Text: "Hello, what can I help you with?"},
			},
		},
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	// resp.AssistantGenMultiContent stores the multi-modal return results
	for _, content := range resp.AssistantGenMultiContent {
		fmt.Printf("gen multi content: \n%v", content)
	}
	***/

}

func of[T any](a T) *T {
	return &a
}
