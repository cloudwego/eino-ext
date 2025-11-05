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

package runner_wrapper

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestAgentRunner(t *testing.T) {
	ctx := context.Background()

	handler := &cbHandler{}
	callbacks.AppendGlobalHandlers(handler)
	msgs := []*schema.Message{
		{Role: schema.Assistant, Content: "h"},
		{Content: "e"},
		{Content: "l"},
		{Content: "l"},
		{Content: "o"},
	}

	t.Run("test message success", func(t *testing.T) {
		defer handler.reset()
		agent := &mockAgent{msgs: msgs}
		runner := NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: false,
		})
		iterator := runner.Query(ctx, "bye")
		idx := 0
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			assert.Equal(t, msgs[idx], event.Output.MessageOutput.Message)
			idx++
		}
		assert.Equal(t, 1, handler.start)
		assert.Equal(t, 1, handler.end)
	})

	t.Run("test stream message success", func(t *testing.T) {
		defer handler.reset()
		agent := &mockAgent{msgs: msgs, stream: true}
		runner := NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
		})
		iterator := runner.Query(ctx, "bye")
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			assert.NotNil(t, event.Output.MessageOutput.MessageStream)
			var chunks []adk.Message
			for {
				chunk, err := event.Output.MessageOutput.MessageStream.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					assert.NoError(t, err)
				}
				chunks = append(chunks, chunk)
			}
			msg, err := schema.ConcatMessages(chunks)
			assert.NoError(t, err)
			assert.Equal(t, "hello", msg.Content)
		}
		assert.Equal(t, 1, handler.start)
		assert.Equal(t, 1, handler.end)
	})

	t.Run("test interrupted", func(t *testing.T) {
		defer handler.reset()
		agent := &mockAgent{msgs: msgs, interrupt: true}
		runner := NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: false,
		})
		iterator := runner.Query(ctx, "bye")
		idx := 0
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			if idx < 5 {
				assert.Equal(t, msgs[idx], event.Output.MessageOutput.Message)
			} else {
				assert.NotNil(t, event.Action.Interrupted)
			}
			idx++
		}
		assert.Equal(t, 1, handler.start)
		assert.Equal(t, 1, handler.end)
	})

	t.Run("test resume", func(t *testing.T) {
		defer handler.reset()
		agent := &mockAgent{msgs: msgs, interrupt: true}
		runner := NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: false,
			CheckPointStore: &inMemoryStore{make(map[string][]byte)},
		})
		iterator := runner.Query(ctx, "bye", adk.WithCheckPointID("1"))
		idx := 0
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			if idx < 5 {
				assert.Equal(t, msgs[idx], event.Output.MessageOutput.Message)
			} else {
				assert.NotNil(t, event.Action.Interrupted)
			}
			idx++
		}
		assert.Equal(t, 1, handler.start)
		assert.Equal(t, 1, handler.end)

		iterator, err := runner.Resume(ctx, "1")
		assert.NoError(t, err)
		idx = 0
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			assert.Equal(t, msgs[idx], event.Output.MessageOutput.Message)
			idx++
		}
		assert.Equal(t, 2, handler.start)
		assert.Equal(t, 2, handler.end)
	})

	t.Run("test run error", func(t *testing.T) {
		defer handler.reset()
		agent := &mockAgent{msgs: msgs, err: fmt.Errorf("mock err")}
		runner := NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: false,
			CheckPointStore: &inMemoryStore{make(map[string][]byte)},
		})
		iterator := runner.Query(ctx, "bye", adk.WithCheckPointID("1"))
		idx := 0
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			if idx < 5 {
				assert.Equal(t, msgs[idx], event.Output.MessageOutput.Message)
			} else {
				assert.NotNil(t, event.Err)
			}
			idx++
		}
		assert.Equal(t, 1, handler.start)
		assert.Equal(t, 1, handler.err)
	})

	t.Run("test resume error", func(t *testing.T) {
		defer handler.reset()
		agent := &mockAgent{msgs: msgs, interrupt: true}
		runner := NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: false,
			CheckPointStore: &inMemoryStore{make(map[string][]byte)},
		})
		iterator := runner.Query(ctx, "bye", adk.WithCheckPointID("1"))
		idx := 0
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			if idx < 5 {
				assert.Equal(t, msgs[idx], event.Output.MessageOutput.Message)
			} else {
				assert.NotNil(t, event.Action.Interrupted)
			}
			idx++
		}
		assert.Equal(t, 1, handler.start)
		assert.Equal(t, 1, handler.end)

		agent.err = fmt.Errorf("mock err")
		iterator, err := runner.Resume(ctx, "1")
		assert.NoError(t, err)
		idx = 0
		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}
			if idx < 5 {
				assert.Equal(t, msgs[idx], event.Output.MessageOutput.Message)
			} else {
				assert.NotNil(t, event.Err)
			}
			idx++
		}
		assert.Equal(t, 2, handler.start)
		assert.Equal(t, 1, handler.end)
		assert.Equal(t, 1, handler.err)
	})
}

type mockAgent struct {
	msgs      []*schema.Message
	stream    bool
	interrupt bool
	err       error
}

func (m *mockAgent) Name(ctx context.Context) string {
	return "mock_agent"
}

func (m *mockAgent) Description(ctx context.Context) string {
	return "mock_description"
}

func (m *mockAgent) Run(ctx context.Context, input *adk.AgentInput, options ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	i, g := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer g.Close()

		if m.stream {
			sr, sw := schema.Pipe[adk.Message](5)
			for idx := range m.msgs {
				sw.Send(m.msgs[idx], nil)
			}
			sw.Close()
			g.Send(adk.EventFromMessage(nil, sr, schema.Assistant, ""))
		} else {
			for idx := range m.msgs {
				g.Send(adk.EventFromMessage(m.msgs[idx], nil, schema.Assistant, ""))
			}
		}

		if m.err != nil {
			g.Send(&adk.AgentEvent{
				Err: m.err,
			})
			m.err = nil
		} else if m.interrupt {
			g.Send(&adk.AgentEvent{
				Action: &adk.AgentAction{
					Interrupted: &adk.InterruptInfo{
						Data: "mock_data",
					},
				},
			})
			m.interrupt = false
		}
	}()

	return i
}

func (m *mockAgent) Resume(ctx context.Context, info *adk.ResumeInfo, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return m.Run(ctx, &adk.AgentInput{}, opts...)
}

type cbHandler struct {
	start, end, err int
}

func (c *cbHandler) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	c.start++
	return ctx
}

func (c *cbHandler) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	c.end++
	return ctx
}

func (c *cbHandler) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	c.err++
	return ctx
}

func (c *cbHandler) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	return ctx
}

func (c *cbHandler) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	return ctx
}

func (c *cbHandler) reset() {
	c.start = 0
	c.err = 0
	c.end = 0
}

type inMemoryStore struct {
	m map[string][]byte
}

func (i *inMemoryStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	v, ok := i.m[checkPointID]
	return v, ok, nil
}

func (i *inMemoryStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	i.m[checkPointID] = checkPoint
	return nil
}
