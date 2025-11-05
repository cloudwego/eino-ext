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

package main

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/adk/utils/runner_wrapper"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()

	handler := &mockCallback{}
	callbacks.AppendGlobalHandlers(handler)

	agent := &mockAgent{}
	r := runner_wrapper.NewRunner(ctx, adk.RunnerConfig{
		Agent: agent,
	})

	iter := r.Query(ctx, "hello")
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}
}

type mockAgent struct{}

func (m mockAgent) Name(ctx context.Context) string {
	return "mock_agent"
}

func (m mockAgent) Description(ctx context.Context) string {
	return "mock_completion"
}

func (m mockAgent) Run(ctx context.Context, input *adk.AgentInput, options ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	i, g := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	g.Send(adk.EventFromMessage(schema.AssistantMessage("bye", nil), nil, schema.Assistant, ""))
	g.Close()
	return i
}

type mockCallback struct{}

func (m *mockCallback) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	log.Printf("mockCallback.OnStart: RunInfo: %+v, Input: %+v\n", info, input)
	return ctx
}

func (m *mockCallback) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	log.Printf("mockCallback.OnEnd: RunInfo: %+v, Output: %+v\n", info, output)
	return ctx
}

func (m *mockCallback) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	log.Printf("mockCallback.OnError: RunInfo: %+v, Error: %+v\n", info, err)
	return ctx
}

func (m *mockCallback) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	input.Close()
	return ctx
}

func (m *mockCallback) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	output.Close()
	return ctx
}
