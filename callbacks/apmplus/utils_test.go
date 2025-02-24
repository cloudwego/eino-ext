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

package apmplus

import (
	"errors"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/smartystreets/goconvey/convey"
)

func Test_getName(t *testing.T) {
	mockey.PatchConvey("Test getName with non-empty Name", t, func() {
		info := &callbacks.RunInfo{Name: "testName", Type: "testType", Component: components.ComponentOfEmbedding}
		actual := getName(info)
		convey.So(actual, convey.ShouldEqual, "testName")
	})
	mockey.PatchConvey("Test getName with empty Name", t, func() {
		info := &callbacks.RunInfo{Name: "", Type: "testType", Component: components.ComponentOfEmbedding}
		actual := getName(info)
		convey.So(actual, convey.ShouldEqual, "testType "+string(components.ComponentOfEmbedding))
	})
	mockey.PatchConvey("Test getName with empty Name and Type", t, func() {
		info := &callbacks.RunInfo{Name: "", Type: "", Component: components.ComponentOfEmbedding}
		actual := getName(info)
		convey.So(actual, convey.ShouldEqual, string(components.ComponentOfEmbedding))
	})
	mockey.PatchConvey("Test getName with empty Name and Component", t, func() {
		info := &callbacks.RunInfo{Name: "", Type: "testType", Component: ""}
		actual := getName(info)
		convey.So(actual, convey.ShouldEqual, "testType")
	})
	mockey.PatchConvey("Test getName with all empty fields", t, func() {
		info := &callbacks.RunInfo{Name: "", Type: "", Component: ""}
		actual := getName(info)
		convey.So(actual, convey.ShouldEqual, "")
	})
}

func Test_extractModelInput(t *testing.T) {
	mockey.PatchConvey("Test extractModelInput with nil inputs", t, func() {
		actualConfig, actualMessages, actualExtra, actualErr := extractModelInput(nil)
		convey.So(actualConfig, convey.ShouldBeNil)
		convey.So(actualMessages, convey.ShouldBeEmpty)
		convey.So(actualExtra, convey.ShouldBeNil)
		convey.So(actualErr, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test extractModelInput with empty inputs", t, func() {
		actualConfig, actualMessages, actualExtra, actualErr := extractModelInput([]*model.CallbackInput{})
		convey.So(actualConfig, convey.ShouldBeNil)
		convey.So(actualMessages, convey.ShouldBeEmpty)
		convey.So(actualExtra, convey.ShouldBeNil)
		convey.So(actualErr, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test extractModelInput with valid inputs", t, func() {
		inputs := []*model.CallbackInput{
			{
				Messages: []*schema.Message{
					{Role: "user", Content: "Hello"},
				},
				Config: &model.Config{
					Model: "gpt-3.5-turbo",
				},
				Extra: map[string]interface{}{
					"key1": "value1",
				},
			},
			{
				Messages: []*schema.Message{
					{Role: "assistant", Content: "Hi there!"},
				},
				Extra: map[string]interface{}{
					"key2": "value2",
				},
			},
		}

		expectedConfig := &model.Config{
			Model: "gpt-3.5-turbo",
		}
		expectedMessages := []*schema.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		}
		expectedExtra := map[string]interface{}{
			"key2": "value2",
		}

		mockey.Mock(concatMessageArray).Return(expectedMessages, nil).Build()
		actualConfig, actualMessages, actualExtra, actualErr := extractModelInput(inputs)
		convey.So(actualConfig, convey.ShouldEqual, expectedConfig)
		convey.So(actualMessages, convey.ShouldEqual, expectedMessages)
		convey.So(actualExtra, convey.ShouldEqual, expectedExtra)
		convey.So(actualErr, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test extractModelInput with concatMessageArray failure", t, func() {
		inputs := []*model.CallbackInput{
			{
				Messages: []*schema.Message{
					{Role: "user", Content: "Hello"},
				},
				Config: &model.Config{
					Model: "gpt-3.5-turbo",
				},
				Extra: map[string]interface{}{
					"key1": "value1",
				},
			},
			{
				Messages: []*schema.Message{
					{Role: "assistant", Content: "Hi there!"},
				},
				Extra: map[string]interface{}{
					"key2": "value2",
				},
			},
		}

		mockey.Mock(concatMessageArray).Return(nil, errors.New("concat error")).Build()
		actualConfig, actualMessages, actualExtra, actualErr := extractModelInput(inputs)
		convey.So(actualConfig, convey.ShouldBeNil)
		convey.So(actualMessages, convey.ShouldBeNil)
		convey.So(actualExtra, convey.ShouldBeNil)
		convey.So(actualErr, convey.ShouldNotBeNil)
	})
}

func Test_extractModelOutput(t *testing.T) {
	mockey.PatchConvey("Test with nil outputs", t, func() {
		usage, messages, extra, config, err := extractModelOutput(nil)
		convey.So(usage, convey.ShouldBeNil)
		convey.So(messages, convey.ShouldBeNil)
		convey.So(extra, convey.ShouldBeNil)
		convey.So(config, convey.ShouldBeNil)
		convey.So(err, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test with empty outputs", t, func() {
		usage, messages, extra, config, err := extractModelOutput([]*model.CallbackOutput{})
		convey.So(usage, convey.ShouldBeNil)
		convey.So(messages, convey.ShouldBeNil)
		convey.So(extra, convey.ShouldBeNil)
		convey.So(config, convey.ShouldBeNil)
		convey.So(err, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test with single nil output", t, func() {
		usage, messages, extra, config, err := extractModelOutput([]*model.CallbackOutput{nil})
		convey.So(usage, convey.ShouldBeNil)
		convey.So(messages, convey.ShouldBeNil)
		convey.So(extra, convey.ShouldBeNil)
		convey.So(config, convey.ShouldBeNil)
		convey.So(err, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test with single valid output", t, func() {
		expectedUsage := &model.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
		expectedMessage := &schema.Message{Role: "user", Content: "Hello"}
		expectedExtra := map[string]interface{}{"key": "value"}
		expectedConfig := &model.Config{Model: "gpt-3.5-turbo", MaxTokens: 100, Temperature: 0.7, TopP: 0.9, Stop: []string{"\n"}}
		output := &model.CallbackOutput{
			Message:    expectedMessage,
			Config:     expectedConfig,
			TokenUsage: expectedUsage,
			Extra:      expectedExtra,
		}

		usage, messages, extra, config, err := extractModelOutput([]*model.CallbackOutput{output})
		convey.So(usage, convey.ShouldEqual, expectedUsage)
		convey.So(messages, convey.ShouldHaveLength, 1)
		convey.So(messages[0], convey.ShouldEqual, expectedMessage)
		convey.So(extra, convey.ShouldEqual, expectedExtra)
		convey.So(config, convey.ShouldEqual, expectedConfig)
		convey.So(err, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test with multiple valid outputs", t, func() {
		expectedUsage := &model.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
		expectedMessage1 := &schema.Message{Role: "user", Content: "Hello"}
		expectedMessage2 := &schema.Message{Role: "assistant", Content: "Hi there"}
		expectedExtra := map[string]interface{}{"key": "value"}
		expectedConfig := &model.Config{Model: "gpt-3.5-turbo", MaxTokens: 100, Temperature: 0.7, TopP: 0.9, Stop: []string{"\n"}}
		output1 := &model.CallbackOutput{
			Message:    expectedMessage1,
			Config:     expectedConfig,
			TokenUsage: expectedUsage,
			Extra:      expectedExtra,
		}
		output2 := &model.CallbackOutput{
			Message:    expectedMessage2,
			Config:     expectedConfig,
			TokenUsage: expectedUsage,
			Extra:      expectedExtra,
		}

		usage, messages, extra, config, err := extractModelOutput([]*model.CallbackOutput{output1, output2})
		convey.So(usage, convey.ShouldEqual, expectedUsage)
		convey.So(messages, convey.ShouldHaveLength, 2)
		convey.So(messages[0], convey.ShouldBeIn, []*schema.Message{expectedMessage1, expectedMessage2})
		convey.So(messages[1], convey.ShouldBeIn, []*schema.Message{expectedMessage1, expectedMessage2})
		convey.So(extra, convey.ShouldEqual, expectedExtra)
		convey.So(config, convey.ShouldEqual, expectedConfig)
		convey.So(err, convey.ShouldBeNil)
	})
}

func Test_concatMessageArray(t *testing.T) {
	mockey.PatchConvey("Test empty input", t, func() {
		mas := [][]*schema.Message{}
		actual, err := concatMessageArray(mas)
		convey.So(actual, convey.ShouldBeNil)
		convey.So(err, convey.ShouldNotBeNil)
	})

	mockey.PatchConvey("Test single message array with correct length", t, func() {
		mas := [][]*schema.Message{
			{&schema.Message{Role: "user", Content: "Hello"}},
		}
		expected := []*schema.Message{
			{Role: "user", Content: "Hello"},
		}
		actual, err := concatMessageArray(mas)
		convey.So(actual, convey.ShouldEqual, expected)
		convey.So(err, convey.ShouldBeNil)
	})

	mockey.PatchConvey("Test different lengths of message arrays", t, func() {
		mas := [][]*schema.Message{
			{&schema.Message{Role: "user", Content: "Hello"}},
			{&schema.Message{Role: "user", Content: "How are you?"}, &schema.Message{Role: "assistant", Content: "I'm good, thanks!"}},
		}
		actual, err := concatMessageArray(mas)
		convey.So(actual, convey.ShouldBeNil)
		convey.So(err, convey.ShouldNotBeNil)
	})
}

func Test_convSchemaMessage(t *testing.T) {
	mockey.PatchConvey("Test convSchemaMessage with empty input", t, func() {
		input := []*schema.Message{}
		actual := convSchemaMessage(input)
		convey.So(actual, convey.ShouldBeEmpty)
	})

	mockey.PatchConvey("Test convSchemaMessage with nil input", t, func() {
		input := []*schema.Message{nil}
		expected := []*model.CallbackInput{nil}
		mockey.Mock(model.ConvCallbackInput).Return(nil).Build()
		actual := convSchemaMessage(input)
		convey.So(actual, convey.ShouldResemble, expected)
	})
}
