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
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"

	"github.com/cloudwego/eino/schema"
)

type ServerToolCallArguments struct {
	ExecutableCode *ExecutableCode `json:"executable_code,omitempty" mapstructure:"executable_code,omitempty"`
}

type ServerToolCallResult struct {
	CodeExecutionResult *CodeExecutionResult `json:"code_execution_result,omitempty" mapstructure:"code_execution_result,omitempty"`
}

type ExecutableCode struct {
	Code     string   `json:"code,omitempty" mapstructure:"code,omitempty"`
	Language Language `json:"language,omitempty" mapstructure:"language,omitempty"`
}

type CodeExecutionResult struct {
	Outcome Outcome `json:"outcome,omitempty" mapstructure:"outcome,omitempty"`
	Output  string  `json:"output,omitempty" mapstructure:"output,omitempty"`
}

func getServerToolCallArguments(call *schema.ServerToolCall) (*ServerToolCallArguments, error) {
	if call == nil || call.Arguments == nil {
		return nil, fmt.Errorf("server tool call arguments are nil")
	}
	if args, ok := call.Arguments.(*ServerToolCallArguments); ok {
		return args, nil
	}
	if m, ok := call.Arguments.(map[string]any); ok {
		args := &ServerToolCallArguments{}
		if err := mapstructure.Decode(m, args); err != nil {
			return nil, fmt.Errorf("failed to decode server tool call arguments: %w", err)
		}
		return args, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool call arguments", call.Arguments)
}

func getServerToolCallResult(res *schema.ServerToolResult) (*ServerToolCallResult, error) {
	if res == nil || res.Content == nil {
		return nil, fmt.Errorf("server tool result is nil")
	}
	if result, ok := res.Content.(*ServerToolCallResult); ok {
		return result, nil
	}
	if m, ok := res.Content.(map[string]any); ok {
		result := &ServerToolCallResult{}
		if err := mapstructure.Decode(m, result); err != nil {
			return nil, fmt.Errorf("failed to decode server tool result: %w", err)
		}
		return result, nil
	}
	return nil, fmt.Errorf("unexpected type %T for server tool result", res.Content)
}

func concatServerToolCallArguments(ts []*ServerToolCallArguments) (*ServerToolCallArguments, error) {
	var executableCodes []*ExecutableCode
	for _, t := range ts {
		if t.ExecutableCode != nil {
			executableCodes = append(executableCodes, t.ExecutableCode)
		}
	}

	ec, err := concatExecutableCode(executableCodes)
	if err != nil {
		return nil, err
	}

	return &ServerToolCallArguments{
		ExecutableCode: ec,
	}, nil
}

func concatServerToolCallResult(ts []*ServerToolCallResult) (*ServerToolCallResult, error) {
	var codeExecutionResults []*CodeExecutionResult
	for _, t := range ts {
		if t.CodeExecutionResult != nil {
			codeExecutionResults = append(codeExecutionResults, t.CodeExecutionResult)
		}
	}

	ce, err := concatCodeExecutionResult(codeExecutionResults)
	if err != nil {
		return nil, err
	}

	return &ServerToolCallResult{
		CodeExecutionResult: ce,
	}, nil
}

func concatExecutableCode(chunks []*ExecutableCode) (final *ExecutableCode, err error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	var lang Language
	code := &strings.Builder{}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if len(chunk.Language) > 0 {
			lang = chunk.Language
		}
		if len(chunk.Code) > 0 {
			code.WriteString(chunk.Code)
		}
	}
	return &ExecutableCode{
		Code:     code.String(),
		Language: lang,
	}, nil
}

func concatCodeExecutionResult(chunks []*CodeExecutionResult) (final *CodeExecutionResult, err error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	var outcome Outcome
	output := &strings.Builder{}
	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if len(chunk.Outcome) > 0 {
			outcome = chunk.Outcome
		}
		if len(chunk.Output) > 0 {
			output.WriteString(chunk.Output)
		}
	}
	return &CodeExecutionResult{
		Outcome: outcome,
		Output:  output.String(),
	}, nil
}
