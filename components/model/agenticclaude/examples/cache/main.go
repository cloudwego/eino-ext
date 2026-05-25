/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	cacheCtrl := anthropic.NewCacheControlEphemeralParam()
	cacheCtrl.TTL = anthropic.CacheControlEphemeralTTLTTL5m

	am, err := agenticclaude.New(ctx, &agenticclaude.Config{
		BaseURL:      os.Getenv("CLAUDE_BASE_URL"),
		Model:        os.Getenv("CLAUDE_MODEL"),
		APIKey:       os.Getenv("CLAUDE_API_KEY"),
		MaxTokens:    4096,
		CacheControl: &cacheCtrl,
	})
	if err != nil {
		log.Fatalf("failed to create chat model, err=%v", err)
	}

	useMsgs := []*schema.AgenticMessage{
		schema.UserAgenticMessage("Your name is superman"),
		schema.UserAgenticMessage("What's your name?"),
		schema.UserAgenticMessage("What do I ask you last time?"),
	}

	var input []*schema.AgenticMessage
	for _, msg := range useMsgs {
		input = append(input, msg)

		streamResp, err := am.Stream(ctx, input)
		if err != nil {
			log.Fatalf("failed to stream, err: %v", err)
		}

		var messages []*schema.AgenticMessage
		for {
			chunk, err := streamResp.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("failed to receive stream response, err: %v", err)
			}
			messages = append(messages, chunk)
		}

		resp, err := schema.ConcatAgenticMessages(messages)
		if err != nil {
			log.Fatalf("failed to concat agentic messages, err: %v", err)
		}

		jsonBody, _ := json.MarshalIndent(resp, "  ", "  ")

		log.Printf("stream output json: \n%v\n\n", string(jsonBody))

		input = append(input, resp)
	}
}
