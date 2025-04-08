package cozeloop

import (
	"context"
	"testing"

	"github.com/bytedance/mockey"
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



// Test_convertPromptOutput 测试 convertPromptOutput 函数
func Test_convertPromptOutput(t *testing.T) {
	mockey.PatchConvey("测试 convertPromptOutput 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			// 调用待测函数
			output := convertPromptOutput(nil)
			// 断言返回值为 nil
			convey.So(output, convey.ShouldBeNil)
		})

		mockey.PatchConvey("输入不为 nil 的情况", func() {
			// 准备测试数据
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

			// Mock iterSlice 函数
			mockIterSlice := mockey.MockGeneric(iterSlice[*schema.Message, *tracespec.ModelMessage]).Return([]*tracespec.ModelMessage{
				{
					Role:    "user",
					Content: "test content",
				},
			}).Build()
			defer mockIterSlice.UnPatch()

			// 调用待测函数
			output := convertPromptOutput(callbackOutput)
			// 断言返回值不为 nil
			convey.So(output, convey.ShouldNotBeNil)
			// 断言返回值的 Prompts 切片不为空
			convey.So(output.Prompts, convey.ShouldNotBeEmpty)
		})
	})
}



// Test_convertTemplate 测试 convertTemplate 函数
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
			// 这里 mock convertModelMessage 函数，确保其返回预期结果
			mockConvertModelMessage := mockey.Mock(convertModelMessage).Return(expectedResult).Build()
			defer mockConvertModelMessage.UnPatch()

			// Act
			result := convertTemplate(message)

			// Assert
			So(result, ShouldResemble, expectedResult)
		})

		mockey.PatchConvey("输入 template 为其他类型", func() {
			// Arrange
			// 定义一个实现了 MessagesTemplate 接口的其他类型
			type OtherTemplate struct{}
			func (ot OtherTemplate) Format(ctx context.Context, vs map[string]any, formatType schema.FormatType) ([]*schema.Message, error) {
				return nil, nil
			}
			template := OtherTemplate{}

			// Act
			result := convertTemplate(template)

			// Assert
			So(result, ShouldBeNil)
		})
	})
}


// Test_convertPromptArguments 为 convertPromptArguments 函数编写的测试函数
func Test_convertPromptArguments(t *testing.T) {
	mockey.PatchConvey("测试 convertPromptArguments 函数", t, func() {
		mockey.PatchConvey("传入 nil 的 variables", func() {
			// Arrange: 准备传入的参数为 nil
			var variables map[string]any = nil
			// Act: 调用待测函数
			result := convertPromptArguments(variables)
			// Assert: 断言结果为 nil
			convey.So(result, convey.ShouldBeNil)
		})

		mockey.PatchConvey("传入非 nil 的 variables", func() {
			// Arrange: 准备传入的参数
			variables := map[string]any{
				"key1": "value1",
				"key2": 123,
			}
			// Act: 调用待测函数
			result := convertPromptArguments(variables)
			// Assert: 断言结果不为 nil
			convey.So(result, convey.ShouldNotBeNil)
			// 断言结果切片的长度与传入的 map 的长度相同
			convey.So(len(result), convey.ShouldEqual, len(variables))
			// 遍历结果切片，检查每个元素的 Key 和 Value 是否正确
			for _, arg := range result {
				value, exists := variables[arg.Key]
				convey.So(exists, convey.ShouldBeTrue)
				convey.So(arg.Value, convey.ShouldEqual, value)
			}
		})
	})
}


// Test_convertRetrieverOutput 测试 convertRetrieverOutput 函数
func Test_convertRetrieverOutput(t *testing.T) {
	mockey.PatchConvey("测试 convertRetrieverOutput 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			// 调用待测函数
			output := convertRetrieverOutput(nil)
			// 断言返回值为 nil
			convey.So(output, convey.ShouldBeNil)
		})

		mockey.PatchConvey("输入不为 nil 的情况", func() {
			// 模拟 schema.Document 切片
			docs := []*schema.Document{
				{
					ID:      "1",
					Content: "test content",
					MetaData: map[string]any{
						"key": "value",
					},
				},
			}
			// 模拟 retriever.CallbackOutput
			callbackOutput := &retriever.CallbackOutput{
				Docs:  docs,
				Extra: map[string]any{},
			}
			// 模拟 convertDocument 函数
			mockConvertDocument := mockey.Mock(convertDocument).Return(&tracespec.RetrieverDocument{
				ID:      "1",
				Content: "test content",
			}).Build()
			// 模拟 iterSlice 函数
			mockIterSlice := mockey.MockGeneric(iterSlice[*schema.Document, *tracespec.RetrieverDocument]).Return([]*tracespec.RetrieverDocument{
				{
					ID:      "1",
					Content: "test content",
				},
			}).Build()
			// 调用待测函数
			output := convertRetrieverOutput(callbackOutput)
			// 断言返回值不为 nil
			convey.So(output, convey.ShouldNotBeNil)
			// 断言返回值的 Documents 切片长度为 1
			convey.So(len(output.Documents), convey.ShouldEqual, 1)
			// 断言模拟函数被调用
			convey.So(mockConvertDocument.Times(), convey.ShouldEqual, 1)
			convey.So(mockIterSlice.Times(), convey.ShouldEqual, 1)
		})
	})
}

// Test_convertRetrieverCallOption 测试 convertRetrieverCallOption 函数
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
				TopK:   int64(input.TopK),
				Filter: input.Filter,
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
				TopK:   int64(input.TopK),
				Filter: input.Filter,
				MinScore: &score,
			}
			// Act
			result := convertRetrieverCallOption(input)
			// Assert
			convey.So(result, convey.ShouldResemble, expected)
		})
	})
}

// Test_convertDocument 测试 convertDocument 函数
func Test_convertDocument(t *testing.T) {
	mockey.PatchConvey("测试 convertDocument 函数", t, func() {
		mockey.PatchConvey("输入的 doc 为 nil", func() {
			// 调用 convertDocument 函数，传入 nil
			result := convertDocument(nil)
			// 断言结果为 nil
			convey.So(result, convey.ShouldBeNil)
		})

		mockey.PatchConvey("输入的 doc 不为 nil", func() {
			// 创建一个测试用的 Document 实例
			testDoc := &schema.Document{
				ID:      "testID",
				Content: "testContent",
				MetaData: map[string]any{
					"key": "value",
				},
			}
			// 定义测试用的 Score 和 DenseVector 返回值
			testScore := 0.8
			testVector := []float64{1.0, 2.0, 3.0}
			// mock doc.Score() 方法，返回测试用的 Score
			mockScore := mockey.Mock((*schema.Document).Score).Return(testScore).Build()
			// mock doc.DenseVector() 方法，返回测试用的 DenseVector
			mockVector := mockey.Mock((*schema.Document).DenseVector).Return(testVector).Build()
			defer mockScore.UnPatch()
			defer mockVector.UnPatch()

			// 调用 convertDocument 函数，传入测试用的 Document 实例
			result := convertDocument(testDoc)
			// 断言结果不为 nil
			convey.So(result, convey.ShouldNotBeNil)
			// 断言结果的 ID 与测试用的 Document 实例的 ID 相同
			convey.So(result.ID, convey.ShouldEqual, testDoc.ID)
			// 断言结果的 Content 与测试用的 Document 实例的 Content 相同
			convey.So(result.Content, convey.ShouldEqual, testDoc.Content)
			// 断言结果的 Score 与 mock 的 Score 相同
			convey.So(result.Score, convey.ShouldEqual, testScore)
			// 断言结果的 Vector 与 mock 的 DenseVector 相同
			convey.So(result.Vector, convey.ShouldResemble, testVector)
		})
	})
}

