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

	"errors"
	"fmt"
	"io"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {

	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	bound, err := cm.WithTools([]*schema.ToolInfo{{Name: "add", Desc: "Add two integers", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"a": {Type: schema.Integer, Required: true}, "b": {Type: schema.Integer, Required: true},
	})}})
	if err != nil {
		return err
	}
	messages := []*schema.Message{schema.UserMessage("Use add to calculate 8 + 13, then explain the result.")}
	reader, err := bound.Stream(ctx, messages, model.WithToolChoice(schema.ToolChoiceForced, "add"))
	if err != nil {
		return err
	}
	defer reader.Close()
	var chunks []*schema.Message
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		chunks = append(chunks, chunk)
	}
	// A function's JSON arguments may span several chunks.
	assistant, err := schema.ConcatMessages(chunks)
	if err != nil {
		return err
	}
	if len(assistant.ToolCalls) == 0 {
		return errors.New("model returned no tool calls")
	}
	messages = append(messages, assistant)
	for _, call := range assistant.ToolCalls {
		if call.Function.Name != "add" {
			return fmt.Errorf("unexpected tool %q", call.Function.Name)
		}
		var args struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err = sonic.UnmarshalString(call.Function.Arguments, &args); err != nil {
			return err
		}
		result := args.A + args.B
		fmt.Printf("add(%d, %d) = %d\n", args.A, args.B, result)
		messages = append(messages, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: fmt.Sprint(result)})
	}
	reply, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForbidden))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
