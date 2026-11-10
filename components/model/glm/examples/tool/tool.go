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
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/glm"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	// get api key: https://docs.bigmodel.cn/cn/guide/start/model-overview
	apiKey := os.Getenv("ZHIPU_API_KEY")
	modelName := os.Getenv("MODEL_NAME") // e.g. glm-4.6, glm-4.5-flash, glm-4-flash-250414
	cm, err := glm.NewChatModel(ctx, &glm.ChatModelConfig{
		APIKey:      apiKey,
		Model:       modelName,
		Temperature: of(float32(0.7)),
	})
	if err != nil {
		log.Fatalf("NewChatModel of glm failed, err=%v", err)
	}

	err = cm.BindTools([]*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "根据用户的姓名和邮箱，查询用户的公司和职位信息",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name":  {Type: "string", Desc: "用户的姓名"},
					"email": {Type: "string", Desc: "用户的邮箱"},
				}),
		},
	})
	if err != nil {
		log.Fatalf("BindTools of glm failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一名房产经纪人，结合用户的公司和职位，使用 user_company API，为其提供相关的房产信息。邮箱是必须的",
		},
		{
			Role:    schema.User,
			Content: "我的姓名是 zhangsan，我的邮箱是 zhangsan@bytedance.com，请帮我推荐一些适合我的房子。",
		},
	})
	if err != nil {
		log.Fatalf("Generate of glm failed, err=%v", err)
	}

	fmt.Println(resp)
}

func of[T any](t T) *T {
	return &t
}
