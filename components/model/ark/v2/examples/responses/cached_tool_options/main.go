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
	// Reuse these exact options on every call. The first request filters out
	// subtract; continuations inherit the same definitions without resending them.
	options := []model.Option{model.WithToolChoice(schema.ToolChoiceAllowed, "add", "multiply")}
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
		// Allowed may choose a direct answer; the cache can still be reused.
		exampleutil.PrintResponse(assistant)
	}
	history, err = appendToolResults(history, assistant)
	if err != nil {
		return err
	}

	// The tool result continues the cached function call. Tool definitions are
	// inherited from that cache; the adapter omits tools on this request.
	// Allowed permits the model to answer directly after receiving the result.
	answer, err := cm.Generate(ctx, history, options...)
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(answer)
	if answer.ResponseMeta != nil && answer.ResponseMeta.Usage != nil {
		fmt.Println("Cached input tokens:", answer.ResponseMeta.Usage.PromptTokenDetails.CachedTokens)
	}
	history, err = appendToolResults(history, answer)
	if err != nil {
		return err
	}
	history = append(history, schema.UserMessage("What were the two operands and their sum? Reply briefly."))

	// A normal next turn now continues answer's cache, rather than the older
	// tool-call response. Keeping history also permits replay after expiration.
	followUp, err := cm.Generate(ctx, history, options...)
	if err != nil {
		return err
	}
	exampleutil.PrintResponse(followUp)
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
