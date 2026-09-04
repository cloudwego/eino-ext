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

package agenticark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	runtimev2 "github.com/volcengine/ark-runtime-go/arkruntime"
	responsesv2 "github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
	legacyresponses "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
)

// The legacy SDK only accepts string tool outputs. These private prefixes carry
// the new SDK's array form through the legacy request model without expanding
// agenticark's public API.
const (
	multimodalToolOutputPrefix = "\x00eino_ext_agenticark_multimodal_tool_output:"
	escapedToolOutputPrefix    = "\x00eino_ext_agenticark_escaped_tool_output:"
)

type responseStreamReader interface {
	Recv() (*legacyresponses.Event, error)
	Close() error
}

type runtimeBridge struct {
	client *runtimev2.Client
}

func (c *runtimeBridge) CreateResponses(ctx context.Context, req *legacyresponses.ResponsesRequest,
	headers map[string]string) (*legacyresponses.ResponseObject, error) {
	v2Req, err := toV2Request(req)
	if err != nil {
		return nil, err
	}

	res, err := c.client.CreateResponses(ctx, v2Req, runtimev2.WithCustomHeaders(headers))
	if err != nil {
		return nil, err
	}

	var legacy legacyresponses.ResponseObject
	if err := responseToLegacy(&res.Response, &legacy); err != nil {
		return nil, fmt.Errorf("convert responses response to legacy model: %w", err)
	}
	return &legacy, nil
}

func (c *runtimeBridge) CreateResponsesStream(ctx context.Context, req *legacyresponses.ResponsesRequest,
	headers map[string]string) (responseStreamReader, error) {
	v2Req, err := toV2Request(req)
	if err != nil {
		return nil, err
	}

	stream, err := c.client.CreateResponsesStream(ctx, v2Req, runtimev2.WithCustomHeaders(headers))
	if err != nil {
		return nil, err
	}
	return &streamBridge{stream: stream}, nil
}

type streamBridge struct {
	stream interface {
		Recv() (*responsesv2.ResponseStreamEvent, error)
		Close() error
	}
}

func (s *streamBridge) Recv() (*legacyresponses.Event, error) {
	event, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	if legacy, ok := reasoningStreamEventToLegacy(event); ok {
		return legacy, nil
	}

	raw, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal responses stream event: %w", err)
	}

	var legacy legacyresponses.Event
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return nil, fmt.Errorf("convert responses stream event to legacy model: %w", err)
	}
	return &legacy, nil
}

func reasoningStreamEventToLegacy(event *responsesv2.ResponseStreamEvent) (*legacyresponses.Event, bool) {
	switch event.OneOf.Type {
	case responsesv2.ResponseReasoningTextDeltaEventResponseStreamEventSum:
		ev := event.OneOf.ResponseReasoningTextDeltaEvent
		return newLegacyReasoningTextDeltaEvent(ev.ItemID, ev.OutputIndex, ev.ContentIndex, ev.SequenceNumber, ev.Delta), true
	case responsesv2.ResponseReasoningRawTextDeltaEventResponseStreamEventSum:
		ev := event.OneOf.ResponseReasoningRawTextDeltaEvent
		return newLegacyReasoningTextDeltaEvent(ev.ItemID, ev.OutputIndex, ev.ContentIndex, ev.SequenceNumber, ev.Delta), true
	case responsesv2.ResponseReasoningTextDoneEventResponseStreamEventSum:
		ev := event.OneOf.ResponseReasoningTextDoneEvent
		return newLegacyReasoningTextDoneEvent(ev.ItemID, ev.OutputIndex, ev.ContentIndex, ev.SequenceNumber, ev.Text), true
	case responsesv2.ResponseReasoningRawTextDoneEventResponseStreamEventSum:
		ev := event.OneOf.ResponseReasoningRawTextDoneEvent
		return newLegacyReasoningTextDoneEvent(ev.ItemID, ev.OutputIndex, ev.ContentIndex, ev.SequenceNumber, ev.Text), true
	default:
		return nil, false
	}
}

func newLegacyReasoningTextDeltaEvent(itemID string, outputIndex, contentIndex, sequenceNumber int64,
	delta responsesv2.OptString) *legacyresponses.Event {
	event := &legacyresponses.ReasoningTextDeltaEvent{
		Type:           legacyresponses.EventType_response_reasoning_text_delta,
		ItemId:         itemID,
		OutputIndex:    outputIndex,
		ContentIndex:   contentIndex,
		SequenceNumber: sequenceNumber,
	}
	if value, ok := delta.Get(); ok {
		event.Delta = &value
	}
	return &legacyresponses.Event{
		Event: &legacyresponses.Event_ReasoningRawTextDelta{
			ReasoningRawTextDelta: event,
		},
	}
}

func newLegacyReasoningTextDoneEvent(itemID string, outputIndex, contentIndex, sequenceNumber int64,
	text responsesv2.OptString) *legacyresponses.Event {
	event := &legacyresponses.ReasoningTextDoneEvent{
		Type:           legacyresponses.EventType_response_reasoning_text_done,
		ItemId:         itemID,
		OutputIndex:    outputIndex,
		ContentIndex:   contentIndex,
		SequenceNumber: sequenceNumber,
	}
	if value, ok := text.Get(); ok {
		event.Text = &value
	}
	return &legacyresponses.Event{
		Event: &legacyresponses.Event_ReasoningRawTextDone{
			ReasoningRawTextDone: event,
		},
	}
}

func (s *streamBridge) Close() error {
	return s.stream.Close()
}

func toV2Request(req *legacyresponses.ResponsesRequest) (*responsesv2.ResponsesRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("legacy responses request is nil")
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal legacy responses request: %w", err)
	}
	raw, err = rewriteMultimodalToolOutput(raw)
	if err != nil {
		return nil, fmt.Errorf("rewrite multimodal function tool output: %w", err)
	}

	var v2Req responsesv2.ResponsesRequest
	if err := json.Unmarshal(raw, &v2Req); err != nil {
		return nil, fmt.Errorf("unmarshal request into ark-runtime-go model: %w", err)
	}
	return &v2Req, nil
}

func rewriteMultimodalToolOutput(raw json.RawMessage) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		if output, ok := object["output"]; ok {
			var value string
			if err := json.Unmarshal(output, &value); err == nil {
				switch {
				case strings.HasPrefix(value, escapedToolOutputPrefix):
					unescaped, err := json.Marshal(strings.TrimPrefix(value, escapedToolOutputPrefix))
					if err != nil {
						return nil, err
					}
					object["output"] = unescaped
				case strings.HasPrefix(value, multimodalToolOutputPrefix):
					content := json.RawMessage(strings.TrimPrefix(value, multimodalToolOutputPrefix))
					var blocks []json.RawMessage
					if err := json.Unmarshal(content, &blocks); err != nil {
						return nil, fmt.Errorf("invalid multimodal tool output: %w", err)
					}
					object["output"] = content
				}
			}
		}
		for key, child := range object {
			if key == "output" {
				continue
			}
			rewritten, err := rewriteMultimodalToolOutput(child)
			if err != nil {
				return nil, err
			}
			object[key] = rewritten
		}
		return json.Marshal(object)
	}

	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		for i, child := range list {
			rewritten, err := rewriteMultimodalToolOutput(child)
			if err != nil {
				return nil, err
			}
			list[i] = rewritten
		}
		return json.Marshal(list)
	}
	return raw, nil
}

func responseToLegacy(src *responsesv2.Response, dst *legacyresponses.ResponseObject) error {
	raw, err := json.Marshal(src)
	if err != nil {
		return err
	}
	raw, err = rewriteReasoningContent(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func rewriteReasoningContent(raw json.RawMessage) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		var itemType string
		if typeValue, ok := object["type"]; ok {
			_ = json.Unmarshal(typeValue, &itemType)
		}
		if itemType == "reasoning" {
			if err := appendReasoningContentToSummary(object); err != nil {
				return nil, err
			}
		}

		for key, child := range object {
			rewritten, err := rewriteReasoningContent(child)
			if err != nil {
				return nil, err
			}
			object[key] = rewritten
		}
		return json.Marshal(object)
	}

	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		for i, child := range list {
			rewritten, err := rewriteReasoningContent(child)
			if err != nil {
				return nil, err
			}
			list[i] = rewritten
		}
		return json.Marshal(list)
	}
	return raw, nil
}

func appendReasoningContentToSummary(item map[string]json.RawMessage) error {
	content, ok := item["content"]
	if !ok {
		return nil
	}

	var contentItems []json.RawMessage
	if err := json.Unmarshal(content, &contentItems); err != nil {
		return fmt.Errorf("unmarshal reasoning content: %w", err)
	}

	var reasoningTextParts []json.RawMessage
	for _, contentItem := range contentItems {
		var block map[string]json.RawMessage
		if err := json.Unmarshal(contentItem, &block); err != nil {
			return fmt.Errorf("unmarshal reasoning content block: %w", err)
		}

		var blockType string
		if err := json.Unmarshal(block["type"], &blockType); err != nil {
			return fmt.Errorf("unmarshal reasoning content block type: %w", err)
		}
		if blockType != "reasoning_text" {
			continue
		}

		text, ok := block["text"]
		if !ok {
			continue
		}
		reasoningTextParts = append(reasoningTextParts, json.RawMessage(fmt.Sprintf(`{"type":"summary_text","text":%s}`, text)))
	}
	if len(reasoningTextParts) == 0 {
		return nil
	}

	var summary []json.RawMessage
	if summaryJSON, ok := item["summary"]; ok {
		if err := json.Unmarshal(summaryJSON, &summary); err != nil {
			return fmt.Errorf("unmarshal reasoning summary: %w", err)
		}
	}
	summary = append(summary, reasoningTextParts...)

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	item["summary"] = summaryJSON
	return nil
}

func convertJSON(src, dst any) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
