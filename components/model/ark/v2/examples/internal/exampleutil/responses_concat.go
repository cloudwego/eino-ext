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
	"strconv"
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
	for index, part := range merged.AssistantGenMultiContent {
		if part.StreamingMeta == nil || part.StreamingMeta.Index != index {
			return fmt.Errorf("part %d has an unexpected stream merge index", index)
		}
		if part.Type == schema.ChatMessagePartTypeText && fmt.Sprint(part.Extra["ark-responses-content-type"]) == "output_text" {
			text.WriteString(part.Text)
		}
		if fmt.Sprint(part.Extra["ark-responses-item-type"]) == "function_call" {
			return fmt.Errorf("function call metadata belongs in ToolCall.Extra")
		}
	}
	if text.String() != merged.Content {
		return fmt.Errorf("concatenated text differs from the complete output parts")
	}
	seen := make(map[string]bool)
	items := make(map[int]bool)
	for index, call := range merged.ToolCalls {
		if call.Type != "function" || call.ID == "" || call.Function.Name == "" || seen[call.ID] {
			return fmt.Errorf("incomplete or duplicate function call after concat")
		}
		if call.Index == nil || *call.Index != index ||
			fmt.Sprint(call.Extra["ark-responses-item-type"]) != "function_call" {
			return fmt.Errorf("function call lost its Responses item metadata after concat")
		}
		item, err := strconv.Atoi(fmt.Sprint(call.Extra["ark-responses-item-index"]))
		if err != nil || item < 0 || items[item] {
			return fmt.Errorf("function call has an invalid or duplicate Responses item index")
		}
		items[item] = true
		seen[call.ID] = true
		if merged.ResponseMeta.FinishReason == "completed" && !sonic.ValidString(call.Function.Arguments) {
			return fmt.Errorf("function %s has incomplete JSON arguments", call.Function.Name)
		}
	}

	incremental := 0
	for _, chunk := range chunks[:len(chunks)-1] {
		for _, part := range chunk.AssistantGenMultiContent {
			if part.Text != "" || (part.Reasoning != nil && part.Reasoning.Text != "") {
				incremental++
				break
			}
		}
	}
	if (merged.Content != "" || merged.ReasoningContent != "") && incremental == 0 {
		return fmt.Errorf("streamed text was not exposed through incremental output parts")
	}
	fmt.Printf("\nConcat structure OK: chunks=%d parts=%d tools=%d incremental_part_chunks=%d\n", len(chunks), len(merged.AssistantGenMultiContent), len(merged.ToolCalls), incremental)
	display := responsesMessageForDisplay(merged)
	for index, part := range merged.AssistantGenMultiContent {
		fmt.Printf("Part %d: type=%s stream_index=%d\n", index, part.Type, part.StreamingMeta.Index)
	}
	raw, err := sonic.MarshalIndent(&display, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("Concatenated message (signatures abbreviated):\n%s\n", raw)
	return nil
}

// PrintResponsesChunk prints one public Recv result and its JSON-excluded indices.
func PrintResponsesChunk(sequence int, chunk *schema.Message) error {
	display := responsesMessageForDisplay(chunk)
	raw, err := sonic.MarshalIndent(&display, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("\nChunk %d (signatures abbreviated):\n%s\n", sequence, raw)
	for index, part := range chunk.AssistantGenMultiContent {
		if part.StreamingMeta != nil {
			fmt.Printf("Chunk %d part %d: item=%v section=%v summary=%v content=%v stream_index=%d\n", sequence, index,
				part.Extra["ark-responses-item-index"], part.Extra["ark-responses-section"], part.Extra["ark-responses-summary-index"], part.Extra["ark-responses-content-index"], part.StreamingMeta.Index)
		}
	}
	for _, call := range chunk.ToolCalls {
		if call.Index != nil {
			fmt.Printf("Chunk %d tool: item=%v tool_index=%d\n", sequence, call.Extra["ark-responses-item-index"], *call.Index)
		}
	}
	return nil
}

func responsesMessageForDisplay(message *schema.Message) schema.Message {
	display := *message
	display.AssistantGenMultiContent = append([]schema.MessageOutputPart(nil), message.AssistantGenMultiContent...)
	for i := range display.AssistantGenMultiContent {
		part := &display.AssistantGenMultiContent[i]
		if part.Reasoning != nil {
			reasoning := *part.Reasoning
			if reasoning.Signature != "" {
				reasoning.Signature = fmt.Sprintf("<%d encrypted bytes>", len(reasoning.Signature))
			}
			part.Reasoning = &reasoning
		}
	}
	return display
}
