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
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/moonshot"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("MOONSHOT_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	if modelName == "" {
		modelName = "moonshot-v1-8k"
	}

	cm, err := moonshot.NewChatModel(ctx, &moonshot.ChatModelConfig{
		APIKey:      apiKey,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.0)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of moonshot failed, err=%v", err)
	}

	tools := []*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get current weather for a city.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"city": {
					Type:     schema.String,
					Desc:     "City name, e.g. Beijing",
					Required: true,
				},
			}),
		},
	}

	bound, err := cm.WithTools(tools)
	if err != nil {
		log.Fatalf("WithTools of moonshot failed, err=%v", err)
	}

	msgs := []*schema.Message{
		schema.SystemMessage("You are a helpful assistant. When the user asks about weather, you MUST call the get_weather tool."),
		schema.UserMessage("What's the weather in Beijing right now?"),
	}

	resp, err := bound.Generate(ctx, msgs)
	if err != nil {
		log.Fatalf("Generate of moonshot failed, err=%v", err)
	}

	jsonResp, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("generate result:\n%s\n", jsonResp)

	fmt.Printf("\n======== Stream ========\n")
	sr, err := bound.Stream(ctx, msgs)
	if err != nil {
		log.Fatalf("Stream of moonshot failed, err=%v", err)
	}

	var chunks []*schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		chunks = append(chunks, msg)
	}

	merged, err := schema.ConcatMessages(chunks)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}
	jsonMerged, _ := json.MarshalIndent(merged, "", "  ")
	fmt.Printf("stream merged:\n%s\n", jsonMerged)
}

func of[T any](t T) *T {
	return &t
}
