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

package hunyuan

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
	common "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/hunyuan/v20230901"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestChatModelGenerate(t *testing.T) {
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return &hunyuan.ChatCompletionsResponse{
			Response: &hunyuan.ChatCompletionsResponseParams{
				Choices: []*hunyuan.Choice{
					{
						Message: &hunyuan.Message{
							Role:    toPtr("assistant"),
							Content: toPtr("hello world"),
						},
					},
				},
				Usage: &hunyuan.Usage{
					PromptTokens:     toPtr(int64(1)),
					CompletionTokens: toPtr(int64(2)),
					TotalTokens:      toPtr(int64(3)),
				},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	result, err := cm.Generate(ctx, []*schema.Message{
		schema.SystemMessage("system"),
		schema.UserMessage("hello"),
	})
	assert.Nil(t, err)
	assert.Equal(t, "hello world", result.Content)
	assert.Equal(t, schema.Assistant, result.Role)
	assert.Equal(t, 1, result.ResponseMeta.Usage.PromptTokens)
	assert.Equal(t, 2, result.ResponseMeta.Usage.CompletionTokens)
	assert.Equal(t, 3, result.ResponseMeta.Usage.TotalTokens)
}

func TestChatModelGenerateErrorHandling(t *testing.T) {
	// Test API error
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return nil, fmt.Errorf("API error: authentication failed")
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	_, err = cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestChatModelGenerateEmptyResponse(t *testing.T) {
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return &hunyuan.ChatCompletionsResponse{
			Response: &hunyuan.ChatCompletionsResponseParams{
				Choices: []*hunyuan.Choice{},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)
	result, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	assert.Nil(t, err)
	assert.Equal(t, "", result.Content)
}

func TestChatModelGenerateNilContent(t *testing.T) {
	// Test nil content in response
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return &hunyuan.ChatCompletionsResponse{
			Response: &hunyuan.ChatCompletionsResponseParams{
				Choices: []*hunyuan.Choice{
					{
						Message: &hunyuan.Message{
							Role:    toPtr("assistant"),
							Content: nil,
						},
					},
				},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	result, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	assert.Nil(t, err)
	assert.Equal(t, "", result.Content)
}

func TestChatModelStream(t *testing.T) {
	mockStreamData := hunyuan.ChatCompletionsResponseParams{
		Usage: &hunyuan.Usage{
			PromptTokens:     toPtr(int64(1)),
			CompletionTokens: toPtr(int64(2)),
			TotalTokens:      toPtr(int64(3)),
		},
		Choices: []*hunyuan.Choice{
			{
				Message: &hunyuan.Message{
					Role:    toPtr("assistant"),
					Content: toPtr("stream response"),
				},
			},
		},
	}
	dataBytes, _ := json.Marshal(mockStreamData)
	//dataBytes = append([]byte{'\n'}, dataBytes...)
	eventChan := make(chan common.SSEvent)
	go func() {
		eventChan <- common.SSEvent{
			Data: dataBytes,
		}
		close(eventChan)
	}()
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return &hunyuan.ChatCompletionsResponse{
			BaseSSEResponse: common.BaseSSEResponse{
				Events: eventChan,
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	stream, err := cm.Stream(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	assert.Nil(t, err)

	chunk, err := stream.Recv()
	assert.Nil(t, err)
	assert.Equal(t, "stream response", chunk.Content)
	assert.Equal(t, schema.Assistant, chunk.Role)
}

func TestChatModelStreamErrorHandling(t *testing.T) {
	// Test stream error
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return nil, fmt.Errorf("stream error: connection failed")
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	_, err = cm.Stream(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stream error")
}

func TestChatModelWithTools(t *testing.T) {
	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	ncm, err := cm.WithTools([]*schema.ToolInfo{
		{
			Name: "test-tool",
			Desc: "test tool description",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"param1": {Type: schema.String, Desc: "parameter 1"},
			}),
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, "hunyuan-lite", ncm.(*ChatModel).conf.Model)
	assert.Equal(t, "test-tool", ncm.(*ChatModel).rawTools[0].Name)
}

func TestChatModelWithToolsError(t *testing.T) {
	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	// Test with nil tools
	_, err = cm.WithTools(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tools to bind")

	// Test with empty tools
	_, err = cm.WithTools([]*schema.ToolInfo{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tools to bind")
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("test info", []byte("test stack"))
	assert.Equal(t, "panic error: test info, \nstack: test stack", err.Error())
}

func TestToPtr(t *testing.T) {
	// Test string pointer
	strPtr := toPtr("test")
	assert.Equal(t, "test", *strPtr)

	// Test int pointer
	intPtr := toPtr(42)
	assert.Equal(t, 42, *intPtr)

	// Test float pointer
	floatPtr := toPtr(3.14)
	assert.Equal(t, 3.14, *floatPtr)

	// Test bool pointer
	boolPtr := toPtr(true)
	assert.Equal(t, true, *boolPtr)
}

func TestPopulateToolChoice(t *testing.T) {
	toolChoiceForbidden := schema.ToolChoiceForbidden
	toolChoiceAllowed := schema.ToolChoiceAllowed
	toolChoiceForced := schema.ToolChoiceForced

	tool1 := &hunyuan.Tool{
		Function: &hunyuan.ToolFunction{
			Name: toPtr("tool1"),
		},
	}
	tool2 := &hunyuan.Tool{
		Function: &hunyuan.ToolFunction{
			Name: toPtr("tool2"),
		},
	}

	testCases := []struct {
		name        string
		options     *model.Options
		req         *hunyuan.ChatCompletionsRequest
		expectedReq *hunyuan.ChatCompletionsRequest
		expectErr   bool
		errContains string
	}{
		{
			name:        "nil tool choice",
			options:     &model.Options{},
			req:         &hunyuan.ChatCompletionsRequest{},
			expectedReq: &hunyuan.ChatCompletionsRequest{},
			expectErr:   false,
		},
		{
			name: "tool choice forbidden",
			options: &model.Options{
				ToolChoice: &toolChoiceForbidden,
			},
			req: &hunyuan.ChatCompletionsRequest{},
			expectedReq: &hunyuan.ChatCompletionsRequest{
				ToolChoice: toPtr(toolChoiceNone),
			},
			expectErr: false,
		},
		{
			name: "tool choice allowed",
			options: &model.Options{
				ToolChoice: &toolChoiceAllowed,
			},
			req: &hunyuan.ChatCompletionsRequest{},
			expectedReq: &hunyuan.ChatCompletionsRequest{
				ToolChoice: toPtr(toolChoiceAuto),
			},
			expectErr: false,
		},
		{
			name: "tool choice forced with no tools",
			options: &model.Options{
				ToolChoice: &toolChoiceForced,
			},
			req:         &hunyuan.ChatCompletionsRequest{},
			expectErr:   true,
			errContains: "tool choice is forced but tool is not provided",
		},
		{
			name: "tool choice forced with allowed tool name not in tools list",
			options: &model.Options{
				ToolChoice:       &toolChoiceForced,
				AllowedToolNames: []string{"tool3"},
			},
			req: &hunyuan.ChatCompletionsRequest{
				Tools: []*hunyuan.Tool{tool1, tool2},
			},
			expectErr:   true,
			errContains: "allowed tool name 'tool3' not found in tools list",
		},
		{
			name: "tool choice forced with one allowed tool name",
			options: &model.Options{
				ToolChoice:       &toolChoiceForced,
				AllowedToolNames: []string{"tool1"},
			},
			req: &hunyuan.ChatCompletionsRequest{
				Tools: []*hunyuan.Tool{tool1, tool2},
			},
			expectedReq: &hunyuan.ChatCompletionsRequest{
				Tools:      []*hunyuan.Tool{tool1, tool2},
				CustomTool: tool1,
				ToolChoice: toPtr(toolChoiceRequired),
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := populateToolChoice(tc.req, tc.options.ToolChoice, tc.options.AllowedToolNames)

			if tc.expectErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedReq.ToolChoice, tc.req.ToolChoice)
				assert.Equal(t, tc.expectedReq.CustomTool, tc.req.CustomTool)
			}
		})
	}
}

func TestConvertResponse(t *testing.T) {
	// Test basic response conversion
	resp := &hunyuan.ChatCompletionsResponseParams{
		Choices: []*hunyuan.Choice{
			{
				Message: &hunyuan.Message{
					Role:    toPtr("assistant"),
					Content: toPtr("test response"),
				},
			},
		},
		Usage: &hunyuan.Usage{
			PromptTokens:     toPtr(int64(10)),
			CompletionTokens: toPtr(int64(20)),
			TotalTokens:      toPtr(int64(30)),
		},
	}

	msg := convertResponse(resp)
	assert.Equal(t, schema.Assistant, msg.Role)
	assert.Equal(t, "test response", msg.Content)
	assert.Equal(t, 10, msg.ResponseMeta.Usage.PromptTokens)
	assert.Equal(t, 20, msg.ResponseMeta.Usage.CompletionTokens)
	assert.Equal(t, 30, msg.ResponseMeta.Usage.TotalTokens)
}

func TestToTools(t *testing.T) {
	tools := []*schema.ToolInfo{
		{
			Name: "test_tool",
			Desc: "test tool description",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"param": {Type: schema.String, Desc: "test parameter"},
			}),
		},
	}

	hunyuanTools, err := toTools(tools)
	assert.Nil(t, err)
	assert.Len(t, hunyuanTools, 1)
	assert.Equal(t, "test_tool", *hunyuanTools[0].Function.Name)
	assert.Equal(t, "test tool description", *hunyuanTools[0].Function.Description)
}

func TestChatModelWithOptions(t *testing.T) {
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		// Verify that options are properly applied
		assert.Equal(t, float32(0.7), float32(*request.Temperature))
		assert.Equal(t, float32(0.9), float32(*request.TopP))

		return &hunyuan.ChatCompletionsResponse{
			Response: &hunyuan.ChatCompletionsResponseParams{
				Choices: []*hunyuan.Choice{
					{
						Message: &hunyuan.Message{
							Role:    toPtr("assistant"),
							Content: toPtr("test response"),
						},
					},
				},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	_, err = cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	}, model.WithTemperature(0.7), model.WithTopP(0.9), model.WithMaxTokens(100))
	assert.Nil(t, err)
}

func TestChatModelReasoningContent(t *testing.T) {
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return &hunyuan.ChatCompletionsResponse{
			Response: &hunyuan.ChatCompletionsResponseParams{
				Choices: []*hunyuan.Choice{
					{
						Message: &hunyuan.Message{
							Role:             toPtr("assistant"),
							Content:          toPtr("final answer"),
							ReasoningContent: toPtr("step by step reasoning process"),
						},
					},
				},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	result, err := cm.Generate(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	assert.Nil(t, err)
	assert.Equal(t, "final answer", result.Content)
	assert.Equal(t, "step by step reasoning process", result.ReasoningContent)
}

func TestChatModelToolCalls(t *testing.T) {
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		return &hunyuan.ChatCompletionsResponse{
			Response: &hunyuan.ChatCompletionsResponseParams{
				Choices: []*hunyuan.Choice{
					{
						Message: &hunyuan.Message{
							Role:    toPtr("assistant"),
							Content: toPtr(""),
							ToolCalls: []*hunyuan.ToolCall{
								{
									Id:    toPtr("call_123"),
									Index: toPtr(int64(0)),
									Function: &hunyuan.ToolCallFunction{
										Name:      toPtr("get_weather"),
										Arguments: toPtr(`{"city":"Beijing"}`),
									},
								},
							},
						},
					},
				},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	cmWithTools, err := cm.WithTools([]*schema.ToolInfo{
		{
			Name: "get_weather",
			Desc: "Get weather information",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"city": {Type: schema.String, Desc: "City name"},
			}),
		},
	})
	assert.Nil(t, err)

	result, err := cmWithTools.Generate(ctx, []*schema.Message{
		schema.UserMessage("What's the weather in Beijing?"),
	})
	assert.Nil(t, err)
	assert.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "get_weather", result.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"city":"Beijing"}`, result.ToolCalls[0].Function.Arguments)
}

func TestChatModelMultimodalContent(t *testing.T) {
	defer mockey.Mock((*hunyuan.Client).ChatCompletionsWithContext).To(func(ctx context.Context, request *hunyuan.ChatCompletionsRequest) (*hunyuan.ChatCompletionsResponse, error) {
		// Verify multimodal request structure
		assert.Len(t, request.Messages, 1)
		assert.Equal(t, "user", *request.Messages[0].Role)
		assert.Len(t, request.Messages[0].Contents, 2)

		return &hunyuan.ChatCompletionsResponse{
			Response: &hunyuan.ChatCompletionsResponseParams{
				Choices: []*hunyuan.Choice{
					{
						Message: &hunyuan.Message{
							Role:    toPtr("assistant"),
							Content: toPtr("This is an image of a cat"),
						},
					},
				},
			},
		}, nil
	}).Build().UnPatch()

	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "test-secret-id",
		SecretKey: "test-secret-key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	messages := []*schema.Message{
		{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{
					Type: schema.ChatMessagePartTypeText,
					Text: "Describe this image:",
				},
				{
					Type: schema.ChatMessagePartTypeImageURL,
					Image: &schema.MessageInputImage{
						MessagePartCommon: schema.MessagePartCommon{
							MIMEType:   "image/jpeg",
							Base64Data: toPtr("base64-image-data"),
						},
					},
				},
			},
		},
	}

	result, err := cm.Generate(ctx, messages)
	assert.Nil(t, err)
	assert.Equal(t, "This is an image of a cat", result.Content)
}

func TestChatModelDifferentModels(t *testing.T) {
	testCases := []struct {
		name     string
		model    string
		expected string
	}{
		{"hunyuan-lite", "hunyuan-lite", "hunyuan-lite"},
		{"hunyuan-pro", "hunyuan-pro", "hunyuan-pro"},
		{"hunyuan-turbo", "hunyuan-turbo", "hunyuan-turbo"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cm, err := NewChatModel(ctx, &ChatModelConfig{
				SecretId:  "test-secret-id",
				SecretKey: "test-secret-key",
				Model:     tc.model,
			})
			assert.Nil(t, err)
			assert.Equal(t, tc.expected, cm.conf.Model)
		})
	}
}

func TestNewChatModelErrors(t *testing.T) {
	ctx := context.Background()

	// missing model
	_, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "id",
		SecretKey: "key",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model is required")

	// with custom region and language (covers extra branches)
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "id",
		SecretKey: "key",
		Model:     "hunyuan-lite",
		Region:    "ap-shanghai",
		Language:  "en-US",
	})
	assert.Nil(t, err)
	assert.NotNil(t, cm)
	assert.Equal(t, "hunyuan-lite", cm.conf.Model)
}

func TestBindTools(t *testing.T) {
	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "id",
		SecretKey: "key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	// success
	err = cm.BindTools([]*schema.ToolInfo{
		{
			Name: "t1",
			Desc: "tool 1",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"p": {Type: schema.String, Desc: "p"},
			}),
		},
	})
	assert.Nil(t, err)
	assert.Len(t, cm.tools, 1)
	assert.Equal(t, "t1", cm.rawTools[0].Name)
	assert.NotNil(t, cm.toolChoice)

	// nil tools
	err = cm.BindTools(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tools to bind")

	// empty tools
	err = cm.BindTools([]*schema.ToolInfo{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no tools to bind")
}

func TestConvertResponseDelta(t *testing.T) {
	finishReason := "stop"
	resp := &hunyuan.ChatCompletionsResponseParams{
		Choices: []*hunyuan.Choice{
			{
				FinishReason: toPtr(finishReason),
				Delta: &hunyuan.Delta{
					Role:             toPtr("assistant"),
					Content:          toPtr("delta content"),
					ReasoningContent: toPtr("reasoning"),
					ToolCalls: []*hunyuan.ToolCall{
						{
							Id:    toPtr("c1"),
							Index: toPtr(int64(0)),
							Type:  toPtr("function"),
							Function: &hunyuan.ToolCallFunction{
								Name:      toPtr("fn"),
								Arguments: toPtr(`{"a":1}`),
							},
						},
					},
				},
			},
		},
	}

	msg := convertResponse(resp)
	assert.Equal(t, schema.Assistant, msg.Role)
	assert.Equal(t, "delta content", msg.Content)
	assert.Equal(t, "reasoning", msg.ReasoningContent)
	assert.Equal(t, "stop", msg.ResponseMeta.FinishReason)
	assert.Len(t, msg.ToolCalls, 1)
	assert.Equal(t, "fn", msg.ToolCalls[0].Function.Name)
}

func TestConvertMessage_Roles(t *testing.T) {
	// system
	m, err := convertMessage(&schema.Message{Role: schema.System, Content: "sys"})
	assert.Nil(t, err)
	assert.Equal(t, roleSystem, *m.Role)
	assert.Equal(t, "sys", *m.Content)

	// user
	m, err = convertMessage(&schema.Message{Role: schema.User, Content: "u"})
	assert.Nil(t, err)
	assert.Equal(t, roleUser, *m.Role)

	// tool with tool call id
	m, err = convertMessage(&schema.Message{
		Role:       schema.Tool,
		Content:    "tool result",
		ToolCallID: "call_1",
	})
	assert.Nil(t, err)
	assert.Equal(t, roleTool, *m.Role)
	assert.Equal(t, "call_1", *m.ToolCallId)

	// assistant with tool calls + reasoning content
	idx := 0
	m, err = convertMessage(&schema.Message{
		Role:             schema.Assistant,
		ReasoningContent: "thinking",
		ToolCalls: []schema.ToolCall{
			{
				Index: &idx,
				ID:    "c1",
				Type:  "function",
				Function: schema.FunctionCall{
					Name:      "fn",
					Arguments: `{"a":1}`,
				},
			},
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, roleAssistant, *m.Role)
	assert.Equal(t, "thinking", *m.ReasoningContent)
	assert.Len(t, m.ToolCalls, 1)
	assert.Equal(t, "fn", *m.ToolCalls[0].Function.Name)

	// unknown role
	_, err = convertMessage(&schema.Message{Role: schema.RoleType("strange"), Content: "x"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown role type")

	// conflict: both UserInputMultiContent and AssistantGenMultiContent
	_, err = convertMessage(&schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "hi"},
		},
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "hi"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot contain both")

	// AssistantGenMultiContent text path
	m, err = convertMessage(&schema.Message{
		Role: schema.Assistant,
		AssistantGenMultiContent: []schema.MessageOutputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "hello"},
		},
	})
	assert.Nil(t, err)
	assert.Len(t, m.Contents, 1)
	assert.Equal(t, "hello", *m.Contents[0].Text)

	// MultiContent (deprecated) path
	m, err = convertMessage(&schema.Message{
		Role: schema.User,
		MultiContent: []schema.ChatMessagePart{
			{Type: schema.ChatMessagePartTypeText, Text: "deprecated"},
			{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "http://img"}},
			{Type: schema.ChatMessagePartTypeVideoURL, VideoURL: &schema.ChatMessageVideoURL{URL: "http://video"}},
		},
	})
	assert.Nil(t, err)
	assert.Len(t, m.Contents, 3)
}

func TestConvertInputMedia(t *testing.T) {
	// success: text + image url + image base64 + video
	videoURL := "http://video.example/v.mp4"
	parts := []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "hello"},
		{Type: schema.ChatMessagePartTypeText, Text: ""}, // skipped (empty)
		{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL: toPtr("http://img.example/x.png"),
				},
			},
		},
		{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: toPtr("YWJj"),
					MIMEType:   "image/png",
				},
			},
		},
		{
			Type: schema.ChatMessagePartTypeVideoURL,
			Video: &schema.MessageInputVideo{
				MessagePartCommon: schema.MessagePartCommon{
					URL: &videoURL,
				},
			},
		},
	}

	out, err := convertInputMedia(parts)
	assert.Nil(t, err)
	assert.Len(t, out, 4)

	// nil image error
	_, err = convertInputMedia([]schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image field must not be nil")

	// nil video error
	_, err = convertInputMedia([]schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeVideoURL, Video: nil},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "video field must not be nil")

	// video with empty URL
	_, err = convertInputMedia([]schema.MessageInputPart{
		{
			Type: schema.ChatMessagePartTypeVideoURL,
			Video: &schema.MessageInputVideo{
				MessagePartCommon: schema.MessagePartCommon{URL: toPtr("")},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "video field must not be nil")

	// image with neither URL nor base64
	_, err = convertInputMedia([]schema.MessageInputPart{
		{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image part must have either")

	// unsupported type
	_, err = convertInputMedia([]schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeAudioURL},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported chat message part type")
}

func TestConvertOutputMedia(t *testing.T) {
	videoURL := "http://video.example/v.mp4"
	parts := []schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "hi"},
		{Type: schema.ChatMessagePartTypeText, Text: ""}, // skipped
		{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageOutputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL: toPtr("http://img.example/x.png"),
				},
			},
		},
		{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageOutputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: toPtr("YWJj"),
					MIMEType:   "image/jpeg",
				},
			},
		},
		{
			Type: schema.ChatMessagePartTypeVideoURL,
			Video: &schema.MessageOutputVideo{
				MessagePartCommon: schema.MessagePartCommon{URL: &videoURL},
			},
		},
	}

	out, err := convertOutputMedia(parts)
	assert.Nil(t, err)
	assert.Len(t, out, 4)

	// nil image
	_, err = convertOutputMedia([]schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeImageURL},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image field must not be nil")

	// nil video
	_, err = convertOutputMedia([]schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeVideoURL},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "video field must not be nil")

	// image with no url/base64
	_, err = convertOutputMedia([]schema.MessageOutputPart{
		{
			Type:  schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageOutputImage{MessagePartCommon: schema.MessagePartCommon{}},
		},
	})
	assert.Error(t, err)

	// unsupported type
	_, err = convertOutputMedia([]schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeAudioURL},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported chat message part type")
}

func TestConvertMedia(t *testing.T) {
	parts := []schema.ChatMessagePart{
		{Type: schema.ChatMessagePartTypeText, Text: "hi"},
		{Type: schema.ChatMessagePartTypeText, Text: ""}, // skipped
		{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "http://img"}},
		{Type: schema.ChatMessagePartTypeVideoURL, VideoURL: &schema.ChatMessageVideoURL{URL: "http://video"}},
	}
	out, err := convertMedia(parts)
	assert.Nil(t, err)
	assert.Len(t, out, 3)

	// image nil
	_, err = convertMedia([]schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeImageURL}})
	assert.Error(t, err)

	// image empty url
	_, err = convertMedia([]schema.ChatMessagePart{
		{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: ""}},
	})
	assert.Error(t, err)

	// video nil
	_, err = convertMedia([]schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeVideoURL}})
	assert.Error(t, err)

	// video empty url
	_, err = convertMedia([]schema.ChatMessagePart{
		{Type: schema.ChatMessagePartTypeVideoURL, VideoURL: &schema.ChatMessageVideoURL{URL: ""}},
	})
	assert.Error(t, err)

	// unsupported type
	_, err = convertMedia([]schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeAudioURL}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported chat message part type")
}

func TestImageToUrlOrBase64(t *testing.T) {
	// url
	out, err := imageToUrlOrBase64(toPtr("http://x"), nil, "")
	assert.Nil(t, err)
	assert.Equal(t, "http://x", *out.Url)

	// base64
	out, err = imageToUrlOrBase64(nil, toPtr("YWJj"), "image/png")
	assert.Nil(t, err)
	assert.Equal(t, "data:image/png;base64,YWJj", *out.Url)

	// empty url & empty base64 -> error
	_, err = imageToUrlOrBase64(toPtr(""), toPtr(""), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image part must have either")

	// nil pointers
	_, err = imageToUrlOrBase64(nil, nil, "")
	assert.Error(t, err)
}

func TestPopulateToolChoice_ForcedAutoPick(t *testing.T) {
	tc := schema.ToolChoiceForced
	tool := &hunyuan.Tool{
		Function: &hunyuan.ToolFunction{Name: toPtr("only-tool")},
	}
	req := &hunyuan.ChatCompletionsRequest{
		Tools: []*hunyuan.Tool{tool},
	}
	err := populateToolChoice(req, &tc, nil)
	assert.NoError(t, err)
	assert.Equal(t, toolChoiceRequired, *req.ToolChoice)
	assert.Equal(t, tool, req.CustomTool)

	// multiple allowed names not supported
	req2 := &hunyuan.ChatCompletionsRequest{Tools: []*hunyuan.Tool{tool}}
	err = populateToolChoice(req2, &tc, []string{"a", "b"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one allowed tool name")

	// unsupported tool choice
	unknown := schema.ToolChoice("weird")
	err = populateToolChoice(&hunyuan.ChatCompletionsRequest{}, &unknown, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not support")
}

func TestBuildRequest_StopAndOptionsTools(t *testing.T) {
	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "id",
		SecretKey: "key",
		Model:     "hunyuan-lite",
		Stop:      []string{"\n", "User:"},
	})
	assert.Nil(t, err)

	// pass tools via options to cover the options.Tools branch
	req, cbInput, err := cm.buildRequest(
		[]*schema.Message{schema.UserMessage("hi")},
		false,
		model.WithTools([]*schema.ToolInfo{
			{
				Name: "opt_tool",
				Desc: "opt tool",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"p": {Type: schema.String, Desc: "p"},
				}),
			},
		}),
	)
	assert.Nil(t, err)
	assert.NotNil(t, cbInput)
	assert.Len(t, req.Stop, 2)
	assert.Equal(t, "\n", *req.Stop[0])
	assert.Len(t, req.Tools, 1)
	assert.Equal(t, "opt_tool", *req.Tools[0].Function.Name)
}

func TestBuildRequest_ConvertMessageError(t *testing.T) {
	ctx := context.Background()
	cm, err := NewChatModel(ctx, &ChatModelConfig{
		SecretId:  "id",
		SecretKey: "key",
		Model:     "hunyuan-lite",
	})
	assert.Nil(t, err)

	// invalid role triggers convertMessage error
	_, _, err = cm.buildRequest([]*schema.Message{
		{Role: schema.RoleType("invalid"), Content: "x"},
	}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown role type")
}

func TestGetType(t *testing.T) {
	cm := &ChatModel{}
	assert.Equal(t, typ, cm.GetType())
}
