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

package cozeloop

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go"
	"github.com/coze-dev/cozeloop-go/entity"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string {
	return &s
}

func TestRoleConv(t *testing.T) {
	tests := []struct {
		name     string
		input    entity.Role
		expected schema.RoleType
		wantErr  bool
	}{
		{
			name:     "system role",
			input:    entity.RoleSystem,
			expected: schema.System,
			wantErr:  false,
		},
		{
			name:     "user role",
			input:    entity.RoleUser,
			expected: schema.User,
			wantErr:  false,
		},
		{
			name:     "assistant role",
			input:    entity.RoleAssistant,
			expected: schema.Assistant,
			wantErr:  false,
		},
		{
			name:     "tool role",
			input:    entity.RoleTool,
			expected: schema.Tool,
			wantErr:  false,
		},
		{
			name:     "unknown role",
			input:    entity.Role("unknown"),
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := roleConv(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestContentTypeConv(t *testing.T) {
	tests := []struct {
		name     string
		input    entity.ContentType
		expected schema.ChatMessagePartType
		wantErr  bool
	}{
		{
			name:     "text content",
			input:    entity.ContentTypeText,
			expected: schema.ChatMessagePartTypeText,
			wantErr:  false,
		},
		{
			name:     "image url content",
			input:    entity.ContentTypeImageURL,
			expected: schema.ChatMessagePartTypeImageURL,
			wantErr:  false,
		},
		{
			name:     "base64 data content",
			input:    entity.ContentTypeBase64Data,
			expected: schema.ChatMessagePartTypeImageURL,
			wantErr:  false,
		},
		{
			name:     "unknown content type",
			input:    entity.ContentType("unknown"),
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := contentTypeConv(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestInputPartsConv(t *testing.T) {
	tests := []struct {
		name     string
		input    []*entity.ContentPart
		expected []schema.MessageInputPart
		wantErr  bool
	}{
		{
			name: "text part",
			input: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: strPtr("hello world"),
				},
			},
			expected: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "hello world",
				},
			},
			wantErr: false,
		},
		{
			name: "image url part",
			input: []*entity.ContentPart{
				{
					Type:     entity.ContentTypeImageURL,
					ImageURL: strPtr("https://example.com/image.png"),
				},
			},
			expected: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							URL: strPtr("https://example.com/image.png"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "base64 data part",
			input: []*entity.ContentPart{
				{
					Type:       entity.ContentTypeBase64Data,
					Base64Data: strPtr("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="),
				},
			},
			expected: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: strPtr("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="),
							MIMEType:   "image/png",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mixed parts",
			input: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: strPtr("Describe this image: "),
				},
				{
					Type:     entity.ContentTypeImageURL,
					ImageURL: strPtr("https://example.com/image.png"),
				},
				{
					Type: entity.ContentTypeText,
					Text: strPtr(" in detail."),
				},
			},
			expected: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "Describe this image: ",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							URL: strPtr("https://example.com/image.png"),
						},
					},
				},
				{
					Type: schema.ChatMessagePartTypeText,
					Text: " in detail.",
				},
			},
			wantErr: false,
		},
		{
			name: "nil parts are skipped",
			input: []*entity.ContentPart{
				nil,
				{
					Type: entity.ContentTypeText,
					Text: strPtr("hello"),
				},
				nil,
			},
			expected: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "hello",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := inputPartsConv(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestOutputPartsConv(t *testing.T) {
	tests := []struct {
		name     string
		input    []*entity.ContentPart
		expected []schema.MessageOutputPart
		wantErr  bool
	}{
		{
			name: "text part",
			input: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: strPtr("hello world"),
				},
			},
			expected: []schema.MessageOutputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "hello world",
				},
			},
			wantErr: false,
		},
		{
			name: "image url part",
			input: []*entity.ContentPart{
				{
					Type:     entity.ContentTypeImageURL,
					ImageURL: strPtr("https://example.com/image.png"),
				},
			},
			expected: []schema.MessageOutputPart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageOutputImage{
						MessagePartCommon: schema.MessagePartCommon{
							URL: strPtr("https://example.com/image.png"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "base64 data part",
			input: []*entity.ContentPart{
				{
					Type:       entity.ContentTypeBase64Data,
					Base64Data: strPtr("data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAA=="),
				},
			},
			expected: []schema.MessageOutputPart{
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageOutputImage{
						MessagePartCommon: schema.MessagePartCommon{
							Base64Data: strPtr("/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAA=="),
							MIMEType:   "image/jpeg",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil parts are skipped",
			input: []*entity.ContentPart{
				nil,
				{
					Type: entity.ContentTypeText,
					Text: strPtr("hello"),
				},
				nil,
			},
			expected: []schema.MessageOutputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "hello",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := outputPartsConv(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMessageConv(t *testing.T) {
	tests := []struct {
		name     string
		input    *entity.Message
		expected *schema.Message
		wantErr  bool
	}{
		{
			name: "simple user message",
			input: &entity.Message{
				Role:    entity.RoleUser,
				Content: strPtr("hello world"),
			},
			expected: &schema.Message{
				Role:    schema.User,
				Content: "hello world",
			},
			wantErr: false,
		},
		{
			name: "simple assistant message",
			input: &entity.Message{
				Role:    entity.RoleAssistant,
				Content: strPtr("hi there"),
			},
			expected: &schema.Message{
				Role:    schema.Assistant,
				Content: "hi there",
			},
			wantErr: false,
		},
		{
			name: "user message with multi-content",
			input: &entity.Message{
				Role: entity.RoleUser,
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeText,
						Text: strPtr("Describe: "),
					},
					{
						Type:     entity.ContentTypeImageURL,
						ImageURL: strPtr("https://example.com/image.png"),
					},
				},
			},
			expected: &schema.Message{
				Role: schema.User,
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "Describe: ",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{
							MessagePartCommon: schema.MessagePartCommon{
								URL: strPtr("https://example.com/image.png"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "assistant message with multi-content",
			input: &entity.Message{
				Role: entity.RoleAssistant,
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeText,
						Text: strPtr("Here is an image: "),
					},
					{
						Type:     entity.ContentTypeImageURL,
						ImageURL: strPtr("https://example.com/result.png"),
					},
				},
			},
			expected: &schema.Message{
				Role: schema.Assistant,
				AssistantGenMultiContent: []schema.MessageOutputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "Here is an image: ",
					},
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageOutputImage{
							MessagePartCommon: schema.MessagePartCommon{
								URL: strPtr("https://example.com/result.png"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "system message",
			input: &entity.Message{
				Role:    entity.RoleSystem,
				Content: strPtr("You are a helpful assistant."),
			},
			expected: &schema.Message{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
			wantErr: false,
		},
		{
			name: "message with both content and parts (user)",
			input: &entity.Message{
				Role:    entity.RoleUser,
				Content: strPtr("main content"),
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeText,
						Text: strPtr("part content"),
					},
				},
			},
			expected: &schema.Message{
				Role:    schema.User,
				Content: "main content",
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type: schema.ChatMessagePartTypeText,
						Text: "part content",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := messageConv(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestNewPromptHub(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "conf is empty",
		},
		{
			name: "nil client",
			config: &Config{
				Key:            "test.key",
				Version:        "1.0.0",
				CozeLoopClient: nil,
			},
			wantErr: true,
			errMsg:  "cozeloop client in conf is empty",
		},
		{
			name: "valid config",
			config: &Config{
				Key:            "test.key",
				Version:        "1.0.0",
				CozeLoopClient: &mockCozeLoopClient{},
			},
			wantErr: false,
		},
		{
			name: "valid config without version",
			config: &Config{
				Key:            "test.key",
				CozeLoopClient: &mockCozeLoopClient{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ph, err := NewPromptHub(ctx, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, ph)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ph)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		mockClient   *mockCozeLoopClient
		variables    map[string]any
		expectError  bool
		expectMsgCnt int
		validateFunc func(*testing.T, []*schema.Message)
	}{
		{
			name: "simple text message",
			mockClient: &mockCozeLoopClient{
				getPromptResult: &entity.Prompt{
					PromptTemplate: &entity.PromptTemplate{
						Messages: []*entity.Message{
							{
								Role:    entity.RoleUser,
								Content: strPtr("hello world"),
							},
						},
					},
				},
			},
			variables:    map[string]any{},
			expectError:  false,
			expectMsgCnt: 1,
			validateFunc: func(t *testing.T, msgs []*schema.Message) {
				assert.Equal(t, schema.User, msgs[0].Role)
				assert.Equal(t, "hello world", msgs[0].Content)
			},
		},
		{
			name: "multiple messages",
			mockClient: &mockCozeLoopClient{
				getPromptResult: &entity.Prompt{
					PromptTemplate: &entity.PromptTemplate{
						Messages: []*entity.Message{
							{
								Role:    entity.RoleSystem,
								Content: strPtr("You are a helpful assistant."),
							},
							{
								Role:    entity.RoleUser,
								Content: strPtr("Hello!"),
							},
							{
								Role:    entity.RoleAssistant,
								Content: strPtr("Hi there!"),
							},
						},
					},
				},
			},
			variables:    map[string]any{},
			expectError:  false,
			expectMsgCnt: 3,
			validateFunc: func(t *testing.T, msgs []*schema.Message) {
				assert.Equal(t, schema.System, msgs[0].Role)
				assert.Equal(t, "You are a helpful assistant.", msgs[0].Content)
				assert.Equal(t, schema.User, msgs[1].Role)
				assert.Equal(t, "Hello!", msgs[1].Content)
				assert.Equal(t, schema.Assistant, msgs[2].Role)
				assert.Equal(t, "Hi there!", msgs[2].Content)
			},
		},
		{
			name: "message with multi-content",
			mockClient: &mockCozeLoopClient{
				getPromptResult: &entity.Prompt{
					PromptTemplate: &entity.PromptTemplate{
						Messages: []*entity.Message{
							{
								Role: entity.RoleUser,
								Parts: []*entity.ContentPart{
									{
										Type: entity.ContentTypeText,
										Text: strPtr("Describe this: "),
									},
									{
										Type:     entity.ContentTypeImageURL,
										ImageURL: strPtr("https://example.com/image.png"),
									},
								},
							},
						},
					},
				},
				formatResult: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeText,
								Text: strPtr("Describe this: "),
							},
							{
								Type:     entity.ContentTypeImageURL,
								ImageURL: strPtr("https://example.com/image.png"),
							},
						},
					},
				},
			},
			variables:    map[string]any{},
			expectError:  false,
			expectMsgCnt: 1,
			validateFunc: func(t *testing.T, msgs []*schema.Message) {
				assert.Equal(t, schema.User, msgs[0].Role)
				assert.Len(t, msgs[0].UserInputMultiContent, 2)
				assert.Equal(t, schema.ChatMessagePartTypeText, msgs[0].UserInputMultiContent[0].Type)
				assert.Equal(t, "Describe this: ", msgs[0].UserInputMultiContent[0].Text)
				assert.Equal(t, schema.ChatMessagePartTypeImageURL, msgs[0].UserInputMultiContent[1].Type)
				assert.NotNil(t, msgs[0].UserInputMultiContent[1].Image)
			},
		},
		{
			name: "nil messages are skipped",
			mockClient: &mockCozeLoopClient{
				getPromptResult: &entity.Prompt{
					PromptTemplate: &entity.PromptTemplate{
						Messages: []*entity.Message{
							nil,
							{
								Role:    entity.RoleUser,
								Content: strPtr("hello"),
							},
							nil,
						},
					},
				},
			},
			variables:    map[string]any{},
			expectError:  false,
			expectMsgCnt: 1,
			validateFunc: func(t *testing.T, msgs []*schema.Message) {
				assert.Equal(t, schema.User, msgs[0].Role)
				assert.Equal(t, "hello", msgs[0].Content)
			},
		},
		{
			name: "get prompt error",
			mockClient: &mockCozeLoopClient{
				getPromptError: assert.AnError,
			},
			variables:   map[string]any{},
			expectError: true,
		},
		{
			name: "nil prompt result",
			mockClient: &mockCozeLoopClient{
				getPromptResult: nil,
			},
			variables:   map[string]any{},
			expectError: true,
		},
		{
			name: "format error",
			mockClient: &mockCozeLoopClient{
				getPromptResult: &entity.Prompt{
					PromptTemplate: &entity.PromptTemplate{
						Messages: []*entity.Message{
							{
								Role:    entity.RoleUser,
								Content: strPtr("test"),
							},
						},
					},
				},
				formatError: assert.AnError,
			},
			variables:   map[string]any{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ph, err := NewPromptHub(ctx, &Config{
				Key:            "test.key",
				Version:        "1.0.0",
				CozeLoopClient: tt.mockClient,
			})
			assert.NoError(t, err)

			result, err := ph.Format(ctx, tt.variables)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectMsgCnt)
				if tt.validateFunc != nil {
					tt.validateFunc(t, result)
				}
			}
		})
	}
}

// mockCozeLoopClient implements cozeloop.Client interface for testing
type mockCozeLoopClient struct {
	getPromptResult *entity.Prompt
	getPromptError  error
	formatResult    []*entity.Message
	formatError     error
}

func (m *mockCozeLoopClient) GetPrompt(ctx context.Context, param cozeloop.GetPromptParam, options ...cozeloop.GetPromptOption) (*entity.Prompt, error) {
	if m.getPromptError != nil {
		return nil, m.getPromptError
	}
	return m.getPromptResult, nil
}

func (m *mockCozeLoopClient) PromptFormat(ctx context.Context, prompt *entity.Prompt, variables map[string]any, options ...cozeloop.PromptFormatOption) ([]*entity.Message, error) {
	if m.formatError != nil {
		return nil, m.formatError
	}
	if m.formatResult != nil {
		return m.formatResult, nil
	}
	// Default behavior: return the messages from the prompt template
	if prompt != nil && prompt.PromptTemplate != nil {
		return prompt.PromptTemplate.Messages, nil
	}
	return nil, nil
}

func (m *mockCozeLoopClient) Execute(ctx context.Context, param *entity.ExecuteParam, options ...cozeloop.ExecuteOption) (entity.ExecuteResult, error) {
	panic("not implemented")
}

func (m *mockCozeLoopClient) ExecuteStreaming(ctx context.Context, param *entity.ExecuteParam, options ...cozeloop.ExecuteStreamingOption) (entity.StreamReader[entity.ExecuteResult], error) {
	panic("not implemented")
}

func (m *mockCozeLoopClient) StartSpan(ctx context.Context, name, spanType string, opts ...cozeloop.StartSpanOption) (context.Context, cozeloop.Span) {
	return ctx, nil
}

func (m *mockCozeLoopClient) GetSpanFromContext(ctx context.Context) cozeloop.Span {
	return nil
}

func (m *mockCozeLoopClient) GetSpanFromHeader(ctx context.Context, header map[string]string) cozeloop.SpanContext {
	return nil
}

func (m *mockCozeLoopClient) Flush(ctx context.Context) {
}

func (m *mockCozeLoopClient) GetWorkspaceID() string {
	return "test-workspace"
}

func (m *mockCozeLoopClient) Close(ctx context.Context) {
}

func TestParseBase64DataURL(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedMIME   string
		expectedBase64 string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid PNG data URL",
			input:          "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			expectedMIME:   "image/png",
			expectedBase64: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			wantErr:        false,
		},
		{
			name:           "valid JPEG data URL",
			input:          "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAA==",
			expectedMIME:   "image/jpeg",
			expectedBase64: "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAA==",
			wantErr:        false,
		},
		{
			name:           "valid GIF data URL",
			input:          "data:image/gif;base64,R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw==",
			expectedMIME:   "image/gif",
			expectedBase64: "R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw==",
			wantErr:        false,
		},
		{
			name:        "missing data prefix",
			input:       "image/png;base64,iVBORw0KGgo=",
			wantErr:     true,
			errContains: "decode data URL",
		},
		{
			name:        "missing comma separator",
			input:       "data:image/png;base64",
			wantErr:     true,
			errContains: "decode data URL",
		},
		{
			name:           "non-base64 encoding (URL encoded)",
			input:          "data:text/plain,Hello%20World",
			expectedMIME:   "text/plain",
			expectedBase64: "Hello%20World",
			wantErr:        false,
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "decode data URL",
		},
		{
			name:           "data URL with charset",
			input:          "data:text/plain;charset=utf-8;base64,SGVsbG8gV29ybGQ=",
			expectedMIME:   "text/plain", // ContentType() only returns the main MIME type
			expectedBase64: "SGVsbG8gV29ybGQ=",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mimeType, base64Data, err := parseBase64DataURL(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMIME, mimeType)
				assert.Equal(t, tt.expectedBase64, base64Data)
			}
		})
	}
}
