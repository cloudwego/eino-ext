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

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Metadata is emitted once at the terminal event, so token usage and IDs do
// not get duplicated when Eino concatenates stream chunks.
func projectResponsesStreamEvent(event *responses.ResponseStreamEvent, req *responses.ResponsesRequest) (msg *schema.Message, terminal bool, err error) {
	if event == nil {
		return nil, false, nil
	}
	ev := &event.OneOf
	switch ev.Type {
	case responses.ResponseCompletedEventResponseStreamEventSum:
		msg, err = projectResponsesResponse(&ev.ResponseCompletedEvent.Response, req)
		return msg, true, err
	case responses.ResponseIncompleteEventResponseStreamEventSum:
		msg, err = projectResponsesResponse(&ev.ResponseIncompleteEvent.Response, req)
		return msg, true, err
	case responses.ResponseFailedEventResponseStreamEventSum:
		resp := ev.ResponseFailedEvent.Response
		resp.Status = responses.NewOptResponseStatus(responses.ResponseStatusFailed)
		msg, err = projectResponsesMetadata(&resp, req)
		return msg, true, err
	case responses.ResponseErrorEventResponseStreamEventSum:
		return nil, false, fmt.Errorf("Ark stream error (%s): %s", ev.ResponseErrorEvent.Code.Or(""), ev.ResponseErrorEvent.Message)
	case responses.ResponseTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, Content: ev.ResponseTextDeltaEvent.Delta.Or("")}
		msg.AssistantGenMultiContent = []schema.MessageOutputPart{projectResponsesTextPart(int(ev.ResponseTextDeltaEvent.OutputIndex), int(ev.ResponseTextDeltaEvent.ContentIndex), msg.Content)}
	case responses.ResponseReasoningTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
			projectResponsesReasoningPart(int(ev.ResponseReasoningTextDeltaEvent.OutputIndex), int(ev.ResponseReasoningTextDeltaEvent.ContentIndex), ev.ResponseReasoningTextDeltaEvent.Delta.Or("")),
		}}
	case responses.ResponseReasoningRawTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{
			projectResponsesReasoningPart(int(ev.ResponseReasoningRawTextDeltaEvent.OutputIndex), int(ev.ResponseReasoningRawTextDeltaEvent.ContentIndex), ev.ResponseReasoningRawTextDeltaEvent.Delta.Or("")),
		}}
	case responses.ResponseReasoningSummaryTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, ReasoningContent: ev.ResponseReasoningSummaryTextDeltaEvent.Delta.Or("")}
		msg.AssistantGenMultiContent = []schema.MessageOutputPart{projectResponsesSummaryPart(int(ev.ResponseReasoningSummaryTextDeltaEvent.OutputIndex), int(ev.ResponseReasoningSummaryTextDeltaEvent.SummaryIndex), msg.ReasoningContent)}
	case responses.ResponseContentPartAddedEventResponseStreamEventSum:
		e := ev.ResponseContentPartAddedEvent
		msg, err = projectResponsesStreamContentPart(e.Part, int(e.OutputIndex), int(e.ContentIndex))
	case responses.ResponseContentPartDoneEventResponseStreamEventSum:
		e := ev.ResponseContentPartDoneEvent
		msg, err = projectResponsesStreamContentPart(e.Part, int(e.OutputIndex), int(e.ContentIndex))
	case responses.ResponseReasoningSummaryPartAddedEventResponseStreamEventSum:
		e := ev.ResponseReasoningSummaryPartAddedEvent
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{projectResponsesSummaryPart(int(e.OutputIndex), int(e.SummaryIndex), e.Part.Text.Or(""))}}
	case responses.ResponseReasoningSummaryPartDoneEventResponseStreamEventSum:
		e := ev.ResponseReasoningSummaryPartDoneEvent
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{projectResponsesSummaryPart(int(e.OutputIndex), int(e.SummaryIndex), e.Part.Text.Or(""))}}
	case responses.ResponseOutputItemDoneEventResponseStreamEventSum:
		parts, e := projectResponsesItem(ev.ResponseOutputItemDoneEvent.Item, int(ev.ResponseOutputItemDoneEvent.OutputIndex))
		if e != nil {
			return nil, false, e
		}
		msg = &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: parts}
	case responses.ResponseOutputItemAddedEventResponseStreamEventSum:
		added := ev.ResponseOutputItemAddedEvent
		if call, ok := added.Item.OneOf.GetItemFunctionToolCall(); ok {
			index := int(added.OutputIndex)
			msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, ID: call.CallID.Or(""), Type: "function", Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)},
				Function: schema.FunctionCall{Name: call.Name.Or(""), Arguments: call.Arguments.Or("")}}}}
		} else if added.Item.OneOf.Type == responses.ItemOutputMessageOutputItemSum || added.Item.OneOf.Type == responses.ItemReasoningOutputItemSum {
			parts, e := projectResponsesItem(added.Item, int(added.OutputIndex))
			if e != nil {
				return nil, false, e
			}
			msg = &schema.Message{Role: schema.Assistant}
			for _, part := range parts {
				if section, ok := extraString(part.Extra, keyResponsesSection); ok && section != "" {
					msg.AssistantGenMultiContent = append(msg.AssistantGenMultiContent, part)
				}
			}
		}
	case responses.ResponseFunctionCallArgumentsDeltaEventResponseStreamEventSum:
		delta := ev.ResponseFunctionCallArgumentsDeltaEvent
		index := int(delta.OutputIndex)
		msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)}, Function: schema.FunctionCall{Arguments: delta.Delta.Or("")}}}}
	}
	return msg, false, err
}

func projectResponsesStreamContentPart(part responses.OutputContentItem, itemIndex, contentIndex int) (*schema.Message, error) {
	projected, err := projectResponsesOutputContentPart(part, itemIndex, contentIndex)
	if err != nil {
		return nil, err
	}
	return &schema.Message{Role: schema.Assistant, AssistantGenMultiContent: []schema.MessageOutputPart{projected}}, nil
}

// Capture original SSE payloads only for an explicitly configured modifier.
type responsesPayloadDecoder struct {
	raw    []byte
	retain bool
}

func (d *responsesPayloadDecoder) Unmarshal(raw []byte, value interface{}) error {
	if d.retain {
		// Modifiers may retain the original frame in application metadata.
		d.raw = append([]byte(nil), raw...)
	}
	return sonic.Unmarshal(raw, value)
}
