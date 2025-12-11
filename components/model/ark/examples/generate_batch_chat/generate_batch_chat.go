/*
 * Copyright 2024 CloudWeGo Authors
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
	"log"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

func main() {
	ctx := context.Background()
	var batchMaxParallel = 10000

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
		BatchChat: &ark.BatchChatConfig{
			// Control whether to use the batch chat completion API. Only applies to non-streaming scenarios.
			EnableBatchChat: true,
			// Control the timeout for the batch chat completion API. model will keep retrying until a timeout occurs or the execution succeeds.
			BatchChatAsyncRetryTimeout: 30 * time.Minute,
			// Control the maximum number of parallel requests to send to the chat completion API.
			BatchMaxParallel: &batchMaxParallel,
		},
	})

	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	// Generate 10000 requests.
	inMsgList := make([][]*schema.Message, 0, 10000)
	for i := 0; i < 10000; i++ {
		inMsgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "how do you generate answer for user question as a machine, please answer in short?",
			},
		}
		inMsgList = append(inMsgList, inMsgs)
	}

	wg := sync.WaitGroup{}
	// Send 10000 requests in parallel.
	for index, inMsgs := range inMsgList {
		wg.Add(1)
		_inMsgs := inMsgs
		_index := index
		go func() {
			defer wg.Done()
			// Batch chat only applies to non-streaming scenarios
			msg, err := chatModel.Generate(ctx, _inMsgs)
			if err != nil {
				log.Printf("Generate failed,index=%d err=%v", _index, err)
				return
			}
			log.Printf("\nindex:%d generate output,: \n", _index)
			log.Printf("index:%d request_id: %s\n", _index, ark.GetArkRequestID(msg))
			respBody, _ := json.MarshalIndent(msg, "  ", "  ")
			log.Printf("index:%d body: %s\n", _index, string(respBody))
		}()
	}
	wg.Wait()
}
