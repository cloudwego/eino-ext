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
	"errors"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/smartystreets/goconvey/convey"
)

func TestDefaultDataParser_ParseInput_Agent_Run(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseInput_Agent_Run", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Type:      "ChatModel",
			Component: adk.ComponentOfAgent,
		}

		input := &adk.AgentCallbackInput{
			Input: &adk.AgentInput{
				Messages: []*schema.Message{
					{Role: schema.User, Content: "hello"},
				},
				EnableStreaming: true,
			},
		}

		tags := parser.ParseInput(ctx, info, input)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[attrKeyRunMode], convey.ShouldEqual, "run")
		convey.So(tags[attrKeyAgentName], convey.ShouldEqual, "test-agent")
		convey.So(tags[tracespec.Stream], convey.ShouldEqual, true)
		convey.So(tags[tracespec.Input], convey.ShouldNotBeNil)
	})
}

func TestDefaultDataParser_ParseInput_Agent_Resume(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseInput_Agent_Resume", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Type:      "ChatModel",
			Component: adk.ComponentOfAgent,
		}

		input := &adk.AgentCallbackInput{
			ResumeInfo: &adk.ResumeInfo{
				EnableStreaming: false,
				ResumeData:      map[string]any{"key": "value"},
				InterruptState:  map[string]any{"state": "interrupted"},
			},
		}

		tags := parser.ParseInput(ctx, info, input)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[attrKeyRunMode], convey.ShouldEqual, "resume")
		convey.So(tags[attrKeyAgentName], convey.ShouldEqual, "test-agent")
		convey.So(tags[tracespec.Stream], convey.ShouldEqual, false)
		convey.So(tags[tracespec.Input], convey.ShouldNotBeNil)
	})
}

func TestDefaultDataParser_ParseInput_Agent_NilInput(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseInput_Agent_NilInput", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()

		convey.Convey("nil info", func() {
			tags := parser.ParseInput(ctx, nil, &adk.AgentCallbackInput{})
			convey.So(tags, convey.ShouldBeNil)
		})
	})
}

func TestDefaultDataParser_ParseOutput_Agent_MultipleEvents(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_MultipleEvents", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Component: adk.ComponentOfAgent,
		}

		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						IsStreaming: false,
						Message:     &schema.Message{Role: schema.Assistant, Content: "first"},
					},
				},
			})
			gen.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						IsStreaming: false,
						Message:     &schema.Message{Role: schema.Assistant, Content: "second"},
					},
				},
			})
		}()

		output := &adk.AgentCallbackOutput{Events: iter}
		tags := parser.ParseOutput(ctx, info, output)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
	})
}

func TestDefaultDataParser_ParseOutput_Agent_WithError(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_WithError", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Component: adk.ComponentOfAgent,
		}

		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(&adk.AgentEvent{
				Err: errors.New("test error"),
			})
		}()

		output := &adk.AgentCallbackOutput{Events: iter}
		tags := parser.ParseOutput(ctx, info, output)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
	})
}

func TestDefaultDataParser_ParseOutput_Agent_NilOutput(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_NilOutput", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()

		convey.Convey("nil info", func() {
			tags := parser.ParseOutput(ctx, nil, &adk.AgentCallbackOutput{})
			convey.So(tags, convey.ShouldBeNil)
		})

		convey.Convey("nil events", func() {
			info := &callbacks.RunInfo{Name: "test", Component: adk.ComponentOfAgent}
			tags := parser.ParseOutput(ctx, info, &adk.AgentCallbackOutput{Events: nil})
			convey.So(tags, convey.ShouldNotBeNil)
			convey.So(tags[tracespec.Output], convey.ShouldBeNil)
		})
	})
}

func TestDefaultDataParser_ParseOutput_Agent_StreamingMessage(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_StreamingMessage", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Component: adk.ComponentOfAgent,
		}

		msgReader, msgWriter := schema.Pipe[*schema.Message](3)
		go func() {
			defer msgWriter.Close()
			msgWriter.Send(&schema.Message{Role: schema.Assistant, Content: "hello"}, nil)
			msgWriter.Send(&schema.Message{Role: schema.Assistant, Content: "world"}, nil)
		}()

		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						IsStreaming:   true,
						MessageStream: msgReader,
					},
				},
			})
		}()

		output := &adk.AgentCallbackOutput{Events: iter}
		tags := parser.ParseOutput(ctx, info, output)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
	})
}

func TestDefaultDataParser_ParseOutput_Agent_WithAction(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_WithAction", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Component: adk.ComponentOfAgent,
		}

		convey.Convey("exit action", func() {
			iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
			go func() {
				defer gen.Close()
				gen.Send(&adk.AgentEvent{
					Action: &adk.AgentAction{
						Exit: true,
					},
				})
			}()

			output := &adk.AgentCallbackOutput{Events: iter}
			tags := parser.ParseOutput(ctx, info, output)
			convey.So(tags, convey.ShouldNotBeNil)
			convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
		})

		convey.Convey("transfer to agent action", func() {
			iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
			go func() {
				defer gen.Close()
				gen.Send(&adk.AgentEvent{
					Action: &adk.AgentAction{
						TransferToAgent: &adk.TransferToAgentAction{
							DestAgentName: "other-agent",
						},
					},
				})
			}()

			output := &adk.AgentCallbackOutput{Events: iter}
			tags := parser.ParseOutput(ctx, info, output)
			convey.So(tags, convey.ShouldNotBeNil)
			convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
		})

		convey.Convey("break loop action", func() {
			iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
			go func() {
				defer gen.Close()
				gen.Send(&adk.AgentEvent{
					Action: &adk.AgentAction{
						BreakLoop: &adk.BreakLoopAction{},
					},
				})
			}()

			output := &adk.AgentCallbackOutput{Events: iter}
			tags := parser.ParseOutput(ctx, info, output)
			convey.So(tags, convey.ShouldNotBeNil)
			convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
		})

		convey.Convey("customized action", func() {
			iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
			go func() {
				defer gen.Close()
				gen.Send(&adk.AgentEvent{
					Action: &adk.AgentAction{
						CustomizedAction: map[string]any{"custom": "action"},
					},
				})
			}()

			output := &adk.AgentCallbackOutput{Events: iter}
			tags := parser.ParseOutput(ctx, info, output)
			convey.So(tags, convey.ShouldNotBeNil)
			convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
		})
	})
}

func TestDefaultDataParser_ParseOutput_Agent_WithInterrupt(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_WithInterrupt", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Component: adk.ComponentOfAgent,
		}

		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(&adk.AgentEvent{
				Action: &adk.AgentAction{
					Interrupted: &adk.InterruptInfo{
						Data: map[string]any{"interrupt": "data"},
						InterruptContexts: []*adk.InterruptCtx{
							{
								ID:          "ctx-1",
								Info:        map[string]any{"info": "data"},
								IsRootCause: true,
							},
						},
					},
				},
			})
		}()

		output := &adk.AgentCallbackOutput{Events: iter}
		tags := parser.ParseOutput(ctx, info, output)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
	})
}

func TestDefaultDataParser_ParseOutput_Agent_WithAgentName(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_WithAgentName", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Component: adk.ComponentOfAgent,
		}

		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(&adk.AgentEvent{
				AgentName: "nested-agent",
				Output: &adk.AgentOutput{
					MessageOutput: &adk.MessageVariant{
						IsStreaming: false,
						Message:     &schema.Message{Role: schema.Assistant, Content: "nested output"},
					},
				},
			})
		}()

		output := &adk.AgentCallbackOutput{Events: iter}
		tags := parser.ParseOutput(ctx, info, output)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
	})
}

func TestDefaultDataParser_ParseOutput_Agent_WithCustomizedOutput(t *testing.T) {
	convey.Convey("TestDefaultDataParser_ParseOutput_Agent_WithCustomizedOutput", t, func() {
		parser := &defaultDataParser{}
		ctx := context.Background()
		info := &callbacks.RunInfo{
			Name:      "test-agent",
			Component: adk.ComponentOfAgent,
		}

		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(&adk.AgentEvent{
				Output: &adk.AgentOutput{
					CustomizedOutput: map[string]any{"custom": "output"},
				},
			})
		}()

		output := &adk.AgentCallbackOutput{Events: iter}
		tags := parser.ParseOutput(ctx, info, output)
		convey.So(tags, convey.ShouldNotBeNil)
		convey.So(tags[tracespec.Output], convey.ShouldNotBeNil)
	})
}

func TestParseSpanTypeFromComponent_Agent(t *testing.T) {
	convey.Convey("TestParseSpanTypeFromComponent_Agent", t, func() {
		convey.So(parseSpanTypeFromComponent(adk.ComponentOfAgent), convey.ShouldEqual, spanTypeAgent)
	})
}

func TestSerializeRunPath(t *testing.T) {
	convey.Convey("TestSerializeRunPath", t, func() {
		convey.Convey("empty run path", func() {
			result := serializeRunPath(nil)
			convey.So(result, convey.ShouldEqual, "")
		})

		convey.Convey("empty slice", func() {
			result := serializeRunPath([]adk.RunStep{})
			convey.So(result, convey.ShouldEqual, "")
		})
	})
}

func TestSerializeAgentAction(t *testing.T) {
	convey.Convey("TestSerializeAgentAction", t, func() {
		convey.Convey("nil action", func() {
			result := serializeAgentAction(nil)
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("exit action", func() {
			action := &adk.AgentAction{Exit: true}
			result := serializeAgentAction(action)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["exit"], convey.ShouldEqual, true)
		})

		convey.Convey("transfer to agent action", func() {
			action := &adk.AgentAction{
				TransferToAgent: &adk.TransferToAgentAction{DestAgentName: "target-agent"},
			}
			result := serializeAgentAction(action)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["transfer_to_agent"], convey.ShouldEqual, "target-agent")
		})

		convey.Convey("break loop action", func() {
			action := &adk.AgentAction{BreakLoop: &adk.BreakLoopAction{}}
			result := serializeAgentAction(action)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["break_loop"], convey.ShouldEqual, true)
		})

		convey.Convey("customized action", func() {
			action := &adk.AgentAction{CustomizedAction: map[string]any{"key": "value"}}
			result := serializeAgentAction(action)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["customized_action"], convey.ShouldNotBeNil)
		})
	})
}

func TestSerializeInterruptInfo(t *testing.T) {
	convey.Convey("TestSerializeInterruptInfo", t, func() {
		convey.Convey("nil info", func() {
			result := serializeInterruptInfo(nil)
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("with data", func() {
			info := &adk.InterruptInfo{
				Data: map[string]any{"key": "value"},
			}
			result := serializeInterruptInfo(info)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["data"], convey.ShouldNotBeNil)
		})

		convey.Convey("with byte data (should be skipped)", func() {
			info := &adk.InterruptInfo{
				Data: []byte("binary data"),
			}
			result := serializeInterruptInfo(info)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["data"], convey.ShouldBeNil)
		})

		convey.Convey("with interrupt contexts", func() {
			info := &adk.InterruptInfo{
				InterruptContexts: []*adk.InterruptCtx{
					{ID: "ctx-1", IsRootCause: true},
					{ID: "ctx-2"},
				},
			}
			result := serializeInterruptInfo(info)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["interrupt_contexts"], convey.ShouldNotBeNil)
		})
	})
}

func TestSerializeInterruptCtx(t *testing.T) {
	convey.Convey("TestSerializeInterruptCtx", t, func() {
		convey.Convey("nil ctx", func() {
			result := serializeInterruptCtx(nil)
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("with all fields", func() {
			ctx := &adk.InterruptCtx{
				ID:          "ctx-1",
				Info:        map[string]any{"key": "value"},
				IsRootCause: true,
				Parent: &adk.InterruptCtx{
					ID: "parent-ctx",
				},
			}
			result := serializeInterruptCtx(ctx)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["id"], convey.ShouldEqual, "ctx-1")
			convey.So(result["info"], convey.ShouldNotBeNil)
			convey.So(result["is_root_cause"], convey.ShouldEqual, true)
			convey.So(result["parent"], convey.ShouldNotBeNil)
		})

		convey.Convey("with non-empty id", func() {
			ctx := &adk.InterruptCtx{
				ID: "ctx-1",
			}
			result := serializeInterruptCtx(ctx)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["id"], convey.ShouldEqual, "ctx-1")
		})
	})
}

func TestSerializeAgentEvent(t *testing.T) {
	convey.Convey("TestSerializeAgentEvent", t, func() {
		ctx := context.Background()

		convey.Convey("nil event", func() {
			result := serializeAgentEvent(ctx, nil)
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("empty event", func() {
			event := &adk.AgentEvent{}
			result := serializeAgentEvent(ctx, event)
			convey.So(result, convey.ShouldBeNil)
		})

		convey.Convey("event with agent name", func() {
			event := &adk.AgentEvent{AgentName: "test-agent"}
			result := serializeAgentEvent(ctx, event)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["agent_name"], convey.ShouldEqual, "test-agent")
		})

		convey.Convey("event with error", func() {
			event := &adk.AgentEvent{Err: errors.New("test error")}
			result := serializeAgentEvent(ctx, event)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result["error"], convey.ShouldEqual, "test error")
		})
	})
}
