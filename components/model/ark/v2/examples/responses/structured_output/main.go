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
	"log"

	"fmt"
	"os"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	config.ResponseFormat = &responses.TextFormat{}
	if err := sonic.UnmarshalString(`{"type":"json_schema","name":"result","strict":true,"schema":{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}}`, config.ResponseFormat); err != nil {
		return err
	}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "What is 8 + 13? Return the answer as a string."
	}
	reply, err := cm.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return err
	}
	var result struct {
		Answer string `json:"answer"`
	}
	if err = sonic.UnmarshalString(reply.Content, &result); err != nil {
		return err
	}
	fmt.Println("Decoded answer:", result.Answer)
	exampleutil.PrintResponse(reply)
	return nil
}
