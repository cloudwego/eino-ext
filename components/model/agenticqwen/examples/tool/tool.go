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

	"github.com/cloudwego/eino-ext/components/model/agenticqwen"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	modelName := os.Getenv("MODEL_NAME")
	m, err := agenticqwen.New(ctx, &agenticqwen.Config{
		BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKey:      apiKey,
		Timeout:     0,
		Model:       modelName,
		MaxTokens:   of(2048),
		Temperature: of(float32(0.7)),
		TopP:        of(float32(0.7)),
	})
	if err != nil {
		log.Fatalf("New of agenticqwen failed, err=%v", err)
	}

	tools := []*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "根据用户的姓名和邮箱，查询用户的公司和职位信息",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "用户的姓名",
					},
					"email": {
						Type: "string",
						Desc: "用户的邮箱",
					},
				}),
		},
		{
			Name: "user_salary",
			Desc: "根据用户的姓名和邮箱，查询用户的薪酬信息",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "用户的姓名",
					},
					"email": {
						Type: "string",
						Desc: "用户的邮箱",
					},
				}),
		},
	}

	resp, err := m.Generate(ctx, []*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeSystem,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{
					Text: "你是一名房产经纪人，结合用户的薪酬和工作，使用 user_company、user_salary 两个 API，为其提供相关的房产信息。邮箱是必须的",
				}),
			},
		},
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{
					Text: "我的姓名是 zhangsan，我的邮箱是 zhangsan@bytedance.com，请帮我推荐一些适合我的房子。",
				}),
			},
		},
	}, model.WithTools(tools))
	if err != nil {
		log.Fatalf("Generate of agenticqwen failed, err=%v", err)
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
					Text: "你是一名房产经纪人，结合用户的薪酬和工作，使用 user_company、user_salary 两个 API，为其提供相关的房产信息。邮箱是必须的",
				}),
			},
		},
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{
					Text: "我的姓名是 lisi，我的邮箱是 lisi@bytedance.com，请帮我推荐一些适合我的房子。",
				}),
			},
		},
	}, model.WithTools(tools))
	if err != nil {
		log.Fatalf("Stream of agenticqwen failed, err=%v", err)
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
