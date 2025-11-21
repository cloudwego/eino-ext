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

	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
		Cache: &ark.CacheConfig{
			SessionCache: &ark.SessionCacheConfig{
				EnableCache: true,
				TTL:         86400,
			},
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	thinking := &arkModel.Thinking{
		Type: arkModel.ThinkingTypeDisabled,
	}

	message, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage(`You are a film screenwriter who, based on the storyline provided by the user, continues to write other plots of this story`),
		schema.UserMessage("long long ago, there was a wolf and a sheep"),
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}

	message, err = chatModel.Generate(ctx, []*schema.Message{
		message,
		schema.UserMessage("Next sentence"),
	}, ark.WithThinking(thinking))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}
	fmt.Println(message)
	message, err = chatModel.Generate(ctx, []*schema.Message{
		message,
		schema.UserMessage("Next sentence"),
	}, ark.WithThinking(thinking))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}

}
