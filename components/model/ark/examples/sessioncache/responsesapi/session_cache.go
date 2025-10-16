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
	"io"
	"log"
	"os"

	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/schema"
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

	thinking := &arkModel.Thinking{
		Type: arkModel.ThinkingTypeDisabled,
	}
	cacheOpt := &ark.CacheOption{
		APIType: ark.ResponsesAPI,
		SessionCache: &ark.SessionCacheConfig{
			EnableCache: true,
			TTL:         86400,
		},
	}

	useMsgs := []*schema.Message{
		schema.UserMessage("Your name is superman"),
		schema.UserMessage("What's your name?"),
		schema.UserMessage("What do I ask you last time?"),
	}

	var input []*schema.Message
	for _, msg := range useMsgs {
		input = append(input, msg)

		streamResp, err := chatModel.Stream(ctx, input,
			ark.WithThinking(thinking),
			ark.WithCache(cacheOpt))
		if err != nil {
			log.Fatalf("Stream failed, err=%v", err)
		}

		var messages []*schema.Message
		for {
			chunk, err := streamResp.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Recv of streamResp failed, err=%v", err)
			}
			messages = append(messages, chunk)
		}

		resp, err := schema.ConcatMessages(messages)
		if err != nil {
			log.Fatalf("ConcatMessages of ark failed, err=%v", err)
		}

		fmt.Printf("stream output: \n%v\n\n", resp)

		input = append(input, resp)
	}
}
