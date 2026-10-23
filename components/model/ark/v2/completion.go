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

package ark

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func (cm *ChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outMsg *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	options, specOptions := cm.getOptions(opts...)

	req, err := cm.genRequest(in, options, specOptions)
	if err != nil {
		return nil, err
	}

	reqConf := callbackConfig(req)
	req.Stream = chat.NewOptBool(false)
	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &fmodel.CallbackInput{
		Messages:   in,
		Tools:      tools, // join tool info from call options
		ToolChoice: options.ToolChoice,
		Config:     reqConf,
		Extra:      map[string]any{"thinking": callbackThinking(req)},
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	wireReq, extraBody, err := prepareRequestPayload(ctx, in, req, specOptions.requestPayloadModifier)
	if err != nil {
		return nil, err
	}
	var capture *responseCapture
	if specOptions.responseMessageModifier != nil {
		capture = &responseCapture{}
	}
	ctx = context.WithValue(ctx, responseCaptureKey{}, capture)
	var resp *chat.ChatCompletionResponseWithHeader
	if cm.config.BatchChat != nil && cm.config.BatchChat.EnableBatchChat {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cm.config.BatchChat.BatchChatAsyncRetryTimeout)
		defer cancel()
		resp, err = cm.client.CreateBatchChatCompletion(ctx, wireReq, arkruntime.WithCustomHeaders(specOptions.customHeaders), arkruntime.WithExtraBody(extraBody))
	} else {
		resp, err = cm.client.CreateChatCompletion(ctx, wireReq, arkruntime.WithCustomHeaders(specOptions.customHeaders), arkruntime.WithExtraBody(extraBody))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	outMsg, err = resolveChatResponse(&resp.ChatCompletionResponse)
	if err != nil {
		return nil, err
	}
	if specOptions.responseMessageModifier != nil {
		outMsg, err = specOptions.responseMessageModifier(ctx, outMsg, capture.rawBody)
		if err != nil {
			return nil, fmt.Errorf("modify response message: %w", err)
		}
		if outMsg == nil {
			return nil, fmt.Errorf("response message modifier returned nil")
		}
	}

	callbacks.OnEnd(ctx, &fmodel.CallbackOutput{
		Message:    outMsg,
		Config:     reqConf,
		TokenUsage: toModelCallbackUsage(outMsg.ResponseMeta),
		Extra: map[string]any{
			"thinking":   callbackThinking(req),
			"model_name": resp.Model,
		},
	})

	return outMsg, nil
}

func (cm *ChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, cm.GetType(), components.ComponentOfChatModel)

	options, arkOpts := cm.getOptions(opts...)

	req, err := cm.genRequest(in, options, arkOpts)
	if err != nil {
		return nil, err
	}

	req.Stream = chat.NewOptBool(true)
	streamOptions := req.StreamOptions.Or(chat.ChatCompletionStreamOptions{})
	streamOptions.IncludeUsage = chat.NewOptBool(true)
	req.StreamOptions = chat.NewOptChatCompletionStreamOptions(streamOptions)

	reqConf := callbackConfig(req)
	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &fmodel.CallbackInput{
		Messages:   in,
		Tools:      tools,
		ToolChoice: options.ToolChoice,
		Config:     reqConf,
		Extra:      map[string]any{"thinking": callbackThinking(req)},
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	if cm.config.BatchChat != nil && cm.config.BatchChat.EnableBatchChat {
		return nil, fmt.Errorf("batch chat does not support streaming")
	}
	wireReq, extraBody, err := prepareRequestPayload(ctx, in, req, arkOpts.requestPayloadModifier)
	if err != nil {
		return nil, err
	}
	// A stream may be started from a response modifier's context. Do not let
	// that parent's response capture buffer consume this SSE response body.
	ctx = context.WithValue(ctx, responseCaptureKey{}, (*responseCapture)(nil))
	stream, err := cm.client.CreateChatCompletionStream(ctx, wireReq, arkruntime.WithCustomHeaders(arkOpts.customHeaders), arkruntime.WithExtraBody(extraBody))
	if err != nil {
		return nil, err
	}

	decoder := &streamPayloadDecoder{capture: arkOpts.responseChunkMessageModifier != nil}
	stream.Unmarshaler = decoder
	sr, sw := schema.Pipe[*fmodel.CallbackOutput](1)
	go func(ctx context.Context) {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
			_ = stream.Close()

		}()

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				// The SDK also returns EOF for a body closed before [DONE]. Such
				// a response is incomplete and must not run successful finalization.
				if !stream.IsFinished {
					_ = sw.Send(nil, io.ErrUnexpectedEOF)
					return
				}
				if modifier := arkOpts.responseChunkMessageModifier; modifier != nil {
					last, e := modifier(ctx, nil, nil, true)
					if e != nil {
						_ = sw.Send(nil, fmt.Errorf("modify final response chunk message: %w", e))
					} else if last != nil {
						_ = sw.Send(&fmodel.CallbackOutput{Message: last, Config: reqConf, TokenUsage: toModelCallbackUsage(last.ResponseMeta)}, nil)
					}
				}
				return
			}

			if err != nil {
				_ = sw.Send(nil, err)
				return
			}

			msg, msgFound, e := resolveStreamResponse(resp)
			if e != nil {
				_ = sw.Send(nil, e)
				return
			}

			if modifier := arkOpts.responseChunkMessageModifier; modifier != nil {
				msg, e = modifier(ctx, msg, decoder.rawBody, false)
				if e != nil {
					_ = sw.Send(nil, fmt.Errorf("modify response chunk message: %w", e))
					return
				}
				msgFound = msg != nil
			}
			if !msgFound {
				continue
			}

			closed := sw.Send(&fmodel.CallbackOutput{
				Message:    msg,
				Config:     reqConf,
				TokenUsage: toModelCallbackUsage(msg.ResponseMeta),
				Extra: map[string]any{
					"thinking":   callbackThinking(req),
					"model_name": resp.Model,
				},
			}, nil)
			if closed {
				return
			}
		}
	}(ctx)

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *fmodel.CallbackOutput) (callbacks.CallbackOutput, error) {
			return src, nil
		}))

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src callbacks.CallbackOutput) (*schema.Message, error) {
			s := src.(*fmodel.CallbackOutput)
			if s.Message == nil {
				return nil, schema.ErrNoValue
			}

			return s.Message, nil
		},
	)

	return outStream, nil
}
