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

package langsmith

import (
	"context"
	"sync"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestRunInfoToName(t *testing.T) {
	tests := []struct {
		name     string
		info     *callbacks.RunInfo
		expected string
	}{
		{
			name: "with name",
			info: &callbacks.RunInfo{
				Name: "test-name",
				Type: "test-type",
			},
			expected: "test-name",
		},
		{
			name: "without name",
			info: &callbacks.RunInfo{
				Type:      "test-type",
				Component: components.ComponentOfChatModel,
			},
			expected: "test-type" + string(components.ComponentOfChatModel),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runInfoToName(tt.info)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRunInfoToRunType(t *testing.T) {
	tests := []struct {
		name     string
		info     *callbacks.RunInfo
		expected RunType
	}{
		{
			name: "chat model",
			info: &callbacks.RunInfo{
				Component: components.ComponentOfChatModel,
			},
			expected: RunTypeLLM,
		},
		{
			name: "tool",
			info: &callbacks.RunInfo{
				Component: components.ComponentOfTool,
			},
			expected: RunTypeTool,
		},
		{
			name: "chain",
			info: &callbacks.RunInfo{
				Component: components.ComponentOfEmbedding,
			},
			expected: RunTypeChain,
		},
		{
			name: "unknown component",
			info: &callbacks.RunInfo{
				Component: "unknown",
			},
			expected: RunTypeChain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runInfoToRunType(tt.info)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvModelCallbackInput(t *testing.T) {
	tests := []struct {
		name     string
		input    []callbacks.CallbackInput
		expected []*model.CallbackInput
	}{
		{
			name:     "empty input",
			input:    []callbacks.CallbackInput{},
			expected: []*model.CallbackInput{},
		},
		{
			name: "with valid input",
			input: []callbacks.CallbackInput{
				&model.CallbackInput{
					Messages: []*schema.Message{
						{Content: "test"},
					},
				},
			},
			expected: []*model.CallbackInput{
				{
					Messages: []*schema.Message{
						{Content: "test"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convModelCallbackInput(tt.input)
			assert.Equal(t, len(tt.expected), len(result))
			if len(tt.expected) > 0 {
				assert.Equal(t, tt.expected[0].Messages[0].Content, result[0].Messages[0].Content)
			}
		})
	}
}

func TestExtractModelInput(t *testing.T) {
	tests := []struct {
		name         string
		inputs       []*model.CallbackInput
		expectConfig *model.Config
		expectMsgs   []*schema.Message
		expectExtra  map[string]interface{}
		expectErr    bool
	}{
		{
			name:         "empty inputs",
			inputs:       []*model.CallbackInput{},
			expectConfig: nil,
			expectMsgs:   []*schema.Message{},
			expectExtra:  nil,
			expectErr:    false,
		},
		{
			name: "valid input with messages",
			inputs: []*model.CallbackInput{
				{
					Config: &model.Config{Model: "gpt-3.5"},
					Messages: []*schema.Message{
						{Content: "hello"},
					},
					Extra: map[string]interface{}{"key": "value"},
				},
			},
			expectConfig: &model.Config{Model: "gpt-3.5"},
			expectMsgs:   []*schema.Message{{Content: "hello"}},
			expectExtra:  map[string]interface{}{"key": "value"},
			expectErr:    false,
		},
		{
			name: "multiple messages",
			inputs: []*model.CallbackInput{
				{
					Messages: []*schema.Message{{Content: "msg1"}},
				},
				{
					Messages: []*schema.Message{{Content: "msg2"}},
				},
			},
			expectConfig: nil,
			expectMsgs:   []*schema.Message{{Content: "msg1"}, {Content: "msg2"}},
			expectExtra:  nil,
			expectErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, msgs, extra, err := extractModelInput(tt.inputs)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectConfig, config)
				if len(tt.expectMsgs) > 0 {
					assert.Equal(t, 1, len(msgs))
				} else {
					assert.Equal(t, 0, len(msgs))
				}
				assert.Equal(t, tt.expectExtra, extra)
			}
		})
	}
}

func TestConvModelCallbackOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   []callbacks.CallbackOutput
		expected []*model.CallbackOutput
	}{
		{
			name:     "empty output",
			output:   []callbacks.CallbackOutput{},
			expected: []*model.CallbackOutput{},
		},
		{
			name: "with valid output",
			output: []callbacks.CallbackOutput{
				&model.CallbackOutput{
					Message: &schema.Message{Content: "response"},
				},
			},
			expected: []*model.CallbackOutput{
				{
					Message: &schema.Message{Content: "response"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convModelCallbackOutput(tt.output)
			assert.Equal(t, len(tt.expected), len(result))
			if len(tt.expected) > 0 {
				assert.Equal(t, tt.expected[0].Message.Content, result[0].Message.Content)
			}
		})
	}
}

func TestExtractModelOutput(t *testing.T) {
	tests := []struct {
		name        string
		outputs     []*model.CallbackOutput
		expectUsage *model.TokenUsage
		expectMsg   *schema.Message
		expectExtra map[string]interface{}
		expectErr   bool
	}{
		{
			name:        "empty outputs",
			outputs:     []*model.CallbackOutput{},
			expectUsage: nil,
			expectMsg:   &schema.Message{},
			expectExtra: nil,
			expectErr:   false,
		},
		{
			name: "valid output with message",
			outputs: []*model.CallbackOutput{
				{
					TokenUsage: &model.TokenUsage{TotalTokens: 100},
					Message:    &schema.Message{Content: "response"},
					Extra:      map[string]interface{}{"key": "value"},
				},
			},
			expectUsage: &model.TokenUsage{TotalTokens: 100},
			expectMsg:   &schema.Message{Content: "response"},
			expectExtra: map[string]interface{}{"key": "value"},
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, msg, extra, err := extractModelOutput(tt.outputs)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectUsage, usage)
				assert.Equal(t, tt.expectMsg.Content, msg.Content)
				assert.Equal(t, tt.expectExtra, extra)
			}
		})
	}
}

func TestConcatMessageArray(t *testing.T) {
	tests := []struct {
		name      string
		mas       [][]*schema.Message
		expected  []*schema.Message
		expectErr bool
	}{
		{
			name:      "empty arrays",
			mas:       [][]*schema.Message{},
			expected:  []*schema.Message{},
			expectErr: false,
		},
		{
			name: "single array",
			mas: [][]*schema.Message{
				{{Content: "msg1"}, {Content: "msg2"}},
			},
			expected: []*schema.Message{
				{Content: "msg1"}, {Content: "msg2"},
			},
			expectErr: false,
		},
		{
			name: "multiple arrays",
			mas: [][]*schema.Message{
				{{Content: "msg1"}, {Content: "msg2"}},
				{{Content: "msg3"}, {Content: "msg4"}},
			},
			expected: []*schema.Message{
				{Content: "msg1msg3"}, {Content: "msg2msg4"},
			},
			expectErr: false,
		},
		{
			name: "mismatched array lengths",
			mas: [][]*schema.Message{
				{{Content: "msg1"}},
				{{Content: "msg2"}, {Content: "msg3"}},
			},
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := concatMessageArray(tt.mas)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expected), len(result))
				for i := range tt.expected {
					if i < len(result) {
						assert.Equal(t, tt.expected[i].Content, result[i].Content)
					}
				}
			}
		})
	}
}

func TestGetOrInitState(t *testing.T) {
	t.Run("existing state", func(t *testing.T) {
		existingState := &LangsmithState{TraceID: "existing"}
		ctx := context.WithValue(context.Background(), langsmithStateKey{}, existingState)

		newCtx, state := GetOrInitState(ctx)
		assert.Equal(t, ctx, newCtx)
		assert.Equal(t, existingState, state)
		assert.Equal(t, "existing", state.TraceID)
	})

	t.Run("new state without options", func(t *testing.T) {
		ctx := context.Background()
		newCtx, state := GetOrInitState(ctx)
		assert.NotEqual(t, ctx, newCtx)
		assert.NotNil(t, state)
		assert.Empty(t, state.TraceID)
	})

	t.Run("new state with options", func(t *testing.T) {
		opts := &traceOptions{
			TraceID:           "test-trace",
			ParentID:          "parent-id",
			ParentDottedOrder: "1.2.3",
		}
		ctx := context.WithValue(context.Background(), langsmithTraceOptionKey{}, opts)

		newCtx, state := GetOrInitState(ctx)
		assert.NotEqual(t, ctx, newCtx)
		assert.NotNil(t, state)
		assert.Equal(t, "test-trace", state.TraceID)
		assert.Equal(t, "parent-id", state.ParentRunID)
		assert.Equal(t, "1.2.3", state.ParentDottedOrder)
	})
}

func TestGetState(t *testing.T) {
	t.Run("existing state", func(t *testing.T) {
		existingState := &LangsmithState{TraceID: "existing"}
		ctx := context.WithValue(context.Background(), langsmithStateKey{}, existingState)

		newCtx, state := GetState(ctx)
		assert.Equal(t, ctx, newCtx)
		assert.Equal(t, existingState, state)
	})

	t.Run("no state", func(t *testing.T) {
		ctx := context.Background()
		newCtx, state := GetState(ctx)
		assert.Equal(t, ctx, newCtx)
		assert.Nil(t, state)
	})
}

func TestSafeDeepCopyMetadata(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := SafeDeepCopyMetadata(nil)
		assert.NotNil(t, result)
		assert.Contains(t, result, "metadata")
	})

	t.Run("empty map", func(t *testing.T) {
		original := map[string]interface{}{}
		result := SafeDeepCopyMetadata(original)
		assert.NotNil(t, result)
		assert.Contains(t, result, "metadata")
	})

	t.Run("valid metadata", func(t *testing.T) {
		original := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"nested": map[string]interface{}{
				"key3": true,
			},
		}
		result := SafeDeepCopyMetadata(original)
		assert.NotNil(t, result)
		assert.Equal(t, "value1", result["key1"])
		assert.Equal(t, 123.0, result["key2"]) // JSON unmarshal converts numbers to float64
		assert.Equal(t, map[string]interface{}{"key3": true}, result["nested"])
		assert.Contains(t, result, "metadata")
	})

	t.Run("circular reference handling", func(t *testing.T) {
		// This should not panic and return a safe empty map
		original := map[string]interface{}{
			"key": "value",
		}
		result := SafeDeepCopyMetadata(original)
		assert.NotNil(t, result)
		assert.Equal(t, "value", result["key"])
	})
}

func TestSafeDeepCopySyncMapMetadata(t *testing.T) {
	t.Run("nil sync map", func(t *testing.T) {
		result := SafeDeepCopySyncMapMetadata(nil)
		assert.NotNil(t, result)
		assert.Contains(t, result, "metadata")
	})

	t.Run("empty sync map", func(t *testing.T) {
		original := &sync.Map{}
		result := SafeDeepCopySyncMapMetadata(original)
		assert.NotNil(t, result)
		assert.Contains(t, result, "metadata")
	})

	t.Run("with data", func(t *testing.T) {
		original := &sync.Map{}
		original.Store("key1", "value1")
		original.Store("key2", 123)
		original.Store("nested", map[string]interface{}{"key3": true})

		result := SafeDeepCopySyncMapMetadata(original)
		assert.NotNil(t, result)
		assert.Equal(t, "value1", result["key1"])
		assert.Equal(t, 123, result["key2"])
		assert.Equal(t, map[string]interface{}{"key3": true}, result["nested"])
		assert.Contains(t, result, "metadata")
	})
}
