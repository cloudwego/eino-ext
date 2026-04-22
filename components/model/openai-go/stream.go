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

package openaigo

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/responses"
)

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (outStream *schema.StreamReader[*schema.Message], err error) {
	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	params, cbIn, err := cm.buildParams(in, true, opts...)
	if err != nil {
		return nil, err
	}

	ctx = callbacks.OnStart(ctx, cbIn)
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	stream := cm.cli.Responses.NewStreaming(ctx, params)

	sr, sw := schema.Pipe[*model.CallbackOutput](1)
	go func() {
		defer func() {
			pe := recover()
			_ = stream.Close()
			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}
			sw.Close()
		}()

		state := newStreamState()
		for stream.Next() {
			ev := stream.Current()
			msg, done, deltaOnly, err2 := state.consume(ev)
			if err2 != nil {
				_ = sw.Send(nil, err2)
				return
			}
			if msg == nil {
				continue
			}

			// ensure callbacks can receive token usage on final chunk.
			if !deltaOnly {
				msg.ResponseMeta = ensureResponseMeta(msg.ResponseMeta)
			}

			closed := sw.Send(&model.CallbackOutput{
				Message:    msg,
				Config:     cbIn.Config,
				TokenUsage: toModelTokenUsage(msg.ResponseMeta),
				Extra: func() map[string]any {
					if done && state.modelName != "" {
						return map[string]any{callbackExtraModelName: state.modelName}
					}
					return nil
				}(),
			}, nil)
			if closed {
				return
			}
		}

		if stream.Err() != nil {
			_ = sw.Send(nil, stream.Err())
			return
		}
	}()

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *model.CallbackOutput) (callbacks.CallbackOutput, error) { return src, nil },
	))

	outStream = schema.StreamReaderWithConvert(nsr, func(src callbacks.CallbackOutput) (*schema.Message, error) {
		s := src.(*model.CallbackOutput)
		if s.Message == nil {
			return nil, schema.ErrNoValue
		}
		return s.Message, nil
	})

	return outStream, nil
}

// consume and map streaming events into eino messages.
type streamState struct {
	modelName       string
	functionArgBufs map[string]*strings.Builder // key: item_id
	callIDByItemID  map[string]string
	nameByItemID    map[string]string
}

func newStreamState() *streamState {
	return &streamState{
		functionArgBufs: make(map[string]*strings.Builder),
		callIDByItemID:  make(map[string]string),
		nameByItemID:    make(map[string]string),
	}
}

// consume returns:
// - msg: message chunk (delta)
// - done: if this chunk ends the response
// - deltaOnly: whether it's a pure delta message (so no finalization)
func (s *streamState) consume(ev responses.ResponseStreamEventUnion) (msg *schema.Message, done bool, deltaOnly bool, err error) {
	switch v := ev.AsAny().(type) {
	case responses.ResponseErrorEvent:
		return nil, false, false, fmt.Errorf("openai stream error: %s (%s)", v.Message, v.Code)
	case responses.ResponseCreatedEvent:
		s.modelName = string(v.Response.Model)
		return nil, false, true, nil
	case responses.ResponseInProgressEvent:
		// ignore; model name can be here too
		s.modelName = string(v.Response.Model)
		return nil, false, true, nil
	case responses.ResponseTextDeltaEvent:
		if v.Delta == "" {
			return nil, false, true, nil
		}
		return &schema.Message{Role: schema.Assistant, Content: v.Delta}, false, true, nil
	case responses.ResponseReasoningTextDeltaEvent:
		if v.Delta == "" {
			return nil, false, true, nil
		}
		m := &schema.Message{Role: schema.Assistant, ReasoningContent: v.Delta}
		return m, false, true, nil
	case responses.ResponseOutputItemAddedEvent:
		// function call item appears here with call_id and name
		item := v.Item
		if item.Type == "function_call" {
			call := item.AsFunctionCall()
			s.callIDByItemID[item.ID] = call.CallID
			s.nameByItemID[item.ID] = call.Name
		}
		return nil, false, true, nil
	case responses.ResponseFunctionCallArgumentsDeltaEvent:
		if v.Delta == "" {
			return nil, false, true, nil
		}
		b := s.functionArgBufs[v.ItemID]
		if b == nil {
			b = &strings.Builder{}
			s.functionArgBufs[v.ItemID] = b
		}
		b.WriteString(v.Delta)
		return nil, false, true, nil
	case responses.ResponseFunctionCallArgumentsDoneEvent:
		// Finalize args: only emit ToolCalls when arguments are complete.
		callID := s.callIDByItemID[v.ItemID]
		name := s.nameByItemID[v.ItemID]
		if callID == "" {
			callID = v.ItemID
		}

		args := v.Arguments
		if args == "" {
			if b := s.functionArgBufs[v.ItemID]; b != nil {
				args = b.String()
			}
		}
		return &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
			ID:   callID,
			Type: "function",
			Function: schema.FunctionCall{
				Name:      name,
				Arguments: args,
			},
		}}}, false, true, nil
	case responses.ResponseCompletedEvent:
		// IMPORTANT: do not emit the full final assistant message content here.
		// The Responses streaming API already sends the assistant text as deltas
		// (ResponseTextDeltaEvent / ResponseReasoningTextDeltaEvent). Emitting the
		// final full message (resp.OutputText()) would cause downstream consumers
		// that concatenate chunks to duplicate output.
		return &schema.Message{
			Role: schema.Assistant,
			ResponseMeta: &schema.ResponseMeta{
				FinishReason: string(v.Response.Status),
				Usage:        toEinoTokenUsage(v.Response.Usage),
			},
		}, true, false, nil
	case responses.ResponseFailedEvent:
		return &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{FinishReason: string(v.Response.Status)}}, true, false, nil
	case responses.ResponseIncompleteEvent:
		return &schema.Message{Role: schema.Assistant, ResponseMeta: &schema.ResponseMeta{FinishReason: string(v.Response.Status), Usage: toEinoTokenUsage(v.Response.Usage)}}, true, false, nil
	default:
		return nil, false, true, nil
	}
}
