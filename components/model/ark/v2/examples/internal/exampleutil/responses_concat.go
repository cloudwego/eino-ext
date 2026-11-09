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

package exampleutil

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
)

// PrintResponsesConcat checks the standard, unmodified Responses stream and
// prints its assembled structure. Encrypted signatures are abbreviated only in
// the printed copy; the original message remains available for the next turn.
func PrintResponsesConcat(chunks []*schema.Message, merged *schema.Message) error {
	if len(chunks) == 0 || merged == nil || merged.Role != schema.Assistant {
		return fmt.Errorf("stream did not produce an assistant message")
	}
	terminal := chunks[len(chunks)-1]
	if terminal == nil || terminal.ResponseMeta == nil {
		return fmt.Errorf("stream has no terminal response metadata")
	}
	if !reflect.DeepEqual(terminal.ResponseMeta, merged.ResponseMeta) || !reflect.DeepEqual(terminal.Extra, merged.Extra) {
		return fmt.Errorf("concat lost or duplicated terminal metadata or usage")
	}
	var text strings.Builder
	functionItems := 0
	for index, part := range merged.AssistantGenMultiContent {
		if part.StreamingMeta == nil || part.StreamingMeta.Index != index {
			return fmt.Errorf("part %d has an unexpected stream merge index", index)
		}
		if part.Type == schema.ChatMessagePartTypeText && fmt.Sprint(part.Extra["ark-responses-content-type"]) == "output_text" {
			text.WriteString(part.Text)
		}
		if fmt.Sprint(part.Extra["ark-responses-item-type"]) == "function_call" {
			functionItems++
		}
	}
	if text.String() != merged.Content {
		return fmt.Errorf("concatenated text differs from the complete output parts")
	}
	if functionItems != len(merged.ToolCalls) {
		return fmt.Errorf("expected %d function calls, got %d after concat", functionItems, len(merged.ToolCalls))
	}
	seen := make(map[string]bool)
	for _, call := range merged.ToolCalls {
		if call.Type != "function" || call.ID == "" || call.Function.Name == "" || seen[call.ID] {
			return fmt.Errorf("incomplete or duplicate function call after concat")
		}
		seen[call.ID] = true
		if merged.ResponseMeta.FinishReason == "completed" && !sonic.ValidString(call.Function.Arguments) {
			return fmt.Errorf("function %s has incomplete JSON arguments", call.Function.Name)
		}
	}

	incremental := 0
	for _, chunk := range chunks[:len(chunks)-1] {
		if (chunk.Content != "" || chunk.ReasoningContent != "") && len(chunk.AssistantGenMultiContent) > 0 {
			incremental++
		}
	}
	if (merged.Content != "" || merged.ReasoningContent != "") && incremental == 0 {
		return fmt.Errorf("streamed text was not exposed through incremental output parts")
	}
	fmt.Printf("\nConcat structure OK: chunks=%d parts=%d tools=%d incremental_part_chunks=%d\n", len(chunks), len(merged.AssistantGenMultiContent), len(merged.ToolCalls), incremental)
	display := *merged
	display.AssistantGenMultiContent = append([]schema.MessageOutputPart(nil), merged.AssistantGenMultiContent...)
	for index := range display.AssistantGenMultiContent {
		part := &display.AssistantGenMultiContent[index]
		fmt.Printf("Part %d: type=%s stream_index=%d\n", index, part.Type, part.StreamingMeta.Index)
		if part.Reasoning != nil {
			reasoning := *part.Reasoning
			if reasoning.Signature != "" {
				reasoning.Signature = fmt.Sprintf("<%d encrypted bytes>", len(reasoning.Signature))
			}
			part.Reasoning = &reasoning
		}
	}
	raw, err := sonic.MarshalIndent(&display, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("Concatenated message (signatures abbreviated):\n%s\n", raw)
	return nil
}
