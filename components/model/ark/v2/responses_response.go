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

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func responsesMetadata(resp *responses.Response, req *responses.ResponsesRequest) (*schema.Message, error) {
	if resp == nil {
		return nil, ErrEmptyResponse
	}
	if resp.Status.Or("") == responses.ResponseStatusFailed || resp.Error.IsSet() {
		problem := resp.Error.Or(responses.ResponseError{})
		return nil, fmt.Errorf("Ark response failed (%s): %s", problem.Code, problem.Message)
	}
	msg := &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{FinishReason: string(resp.Status.Or(""))}, Extra: map[string]any{}}
	if d, ok := resp.IncompleteDetails.Get(); ok {
		msg.ResponseMeta.FinishReason = d.Reason
	}
	if resp.ID != "" {
		msg.Extra[keyResponseID] = metadataString(resp.ID)
	}
	if resp.Model != "" {
		msg.Extra[keyModelName] = metadataString(resp.Model)
	}
	if tier, ok := resp.ServiceTier.Get(); ok {
		msg.Extra[keyServiceTier] = metadataString(tier)
	}
	if req.Store.Or(false) {
		if expires := resp.ExpireAt.Or(req.ExpireAt.Or(0)); expires > 0 {
			msg.Extra[keyCacheExpiration] = metadataInt64(expires)
		}
	}
	if usage, ok := resp.Usage.Get(); ok {
		msg.ResponseMeta.Usage = &schema.TokenUsage{PromptTokens: int(usage.InputTokens), CompletionTokens: int(usage.OutputTokens), TotalTokens: int(usage.TotalTokens),
			PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: int(usage.InputTokensDetails.CachedTokens)},
			CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: int(usage.OutputTokensDetails.ReasoningTokens)}}
		if err := setResponsesUsage(msg, usage); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

func resolveResponsesResponse(resp *responses.Response, req *responses.ResponsesRequest, raw []byte) (*schema.Message, error) {
	msg, err := responsesMetadata(resp, req)
	if err != nil {
		return nil, err
	}
	if err := validateResponsesOutput(resp, req); err != nil {
		return nil, err
	}
	projectResponsesOutput(msg, resp.Output)
	if err := setResponsesFinalOutput(msg, resp.Output, raw); err != nil {
		return nil, err
	}
	return msg, nil
}

func validateResponsesOutput(resp *responses.Response, req *responses.ResponsesRequest) error {
	if len(resp.Output) == 0 && resp.Status.Or("") != responses.ResponseStatusIncomplete && !req.Caching.Or(responses.ResponseCaching{}).Prefix.Or(false) {
		return ErrEmptyResponse
	}
	return nil
}

func projectResponsesOutput(msg *schema.Message, outputs []responses.OutputItem) {
	var content strings.Builder
	content.WriteString(msg.Content)
	var reasoning []responses.ItemReasoning
	for _, output := range outputs {
		switch output.OneOf.Type {
		case responses.ItemOutputMessageOutputItemSum:
			for _, part := range output.OneOf.ItemOutputMessage.Content {
				if text, ok := part.OneOf.GetOutputContentItemText(); ok {
					content.WriteString(text.Text.Or(""))
				}
			}
		case responses.ItemFunctionToolCallOutputItemSum:
			call := output.OneOf.ItemFunctionToolCall
			msg.ToolCalls = append(msg.ToolCalls, schema.ToolCall{ID: call.CallID.Or(""), Type: "function", Function: schema.FunctionCall{Name: call.Name.Or(""), Arguments: call.Arguments.Or("")}})
		case responses.ItemReasoningOutputItemSum:
			item := output.OneOf.ItemReasoning
			reasoning = append(reasoning, item)
		}
	}
	msg.Content = content.String()
	msg.ReasoningContent = responsesReasoningText(reasoning)
}

// Capture each original SSE payload while retaining the generated SDK decoder.
type responsesPayloadDecoder struct {
	raw    []byte
	retain bool
}

func (d *responsesPayloadDecoder) Unmarshal(raw []byte, value interface{}) error {
	if d.retain {
		// Modifiers may retain the original frame in application metadata.
		d.raw = append([]byte(nil), raw...)
	} else {
		d.raw = append(d.raw[:0], raw...)
	}
	return sonic.Unmarshal(raw, value)
}

// Metadata is emitted once at the terminal event, so token usage and IDs do
// not get duplicated when Eino concatenates stream chunks.
func resolveResponsesStreamEvent(event *responses.ResponseStreamEvent, req *responses.ResponsesRequest, raw []byte) (msg *schema.Message, terminal bool, err error) {
	if event == nil {
		return nil, false, nil
	}
	ev := &event.OneOf
	switch ev.Type {
	case responses.ResponseCompletedEventResponseStreamEventSum:
		msg, err = responsesMetadata(&ev.ResponseCompletedEvent.Response, req)
		if err == nil {
			err = setResponsesFinalOutput(msg, ev.ResponseCompletedEvent.Response.Output, raw)
		}
		return msg, true, err
	case responses.ResponseIncompleteEventResponseStreamEventSum:
		msg, err = responsesMetadata(&ev.ResponseIncompleteEvent.Response, req)
		if err == nil {
			err = setResponsesFinalOutput(msg, ev.ResponseIncompleteEvent.Response.Output, raw)
		}
		return msg, true, err
	case responses.ResponseFailedEventResponseStreamEventSum:
		resp := ev.ResponseFailedEvent.Response
		resp.Status = responses.NewOptResponseStatus(responses.ResponseStatusFailed)
		msg, err = responsesMetadata(&resp, req)
		return msg, true, err
	case responses.ResponseErrorEventResponseStreamEventSum:
		return nil, false, fmt.Errorf("Ark stream error (%s): %s", ev.ResponseErrorEvent.Code.Or(""), ev.ResponseErrorEvent.Message)
	case responses.ResponseTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, Content: ev.ResponseTextDeltaEvent.Delta.Or("")}
	case responses.ResponseReasoningTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, ReasoningContent: ev.ResponseReasoningTextDeltaEvent.Delta.Or("")}
	case responses.ResponseReasoningRawTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, ReasoningContent: ev.ResponseReasoningRawTextDeltaEvent.Delta.Or("")}
	case responses.ResponseReasoningSummaryTextDeltaEventResponseStreamEventSum:
		msg = &schema.Message{Role: schema.Assistant, ReasoningContent: ev.ResponseReasoningSummaryTextDeltaEvent.Delta.Or("")}
	case responses.ResponseOutputItemAddedEventResponseStreamEventSum:
		added := ev.ResponseOutputItemAddedEvent
		if call, ok := added.Item.OneOf.GetItemFunctionToolCall(); ok {
			index := int(added.OutputIndex)
			msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, ID: call.CallID.Or(""), Type: "function",
				Function: schema.FunctionCall{Name: call.Name.Or(""), Arguments: call.Arguments.Or("")}}}}
		}
	case responses.ResponseFunctionCallArgumentsDeltaEventResponseStreamEventSum:
		delta := ev.ResponseFunctionCallArgumentsDeltaEvent
		index := int(delta.OutputIndex)
		msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, Function: schema.FunctionCall{Arguments: delta.Delta.Or("")}}}}
	}
	return msg, false, nil
}

func setResponsesFinalOutput(msg *schema.Message, output []responses.OutputItem, raw []byte) error {
	if len(raw) > 0 {
		return retainResponsesOutput(msg, raw)
	}
	return setJSONExtra(msg, keyResponsesOutput, output)
}

func responsesReasoningText(items []responses.ItemReasoning) string {
	var summary, raw []string
	for _, item := range items {
		for _, part := range item.Summary {
			if text := part.Text.Or(""); text != "" {
				summary = append(summary, text)
			}
		}
		for _, part := range item.Content {
			if text, ok := part.OneOf.GetOutputContentItemReasoningText(); ok {
				raw = append(raw, text.Text.Or(""))
			}
		}
	}
	if len(raw) > 0 {
		return strings.Join(raw, "")
	}
	return strings.Join(summary, "\n\n")
}
