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

package acp

import (
	"encoding/json"
	"fmt"
	"io"
	"iter"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	acpproto "github.com/eino-contrib/acp"
)

// InterruptConverter converts an adk.InterruptInfo into a sequence of ACP SessionUpdates.
// Users can provide a custom implementation to control how interrupt events are presented to the client.
type InterruptConverter func(info *adk.InterruptInfo) iter.Seq2[acpproto.SessionUpdate, error]

// EventConverterOption configures the behavior of AgentEventToSessionUpdate.
type EventConverterOption struct {
	// InterruptConverter is an optional custom converter for interrupt events.
	// If nil, the default conversion is used: the interrupt data is converted to
	// an AgentMessageChunk with the interrupt metadata in _meta.
	InterruptConverter InterruptConverter
}

// AgentEventToSessionUpdate converts an eino AgentEvent into a sequence of ACP SessionUpdate notifications.
// It handles message output (both streaming and non-streaming), tool calls, tool results, and interrupt events.
// For interrupt events, a custom InterruptConverter can be provided via opt; if nil, the default converter
// is used, which serializes the interrupt data as an AgentMessageChunk with interrupt metadata in _meta.
func AgentEventToSessionUpdate(
	event *adk.AgentEvent,
	opt *EventConverterOption,
) iter.Seq2[acpproto.SessionUpdate, error] {
	return func(yield func(acpproto.SessionUpdate, error) bool) {
		if event.Action != nil && event.Action.Interrupted != nil {
			conv := defaultInterruptConverter
			if opt != nil && opt.InterruptConverter != nil {
				conv = opt.InterruptConverter
			}
			for su, err := range conv(event.Action.Interrupted) {
				if !yield(su, err) {
					return
				}
			}
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			return
		}
		mo := event.Output.MessageOutput
		if !mo.IsStreaming {
			yieldMessageUpdates(mo.Message, yield)
			return
		}
		var lastRole schema.RoleType
		// pendingToolCalls accumulates streaming tool call argument chunks.
		// In streaming mode, models may send partial JSON arguments across
		// multiple messages. We buffer them and only yield when complete
		// (i.e. when a new message arrives without continuing the same tool calls,
		// or when the stream ends).
		pendingToolCalls := make(map[string]*schema.ToolCall) // keyed by tool call ID
		var pendingToolCallOrder []string

		flushToolCalls := func() bool {
			for _, id := range pendingToolCallOrder {
				tc := pendingToolCalls[id]
				if !yield(acpproto.NewSessionUpdateToolCall(fromToolCall(*tc)), nil) {
					return false
				}
			}
			pendingToolCalls = make(map[string]*schema.ToolCall)
			pendingToolCallOrder = nil
			return true
		}

		for {
			msg, err := mo.MessageStream.Recv()
			if err == io.EOF {
				flushToolCalls()
				return
			}
			if err != nil {
				flushToolCalls()
				yield(acpproto.SessionUpdate{}, err)
				return
			}
			if msg.Role == "" && lastRole != "" {
				msg.Role = lastRole
			}
			if msg.Role != "" {
				lastRole = msg.Role
			}

			// Accumulate tool call argument chunks instead of yielding immediately.
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					if existing, ok := pendingToolCalls[tc.ID]; ok {
						existing.Function.Arguments += tc.Function.Arguments
					} else {
						tcCopy := tc
						pendingToolCalls[tc.ID] = &tcCopy
						pendingToolCallOrder = append(pendingToolCallOrder, tc.ID)
					}
				}
				// Yield non-tool-call parts of this message (text, reasoning, etc.)
				clone := *msg
				clone.ToolCalls = nil
				if clone.Content != "" || clone.ReasoningContent != "" || len(clone.AssistantGenMultiContent) > 0 ||
					len(clone.UserInputMultiContent) > 0 || clone.ToolCallID != "" {
					if !yieldMessageUpdates(&clone, yield) {
						return
					}
				}
				continue
			}

			// Message has no tool calls — flush any pending tool calls first.
			if len(pendingToolCalls) > 0 {
				if !flushToolCalls() {
					return
				}
			}

			if !yieldMessageUpdates(msg, yield) {
				return
			}
		}
	}
}

func defaultInterruptConverter(info *adk.InterruptInfo) iter.Seq2[acpproto.SessionUpdate, error] {
	return func(yield func(acpproto.SessionUpdate, error) bool) {
		text, meta, err := marshalInterruptInfo(info)
		if err != nil {
			yield(acpproto.SessionUpdate{}, fmt.Errorf("failed to marshal interrupt info: %w", err))
			return
		}
		yield(acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
			Meta:    meta,
			Content: acpproto.NewContentBlockText(acpproto.TextContent{Text: text}),
		}), nil)
	}
}

func marshalInterruptInfo(info *adk.InterruptInfo) (text string, meta map[string]any, err error) {
	meta = map[string]any{
		"eino:interrupted": true,
	}

	// Convert Data to text
	switch v := info.Data.(type) {
	case string:
		text = v
	case nil:
		text = ""
	default:
		b, jErr := json.Marshal(v)
		if jErr != nil {
			text = fmt.Sprintf("%v", v)
		} else {
			text = string(b)
		}
	}

	// Convert InterruptContexts to JSON-safe structure for meta
	if len(info.InterruptContexts) > 0 {
		contexts := make([]map[string]any, 0, len(info.InterruptContexts))
		for _, ic := range info.InterruptContexts {
			ctx := interruptCtxToMap(ic)
			contexts = append(contexts, ctx)
		}
		meta["eino:interruptContexts"] = contexts
	}

	return text, meta, nil
}

func interruptCtxToMap(ic *adk.InterruptCtx) map[string]any {
	m := map[string]any{
		"id":          ic.ID,
		"isRootCause": ic.IsRootCause,
	}

	if len(ic.Address) > 0 {
		segs := make([]map[string]string, 0, len(ic.Address))
		for _, seg := range ic.Address {
			segs = append(segs, map[string]string{
				"type": string(seg.Type),
				"id":   seg.ID,
			})
		}
		m["address"] = segs
	}

	if ic.Info != nil {
		switch v := ic.Info.(type) {
		case string:
			m["info"] = v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				m["info"] = fmt.Sprintf("%v", v)
			} else {
				m["info"] = json.RawMessage(b)
			}
		}
	}

	if ic.Parent != nil {
		m["parentId"] = ic.Parent.ID
	}

	return m
}

func yieldMessageUpdates(msg adk.Message, yield func(acpproto.SessionUpdate, error) bool) bool {
	switch msg.Role {
	case schema.User:
		if msg.Content != "" {
			if !yield(acpproto.NewSessionUpdateUserMessageChunk(acpproto.ContentChunk{
				Content: acpproto.NewContentBlockText(acpproto.TextContent{Text: msg.Content}),
			}), nil) {
				return false
			}
			return true
		}
		for _, part := range msg.UserInputMultiContent {
			cb, err := inputPartToContentBlock(part)
			if err != nil {
				return yield(acpproto.SessionUpdate{}, err)
			}
			if !yield(acpproto.NewSessionUpdateUserMessageChunk(acpproto.ContentChunk{Content: cb}), nil) {
				return false
			}
		}
	case schema.Assistant:
		if msg.ReasoningContent != "" {
			if !yield(acpproto.NewSessionUpdateAgentThoughtChunk(acpproto.ContentChunk{
				Content: acpproto.NewContentBlockText(acpproto.TextContent{Text: msg.ReasoningContent}),
			}), nil) {
				return false
			}
		}
		if msg.Content != "" {
			if !yield(acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
				Content: acpproto.NewContentBlockText(acpproto.TextContent{Text: msg.Content}),
			}), nil) {
				return false
			}
		} else {
			for _, part := range msg.AssistantGenMultiContent {
				su, err := outputPartToSessionUpdate(part)
				if err != nil {
					return yield(acpproto.SessionUpdate{}, err)
				}
				if !yield(su, nil) {
					return false
				}
			}
		}
		for _, tc := range msg.ToolCalls {
			if !yield(acpproto.NewSessionUpdateToolCall(fromToolCall(tc)), nil) {
				return false
			}
		}
	case schema.Tool:
		tcID := acpproto.ToolCallID(msg.ToolCallID)
		if msg.Content != "" {
			if !yield(acpproto.NewSessionUpdateToolCallUpdate(acpproto.ToolCallUpdate{
				Content: []acpproto.ToolCallContent{acpproto.NewToolCallContentContent(acpproto.Content{
					Content: acpproto.NewContentBlockText(acpproto.TextContent{Text: msg.Content}),
				})},
				ToolCallID: tcID,
			}), nil) {
				return false
			}
			return true
		}
		contents := make([]acpproto.ToolCallContent, 0, len(msg.UserInputMultiContent))
		for _, part := range msg.UserInputMultiContent {
			cb, err := inputPartToContentBlock(part)
			if err != nil {
				return yield(acpproto.SessionUpdate{}, err)
			}
			contents = append(contents, acpproto.NewToolCallContentContent(acpproto.Content{Content: cb}))
		}
		if len(contents) > 0 {
			if !yield(acpproto.NewSessionUpdateToolCallUpdate(acpproto.ToolCallUpdate{
				ToolCallID: tcID,
				Content:    contents,
			}), nil) {
				return false
			}
		} else {
			return yield(acpproto.SessionUpdate{}, fmt.Errorf("tool message has no content (ToolCallID: %s)", msg.ToolCallID))
		}
	default:
		return yield(acpproto.SessionUpdate{}, fmt.Errorf("unsupported message role: %s", msg.Role))
	}
	return true
}

func inputPartToContentBlock(part schema.MessageInputPart) (acpproto.ContentBlock, error) {
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		return acpproto.NewContentBlockText(acpproto.TextContent{Text: part.Text}), nil
	case schema.ChatMessagePartTypeImageURL:
		if part.Image == nil {
			return acpproto.ContentBlock{}, fmt.Errorf("input part type is image_url but image field is nil")
		}
		ic := acpproto.ImageContent{MimeType: part.Image.MIMEType}
		if part.Image.URL != nil {
			ic.URI = *part.Image.URL
			return acpproto.NewContentBlockImage(ic), nil
		}
		if part.Image.Base64Data != nil {
			ic.Data = *part.Image.Base64Data
			return acpproto.NewContentBlockImage(ic), nil
		}
		return acpproto.ContentBlock{}, fmt.Errorf("input part image has neither URL nor base64 data")
	case schema.ChatMessagePartTypeAudioURL:
		if part.Audio == nil {
			return acpproto.ContentBlock{}, fmt.Errorf("input part type is audio_url but audio field is nil")
		}
		ac := acpproto.AudioContent{MimeType: part.Audio.MIMEType}
		if part.Audio.Base64Data != nil {
			ac.Data = *part.Audio.Base64Data
			return acpproto.NewContentBlockAudio(ac), nil
		}
		if part.Audio.URL != nil {
			return acpproto.ContentBlock{}, fmt.Errorf("input part audio has URL data, but ACP only supports base64-encoded audio")
		}
		return acpproto.ContentBlock{}, fmt.Errorf("input part audio has neither URL nor base64 data")
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return acpproto.ContentBlock{}, fmt.Errorf("input part type is video_url but video field is nil")
		}
		if part.Video.URL != nil {
			rl := acpproto.ResourceLink{MimeType: part.Video.MIMEType, URI: *part.Video.URL}
			return acpproto.NewContentBlockResourceLink(rl), nil
		}
		if part.Video.Base64Data != nil {
			// todo
		}
		return acpproto.ContentBlock{}, fmt.Errorf("input part video has neither URL nor base64 data")

	case schema.ChatMessagePartTypeFileURL:
		if part.File == nil {
			return acpproto.ContentBlock{}, fmt.Errorf("input part type is file_url but file field is nil")
		}
		if part.File.URL != nil {
			rl := acpproto.ResourceLink{Name: part.File.Name, MimeType: part.File.MIMEType, URI: *part.File.URL}
			return acpproto.NewContentBlockResourceLink(rl), nil
		}
		if part.File.Base64Data != nil {
			// todo
		}
		return acpproto.ContentBlock{}, fmt.Errorf("input part file has neither URL nor base64 data")

	default:
		return acpproto.ContentBlock{}, fmt.Errorf("unsupported input part type: %s", part.Type)
	}
}

func outputPartToSessionUpdate(part schema.MessageOutputPart) (acpproto.SessionUpdate, error) {
	switch part.Type {
	case schema.ChatMessagePartTypeText:
		return acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
			Content: acpproto.NewContentBlockText(acpproto.TextContent{Text: part.Text}),
		}), nil
	case schema.ChatMessagePartTypeReasoning:
		if part.Reasoning == nil {
			return acpproto.SessionUpdate{}, fmt.Errorf("output part type is reasoning but reasoning field is nil")
		}
		return acpproto.NewSessionUpdateAgentThoughtChunk(acpproto.ContentChunk{
			Content: acpproto.NewContentBlockText(acpproto.TextContent{Text: part.Reasoning.Text}),
		}), nil
	case schema.ChatMessagePartTypeImageURL:
		if part.Image == nil {
			return acpproto.SessionUpdate{}, fmt.Errorf("output part type is image_url but image field is nil")
		}
		ic := acpproto.ImageContent{MimeType: part.Image.MIMEType}
		if part.Image.URL != nil {
			ic.URI = *part.Image.URL
			return acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
				Content: acpproto.NewContentBlockImage(ic),
			}), nil
		}
		if part.Image.Base64Data != nil {
			ic.Data = *part.Image.Base64Data
			return acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
				Content: acpproto.NewContentBlockImage(ic),
			}), nil
		}
		return acpproto.SessionUpdate{}, fmt.Errorf("output part image has neither URL nor base64 data")

	case schema.ChatMessagePartTypeAudioURL:
		if part.Audio == nil {
			return acpproto.SessionUpdate{}, fmt.Errorf("output part type is audio_url but audio field is nil")
		}
		if part.Audio.Base64Data != nil {
			ac := acpproto.AudioContent{MimeType: part.Audio.MIMEType, Data: *part.Audio.Base64Data}
			return acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
				Content: acpproto.NewContentBlockAudio(ac),
			}), nil
		}
		if part.Audio.URL != nil {
			return acpproto.SessionUpdate{}, fmt.Errorf("output part audio has URL data, but ACP only supports base64-encoded audio")
		}
		return acpproto.SessionUpdate{}, fmt.Errorf("output part audio has neither URL nor base64 data")
	case schema.ChatMessagePartTypeVideoURL:
		if part.Video == nil {
			return acpproto.SessionUpdate{}, fmt.Errorf("output part type is video_url but video field is nil")
		}
		if part.Video.URL != nil {
			rc := acpproto.NewContentBlockResourceLink(acpproto.ResourceLink{
				MimeType: part.Video.MIMEType,
				URI:      *part.Video.URL,
			})
			return acpproto.NewSessionUpdateAgentMessageChunk(acpproto.ContentChunk{
				Content: rc,
			}), nil
		}
		if part.Video.Base64Data != nil {
			// todo
		}
		return acpproto.SessionUpdate{}, fmt.Errorf("output part video has neither URL nor base64 data")

	default:
		return acpproto.SessionUpdate{}, fmt.Errorf("unsupported output part type: %s", part.Type)
	}
}

func fromToolCall(call schema.ToolCall) acpproto.ToolCall {
	return acpproto.ToolCall{
		ToolCallID: acpproto.ToolCallID(call.ID),
		Title:      call.Function.Name,
		RawInput:   json.RawMessage(call.Function.Arguments),
	}
}
