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

	"github.com/cloudwego/eino-ext/components/model/agenticdeepseek"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	m, err := agenticdeepseek.New(ctx, &agenticdeepseek.Config{
		APIKey:      apiKey,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})
	if err != nil {
		log.Fatalf("New of agenticdeepseek failed, err=%v", err)
	}

	tools := []*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "Retrieve the user's company and position based on their name and email.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "user's name",
					},
					"email": {
						Type: "string",
						Desc: "user's email",
					},
				}),
		},
		{
			Name: "user_salary",
			Desc: "Retrieve the user's salary based on their name and email.",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "user's name",
					},
					"email": {
						Type: "string",
						Desc: "user's email",
					},
				}),
		},
	}

	resp, err := m.Generate(ctx, []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeSystem,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{
					Text: "As a real estate agent, provide relevant property information based on the user's salary and job using the user_company and user_salary APIs. An email address is required.",
				}),
			},
		},
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{
					Text: "My name is John and my email is john@abc.com. Please recommend some houses that suit me.",
				}),
			},
		},
	}, model.WithTools(tools))
	if err != nil {
		log.Fatalf("Generate of agenticdeepseek failed, err=%v", err)
	}

	fmt.Println(resp)

	// ==========================
	// using stream
	fmt.Printf("\n\n======== Stream ========\n")
	sr, err := m.Stream(ctx, []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeSystem,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{
					Text: "As a real estate agent, provide relevant property information based on the user's salary and job using the user_company and user_salary APIs. An email address is required.",
				}),
			},
		},
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{
					Text: "My name is Jane and my email is jane@abc.com. Please recommend some houses that suit me.",
				}),
			},
		},
	}, model.WithTools(tools))
	if err != nil {
		log.Fatalf("Stream of agenticdeepseek failed, err=%v", err)
	}

	msgs := make([]*schema.AgenticMessage, 0)
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			log.Fatalf("json.Marshal failed, err=%v", err)
		}
		fmt.Printf("%s\n", jsonMsg)
		msgs = append(msgs, msg)
	}

	msg, err := schema.ConcatAgenticMessages(msgs)
	if err != nil {
		log.Fatalf("ConcatAgenticMessages failed, err=%v", err)
	}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("json.Marshal failed, err=%v", err)
	}
	fmt.Printf("final: %s\n", jsonMsg)
}

func of[T any](t T) *T {
	return &t
}
