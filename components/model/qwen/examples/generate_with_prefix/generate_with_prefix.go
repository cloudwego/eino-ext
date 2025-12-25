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
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/schema"
	"io"
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

	// make the message attribute partial=true
	partialMessage := qwen.NewPartialMessage(schema.AssistantMessage("def calculate_fibonacci(n):\\n"+
		"    if n <= 1:\\n        return n\\n    else:\\n", nil), true)
	messages := []*schema.Message{
		// give a user message
		schema.UserMessage("Please complete this Fibonacci function without adding any other content."),
		// give a prefix of the function
		partialMessage,
	}
	callGenerate(err, chatModel, ctx, messages)

	callStream(err, chatModel, ctx, messages)
}

func callGenerate(err error, chatModel *qwen.ChatModel, ctx context.Context, messages []*schema.Message) {
	resp, err := chatModel.Generate(ctx, messages)
	// add partial=true in the target message
	// {
	//    "max_tokens": 2048,
	//    "messages": [{
	//            "role": "user",
	//            "content": "Please complete this Fibonacci function without adding any other content."
	//        }, {
	//            "role": "assistant",
	//            "content": "def calculate_fibonacci(n):\n    if n <= 1:\n        return n\n    else:\n",
	//            "partial": true
	//        }
	//    ],
	//    "model": "qwen-plus",
	//    "temperature": 0.7,
	//    "top_p": 0.7
	//}
	if err != nil {
		log.Fatalf("Generate of qwen failed, err=%v", err)
	}

	fmt.Printf("generate output: \n%v\n", resp)
}

func callStream(err error, chatModel *qwen.ChatModel, ctx context.Context, messages []*schema.Message) {
	sr, err := chatModel.Stream(ctx, messages)
	if err != nil {
		log.Fatalf("Stream of qwen failed, err=%v", err)
	}
	var msgs []*schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatalf("Stream of qwen failed, err=%v", err)
		}
		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	fmt.Printf("stream output: \n%v\n", msg)
}

func of[T any](t T) *T {
	return &t
}
