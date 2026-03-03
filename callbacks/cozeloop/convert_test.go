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
	"errors"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/consts"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	. "github.com/smartystreets/goconvey/convey"
)

// 定义一个辅助的 MessagesTemplate 实现
type MockMessagesTemplate struct{}

func (m *MockMessagesTemplate) Format(ctx context.Context, vs map[string]any, formatType schema.FormatType) ([]*schema.Message, error) {
	return nil, nil
}

func Test_convertPromptInput(t *testing.T) {
	mockey.PatchConvey("测试 convertPromptInput 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			// Arrange
			var input *prompt.CallbackInput = nil

			// Act
			result := convertPromptInput(input)

			// Assert
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("输入不为 nil 的情况", func() {
			// Arrange
			variables := map[string]any{"key": "value"}
			templates := []schema.MessagesTemplate{&MockMessagesTemplate{}}
			extra := map[string]any{"extraKey": "extraValue"}
			input := &prompt.CallbackInput{
				Variables: variables,
				Templates: templates,
				Extra:     extra,
			}

			// Act
			result := convertPromptInput(input)

			// Assert
			So(result, ShouldNotBeNil)
		})
	})
}

func Test_convertPromptOutput(t *testing.T) {
	mockey.PatchConvey("测试 convertPromptOutput 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			output := convertPromptOutput(nil)
			So(output, ShouldBeNil)
		})

		mockey.PatchConvey("输入不为 nil 的情况", func() {
			result := []*schema.Message{
				{
					Role:    "user",
					Content: "test content",
				},
			}
			templates := []schema.MessagesTemplate{}
			extra := map[string]any{}
			callbackOutput := &prompt.CallbackOutput{
				Result:    result,
				Templates: templates,
				Extra:     extra,
			}

			output := convertPromptOutput(callbackOutput)
			So(output, ShouldNotBeNil)
			So(output.Prompts, ShouldNotBeEmpty)
		})
	})
}

func Test_convertTemplate(t *testing.T) {
	mockey.PatchConvey("测试 convertTemplate 函数", t, func() {
		mockey.PatchConvey("输入 template 为 nil", func() {
			// Arrange
			var template schema.MessagesTemplate = nil

			// Act
			result := convertTemplate(template)

			// Assert
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("输入 template 为 *schema.Message 类型", func() {
			// Arrange
			message := &schema.Message{
				Role:    "test_role",
				Content: "test_content",
			}
			expectedResult := &tracespec.ModelMessage{
				Role:    "test_role",
				Content: "test_content",
			}
			mockConvertModelMessage := mockey.Mock(convertModelMessage).Return(expectedResult).Build()
			defer mockConvertModelMessage.UnPatch()

			// Act
			result := convertTemplate(message)

			// Assert
			So(result, ShouldResemble, expectedResult)
		})

		mockey.PatchConvey("输入 template 为其他类型", func() {
			template := OtherTemplate{}
			// Act
			result := convertTemplate(template)
			// Assert
			So(result, ShouldBeNil)
		})
	})
}

type OtherTemplate struct{}

func (ot OtherTemplate) Format(ctx context.Context, vs map[string]any, formatType schema.FormatType) ([]*schema.Message, error) {
	return nil, nil
}

func Test_convertPromptArguments(t *testing.T) {
	mockey.PatchConvey("测试 convertPromptArguments 函数", t, func() {
		mockey.PatchConvey("传入 nil 的 variables", func() {
			var variables map[string]any = nil
			result := convertPromptArguments(variables)
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("传入非 nil 的 variables", func() {
			variables := map[string]any{
				"key1": "value1",
				"key2": 123,
			}
			result := convertPromptArguments(variables)
			So(result, ShouldNotBeNil)
			So(len(result), ShouldEqual, len(variables))
			for _, arg := range result {
				value, exists := variables[arg.Key]
				So(exists, ShouldBeTrue)
				So(arg.Value, ShouldEqual, value)
			}
		})
	})
}

func Test_convertRetrieverOutput(t *testing.T) {
	mockey.PatchConvey("测试 convertRetrieverOutput 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			output := convertRetrieverOutput(nil)
			So(output, ShouldBeNil)
		})

		mockey.PatchConvey("输入不为 nil 的情况", func() {
			docs := []*schema.Document{
				{
					ID:      "1",
					Content: "test content",
					MetaData: map[string]any{
						"key": "value",
					},
				},
			}
			callbackOutput := &retriever.CallbackOutput{
				Docs:  docs,
				Extra: map[string]any{},
			}

			output := convertRetrieverOutput(callbackOutput)
			So(output, ShouldNotBeNil)
			So(len(output.Documents), ShouldEqual, 1)

		})
	})
}

func Test_convertRetrieverCallOption(t *testing.T) {
	mockey.PatchConvey("测试 convertRetrieverCallOption 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			// Arrange
			var input *retriever.CallbackInput = nil
			// Act
			result := convertRetrieverCallOption(input)
			// Assert
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("输入不为 nil，ScoreThreshold 为 nil 的情况", func() {
			// Arrange
			input := &retriever.CallbackInput{
				Query:          "test query",
				TopK:           10,
				Filter:         "test filter",
				ScoreThreshold: nil,
				Extra:          map[string]any{"key": "value"},
			}
			expected := &tracespec.RetrieverCallOption{
				TopK:     int64(input.TopK),
				Filter:   input.Filter,
				MinScore: nil,
			}
			// Act
			result := convertRetrieverCallOption(input)
			// Assert
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("输入不为 nil，ScoreThreshold 不为 nil 的情况", func() {
			// Arrange
			score := 0.5
			input := &retriever.CallbackInput{
				Query:          "test query",
				TopK:           10,
				Filter:         "test filter",
				ScoreThreshold: &score,
				Extra:          map[string]any{"key": "value"},
			}
			expected := &tracespec.RetrieverCallOption{
				TopK:     int64(input.TopK),
				Filter:   input.Filter,
				MinScore: &score,
			}
			// Act
			result := convertRetrieverCallOption(input)
			// Assert
			So(result, ShouldResemble, expected)
		})
	})
}

func Test_convertDocument(t *testing.T) {
	mockey.PatchConvey("测试 convertDocument 函数", t, func() {
		mockey.PatchConvey("输入的 doc 为 nil", func() {
			result := convertDocument(nil)
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("输入的 doc 不为 nil", func() {
			testDoc := &schema.Document{
				ID:      "testID",
				Content: "testContent",
				MetaData: map[string]any{
					"key": "value",
				},
			}
			testScore := 0.8
			testVector := []float64{1.0, 2.0, 3.0}
			mockScore := mockey.Mock((*schema.Document).Score).Return(testScore).Build()
			mockVector := mockey.Mock((*schema.Document).DenseVector).Return(testVector).Build()
			defer mockScore.UnPatch()
			defer mockVector.UnPatch()

			result := convertDocument(testDoc)
			So(result, ShouldNotBeNil)
			So(result.ID, ShouldEqual, testDoc.ID)
			So(result.Content, ShouldEqual, testDoc.Content)
		})
	})
}

func Test_addToolName(t *testing.T) {
	mockey.PatchConvey("测试 addToolName 函数", t, func() {
		mockey.PatchConvey("输入的 message 为 nil", func() {
			result := addToolName(context.Background(), nil)
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("输入的 message 不为 nil, ctx中没有tool信息", func() {
			result := addToolName(context.Background(), &tracespec.ModelMessage{})
			So(result.Name, ShouldEqual, "")
		})

		mockey.PatchConvey("输入的 message 不为 nil, ctx中有tool信息", func() {
			ctx := context.Background()
			ctx = context.WithValue(ctx, consts.CozeLoopToolIDNameMap, map[string]string{"1234567890": "testTool"})
			result := addToolName(ctx, &tracespec.ModelMessage{
				ToolCallID: "1234567890",
			})
			So(result.Name, ShouldEqual, "testTool")
		})
	})
}

func Test_iterSliceWithCtx(t *testing.T) {
	mockey.PatchConvey("测试 iterSliceWithCtx 函数", t, func() {
		mockey.PatchConvey("输入的 message 不为 nil, ctx中没有tool信息", func() {
			result := iterSliceWithCtx(context.Background(), []*tracespec.ModelMessage{
				{
					ToolCallID: "1234567890",
				},
			}, addToolName)
			So(len(result), ShouldEqual, 1)
		})
	})
}

func Test_convertModelOutput(t *testing.T) {
	mockey.PatchConvey("Test_convertModelOutput", t, func() {
		mockey.PatchConvey("场景一：当输入为nil时，应返回nil", func() {
			// Arrange: 准备一个nil输入
			var output *model.CallbackOutput = nil

			// Act: 调用被测函数
			result := convertModelOutput(output)

			// Assert: 验证结果是否为nil
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("场景二：当输入为基本的CallbackOutput时，应正确转换", func() {
			// Arrange: 准备一个基本的输入数据
			output := &model.CallbackOutput{
				Message: &schema.Message{
					Role:    "assistant",
					Content: "Hello there!",
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: "stop",
					},
				},
			}

			// 准备期望的输出结果
			expected := &tracespec.ModelOutput{
				Choices: []*tracespec.ModelChoice{
					{
						Index:        0,
						FinishReason: "stop",
						Message: &tracespec.ModelMessage{
							Role:      "assistant",
							Content:   "Hello there!",
							Parts:     make([]*tracespec.ModelMessagePart, 0),
							ToolCalls: make([]*tracespec.ModelToolCall, 0),
						},
					},
				},
			}

			// Act: 调用被测函数
			result := convertModelOutput(output)

			// Assert: 验证转换结果是否与预期相符
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("场景三：当输入包含ToolCalls和MultiContent时，应正确转换", func() {
			// Arrange: 准备一个包含复杂字段的输入数据
			output := &model.CallbackOutput{
				Message: &schema.Message{
					Role: "assistant",
					MultiContent: []schema.ChatMessagePart{
						{
							Type: "text",
							Text: "Here is an image.",
						},
						{
							Type: "image_url",
							ImageURL: &schema.ChatMessageImageURL{
								URL:    "https://example.com/image.jpg",
								Detail: "high",
							},
						},
					},
					ToolCalls: []schema.ToolCall{
						{
							ID:   "call_abc_123",
							Type: "function",
							Function: schema.FunctionCall{
								Name:      "get_current_weather",
								Arguments: `{"location": "San Francisco"}`,
							},
						},
					},
					ResponseMeta: &schema.ResponseMeta{
						FinishReason: "tool_calls",
					},
				},
			}

			// 准备期望的输出结果
			expected := &tracespec.ModelOutput{
				Choices: []*tracespec.ModelChoice{
					{
						Index:        0,
						FinishReason: "tool_calls",
						Message: &tracespec.ModelMessage{
							Role: "assistant",
							Parts: []*tracespec.ModelMessagePart{
								{
									Type: "text",
									Text: "Here is an image.",
								},
								{
									Type: "image_url",
									ImageURL: &tracespec.ModelImageURL{
										URL:    "https://example.com/image.jpg",
										Detail: "high",
									},
								},
							},
							ToolCalls: []*tracespec.ModelToolCall{
								{
									ID:   "call_abc_123",
									Type: toolTypeFunction, // 使用常量 "function"
									Function: &tracespec.ModelToolCallFunction{
										Name:      "get_current_weather",
										Arguments: `{"location": "San Francisco"}`,
									},
								},
							},
						},
					},
				},
			}

			// Act: 调用被测函数
			result := convertModelOutput(output)

			// Assert: 验证复杂结构的转换结果是否与预期相符
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("场景四：当输入的CallbackOutput中Message字段为nil时，应能正确处理", func() {
			// Arrange: 准备一个Message字段为nil的输入
			output := &model.CallbackOutput{
				Message: nil,
			}

			// 准备期望的输出结果
			expected := &tracespec.ModelOutput{
				Choices: []*tracespec.ModelChoice{
					{
						Index:        0,
						FinishReason: "",  // getFinishReason(nil) 返回空字符串
						Message:      nil, // convertModelMessage(nil) 返回nil
					},
				},
			}

			// Act: 调用被测函数
			result := convertModelOutput(output)

			// Assert: 验证对nil Message的处理是否符合预期
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("场景五：当输入的Message中ResponseMeta为nil时，FinishReason应为空", func() {
			// Arrange: 准备一个ResponseMeta为nil的输入
			output := &model.CallbackOutput{
				Message: &schema.Message{
					Role:         "user",
					Content:      "No finish reason.",
					ResponseMeta: nil, // ResponseMeta为nil
				},
			}

			// 准备期望的输出结果
			expected := &tracespec.ModelOutput{
				Choices: []*tracespec.ModelChoice{
					{
						Index:        0,
						FinishReason: "", // getFinishReason 在 ResponseMeta 为 nil 时返回空字符串
						Message: &tracespec.ModelMessage{
							Role:      "user",
							Content:   "No finish reason.",
							Parts:     make([]*tracespec.ModelMessagePart, 0),
							ToolCalls: make([]*tracespec.ModelToolCall, 0),
						},
					},
				},
			}

			// Act: 调用被测函数
			result := convertModelOutput(output)

			// Assert: 验证对nil ResponseMeta的处理是否符合预期
			So(result, ShouldResemble, expected)
		})
	})
}

func Test_convertModelMessage(t *testing.T) {
	mockey.PatchConvey("Test convertModelMessage", t, func() {
		mockey.PatchConvey("场景一：当输入message为nil时，应返回nil", func() {
			// Act: 调用被测函数，输入为nil
			result := convertModelMessage(nil)

			// Assert: 断言结果为nil
			So(result, ShouldBeNil)
		})

		mockey.PatchConvey("场景二：当输入一个只包含基本字段的message时，应正确转换", func() {
			// Arrange: 准备一个只包含基本字段的输入消息
			input := &schema.Message{
				Role:             "user",
				Content:          "Hello, world!",
				Name:             "test_user",
				ToolCallID:       "call_123",
				ReasoningContent: "User is greeting.",
			}

			// 准备期望的输出结果
			expected := &tracespec.ModelMessage{
				Role:             "user",
				Content:          "Hello, world!",
				Parts:            []*tracespec.ModelMessagePart{},
				Name:             "test_user",
				ToolCalls:        []*tracespec.ModelToolCall{},
				ToolCallID:       "call_123",
				ReasoningContent: "User is greeting.",
			}

			// Act: 调用被测函数
			result := convertModelMessage(input)

			// Assert: 断言转换结果与期望值一致
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("场景三：当message包含MultiContent时，应正确转换", func() {
			// Arrange: 准备一个包含MultiContent的输入消息
			input := &schema.Message{
				Role:    "user",
				Content: "A message with an image",
				MultiContent: []schema.ChatMessagePart{
					{
						Type: "text",
						Text: "Here is an image.",
					},
					{
						Type: "image_url",
						ImageURL: &schema.ChatMessageImageURL{
							URL:    "https://example.com/image.jpg",
							Detail: "high",
						},
					},
				},
			}

			// 准备期望的输出结果
			expected := &tracespec.ModelMessage{
				Role:    "user",
				Content: "A message with an image",
				Parts: []*tracespec.ModelMessagePart{
					{
						Type: "text",
						Text: "Here is an image.",
					},
					{
						Type: "image_url",
						ImageURL: &tracespec.ModelImageURL{
							URL:    "https://example.com/image.jpg",
							Detail: "high",
						},
					},
				},
				ToolCalls: []*tracespec.ModelToolCall{},
			}

			// Act: 调用被测函数
			result := convertModelMessage(input)

			// Assert: 断言转换结果与期望值一致
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("场景四：当message包含ToolCalls时，应正确转换", func() {
			// Arrange: 准备一个包含ToolCalls的输入消息
			input := &schema.Message{
				Role: "assistant",
				ToolCalls: []schema.ToolCall{
					{
						ID:   "tool_call_abc",
						Type: "some_other_type", // 注意：这个类型会被忽略
						Function: schema.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location": "Shanghai"}`,
						},
					},
				},
			}

			// 准备期望的输出结果
			expected := &tracespec.ModelMessage{
				Role:  "assistant",
				Parts: []*tracespec.ModelMessagePart{},
				ToolCalls: []*tracespec.ModelToolCall{
					{
						ID:   "tool_call_abc",
						Type: toolTypeFunction, // 类型应被硬编码为 "function"
						Function: &tracespec.ModelToolCallFunction{
							Name:      "get_weather",
							Arguments: `{"location": "Shanghai"}`,
						},
					},
				},
			}

			// Act: 调用被测函数
			result := convertModelMessage(input)

			// Assert: 断言转换结果与期望值一致
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("场景五：当message包含Extra且序列化成功时，应正确转换", func() {
			// Arrange: 准备一个包含Extra的输入消息
			input := &schema.Message{
				Role: "user",
				Extra: map[string]any{
					"request_id": "req-12345",
					"user_level": 5,
				},
			}
			// Mock sonic.MarshalString 函数，使其对不同类型的值返回不同的序列化结果
			mockey.Mock(sonic.MarshalString).To(func(v any) (string, error) {
				if s, ok := v.(string); ok && s == "req-12345" {
					return `"req-12345"`, nil
				}
				if i, ok := v.(int); ok && i == 5 {
					return `5`, nil
				}
				return "", errors.New("unmocked value")
			}).Build()

			// 准备期望的输出结果
			expected := &tracespec.ModelMessage{
				Role:      "user",
				Parts:     []*tracespec.ModelMessagePart{},
				ToolCalls: []*tracespec.ModelToolCall{},
				Metadata: map[string]string{
					"request_id": `"req-12345"`,
					"user_level": `5`,
				},
			}

			// Act: 调用被测函数
			result := convertModelMessage(input)

			// Assert: 断言转换结果与期望值一致
			So(result, ShouldResemble, expected)
		})

		mockey.PatchConvey("场景六：当message包含Extra但序列化失败时，应忽略失败的字段", func() {
			// Arrange: 准备一个包含Extra的输入消息
			input := &schema.Message{
				Role: "user",
				Extra: map[string]any{
					"some_data": "this will fail",
				},
			}
			// Mock sonic.MarshalString 函数，使其返回错误
			mockErr := errors.New("marshal error")
			mockey.Mock(sonic.MarshalString).Return("", mockErr).Build()

			// 准备期望的输出结果，Metadata应为空
			expected := &tracespec.ModelMessage{
				Role:      "user",
				Parts:     []*tracespec.ModelMessagePart{},
				ToolCalls: []*tracespec.ModelToolCall{},
				Metadata:  map[string]string{},
			}

			// Act: 调用被测函数
			result := convertModelMessage(input)

			// Assert: 断言转换结果与期望值一致
			So(result, ShouldResemble, expected)
		})
	})
}
