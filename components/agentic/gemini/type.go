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

package gemini

import (
	"github.com/cloudwego/eino/schema"
)

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
	walkContentBlockExtra(cb, func(m map[string]any) map[string]any {
		if m == nil {
			m = make(map[string]any)
		}
		m[thoughtSignatureExtraKey] = ts
		return m
	})
}

func getThoughtSignature(cb *schema.ContentBlock) []byte {
	var ts []byte
	walkContentBlockExtra(cb, func(m map[string]any) map[string]any {
		if v, ok := m[thoughtSignatureExtraKey].([]byte); ok {
			ts = v
		}
		return m
	})
	return ts
}

func walkContentBlockExtra(cb *schema.ContentBlock, handler func(map[string]any) map[string]any) {
	if cb == nil {
		return
	}
	switch cb.Type {
	case schema.ContentBlockTypeReasoning:
		if cb.Reasoning != nil {
			cb.Reasoning.Extra = handler(cb.Reasoning.Extra)
		}
	case schema.ContentBlockTypeUserInputText:
		if cb.UserInputText != nil {
			cb.UserInputText.Extra = handler(cb.UserInputText.Extra)
		}
	case schema.ContentBlockTypeUserInputImage:
		if cb.UserInputImage != nil {
			cb.UserInputImage.Extra = handler(cb.UserInputImage.Extra)
		}
	case schema.ContentBlockTypeUserInputAudio:
		if cb.UserInputAudio != nil {
			cb.UserInputAudio.Extra = handler(cb.UserInputAudio.Extra)
		}
	case schema.ContentBlockTypeUserInputVideo:
		if cb.UserInputVideo != nil {
			cb.UserInputVideo.Extra = handler(cb.UserInputVideo.Extra)
		}
	case schema.ContentBlockTypeUserInputFile:
		if cb.UserInputFile != nil {
			cb.UserInputFile.Extra = handler(cb.UserInputFile.Extra)
		}
	case schema.ContentBlockTypeAssistantGenText:
		if cb.AssistantGenText != nil {
			cb.AssistantGenText.Extra = handler(cb.AssistantGenText.Extra)
		}
	case schema.ContentBlockTypeAssistantGenImage:
		if cb.AssistantGenImage != nil {
			cb.AssistantGenImage.Extra = handler(cb.AssistantGenImage.Extra)
		}
	case schema.ContentBlockTypeAssistantGenAudio:
		if cb.AssistantGenAudio != nil {
			cb.AssistantGenAudio.Extra = handler(cb.AssistantGenAudio.Extra)
		}
	case schema.ContentBlockTypeAssistantGenVideo:
		if cb.AssistantGenVideo != nil {
			cb.AssistantGenVideo.Extra = handler(cb.AssistantGenVideo.Extra)
		}
	case schema.ContentBlockTypeFunctionToolCall:
		if cb.FunctionToolCall != nil {
			cb.FunctionToolCall.Extra = handler(cb.FunctionToolCall.Extra)
		}
	case schema.ContentBlockTypeFunctionToolResult:
		if cb.FunctionToolResult != nil {
			cb.FunctionToolResult.Extra = handler(cb.FunctionToolResult.Extra)
		}
	case schema.ContentBlockTypeServerToolCall:
		if cb.ServerToolCall != nil {
			cb.ServerToolCall.Extra = handler(cb.ServerToolCall.Extra)
		}
	case schema.ContentBlockTypeServerToolResult:
		if cb.ServerToolResult != nil {
			cb.ServerToolResult.Extra = handler(cb.ServerToolResult.Extra)
		}
	case schema.ContentBlockTypeMCPToolCall:
		if cb.MCPToolCall != nil {
			cb.MCPToolCall.Extra = handler(cb.MCPToolCall.Extra)
		}
	case schema.ContentBlockTypeMCPToolResult:
		if cb.MCPToolResult != nil {
			cb.MCPToolResult.Extra = handler(cb.MCPToolResult.Extra)
		}
	case schema.ContentBlockTypeMCPListTools:
		if cb.MCPListToolsResult != nil {
			cb.MCPListToolsResult.Extra = handler(cb.MCPListToolsResult.Extra)
		}
	case schema.ContentBlockTypeMCPToolApprovalRequest:
		if cb.MCPToolApprovalRequest != nil {
			cb.MCPToolApprovalRequest.Extra = handler(cb.MCPToolApprovalRequest.Extra)
		}
	case schema.ContentBlockTypeMCPToolApprovalResponse:
		if cb.MCPToolApprovalResponse != nil {
			cb.MCPToolApprovalResponse.Extra = handler(cb.MCPToolApprovalResponse.Extra)
		}
	default:
		return
	}
}
