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
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ollama"
)

func main() {

	ctx := context.Background()
	modelName := os.Getenv("MODEL_NAME")
	chatModel, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: "http://localhost:11434",
		Model:   modelName,
	})
	if err != nil {
		log.Printf("NewChatModel failed, err=%v", err)
		return
	}

	err = chatModel.BindTools([]*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "Query the user's company and position information based on their name and email",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "The user's name",
					},
					"email": {
						Type: "string",
						Desc: "The user's email",
					},
				}),
		},
		{
			Name: "user_salary",
			Desc: "Query the user's salary information based on their name and email",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"name": {
						Type: "string",
						Desc: "The user's name",
					},
					"email": {
						Type: "string",
						Desc: "The user's email",
					},
				}),
		},
	})
	if err != nil {
		log.Printf("BindForcedTools failed, err=%v", err)
		return
	}

	resp, err := chatModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a real estate agent. Use the user_company and user_salary APIs to provide relevant property information based on the user's salary and job. Email is required",
		},
		{
			Role:    schema.User,
			Content: "My name is zhangsan, and my email is zhangsan@bytedance.com. Please recommend some suitable houses for me.",
		},
	})

	if err != nil {
		log.Printf("Generate failed, err=%v", err)
		return
	}

	log.Printf("output: \n%+v", resp)
}
