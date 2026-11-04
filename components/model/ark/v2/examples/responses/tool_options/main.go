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
	"log"

	"github.com/bytedance/sonic"
	ark "github.com/cloudwego/eino-ext/components/model/ark/v2"
	"github.com/cloudwego/eino-ext/components/model/ark/v2/examples/internal/exampleutil"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

func main() {
	if err := exampleutil.RunResponses(run); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, config *ark.ResponsesAPIConfig) error {
	strict := true
	config.ToolStrict = &strict
	var parameters jsonschema.Schema
	if err := sonic.UnmarshalString(`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"additionalProperties":false}`, &parameters); err != nil {
		return err
	}
	add := &schema.ToolInfo{Name: "add", Desc: "Add two integers", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameters)}
	multiply := ark.WithToolStrict(&schema.ToolInfo{Name: "multiply", Desc: "Multiply two integers", ParamsOneOf: add.ParamsOneOf}, false)
	subtract := ark.WithToolStrict(&schema.ToolInfo{Name: "subtract", Desc: "Subtract b from a", ParamsOneOf: add.ParamsOneOf}, true)
	// add inherits strict=true; multiply overrides it with false. All three
	// tools are available locally, but the candidate names exclude subtract.
	tools := []*schema.ToolInfo{add, multiply, subtract}
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	if err := cm.BindTools(tools); err != nil {
		return err
	}

	fmt.Println("Allowed subset: add, multiply")
	history := []*schema.Message{schema.UserMessage("Use add to calculate 8 + 13. Do not calculate it yourself.")}
	assistant, err := cm.Generate(ctx, history, model.WithToolChoice(schema.ToolChoiceAllowed, "add", "multiply"))
	if err != nil {
		return err
	}
	if len(assistant.ToolCalls) == 0 {
		// Allowed permits a direct answer. It does not require a tool call.
		exampleutil.PrintResponse(assistant)
	}
	history, err = appendToolResults(history, assistant)
	if err != nil {
		return err
	}

	fmt.Println("Forced subset: add, multiply")
	history = append(history, schema.UserMessage("Now use multiply to calculate 6 * 7."))
	// The per-call definitions replace the bindings; strict overrides are
	// preserved. Multiple forced names send required with a filtered tool set.
	reader, err := cm.Stream(ctx, history, model.WithTools(tools), model.WithToolChoice(schema.ToolChoiceForced, "add", "multiply"))
	if err != nil {
		return err
	}
	assistant, err = schema.ConcatMessageStream(reader)
	if err != nil {
		return err
	}
	history, err = appendToolResults(history, assistant)
	if err != nil {
		return err
	}
	reply, err := cm.Generate(ctx, history, model.WithToolChoice(schema.ToolChoiceForbidden))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(reply)
	return nil
}

func appendToolResults(history []*schema.Message, assistant *schema.Message) ([]*schema.Message, error) {
	history = append(history, assistant)
	for _, call := range assistant.ToolCalls {
		var args struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := sonic.UnmarshalString(call.Function.Arguments, &args); err != nil {
			return nil, err
		}
		var result int
		switch call.Function.Name {
		case "add":
			result = args.A + args.B
		case "multiply":
			result = args.A * args.B
		default:
			return nil, fmt.Errorf("unexpected tool %q", call.Function.Name)
		}
		fmt.Printf("%s(%d, %d) = %d\n", call.Function.Name, args.A, args.B, result)
		history = append(history, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: fmt.Sprint(result)})
	}
	return history, nil
}
