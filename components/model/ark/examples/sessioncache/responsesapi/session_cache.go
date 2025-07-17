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
	thinking := &arkModel.Thinking{
		Type: arkModel.ThinkingTypeDisabled,
	}

	persist := true
	ttl := 86400
	cacheOpt := &ark.CacheOption{
		APIType: ark.ResponsesAPI,
		SessionCacheOption: &ark.SessionCacheOption{
			PersistCurrentContext: &persist,
			TTL:                   &ttl,
		},
	}

	msg, err := chatModel.Generate(ctx, instructions,
		ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("CreatePrefix failed, err=%v", err)
	}

	firstContextID, ok := ark.GetContextID(msg)
	if !ok {
		log.Fatalf("GetContextID failed, err=%v", err)
	}

	cacheOpt.ContextID = &firstContextID

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

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

	secondContextID, ok := ark.GetContextID(msg)
	if !ok {
		log.Fatalf("GetContextID failed, err=%v", err)
	}

	cacheOpt.ContextID = &secondContextID

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

	log.Println("typewriter output:")
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
	log.Printf("\nstream output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ = json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
