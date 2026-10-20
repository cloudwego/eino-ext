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
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/moonshot"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("MOONSHOT_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	if modelName == "" {
		modelName = "moonshot-v1-8k"
	}

	cm, err := moonshot.NewChatModel(ctx, &moonshot.ChatModelConfig{
		APIKey:      apiKey,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.3)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of moonshot failed, err=%v", err)
	}

	sr, err := cm.Stream(ctx, []*schema.Message{
		schema.UserMessage("你好"),
	})
	if err != nil {
		log.Fatalf("Stream of moonshot failed, err=%v", err)
	}

	var msgs []*schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("Stream of moonshot failed, err=%v", err)
		}
		fmt.Println(msg)
		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}

	fmt.Println(msg)
}

func of[T any](t T) *T {
	return &t
}
