/*
 * Copyright 2024 CloudWeGo Authors
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

package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/meguminnnnnnnnn/go-openai"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var _ model.AgenticModel = (*AgenticClient)(nil)

type AgenticClient struct {
	client *Client
}

func NewAgenticClient(ctx context.Context, config *Config) (*AgenticClient, error) {
	c, err := NewClient(ctx, config)
	if err != nil {
		return nil, err
	}
	return &AgenticClient{client: c}, nil
}

func (ac *AgenticClient) Generate(ctx context.Context, input []*schema.AgenticMessage,
	opts ...model.Option) (outMsg *schema.AgenticMessage, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, ac.GetType(), components.ComponentOfAgenticModel)

	in, err := agenticMessagesToMessages(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert agentic messages: %w", err)
	}

	opts = ac.convertAgenticToolChoiceOpts(opts)

	req, cbInput, reqOpts, specOptions, err := ac.client.genRequest(ctx, in, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion request: %w", err)
	}

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    cbInput.Tools,
		Config:   toAgenticConfig(cbInput.Config),
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	resp, err := ac.client.cli.CreateChatCompletion(ctx, *req, reqOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	msg, err := buildGenerateResponse(resp, ac.client.config)
	if err != nil {
		return nil, err
	}

	if specOptions.ResponseMessageModifier != nil {
		msg, err = specOptions.ResponseMessageModifier(ctx, msg, resp.RawBody)
		if err != nil {
			return nil, fmt.Errorf("failed to modify response message: %w", err)
		}
	}

	outMsg, err = messageToAgenticMessage(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response to agentic message: %w", err)
	}

	callbacks.OnEnd(ctx, &model.AgenticCallbackOutput{
		Message:    outMsg,
		Config:     toAgenticConfig(cbInput.Config),
		TokenUsage: toAgenticModelTokenUsage(outMsg.ResponseMeta),
	})

	return outMsg, nil
}

func (ac *AgenticClient) Stream(ctx context.Context, input []*schema.AgenticMessage,
	opts ...model.Option) (outStream *schema.StreamReader[*schema.AgenticMessage], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, ac.GetType(), components.ComponentOfAgenticModel)

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	in, err := agenticMessagesToMessages(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert agentic messages: %w", err)
	}

	opts = ac.convertAgenticToolChoiceOpts(opts)

	req, cbInput, reqOpts, specOptions, err := ac.client.genRequest(ctx, in, opts...)
	if err != nil {
		return nil, err
	}

	req.Stream = true
	req.StreamOptions = &openai.StreamOptions{IncludeUsage: true}

	config := toAgenticConfig(cbInput.Config)

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    cbInput.Tools,
		Config:   config,
	})

	stream, err := ac.client.cli.CreateChatCompletionStream(ctx, *req, reqOpts...)
	if err != nil {
		return nil, err
	}

	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](1)

	builder := newStreamMessageBuilder(ac.client.config.Audio)
	converter := newChunkConverter()

	go func(ctx_ context.Context) {
		defer func() {
			panicErr := recover()
			_ = stream.Close()

			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
		}()

		var lastEmptyMsg *schema.Message

		for {
			chunk, chunkErr := stream.Recv()
			if errors.Is(chunkErr, io.EOF) {
				if specOptions.ResponseChunkMessageModifier != nil {
					var err_ error
					lastEmptyMsg, err_ = specOptions.ResponseChunkMessageModifier(ctx_, lastEmptyMsg, nil, true)
					if err_ != nil {
						sw.Send(nil, fmt.Errorf("failed to modify chunk message: %w", err_))
						return
					}
				}
				if lastEmptyMsg != nil {
					agMsg, convErr := converter.convert(lastEmptyMsg)
					if convErr != nil {
						sw.Send(nil, fmt.Errorf("failed to convert chunk to agentic message: %w", convErr))
						return
					}
					sw.Send(&model.AgenticCallbackOutput{
						Message:    agMsg,
						Config:     config,
						TokenUsage: toAgenticModelTokenUsage(agMsg.ResponseMeta),
					}, nil)
				}
				return
			}

			if chunkErr != nil {
				_ = sw.Send(nil, fmt.Errorf("failed to receive stream chunk: %w", chunkErr))
				return
			}

			msg, found, buildErr := builder.build(chunk)
			if buildErr != nil {
				_ = sw.Send(nil, fmt.Errorf("failed to build message from stream chunk: %w", buildErr))
				return
			}
			if !found {
				continue
			}

			rc, ok := GetReasoningContent(msg)
			if lastEmptyMsg != nil {
				cMsg, cErr := schema.ConcatMessages([]*schema.Message{lastEmptyMsg, msg})
				if cErr != nil {
					_ = sw.Send(nil, fmt.Errorf("failed to concatenate stream messages: %w", cErr))
					return
				}
				msg = cMsg
			}

			if msg.Content == "" && len(msg.ToolCalls) == 0 && !(ok && len(rc) > 0) {
				lastEmptyMsg = msg
				continue
			}

			lastEmptyMsg = nil

			if specOptions.ResponseChunkMessageModifier != nil {
				var err_ error
				msg, err_ = specOptions.ResponseChunkMessageModifier(ctx_, msg, chunk.RawBody, false)
				if err_ != nil {
					sw.Send(nil, fmt.Errorf("failed to modify chunk message: %w", err_))
					return
				}
			}

			agMsg, convErr := converter.convert(msg)
			if convErr != nil {
				_ = sw.Send(nil, fmt.Errorf("failed to convert chunk to agentic message: %w", convErr))
				return
			}

			closed := sw.Send(&model.AgenticCallbackOutput{
				Message:    agMsg,
				Config:     config,
				TokenUsage: toAgenticModelTokenUsage(agMsg.ResponseMeta),
			}, nil)

			if closed {
				return
			}
		}
	}(ctx)

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *model.AgenticCallbackOutput) (callbacks.CallbackOutput, error) {
			return src, nil
		}))

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src callbacks.CallbackOutput) (*schema.AgenticMessage, error) {
			s := src.(*model.AgenticCallbackOutput)
			if s.Message == nil {
				return nil, schema.ErrNoValue
			}
			return s.Message, nil
		},
	)

	return outStream, nil
}

func (ac *AgenticClient) WithTools(tools []*schema.ToolInfo) (model.AgenticModel, error) {
	nc, err := ac.client.WithToolsForClient(tools)
	if err != nil {
		return nil, err
	}
	return &AgenticClient{client: nc}, nil
}

func (ac *AgenticClient) GetType() string {
	return "ChatModel/OpenAI"
}

func (ac *AgenticClient) IsCallbacksEnabled() bool {
	return true
}

func (ac *AgenticClient) convertAgenticToolChoiceOpts(opts []model.Option) []model.Option {
	options := model.GetCommonOptions(nil, opts...)
	if options.AgenticToolChoice == nil {
		return opts
	}

	tc, allowedNames := agenticToolChoiceToToolChoice(options.AgenticToolChoice)
	if tc != nil {
		opts = append(opts, model.WithToolChoice(*tc, allowedNames...))
	}
	return opts
}

func agenticToolChoiceToToolChoice(atc *schema.AgenticToolChoice) (*schema.ToolChoice, []string) {
	if atc == nil {
		return nil, nil
	}

	tc := atc.Type
	var allowedNames []string

	switch tc {
	case schema.ToolChoiceAllowed:
		if atc.Allowed != nil {
			for _, t := range atc.Allowed.Tools {
				if t.FunctionName != "" {
					allowedNames = append(allowedNames, t.FunctionName)
				}
			}
		}
	case schema.ToolChoiceForced:
		if atc.Forced != nil {
			for _, t := range atc.Forced.Tools {
				if t.FunctionName != "" {
					allowedNames = append(allowedNames, t.FunctionName)
				}
			}
		}
	}

	return &tc, allowedNames
}

func toAgenticConfig(config *model.Config) *model.AgenticConfig {
	if config == nil {
		return nil
	}
	return &model.AgenticConfig{
		Model:       config.Model,
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
		TopP:        config.TopP,
	}
}

func toAgenticModelTokenUsage(meta *schema.AgenticResponseMeta) *model.TokenUsage {
	if meta == nil || meta.TokenUsage == nil {
		return nil
	}
	usage := meta.TokenUsage
	return &model.TokenUsage{
		PromptTokens: usage.PromptTokens,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
		CompletionTokens: usage.CompletionTokens,
		CompletionTokensDetails: model.CompletionTokensDetails{
			ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		},
		TotalTokens: usage.TotalTokens,
	}
}
