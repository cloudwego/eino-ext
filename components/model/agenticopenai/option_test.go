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
	"github.com/cloudwego/eino/components/model"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/assert"
)

func TestWithStore(t *testing.T) {
	mockey.PatchConvey("WithStore", t, func() {
		mockey.PatchConvey("set store to true", func() {
			opt := WithStore(true)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.store)
			assert.True(t, *opts.store)
		})

		mockey.PatchConvey("set store to false", func() {
			opt := WithStore(false)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.store)
			assert.False(t, *opts.store)
		})
	})
}

func TestWithPromptCacheKey(t *testing.T) {
	mockey.PatchConvey("WithPromptCacheKey", t, func() {
		mockey.PatchConvey("set cache key", func() {
			key := "test-cache-key"
			opt := WithPromptCacheKey(key)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.promptCacheKey)
			assert.Equal(t, key, *opts.promptCacheKey)
		})

		mockey.PatchConvey("set empty cache key", func() {
			opt := WithPromptCacheKey("")
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.promptCacheKey)
			assert.Equal(t, "", *opts.promptCacheKey)
		})
	})
}

func TestWithReasoning(t *testing.T) {
	mockey.PatchConvey("WithReasoning", t, func() {
		mockey.PatchConvey("set reasoning param", func() {
			reasoning := &responses.ReasoningParam{
				Effort: responses.ReasoningEffortLow,
			}
			opt := WithReasoning(reasoning)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.reasoning)
			assert.Equal(t, reasoning, opts.reasoning)
		})

		mockey.PatchConvey("set nil reasoning", func() {
			opt := WithReasoning(nil)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.Nil(t, opts.reasoning)
		})
	})
}

func TestWithText(t *testing.T) {
	mockey.PatchConvey("WithText", t, func() {
		mockey.PatchConvey("set text config", func() {
			text := &responses.ResponseTextConfigParam{
				Verbosity: responses.ResponseTextConfigVerbosityLow,
			}
			opt := WithText(text)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.text)
			assert.Equal(t, text, opts.text)
		})

		mockey.PatchConvey("set nil text", func() {
			opt := WithText(nil)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.Nil(t, opts.text)
		})
	})
}

func TestWithMaxToolCalls(t *testing.T) {
	mockey.PatchConvey("WithMaxToolCalls", t, func() {
		mockey.PatchConvey("set positive value", func() {
			maxCalls := 5
			opt := WithMaxToolCalls(maxCalls)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.maxToolCalls)
			assert.Equal(t, maxCalls, *opts.maxToolCalls)
		})

		mockey.PatchConvey("set zero value", func() {
			opt := WithMaxToolCalls(0)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.maxToolCalls)
			assert.Equal(t, 0, *opts.maxToolCalls)
		})
	})
}

func TestWithParallelToolCalls(t *testing.T) {
	mockey.PatchConvey("WithParallelToolCalls", t, func() {
		mockey.PatchConvey("set to true", func() {
			opt := WithParallelToolCalls(true)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.parallelToolCalls)
			assert.True(t, *opts.parallelToolCalls)
		})

		mockey.PatchConvey("set to false", func() {
			opt := WithParallelToolCalls(false)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.parallelToolCalls)
			assert.False(t, *opts.parallelToolCalls)
		})
	})
}

func TestWithServerTools(t *testing.T) {
	mockey.PatchConvey("WithServerTools", t, func() {
		mockey.PatchConvey("set server tools", func() {
			tools := []*ServerToolConfig{
				{
					WebSearch: &responses.WebSearchToolParam{
						Type: responses.WebSearchToolTypeWebSearch,
					},
				},
			}
			opt := WithServerTools(tools)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.serverTools)
			assert.Len(t, opts.serverTools, 1)
			assert.Equal(t, tools, opts.serverTools)
		})

		mockey.PatchConvey("set empty tools", func() {
			opt := WithServerTools([]*ServerToolConfig{})
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.serverTools)
			assert.Len(t, opts.serverTools, 0)
		})

		mockey.PatchConvey("set nil tools", func() {
			opt := WithServerTools(nil)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.Nil(t, opts.serverTools)
		})
	})
}

func TestWithMCPTools(t *testing.T) {
	mockey.PatchConvey("WithMCPTools", t, func() {
		mockey.PatchConvey("set mcp tools", func() {
			tools := []*responses.ToolMcpParam{
				{
					ServerLabel: "test-server",
				},
			}
			opt := WithMCPTools(tools)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.mcpTools)
			assert.Len(t, opts.mcpTools, 1)
			assert.Equal(t, tools, opts.mcpTools)
		})

		mockey.PatchConvey("set empty tools", func() {
			opt := WithMCPTools([]*responses.ToolMcpParam{})
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.mcpTools)
			assert.Len(t, opts.mcpTools, 0)
		})

		mockey.PatchConvey("set nil tools", func() {
			opt := WithMCPTools(nil)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.Nil(t, opts.mcpTools)
		})
	})
}

func TestWithCustomHeaders(t *testing.T) {
	mockey.PatchConvey("WithCustomHeaders", t, func() {
		mockey.PatchConvey("set custom headers", func() {
			headers := map[string]string{
				"X-Custom-Header": "value",
				"Authorization":   "Bearer token",
			}
			opt := WithCustomHeaders(headers)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.customHeaders)
			assert.Equal(t, headers, opts.customHeaders)
		})

		mockey.PatchConvey("set empty headers", func() {
			opt := WithCustomHeaders(map[string]string{})
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.customHeaders)
			assert.Len(t, opts.customHeaders, 0)
		})

		mockey.PatchConvey("set nil headers", func() {
			opt := WithCustomHeaders(nil)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.Nil(t, opts.customHeaders)
		})
	})
}

func TestWithExtraFields(t *testing.T) {
	mockey.PatchConvey("WithExtraFields", t, func() {
		mockey.PatchConvey("set extra fields", func() {
			fields := map[string]any{
				"field1": "value1",
				"field2": 123,
				"field3": true,
			}
			opt := WithExtraFields(fields)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.extraFields)
			assert.Equal(t, fields, opts.extraFields)
		})

		mockey.PatchConvey("set empty fields", func() {
			opt := WithExtraFields(map[string]any{})
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.NotNil(t, opts.extraFields)
			assert.Len(t, opts.extraFields, 0)
		})

		mockey.PatchConvey("set nil fields", func() {
			opt := WithExtraFields(nil)
			opts := model.GetImplSpecificOptions(&openaiOptions{}, opt)
			assert.Nil(t, opts.extraFields)
		})
	})
}
