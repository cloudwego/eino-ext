/*
 * Copyright 2024 CloudWeGo Authors
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

package langfuse

import (
	"context"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/stretchr/testify/assert"
	"io"
	"strconv"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/libs/acl/langfuse"
	"github.com/cloudwego/eino-ext/libs/acl/langfuse/mock"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/golang/mock/gomock"
)

func TestLangfuseCallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockLangfuse := mock.NewMockLangfuse(ctrl)
	cbh := NewLangfuseHandler(mockLangfuse,
		WithName("MyTrace"),
		WithPublic(true),
		WithRelease("release"),
		WithUserID("user id"),
		WithSessionID("session"),
		WithTags([]string{"tag1", "tag2"}),
	)
	callbacks.InitCallbackHandlers([]callbacks.Handler{cbh})
	ctx := context.Background()

	g := compose.NewGraph[string, string]()
	err := g.AddLambdaNode("node1", compose.InvokableLambda(func(ctx context.Context, input string) (output string, err error) {
		return input, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	err = g.AddLambdaNode("node2", compose.StreamableLambda(func(ctx context.Context, input string) (output *schema.StreamReader[string], err error) {
		sr, sw := schema.Pipe[string](10)
		for i := 0; i < 10; i++ {
			sw.Send(input, nil)
		}
		sw.Close()
		return sr, nil
	}))
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
		mockLangfuse.EXPECT().CreateTrace(gomock.Any()).Return("trace id", nil).Times(2)
		createSpanTimes := 0
		mockLangfuse.EXPECT().CreateSpan(gomock.Any()).DoAndReturn(func(body *langfuse.SpanEventBody) (string, error) {
			defer func() {
				createSpanTimes++
			}()
			switch createSpanTimes {
			case 0:
				if body.TraceID != "trace id" {
					t.Fatalf("expect trace id, but got %s", body.TraceID)
				}
				if body.Input != "\"input\"" {
					t.Fatalf("expect input, but got %s", body.Input)
				}
				if len(body.ParentObservationID) != 0 {
					t.Fatalf("expect empty parentObservationID, but got %s", body.ParentObservationID)
				}
				return "graph span id", nil
			case 3:
				if body.TraceID != "trace id" {
					t.Fatalf("expect trace id, but got %s", body.TraceID)
				}
				if body.Input != "[\"input\"]" {
					t.Fatalf("expect input, but got %s", body.Input)
				}
				if len(body.ParentObservationID) != 0 {
					t.Fatalf("expect empty parentObservationID, but got %s", body.ParentObservationID)
				}
				return "graph span id", nil
			case 1, 2:
				if body.TraceID != "trace id" {
					t.Fatalf("expect trace id, but got %s", body.TraceID)
				}
				if body.Input != "\"input\"" {
					t.Fatalf("expect input, but got %s", body.Input)
				}
				if body.ParentObservationID != "graph span id" {
					t.Fatalf("expect graph span id, but got %s", body.ParentObservationID)
				}
				return "node span id " + strconv.Itoa(createSpanTimes), nil
			case 4, 5:
				if body.TraceID != "trace id" {
					t.Fatalf("expect trace id, but got %s", body.TraceID)
				}
				if body.Input != "\"input\"" {
					t.Fatalf("expect input, but got %s", body.Input)
				}
				if body.ParentObservationID != "graph span id" {
					t.Fatalf("expect graph span id, but got %s", body.ParentObservationID)
				}
				return "node span id " + strconv.Itoa(createSpanTimes-3), nil
			default:
				t.Fatalf("expect createSpanTimes, but got %d", createSpanTimes)
			}
			return "", nil
		}).Times(6)
		endSpanTimes := 0
		mockLangfuse.EXPECT().EndSpan(gomock.Any()).DoAndReturn(func(body *langfuse.SpanEventBody) error {
			defer func() {
				endSpanTimes++
			}()
			switch endSpanTimes {
			case 0, 3:
				if body.ID != "node span id 1" {
					t.Fatalf("expect node span id 1, but got %s", body.ID)
				}
				if body.Output != "\"input\"" {
					t.Fatalf("expect input, but got %s", body.Output)
				}
			case 1, 4:
				if body.ID != "node span id 2" {
					t.Fatalf("expect node span id 2, but got %s", body.ID)
				}
				if body.Output != "[\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\"]" {
					t.Fatalf("expect input, but got %s", body.Input)
				}
			case 2:
				if body.ID != "graph span id" {
					t.Fatalf("expect graph span id, but got %s", body.ID)
				}
				if body.Output != "\"inputinputinputinputinputinputinputinputinputinput\"" {
					t.Fatalf("expect input, but got %s", body.Output)
				}
			case 5:
				if body.ID != "graph span id" {
					t.Fatalf("expect graph span id, but got %s", body.ID)
				}
				if body.Output != "[\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\",\"input\"]" {
					t.Fatalf("expect input, but got %s", body.Output)
				}
			default:
				t.Fatalf("expect endSpanTimes, but got %d", endSpanTimes)
			}
			return nil
		}).Times(6)

		result, err_ := runner.Invoke(ctx, "input")
		if err_ != nil {
			t.Fatal(err_)
		}
		if result != "inputinputinputinputinputinputinputinputinputinput" {
			t.Fatalf("expect input, but got %s", result)
		}

		streamResult, err_ := runner.Stream(ctx, "input")
		if err_ != nil {
			t.Fatal(err_)
		}
		result = ""
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
		mockLangfuse.EXPECT().CreateTrace(gomock.Any()).Return("trace id", nil).Times(2)
		mockLangfuse.EXPECT().CreateGeneration(gomock.Any()).DoAndReturn(func(body *langfuse.GenerationEventBody) (string, error) {
			assert.Equal(t, body.MetaData, map[string]any{"key": "value"})
			assert.Equal(t, body.InMessages, []*schema.Message{
				{Role: schema.System, Content: "system message"}, {Role: schema.User, Content: "user message"},
			})
			assert.Equal(t, body.Model, "model")
			assert.Equal(t, body.ModelParameters.(*model.Config), &model.Config{
				Model: "model", MaxTokens: 1, Temperature: 2, TopP: 3, Stop: []string{"stop"},
			})
			return "generation id", nil
		}).Times(2)
		mockLangfuse.EXPECT().EndGeneration(gomock.Any()).DoAndReturn(func(body *langfuse.GenerationEventBody) error {
			assert.Equal(t, body.ID, "generation id")
			assert.Equal(t, body.OutMessage, &schema.Message{Role: schema.Assistant, Content: "assistant message"})
			assert.Equal(t, body.Usage, &langfuse.Usage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			})
			return nil
		}).Times(2)

		ctx1 := cbh.OnStart(ctx, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackInput{
			Messages: []*schema.Message{{Role: schema.System, Content: "system message"}, {Role: schema.User, Content: "user message"}},
			Config: &model.Config{
				Model: "model", MaxTokens: 1, Temperature: 2, TopP: 3, Stop: []string{"stop"},
			},
			Extra: map[string]interface{}{"key": "value"},
		})
		cbh.OnEnd(ctx1, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, &model.CallbackOutput{
			Message: &schema.Message{Role: schema.Assistant, Content: "assistant message"},
			TokenUsage: &model.TokenUsage{
				PromptTokens:     1,
				CompletionTokens: 2,
				TotalTokens:      3,
			},
		})

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
		cbh.OnEndWithStreamOutput(ctx2, &callbacks.RunInfo{Component: components.ComponentOfChatModel}, outsr)
	})
}