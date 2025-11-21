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
	"fmt"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"log"
	"os"
)

func main() {
	ctx := context.Background()
	// get api key: https://help.aliyun.com/zh/model-studio/developer-reference/get-api-key?spm=a2c4g.11186623.help-menu-2400256.d_3_0.1ebc47bb0ClCgF
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})

	if err != nil {
		log.Fatalf("NewChatModel of qwen failed, err=%v", err)
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		// give a user message
		schema.UserMessage("Please write a short science fiction story."),
		// give a prefix of the story
		schema.AssistantMessage("In the year 2347 AD, Earth was no longer the only home for humanity.", nil),
		// make the last message attribute partial=true
	}, withLastPartialMessageOption())
	if err != nil {
		log.Fatalf("Generate of qwen failed, err=%v", err)
	}

	fmt.Printf("output: \n%v", resp)
}

func withLastPartialMessageOption() model.Option {
	return openai.WithRequestBodyModifier(func(rawBody []byte) ([]byte, error) {
		var data map[string]interface{}
		err := json.Unmarshal(rawBody, &data)
		if err != nil {
			return nil, err
		}

		messages, ok := data[keyMessages].([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected messages array, got=%v", messages)
		}
		modifiedMessages := make([]interface{}, 0, len(messages))
		for i, msg := range messages {
			if i == len(messages)-1 {
				lastMsg, ok := msg.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("expected messages array, got=%v", messages)
				}
				lastMsg[keyPartial] = true
				modifiedMessages = append(modifiedMessages, lastMsg)
			} else {
				modifiedMessages = append(modifiedMessages, msg)
			}
		}
		data[keyMessages] = modifiedMessages

		modifiedBody, err := json.Marshal(&data)
		// add partial=true in the last message
		// {
		//    "max_tokens": 2048,
		//    "messages": [{
		//            "content": "Please write a short science fiction story.",
		//            "role": "user"
		//        }, {
		//            "content": "In the year 2347 AD, Earth was no longer the only home for humanity.",
		//            "partial": true,
		//            "role": "assistant"
		//        }
		//    ],
		//    "model": "qwen-plus",
		//    "temperature": 0.7,
		//    "top_p": 0.7
		//}
		return modifiedBody, err
	})
}

func of[T any](t T) *T {
	return &t
}

const (
	keyMessages = "messages"
	keyPartial  = "partial"
)
