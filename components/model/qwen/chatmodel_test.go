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

package qwen

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/bytedance/mockey"
	"github.com/smartystreets/goconvey/convey"

	"github.com/cloudwego/eino/schema"
)

func TestChatModel(t *testing.T) {
	PatchConvey("test ChatModel", t, func() {
		ctx := context.Background()
		cm, err := NewChatModel(ctx, nil)
		convey.So(err, convey.ShouldNotBeNil)

		cm, err = NewChatModel(ctx, &ChatModelConfig{
			BaseURL: "asd",
			APIKey:  "qwe",
			Model:   "zxc",
		})
		convey.So(err, convey.ShouldBeNil)
		convey.So(cm, convey.ShouldNotBeNil)

		cli := cm.cli

		PatchConvey("test Generate", func() {
			Mock(GetMethod(cli, "Generate")).Return(nil, fmt.Errorf("mock err")).Build()
			msg, err := cm.Generate(ctx, []*schema.Message{
				schema.UserMessage("hello"),
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(msg, convey.ShouldBeNil)
		})

		PatchConvey("test Stream", func() {
			Mock(GetMethod(cli, "Stream")).Return(nil, fmt.Errorf("mock err")).Build()
			sr, err := cm.Stream(ctx, []*schema.Message{
				schema.UserMessage("hello"),
			})
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(sr, convey.ShouldBeNil)
		})
	})
}

func TestNewPartialMessage(t *testing.T) {
	PatchConvey("test new message with partial flag in Extra", t, func() {
		PatchConvey("test Extra is nil", func() {
			message := &schema.Message{}

			partialMessage := NewPartialMessage(message, true)

			convey.So(partialMessage.Extra, convey.ShouldNotBeNil)
			convey.So(partialMessage.Extra[fieldNamePartialInExtraMsg], convey.ShouldBeTrue)
		})

		PatchConvey("test Extra is not nil", func() {
			extra := make(map[string]any)
			message := &schema.Message{
				Extra: extra,
			}

			partialMessage := NewPartialMessage(message, true)

			convey.So(partialMessage.Extra, convey.ShouldNotBeNil)
			convey.So(partialMessage.Extra[fieldNamePartialInExtraMsg], convey.ShouldBeTrue)
		})

		PatchConvey("test Extra is not nil and keep value", func() {
			extra := make(map[string]any)
			extra["testKey"] = "testValue"
			message := &schema.Message{
				Extra: extra,
			}

			partialMessage := NewPartialMessage(message, true)

			convey.So(partialMessage.Extra, convey.ShouldNotBeNil)
			convey.So(partialMessage.Extra[fieldNamePartialInExtraMsg], convey.ShouldBeTrue)
			convey.So(partialMessage.Extra["testKey"], convey.ShouldEqual, "testValue")
		})
	})
}

func TestAppendBodyModifierOptions(t *testing.T) {
	PatchConvey("test append body modifier options", t, func() {
		ctx := context.Background()
		cm, err := NewChatModel(ctx, nil)
		convey.So(err, convey.ShouldNotBeNil)

		PatchConvey("test no single message is partial", func() {
			in := []*schema.Message{
				schema.UserMessage("aaa"),
				schema.UserMessage("bbb"),
			}
			opts := cm.appendBodyModifierOptions(in)
			convey.So(opts, convey.ShouldBeNil)
		})

		PatchConvey("test 1 single message is partial", func() {
			in := []*schema.Message{
				schema.UserMessage("aaa"),
				NewPartialMessage(schema.UserMessage("bbb"), true),
			}
			opts := cm.appendBodyModifierOptions(in)
			convey.So(opts, convey.ShouldNotBeNil)
			convey.So(opts, convey.ShouldHaveLength, 1)
		})

		PatchConvey("test multi different messages are partial", func() {
			in := []*schema.Message{
				NewPartialMessage(schema.UserMessage("aaa"), true),
				NewPartialMessage(schema.UserMessage("bbb"), true),
			}
			opts := cm.appendBodyModifierOptions(in)
			convey.So(opts, convey.ShouldNotBeNil)
			convey.So(opts, convey.ShouldHaveLength, 1)
		})

		PatchConvey("test multi same messages are partial", func() {
			in := []*schema.Message{
				schema.UserMessage("aaa"),
				NewPartialMessage(schema.UserMessage("bbb"), true),
				NewPartialMessage(schema.UserMessage("bbb"), true),
			}
			opts := cm.appendBodyModifierOptions(in)
			convey.So(opts, convey.ShouldNotBeNil)
			convey.So(opts, convey.ShouldHaveLength, 1)
		})
	})
}

// {"max_tokens":2048,"model":"qwen-plus","temperature":0.7,"top_p":0.7,
// "messages":[{"content":"aaa","role":"user"},{"content":"bbb","role":"assistant"}]}
func buildChatCompletionRequest(in ...*schema.Message) []byte {
	requestBody := make(map[string]any)
	requestBody["max_tokens"] = 2048
	requestBody["model"] = "qwen-plus"
	requestBody["temperature"] = 0.7
	requestBody["top_p"] = 0.7
	messages := make([]any, 0)
	for _, inMsg := range in {
		message := make(map[string]any)
		message["content"] = inMsg.Content
		message["role"] = inMsg.Role
		messages = append(messages, message)
	}
	requestBody["messages"] = messages
	bytes, _ := json.Marshal(&requestBody)
	return bytes
}

func TestModifyRequestBody(t *testing.T) {
	PatchConvey("test modify request body", t, func() {
		PatchConvey("test no single message is partial", func() {
			bytes := buildChatCompletionRequest(schema.UserMessage("aaa"),
				schema.UserMessage("bbb"))
			modifiedBytes := modifyRequestBody(bytes, make(map[string]struct{}))
			convey.So(modifiedBytes, convey.ShouldHaveLength, len(bytes))
		})

		PatchConvey("test 1 single message is partial", func() {
			bytes := buildChatCompletionRequest(schema.UserMessage("aaa"),
				schema.UserMessage("bbb"))
			messageKeys := make(map[string]struct{})
			messageKeys["userbbb"] = struct{}{}

			modifiedBytes := modifyRequestBody(bytes, messageKeys)

			var modifiedRequest map[string]interface{}
			err := json.Unmarshal(modifiedBytes, &modifiedRequest)
			convey.So(err, convey.ShouldBeNil)
			convey.So(modifiedRequest, convey.ShouldNotBeNil)
			messages := modifiedRequest["messages"].([]any)
			convey.So(messages, convey.ShouldNotBeNil)
			convey.So(messages, convey.ShouldHaveLength, 2)
			message1 := messages[0].(map[string]any)
			convey.So(message1["partial"], convey.ShouldBeNil)
			message2 := messages[1].(map[string]any)
			convey.So(message2["partial"].(bool), convey.ShouldBeTrue)
		})

		PatchConvey("test multi different messages are partial", func() {
			bytes := buildChatCompletionRequest(schema.UserMessage("aaa"),
				schema.UserMessage("bbb"))
			messageKeys := make(map[string]struct{})
			messageKeys["useraaa"] = struct{}{}
			messageKeys["userbbb"] = struct{}{}

			modifiedBytes := modifyRequestBody(bytes, messageKeys)

			var modifiedRequest map[string]interface{}
			err := json.Unmarshal(modifiedBytes, &modifiedRequest)
			convey.So(err, convey.ShouldBeNil)
			convey.So(modifiedRequest, convey.ShouldNotBeNil)
			messages := modifiedRequest["messages"].([]any)
			convey.So(messages, convey.ShouldNotBeNil)
			convey.So(messages, convey.ShouldHaveLength, 2)
			message1 := messages[0].(map[string]any)
			convey.So(message1["partial"].(bool), convey.ShouldBeTrue)
			message2 := messages[1].(map[string]any)
			convey.So(message2["partial"].(bool), convey.ShouldBeTrue)
		})

		PatchConvey("test multi same messages are partial", func() {
			bytes := buildChatCompletionRequest(schema.UserMessage("aaa"),
				schema.UserMessage("aaa"))
			messageKeys := make(map[string]struct{})
			messageKeys["useraaa"] = struct{}{}

			modifiedBytes := modifyRequestBody(bytes, messageKeys)

			var modifiedRequest map[string]interface{}
			err := json.Unmarshal(modifiedBytes, &modifiedRequest)
			convey.So(err, convey.ShouldBeNil)
			convey.So(modifiedRequest, convey.ShouldNotBeNil)
			messages := modifiedRequest["messages"].([]any)
			convey.So(messages, convey.ShouldNotBeNil)
			convey.So(messages, convey.ShouldHaveLength, 2)
			message1 := messages[0].(map[string]any)
			convey.So(message1["partial"].(bool), convey.ShouldBeTrue)
			message2 := messages[1].(map[string]any)
			convey.So(message2["partial"].(bool), convey.ShouldBeTrue)
		})

		PatchConvey("test invalid request body", func() {
			bytes := []byte("abcdefg")
			messageKeys := make(map[string]struct{})

			modifiedBytes := modifyRequestBody(bytes, messageKeys)

			convey.So(modifiedBytes, convey.ShouldNotBeNil)
			convey.So(modifiedBytes, convey.ShouldEqual, bytes)
		})

		PatchConvey("test no messages in request body", func() {
			bytes := []byte("{}")
			messageKeys := make(map[string]struct{})

			modifiedBytes := modifyRequestBody(bytes, messageKeys)

			convey.So(modifiedBytes, convey.ShouldNotBeNil)
			convey.So(modifiedBytes, convey.ShouldEqual, bytes)
		})

		PatchConvey("test messages not slice in request body", func() {
			bytes := []byte("{\"messages\": 999}")
			messageKeys := make(map[string]struct{})

			modifiedBytes := modifyRequestBody(bytes, messageKeys)

			convey.So(modifiedBytes, convey.ShouldNotBeNil)
			convey.So(modifiedBytes, convey.ShouldEqual, bytes)
		})

		PatchConvey("test message not map in request body", func() {
			bytes := []byte("{\"messages\": [\"xxx\", \"yyy\"]}")
			messageKeys := make(map[string]struct{})

			modifiedBytes := modifyRequestBody(bytes, messageKeys)

			convey.So(modifiedBytes, convey.ShouldNotBeNil)
			convey.So(modifiedBytes, convey.ShouldEqual, bytes)
		})
	})
}
