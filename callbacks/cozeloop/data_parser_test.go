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
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino-ext/callbacks/cozeloop/internal/async"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/smartystreets/goconvey/convey"
)

func Test_defaultDataParser_ParseInput(t *testing.T) {
	mockey.PatchConvey("测试 defaultDataParser 的 ParseInput 方法", t, func() {
		mockey.PatchConvey("测试 ComponentOfChatModel 场景", func() {
			ctx := context.Background()
			info := &callbacks.RunInfo{
				Name:      "test",
				Type:      "testType",
				Component: components.ComponentOfChatModel,
			}
			var input callbacks.CallbackInput = []*schema.Message{
				{Role: schema.System, Content: "system message"},
				{Role: schema.User, Content: "user message"},
			}
			d := defaultDataParser{}

			result := d.ParseInput(ctx, info, input)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result[tracespec.ModelProvider], convey.ShouldEqual, "testType")
		})

		mockey.PatchConvey("测试 ComponentOfPrompt 场景", func() {
			ctx := context.Background()
			info := &callbacks.RunInfo{
				Name:      "test",
				Type:      "testType",
				Component: components.ComponentOfPrompt,
			}
			var input callbacks.CallbackInput = []*schema.Message{
				{Role: schema.System, Content: "system message"},
				{Role: schema.User, Content: "user message"},
			}
			d := defaultDataParser{}

			result := d.ParseInput(ctx, info, input)
			convey.So(result, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("测试 info 为 nil 的场景", func() {
			ctx := context.Background()
			var info *callbacks.RunInfo = nil
			var input callbacks.CallbackInput = []*schema.Message{
				{Role: schema.System, Content: "system message"},
				{Role: schema.User, Content: "user message"},
			}
			d := defaultDataParser{}

			result := d.ParseInput(ctx, info, input)
			convey.So(result, convey.ShouldBeNil)
		})

		mockey.PatchConvey("测试 input 为 nil 的场景，且为1级节点", func() {
			ctx := context.Background()
			info := &callbacks.RunInfo{
				Component: compose.ComponentOfGraph,
			}
			var input callbacks.CallbackInput = []*schema.Message{
				{Role: schema.System, Content: "system message"},
				{Role: schema.User, Content: "user message"},
			}
			ctx = injectGraphNodeLevelToCtx(ctx, 1)
			ctx = injectAggrMessageOutputHookToCtx(ctx)
			d := defaultDataParser{}
			result := d.ParseInput(ctx, info, input)
			convey.So(result, convey.ShouldNotBeNil)
			collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)
			convey.So(len(collectOutput.Messages), convey.ShouldEqual, 2)
		})
	})
}

// Test_defaultDataParser_ParseOutput 测试 defaultDataParser 的 ParseOutput 方法
func Test_defaultDataParser_ParseOutput(t *testing.T) {
	mockey.PatchConvey("测试 defaultDataParser 的 ParseOutput 方法", t, func() {
		// 初始化 defaultDataParser 实例
		d := defaultDataParser{
			concatFuncs: make(map[reflect.Type]any),
		}
		ctx := context.Background()
		var output callbacks.CallbackOutput = &model.CallbackOutput{
			Message: &schema.Message{
				Role:    schema.Assistant,
				Content: "Hello, how can I assist you today?",
			},
		}
		var outputs callbacks.CallbackOutput = []*schema.Message{
			{
				Role:    schema.Assistant,
				Content: "Hello, how can I assist you today?",
			},
		}

		mockey.PatchConvey("当 info 为 nil 时", func() {
			// 调用 ParseOutput 方法
			result := d.ParseOutput(ctx, nil, output)
			// 断言结果为 nil
			convey.So(result, convey.ShouldBeNil)
		})

		mockey.PatchConvey("当 info.Component 为 ComponentOfChatModel 时", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfChatModel,
			}
			mockConvertModelOutput := mockey.Mock(convertModelOutput).Return(&tracespec.ModelOutput{}).Build()
			mockGetTraceVariablesValue := mockey.Mock(getTraceVariablesValue).Return(&async.TraceVariablesValue{
				StartTime: time.Now().Add(-time.Second),
			}, true).Build()

			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			mockConvertModelOutput.UnPatch()
			mockGetTraceVariablesValue.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 ComponentOfChatModel 时，且为2级节点", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfChatModel,
			}
			mockGetTraceVariablesValue := mockey.Mock(getTraceVariablesValue).Return(&async.TraceVariablesValue{
				StartTime: time.Now().Add(-time.Second),
			}, true).Build()

			ctx = injectGraphNodeLevelToCtx(ctx, 2)
			ctx = injectAggrMessageOutputHookToCtx(ctx)
			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)
			convey.So(len(collectOutput.Messages), convey.ShouldEqual, 1)

			mockGetTraceVariablesValue.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 ComponentOfPrompt 时", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfPrompt,
			}
			mockConvPromptOutput := mockey.Mock(prompt.ConvCallbackOutput).Return(&prompt.CallbackOutput{}).Build()
			mockConvertPromptOutput := mockey.Mock(convertPromptOutput).Return(&tracespec.PromptOutput{}).Build()

			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)

			mockConvPromptOutput.UnPatch()
			mockConvertPromptOutput.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 ComponentOfPrompt 时，且为2级节点", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfPrompt,
			}
			mockConvPromptOutput := mockey.Mock(prompt.ConvCallbackOutput).Return(&prompt.CallbackOutput{
				Result: []*schema.Message{
					{Role: schema.System, Content: "system message"},
					{Role: schema.User, Content: "user message"},
				},
			}).Build()

			ctx = injectGraphNodeLevelToCtx(ctx, 2)
			ctx = injectAggrMessageOutputHookToCtx(ctx)
			result := d.ParseOutput(ctx, info, output)
			convey.So(result, convey.ShouldNotBeNil)

			collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)
			convey.So(len(collectOutput.Messages), convey.ShouldEqual, 2)
			mockConvPromptOutput.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 ComponentOfEmbedding 时", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfEmbedding,
			}

			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)

			mockParseAny.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 ComponentOfIndexer 时", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfIndexer,
			}
			mockConvIndexerOutput := mockey.Mock(indexer.ConvCallbackOutput).Return(&indexer.CallbackOutput{
				IDs: []string{"id1", "id2"},
			}).Build()
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)

			mockConvIndexerOutput.UnPatch()
			mockParseAny.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 ComponentOfRetriever 时", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfRetriever,
			}
			mockConvRetrieverOutput := mockey.Mock(retriever.ConvCallbackOutput).Return(&retriever.CallbackOutput{}).Build()
			mockConvertRetrieverOutput := mockey.Mock(convertRetrieverOutput).Return(&tracespec.RetrieverOutput{}).Build()

			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)

			mockConvRetrieverOutput.UnPatch()
			mockConvertRetrieverOutput.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 compose.ComponentOfLambda 时", func() {
			info := &callbacks.RunInfo{
				Component: compose.ComponentOfLambda,
			}
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			mockParseAny.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 compose.ComponentOfTool 时", func() {
			info := &callbacks.RunInfo{
				Component: components.ComponentOfTool,
			}
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()
			result := d.ParseOutput(ctx, info, outputs)

			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)
			mockParseAny.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为 compose.ComponentOfLambda 时，且为2级节点", func() {
			info := &callbacks.RunInfo{
				Component: compose.ComponentOfLambda,
			}
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			ctx = injectGraphNodeLevelToCtx(ctx, 2)
			ctx = injectAggrMessageOutputHookToCtx(ctx)
			result := d.ParseOutput(ctx, info, outputs)

			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)
			convey.So(len(collectOutput.Messages), convey.ShouldEqual, 1)

			mockParseAny.UnPatch()
		})
		mockey.PatchConvey("当 info.Component 为 compose.ComponentOfToolsNode 时，且为2级节点", func() {
			info := &callbacks.RunInfo{
				Component: compose.ComponentOfToolsNode,
			}
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			ctx = injectGraphNodeLevelToCtx(ctx, 2)
			ctx = injectAggrMessageOutputHookToCtx(ctx)
			result := d.ParseOutput(ctx, info, outputs)

			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)
			convey.So(len(collectOutput.Messages), convey.ShouldEqual, 1)

			mockParseAny.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为其他值时", func() {
			info := &callbacks.RunInfo{
				Component: "unknown_component",
			}
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			result := d.ParseOutput(ctx, info, output)

			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			mockParseAny.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为其他值时，且为2级节点", func() {
			info := &callbacks.RunInfo{
				Component: "unknown_component",
			}
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			ctx = injectGraphNodeLevelToCtx(ctx, 2)
			ctx = injectAggrMessageOutputHookToCtx(ctx)
			result := d.ParseOutput(ctx, info, outputs)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)
			convey.So(len(collectOutput.Messages), convey.ShouldEqual, 1)

			mockParseAny.UnPatch()
		})

		mockey.PatchConvey("当 info.Component 为其他值时，且为1级节点", func() {
			info := &callbacks.RunInfo{
				Component: "unknown_component",
			}
			mockParseAny := mockey.Mock(parseAny).Return("test_output").Build()

			ctx = injectGraphNodeLevelToCtx(ctx, 1)
			ctx = injectAggrMessageOutputHookToCtx(ctx)
			ds := defaultDataParser{
				concatFuncs: make(map[reflect.Type]any),
			}
			ds.enableAggrMessageOutput = true

			result := ds.ParseOutput(ctx, info, outputs)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(result, convey.ShouldContainKey, tracespec.Output)

			collectOutput, _ := getAggrMessageOutputHookFromCtx(ctx)
			convey.So(len(collectOutput.Messages), convey.ShouldEqual, 0)

			mockParseAny.UnPatch()
		})
	})
}

// Test_defaultDataParser_tryConcatChunks 为 defaultDataParser 的 tryConcatChunks 方法编写单元测试
func Test_defaultDataParser_tryConcatChunks(t *testing.T) {
	mockey.PatchConvey("测试 defaultDataParser 的 tryConcatChunks 方法", t, func() {
		// 场景1：输入的 chunks 切片为空
		mockey.PatchConvey("输入的 chunks 切片为空", func() {
			// 初始化 defaultDataParser 实例
			d := defaultDataParser{}
			chunks := []any{}
			// 调用待测方法
			result, err := d.tryConcatChunks(chunks)
			// 断言结果
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldResemble, chunks)
		})

		// 场景2：输入的 chunks 切片不为空，且 getConcatFunc 返回的拼接函数不为 nil
		mockey.PatchConvey("输入的 chunks 切片不为空，且 getConcatFunc 返回的拼接函数不为 nil", func() {
			// 初始化 defaultDataParser 实例
			d := defaultDataParser{}
			chunks := []any{1, 2, 3}

			result, err := d.tryConcatChunks(chunks)
			// 断言结果
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldEqual, []any{1, 2, 3})
		})

		// 场景3：输入的 chunks 切片不为空，且 getConcatFunc 返回的拼接函数为 nil
		mockey.PatchConvey("输入的 chunks 切片不为空，且 getConcatFunc 返回的拼接函数为 nil", func() {
			// 初始化 defaultDataParser 实例
			d := defaultDataParser{}
			chunks := []any{1, 2, 3}
			// 调用待测方法
			result, err := d.tryConcatChunks(chunks)
			// 断言结果
			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldResemble, chunks)
		})
	})
}

// Test_defaultDataParser_getConcatFunc 测试 defaultDataParser 的 getConcatFunc 方法
func Test_defaultDataParser_getConcatFunc(t *testing.T) {
	mockey.PatchConvey("测试 defaultDataParser 的 getConcatFunc 方法", t, func() {
		mockey.PatchConvey("场景1：concatFuncs 中存在对应类型的函数", func() {
			// 准备测试数据
			d := defaultDataParser{
				concatFuncs: make(map[reflect.Type]any),
			}
			testType := reflect.TypeOf(int(0))
			testFunc := func(a reflect.Value) (reflect.Value, error) {
				return reflect.ValueOf(1), nil
			}
			d.concatFuncs[testType] = testFunc

			// 调用待测方法
			result := d.getConcatFunc(testType)

			// 断言结果
			convey.So(result, convey.ShouldNotBeNil)
		})

		mockey.PatchConvey("场景2：concatFuncs 中不存在对应类型的函数", func() {
			// 准备测试数据
			d := defaultDataParser{
				concatFuncs: make(map[reflect.Type]any),
			}
			testType := reflect.TypeOf(int(0))

			// 调用待测方法
			result := d.getConcatFunc(testType)

			// 断言结果
			convey.So(result, convey.ShouldBeNil)
		})
	})
}

// Test_parseAny 测试 parseAny 函数
func Test_parseAny(t *testing.T) {
	mockey.PatchConvey("测试 parseAny 函数", t, func() {
		mockey.PatchConvey("输入为 nil 的情况", func() {
			// Arrange
			ctx := context.Background()
			var v any = nil
			bStream := false

			// Act
			result := parseAny(ctx, v, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, "")
		})

		mockey.PatchConvey("输入为 []*schema.Message 的情况", func() {
			// Arrange
			ctx := context.Background()
			msg := &schema.Message{
				Role:    "user",
				Content: "Hello",
			}
			msgs := []*schema.Message{msg}
			bStream := false
			expectedJSON, _ := json.Marshal(msgs)
			expected := string(expectedJSON)

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, msgs, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 *schema.Message 的情况", func() {
			// Arrange
			ctx := context.Background()
			msg := &schema.Message{
				Role:    "user",
				Content: "Hello",
			}
			bStream := false
			expectedJSON, _ := json.Marshal(msg)
			expected := string(expectedJSON)

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, msg, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 string 且 bStream 为 false 的情况", func() {
			// Arrange
			ctx := context.Background()
			inputStr := "test string"
			bStream := false

			// Act
			result := parseAny(ctx, inputStr, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, inputStr)
		})

		mockey.PatchConvey("输入为 string 且 bStream 为 true 的情况", func() {
			// Arrange
			ctx := context.Background()
			inputStr := "test string"
			bStream := true
			expectedJSON, _ := json.Marshal(inputStr)
			expected := string(expectedJSON)

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, inputStr, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 json.Marshaler 的情况", func() {
			// Arrange
			ctx := context.Background()
			marshaler := json.RawMessage(`{"key": "value"}`)
			bStream := false
			expectedJSON, _ := json.Marshal(marshaler)
			expected := string(expectedJSON)

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, marshaler, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 map[string]any 的情况", func() {
			// Arrange
			ctx := context.Background()
			inputMap := map[string]any{
				"key": "value",
			}
			bStream := false
			expectedJSON, _ := json.Marshal(inputMap)
			expected := string(expectedJSON)

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, inputMap, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 []callbacks.CallbackInput 的情况", func() {
			// Arrange
			ctx := context.Background()
			input := callbacks.CallbackInput("test input")
			inputs := []callbacks.CallbackInput{input}
			bStream := false
			expectedSlice := []any{input}
			expectedJSON, _ := json.Marshal(expectedSlice)
			expected := string(expectedJSON)

			// Mock toAnySlice 函数
			//mockey.Mock(toAnySlice).To(func(src []callbacks.CallbackInput) []any {
			//	return expectedSlice
			//}).Build()

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, inputs, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 []callbacks.CallbackOutput 的情况", func() {
			// Arrange
			ctx := context.Background()
			output := callbacks.CallbackOutput("test output")
			outputs := []callbacks.CallbackOutput{output}
			bStream := false
			expectedSlice := []any{output}
			expectedJSON, _ := json.Marshal(expectedSlice)
			expected := string(expectedJSON)

			// Mock toAnySlice 函数
			//mockey.Mock(toAnySlice).To(func(src []callbacks.CallbackOutput) []any {
			//	return expectedSlice
			//}).Build()

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, outputs, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 []any 且第一个元素为 *schema.Message 的情况", func() {
			// Arrange
			ctx := context.Background()
			msg := &schema.Message{
				Role:    "user",
				Content: "Hello",
			}
			inputSlice := []any{msg}
			bStream := false
			msgs := []*schema.Message{msg}
			expectedJSON, _ := json.Marshal(msgs)
			expected := string(expectedJSON)

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, inputSlice, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})

		mockey.PatchConvey("输入为 []any 且第一个元素不是 *schema.Message 的情况", func() {
			// Arrange
			ctx := context.Background()
			inputSlice := []any{1, 2, 3}
			bStream := false
			expectedJSON, _ := json.Marshal(inputSlice)
			expected := string(expectedJSON)

			// Mock toJson 函数
			mockey.Mock(toJson).To(func(v any, bStream bool) string {
				return expected
			}).Build()

			// Act
			result := parseAny(ctx, inputSlice, bStream)

			// Assert
			convey.So(result, convey.ShouldEqual, expected)
		})
	})
}

// Test_toAnySlice 为 toAnySlice 函数编写的单元测试
func Test_toAnySlice(t *testing.T) {
	mockey.PatchConvey("测试 toAnySlice 函数", t, func() {
		mockey.PatchConvey("输入为空切片的情况", func() {
			// 准备输入数据，一个空的 int 切片
			src := []int{}
			// 调用待测函数
			result := toAnySlice(src)
			// 断言返回的切片为空
			convey.So(result, convey.ShouldBeEmpty)
		})

		mockey.PatchConvey("输入为非空切片的情况", func() {
			// 准备输入数据，一个包含元素的 int 切片
			src := []int{1, 2, 3}
			// 调用待测函数
			result := toAnySlice(src)
			// 断言返回的切片长度与输入切片长度相同
			convey.So(len(result), convey.ShouldEqual, len(src))
			// 遍历输入切片和返回切片，断言对应元素相等
			for i := range src {
				convey.So(result[i], convey.ShouldEqual, src[i])
			}
		})
	})
}

// Test_parseSpanTypeFromComponent 测试 parseSpanTypeFromComponent 函数
func Test_parseSpanTypeFromComponent(t *testing.T) {
	mockey.PatchConvey("测试 parseSpanTypeFromComponent 函数", t, func() {
		mockey.PatchConvey("测试 ComponentOfPrompt 输入", func() {
			// 调用待测函数
			result := parseSpanTypeFromComponent(components.ComponentOfPrompt)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, "prompt")
		})
		mockey.PatchConvey("测试 ComponentOfChatModel 输入", func() {
			// 调用待测函数
			result := parseSpanTypeFromComponent(components.ComponentOfChatModel)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, "model")
		})
		mockey.PatchConvey("测试 ComponentOfEmbedding 输入", func() {
			// 调用待测函数
			result := parseSpanTypeFromComponent(components.ComponentOfEmbedding)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, "embedding")
		})
		mockey.PatchConvey("测试 ComponentOfIndexer 输入", func() {
			// 调用待测函数
			result := parseSpanTypeFromComponent(components.ComponentOfIndexer)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, "store")
		})
		mockey.PatchConvey("测试 ComponentOfRetriever 输入", func() {
			// 调用待测函数
			result := parseSpanTypeFromComponent(components.ComponentOfRetriever)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, "retriever")
		})
		mockey.PatchConvey("测试 ComponentOfLoader 输入", func() {
			// 调用待测函数
			result := parseSpanTypeFromComponent(components.ComponentOfLoader)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, "loader")
		})
		mockey.PatchConvey("测试 ComponentOfTool 输入", func() {
			// 调用待测函数
			result := parseSpanTypeFromComponent(components.ComponentOfTool)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, "tool")
		})
		mockey.PatchConvey("测试默认情况输入", func() {
			// 定义一个不在常量列表中的 Component
			unknownComponent := components.Component("UnknownComponent")
			// 调用待测函数
			result := parseSpanTypeFromComponent(unknownComponent)
			// 断言结果是否符合预期
			convey.So(result, convey.ShouldEqual, string(unknownComponent))
		})
	})
}
