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

package claude

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChatModel(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		validate    func(t *testing.T, model *ChatModel)
	}{
		{
			name: "valid basic config",
			config: &Config{
				APIKey:    "test-key",
				Model:     "claude-3-opus-20240229",
				MaxTokens: 1000,
			},
			expectError: false,
			validate: func(t *testing.T, model *ChatModel) {
				assert.Equal(t, "claude-3-opus-20240229", model.model)
				assert.Equal(t, 1000, model.maxTokens)
				assert.Nil(t, model.responseSchema)
			},
		},
		{
			name: "config with response schema",
			config: &Config{
				APIKey:    "test-key",
				Model:     "claude-3-opus-20240229",
				MaxTokens: 1000,
				ResponseSchema: &openapi3.Schema{
					Type: &openapi3.Types{openapi3.TypeInteger},
					Properties: map[string]*openapi3.SchemaRef{
						"answer": {
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
							},
						},
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, model *ChatModel) {
				assert.NotNil(t, model.responseSchema)
				assert.Equal(t, &openapi3.Types{"object"}, model.responseSchema.Type)
			},
		},
		{
			name: "config with temperature and topP",
			config: &Config{
				APIKey:      "test-key",
				Model:       "claude-3-opus-20240229",
				MaxTokens:   1000,
				Temperature: of(float32(0.7)),
				TopP:        of(float32(0.9)),
				TopK:        of(int32(40)),
			},
			expectError: false,
			validate: func(t *testing.T, model *ChatModel) {
				assert.Equal(t, float32(0.7), *model.temperature)
				assert.Equal(t, float32(0.9), *model.topP)
				assert.Equal(t, int32(40), *model.topK)
			},
		},
		{
			name: "config with stop sequences",
			config: &Config{
				APIKey:        "test-key",
				Model:         "claude-3-opus-20240229",
				MaxTokens:     1000,
				StopSequences: []string{"\n\nHuman:", "\n\nAssistant:"},
			},
			expectError: false,
			validate: func(t *testing.T, model *ChatModel) {
				assert.Equal(t, []string{"\n\nHuman:", "\n\nAssistant:"}, model.stopSequences)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewChatModel(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, model)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, model)
				if tt.validate != nil {
					tt.validate(t, model)
				}
			}
		})
	}
}

func TestChatModel_BindTools(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		tools       []*schema.ToolInfo
		expectError bool
	}{
		{
			name: "bind single tool",
			tools: []*schema.ToolInfo{
				{
					Name: "get_weather",
					Desc: "Get weather information",
					ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
						Type: &openapi3.Types{"object"},
						Properties: map[string]*openapi3.SchemaRef{
							"city": {
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								},
							},
						},
					}),
				},
			},
			expectError: false,
		},
		{
			name: "bind multiple tools",
			tools: []*schema.ToolInfo{
				{
					Name: "tool1",
					Desc: "Tool 1",
					ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
						Type: &openapi3.Types{"object"},
					}),
				},
				{
					Name: "tool2",
					Desc: "Tool 2",
					ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
						Type: &openapi3.Types{"object"},
					}),
				},
			},
			expectError: false,
		},
		{
			name:        "bind empty tools",
			tools:       []*schema.ToolInfo{},
			expectError: true, // BindTools returns error for empty tools
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh model for each test to avoid state contamination
			model, err := NewChatModel(ctx, &Config{
				APIKey:    "test-key",
				Model:     "claude-3-opus-20240229",
				MaxTokens: 1000,
			})
			require.NoError(t, err)

			err = model.BindTools(tt.tools)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.tools), len(model.origTools))
			}
		})
	}
}

func TestChatModel_BindForcedTools(t *testing.T) {
	ctx := context.Background()
	model, err := NewChatModel(ctx, &Config{
		APIKey:    "test-key",
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1000,
	})
	require.NoError(t, err)

	tools := []*schema.ToolInfo{
		{
			Name: "forced_tool",
			Desc: "A forced tool",
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
				Type: &openapi3.Types{"object"},
			}),
		},
	}

	err = model.BindForcedTools(tools)
	assert.NoError(t, err)
	assert.Equal(t, len(tools), len(model.origTools))
	assert.NotNil(t, model.toolChoice)
	assert.Equal(t, schema.ToolChoiceForced, *model.toolChoice)
}

func TestCreateResponseFormatterToolParam(t *testing.T) {
	tests := []struct {
		name        string
		schema      *openapi3.Schema
		expectError bool
		validate    func(t *testing.T, tool any)
	}{
		{
			name: "valid schema",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"answer": {
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						},
					},
				},
				Required: []string{"answer"},
			},
			expectError: false,
			validate: func(t *testing.T, tool any) {
				assert.NotNil(t, tool)
				// Further validation would require anthropic types
			},
		},
		{
			name:        "nil schema",
			schema:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := createResponseFormatterToolParam(tt.schema)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tool)
				}
			}
		})
	}
}

func TestMakeResponseFormatterTransparent(t *testing.T) {
	tests := []struct {
		name     string
		msg      *schema.Message
		expected *schema.Message
	}{
		{
			name: "message with formatter tool call",
			msg: &schema.Message{
				Role:    schema.Assistant,
				Content: "",
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "other_tool",
							Arguments: `{"param":"value"}`,
						},
					},
					{
						ID: "call_2",
						Function: schema.FunctionCall{
							Name:      RESPONSE_FORMATTER_TOOL_NAME,
							Arguments: `{"result":"formatted response"}`,
						},
					},
				},
			},
			expected: &schema.Message{
				Role:    schema.Assistant,
				Content: `{"result":"formatted response"}`,
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "other_tool",
							Arguments: `{"param":"value"}`,
						},
					},
				},
			},
		},
		{
			name: "message with only formatter tool call",
			msg: &schema.Message{
				Role:    schema.Assistant,
				Content: "",
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      RESPONSE_FORMATTER_TOOL_NAME,
							Arguments: `{"answer":"42"}`,
						},
					},
				},
			},
			expected: &schema.Message{
				Role:      schema.Assistant,
				Content:   `{"answer":"42"}`,
				ToolCalls: nil,
			},
		},
		{
			name: "message without formatter tool call",
			msg: &schema.Message{
				Role:    schema.Assistant,
				Content: "Regular response",
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "other_tool",
							Arguments: `{"param":"value"}`,
						},
					},
				},
			},
			expected: &schema.Message{
				Role:    schema.Assistant,
				Content: "Regular response",
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name:      "other_tool",
							Arguments: `{"param":"value"}`,
						},
					},
				},
			},
		},
		{
			name:     "nil message",
			msg:      nil,
			expected: nil,
		},
		{
			name: "message with nil tool calls",
			msg: &schema.Message{
				Role:      schema.Assistant,
				Content:   "Regular response",
				ToolCalls: nil,
			},
			expected: &schema.Message{
				Role:      schema.Assistant,
				Content:   "Regular response",
				ToolCalls: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeResponseFormatterTransparent(tt.msg)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			assert.Equal(t, tt.expected.Role, result.Role)
			assert.Equal(t, tt.expected.Content, result.Content)
			assert.Equal(t, len(tt.expected.ToolCalls), len(result.ToolCalls))

			for i, expectedCall := range tt.expected.ToolCalls {
				assert.Equal(t, expectedCall.ID, result.ToolCalls[i].ID)
				assert.Equal(t, expectedCall.Function.Name, result.ToolCalls[i].Function.Name)
				assert.Equal(t, expectedCall.Function.Arguments, result.ToolCalls[i].Function.Arguments)
			}
		})
	}
}

func TestChatModel_GetType(t *testing.T) {
	ctx := context.Background()
	model, err := NewChatModel(ctx, &Config{
		APIKey:    "test-key",
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1000,
	})
	require.NoError(t, err)

	assert.Equal(t, "Claude", model.GetType())
}

func TestChatModel_IsCallbacksEnabled(t *testing.T) {
	ctx := context.Background()
	model, err := NewChatModel(ctx, &Config{
		APIKey:    "test-key",
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1000,
	})
	require.NoError(t, err)

	// Should return true by default
	assert.True(t, model.IsCallbacksEnabled())
}

func TestChatModel_WithTools(t *testing.T) {
	ctx := context.Background()
	model, err := NewChatModel(ctx, &Config{
		APIKey:    "test-key",
		Model:     "claude-3-opus-20240229",
		MaxTokens: 1000,
	})
	require.NoError(t, err)

	tools := []*schema.ToolInfo{
		{
			Name: "test_tool",
			Desc: "A test tool",
			ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(&openapi3.Schema{
				Type: &openapi3.Types{"object"},
			}),
		},
	}

	newModel, err := model.WithTools(tools)
	assert.NoError(t, err)
	assert.NotNil(t, newModel)

	claudeModel, ok := newModel.(*ChatModel)
	assert.True(t, ok)
	assert.Equal(t, len(tools), len(claudeModel.origTools))
	assert.Equal(t, "test_tool", claudeModel.origTools[0].Name)
}

func TestPanicErr(t *testing.T) {
	info := "test panic info"
	stack := []byte("test stack trace")

	err := newPanicErr(info, stack)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), info)
	assert.Contains(t, err.Error(), "test stack trace")
}

func TestConvImageBase64(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		expectError  bool
		expectedURL  string
		expectedMIME string
	}{
		{
			name:         "valid jpeg base64",
			data:         "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAYABgAAD",
			expectError:  false,
			expectedURL:  "/9j/4AAQSkZJRgABAQEAYABgAAD",
			expectedMIME: "image/jpeg",
		},
		{
			name:         "valid png base64",
			data:         "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAE",
			expectError:  false,
			expectedURL:  "iVBORw0KGgoAAAANSUhEUgAAAAE",
			expectedMIME: "image/png",
		},
		{
			name:        "invalid format",
			data:        "invalid-data",
			expectError: true,
		},
		{
			name:         "missing base64 data",
			data:         "data:image/jpeg;base64,",
			expectError:  false,
			expectedMIME: "image/jpeg",
			expectedURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mimeType, url, err := convImageBase64(tt.data)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMIME, mimeType)
				assert.Equal(t, tt.expectedURL, url)
			}
		})
	}
}

func TestIsMessageEmpty(t *testing.T) {
	tests := []struct {
		name     string
		message  *schema.Message
		expected bool
	}{
		{
			name: "empty content and no tool calls",
			message: &schema.Message{
				Content: "",
			},
			expected: true,
		},
		{
			name: "message with content",
			message: &schema.Message{
				Content: "Hello",
			},
			expected: false,
		},
		{
			name: "message with tool calls",
			message: &schema.Message{
				Content: "",
				ToolCalls: []schema.ToolCall{
					{
						ID: "call_1",
						Function: schema.FunctionCall{
							Name: "test_tool",
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMessageEmpty(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test nil message separately to avoid panic
	t.Run("nil message should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			isMessageEmpty(nil)
		})
	})
}

// TestResponseSchemaIntegration tests the integration of response schema functionality
func TestResponseSchemaIntegration(t *testing.T) {
	ctx := context.Background()

	responseSchema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"answer": {
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"string"},
				},
			},
			"confidence": {
				Value: &openapi3.Schema{
					Type: &openapi3.Types{"number"},
				},
			},
		},
		Required: []string{"answer"},
	}

	model, err := NewChatModel(ctx, &Config{
		APIKey:         "test-key",
		Model:          "claude-3-opus-20240229",
		MaxTokens:      1000,
		ResponseSchema: responseSchema,
	})
	require.NoError(t, err)
	assert.NotNil(t, model.responseSchema)

	// Test that response schema is preserved
	assert.Equal(t, responseSchema, model.responseSchema)
	assert.Equal(t, &openapi3.Types{"object"}, model.responseSchema.Type)
	assert.Contains(t, model.responseSchema.Properties, "answer")
	assert.Contains(t, model.responseSchema.Properties, "confidence")
	assert.Equal(t, []string{"answer"}, model.responseSchema.Required)
}
