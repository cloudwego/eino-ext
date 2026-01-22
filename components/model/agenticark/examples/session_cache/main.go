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
	"time"

	"github.com/cloudwego/eino-ext/components/model/agenticark"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func main() {
	ctx := context.Background()

	expireAtSec := time.Now().Add(10 * time.Minute).Unix()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	am, err := agenticark.New(ctx, &agenticark.Config{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
		Thinking: &responses.ResponsesThinking{
			Type: responses.ThinkingType_disabled.Enum(),
		},
		Cache: &agenticark.CacheConfig{
			SessionCache: &agenticark.SessionCacheConfig{
				EnableCache: true,
				ExpireAtSec: expireAtSec,
			},
		},
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
