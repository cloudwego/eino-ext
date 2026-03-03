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
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ark"
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

	info, err := chatModel.CreatePrefixCache(ctx, []*schema.Message{
		schema.UserMessage("my name is megumin"),
	}, 3600)
	if err != nil {
		log.Fatalf("CreatePrefix failed, err=%v", err)
	}

	inMsgs := []*schema.Message{
		{
			Role:    schema.User,
			Content: "what is my name?",
		},
	}

	msg, err := chatModel.Generate(ctx, inMsgs, ark.WithCache(&ark.CacheOption{
		APIType:   ark.ContextAPI,
		ContextID: &info.ContextID,
	}))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))

	outStreamReader, err := chatModel.Stream(ctx, inMsgs, ark.WithCache(&ark.CacheOption{
		APIType:   ark.ContextAPI,
		ContextID: &info.ContextID,
	}))
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	var msgs []*schema.Message
	for {
		item, e := outStreamReader.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			log.Fatal(e)
		}

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
