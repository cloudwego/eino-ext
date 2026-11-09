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
	"os"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {
	config.Thinking = &responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeEnabled)}
	parallel := true
	config.ParallelToolCalls = &parallel
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	parameters := schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"a": {Type: schema.Integer, Required: true}, "b": {Type: schema.Integer, Required: true},
	})
	bound, err := cm.WithTools([]*schema.ToolInfo{
		{Name: "add", Desc: "Add two integers", ParamsOneOf: parameters},
		{Name: "multiply", Desc: "Multiply two integers", ParamsOneOf: parameters},
	})
	if err != nil {
		return err
	}
	prompt := os.Getenv("ARK_PROMPT")
	if prompt == "" {
		prompt = "First tell me briefly which two calculations you will perform. Then call add for 8 + 13 and multiply for 6 * 7 in parallel, in this same response. Use both tools before waiting for either result."
	}
	history := []*schema.Message{
		schema.SystemMessage("Before calling tools, send a short progress sentence in the assistant's visible output. Keep it separate from private reasoning. Issue independent function calls together."),
		schema.UserMessage(prompt),
	}
	reader, err := bound.Stream(ctx, history, model.WithToolChoice(schema.ToolChoiceAllowed))
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
		if err := exampleutil.PrintResponsesChunk(len(chunks), chunk); err != nil {
			return err
		}
	}
	assistant, err := schema.ConcatMessages(chunks)
	if err != nil {
		return err
	}
	if err := exampleutil.PrintResponsesConcat(chunks, assistant); err != nil {
		return err
	}
	reasoning := false
	for _, part := range assistant.AssistantGenMultiContent {
		reasoning = reasoning || part.Reasoning != nil
	}
	// Models may omit the requested visible progress sentence. Print the actual
	// shape rather than adding synthetic content to the streamed messages.
	fmt.Printf("Mixed response: reasoning=%t content=%t tool_calls=%d\n", reasoning, assistant.Content != "", len(assistant.ToolCalls))
	if len(assistant.ToolCalls) != 2 {
		return fmt.Errorf("expected two independent tool calls, got %d", len(assistant.ToolCalls))
	}
	history = append(history, assistant)
	seen := make(map[string]bool)
	for _, call := range assistant.ToolCalls {
		if seen[call.Function.Name] {
			return fmt.Errorf("duplicate tool %q", call.Function.Name)
		}
		seen[call.Function.Name] = true
		var args struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := sonic.UnmarshalString(call.Function.Arguments, &args); err != nil {
			return err
		}
		var result int
		switch call.Function.Name {
		case "add":
			result = args.A + args.B
		case "multiply":
			result = args.A * args.B
		default:
			return fmt.Errorf("unexpected tool %q", call.Function.Name)
		}
		fmt.Printf("%s(%d, %d) = %d\n", call.Function.Name, args.A, args.B, result)
		history = append(history, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: fmt.Sprint(result)})
	}
	reply, err := bound.Generate(ctx, history, model.WithToolChoice(schema.ToolChoiceForbidden),
		ark.WithResponsesThinking(&responses.Thinking{Type: responses.NewOptThinkingMode(responses.ThinkingModeDisabled)}))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}
