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

package ark

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Track the original projections before modifiers. Terminal output supplies any
// fields or suffixes not already emitted, without duplicating existing deltas.
type responsesStreamState struct {
	parts          responsesPartsStream
	content        strings.Builder
	tools          map[int]*responsesStreamTool
	summaryIndex   int
	summaryStarted bool
}

type responsesStreamTool struct {
	id, name  string
	arguments strings.Builder
}

func (s *responsesStreamState) record(msg *schema.Message) error {
	s.content.WriteString(msg.Content)
	for _, call := range msg.ToolCalls {
		index := *call.Index
		if s.tools == nil {
			s.tools = make(map[int]*responsesStreamTool)
		}
		tool := s.tools[index]
		if tool == nil {
			tool = &responsesStreamTool{}
			s.tools[index] = tool
		}
		if call.ID != "" {
			if _, err := responsesStreamIdentity(call.ID, tool.id, "tool call ID"); err != nil {
				return err
			}
			tool.id = call.ID
		}
		if call.Function.Name != "" {
			if _, err := responsesStreamIdentity(call.Function.Name, tool.name, "tool name"); err != nil {
				return err
			}
			tool.name = call.Function.Name
		}
		tool.arguments.WriteString(call.Function.Arguments)
	}
	return nil
}

func (s *responsesStreamState) resolve(event *responses.ResponseStreamEvent, req *responses.ResponsesRequest) (*schema.Message, bool, error) {
	msg, terminal, err := resolveResponsesStreamEvent(event, req)
	if err != nil {
		return msg, terminal, err
	}
	parts, err := s.parts.resolve(event, msg, terminal)
	if err != nil {
		return nil, terminal, err
	}
	if msg == nil && len(parts) > 0 {
		msg = &schema.Message{Role: schema.Assistant}
	}
	if msg == nil {
		return nil, terminal, nil
	}
	msg.AssistantGenMultiContent = parts
	msg.ReasoningContent = s.summaryDelta(parts)
	if !terminal {
		if err := s.record(msg); err != nil {
			return nil, false, err
		}
		return msg, false, nil
	}
	var resp *responses.Response
	switch event.OneOf.Type {
	case responses.ResponseCompletedEventResponseStreamEventSum:
		resp = &event.OneOf.ResponseCompletedEvent.Response
	case responses.ResponseIncompleteEventResponseStreamEventSum:
		resp = &event.OneOf.ResponseIncompleteEvent.Response
	default:
		return msg, terminal, nil
	}
	msg.Content, err = responsesStreamSuffix(msg.Content, s.content.String(), "content")
	if err != nil {
		return nil, true, err
	}
	var toolDeltas []schema.ToolCall
	for index, output := range resp.Output {
		call, ok := output.OneOf.GetItemFunctionToolCall()
		if !ok {
			continue
		}
		seen := s.tools[index]
		if seen == nil {
			seen = &responsesStreamTool{}
		}
		id, err := responsesStreamIdentity(call.CallID.Or(seen.id), seen.id, "tool call ID")
		if err != nil {
			return nil, true, err
		}
		name, err := responsesStreamIdentity(call.Name.Or(seen.name), seen.name, "tool name")
		if err != nil {
			return nil, true, err
		}
		args, err := responsesStreamSuffix(call.Arguments.Or(seen.arguments.String()), seen.arguments.String(), "tool arguments")
		if err != nil {
			return nil, true, err
		}
		if id != "" || name != "" || args != "" {
			toolIndex := index
			toolDeltas = append(toolDeltas, schema.ToolCall{Index: &toolIndex, ID: id, Type: "function", Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)}, Function: schema.FunctionCall{Name: name, Arguments: args}})
		}
	}
	msg.ToolCalls = toolDeltas
	if err := s.record(msg); err != nil {
		return nil, true, err
	}

	return msg, true, nil
}

// Canonical part chunks already contain only un-emitted suffixes in order.
// Derive the display from summary chunks alone, including added/done snapshots.
func (s *responsesStreamState) summaryDelta(parts []schema.MessageOutputPart) string {
	var text strings.Builder
	for _, part := range parts {
		section, _ := extraString(part.Extra, keyResponsesSection)
		if section != "summary" || part.Reasoning == nil || part.Reasoning.Text == "" {
			continue
		}
		index := part.StreamingMeta.Index
		if s.summaryStarted && s.summaryIndex != index {
			text.WriteByte('\n')
		}
		text.WriteString(part.Reasoning.Text)
		s.summaryIndex, s.summaryStarted = index, true
	}
	return text.String()
}

// Eino merges tool IDs and names atomically; only arguments are text deltas.
func responsesStreamIdentity(full, emitted, field string) (string, error) {
	if emitted == "" {
		return full, nil
	}
	if full != emitted {
		return "", fmt.Errorf("Responses stream %s differs from terminal output", field)
	}
	return "", nil
}

func responsesStreamSuffix(full, emitted, field string) (string, error) {
	if !strings.HasPrefix(full, emitted) {
		return "", fmt.Errorf("Responses stream %s differs from terminal output", field)
	}
	return full[len(emitted):], nil
}
