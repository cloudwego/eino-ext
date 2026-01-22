/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"testing"

	"github.com/bytedance/mockey"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestGetServerToolCallArguments(t *testing.T) {
	mockey.PatchConvey("getServerToolCallArguments", t, func() {
		mockey.PatchConvey("success", func() {
			args := &ServerToolCallArguments{}
			call := &schema.ServerToolCall{
				Arguments: args,
			}
			res, err := getServerToolCallArguments(call)
			assert.NoError(t, err)
			assert.Equal(t, args, res)
		})

		mockey.PatchConvey("nil input", func() {
			res, err := getServerToolCallArguments(nil)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("nil arguments", func() {
			call := &schema.ServerToolCall{
				Arguments: nil,
			}
			res, err := getServerToolCallArguments(call)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("wrong type", func() {
			call := &schema.ServerToolCall{
				Arguments: "wrong type",
			}
			res, err := getServerToolCallArguments(call)
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	})
}

func TestGetServerToolResult(t *testing.T) {
	mockey.PatchConvey("getServerToolResult", t, func() {
		mockey.PatchConvey("success", func() {
			result := &ServerToolResult{}
			content := &schema.ServerToolResult{
				Result: result,
			}
			res, err := getServerToolResult(content)
			assert.NoError(t, err)
			assert.Equal(t, result, res)
		})

		mockey.PatchConvey("nil input", func() {
			res, err := getServerToolResult(nil)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("nil result", func() {
			content := &schema.ServerToolResult{
				Result: nil,
			}
			res, err := getServerToolResult(content)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("wrong type", func() {
			content := &schema.ServerToolResult{
				Result: "wrong type",
			}
			res, err := getServerToolResult(content)
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	})
}

func TestConcatServerToolCallArguments(t *testing.T) {
	mockey.PatchConvey("concatServerToolCallArguments", t, func() {
		mockey.PatchConvey("empty chunks", func() {
			res, err := concatServerToolCallArguments(nil)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("one chunk", func() {
			args := &ServerToolCallArguments{}
			res, err := concatServerToolCallArguments([]*ServerToolCallArguments{args})
			assert.NoError(t, err)
			assert.Equal(t, args, res)
		})

		mockey.PatchConvey("multiple chunks", func() {
			args1 := &ServerToolCallArguments{}
			args2 := &ServerToolCallArguments{}
			res, err := concatServerToolCallArguments([]*ServerToolCallArguments{args1, args2})
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	})
}

func TestConcatServerToolResult(t *testing.T) {
	mockey.PatchConvey("concatServerToolResult", t, func() {
		mockey.PatchConvey("empty chunks", func() {
			res, err := concatServerToolResult(nil)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

		mockey.PatchConvey("one chunk", func() {
			result := &ServerToolResult{}
			res, err := concatServerToolResult([]*ServerToolResult{result})
			assert.NoError(t, err)
			assert.Equal(t, result, res)
		})

		mockey.PatchConvey("multiple chunks", func() {
			result1 := &ServerToolResult{}
			result2 := &ServerToolResult{}
			res, err := concatServerToolResult([]*ServerToolResult{result1, result2})
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	})
}
