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
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	chatModelWithTools, err := chatModel.WithTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get the current weather in a given location",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"location": {
						Type: "string",
						Desc: "The city and state, e.g. San Francisco, CA",
					},
				},
			),
		},
	})
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
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

	outMsg, err := chatModelWithTools.Generate(ctx, []*schema.Message{
		schema.UserMessage("my name is megumin"),
	}, ark.WithThinking(thinking),
		ark.WithCache(cacheOpt))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	respID, ok := ark.GetResponseID(outMsg)
	if !ok {
		log.Fatalf("not found response id in message")
	}

	msg, err := chatModelWithTools.Generate(ctx, []*schema.Message{
		schema.UserMessage("what is my name?"),
	}, ark.WithThinking(thinking),
		ark.WithCache(&ark.CacheOption{
			APIType:                ark.ResponsesAPI,
			HeadPreviousResponseID: &respID,
		}),
	)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", ark.GetArkRequestID(msg))
	respBody, _ := json.MarshalIndent(msg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
