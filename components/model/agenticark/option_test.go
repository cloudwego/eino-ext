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

package agenticark

import (
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

func TestWithReasoning(t *testing.T) {
	base := &arkOptions{}
	reasoning := &responses.ResponsesReasoning{}
	got := model.GetImplSpecificOptions(base, WithReasoning(reasoning))
	assert.Same(t, reasoning, got.reasoning)
}

func TestWithThinking(t *testing.T) {
	base := &arkOptions{}
	thinking := &responses.ResponsesThinking{}
	got := model.GetImplSpecificOptions(base, WithThinking(thinking))
	assert.Same(t, thinking, got.thinking)
}

func TestWithText(t *testing.T) {
	base := &arkOptions{}
	text := &responses.ResponsesText{}
	got := model.GetImplSpecificOptions(base, WithText(text))
	assert.Same(t, text, got.text)
}

func TestWithMaxToolCalls(t *testing.T) {
	base := &arkOptions{}
	got := model.GetImplSpecificOptions(base, WithMaxToolCalls(10))
	if assert.NotNil(t, got.maxToolCalls) {
		assert.Equal(t, int64(10), *got.maxToolCalls)
	}
}

func TestWithParallelToolCalls(t *testing.T) {
	base := &arkOptions{}
	got := model.GetImplSpecificOptions(base, WithParallelToolCalls(true))
	if assert.NotNil(t, got.parallelToolCalls) {
		assert.True(t, *got.parallelToolCalls)
	}
}

func TestWithServerTools(t *testing.T) {
	base := &arkOptions{}
	serverTools := []*ServerToolConfig{
		{WebSearch: &responses.ToolWebSearch{}},
	}
	got := model.GetImplSpecificOptions(base, WithServerTools(serverTools))
	assert.Equal(t, serverTools, got.serverTools)
}

func TestWithMCPTools(t *testing.T) {
	base := &arkOptions{}
	mcpTools := []*responses.ToolMcp{
		{Type: responses.ToolType_mcp},
	}
	got := model.GetImplSpecificOptions(base, WithMCPTools(mcpTools))
	assert.Equal(t, mcpTools, got.mcpTools)
}

func TestWithCustomHeaders(t *testing.T) {
	base := &arkOptions{}
	headers := map[string]string{"K": "V"}
	got := model.GetImplSpecificOptions(base, WithCustomHeaders(headers))
	assert.Equal(t, headers, got.customHeaders)
}

func TestWithCache(t *testing.T) {
	base := &arkOptions{}
	cache := &CacheOption{}
	got := model.GetImplSpecificOptions(base, WithCache(cache))
	assert.Same(t, cache, got.cache)
}
