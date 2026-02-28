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
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func init() {
	compose.RegisterStreamChunkConcatFunc(func(ts []*ServerToolCallArguments) (*ServerToolCallArguments, error) {
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
	})

	compose.RegisterStreamChunkConcatFunc(func(ts []*ServerToolCallResult) (*ServerToolCallResult, error) {
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
	})
}

type ServerToolCallArguments struct {
	ExecutableCode *ExecutableCode
}

type ServerToolCallResult struct {
	CodeExecutionResult *CodeExecutionResult
}

type Outcome string

const (
	// OutcomeUnspecified specifies that unspecified status. This value should not be used.
	OutcomeUnspecified Outcome = "OUTCOME_UNSPECIFIED"
	// OutcomeOK specifies that code execution completed successfully.
	OutcomeOK Outcome = "OUTCOME_OK"
	// OutcomeFailed specifies that code execution finished but with a failure. `stderr` should contain the reason.
	OutcomeFailed Outcome = "OUTCOME_FAILED"
	// OutcomeDeadlineExceeded specifies that code execution ran for too long, and was cancelled. There may or may not be a partial
	// output present.
	OutcomeDeadlineExceeded Outcome = "OUTCOME_DEADLINE_EXCEEDED"
)

type Language string

const (
	LanguageUnspecified Language = "LANGUAGE_UNSPECIFIED"
	LanguagePython      Language = "PYTHON"
)

type ExecutableCode struct {
	Code     string
	Language Language
}

type CodeExecutionResult struct {
	Outcome Outcome
	Output  string
}

const (
	ServerToolNameCodeExecution = "CodeExecution"

	thoughtSignatureExtraKey = "_eino_ext_agentic_gemini_thought_signature"
)

func setThoughtSignature(cb *schema.ContentBlock, ts []byte) {
	if cb == nil {
		return
	}
	if cb.Extra == nil {
		cb.Extra = make(map[string]interface{})
	}
	cb.Extra[thoughtSignatureExtraKey] = ts
}

func getThoughtSignature(cb *schema.ContentBlock) []byte {
	if cb == nil {
		return nil
	}
	if cb.Extra == nil {
		cb.Extra = make(map[string]interface{})
	}
	if v, ok := cb.Extra[thoughtSignatureExtraKey].([]byte); ok {
		return v
	}
	return nil
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
