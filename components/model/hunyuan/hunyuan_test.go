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
