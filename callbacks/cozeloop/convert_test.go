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

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/consts"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/smartystreets/goconvey/convey"
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
			convey.So(result, convey.ShouldBeNil)
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
			convey.So(result, convey.ShouldNotBeNil)
		})
	})
}

func Test_convertPromptOutput(t *testing.T) {
	mockey.PatchConvey("测试 convertPromptOutput 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			output := convertPromptOutput(nil)
			convey.So(output, convey.ShouldBeNil)
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
			convey.So(output, convey.ShouldNotBeNil)
			convey.So(output.Prompts, convey.ShouldNotBeEmpty)
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
			convey.So(result, convey.ShouldBeNil)
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
			convey.So(result, convey.ShouldResemble, expectedResult)
		})

		mockey.PatchConvey("输入 template 为其他类型", func() {
			template := OtherTemplate{}
			// Act
			result := convertTemplate(template)
			// Assert
			convey.So(result, convey.ShouldBeNil)
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
			convey.So(result, convey.ShouldBeNil)
		})

		mockey.PatchConvey("传入非 nil 的 variables", func() {
			variables := map[string]any{
				"key1": "value1",
				"key2": 123,
			}
			result := convertPromptArguments(variables)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result), convey.ShouldEqual, len(variables))
			for _, arg := range result {
				value, exists := variables[arg.Key]
				convey.So(exists, convey.ShouldBeTrue)
				convey.So(arg.Value, convey.ShouldEqual, value)
			}
		})
	})
}

func Test_convertRetrieverOutput(t *testing.T) {
	mockey.PatchConvey("测试 convertRetrieverOutput 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			output := convertRetrieverOutput(nil)
			convey.So(output, convey.ShouldBeNil)
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
			convey.So(output, convey.ShouldNotBeNil)
			convey.So(len(output.Documents), convey.ShouldEqual, 1)

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
			convey.So(result, convey.ShouldBeNil)
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
			convey.So(result, convey.ShouldResemble, expected)
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
			convey.So(result, convey.ShouldResemble, expected)
		})
	})
}

func Test_convertDocument(t *testing.T) {
	mockey.PatchConvey("测试 convertDocument 函数", t, func() {
		mockey.PatchConvey("输入的 doc 为 nil", func() {
			result := convertDocument(nil)
			convey.So(result, convey.ShouldBeNil)
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
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result.ID, convey.ShouldEqual, testDoc.ID)
			convey.So(result.Content, convey.ShouldEqual, testDoc.Content)
		})
	})
}

func Test_addToolName(t *testing.T) {
	mockey.PatchConvey("测试 addToolName 函数", t, func() {
		mockey.PatchConvey("输入的 message 为 nil", func() {
			result := addToolName(context.Background(), nil)
			convey.So(result, convey.ShouldBeNil)
		})

		mockey.PatchConvey("输入的 message 不为 nil, ctx中没有tool信息", func() {
			result := addToolName(context.Background(), &tracespec.ModelMessage{})
			convey.So(result.Name, convey.ShouldEqual, "")
		})

		mockey.PatchConvey("输入的 message 不为 nil, ctx中有tool信息", func() {
			ctx := context.Background()
			ctx = context.WithValue(ctx, consts.CozeLoopToolIDNameMap, map[string]string{"1234567890": "testTool"})
			result := addToolName(ctx, &tracespec.ModelMessage{
				ToolCallID: "1234567890",
			})
			convey.So(result.Name, convey.ShouldEqual, "testTool")
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
			convey.So(len(result), convey.ShouldEqual, 1)
		})
	})
}
