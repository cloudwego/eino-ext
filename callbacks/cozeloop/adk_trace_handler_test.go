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

package cozeloop

import (
	"context"
	"os"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/smartystreets/goconvey/convey"
)

func TestEinoTracer_OnAgentStart(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("test einoTracer OnStart with Agent component", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}
		runtime := &tracespec.Runtime{}

		tracer := &einoTracer{
			client:  client,
			parser:  &defaultDataParser{},
			runtime: runtime,
			logger:  cozeloop.GetLogger(),
		}

		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Type:      "ChatModel",
			Component: adk.ComponentOfAgent,
		}

		mockey.PatchConvey("test OnStart with AgentInput (run)", func() {
			input := &adk.AgentCallbackInput{
				Input: &adk.AgentInput{
					Messages: []*schema.Message{
						{Role: schema.User, Content: "hello"},
					},
					EnableStreaming: false,
				},
			}

			ctx := tracer.OnStart(context.Background(), info, input)
			convey.So(ctx, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("test OnStart with ResumeInfo (resume)", func() {
			input := &adk.AgentCallbackInput{
				ResumeInfo: &adk.ResumeInfo{
					EnableStreaming: true,
					ResumeData:      map[string]any{"key": "value"},
					InterruptState:  map[string]any{"state": "interrupted"},
				},
			}

			ctx := tracer.OnStart(context.Background(), info, input)
			convey.So(ctx, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("test OnStart with nil info", func() {
			input := &adk.AgentCallbackInput{
				Input: &adk.AgentInput{
					Messages: []*schema.Message{
						{Role: schema.User, Content: "hello"},
					},
				},
			}

			ctx := tracer.OnStart(context.Background(), nil, input)
			convey.So(ctx, convey.ShouldNotBeNil)
		})
	})
}

func TestEinoTracer_OnAgentEnd(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("test einoTracer OnEnd with Agent component", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}
		runtime := &tracespec.Runtime{}

		tracer := &einoTracer{
			client:  client,
			parser:  &defaultDataParser{},
			runtime: runtime,
			logger:  cozeloop.GetLogger(),
		}

		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Type:      "ChatModel",
			Component: adk.ComponentOfAgent,
		}

		mockey.PatchConvey("test OnEnd with multiple events", func() {
			ctx := tracer.OnStart(context.Background(), info, &adk.AgentCallbackInput{
				Input: &adk.AgentInput{
					Messages: []*schema.Message{{Role: schema.User, Content: "hello"}},
				},
			})

			iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
			go func() {
				defer gen.Close()
				gen.Send(&adk.AgentEvent{
					Output: &adk.AgentOutput{
						MessageOutput: &adk.MessageVariant{
							IsStreaming: false,
							Message:     &schema.Message{Role: schema.Assistant, Content: "response"},
						},
					},
				})
			}()

			output := &adk.AgentCallbackOutput{Events: iter}
			result := tracer.OnEnd(ctx, info, output)
			convey.So(result, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("test OnEnd with nil info", func() {
			ctx := context.Background()
			iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
			go func() {
				gen.Close()
			}()

			output := &adk.AgentCallbackOutput{Events: iter}
			result := tracer.OnEnd(ctx, nil, output)
			convey.So(result, convey.ShouldNotBeNil)
		})
	})
}

func TestEinoTracer_OnStartWithStreamInput_Agent(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("test einoTracer OnStartWithStreamInput with Agent component", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}
		runtime := &tracespec.Runtime{}

		tracer := &einoTracer{
			client:  client,
			parser:  &defaultDataParser{},
			runtime: runtime,
			logger:  cozeloop.GetLogger(),
		}

		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Type:      "ChatModel",
			Component: adk.ComponentOfAgent,
		}

		mockey.PatchConvey("test OnStartWithStreamInput", func() {
			reader, writer := schema.Pipe[callbacks.CallbackInput](2)
			go func() {
				defer writer.Close()
				writer.Send(&adk.AgentCallbackInput{
					Input: &adk.AgentInput{
						Messages: []*schema.Message{{Role: schema.User, Content: "hello"}},
					},
				}, nil)
			}()

			ctx := tracer.OnStartWithStreamInput(context.Background(), info, reader)
			convey.So(ctx, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("test OnStartWithStreamInput with nil info", func() {
			reader, writer := schema.Pipe[callbacks.CallbackInput](2)
			go func() {
				defer writer.Close()
			}()

			ctx := tracer.OnStartWithStreamInput(context.Background(), nil, reader)
			convey.So(ctx, convey.ShouldNotBeNil)
		})
	})
}

func TestNewTraceCallbackHandler_WithCustomParser(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("test newTraceCallbackHandler with custom parser", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}

		customParser := &mockDataParser{}

		handler := newTraceCallbackHandler(client, &options{
			parser: customParser,
		})
		convey.So(handler, convey.ShouldNotBeNil)

		tracer, ok := handler.(*einoTracer)
		convey.So(ok, convey.ShouldBeTrue)
		convey.So(tracer.parser, convey.ShouldEqual, customParser)
	})
}

func TestNewTraceCallbackHandler_WithDefaultParser(t *testing.T) {
	os.Setenv(cozeloop.EnvWorkspaceID, "1234567890")
	os.Setenv(cozeloop.EnvApiToken, "xxxx")
	mockey.PatchConvey("test newTraceCallbackHandler with default parser", t, func() {
		client, err := cozeloop.NewClient()
		if err != nil {
			return
		}

		handler := newTraceCallbackHandler(client, &options{})
		convey.So(handler, convey.ShouldNotBeNil)

		tracer, ok := handler.(*einoTracer)
		convey.So(ok, convey.ShouldBeTrue)
		convey.So(tracer.parser, convey.ShouldNotBeNil)
	})
}

type mockDataParser struct {
	defaultDataParser
}
