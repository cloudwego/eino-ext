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
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func main() {
	if err := exampleutil.Run(2*time.Minute, run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ChatModelConfig) error {
	config.Thinking = &chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)}
	cm, err := ark.NewChatModel(ctx, config)
	if err != nil {
		return err
	}
	params := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"a": {Type: schema.Integer, Required: true, Desc: "First integer"},
		"b": {Type: schema.Integer, Required: true, Desc: "Second integer"},
	})
	bound, err := cm.WithTools([]*schema.ToolInfo{
		{Name: "add", Desc: "Add two integers", ParamsOneOf: params},
		{Name: "multiply", Desc: "Multiply two integers", ParamsOneOf: params},
	})
	if err != nil {
		return err
	}
	messages := []*schema.Message{{Role: schema.User, Content: "Use the add tool to add 8 and 13, then explain the result."}}
	// Restrict this call to add, even though two tools are bound.
	reader, err := bound.Stream(ctx, messages, model.WithToolChoice(schema.ToolChoiceForced, "add"), ark.WithParallelToolCalls(false))
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
	// Function arguments can be split across chunks. Decode the assembled JSON.
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
			A *int `json:"a"`
			B *int `json:"b"`
		}
		if err := sonic.UnmarshalString(call.Function.Arguments, &args); err != nil {
			return fmt.Errorf("decode add arguments: %w", err)
		}
		if args.A == nil || args.B == nil {
			return errors.New("add requires a and b")
		}
		result := strconv.Itoa(*args.A + *args.B)
		fmt.Printf("add(%d, %d) = %s\n", *args.A, *args.B, result)
		messages = append(messages, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: result})
	}
	// Preserve the assistant's reasoning/tool calls in history, and stop this
	// two-step example from asking for more tool executions.
	answer, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForbidden))
	if err != nil {
		return err
	}
	fmt.Println(answer.Content)
	return nil
}
