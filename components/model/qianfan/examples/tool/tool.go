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
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/model/qianfan"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	qcfg := qianfan.GetQianfanSingletonConfig()
	// How to get Access Key/Secret Key: https://cloud.baidu.com/doc/Reference/s/9jwvz2egb
	qcfg.AccessKey = "your_access_key"
	qcfg.SecretKey = "your_secret_key"

	cm, err := qianfan.NewChatModel(ctx, &qianfan.ChatModelConfig{
		Model:               "ernie-3.5-8k",
		Temperature:         of(float32(0.7)),
		TopP:                of(float32(0.7)),
		MaxCompletionTokens: of(1024),
	})
	if err != nil {
		log.Fatalf("NewChatModel of qianfan failed, err=%v", err)
	}

	err = cm.BindTools([]*schema.ToolInfo{
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
	})
	if err != nil {
		log.Fatalf("BindTools of qianfan failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "你是一名房产经纪人，结合用户的薪酬和工作，使用 user_company、user_salary 两个 API，为其提供相关的房产信息。邮箱是必须的",
		},
		{
			Role:    schema.User,
			Content: "我的姓名是 zhangsan，我的邮箱是 zhangsan@bytedance.com，请帮我推荐一些适合我的房子。",
		},
	})

	if err != nil {
		log.Fatalf("Generate of qianfan failed, err=%v", err)
	}

	fmt.Println(resp)
	// tool_calls: [{0x14000198780 19f0f992160c4000  {user_company {"name": "zhangsan", "email": "zhangsan@bytedance.com"}} map[]} {0x14000198788 19f0f992160c4001  {user_salary {"name": "zhangsan", "email": "zhangsan@bytedance.com"}} map[]}]
}

func of[T any](t T) *T {
	return &t
}
