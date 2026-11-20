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

package ark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/bytedance/mockey"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	toolcomponent "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
)

type textEnhancedTool struct{}

func (t *textEnhancedTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "text_enhanced_tool",
		Desc: "returns a text-only enhanced tool result",
	}, nil
}

func (t *textEnhancedTool) InvokableRun(context.Context, *schema.ToolArgument, ...toolcomponent.Option) (*schema.ToolResult, error) {
	return &schema.ToolResult{
		Parts: []schema.ToolOutputPart{
			{Type: schema.ToolPartTypeText, Text: "enhanced tool output"},
		},
	}, nil
}

func TestResponsesAPIChatModelGenerate(t *testing.T) {
	PatchConvey("test Generate", t, func() {
		Mock(callbacks.OnError).Return(context.Background()).Build()
		Mock((*ResponsesAPIChatModel).genRequestAndOptions).
			Return(&responses.ResponsesRequest{}, nil).Build()
		Mock((*ResponsesAPIChatModel).toCallbackConfig).
			Return(&model.Config{}).Build()
		MockGeneric(callbacks.OnStart[*callbacks.CallbackInput]).Return(context.Background()).Build()

		Mock((*arkruntime.Client).CreateResponses).
			Return(&responses.ResponseObject{
				Usage: &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{},
					OutputTokensDetails: &responses.OutputTokensDetails{ReasoningTokens: 100}},
			}, nil).Build()

		Mock((*ResponsesAPIChatModel).toOutputMessage).To(func(resp *responses.ResponseObject, cache *cacheConfig) (*schema.Message, error) {
			cm := &ResponsesAPIChatModel{}
			return &schema.Message{
				Role:    schema.Assistant,
				Content: "assistant",
				ResponseMeta: &schema.ResponseMeta{
					Usage: cm.toEinoTokenUsage(resp.Usage),
				},
			}, nil
		}).Build()

		MockGeneric(callbacks.OnEnd[*callbacks.CallbackOutput]).Return(context.Background()).Build()

		cm := &ResponsesAPIChatModel{}
		msg, err := cm.Generate(context.Background(), []*schema.Message{
			{
				Role:    schema.User,
				Content: "user",
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, 100, msg.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens)
		assert.Equal(t, "assistant", msg.Content)
	})
}

func TestResponsesAPIChatModelPreservesModelName(t *testing.T) {
	const modelName = "doubao-seed-2-0-lite"

	cm := &ResponsesAPIChatModel{}

	t.Run("non-streaming response", func(t *testing.T) {
		msg, err := cm.toOutputMessage(&responses.ResponseObject{
			Model:  modelName,
			Status: responses.ResponseStatus_completed,
			Usage:  &responses.Usage{},
			Output: []*responses.OutputItem{
				{
					Union: &responses.OutputItem_OutputMessage{
						OutputMessage: &responses.ItemOutputMessage{
							Content: []*responses.OutputContentItem{
								{
									Union: &responses.OutputContentItem_Text{
										Text: &responses.OutputContentItemText{Text: "hello"},
									},
								},
							},
						},
					},
				},
			},
		}, nil)
		assert.NoError(t, err)

		name, ok := GetModelName(msg)
		assert.True(t, ok)
		assert.Equal(t, modelName, name)
	})

	t.Run("concatenated streaming response", func(t *testing.T) {
		metadataMsg := &schema.Message{Role: schema.Assistant}
		cm.setStreamChunkDefaultExtra(metadataMsg, &responses.ResponseObject{
			Model: modelName,
		}, &cacheConfig{})

		textMsg := &schema.Message{
			Role:    schema.Assistant,
			Content: "hello",
		}
		msg, err := schema.ConcatMessages([]*schema.Message{
			metadataMsg,
			textMsg,
		})
		assert.NoError(t, err)

		name, ok := GetModelName(msg)
		assert.True(t, ok)
		assert.Equal(t, modelName, name)
	})
}

func TestResponsesAPIChatModelStream(t *testing.T) {
	PatchConvey("test Stream", t, func() {
		ctx := context.Background()
		sr, sw := schema.Pipe[*model.CallbackOutput](1)

		Mock(callbacks.OnError).Return(ctx).Build()

		Mock((*ResponsesAPIChatModel).genRequestAndOptions).
			Return(&responses.ResponsesRequest{}, nil).Build()

		Mock((*ResponsesAPIChatModel).toCallbackConfig).
			Return(&model.Config{}).Build()
		MockGeneric(callbacks.OnStart[*callbacks.CallbackInput]).Return(context.Background()).Build()

		Mock((*arkruntime.Client).CreateResponsesStream).
			Return(&utils.ResponsesStreamReader{}, nil).Build()

		Mock((*utils.ChatCompletionStreamReader).Close).Return(nil).Build()

		MockGeneric(schema.Pipe[*model.CallbackOutput]).
			Return(sr, sw).Build()

		Mock((*ResponsesAPIChatModel).receivedStreamResponse).Return().Build()

		cm := &ResponsesAPIChatModel{}
		stream, err := cm.Stream(context.Background(), []*schema.Message{
			{
				Role:    schema.User,
				Content: "user",
			},
		})
		assert.Nil(t, err)

		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			}
			assert.Nil(t, err)
			assert.Equal(t, "1", msg.Content)
		}
	})
}

func TestResponsesAPIChatModelInjectInput(t *testing.T) {
	cm := &ResponsesAPIChatModel{}

	PatchConvey("empty input message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		var in []*schema.Message
		err := cm.populateInput(in, req, false)
		assert.Nil(t, err)
	})

	PatchConvey("user message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		err := cm.populateInput(in, req, false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))
		item := req.GetInput().GetListValue().GetListValue()[0].GetInputMessage()
		assert.Equal(t, responses.MessageRole_user, item.Role)
		assert.Equal(t, "Hello", item.Content[0].GetText().GetText())
	})

	PatchConvey("assistant message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.Assistant,
				Content: "Hi there",
			},
		}

		err := cm.populateInput(in, req, false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))

		item := req.GetInput().GetListValue().GetListValue()[0].GetInputMessage()
		assert.Equal(t, responses.MessageRole_assistant, item.Role)
		assert.Equal(t, "Hi there", item.Content[0].GetText().GetText())
	})

	PatchConvey("system message", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    schema.System,
				Content: "You are a helpful assistant.",
			},
		}

		err := cm.populateInput(in, req, false)
		assert.Nil(t, err)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))

		item := req.GetInput().GetListValue().GetListValue()[0].GetInputMessage()
		assert.Equal(t, responses.MessageRole_system, item.Role)
		assert.Equal(t, "You are a helpful assistant.", item.Content[0].GetText().GetText())

	})
	//
	PatchConvey("tool call", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:       schema.Tool,
				ToolCallID: "call_123",
				Content:    "tool output",
			},
		}

		err := cm.populateInput(in, req, false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))

		item := req.GetInput().GetListValue().GetListValue()[0].GetFunctionToolCallOutput()
		assert.Equal(t, "call_123", item.CallId)
		assert.Equal(t, "tool output", item.Output)
	})

	PatchConvey("tool call with text-only multi content", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:       schema.Tool,
				ToolCallID: "call_123",
				UserInputMultiContent: []schema.MessageInputPart{
					{Type: schema.ChatMessagePartTypeText, Text: "tool "},
					{Type: schema.ChatMessagePartTypeText, Text: "output"},
				},
			},
		}

		err := cm.populateInput(in, req, false)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(req.GetInput().GetListValue().GetListValue()))

		item := req.GetInput().GetListValue().GetListValue()[0].GetFunctionToolCallOutput()
		assert.Equal(t, "call_123", item.CallId)
		assert.Equal(t, "tool output", item.Output)
	})

	PatchConvey("tool call with non-text multi content", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		imageURL := "https://example.com/tool-output.png"
		in := []*schema.Message{
			{
				Role:       schema.Tool,
				ToolCallID: "call_123",
				UserInputMultiContent: []schema.MessageInputPart{
					{
						Type: schema.ChatMessagePartTypeImageURL,
						Image: &schema.MessageInputImage{
							MessagePartCommon: schema.MessagePartCommon{URL: &imageURL},
						},
					},
				},
			},
		}

		err := cm.populateInput(in, req, false)
		assert.ErrorContains(t, err, "only supports text tool result")
	})

	PatchConvey("unknown role", t, func() {
		req := &responses.ResponsesRequest{
			Model: "test-model",
		}
		in := []*schema.Message{
			{
				Role:    "unknown_role",
				Content: "some content",
			},
		}
		err := cm.populateInput(in, req, false)
		assert.NotNil(t, err)
	})
}

func TestResponsesAPIChatModelPopulateInputFromEnhancedTool(t *testing.T) {
	ctx := context.Background()
	toolNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: []toolcomponent.BaseTool{&textEnhancedTool{}},
	})
	assert.NoError(t, err)
	if err != nil {
		return
	}

	toolMessages, err := toolNode.Invoke(ctx, schema.AssistantMessage("", []schema.ToolCall{
		{
			ID: "call_enhanced",
			Function: schema.FunctionCall{
				Name:      "text_enhanced_tool",
				Arguments: `{}`,
			},
		},
	}))
	assert.NoError(t, err)
	assert.Len(t, toolMessages, 1)
	if err != nil || len(toolMessages) != 1 {
		return
	}
	assert.Empty(t, toolMessages[0].Content)
	assert.Len(t, toolMessages[0].UserInputMultiContent, 1)

	req := &responses.ResponsesRequest{Model: "test-model"}
	err = (&ResponsesAPIChatModel{}).populateInput(toolMessages, req, false)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	item := req.GetInput().GetListValue().GetListValue()[0].GetFunctionToolCallOutput()
	assert.Equal(t, "call_enhanced", item.CallId)
	assert.Equal(t, "enhanced tool output", item.Output)
}

func TestResponsesAPIChatModelPopulateInputReasoningPassback(t *testing.T) {
	cm := &ResponsesAPIChatModel{}

	PatchConvey("enabled, no previous_response_id, has reasoning content", t, func() {
		req := &responses.ResponsesRequest{Model: "test-model"}
		in := []*schema.Message{
			{
				Role:             schema.Assistant,
				Content:          "Hi",
				ReasoningContent: "thinking step",
			},
		}
		err := cm.populateInput(in, req, true)
		assert.Nil(t, err)
		items := req.GetInput().GetListValue().GetListValue()
		assert.Equal(t, 2, len(items))
		reasoning := items[0].GetReasoning()
		assert.NotNil(t, reasoning)
		assert.Equal(t, "thinking step", reasoning.GetSummary()[0].Text)
		msg := items[1].GetInputMessage()
		assert.Equal(t, responses.MessageRole_assistant, msg.Role)
	})

	PatchConvey("enabled, has previous_response_id, should skip reasoning passback", t, func() {
		for _, previousResponseID := range []string{"resp_123", ""} {
			req := &responses.ResponsesRequest{
				Model:              "test-model",
				PreviousResponseId: ptrOf(previousResponseID),
			}
			in := []*schema.Message{
				{
					Role:             schema.Assistant,
					Content:          "Hi",
					ReasoningContent: "thinking step",
				},
			}
			err := cm.populateInput(in, req, true)
			assert.Nil(t, err)
			items := req.GetInput().GetListValue().GetListValue()
			assert.Equal(t, 1, len(items))
			assert.Nil(t, items[0].GetReasoning())
		}
	})

	PatchConvey("disabled, has reasoning content, should not passback", t, func() {
		req := &responses.ResponsesRequest{Model: "test-model"}
		in := []*schema.Message{
			{
				Role:             schema.Assistant,
				Content:          "Hi",
				ReasoningContent: "thinking step",
			},
		}
		err := cm.populateInput(in, req, false)
		assert.Nil(t, err)
		items := req.GetInput().GetListValue().GetListValue()
		assert.Equal(t, 1, len(items))
		assert.Nil(t, items[0].GetReasoning())
	})
}

func TestResponsesAPIChatModelRawReasoningContentResponse(t *testing.T) {
	const expectedRawReasoningContent = "\n用户现在问1+1等于几，首先正常的数学答案是2。"
	const outputText = "在最基础的十进制数学算术里，1+1=2。"
	responseJSON := `{
		"created_at": 1788353431,
		"id": "mock-response-id",
		"model": "mock-model",
		"object": "response",
		"output": [
			{
				"id": "mock-reasoning-id",
				"type": "reasoning",
				"status": "completed",
				"content": [
					{
						"type": "reasoning_text",
						"text": "\n用户现在问1+1等于几，首先正常的数学答案是2。"
					}
				]
			},
			{
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "output_text",
						"text": "在最基础的十进制数学算术里，1+1=2。"
					}
				],
				"status": "completed",
				"id": "mock-message-id"
			}
		],
		"thinking": {"type": "enabled"},
		"status": "completed",
		"usage": {
			"input_tokens": 54,
			"output_tokens": 305,
			"total_tokens": 359,
			"input_tokens_details": {"cached_tokens": 0},
			"output_tokens_details": {"reasoning_tokens": 203}
		}
	}`

	var resp responses.ResponseObject
	err := json.Unmarshal([]byte(responseJSON), &resp)
	assert.NoError(t, err)
	if assert.Len(t, resp.Output, 2) {
		assert.Len(t, resp.Output[0].GetReasoning().GetContent(), 1,
			"SDK must preserve reasoning.content from the API response")
	}

	cm := &ResponsesAPIChatModel{}
	msg, err := cm.toOutputMessage(&resp, nil)
	assert.NoError(t, err)
	assert.Empty(t, msg.ReasoningContent)
	assert.Equal(t, outputText, msg.Content)
	rawReasoning, ok := GetRawReasoningContent(&msg.AssistantGenMultiContent[0])
	assert.True(t, ok)
	assert.Equal(t, []string{expectedRawReasoningContent}, rawReasoning)

	req := &responses.ResponsesRequest{Model: "mock-model"}
	err = cm.populateInput([]*schema.Message{msg}, req, true)
	assert.NoError(t, err)
	items := req.GetInput().GetListValue().GetListValue()
	if assert.Len(t, items, 2) {
		reasoning := items[0].GetReasoning()
		if assert.NotNil(t, reasoning) {
			assert.Empty(t, reasoning.GetSummary())
			if assert.Len(t, reasoning.GetContent(), 1) {
				assert.Equal(t, responses.ContentItemType_reasoning_text, reasoning.GetContent()[0].GetText().GetType())
				assert.Equal(t, expectedRawReasoningContent, reasoning.GetContent()[0].GetText().GetText())
			}
		}
	}
}

func TestResponsesAPIChatModelReasoningSummaryResponse(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	msg, err := cm.toOutputMessage(&responses.ResponseObject{
		Status: responses.ResponseStatus_completed,
		Output: []*responses.OutputItem{
			{
				Union: &responses.OutputItem_Reasoning{
					Reasoning: &responses.ItemReasoning{
						Summary: []*responses.ReasoningSummaryPart{
							{Text: "first summary"},
							{Text: "second summary"},
						},
					},
				},
			},
		},
		Usage: &responses.Usage{
			InputTokensDetails:  &responses.InputTokensDetails{},
			OutputTokensDetails: &responses.OutputTokensDetails{},
		},
	}, nil)

	assert.NoError(t, err)
	assert.Equal(t, "first summary\n\nsecond summary", msg.ReasoningContent)
	_, ok := GetRawReasoningContent(&msg.AssistantGenMultiContent[0])
	assert.False(t, ok)
}

func TestResponsesAPIChatModelToOpenaiMultiModalContent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}

	PatchConvey("image message", t, func() {
		msg := &schema.Message{
			Role: schema.User,
			UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
					MessagePartCommon: schema.MessagePartCommon{
						URL: ptrOf("https://example.com/image.png"),
					},
				}},
			},
		}

		content, err := cm.toArkUserRoleItemInputMessage(msg)
		assert.Nil(t, err)

		contentList := content.Content
		assert.Equal(t, 1, len(contentList))
		assert.Equal(t, "https://example.com/image.png", *contentList[0].GetImage().ImageUrl)
	})

	PatchConvey("unknown modal type", t, func() {
		msg := &schema.Message{
			Role: schema.User,
			MultiContent: []schema.ChatMessagePart{
				{
					Type: "unsupported_type",
				},
			},
		}
		_, err := cm.toArkUserRoleItemInputMessage(msg)
		assert.NotNil(t, err)
	})
}

func TestResponsesAPIChatModelToTools(t *testing.T) {
	cm := &ResponsesAPIChatModel{}

	PatchConvey("empty tools", t, func() {
		tools := []*schema.ToolInfo{}
		openAITools, err := cm.toTools(tools)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(openAITools))
	})

	PatchConvey("single tool", t, func() {
		tools := []*schema.ToolInfo{
			{
				Name: "test tool",
				Desc: "description of test tool",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"param": {
						Type:     schema.String,
						Desc:     "description of param1",
						Required: true,
					},
				}),
			},
		}
		responsesTools, err := cm.toTools(tools)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(responsesTools))
		assert.Equal(t, tools[0].Name, responsesTools[0].GetToolFunction().Name)
		assert.Equal(t, "description of test tool", *responsesTools[0].GetToolFunction().Description)
		assert.NotNil(t, responsesTools[0].GetToolFunction().Parameters.GetValue())
	})
}

func TestResponsesAPIChatModelInjectCache(t *testing.T) {
	PatchConvey("not configure", t, func() {
		var (
			cm = &ResponsesAPIChatModel{}
		)
		arkOpts := &arkOptions{}
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
			},
		}

		reqParams := &responses.ResponsesRequest{}

		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, false, *reqParams.Store)
		assert.Nil(t, reqParams.Caching)
		assert.Len(t, in_, 1)
	})

	PatchConvey("enable cache", t, func() {
		cm := &ResponsesAPIChatModel{
			cache: &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
				},
			},
		}
		arkOpts := &arkOptions{}
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
				Extra: map[string]any{
					keyOfResponseID:            "test-response-id",
					keyOfResponseCacheExpireAt: time.Now().Unix() + 259200,
				},
			},
			{
				Role:    schema.User,
				Content: "World",
			},
		}
		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, true, *reqParams.Store)
		assert.Equal(t, "test-response-id", *reqParams.PreviousResponseId)
		assert.Len(t, in_, 1)
		assert.Equal(t, "World", in_[0].Content)
		assert.NotNil(t, reqParams.ExpireAt)
		assert.NotNil(t, reqParams.Caching)
		assert.Equal(t, responses.CacheType_enabled, *reqParams.Caching.Type)
	})
	PatchConvey("option overridden config", t, func() {
		cm := &ResponsesAPIChatModel{
			cache: &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: false,
				},
			},
		}

		contextID := "test-context"
		arkOpts := &arkOptions{
			cache: &CacheOption{
				ContextID: &contextID,
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
				},
			},
		}
		msgs := []*schema.Message{
			{
				Role:    schema.User,
				Content: "Hello",
				Extra: map[string]any{
					keyOfResponseID:            "test-response-id",
					keyOfResponseCacheExpireAt: time.Now().Unix() + 259200,
				},
			},
			{
				Role:    schema.User,
				Content: "World",
			},
		}

		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		//assert.Equal(t, initialReqOptsLen+2, len(reqParams.opts))
		assert.Equal(t, true, *reqParams.Store)
		assert.Equal(t, "test-context", *reqParams.PreviousResponseId)
		assert.Len(t, in_, 2)
		assert.NotNil(t, reqParams.ExpireAt)
		assert.NotNil(t, reqParams.Caching)
		assert.Equal(t, responses.CacheType_enabled, *reqParams.Caching.Type)
	})

	PatchConvey("config disabled, option not set", t, func() {
		cm := &ResponsesAPIChatModel{
			cache: &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: false,
					TTL:         300,
				},
			},
		}
		arkOpts := &arkOptions{}
		msgs := []*schema.Message{
			{Role: schema.User, Content: "Hello"},
		}
		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, false, *reqParams.Store)
		assert.NotNil(t, reqParams.Caching)
		assert.Equal(t, responses.CacheType_disabled, *reqParams.Caching.Type)
		assert.Len(t, in_, 1)
	})

	PatchConvey("config enabled, option explicitly disables", t, func() {
		cm := &ResponsesAPIChatModel{
			cache: &CacheConfig{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
					TTL:         300,
				},
			},
		}
		arkOpts := &arkOptions{
			cache: &CacheOption{
				SessionCache: &SessionCacheConfig{
					EnableCache: false,
					TTL:         600,
				},
			},
		}
		msgs := []*schema.Message{
			{Role: schema.User, Content: "Hello"},
		}
		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, false, *reqParams.Store)
		assert.NotNil(t, reqParams.Caching)
		assert.Equal(t, responses.CacheType_disabled, *reqParams.Caching.Type)
		assert.Len(t, in_, 1)
	})

	PatchConvey("config not set, option enabled", t, func() {
		cm := &ResponsesAPIChatModel{}
		arkOpts := &arkOptions{
			cache: &CacheOption{
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
					TTL:         300,
				},
			},
		}
		msgs := []*schema.Message{
			{Role: schema.User, Content: "Hello"},
		}
		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, true, *reqParams.Store)
		assert.NotNil(t, reqParams.Caching)
		assert.Equal(t, responses.CacheType_enabled, *reqParams.Caching.Type)
		assert.Len(t, in_, 1)
	})

	PatchConvey("config not set, option disabled", t, func() {
		cm := &ResponsesAPIChatModel{}
		arkOpts := &arkOptions{
			cache: &CacheOption{
				SessionCache: &SessionCacheConfig{
					EnableCache: false,
					TTL:         300,
				},
			},
		}
		msgs := []*schema.Message{
			{Role: schema.User, Content: "Hello"},
		}
		reqParams := &responses.ResponsesRequest{}
		in_, err := cm.populateCache(msgs, reqParams, arkOpts)
		assert.Nil(t, err)
		assert.Equal(t, false, *reqParams.Store)
		assert.NotNil(t, reqParams.Caching)
		assert.Equal(t, responses.CacheType_disabled, *reqParams.Caching.Type)
		assert.Len(t, in_, 1)
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseCreatedEvent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}

	PatchConvey("ResponseCreatedEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Response{
				Response: &responses.ResponseEvent{
					Response: &responses.ResponseObject{},
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		mocker := Mock((*ResponsesAPIChatModel).sendCallbackOutput).Return().Build()
		streamReader := &utils.ResponsesStreamReader{}
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseCompletedEvent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	PatchConvey("ResponseCompletedEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_ResponseCompleted{
				ResponseCompleted: &responses.ResponseCompletedEvent{
					Response: &responses.ResponseObject{
						Usage: &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{}},
					},
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		mocker := Mock((*ResponsesAPIChatModel).sendCallbackOutput).Return().Build()
		streamReader := &utils.ResponsesStreamReader{}
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil, nil)
		assert.Equal(t, 1, mocker.Times())
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseErrorEvent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	PatchConvey("ResponseErrorEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Error{
				Error: &responses.ErrorEvent{
					Message: "error msg",
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		sr, sw := schema.Pipe[*model.CallbackOutput](1)
		streamReader := &utils.ResponsesStreamReader{}
		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, sw, nil)

		_, err := sr.Recv()
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "error msg")
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseIncompleteEvent(t *testing.T) {

	cm := &ResponsesAPIChatModel{}
	PatchConvey("ResponseIncompleteEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_ResponseIncomplete{
				ResponseIncomplete: &responses.ResponseIncompleteEvent{
					Response: &responses.ResponseObject{
						IncompleteDetails: &responses.IncompleteDetails{},
						Usage:             &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{}},
					},
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		streamReader := &utils.ResponsesStreamReader{}
		mocker := Mock((*ResponsesAPIChatModel).sendCallbackOutput).Return().Build()

		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil, nil)

		assert.Equal(t, 1, mocker.Times())
	})

}

func TestResponsesAPIChatModelReceivedStreamResponse_ResponseFailedEvent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	PatchConvey("ResponseFailedEvent", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(&responses.Event{
			Event: &responses.Event_ResponseFailed{
				ResponseFailed: &responses.ResponseFailedEvent{
					Response: &responses.ResponseObject{},
				},
			},
		}, nil).Build()
		sr, sw := schema.Pipe[*model.CallbackOutput](1)
		defer sr.Close()
		defer sw.Close()
		cm.receivedStreamResponse(&utils.ResponsesStreamReader{}, nil, &cacheConfig{}, sw, nil)
		_, err := sr.Recv()
		var apiErr *arkModel.APIError
		require.True(t, errors.As(err, &apiErr))
		require.Equal(t, "response.failed", apiErr.Type)
	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_Default(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	PatchConvey("Default", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Text{
				Text: &responses.OutputTextEvent{
					Delta: ptrOf("ok"),
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		streamReader := &utils.ResponsesStreamReader{}
		mocker := Mock((*ResponsesAPIChatModel).sendCallbackOutput).Return().Build()

		cm.receivedStreamResponse(streamReader, nil, &cacheConfig{Enabled: true}, nil, nil)

		assert.Equal(t, 1, mocker.Times())

	})
}

func TestResponsesAPIChatModelReceivedStreamResponse_RawReasoningContent(t *testing.T) {
	const eventStream = "event: response.reasoning_text.delta\n" +
		"data: {\"type\":\"response.reasoning_text.delta\",\"content_index\":0," +
		"\"delta\":\"raw \",\"item_id\":\"mock-reasoning-id\",\"output_index\":0,\"sequence_number\":1}\n\n" +
		"event: response.reasoning_text.delta\n" +
		"data: {\"type\":\"response.reasoning_text.delta\",\"content_index\":0," +
		"\"delta\":\"reasoning\",\"item_id\":\"mock-reasoning-id\",\"output_index\":0,\"sequence_number\":2}\n\n" +
		"data: [DONE]\n\n"

	body := io.NopCloser(strings.NewReader(eventStream))
	streamReader := &utils.ResponsesStreamReader{
		ChatCompletionStreamReader: utils.ChatCompletionStreamReader{
			Unmarshaler: &utils.JSONUnmarshaler{},
		},
		Decoder: utils.NewEventStreamDecoder(body),
	}
	sr, sw := schema.Pipe[*model.CallbackOutput](2)
	defer sr.Close()

	cm := &ResponsesAPIChatModel{}
	cm.receivedStreamResponse(streamReader, &model.Config{}, nil, sw, nil)
	sw.Close()

	chunks := make([]*schema.Message, 0, 2)
	for {
		output, err := sr.Recv()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		if assert.NotNil(t, output) && assert.NotNil(t, output.Message) {
			chunks = append(chunks, output.Message)
		}
	}

	msg, err := schema.ConcatMessages(chunks)
	assert.NoError(t, err)
	assert.Empty(t, msg.ReasoningContent)
	rawReasoning, ok := GetRawReasoningContent(&msg.AssistantGenMultiContent[0])
	assert.True(t, ok)
	assert.Equal(t, []string{"raw reasoning"}, rawReasoning)
}

func TestResponsesAPIChatModelReceivedStreamResponse_ToolCallMetaMsg(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	PatchConvey("ToolCallMetaMsg", t, func() {
		Mock((*utils.ResponsesStreamReader).Recv).Return(Sequence(&responses.Event{
			Event: &responses.Event_Item{
				Item: &responses.ItemEvent{
					Item: &responses.OutputItem{
						Union: &responses.OutputItem_FunctionToolCall{
							FunctionToolCall: &responses.ItemFunctionToolCall{
								Id:     ptrOf("123"),
								CallId: "123",
								Name:   "test",
								Type:   responses.ItemType_function_call,
							},
						},
					},
				},
			},
		}, nil).Then(&responses.Event{
			Event: &responses.Event_FunctionCallArguments{
				FunctionCallArguments: &responses.FunctionCallArgumentsEvent{
					Delta:  ptrOf("arguments"),
					ItemId: "123",
				},
			},
		}, nil).Then(nil, io.EOF)).Build()
		streamReader := &utils.ResponsesStreamReader{}

		mocker := Mock((*ResponsesAPIChatModel).sendCallbackOutput).To(
			func(sw *schema.StreamWriter[*model.CallbackOutput], reqConf *model.Config, modelName string, msg *schema.Message, thinking *arkModel.Thinking) {
				assert.Equal(t, "123", msg.ToolCalls[0].ID)
				assert.Equal(t, "test", msg.ToolCalls[0].Function.Name)
				assert.Equal(t, "arguments", msg.ToolCalls[0].Function.Arguments)
				assert.Equal(t, "function_call", msg.ToolCalls[0].Type)
			}).Build()

		cache := &cacheConfig{Enabled: true}

		cm.receivedStreamResponse(streamReader, nil, cache, nil, nil)

		assert.Equal(t, 1, mocker.Times())

	})
}

func TestResponsesAPIChatModelHandleGenRequestAndOptions(t *testing.T) {
	cm := &ResponsesAPIChatModel{
		temperature: ptrOf(float32(1.0)),
		maxTokens:   ptrOf(1),
		model:       "model",
		topP:        ptrOf(float32(1.0)),
		serviceTier: ptrOf("fast"),
		thinking: &arkModel.Thinking{
			Type: arkModel.ThinkingTypeDisabled,
		},
		customHeader: map[string]string{
			"h1": "v1",
		},
		responseFormat: &ResponseFormat{
			Type: arkModel.ResponseFormatJSONSchema,
			JSONSchema: &arkModel.ResponseFormatJSONSchemaJSONSchemaParam{
				Name: "json_schema",
			},
		},
	}

	PatchConvey("vv", t, func() {
		Mock((*ResponsesAPIChatModel).checkOptions).To(func(mOpts *model.Options, arkOpts *arkOptions) error {
			assert.Equal(t, int(float32(2.0)), int(*mOpts.Temperature))
			assert.Equal(t, 2, *mOpts.MaxTokens)
			assert.Equal(t, int(float32(2.0)), int(*mOpts.TopP))
			assert.Equal(t, "model2", *mOpts.Model)

			assert.Equal(t, arkModel.ThinkingTypeAuto, arkOpts.thinking.Type)
			assert.Len(t, arkOpts.customHeaders, 2)
			assert.Equal(t, "v2", arkOpts.customHeaders["h2"])
			assert.Equal(t, "v3", arkOpts.customHeaders["h3"])

			return nil
		}).Build()

		Mock((*ResponsesAPIChatModel).populateCache).To(func(in []*schema.Message, respRequest *responses.ResponsesRequest, arkOpts *arkOptions,
		) ([]*schema.Message, error) {
			return in, nil
		}).Build()

		in := []*schema.Message{
			{
				Role:    schema.User,
				Content: "user",
			},
		}

		opts := []model.Option{
			model.WithTemperature(2.0),
			model.WithMaxTokens(2),
			model.WithTopP(2.0),
			model.WithModel("model2"),
			model.WithTools([]*schema.ToolInfo{
				{
					Name: "test tool",
					Desc: "description of test tool",
					ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
						"param": {
							Type:     schema.String,
							Desc:     "description of param1",
							Required: true,
						},
					}),
				},
			}),
			WithThinking(&arkModel.Thinking{Type: arkModel.ThinkingTypeAuto}),
			WithCustomHeader(map[string]string{
				"h2": "v2",
				"h3": "v3",
			}),
		}

		options, specOptions, err := cm.getOptions(opts)
		assert.NoError(t, err)

		reqParams, err := cm.genRequestAndOptions(in, options, specOptions)
		assert.Nil(t, err)
		assert.Equal(t, "model2", reqParams.Model)
		assert.Len(t, reqParams.Input.GetListValue().GetListValue(), 1)
		assert.Equal(t, "user", reqParams.Input.GetListValue().ListValue[0].GetInputMessage().GetContent()[0].GetText().GetText())
		assert.Len(t, reqParams.Tools, 1)
		assert.Equal(t, "test tool", reqParams.Tools[0].GetToolFunction().Name)
		assert.Equal(t, responses.ResponsesServiceTier_fast, *reqParams.ServiceTier)

		assert.Equal(t, "json_schema", reqParams.Text.Format.GetName())
	})
}

func TestResponsesAPIChatModel_toOpenaiMultiModalContent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	base64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
	httpURL := "https://example.com/image.png"

	PatchConvey("Test toOpenaiMultiModalContent Comprehensive", t, func() {
		PatchConvey("Pure Text Content", func() {
			msg := &schema.Message{Role: schema.User, Content: "just text"}
			inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
			assert.Nil(t, err)
			assert.Equal(t, "just text", inputMessage.Content[0].GetText().GetText())
		})

		PatchConvey("UserInputMultiContent", func() {
			PatchConvey("Success with all types", func() {
				msg := &schema.Message{
					Role:    schema.User,
					Content: "initial text",
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeText, Text: " more text"},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
						{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{URL: &httpURL}}},
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "image/png"}}},
						{Type: schema.ChatMessagePartTypeFileURL, File: &schema.MessageInputFile{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data, MIMEType: "application/pdf"}}},
					},
				}
				inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.Nil(t, err)
				assert.Len(t, inputMessage.Content, 5)
			})

			PatchConvey("Error on missing MIMEType for Base64", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &base64Data}}},
					},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "message part must have MIMEType when use Base64Data")
			})

			PatchConvey("Error on nil Image", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "image field must not be nil")
			})

			PatchConvey("Audio success with URL", func() {
				audioURL := "https://example.com/audio.mp3"
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{
							MessagePartCommon: schema.MessagePartCommon{URL: &audioURL},
						}},
					},
				}
				inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.Nil(t, err)
				assert.Len(t, inputMessage.Content, 1)
				audioItem := inputMessage.Content[0].GetAudio()
				assert.NotNil(t, audioItem)
				assert.Equal(t, audioURL, audioItem.AudioUrl)
				assert.Equal(t, responses.ContentItemType_input_audio, audioItem.Type)
			})

			PatchConvey("Audio success with Base64 and MIMEType", func() {
				audioB64 := "SGVsbG9BdWRpbw=="
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{
							MessagePartCommon: schema.MessagePartCommon{Base64Data: &audioB64, MIMEType: "audio/mpeg"},
						}},
					},
				}
				inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.Nil(t, err)
				assert.Len(t, inputMessage.Content, 1)
				audioItem := inputMessage.Content[0].GetAudio()
				assert.NotNil(t, audioItem)
				assert.Contains(t, audioItem.AudioUrl, "data:audio/mpeg;base64,")
			})

			PatchConvey("Audio error on nil Audio", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeAudioURL, Audio: nil},
					},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "audio field must not be nil")
			})

			PatchConvey("Audio error on missing URL and Base64", func() {
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{}},
					},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "must have URL or Base64Data")
			})

			PatchConvey("Audio error on missing MIMEType for Base64", func() {
				audioB64 := "SGVsbG9BdWRpbw=="
				msg := &schema.Message{
					Role: schema.User,
					UserInputMultiContent: []schema.MessageInputPart{
						{Type: schema.ChatMessagePartTypeAudioURL, Audio: &schema.MessageInputAudio{
							MessagePartCommon: schema.MessagePartCommon{Base64Data: &audioB64},
						}},
					},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "must have MIMEType when use Base64Data")
			})

		})

		PatchConvey("AssistantGenMultiContent", func() {
			PatchConvey("Success with all types", func() {
				msg := &schema.Message{
					Role:    schema.Assistant,
					Content: "assistant text",
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeText, Text: " more assistant text"},
					},
				}
				inputMessage, err := cm.toArkAssistantRoleItemInputMessage(msg)
				assert.Nil(t, err)
				assert.Len(t, inputMessage.Content, 1)
			})

			PatchConvey("Error on wrong role", func() {
				msg := &schema.Message{
					Role: schema.User,
					AssistantGenMultiContent: []schema.MessageOutputPart{{
						Type: schema.ChatMessagePartTypeText,
						Text: " more assistant text",
					}},
				}
				_, err := cm.toArkUserRoleItemInputMessage(msg)
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, "if user role, AssistantGenMultiContent cannot be set")
			})

			PatchConvey("Error on nil Image", func() {
				msg := &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{
						{Type: schema.ChatMessagePartTypeImageURL, Image: nil},
					},
				}
				_, err := cm.toArkAssistantRoleItemInputMessage(msg)
				assert.NotNil(t, err)

			})

		})

		PatchConvey("MultiContent (Legacy 1)", func() {
			msg := &schema.Message{
				Content: "legacy text",
			}
			inputMessage, err := cm.toArkAssistantRoleItemInputMessage(msg)
			assert.Nil(t, err)
			assert.Len(t, inputMessage.Content, 1)
		})

		PatchConvey("MultiContent (Legacy 2", func() {
			msg := &schema.Message{
				MultiContent: []schema.ChatMessagePart{
					{Type: schema.ChatMessagePartTypeText, Text: " more legacy text"},
					{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: httpURL}},
					{Type: schema.ChatMessagePartTypeFileURL, FileURL: &schema.ChatMessageFileURL{URL: httpURL}},
				},
			}
			inputMessage, err := cm.toArkUserRoleItemInputMessage(msg)
			assert.Nil(t, err)
			assert.Len(t, inputMessage.Content, 3)
		})

		PatchConvey("Error on both UserInputMultiContent and AssistantGenMultiContent", func() {
			msg := &schema.Message{
				UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: "user"}},
				AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: "assistant"}},
			}
			_, err := cm.toArkAssistantRoleItemInputMessage(msg)
			assert.NotNil(t, err)
			assert.ErrorContains(t, err, "if assistant role, UserInputMultiContent cannot be set")
		})
	})
}

func Test_responsesAPIChatModel_handleCompletedStreamEvent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	msg := cm.handleCompletedStreamEvent(&responses.ResponseObject{
		Status: responses.ResponseStatus_completed,
		Usage:  &responses.Usage{InputTokensDetails: &responses.InputTokensDetails{}},
	})
	assert.Equal(t, responses.ResponseStatus_completed.String(), msg.ResponseMeta.FinishReason)

}

func TestResponsesAPIChatModel_populateToolChoice(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	convey.Convey("TestPopulateToolChoice", t, func() {
		convey.Convey("no tool choice", func() {
			req := &responses.ResponsesRequest{}
			options := &model.Options{}
			err := cm.populateTools(req, options, nil, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.ToolChoice, convey.ShouldBeNil)
		})
		convey.Convey("tool choice forbidden", func() {
			req := &responses.ResponsesRequest{}
			toolChoice := schema.ToolChoiceForbidden
			options := &model.Options{
				ToolChoice: &toolChoice,
			}
			err := cm.populateTools(req, options, nil, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.ToolChoice.GetMode(), convey.ShouldEqual, responses.ToolChoiceMode_none)
		})
		convey.Convey("tool choice allowed", func() {
			req := &responses.ResponsesRequest{}
			toolChoice := schema.ToolChoiceAllowed
			options := &model.Options{
				ToolChoice: &toolChoice,
			}
			err := cm.populateTools(req, options, nil, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.ToolChoice.GetMode(), convey.ShouldEqual, responses.ToolChoiceMode_auto)
		})
		convey.Convey("tool choice forced with no tools", func() {
			req := &responses.ResponsesRequest{}
			toolChoice := schema.ToolChoiceForced
			options := &model.Options{
				ToolChoice: &toolChoice,
			}
			err := cm.populateTools(req, options, nil, nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "tool_choice is forced but no tools are provided")
		})
		convey.Convey("tool choice forced with tools", func() {
			req := &responses.ResponsesRequest{}
			toolChoice := schema.ToolChoiceForced
			options := &model.Options{
				ToolChoice: &toolChoice,
				Tools: []*schema.ToolInfo{
					{
						Name: "my_func",
						Desc: "description",
						ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
							"param1": {Type: schema.String},
						}),
					},
					{
						Name: "my_func2",
						Desc: "description",
						ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
							"param1": {Type: schema.String},
						}),
					},
				},
			}
			err := cm.populateTools(req, options, nil, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.ToolChoice.GetMode(), convey.ShouldEqual, responses.ToolChoiceMode_required)
		})

		convey.Convey("tool choice forced with web search", func() {
			req := &responses.ResponsesRequest{}
			toolChoice := schema.ToolChoiceForced
			options := &model.Options{
				ToolChoice: &toolChoice,
				Tools: []*schema.ToolInfo{
					{
						Name: "my_func",
						Desc: "description",
						ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
							"param1": {Type: schema.String},
						}),
					},
					{
						Name: "my_func2",
						Desc: "description",
						ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
							"param1": {Type: schema.String},
						}),
					},
				},
			}
			err := cm.populateTools(req, options, &ToolWebSearch{
				Limit:   ptrOf(int64(2)),
				Sources: []Source{SourceOfToutiao},
			}, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.ToolChoice.GetMode(), convey.ShouldEqual, responses.ToolChoiceMode_required)
			convey.So(len(req.Tools), convey.ShouldEqual, 3)
		})
		convey.Convey("tool choice forced with allowed tool name", func() {
			req := &responses.ResponsesRequest{}
			toolChoice := schema.ToolChoiceForced
			options := &model.Options{
				ToolChoice:       &toolChoice,
				AllowedToolNames: []string{"my_func"},
				Tools: []*schema.ToolInfo{
					{
						Name: "my_func",
						Desc: "description",
						ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
							"param1": {Type: schema.String},
						}),
					},
				},
			}
			err := cm.populateTools(req, options, nil, nil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(req.ToolChoice.GetFunctionToolChoice(), convey.ShouldEqual, &responses.FunctionToolChoice{
				Name: "my_func",
				Type: responses.ToolType_function,
			})
		})

	})
}
func TestNewResponsesAPIChatModel(t *testing.T) {
	PatchConvey("TestNewResponsesAPIChatModel", t, func() {
		ctx := context.Background()

		PatchConvey("NilConfig", func() {
			m, err := NewResponsesAPIChatModel(ctx, nil)
			assert.Error(t, err)
			assert.Nil(t, m)
			assert.Equal(t, "config cannot be nil", err.Error())
		})

		PatchConvey("NoCredentials", func() {
			config := &ResponsesAPIConfig{
				Model: "test-model",
			}
			m, err := NewResponsesAPIChatModel(ctx, config)
			assert.Error(t, err)
			assert.Nil(t, m)
			assert.Contains(t, err.Error(), "missing credentials")
		})

		PatchConvey("PartialCredentials", func() {
			config := &ResponsesAPIConfig{
				Model:     "test-model",
				AccessKey: "test-access-key",
			}
			m, err := NewResponsesAPIChatModel(ctx, config)
			assert.Error(t, err)
			assert.Nil(t, m)
			assert.Contains(t, err.Error(), "missing credentials")
		})

		PatchConvey("SuccessWithAPIKey", func() {
			config := &ResponsesAPIConfig{
				Model:  "test-model",
				APIKey: "test-api-key",
			}
			m, err := NewResponsesAPIChatModel(ctx, config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			assert.Equal(t, "test-model", m.model)
		})

		PatchConvey("SuccessWithAkSk", func() {
			config := &ResponsesAPIConfig{
				Model:     "test-model",
				AccessKey: "test-access-key",
				SecretKey: "test-secret-key",
			}
			m, err := NewResponsesAPIChatModel(ctx, config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			assert.Equal(t, "test-model", m.model)
		})

		PatchConvey("SuccessWithDefaults", func() {
			config := &ResponsesAPIConfig{
				Model:  "test-model",
				APIKey: "test-api-key",
			}
			m, err := NewResponsesAPIChatModel(ctx, config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			// check default values are not implemented in the function itself, but in the arkruntime client
		})

		PatchConvey("FullConfig", func() {
			timeout := 10 * time.Second
			retryTimes := 3
			maxTokens := 1024
			temperature := float32(0.8)
			topP := float32(0.9)
			config := &ResponsesAPIConfig{
				Timeout:         &timeout,
				RetryTimes:      &retryTimes,
				BaseURL:         "https://example.com",
				Region:          "us-east-1",
				APIKey:          "test-api-key",
				Model:           "test-model-full",
				MaxOutputTokens: &maxTokens,
				Temperature:     &temperature,
				TopP:            &topP,
				CustomHeader: map[string]string{
					"X-Custom": "true",
				},
				ResponseFormat: &ResponseFormat{
					Type: arkModel.ResponseFormatJsonObject,
				},
				Thinking: &arkModel.Thinking{
					Type: arkModel.ThinkingTypeAuto,
				},
				ServiceTier:     ptrOf("tier1"),
				ReasoningEffort: ptrOf(arkModel.ReasoningEffortLow),
				SessionCache: &SessionCacheConfig{
					EnableCache: true,
				},
				EnableToolWebSearch: &ToolWebSearch{},
				MaxToolCalls:        ptrOf(int64(5)),

				EnableReasoningContentPassback: true,
			}
			m, err := NewResponsesAPIChatModel(ctx, config)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			assert.Equal(t, "test-model-full", m.model)
			assert.Equal(t, &maxTokens, m.maxTokens)
			assert.Equal(t, &temperature, m.temperature)
			assert.Equal(t, &topP, m.topP)
			assert.Equal(t, "true", m.customHeader["X-Custom"])
			assert.NotNil(t, m.responseFormat)
			assert.NotNil(t, m.thinking)
			assert.NotNil(t, m.cache.SessionCache)
			assert.True(t, m.cache.SessionCache.EnableCache)
			assert.NotNil(t, m.serviceTier)
			assert.NotNil(t, m.reasoningEffort)
			assert.NotNil(t, m.enableToolWebSearch)
			assert.Equal(t, int64(5), *m.maxToolCalls)
			assert.True(t, m.enableReasoningContentPassback)
		})
	})
}

func TestResponsesAPIChatModel_CreatePrefixCache(t *testing.T) {
	PatchConvey("Test CreatePrefixCache", t, func() {
		ctx := context.Background()

		chatModel := &ResponsesAPIChatModel{}

		PatchConvey("Success", func() {
			var ttl = 3600
			exAt := ptrOf(time.Now().Unix() + int64(ttl))
			Mock((*arkruntime.Client).CreateResponses).Return(&responses.ResponseObject{Id: "test-cache-id", Usage: &responses.Usage{}, ExpireAt: exAt}, nil).Build()

			prefix := []*schema.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			}

			info, err := chatModel.CreatePrefixCache(ctx, prefix, ttl)

			assert.NoError(t, err)
			assert.NotNil(t, info)
			assert.Equal(t, "test-cache-id", info.ResponseID)

		})
		PatchConvey("Error: Nil Prefix", func() {
			info, err := chatModel.CreatePrefixCache(ctx, nil, 3600)
			assert.Error(t, err)
			assert.Nil(t, info)
			assert.Equal(t, "prefix messages cannot be empty", err.Error())
		})

	})
}

func encryptedFixture(t *testing.T, output string) *responses.ResponseObject {
	t.Helper()
	var response responses.ResponseObject
	require.NoError(t, json.Unmarshal([]byte(`{"id":"resp_1","status":"completed","output":`+output+`,"usage":{"input_tokens_details":{},"output_tokens_details":{}}}`), &response))
	return &response
}

func TestResponsesIncompleteKeepsMetadataOnly(t *testing.T) {
	response := encryptedFixture(t, `[
		{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}],"encrypted_content":"cipher"},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"partial answer"}]}
	]`)
	response.Status = responses.ResponseStatus_incomplete
	response.IncompleteDetails = &responses.IncompleteDetails{Reason: "max_output_tokens"}
	msg, err := (&ResponsesAPIChatModel{}).toOutputMessage(response, nil)
	require.NoError(t, err)
	require.Equal(t, "max_output_tokens", msg.ResponseMeta.FinishReason)
	require.Empty(t, msg.Content)
	require.Empty(t, msg.ReasoningContent)
	require.Empty(t, msg.AssistantGenMultiContent)
}

func TestEncryptedResponseRoundTrip(t *testing.T) {
	r1 := `{"type":"reasoning","id":"r1","summary":[{"type":"summary_text","text":"first"}],"encrypted_content":"cipher1"}`
	r2 := `{"type":"reasoning","id":"r2","summary":[],"encrypted_content":"cipher2"}`
	m1 := `{"type":"message","id":"m1","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	m2 := `{"type":"message","id":"m2","role":"assistant","content":[{"type":"output_text","text":"world"}]}`
	for _, tc := range []struct {
		name, output, text string
		parts              int
		ciphers            []string
	}{
		{"single", "[" + r1 + "," + m1 + "]", "hello", 2, []string{"cipher1"}},
		{"reasoning only", "[" + r2 + "]", "", 1, []string{"cipher2"}},
		{"multiple reasoning", "[" + r1 + "," + r2 + "," + m1 + "]", "hello", 3, []string{"cipher1", "cipher2"}},
		{"multiple text", "[" + r1 + "," + m1 + "," + m2 + "]", "helloworld", 3, []string{"cipher1"}},
		{"both", "[" + r1 + "," + m1 + "," + r2 + "," + m2 + "]", "helloworld", 4, []string{"cipher1", "cipher2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm := &ResponsesAPIChatModel{}
			msg, err := cm.toOutputMessage(encryptedFixture(t, tc.output), nil)
			require.NoError(t, err)
			require.Equal(t, tc.text, msg.Content)
			require.Len(t, msg.AssistantGenMultiContent, tc.parts)
			for _, part := range msg.AssistantGenMultiContent {
				require.Nil(t, part.StreamingMeta)
			}
			b, err := json.Marshal(msg)
			require.NoError(t, err)
			var restored schema.Message
			require.NoError(t, json.Unmarshal(b, &restored))
			req := &responses.ResponsesRequest{}
			require.NoError(t, cm.populateInput([]*schema.Message{&restored}, req, true))
			var ciphers []string
			var text strings.Builder
			for _, item := range req.GetInput().GetListValue().GetListValue() {
				if r := item.GetReasoning(); r != nil {
					require.Nil(t, r.Id)
					ciphers = append(ciphers, r.GetEncryptedContent())
				}
				if m := item.GetInputMessage(); m != nil {
					for _, c := range m.Content {
						text.WriteString(c.GetText().GetText())
					}
				}
			}
			require.Equal(t, tc.ciphers, ciphers)
			require.Equal(t, tc.text, text.String())
			require.NoError(t, cm.populateInput([]*schema.Message{&restored}, req, false))
			for _, item := range req.GetInput().GetListValue().GetListValue() {
				require.Nil(t, item.GetReasoning())
			}
		})
	}
}

func readEncryptedStream(t *testing.T, events []string) *schema.Message {
	t.Helper()
	msg, err := schema.ConcatMessages(readEncryptedChunks(t, events))
	require.NoError(t, err)
	return msg
}

func readEncryptedChunks(t *testing.T, events []string) []*schema.Message {
	t.Helper()
	var wire strings.Builder
	for _, event := range events {
		fmt.Fprintf(&wire, "data: %s\n\n", event)
	}
	wire.WriteString("data: [DONE]\n\n")
	body := io.NopCloser(strings.NewReader(wire.String()))
	reader := &utils.ResponsesStreamReader{ChatCompletionStreamReader: utils.ChatCompletionStreamReader{Unmarshaler: &utils.JSONUnmarshaler{}}, Decoder: utils.NewEventStreamDecoder(body)}
	sr, sw := schema.Pipe[*model.CallbackOutput](64)
	defer sr.Close()
	cm := &ResponsesAPIChatModel{}
	cm.receivedStreamResponse(reader, &model.Config{}, &cacheConfig{}, sw, nil)
	sw.Close()
	var chunks []*schema.Message
	completed := false
	for {
		out, err := sr.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.False(t, completed, "normal completion must carry all replay metadata; no trailing chunk")
		if out.Message.ResponseMeta != nil && out.Message.ResponseMeta.FinishReason == "completed" {
			completed = true
		}
		chunks = append(chunks, out.Message)
	}
	return chunks
}

func TestEncryptedStreamRoundTrip(t *testing.T) {
	msg := readEncryptedStream(t, []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"reasoning","id":"r1","summary":[]}}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"item_id":"r1","summary_index":0,"delta":"think"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"r1","summary":[{"type":"summary_text","text":"think"}],"encrypted_content":"cipher1"}}`,
		`{"type":"response.output_text.delta","output_index":1,"content_index":0,"item_id":"m1","delta":"answer"}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens_details":{},"output_tokens_details":{}}}}`,
	})
	require.Equal(t, "think", msg.ReasoningContent)
	require.Equal(t, "answer", msg.Content)
	require.Len(t, msg.AssistantGenMultiContent, 2)
	b, err := json.Marshal(msg)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &msg))
	req := &responses.ResponsesRequest{}
	require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 2)
	require.Empty(t, items[0].GetReasoning().GetId())
	require.Equal(t, "cipher1", items[0].GetReasoning().GetEncryptedContent())
}

func TestEncryptedMultipleStreamAndClear(t *testing.T) {
	r1 := `{"type":"reasoning","id":"r1","encrypted_content":"cipher1","summary":[{"type":"summary_text","text":"first"}]}`
	r2 := `{"type":"reasoning","id":"r2","encrypted_content":"cipher2","summary":[{"type":"summary_text","text":"second"}]}`
	m := `{"type":"message","id":"m1","role":"assistant","content":[{"type":"output_text","text":"answer"}]}`
	msg := readEncryptedStream(t, []string{
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"item_id":"r1","summary_index":0,"delta":"first"}`,
		`{"type":"response.output_item.done","output_index":0,"item":` + r1 + `}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":1,"item_id":"r2","summary_index":0,"delta":"second"}`,
		`{"type":"response.output_item.done","output_index":1,"item":` + r2 + `}`,
		`{"type":"response.output_text.delta","output_index":2,"item_id":"m1","content_index":0,"delta":"answer"}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[` + r1 + `,` + r2 + `,` + m + `],"usage":{"input_tokens_details":{},"output_tokens_details":{}}}}`,
	})
	require.Equal(t, "first\n\nsecond", msg.ReasoningContent)
	require.Equal(t, "answer", msg.Content)
	require.Len(t, msg.AssistantGenMultiContent, 3)
	for _, p := range msg.AssistantGenMultiContent[:2] {
		require.Equal(t, schema.ChatMessagePartTypeReasoning, p.Type)
	}
	cm := &ResponsesAPIChatModel{}
	req := &responses.ResponsesRequest{}
	require.NoError(t, cm.populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 3)
	require.Equal(t, "cipher1", items[0].GetReasoning().GetEncryptedContent())
	require.Equal(t, "cipher2", items[1].GetReasoning().GetEncryptedContent())
	require.Equal(t, "answer", items[2].GetInputMessage().Content[0].GetText().Text)
	setResponseCacheExpireAt(msg, arkResponseCacheExpireAt(1<<40))
	msg.ToolCalls = []schema.ToolCall{{ID: "call1", Function: schema.FunctionCall{Name: "tool", Arguments: "{}"}}}
	before, err := json.Marshal(msg)
	require.NoError(t, err)
	require.Panics(t, func() { ClearReasoning([]*schema.Message{msg, nil}) })
	cleaned := ClearReasoning([]*schema.Message{msg})
	require.Empty(t, cleaned[0].ReasoningContent)
	require.Len(t, cleaned[0].AssistantGenMultiContent, 1)
	after, err := json.Marshal(msg)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.Equal(t, "call1", cleaned[0].ToolCalls[0].ID)
	expireAt, ok := GetCacheExpiration(cleaned[0])
	require.True(t, ok)
	require.Equal(t, int64(1<<40), expireAt)
	require.Equal(t, cleaned, ClearReasoning(cleaned))
	require.NoError(t, cm.populateInput(cleaned, req, true))
	require.Len(t, req.GetInput().GetListValue().GetListValue(), 2)
}

func TestEncryptedClearSingleAndPassbackWithoutID(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	msg, err := cm.toOutputMessage(encryptedFixture(t, `[{"type":"reasoning","id":"r1","encrypted_content":"cipher1","summary":[{"type":"summary_text","text":"think"}]}]`), nil)
	require.NoError(t, err)
	require.Len(t, msg.AssistantGenMultiContent, 1)
	require.Equal(t, "cipher1", msg.AssistantGenMultiContent[0].Reasoning.Signature)
	require.Equal(t, "think", msg.ReasoningContent)
	cleaned := ClearReasoning([]*schema.Message{msg})
	_, ok := getMsgExtraValue[string](cleaned[0], keyOfReasoningEncryptedContent)
	require.Empty(t, cleaned[0].AssistantGenMultiContent)
	require.False(t, ok)
	_, ok = GetReasoningContent(cleaned[0])
	require.False(t, ok)
	require.Nil(t, ClearReasoning(nil))
	req := &responses.ResponsesRequest{}
	for _, text := range []string{"think", ""} {
		msg.AssistantGenMultiContent[0].Reasoning.Text = text
		require.NoError(t, cm.populateInput([]*schema.Message{msg}, req, true))
		items := req.GetInput().GetListValue().GetListValue()
		require.Len(t, items, 1)
		require.Empty(t, items[0].GetReasoning().GetId())
		require.Equal(t, "cipher1", items[0].GetReasoning().GetEncryptedContent())
	}
	require.NoError(t, cm.populateInput([]*schema.Message{msg}, req, false))
	req.PreviousResponseId = ptrOf("cached")
	require.NoError(t, cm.populateInput([]*schema.Message{msg}, req, true))
}

func TestEncryptedToolCallsFollowContent(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	msg, err := cm.toOutputMessage(encryptedFixture(t, `[
 {"type":"reasoning","id":"r1","encrypted_content":"cipher1","summary":[{"type":"summary_text","text":"first"}]},
 {"type":"function_call","id":"fc1","call_id":"call1","name":"tool","arguments":"{}"},
 {"type":"reasoning","id":"r2","encrypted_content":"cipher2","summary":[{"type":"summary_text","text":"second"}]}
 ]`), nil)
	require.NoError(t, err)
	b, err := json.Marshal(msg)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &msg))
	req := &responses.ResponsesRequest{}
	require.NoError(t, cm.populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 3)
	require.Empty(t, items[0].GetReasoning().GetId())
	require.Empty(t, items[1].GetReasoning().GetId())
	require.NotNil(t, items[2].GetFunctionToolCall())
	require.Empty(t, items[2].GetFunctionToolCall().GetId())
	require.Equal(t, "call1", items[2].GetFunctionToolCall().CallId)
}

func TestEncryptedStreamRawReasoningContentPassback(t *testing.T) {
	msg := readEncryptedStream(t, []string{
		`{"type":"response.reasoning_text.delta","output_index":0,"content_index":0,"item_id":"r1","delta":"raw thought"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"r1","encrypted_content":"cipher","content":[{"type":"reasoning_text","text":"raw thought"}]}}`,
	})
	req := &responses.ResponsesRequest{}
	require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
	r := req.GetInput().GetListValue().GetListValue()[0].GetReasoning()
	require.Len(t, r.Content, 1)
	require.Equal(t, "raw thought", r.Content[0].GetText().GetText())
	require.Equal(t, "cipher", r.GetEncryptedContent())
	require.Empty(t, r.Summary)
}

func TestEncryptedStreamSequentialTools(t *testing.T) {
	msg := readEncryptedStream(t, []string{
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc1","call_id":"call1","name":"tool1","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc1","delta":"{"}`,
		`{"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc1","delta":"}"}`,
		`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"fc2","call_id":"call2","name":"tool2","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","output_index":1,"item_id":"fc2","delta":"{}"}`,
	})
	require.Len(t, msg.ToolCalls, 2)
	req := &responses.ResponsesRequest{}
	require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 2)
	for i, id := range []string{"call1", "call2"} {
		require.Empty(t, items[i].GetFunctionToolCall().GetId())
		require.Equal(t, id, items[i].GetFunctionToolCall().CallId)
		require.Empty(t, msg.ToolCalls[i].Extra)
		require.Equal(t, "{}", items[i].GetFunctionToolCall().Arguments)
	}
}

func TestEncryptedGeneratePreservesPartOrderWithTools(t *testing.T) {
	cm := &ResponsesAPIChatModel{}
	msg, err := cm.toOutputMessage(encryptedFixture(t, `[
 {"type":"message","id":"m1","role":"assistant","content":[{"type":"output_text","text":"answer"}]},
 {"type":"reasoning","id":"r1","encrypted_content":"cipher1","summary":[{"type":"summary_text","text":"first"}]},
 {"type":"reasoning","id":"r2","encrypted_content":"cipher2","summary":[{"type":"summary_text","text":"second"}]},
 {"type":"function_call","id":"fc1","call_id":"call1","name":"tool","arguments":"{}"}
 ]`), nil)
	require.NoError(t, err)
	req := &responses.ResponsesRequest{}
	require.NoError(t, cm.populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Equal(t, "answer", items[0].GetInputMessage().Content[0].GetText().Text)
	require.Empty(t, items[1].GetReasoning().GetId())
	require.Empty(t, items[2].GetReasoning().GetId())
}

func TestEncryptedLegacyTextPartsKeepOneMessage(t *testing.T) {
	// Previously callers could supply text parts without provider metadata. They
	// must still form one assistant protocol message rather than multiple turns.
	msg := &schema.Message{Role: schema.Assistant, Content: "ignored projection", ReasoningContent: "legacy thinking", AssistantGenMultiContent: []schema.MessageOutputPart{
		{Type: schema.ChatMessagePartTypeText, Text: ""},
		{Type: schema.ChatMessagePartTypeText, Text: "hello"},
		{Type: schema.ChatMessagePartTypeText, Text: "world"},
		{Type: schema.ChatMessagePartTypeText, Text: ""},
	}}
	req := &responses.ResponsesRequest{}
	require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 2)
	require.Equal(t, "legacy thinking", items[0].GetReasoning().Summary[0].Text)
	require.Len(t, items[1].GetInputMessage().Content, 4)
	require.Equal(t, "hello", items[1].GetInputMessage().Content[1].GetText().Text)
	require.Equal(t, "world", items[1].GetInputMessage().Content[2].GetText().Text)
	require.Empty(t, items[1].GetInputMessage().Content[0].GetText().Text)
	require.Empty(t, items[1].GetInputMessage().Content[3].GetText().Text)
}

func TestEncryptedPassbackWithOrWithoutReasoningText(t *testing.T) {
	for _, tc := range []struct{ name, text, rawReasoning, want string }{
		{"ciphertext only", "", "", "ciphertext"},
		{"raw only", "", "raw thought", "raw"},
		{"summary", "summary", "", "encrypted"},
	} {
		for _, multipart := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/parts=%v", tc.name, multipart), func(t *testing.T) {
				msg := &schema.Message{Role: schema.Assistant, Content: "answer", ReasoningContent: tc.text,
					Extra: map[string]any{keyOfReasoningEncryptedContent: "cipher", keyOfRawReasoningContent: tc.rawReasoning}}
				if multipart {
					msg.AssistantGenMultiContent = []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeReasoning,
						Reasoning: &schema.MessageOutputReasoning{Text: tc.text, Signature: "cipher"},
						Extra:     map[string]any{keyOfRawReasoningContent: tc.rawReasoning}}}
				}
				before, err := json.Marshal(msg)
				require.NoError(t, err)
				req := &responses.ResponsesRequest{}
				require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
				items := req.GetInput().GetListValue().GetListValue()
				require.Len(t, items, 2)
				r := items[0].GetReasoning()
				require.NotNil(t, r)
				require.Equal(t, "cipher", r.GetEncryptedContent())
				require.Empty(t, r.GetId())
				switch tc.want {
				case "encrypted":
					require.Equal(t, "summary", r.Summary[0].Text)
				case "raw":
					require.Empty(t, r.Summary)
					require.Equal(t, "raw thought", r.Content[0].GetText().Text)
				case "ciphertext":
					require.Empty(t, r.Summary)
					require.Empty(t, r.Content)
					wire, err := json.Marshal(items[0])
					require.NoError(t, err)
					require.JSONEq(t, `{"type":"reasoning","encrypted_content":"cipher"}`, string(wire))
				}
				after, err := json.Marshal(msg)
				require.NoError(t, err)
				require.Equal(t, string(before), string(after), "passback must retain stored ciphertext")
			})
		}
	}
}

func TestEncryptedStreamDeltasImmediatelyContainParts(t *testing.T) {
	chunks := readEncryptedChunks(t, []string{
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"item_id":"r1","summary_index":0,"delta":""}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"item_id":"r1","summary_index":0,"delta":"think"}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"r1","encrypted_content":"cipher","summary":[{"type":"summary_text","text":"think"}]}}`,
		`{"type":"response.output_text.delta","output_index":1,"content_index":0,"item_id":"m1","delta":"answer"}`,
	})
	require.Len(t, chunks, 4, "no EOF finalization chunk")
	require.Empty(t, chunks[0].ReasoningContent)
	require.Len(t, chunks[0].AssistantGenMultiContent, 1)
	require.Empty(t, chunks[0].AssistantGenMultiContent[0].Reasoning.Text)
	require.Equal(t, "think", chunks[1].ReasoningContent)
	require.Len(t, chunks[1].AssistantGenMultiContent, 1)
	require.Equal(t, "think", chunks[1].AssistantGenMultiContent[0].Reasoning.Text)
	require.Equal(t, "cipher", chunks[2].AssistantGenMultiContent[0].Reasoning.Signature)
	require.Equal(t, "answer", chunks[3].Content)
	require.Equal(t, "answer", chunks[3].AssistantGenMultiContent[0].Text)
	msg, err := schema.ConcatMessages(chunks)
	require.NoError(t, err)
	require.Len(t, msg.AssistantGenMultiContent, 2)
	require.Equal(t, "think", msg.AssistantGenMultiContent[0].Reasoning.Text)
}

func TestEncryptedGenerateCombinesAdjacentTextItems(t *testing.T) {
	msg, err := (&ResponsesAPIChatModel{}).toOutputMessage(encryptedFixture(t, `[
	 {"type":"reasoning","id":"r1","encrypted_content":"cipher1","summary":[{"type":"summary_text","text":"first"}]},
	 {"type":"reasoning","id":"r2","encrypted_content":"cipher2","summary":[{"type":"summary_text","text":"second"}]},
	 {"type":"message","id":"m1","role":"assistant","content":[{"type":"output_text","text":"a"}]},
	 {"type":"message","id":"m2","role":"assistant","content":[{"type":"output_text","text":"b"}]}
	]`), nil)
	require.NoError(t, err)
	require.Equal(t, "ab", msg.Content)
	require.Equal(t, "first\n\nsecond", msg.ReasoningContent)
	require.Len(t, msg.AssistantGenMultiContent, 4)
	require.Equal(t, "first", msg.AssistantGenMultiContent[0].Reasoning.Text)
	require.Equal(t, "second", msg.AssistantGenMultiContent[1].Reasoning.Text)
	encoded, err := json.Marshal(msg)
	require.NoError(t, err)
	var restored schema.Message
	require.NoError(t, json.Unmarshal(encoded, &restored))
	require.Len(t, restored.AssistantGenMultiContent, 4)
	req := &responses.ResponsesRequest{}
	require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{&restored}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 3)
	require.Empty(t, items[0].GetReasoning().GetId())
	require.Equal(t, "cipher1", items[0].GetReasoning().GetEncryptedContent())
	require.Empty(t, items[1].GetReasoning().GetId())
	require.Equal(t, "cipher2", items[1].GetReasoning().GetEncryptedContent())
	require.Empty(t, items[2].GetInputMessage().GetId())
	require.Len(t, items[2].GetInputMessage().Content, 2)
}

func TestEncryptedStreamTextPartsDoNotRepeatDoneText(t *testing.T) {
	item := `{"type":"message","id":"m1","role":"assistant","content":[{"type":"output_text","text":"hello"},{"type":"output_text","text":"world"}]}`
	chunks := readEncryptedChunks(t, []string{
		`{"type":"response.output_text.delta","output_index":0,"content_index":0,"item_id":"m1","delta":""}`,
		`{"type":"response.output_text.delta","output_index":0,"content_index":0,"item_id":"m1","delta":"hel"}`,
		`{"type":"response.output_text.delta","output_index":0,"content_index":0,"item_id":"m1","delta":"lo"}`,
		`{"type":"response.output_text.delta","output_index":0,"content_index":1,"item_id":"m1","delta":"world"}`,
		`{"type":"response.output_text.delta","output_index":0,"content_index":1,"item_id":"m1","delta":""}`,
		`{"type":"response.output_item.done","output_index":0,"item":` + item + `}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[` + item + `],"usage":{"input_tokens_details":{},"output_tokens_details":{}}}}`,
	})
	require.Len(t, chunks, 6)
	require.Empty(t, chunks[0].Content)
	require.Len(t, chunks[0].AssistantGenMultiContent, 1)
	require.Empty(t, chunks[4].Content)
	msg, err := schema.ConcatMessages(chunks)
	require.NoError(t, err)
	require.Equal(t, "helloworld", msg.Content)
	require.Len(t, msg.AssistantGenMultiContent, 2)
	require.Equal(t, "hello", msg.AssistantGenMultiContent[0].Text)
	require.Equal(t, "world", msg.AssistantGenMultiContent[1].Text)
	req := &responses.ResponsesRequest{}
	require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 1)
	require.Empty(t, items[0].GetInputMessage().GetId())
	require.Len(t, items[0].GetInputMessage().Content, 2)
}

func TestEncryptedStreamDoneOnlyEmitsMetadata(t *testing.T) {
	chunks := readEncryptedChunks(t, []string{
		`{"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","id":"r1","encrypted_content":"cipher","summary":[{"type":"summary_text","text":"not a delta"}]}}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[{"type":"reasoning","id":"r1","encrypted_content":"cipher","summary":[{"type":"summary_text","text":"not a delta"}]}],"usage":{"input_tokens_details":{},"output_tokens_details":{}}}}`,
	})
	require.Len(t, chunks, 2)
	require.Empty(t, chunks[0].ReasoningContent)
	require.Empty(t, chunks[0].AssistantGenMultiContent[0].Reasoning.Text)
	require.Equal(t, "cipher", chunks[0].AssistantGenMultiContent[0].Reasoning.Signature)
	require.Empty(t, chunks[1].AssistantGenMultiContent)
	msg, err := schema.ConcatMessages(chunks)
	require.NoError(t, err)
	req := &responses.ResponsesRequest{}
	require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
	items := req.GetInput().GetListValue().GetListValue()
	require.Len(t, items, 1)
	require.Equal(t, "cipher", items[0].GetReasoning().GetEncryptedContent())
}

func TestResponsesSummaryAndRawReasoningContentAreIndependent(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		for _, summary := range []string{"", "summary"} {
			t.Run(fmt.Sprintf("stream=%v/summary=%s", streamed, summary), func(t *testing.T) {
				item := `{"type":"reasoning","id":"r1","content":[{"type":"reasoning_text","text":"raw thought"}],"summary":[]}`
				if summary != "" {
					item = strings.Replace(item, `"summary":[]`, `"summary":[{"type":"summary_text","text":"first"},{"type":"summary_text","text":"second"}]`, 1)
					summary = "first\n\nsecond"
				}
				var msg *schema.Message
				if streamed {
					events := []string{`{"type":"response.reasoning_text.delta","output_index":0,"item_id":"r1","content_index":0,"delta":"raw thought"}`}
					if summary != "" {
						events = append(events, `{"type":"response.reasoning_summary_text.delta","output_index":0,"item_id":"r1","summary_index":0,"delta":"first"}`, `{"type":"response.reasoning_summary_text.delta","output_index":0,"item_id":"r1","summary_index":1,"delta":"second"}`)
					}
					events = append(events, `{"type":"response.output_item.done","output_index":0,"item":`+item+`}`)
					msg = readEncryptedStream(t, events)
				} else {
					var err error
					msg, err = (&ResponsesAPIChatModel{}).toOutputMessage(encryptedFixture(t, "["+item+"]"), nil)
					require.NoError(t, err)
				}
				require.Equal(t, summary, msg.ReasoningContent)
				require.Len(t, msg.AssistantGenMultiContent, 1)
				require.Equal(t, summary, msg.AssistantGenMultiContent[0].Reasoning.Text)
				rawReasoning, ok := GetRawReasoningContent(&msg.AssistantGenMultiContent[0])
				require.True(t, ok)
				require.Equal(t, []string{"raw thought"}, rawReasoning)
				encoded, err := json.Marshal(msg)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(encoded, &msg))
				for _, parts := range []bool{true, false} {
					if !parts {
						msg.AssistantGenMultiContent = nil
						msg.Extra = map[string]any{keyOfRawReasoningContent: "raw thought"} // Legacy history.
					}
					req := &responses.ResponsesRequest{}
					require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{msg}, req, true))
					items := req.GetInput().GetListValue().GetListValue()
					require.Len(t, items, 1)
					r := items[0].GetReasoning()
					require.Equal(t, "raw thought", r.Content[0].GetText().Text)
					if summary == "" {
						require.Empty(t, r.Summary)
					} else {
						require.Equal(t, summary, r.Summary[0].Text)
					}
				}
				cleared := ClearReasoning([]*schema.Message{msg})[0]
				require.NotContains(t, cleared.Extra, keyOfRawReasoningContent)
				require.Empty(t, cleared.AssistantGenMultiContent)
				require.Empty(t, cleared.ReasoningContent)
			})
		}
	}
}

func TestRawReasoningContentArrayRoundTrip(t *testing.T) {
	for _, streamed := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%v", streamed), func(t *testing.T) {
			var msg *schema.Message
			if streamed {
				msg = readEncryptedStream(t, []string{
					`{"type":"response.reasoning_text.delta","output_index":0,"content_index":0,"item_id":"r1","delta":"fir"}`,
					`{"type":"response.reasoning_text.delta","output_index":0,"content_index":0,"item_id":"r1","delta":"st"}`,
					`{"type":"response.reasoning_text.delta","output_index":0,"content_index":1,"item_id":"r1","delta":"second"}`,
					`{"type":"response.reasoning_text.delta","output_index":1,"content_index":0,"item_id":"r2","delta":"third"}`,
				})
			} else {
				var err error
				msg, err = (&ResponsesAPIChatModel{}).toOutputMessage(encryptedFixture(t, `[
					{"type":"reasoning","id":"r1","content":[{"type":"reasoning_text","text":"first"},{"type":"reasoning_text","text":"second"}]},
					{"type":"reasoning","id":"r2","content":[{"type":"reasoning_text","text":"third"}]}
				]`), nil)
				require.NoError(t, err)
			}
			require.Empty(t, msg.ReasoningContent)
			rawReasoning, ok := GetRawReasoningContent(&msg.AssistantGenMultiContent[0])
			require.True(t, ok)
			require.Equal(t, []string{"first", "second"}, rawReasoning)
			require.Len(t, msg.AssistantGenMultiContent, 2)
			rawReasoning, ok = GetRawReasoningContent(&msg.AssistantGenMultiContent[1])
			require.True(t, ok)
			require.Equal(t, []string{"third"}, rawReasoning)
			require.NotContains(t, msg.Extra, keyOfRawReasoningContent)
			encoded, err := json.Marshal(msg)
			require.NoError(t, err)
			var restored schema.Message
			require.NoError(t, json.Unmarshal(encoded, &restored))
			rawReasoning, ok = GetRawReasoningContent(&restored.AssistantGenMultiContent[0])
			require.True(t, ok)
			require.Equal(t, []string{"first", "second"}, rawReasoning)
			rawReasoning, ok = GetRawReasoningContent(&restored.AssistantGenMultiContent[1])
			require.True(t, ok)
			require.Equal(t, []string{"third"}, rawReasoning)
			req := &responses.ResponsesRequest{}
			require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{&restored}, req, true))
			items := req.GetInput().GetListValue().GetListValue()
			require.Len(t, items, 2)
			require.Len(t, items[0].GetReasoning().Content, 2)
			require.Equal(t, "first", items[0].GetReasoning().Content[0].GetText().Text)
			require.Equal(t, "second", items[0].GetReasoning().Content[1].GetText().Text)
			require.Len(t, items[1].GetReasoning().Content, 1)
			require.Equal(t, "third", items[1].GetReasoning().Content[0].GetText().Text)
			restored.AssistantGenMultiContent = nil
			require.NoError(t, (&ResponsesAPIChatModel{}).populateInput([]*schema.Message{&restored}, req, true))
			require.Empty(t, req.GetInput().GetListValue().GetListValue(), "removing reasoning parts must remove raw reasoning from replay")
		})
	}
}

func BenchmarkRawReasoningContentConcat(b *testing.B) {
	for _, count := range []int{512, 4096} {
		b.Run(fmt.Sprintf("deltas=%d", count), func(b *testing.B) {
			const delta = "0123456789abcdef0123456789abcdef"
			messages := make([]*schema.Message, count)
			for i := range messages {
				contentIndex := i / (count / 2)
				rawContent := make(arkRawReasoningContent, contentIndex+1)
				rawContent[contentIndex] = delta
				messages[i] = &schema.Message{
					Role: schema.Assistant,
					AssistantGenMultiContent: []schema.MessageOutputPart{{
						Type:          schema.ChatMessagePartTypeReasoning,
						Reasoning:     &schema.MessageOutputReasoning{},
						Extra:         map[string]any{keyOfRawReasoningContent: rawContent},
						StreamingMeta: &schema.MessageStreamingMeta{Index: 0},
					}},
				}
			}
			b.ReportAllocs()
			b.SetBytes(int64(count * len(delta)))
			b.ResetTimer()
			var result *schema.Message
			for i := 0; i < b.N; i++ {
				var err error
				result, err = schema.ConcatMessages(messages)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			rawContent, ok := GetRawReasoningContent(&result.AssistantGenMultiContent[0])
			if !ok || len(rawContent) != 2 {
				b.Fatalf("expected two raw content entries, got %v", rawContent)
			}
			expected := strings.Repeat(delta, count/2)
			for _, text := range rawContent {
				if text != expected {
					b.Fatal("raw reasoning content changed during aggregation")
				}
			}
		})
	}
}

func TestResponsesStreamTerminalErrors(t *testing.T) {
	for _, event := range []struct {
		name, wire, param string
	}{
		{"response.failed", `{"type":"response.failed","response":{"id":"resp_failed","status":"failed","error":{"code":"server_error","message":"generation failed"},"usage":{"input_tokens":1,"output_tokens":0,"total_tokens":1}}}`, ""},
		{"error", `{"type":"error","code":"server_error","message":"generation failed","param":"model","sequence_number":1}`, "model"},
	} {
		for _, prefix := range []struct {
			name, wire string
		}{
			{"none", ""},
			{"text", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"partial"}`},
			{"tool", `{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":""}}` + "\n\n" + `data: {"type":"response.function_call_arguments.delta","output_index":0,"item_id":"fc_1","delta":"{"}`},
		} {
			t.Run(event.name+"/"+prefix.name, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					if prefix.wire != "" {
						fmt.Fprintf(w, "data: %s\n\n", prefix.wire)
					}
					fmt.Fprintf(w, "data: %s\n\n", event.wire)
					fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"output_index\":1,\"content_index\":0,\"delta\":\"must not be emitted\"}\n\ndata: [DONE]\n\n")
				}))
				defer server.Close()
				cm, err := NewResponsesAPIChatModel(context.Background(), &ResponsesAPIConfig{
					APIKey: "test-key", Model: "test-model", BaseURL: server.URL, HTTPClient: server.Client(), RetryTimes: ptrOf(0),
				})
				require.NoError(t, err)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				input := []*schema.Message{schema.UserMessage("test")}
				stream, err := cm.Stream(ctx, input)
				require.NoError(t, err)
				defer stream.Close()
				if prefix.name != "none" {
					msg, err := stream.Recv()
					require.NoError(t, err)
					if prefix.name == "text" {
						require.Equal(t, "partial", msg.Content)
					} else {
						require.Len(t, msg.ToolCalls, 1)
						require.Equal(t, "call_1", msg.ToolCalls[0].ID)
					}
				}
				msg, err := stream.Recv()
				require.Error(t, err)
				require.Nil(t, msg)
				var apiErr *arkModel.APIError
				require.True(t, errors.As(err, &apiErr))
				require.Equal(t, "server_error", apiErr.Code)
				require.Equal(t, "generation failed", apiErr.Message)
				require.Equal(t, event.name, apiErr.Type)
				if event.param != "" {
					require.Equal(t, event.param, *apiErr.Param)
				}
				_, err = stream.Recv()
				require.ErrorIs(t, err, io.EOF)
				stream, err = cm.Stream(ctx, input)
				require.NoError(t, err)
				msg, err = schema.ConcatMessageStream(stream)
				require.Nil(t, msg)
				require.True(t, errors.As(err, &apiErr))
				require.Equal(t, "server_error", apiErr.Code)
			})
		}
	}
}
