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

func TestWithResponsesStore(t *testing.T) {
	mockey.PatchConvey("WithResponsesStore", t, func() {
		mockey.PatchConvey("set store to true", func() {
			opt := WithResponsesStore(true)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.store)
			assert.True(t, *opts.store)
		})

		mockey.PatchConvey("set store to false", func() {
			opt := WithResponsesStore(false)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.store)
			assert.False(t, *opts.store)
		})
	})
}

func TestWithResponsesPromptCacheKey(t *testing.T) {
	mockey.PatchConvey("WithResponsesPromptCacheKey", t, func() {
		mockey.PatchConvey("set cache key", func() {
			key := "test-cache-key"
			opt := WithResponsesPromptCacheKey(key)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.promptCacheKey)
			assert.Equal(t, key, *opts.promptCacheKey)
		})

		mockey.PatchConvey("set empty cache key", func() {
			opt := WithResponsesPromptCacheKey("")
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.promptCacheKey)
			assert.Equal(t, "", *opts.promptCacheKey)
		})
	})
}

func TestWithResponsesReasoning(t *testing.T) {
	mockey.PatchConvey("WithResponsesReasoning", t, func() {
		mockey.PatchConvey("set reasoning param", func() {
			reasoning := &responses.ReasoningParam{
				Effort: responses.ReasoningEffortLow,
			}
			opt := WithResponsesReasoning(reasoning)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.reasoning)
			assert.Equal(t, reasoning, opts.reasoning)
		})

		mockey.PatchConvey("set nil reasoning", func() {
			opt := WithResponsesReasoning(nil)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.Nil(t, opts.reasoning)
		})
	})
}

func TestWithResponsesText(t *testing.T) {
	mockey.PatchConvey("WithResponsesText", t, func() {
		mockey.PatchConvey("set text config", func() {
			text := &responses.ResponseTextConfigParam{
				Verbosity: responses.ResponseTextConfigVerbosityLow,
			}
			opt := WithResponsesText(text)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.text)
			assert.Equal(t, text, opts.text)
		})

		mockey.PatchConvey("set nil text", func() {
			opt := WithResponsesText(nil)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.Nil(t, opts.text)
		})
	})
}

func TestWithResponsesMaxToolCalls(t *testing.T) {
	mockey.PatchConvey("WithResponsesMaxToolCalls", t, func() {
		mockey.PatchConvey("set positive value", func() {
			maxCalls := 5
			opt := WithResponsesMaxToolCalls(maxCalls)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.maxToolCalls)
			assert.Equal(t, maxCalls, *opts.maxToolCalls)
		})

		mockey.PatchConvey("set zero value", func() {
			opt := WithResponsesMaxToolCalls(0)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.maxToolCalls)
			assert.Equal(t, 0, *opts.maxToolCalls)
		})
	})
}

func TestWithResponsesParallelToolCalls(t *testing.T) {
	mockey.PatchConvey("WithResponsesParallelToolCalls", t, func() {
		mockey.PatchConvey("set to true", func() {
			opt := WithResponsesParallelToolCalls(true)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.parallelToolCalls)
			assert.True(t, *opts.parallelToolCalls)
		})

		mockey.PatchConvey("set to false", func() {
			opt := WithResponsesParallelToolCalls(false)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.parallelToolCalls)
			assert.False(t, *opts.parallelToolCalls)
		})
	})
}

func TestWithResponsesServerTools(t *testing.T) {
	mockey.PatchConvey("WithResponsesServerTools", t, func() {
		mockey.PatchConvey("set server tools", func() {
			tools := []*ResponsesServerToolConfig{
				{
					WebSearch: &responses.WebSearchToolParam{
						Type: responses.WebSearchToolTypeWebSearch,
					},
				},
			}
			opt := WithResponsesServerTools(tools)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.serverTools)
			assert.Len(t, opts.serverTools, 1)
			assert.Equal(t, tools, opts.serverTools)
		})

		mockey.PatchConvey("set empty tools", func() {
			opt := WithResponsesServerTools([]*ResponsesServerToolConfig{})
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.serverTools)
			assert.Len(t, opts.serverTools, 0)
		})

		mockey.PatchConvey("set nil tools", func() {
			opt := WithResponsesServerTools(nil)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.Nil(t, opts.serverTools)
		})
	})
}

func TestWithResponsesMCPTools(t *testing.T) {
	mockey.PatchConvey("WithResponsesMCPTools", t, func() {
		mockey.PatchConvey("set mcp tools", func() {
			tools := []*responses.ToolMcpParam{
				{
					ServerLabel: "test-server",
				},
			}
			opt := WithResponsesMCPTools(tools)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.mcpTools)
			assert.Len(t, opts.mcpTools, 1)
			assert.Equal(t, tools, opts.mcpTools)
		})

		mockey.PatchConvey("set empty tools", func() {
			opt := WithResponsesMCPTools([]*responses.ToolMcpParam{})
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.mcpTools)
			assert.Len(t, opts.mcpTools, 0)
		})

		mockey.PatchConvey("set nil tools", func() {
			opt := WithResponsesMCPTools(nil)
			opts := model.GetImplSpecificOptions(&options{}, opt)
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
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.customHeaders)
			assert.Equal(t, headers, opts.customHeaders)
		})

		mockey.PatchConvey("set empty headers", func() {
			opt := WithCustomHeaders(map[string]string{})
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.customHeaders)
			assert.Len(t, opts.customHeaders, 0)
		})

		mockey.PatchConvey("set nil headers", func() {
			opt := WithCustomHeaders(nil)
			opts := model.GetImplSpecificOptions(&options{}, opt)
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
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.extraFields)
			assert.Equal(t, fields, opts.extraFields)
		})

		mockey.PatchConvey("set empty fields", func() {
			opt := WithExtraFields(map[string]any{})
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.NotNil(t, opts.extraFields)
			assert.Len(t, opts.extraFields, 0)
		})

		mockey.PatchConvey("set nil fields", func() {
			opt := WithExtraFields(nil)
			opts := model.GetImplSpecificOptions(&options{}, opt)
			assert.Nil(t, opts.extraFields)
		})
	})
}
