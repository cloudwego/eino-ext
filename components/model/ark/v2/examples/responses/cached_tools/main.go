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
	config.SessionCache = &ark.SessionCacheConfig{EnableCache: true, TTL: 3600}
	strict := true
	config.ToolStrict = &strict
	cm, err := ark.NewResponsesAPIChatModel(ctx, config)
	if err != nil {
		return err
	}
	var parameters jsonschema.Schema
	if err := sonic.UnmarshalString(`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}},"required":["a","b"],"additionalProperties":false}`, &parameters); err != nil {
		return err
	}
	err = cm.BindTools([]*schema.ToolInfo{
		{Name: "add", Desc: "Add two integers", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameters)},
		{Name: "multiply", Desc: "Multiply two integers", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameters)},
		{Name: "subtract", Desc: "Subtract b from a", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&parameters)},
	})
	if err != nil {
		return err
	}

	// Bind tools once. Send the full history on every call; session caching
	// selects the newest response ID and sends only the incremental messages.
	history := []*schema.Message{schema.UserMessage("Use add to calculate 8 + 13, then tell me the result.")}
	// Reuse these exact options for both tool calls. The first request filters out
	// subtract; continuations inherit the same definitions without resending them.
	options := []model.Option{model.WithToolChoice(schema.ToolChoiceForced, "add", "multiply")}
	reader, err := cm.Stream(ctx, history, options...)
	if err != nil {
		return err
	}
	assistant, err := schema.ConcatMessageStream(reader)
	if err != nil {
		return err
	}
	id, _ := ark.GetResponseID(assistant)
	fmt.Println("Cached tool-call response:", id)
	if len(assistant.ToolCalls) == 0 {
		return fmt.Errorf("model returned no tool calls")
	}
	history = append(history, assistant)
	for _, call := range assistant.ToolCalls {
		if call.Function.Name != "add" {
			return fmt.Errorf("unexpected tool %q", call.Function.Name)
		}
		var args struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := sonic.UnmarshalString(call.Function.Arguments, &args); err != nil {
			return err
		}
		result := args.A + args.B
		fmt.Printf("add(%d, %d) = %d\n", args.A, args.B, result)
		history = append(history, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: fmt.Sprint(result)})
	}

	// The tool result continues the cached function call. Tool definitions are
	// inherited from that cache; the adapter omits tools on this request.
	// Keep Forced and the same candidate names on this cached continuation.
	history = append(history, schema.UserMessage("Now use multiply to double the sum you just received."))
	answer, err := cm.Generate(ctx, history, options...)
	if err != nil {
		return err
	}
	id, _ = ark.GetResponseID(answer)
	fmt.Println("Cached second tool-call response:", id)
	if len(answer.ToolCalls) == 0 {
		return fmt.Errorf("model returned no tool calls")
	}
	history = append(history, answer)
	for _, call := range answer.ToolCalls {
		if call.Function.Name != "multiply" {
			return fmt.Errorf("unexpected tool %q", call.Function.Name)
		}
		var args struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := sonic.UnmarshalString(call.Function.Arguments, &args); err != nil {
			return err
		}
		result := args.A * args.B
		fmt.Printf("multiply(%d, %d) = %d\n", args.A, args.B, result)
		history = append(history, &schema.Message{Role: schema.Tool, ToolCallID: call.ID, Content: fmt.Sprint(result)})
	}
	if answer.ResponseMeta != nil && answer.ResponseMeta.Usage != nil {
		fmt.Println("Cached input tokens:", answer.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
	}

	// Cached continuations cannot send tool_choice. Start a new chain from the
	// complete history to make Forbidden take effect on the final answer.
	if err := ark.InvalidateMessageCaches(history); err != nil {
		return err
	}
	history = append(history, schema.UserMessage("What is the final result? Reply with only the number."))
	followUp, err := cm.Generate(ctx, history, model.WithToolChoice(schema.ToolChoiceForbidden))
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(followUp)
	return nil
}
