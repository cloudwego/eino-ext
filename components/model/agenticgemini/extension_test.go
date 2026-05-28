/*
 * Copyright 2026 CloudWeGo Authors
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

package agenticgemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genai"

	"github.com/cloudwego/eino/schema"
)

func TestGetServerToolCallArguments(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		args, err := getServerToolCallArguments(nil)
		assert.Nil(t, args)
		assert.EqualError(t, err, "server tool call arguments are nil")
	})

	t.Run("typed", func(t *testing.T) {
		expected := &ServerToolCallArguments{
			ExecutableCode: &ExecutableCode{
				Code:     "print('hello')",
				Language: LanguagePython,
			},
		}

		args, err := getServerToolCallArguments(&schema.ServerToolCall{Arguments: expected})
		assert.NoError(t, err)
		assert.Same(t, expected, args)
	})

	t.Run("map", func(t *testing.T) {
		args, err := getServerToolCallArguments(&schema.ServerToolCall{
			Arguments: map[string]any{
				"executable_code": map[string]any{
					"code":     "print('hello')",
					"language": string(LanguagePython),
				},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, &ServerToolCallArguments{
			ExecutableCode: &ExecutableCode{
				Code:     "print('hello')",
				Language: LanguagePython,
			},
		}, args)
	})

	t.Run("unexpected type", func(t *testing.T) {
		args, err := getServerToolCallArguments(&schema.ServerToolCall{Arguments: "bad"})
		assert.Nil(t, args)
		assert.EqualError(t, err, "unexpected type string for server tool call arguments")
	})
}

func TestGetServerToolCallResult(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result, err := getServerToolCallResult(nil)
		assert.Nil(t, result)
		assert.EqualError(t, err, "server tool result is nil")
	})

	t.Run("typed", func(t *testing.T) {
		expected := &ServerToolCallResult{
			CodeExecutionResult: &CodeExecutionResult{
				Outcome: OutcomeOK,
				Output:  "done",
			},
		}

		result, err := getServerToolCallResult(&schema.ServerToolResult{Content: expected})
		assert.NoError(t, err)
		assert.Same(t, expected, result)
	})

	t.Run("map", func(t *testing.T) {
		result, err := getServerToolCallResult(&schema.ServerToolResult{
			Content: map[string]any{
				"code_execution_result": map[string]any{
					"outcome": string(OutcomeOK),
					"output":  "done",
				},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, &ServerToolCallResult{
			CodeExecutionResult: &CodeExecutionResult{
				Outcome: OutcomeOK,
				Output:  "done",
			},
		}, result)
	})

	t.Run("unexpected type", func(t *testing.T) {
		result, err := getServerToolCallResult(&schema.ServerToolResult{Content: 1})
		assert.Nil(t, result)
		assert.EqualError(t, err, "unexpected type int for server tool result")
	})
}

func TestConcatServerToolCallArguments(t *testing.T) {
	args, err := concatServerToolCallArguments([]*ServerToolCallArguments{
		{ExecutableCode: &ExecutableCode{Code: "print(", Language: LanguageUnspecified}},
		{ExecutableCode: &ExecutableCode{Code: "'hello')", Language: LanguagePython}},
		{},
	})
	assert.NoError(t, err)
	assert.Equal(t, &ServerToolCallArguments{
		ExecutableCode: &ExecutableCode{
			Code:     "print('hello')",
			Language: LanguagePython,
		},
	}, args)
}

func TestConcatServerToolCallResult(t *testing.T) {
	result, err := concatServerToolCallResult([]*ServerToolCallResult{
		{CodeExecutionResult: &CodeExecutionResult{Output: "hello", Outcome: OutcomeUnspecified}},
		{CodeExecutionResult: &CodeExecutionResult{Output: " world", Outcome: OutcomeOK}},
		{},
	})
	assert.NoError(t, err)
	assert.Equal(t, &ServerToolCallResult{
		CodeExecutionResult: &CodeExecutionResult{
			Outcome: OutcomeOK,
			Output:  "hello world",
		},
	}, result)
}

func TestConcatExecutableCode(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result, err := concatExecutableCode(nil)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("skip nil chunks", func(t *testing.T) {
		result, err := concatExecutableCode([]*ExecutableCode{
			nil,
			{Code: "a"},
			{Language: LanguagePython},
			{Code: "b"},
		})
		assert.NoError(t, err)
		assert.Equal(t, &ExecutableCode{
			Code:     "ab",
			Language: LanguagePython,
		}, result)
	})
}

func TestConcatCodeExecutionResult(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		result, err := concatCodeExecutionResult(nil)
		assert.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("skip nil chunks", func(t *testing.T) {
		result, err := concatCodeExecutionResult([]*CodeExecutionResult{
			nil,
			{Output: "part1"},
			{Outcome: OutcomeFailed},
			{Output: "part2"},
		})
		assert.NoError(t, err)
		assert.Equal(t, &CodeExecutionResult{
			Outcome: OutcomeFailed,
			Output:  "part1part2",
		}, result)
	})
}

func TestToServerTools(t *testing.T) {
	tools, err := toServerTools([]*ServerToolConfig{
		{CodeExecution: &genai.ToolCodeExecution{}},
		{GoogleSearch: &genai.GoogleSearch{}},
		{GoogleSearchRetrieval: &genai.GoogleSearchRetrieval{}},
		{URLContext: &genai.URLContext{}},
		{FileSearch: &genai.FileSearch{}},
		{GoogleMaps: &genai.GoogleMaps{}},
	})
	assert.NoError(t, err)
	assert.Len(t, tools, 6)
	assert.NotNil(t, tools[0].CodeExecution)
	assert.NotNil(t, tools[1].GoogleSearch)
	assert.NotNil(t, tools[2].GoogleSearchRetrieval)
	assert.NotNil(t, tools[3].URLContext)
	assert.NotNil(t, tools[4].FileSearch)
	assert.NotNil(t, tools[5].GoogleMaps)

	tools, err = toServerTools([]*ServerToolConfig{nil})
	assert.Nil(t, tools)
	assert.EqualError(t, err, "unknown server tool type")

	tools, err = toServerTools([]*ServerToolConfig{{}})
	assert.Nil(t, tools)
	assert.EqualError(t, err, "unknown server tool type")
}

func TestPanicErr(t *testing.T) {
	err := newPanicErr("boom", []byte("stacktrace"))
	assert.EqualError(t, err, "panic error: boom, \nstack: stacktrace")
}
