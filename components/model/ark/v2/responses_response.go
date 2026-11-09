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

func resolveResponsesResponse(resp *responses.Response, req *responses.ResponsesRequest) (*schema.Message, error) {
	msg, err := responsesMetadata(resp, req)
	if err != nil {
		return nil, err
	}
	if err := validateResponsesOutput(resp, req); err != nil {
		return nil, err
	}
	if err := projectResponsesOutput(msg, resp.Output); err != nil {
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

// Metadata is emitted once at the terminal event, so token usage and IDs do
// not get duplicated when Eino concatenates stream chunks.
func resolveResponsesStreamEvent(event *responses.ResponseStreamEvent, req *responses.ResponsesRequest) (msg *schema.Message, terminal bool, err error) {
	if event == nil {
		return nil, false, nil
	}
	ev := &event.OneOf
	switch ev.Type {
	case responses.ResponseCompletedEventResponseStreamEventSum:
		msg, err = resolveResponsesResponse(&ev.ResponseCompletedEvent.Response, req)
		return msg, true, err
	case responses.ResponseIncompleteEventResponseStreamEventSum:
		msg, err = resolveResponsesResponse(&ev.ResponseIncompleteEvent.Response, req)
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
			msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, ID: call.CallID.Or(""), Type: "function", Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)},
				Function: schema.FunctionCall{Name: call.Name.Or(""), Arguments: call.Arguments.Or("")}}}}
		}
	case responses.ResponseFunctionCallArgumentsDeltaEventResponseStreamEventSum:
		delta := ev.ResponseFunctionCallArgumentsDeltaEvent
		index := int(delta.OutputIndex)
		msg = &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &index, Extra: map[string]any{keyResponsesItemIndex: metadataInt64(index)}, Function: schema.FunctionCall{Arguments: delta.Delta.Or("")}}}}
	}
	return msg, false, nil
}
