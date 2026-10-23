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
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func main() {
	if err := exampleutil.Run(2*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	config.Thinking = &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeEnabled)}
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	// Cancel this context as well as closing the reader if stopping early.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, err := cm.Stream(ctx, []*schema.Message{{Role: schema.User, Content: "Explain why the sum of two odd integers is even."}})
	if err != nil {
		return err
	}
	defer reader.Close()
	var chunks []*schema.Message
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		fmt.Print(chunk.ReasoningContent)
		fmt.Print(chunk.Content)
		chunks = append(chunks, chunk)
	}
	// Keep usage-only chunks and assemble only after the complete stream.
	message, err := schema.ConcatMessages(chunks)
	if err != nil {
		return err
	}
	fmt.Printf("\nrequest_id=%s\nreasoning=%s\nanswer=%s\nusage=%+v\n",
		ark.GetArkRequestID(message), message.ReasoningContent, message.Content, message.ResponseMeta.Usage)
	return nil
}
