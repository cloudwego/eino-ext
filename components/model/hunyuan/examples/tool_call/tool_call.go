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
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/hunyuan"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	secretId := os.Getenv("HUNYUAN_SECRET_ID")
	secretKey := os.Getenv("HUNYUAN_SECRET_KEY")

	if secretId == "" || secretKey == "" {
		log.Fatal("HUNYUAN_SECRET_ID and HUNYUAN_SECRET_KEY environment variables are required")
	}

	cm, err := hunyuan.NewChatModel(ctx, &hunyuan.ChatModelConfig{
		SecretId:  secretId,
		SecretKey: secretKey,
		Model:     "hunyuan-vision",
	})

	if err != nil {
		log.Printf("NewChatModel failed, err=%v", err)
		return
	}

	fmt.Printf("\n==========text tool call==========\n")
	textToolCall(ctx, cm)
	fmt.Printf("\n==========image tool call==========\n")
	imageToolCall(ctx, cm)
}

func textToolCall(ctx context.Context, chatModel model.ToolCallingChatModel) {
	chatModel, err := chatModel.WithTools([]*schema.ToolInfo{
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
		log.Fatalf("WithTools failed, err=%v", err)
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
		log.Fatalf("Generate failed, err=%v", err)
		return
	}

	fmt.Printf("output: \n%v", resp)
}

func imageToolCall(ctx context.Context, chatModel model.ToolCallingChatModel) {
	image, err := os.ReadFile("./examples/tool_call/img.png")
	if err != nil {
		log.Fatalf("os.ReadFile failed, err=%v\n", err)
	}

	imageStr := base64.StdEncoding.EncodeToString(image)

	chatModel, err = chatModel.WithTools([]*schema.ToolInfo{
		{
			Name: "image_generator",
			Desc: "a tool can generate images",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"description": {
						Type: "string",
						Desc: "The description of the image",
					},
				}),
		},
	})
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
		return
	}

	query := []*schema.Message{
		{
			Role:    schema.System,
			Content: "You are a helpful assistant. If the user needs to generate an image, call the image_generator tool to generate the image.",
		},
		{
			Role:    schema.User,
			Content: "Generator a cat image",
		},
	}

	resp, err := chatModel.Generate(ctx, query)
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
		return
	}

	if len(resp.ToolCalls) == 0 {
		log.Fatalf("No tool calls found")
		return
	}

	fmt.Printf("output: \n%v", resp)

	resp, err = chatModel.Generate(ctx, append(query, resp, &schema.Message{
		Role:       schema.Tool,
		ToolCallID: resp.ToolCalls[0].ID,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "Image generation successful.",
			},
			{
				Type: schema.ChatMessagePartTypeImageURL,
				Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						Base64Data: &imageStr,
						MIMEType:   "image/png",
					},
				},
			},
		},
	}))
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}
	fmt.Printf("output: \n%v", resp)
}
