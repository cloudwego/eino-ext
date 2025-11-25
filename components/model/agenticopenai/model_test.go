/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	mockey.PatchConvey("TestNew", t, func() {
		mockey.PatchConvey("success", func() {
			config := &Config{
				APIKey: "test",
			}
			m, err := New(context.Background(), config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			assert.Equal(t, implType, m.GetType())
		})
		mockey.PatchConvey("config nil", func() {
			m, err := New(context.Background(), nil)
			assert.NoError(t, err)
			assert.NotNil(t, m)
		})
	})
}

func TestModelGenerate(t *testing.T) {
	mockey.PatchConvey("TestModelGenerate", t, func() {
		ctx := context.Background()
		config := &Config{APIKey: "test"}
		m, err := New(ctx, config)
		assert.NoError(t, err)

		input := []*schema.AgenticMessage{
			{Role: schema.AgenticRoleTypeUser, ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.UserInputText{Text: "hi"})}},
		}

		mockey.PatchConvey("success", func() {
			mockey.Mock((*responses.ResponseService).New).Return(&responses.Response{}, nil).Build()

			mockey.Mock(toOutputMessage).Return(&schema.AgenticMessage{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{
					schema.NewContentBlock(&schema.AssistantGenText{Text: "hello"}),
				},
			}, nil).Build()

			out, err := m.Generate(ctx, input)
			assert.NoError(t, err)
			assert.NotNil(t, out)
			if assert.NotEmpty(t, out.ContentBlocks) {
				assert.Equal(t, "hello", out.ContentBlocks[0].AssistantGenText.Text)
			}
		})

		mockey.PatchConvey("genRequest error", func() {
			invalidInput := []*schema.AgenticMessage{
				{Role: "invalid", ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.UserInputText{Text: "hi"})}},
			}
			out, err := m.Generate(ctx, invalidInput)
			assert.Error(t, err)
			assert.Nil(t, out)
			assert.Contains(t, err.Error(), "invalid role")
		})

		mockey.PatchConvey("cli.New error", func() {
			mockey.Mock((*responses.ResponseService).New).Return(nil, errors.New("api error")).Build()
			out, err := m.Generate(ctx, input)
			assert.Error(t, err)
			assert.Nil(t, out)
			assert.Contains(t, err.Error(), "api error")
		})

		mockey.PatchConvey("toOutputMessage error", func() {
			mockey.Mock((*responses.ResponseService).New).Return(&responses.Response{}, nil).Build()
			mockey.Mock(toOutputMessage).Return(nil, errors.New("convert error")).Build()

			out, err := m.Generate(ctx, input)
			assert.Error(t, err)
			assert.Nil(t, out)
		})
	})
}

func TestModelStream(t *testing.T) {
	mockey.PatchConvey("TestModelStream", t, func() {
		ctx := context.Background()
		config := &Config{APIKey: "test", Model: "gpt-4"}
		m, err := New(ctx, config)
		assert.NoError(t, err)

		input := []*schema.AgenticMessage{
			{Role: schema.AgenticRoleTypeUser, ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.UserInputText{Text: "hi"})}},
		}

		mockey.PatchConvey("success", func() {
			// Create a real stream with a mock decoder to avoid mockey ARM64 issues with generic methods
			mockStream := ssestream.NewStream[responses.ResponseStreamEventUnion](&modelTestMockDecoder{}, nil)

			mockey.Mock((*responses.ResponseService).NewStreaming).Return(mockStream).Build()

			// Mock AsAny to return a completed event
			mockey.Mock(responses.ResponseStreamEventUnion.AsAny).Return(responses.ResponseCompletedEvent{
				Response: responses.Response{
					Output: []responses.ResponseOutputItemUnion{
						{Type: "message", ID: "m1", Status: "completed"},
					},
				},
			}).Build()

			s, err := m.Stream(ctx, input)
			assert.NoError(t, err)
			assert.NotNil(t, s)
			defer s.Close()

			// The stream should eventually close without errors
			// We just verify it was created successfully
		})

		mockey.PatchConvey("genRequest error", func() {
			invalidInput := []*schema.AgenticMessage{
				{Role: "invalid", ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.UserInputText{Text: "hi"})}},
			}
			s, err := m.Stream(ctx, invalidInput)
			assert.Error(t, err)
			assert.Nil(t, s)
		})
	})
}

func TestModelWithTools(t *testing.T) {
	mockey.PatchConvey("TestModelWithTools", t, func() {
		ctx := context.Background()
		m, err := New(ctx, &Config{APIKey: "test"})
		assert.NoError(t, err)

		mockey.PatchConvey("success", func() {
			tools := []*schema.ToolInfo{
				{Name: "tool1", Desc: "d", ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{Type: "object"})},
			}
			nm, err := m.WithTools(tools)
			assert.NoError(t, err)
			assert.NotNil(t, nm)
		})

		mockey.PatchConvey("empty tools", func() {
			nm, err := m.WithTools(nil)
			assert.Error(t, err)
			assert.Nil(t, nm)
		})
	})
}

func TestModelGetType(t *testing.T) {
	mockey.PatchConvey("TestModelGetType", t, func() {
		m, err := New(context.Background(), &Config{APIKey: "test"})
		assert.NoError(t, err)
		assert.Equal(t, "OpenAI", m.GetType())
	})
}

func TestModelIsCallbacksEnabled(t *testing.T) {
	mockey.PatchConvey("TestModelIsCallbacksEnabled", t, func() {
		m, err := New(context.Background(), &Config{APIKey: "test"})
		assert.NoError(t, err)
		assert.True(t, m.IsCallbacksEnabled())
	})
}

type modelTestMockDecoder struct {
	index int
}

func (d *modelTestMockDecoder) Next() bool {
	if d.index < 1 {
		d.index++
		return true
	}
	return false
}

func (d *modelTestMockDecoder) Event() ssestream.Event {
	return ssestream.Event{
		Type: "response.completed",
		Data: []byte(`{"type":"response.completed"}`),
	}
}

func (d *modelTestMockDecoder) Close() error { return nil }

func (d *modelTestMockDecoder) Err() error { return nil }
