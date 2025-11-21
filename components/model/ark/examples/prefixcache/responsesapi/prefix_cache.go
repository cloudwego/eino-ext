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
		Cache: &ark.CacheConfig{
			APIType: ptrOf(ark.ResponsesAPI),
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	// create response prefix cache, note: more than 1024 tokens are required, otherwise the prefix cache cannot be created
	cacheInfo, err := chatModel.CreatePrefixCache(ctx, []*schema.Message{
		schema.SystemMessage("If you are an expert in analyzing novels, please analyze the issues related to the Romance of the Three Kingdoms based on the following content: ......"),
	}, 300)
	if err != nil {
		log.Fatalf("CreatePrefixCache failed, err=%v", err)
	}

	// use cache information in subsequent requests
	cacheOpt := &ark.CacheOption{
		APIType:                ark.ResponsesAPI,
		HeadPreviousResponseID: &cacheInfo.ResponseID,
	}

	outMsg, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("What is the main idea expressed above？"),
	}, ark.WithCache(cacheOpt))

	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	respID, ok := ark.GetResponseID(outMsg)
	if !ok {
		log.Fatalf("not found response id in message")
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", respID)
	respBody, _ := json.MarshalIndent(outMsg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
func ptrOf[T any](v T) *T {
	return &v

}
