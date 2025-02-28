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

package apmplus

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func TestApmplusCallback(t *testing.T) {
	cbh, _, _ := NewApmplusHandler(&Config{
		Host:        "apmplus host",
		AppKey:      "app key",
		ServiceName: "MyService",
		Release:     "release",
	})
	callbacks.InitCallbackHandlers([]callbacks.Handler{cbh})
	ctx := context.Background()

	g := compose.NewGraph[string, string]()
	err := g.AddLambdaNode("node1", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	}), compose.WithNodeName("node1"))
	if err != nil {
		t.Fatal(err)
	}
	err = g.AddLambdaNode("node2", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		sb := strings.Builder{}
		for i := 0; i < 10; i++ {
			sb.WriteString(input)
		}
		return sb.String(), nil
	}), compose.WithNodeName("node2"))
	if err != nil {
		t.Fatal(err)
	}
	err = g.AddEdge(compose.START, "node1")
	if err != nil {
		t.Fatal(err)
	}
	err = g.AddEdge("node1", "node2")
	if err != nil {
		t.Fatal(err)
	}
	err = g.AddEdge("node2", compose.END)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := g.Compile(ctx)
	if err != nil {
		t.Fatal(err)
	}

	mockey.PatchConvey("test span", t, func() {
		result, err_ := runner.Invoke(ctx, "input")
		if err_ != nil {
			t.Fatal(err_)
		}
		if result != "inputinputinputinputinputinputinputinputinputinput" {
			t.Fatalf("expect input, but got %s", result)
		}
	})

	mockey.PatchConvey("test span stream", t, func() {
		streamResult, err_ := runner.Stream(ctx, "input")
		if err_ != nil {
			t.Fatal(err_)
		}
		result := ""
		for {
			chunk, err__ := streamResult.Recv()
			if err__ == io.EOF {
				break
			}
			if err__ != nil {
				t.Fatal(err_)
			}
			result += chunk
		}
		if result != "inputinputinputinputinputinputinputinputinputinput" {
			t.Fatalf("expect input, but got %s", result)
		}
	})

	mockey.PatchConvey("test generation", t, func() {
		ctx1 := cbh.OnStart(ctx, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackInput{
			Messages: []*schema.Message{{Role: schema.System, Content: "system message"}, {Role: schema.User, Content: "user message"}},
			Config: &model.Config{
				Model: "model", MaxTokens: 1, Temperature: 2, TopP: 3, Stop: []string{"stop"},
			},
			Extra: map[string]interface{}{"key": "value"},
		})
		_, ok := ctx1.Value(apmplusStateKey{}).(*apmplusState)
		if !ok {
			t.Fatal("except state exists")
		}
		cbh.OnEnd(ctx1, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackOutput{
			Message: &schema.Message{Role: schema.Assistant, Content: "assistant message"},
			TokenUsage: &model.TokenUsage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			},
		})
	})

	mockey.PatchConvey("test generation stream", t, func() {
		insr, insw := schema.Pipe[callbacks.CallbackInput](3)
		insw.Send(&model.CallbackInput{
			Messages: []*schema.Message{{Role: schema.System, Content: "system "}, {Role: schema.User, Content: ""}},
		}, nil)
		insw.Send(&model.CallbackInput{
			Messages: []*schema.Message{{Role: schema.System, Content: "message"}, {Role: schema.User, Content: "user "}},
			Config: &model.Config{
				Model: "model", MaxTokens: 1, Temperature: 2, TopP: 3, Stop: []string{"stop"},
			},
			Extra: map[string]interface{}{"key": "value"},
		}, nil)
		insw.Send(&model.CallbackInput{
			Messages: []*schema.Message{{Role: schema.System, Content: ""}, {Role: schema.User, Content: "message"}},
		}, nil)
		insw.Close()
		outsr, outsw := schema.Pipe[callbacks.CallbackOutput](3)
		outsw.Send(&model.CallbackOutput{
			Message: &schema.Message{Role: schema.Assistant, Content: "assistant"},
		}, nil)
		outsw.Send(&model.CallbackOutput{
			Message: &schema.Message{Role: schema.Assistant, Content: " "},
			TokenUsage: &model.TokenUsage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			},
		}, nil)
		outsw.Send(&model.CallbackOutput{
			Message: &schema.Message{Role: schema.Assistant, Content: "message"},
		}, nil)
		outsw.Close()
		ctx2 := cbh.OnStartWithStreamInput(ctx, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, insr)
		_, ok := ctx2.Value(apmplusStateKey{}).(*apmplusState)
		if !ok {
			t.Fatal("except state exists")
		}
		cbh.OnEndWithStreamOutput(ctx2, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, outsr)
	})
}
