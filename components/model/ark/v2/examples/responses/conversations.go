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
	"strings"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func runTools(ctx context.Context, cm *ark.ResponsesAPIChatModel) (*schema.Message, error) {
	bound, err := cm.WithTools([]*schema.ToolInfo{{Name: "add", Desc: "Add two integers", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"a": {Type: schema.Integer, Required: true}, "b": {Type: schema.Integer, Required: true},
	})}})
	if err != nil {
		return nil, err
	}
	messages := []*schema.Message{schema.UserMessage("Use add to calculate 8 + 13, then explain the result.")}
	reader, err := bound.Stream(ctx, messages, model.WithToolChoice(schema.ToolChoiceForced, "add"))
	if err != nil {
		return nil, err
	}
	assistant, err := collect(reader, false)
	if err != nil {
		return nil, err
	}
	if len(assistant.ToolCalls) == 0 {
		return nil, errors.New("model returned no tool calls")
	}
	messages = append(messages, assistant)
	for _, call := range assistant.ToolCalls {
		if call.Function.Name != "add" || call.ID == "" {
			return nil, fmt.Errorf("unexpected tool call: %+v", call)
		}
		var args struct {
			A *int `json:"a"`
			B *int `json:"b"`
		}
		if err = sonic.UnmarshalString(call.Function.Arguments, &args); err != nil {
			return nil, err
		}
		if args.A == nil || args.B == nil {
			return nil, errors.New("add arguments must include a and b")
		}
		result := *args.A + *args.B
		if *args.A != 8 || *args.B != 13 {
			return nil, fmt.Errorf("unexpected add arguments: %s", call.Function.Arguments)
		}
		fmt.Printf("Tool %s(%s) = %d\n", call.Function.Name, call.Function.Arguments, result)
		messages = append(messages, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: fmt.Sprint(result)})
	}
	answer, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForbidden))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(answer.Content, "21") {
		return nil, fmt.Errorf("final answer did not use the tool result: %q", answer.Content)
	}
	return answer, nil
}

func runToolResult(ctx context.Context, cm *ark.ResponsesAPIChatModel, media schema.MessageInputPart) (*schema.Message, error) {
	bound, err := cm.WithTools([]*schema.ToolInfo{{Name: "read_media", Desc: "Return the media requested by the user", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}})
	if err != nil {
		return nil, err
	}
	messages := []*schema.Message{schema.UserMessage("Call read_media, then answer this question using the actual returned media: " + *prompt)}
	call, err := bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForced, "read_media"))
	if err != nil {
		return nil, err
	}
	if len(call.ToolCalls) != 1 || call.ToolCalls[0].Function.Name != "read_media" || call.ToolCalls[0].ID == "" {
		return nil, errors.New("expected one read_media call")
	}
	fmt.Println("Returning media for tool call:", call.ToolCalls[0].ID)
	messages = append(messages, call, &schema.Message{Role: schema.Tool, ToolCallID: call.ToolCalls[0].ID, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "Read the attached media to answer the user's question; do not invent content."}, media,
	}})
	return bound.Generate(ctx, messages, model.WithToolChoice(schema.ToolChoiceForbidden))
}

func runReasoning(ctx context.Context, cm *ark.ResponsesAPIChatModel) (*schema.Message, error) {
	question := schema.UserMessage("What is 17 times 19? Answer briefly.")
	reader, err := cm.Stream(ctx, []*schema.Message{question})
	if err != nil {
		return nil, err
	}
	assistant, err := collect(reader, true)
	if err != nil {
		return nil, err
	}
	if assistant.ReasoningContent == "" {
		return nil, errors.New("model returned no reasoning content")
	}
	if !strings.Contains(assistant.Content, "323") {
		return nil, fmt.Errorf("unexpected multiplication answer: %q", assistant.Content)
	}
	fmt.Println("Reasoning preserved; sending the assistant message back with a follow-up.")
	// No previous_response_id is used: EnableReasoningContentPassback sends the
	// preserved reasoning item rather than relying on server-side context.
	answer, err := cm.Generate(ctx, []*schema.Message{question, assistant, schema.UserMessage("Add one to that answer. Reply with only the number.")},
		ark.WithThinking(&chat.Thinking{Type: chat.NewOptThinkingMode(chat.ThinkingModeDisabled)}))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(answer.Content, "324") {
		return nil, fmt.Errorf("reasoning follow-up was incorrect: %q", answer.Content)
	}
	return answer, nil
}
