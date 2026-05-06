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
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino/schema"

	openaigo "github.com/cloudwego/eino-ext/components/model/openai-go"
)

func main() {
	ctx := context.Background()

	chatModel, err := openaigo.NewChatModel(ctx, &openaigo.Config{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		Reasoning: &openaigo.Reasoning{
			Effort:  openaigo.ReasoningEffortMedium,
			Summary: openaigo.ReasoningSummaryAuto,
		},
	})
	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	cm, err := chatModel.WithTools([]*schema.ToolInfo{
		{
			Name: "user_company",
			Desc: "Retrieve the user's company and position based on their name and email.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"name":  {Type: "string", Desc: "user's name"},
				"email": {Type: "string", Desc: "user's email"},
			}),
		},
		{
			Name: "user_salary",
			Desc: "Retrieve the user's salary based on their name and email.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"name":  {Type: "string", Desc: "user's name"},
				"email": {Type: "string", Desc: "user's email"},
			}),
		},
	})
	if err != nil {
		log.Fatalf("WithTools failed, err=%v", err)
	}

	resp, err := cm.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "As a real estate agent, provide relevant property information based on the user's salary and job using the user_company and user_salary APIs. An email address is required.",
		},
		{
			Role:    schema.User,
			Content: "My name is John and my email is john@abc.com, please recommend some houses that suit me.",
		},
	})
	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}
	fmt.Printf("output: \n%v\n", resp)

	streamResp, err := cm.Stream(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: "As a real estate agent, provide relevant property information based on the user's salary and job using the user_company and user_salary APIs. An email address is required.",
		},
		{
			Role:    schema.User,
			Content: "My name is John and my email is john@abc.com, please recommend some houses that suit me.",
		},
	})
	if err != nil {
		log.Fatalf("Stream failed, err=%v", err)
	}

	var messages []*schema.Message
	for {
		chunk, err := streamResp.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Recv failed, err=%v", err)
		}
		messages = append(messages, chunk)
	}
	resp2, err := schema.ConcatMessages(messages)
	if err != nil {
		log.Fatalf("ConcatMessages failed, err=%v", err)
	}
	fmt.Printf("stream output: \n%v\n", resp2)
}
