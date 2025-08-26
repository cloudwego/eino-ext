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

	"github.com/eino-contrib/jsonschema"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	autils "github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type completionAPIChatModel struct {
	client *arkruntime.Client

	tools    []tool
	rawTools []*schema.ToolInfo

	model            string
	maxTokens        *int
	temperature      *float32
	topP             *float32
	stop             []string
	frequencyPenalty *float32
	logitBias        map[string]int
	presencePenalty  *float32
	customHeader     map[string]string
	logProbs         bool
	topLogProbs      int
	responseFormat   *ResponseFormat
	thinking         *model.Thinking
	cache            *CacheConfig
}

type tool struct {
	Function *functionDefinition `json:"function,omitempty"`
}

type functionDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  *jsonschema.Schema `json:"parameters"`
	Examples    []string           `json:"examples"`
}

func (cm *completionAPIChatModel) Generate(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outMsg *schema.Message, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, getType(), components.ComponentOfChatModel)

	options := fmodel.GetCommonOptions(&fmodel.Options{
		Temperature: cm.temperature,
		MaxTokens:   cm.maxTokens,
		Model:       &cm.model,
		TopP:        cm.topP,
		Stop:        cm.stop,
		Tools:       nil,
	}, opts...)

	arkOpts := fmodel.GetImplSpecificOptions(&arkOptions{
		customHeaders: cm.customHeader,
		thinking:      cm.thinking,
	}, opts...)

	req, err := cm.genRequest(in, options, arkOpts)
	if err != nil {
		return nil, err
	}

	reqConf := &fmodel.Config{
		Model:       req.Model,
		MaxTokens:   dereferenceOrZero(req.MaxTokens),
		Temperature: dereferenceOrZero(req.Temperature),
		TopP:        dereferenceOrZero(req.TopP),
		Stop:        req.Stop,
	}

	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &fmodel.CallbackInput{
		Messages: in,
		Tools:    tools, // join tool info from call options
		Config:   reqConf,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	var resp model.ChatCompletionResponse
	if arkOpts.cache != nil && arkOpts.cache.ContextID != nil {
		resp, err = cm.client.CreateContextChatCompletion(ctx, *cm.convCompletionRequest(req, *arkOpts.cache.ContextID),
			arkruntime.WithCustomHeaders(arkOpts.customHeaders))
	} else {
		resp, err = cm.client.CreateChatCompletion(ctx, *req, arkruntime.WithCustomHeaders(arkOpts.customHeaders))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	outMsg, err = cm.resolveChatResponse(resp)
	if err != nil {
		return nil, err
	}

	callbacks.OnEnd(ctx, &fmodel.CallbackOutput{
		Message:    outMsg,
		Config:     reqConf,
		TokenUsage: cm.toModelCallbackUsage(outMsg.ResponseMeta),
	})

	return outMsg, nil
}

func (cm *completionAPIChatModel) Stream(ctx context.Context, in []*schema.Message, opts ...fmodel.Option) (
	outStream *schema.StreamReader[*schema.Message], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, getType(), components.ComponentOfChatModel)

	options := fmodel.GetCommonOptions(&fmodel.Options{
		Temperature: cm.temperature,
		MaxTokens:   cm.maxTokens,
		Model:       &cm.model,
		TopP:        cm.topP,
		Stop:        cm.stop,
		Tools:       nil,
	}, opts...)

	arkOpts := fmodel.GetImplSpecificOptions(&arkOptions{
		customHeaders: cm.customHeader,
		thinking:      cm.thinking,
	}, opts...)

	req, err := cm.genRequest(in, options, arkOpts)
	if err != nil {
		return nil, err
	}

	req.Stream = ptrOf(true)
	req.StreamOptions = &model.StreamOptions{IncludeUsage: true}

	reqConf := &fmodel.Config{
		Model:       req.Model,
		MaxTokens:   dereferenceOrZero(req.MaxTokens),
		Temperature: dereferenceOrZero(req.Temperature),
		TopP:        dereferenceOrZero(req.TopP),
		Stop:        req.Stop,
	}

	tools := cm.rawTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &fmodel.CallbackInput{
		Messages: in,
		Tools:    tools,
		Config:   reqConf,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	var stream *autils.ChatCompletionStreamReader
	if arkOpts.cache != nil && arkOpts.cache.ContextID != nil {
		stream, err = cm.client.CreateContextChatCompletionStream(ctx, *cm.convCompletionRequest(req, *arkOpts.cache.ContextID),
			arkruntime.WithCustomHeaders(arkOpts.customHeaders))
	} else {
		stream, err = cm.client.CreateChatCompletionStream(ctx, *req, arkruntime.WithCustomHeaders(arkOpts.customHeaders))
	}
	if err != nil {
		return nil, err
	}

	sr, sw := schema.Pipe[*fmodel.CallbackOutput](1)
	go func() {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				_ = sw.Send(nil, newPanicErr(panicErr, debug.Stack()))
			}

			sw.Close()
			_ = cm.closeArkStreamReader(stream)

		}()

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}

			if err != nil {
				_ = sw.Send(nil, err)
				return
			}

			msg, msgFound, e := cm.resolveStreamResponse(resp)
			if e != nil {
				_ = sw.Send(nil, e)
				return
			}

			if !msgFound {
				continue
			}

			closed := sw.Send(&fmodel.CallbackOutput{
				Message:    msg,
				Config:     reqConf,
				TokenUsage: cm.toModelCallbackUsage(msg.ResponseMeta),
			}, nil)
			if closed {
				return
			}
		}
	}()

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

func (cm *completionAPIChatModel) genRequest(in []*schema.Message, options *fmodel.Options, arkOpts *arkOptions) (req *model.CreateChatCompletionRequest, err error) {
	req = &model.CreateChatCompletionRequest{
		MaxTokens:        options.MaxTokens,
		Temperature:      options.Temperature,
		TopP:             options.TopP,
		Model:            dereferenceOrZero(options.Model),
		Stop:             options.Stop,
		FrequencyPenalty: cm.frequencyPenalty,
		LogitBias:        cm.logitBias,
		PresencePenalty:  cm.presencePenalty,
		Thinking:         arkOpts.thinking,
	}

	if cm.responseFormat != nil {
		req.ResponseFormat = &model.ResponseFormat{
			Type:       cm.responseFormat.Type,
			JSONSchema: cm.responseFormat.JSONSchema,
		}
	}

	if cm.logProbs {
		req.LogProbs = &cm.logProbs
	}
	if cm.topLogProbs > 0 {
		req.TopLogProbs = &cm.topLogProbs
	}

	for _, msg := range in {
		content, e := cm.toArkContent(msg.Content, msg.MultiContent)
		if e != nil {
			return req, e
		}

		nMsg := &model.ChatCompletionMessage{
			Content:    content,
			Role:       string(msg.Role),
			ToolCallID: msg.ToolCallID,
			ToolCalls:  cm.toArkToolCalls(msg.ToolCalls),
		}
		if len(msg.Name) > 0 {
			nMsg.Name = &msg.Name
		}
		req.Messages = append(req.Messages, nMsg)
	}

	tools := cm.tools
	if options.Tools != nil {
		if tools, err = cm.toTools(options.Tools); err != nil {
			return nil, err
		}
	}

	if tools != nil {
		req.Tools = make([]*model.Tool, 0, len(tools))

		for _, tool := range tools {
			arkTool := &model.Tool{
				Type: model.ToolTypeFunction,
				Function: &model.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}

			req.Tools = append(req.Tools, arkTool)
		}
	}

	return req, nil
}

func (cm *completionAPIChatModel) toLogProbs(probs *model.LogProbs) *schema.LogProbs {
	if probs == nil {
		return nil
	}
	ret := &schema.LogProbs{}
	for _, content := range probs.Content {
		schemaContent := schema.LogProb{
			Token:       content.Token,
			LogProb:     content.LogProb,
			Bytes:       runeSlice2int64(content.Bytes),
			TopLogProbs: cm.toTopLogProb(content.TopLogProbs),
		}
		ret.Content = append(ret.Content, schemaContent)
	}
	return ret
}

func (cm *completionAPIChatModel) toTopLogProb(probs []*model.TopLogProbs) []schema.TopLogProb {
	ret := make([]schema.TopLogProb, 0, len(probs))
	for _, prob := range probs {
		ret = append(ret, schema.TopLogProb{
			Token:   prob.Token,
			LogProb: prob.LogProb,
			Bytes:   runeSlice2int64(prob.Bytes),
		})
	}
	return ret
}

func runeSlice2int64(in []rune) []int64 {
	ret := make([]int64, 0, len(in))
	for _, v := range in {
		ret = append(ret, int64(v))
	}
	return ret
}

func (cm *completionAPIChatModel) resolveChatResponse(resp model.ChatCompletionResponse) (msg *schema.Message, err error) {
	if len(resp.Choices) == 0 {
		return nil, ErrEmptyResponse
	}

	var choice *model.ChatCompletionChoice

	for _, c := range resp.Choices {
		if c.Index == 0 {
			choice = c
			break
		}
	}

	if choice == nil {
		return nil, fmt.Errorf("invalid response format: choice with index 0 not found")
	}

	content := choice.Message.Content
	if content == nil && len(choice.Message.ToolCalls) == 0 {
		return nil, fmt.Errorf("invalid response format: message has neither content nor tool calls")
	}

	msg = &schema.Message{
		Role:       schema.RoleType(choice.Message.Role),
		ToolCallID: choice.Message.ToolCallID,
		ToolCalls:  cm.toMessageToolCalls(choice.Message.ToolCalls),
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: string(choice.FinishReason),
			Usage:        cm.toEinoTokenUsage(&resp.Usage),
			LogProbs:     cm.toLogProbs(choice.LogProbs),
		},
		Extra: map[string]any{},
	}

	setModelName(msg, resp.Model)
	setArkRequestID(msg, resp.ID)

	if content != nil && content.StringValue != nil {
		msg.Content = *content.StringValue
	}

	if choice.Message.ReasoningContent != nil {
		setReasoningContent(msg, *choice.Message.ReasoningContent)
		msg.ReasoningContent = *choice.Message.ReasoningContent
	}

	return msg, nil
}

func (cm *completionAPIChatModel) resolveStreamResponse(resp model.ChatCompletionStreamResponse) (msg *schema.Message, msgFound bool, err error) {
	if len(resp.Choices) > 0 {

		for _, choice := range resp.Choices {
			if choice.Index != 0 {
				continue
			}

			msgFound = true
			msg = &schema.Message{
				Role:      schema.RoleType(choice.Delta.Role),
				ToolCalls: cm.toMessageToolCalls(choice.Delta.ToolCalls),
				Content:   choice.Delta.Content,
				ResponseMeta: &schema.ResponseMeta{
					FinishReason: string(choice.FinishReason),
					Usage:        cm.toEinoTokenUsage(resp.Usage),
					LogProbs:     cm.toLogProbs(choice.LogProbs),
				},
				Extra: map[string]any{},
			}

			if choice.Delta.ReasoningContent != nil {
				setReasoningContent(msg, *choice.Delta.ReasoningContent)
				msg.ReasoningContent = *choice.Delta.ReasoningContent
			}

			break
		}
	}

	if !msgFound && resp.Usage != nil {
		msgFound = true
		msg = &schema.Message{
			ResponseMeta: &schema.ResponseMeta{
				Usage: cm.toEinoTokenUsage(resp.Usage),
			},
			Extra: map[string]any{},
		}
	}
	setArkRequestID(msg, resp.ID)
	setModelName(msg, resp.Model)

	return msg, msgFound, nil
}

func (cm *completionAPIChatModel) toTools(tls []*schema.ToolInfo) ([]tool, error) {
	tools := make([]tool, len(tls))
	for i := range tls {
		ti := tls[i]
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil")
		}

		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSONSchema: %w", err)
		}

		tools[i] = tool{
			Function: &functionDefinition{
				Name:        ti.Name,
				Description: ti.Desc,
				Parameters:  paramsJSONSchema,
			},
		}
	}

	return tools, nil
}

func (cm *completionAPIChatModel) toMessageToolCalls(toolCalls []*model.ToolCall) []schema.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]schema.ToolCall, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		ret[i] = schema.ToolCall{
			Index: toolCall.Index,
			ID:    toolCall.ID,
			Type:  string(toolCall.Type),
			Function: schema.FunctionCall{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}
	}

	return ret
}

func (cm *completionAPIChatModel) toArkContent(content string, multiContent []schema.ChatMessagePart) (*model.ChatCompletionMessageContent, error) {
	if len(multiContent) == 0 {
		return &model.ChatCompletionMessageContent{StringValue: ptrOf(content)}, nil
	}

	parts := make([]*model.ChatCompletionMessageContentPart, 0, len(multiContent))

	for _, part := range multiContent {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			parts = append(parts, &model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeText,
				Text: part.Text,
			})
		case schema.ChatMessagePartTypeImageURL:
			if part.ImageURL == nil {
				return nil, fmt.Errorf("ImageURL field must not be nil when Type is ChatMessagePartTypeImageURL")
			}
			parts = append(parts, &model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeImageURL,
				ImageURL: &model.ChatMessageImageURL{
					URL:    part.ImageURL.URL,
					Detail: model.ImageURLDetail(part.ImageURL.Detail),
				},
			})
		case schema.ChatMessagePartTypeVideoURL:
			if part.VideoURL == nil {
				return nil, fmt.Errorf("VideoURL field must not be nil when Type is ChatMessagePartTypeVideoURL")
			}
			parts = append(parts, &model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeVideoURL,
				VideoURL: &model.ChatMessageVideoURL{
					URL: part.VideoURL.URL,
					FPS: GetFPS(part.VideoURL),
				},
			})
		default:
			return nil, fmt.Errorf("unsupported chat message part type: %s", part.Type)
		}
	}

	return &model.ChatCompletionMessageContent{
		ListValue: parts,
	}, nil
}

func (cm *completionAPIChatModel) toArkToolCalls(toolCalls []schema.ToolCall) []*model.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	ret := make([]*model.ToolCall, len(toolCalls))
	for i := range toolCalls {
		toolCall := toolCalls[i]
		ret[i] = &model.ToolCall{
			ID:   toolCall.ID,
			Type: model.ToolTypeFunction,
			Function: model.FunctionCall{
				Arguments: toolCall.Function.Arguments,
				Name:      toolCall.Function.Name,
			},
			Index: toolCall.Index,
		}
	}

	return ret
}

func (cm *completionAPIChatModel) convCompletionRequest(req *model.CreateChatCompletionRequest, contextID string) *model.ContextChatCompletionRequest {
	return &model.ContextChatCompletionRequest{
		ContextID:        contextID,
		Model:            req.Model,
		Messages:         req.Messages,
		MaxTokens:        dereferenceOrZero(req.MaxTokens),
		Temperature:      dereferenceOrZero(req.Temperature),
		TopP:             dereferenceOrZero(req.TopP),
		Stream:           dereferenceOrZero(req.Stream),
		Stop:             req.Stop,
		FrequencyPenalty: dereferenceOrZero(req.FrequencyPenalty),
		LogitBias:        req.LogitBias,
		LogProbs:         dereferenceOrZero(req.LogProbs),
		TopLogProbs:      dereferenceOrZero(req.TopLogProbs),
		User:             dereferenceOrZero(req.User),
		FunctionCall:     req.FunctionCall,
		Tools:            req.Tools,
		ToolChoice:       req.ToolChoice,
		StreamOptions:    req.StreamOptions,
	}
}

func (cm *completionAPIChatModel) closeArkStreamReader(r *autils.ChatCompletionStreamReader) error {
	if r == nil || r.Response == nil || r.Response.Body == nil {
		return nil
	}
	return r.Close()
}

func (cm *completionAPIChatModel) toEinoTokenUsage(usage *model.Usage) *schema.TokenUsage {
	if usage == nil {
		return nil
	}
	return &schema.TokenUsage{
		CompletionTokens: usage.CompletionTokens,
		PromptTokens:     usage.PromptTokens,
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: usage.PromptTokensDetails.CachedTokens,
		},
		TotalTokens: usage.TotalTokens,
	}
}

func (cm *completionAPIChatModel) toModelCallbackUsage(respMeta *schema.ResponseMeta) *fmodel.TokenUsage {
	if respMeta == nil {
		return nil
	}
	usage := respMeta.Usage
	if usage == nil {
		return nil
	}
	return &fmodel.TokenUsage{
		CompletionTokens: usage.CompletionTokens,
		PromptTokens:     usage.PromptTokens,
		PromptTokenDetails: fmodel.PromptTokenDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
		TotalTokens: usage.TotalTokens,
	}
}
