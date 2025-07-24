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
	"io"
	"log"
	"os"
	"time"

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

	instructions := []*schema.Message{
		schema.SystemMessage("Your name is superman"),
	}

	cacheInfo, err := chatModel.CreateSessionCache(ctx, instructions, 86400, nil)
	if err != nil {
		log.Fatalf("CreateSessionCache failed, err=%v", err)
	}

	thinking := &arkModel.Thinking{
		Type: arkModel.ThinkingTypeDisabled,
	}

	cacheOpt := &ark.CacheOption{
		APIType:   ark.ContextAPI,
		ContextID: &cacheInfo.ContextID,
		SessionCache: &ark.SessionCacheConfig{
			EnableCache: true,
			TTL:         86400,
		},
	}

	msg, err := chatModel.Generate(ctx, instructions,
		ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	<-time.After(500 * time.Millisecond)

	msg, err = chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What's your name?",
		},
	},
		ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	fmt.Printf("\ngenerate output: \n")
	fmt.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	fmt.Printf("  body: %s\n", string(respBody))

	outStreamReader, err := chatModel.Stream(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "What do I ask you last time?",
		},
	},
		ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	fmt.Println("\ntypewriter output:")
	var msgs []*schema.Message
	for {
		item, e := outStreamReader.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			log.Fatal(e)
		}

		fmt.Print(item.Content)
		msgs = append(msgs, item)
	}

	msg, err = schema.ConcatMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}
	fmt.Print("\n\nstream output: \n")
	fmt.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ = json.MarshalIndent(msg, "  ", "  ")
	fmt.Printf("  body: %s\n", string(respBody))
}
