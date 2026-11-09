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
	"math"

	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

// Keep the SDK payload and selected local callback metadata together.
type chatRequest struct {
	payload       *chat.ChatCompletionRequest
	callbackTools []*schema.ToolInfo
}

func (cm *ChatModel) genRequest(in []*schema.Message, options *fmodel.Options, opts *arkOptions) (*chatRequest, error) {
	c := &cm.config
	req := &chat.ChatCompletionRequest{
		Model: dereferenceOrZero(options.Model), Temperature: optFloat(options.Temperature),
		TopP: optFloat(options.TopP), FrequencyPenalty: optFloat(c.FrequencyPenalty),
		PresencePenalty: optFloat(c.PresencePenalty), RepetitionPenalty: optFloat(c.RepetitionPenalty),
		Messages: make([]chat.ChatCompletionRequestMessage, 0, len(in)),
	}
	var err error
	if req.MaxTokens, err = optInt(options.MaxTokens, "max_tokens"); err != nil {
		return nil, err
	}
	if req.MaxCompletionTokens, err = optInt(opts.maxCompletionTokens, "max_completion_tokens"); err != nil {
		return nil, err
	}
	if c.TopLogProbs > 0 {
		if req.TopLogprobs, err = optInt(&c.TopLogProbs, "top_logprobs"); err != nil {
			return nil, err
		}
	}
	if c.LogProbs {
		req.Logprobs = chat.NewOptBool(true)
	}
	if options.Stop != nil {
		req.Stop = chat.NewOptChatCompletionStop(chat.NewStringArrayChatCompletionStop(append([]string{}, options.Stop...)))
	}
	if c.LogitBias != nil {
		bias := make(chat.ChatCompletionRequestLogitBias, len(c.LogitBias))
		for k, v := range c.LogitBias {
			bias[k] = float32(v)
		}
		req.LogitBias = chat.NewOptChatCompletionRequestLogitBias(bias)
	}
	if c.ResponseFormat != nil {
		format := *c.ResponseFormat
		// The schema bytes are the format's only reference-valued field.
		// Copy them directly instead of serializing the entire SDK structure.
		format.JSONSchema.Value.Schema = append([]byte(nil), format.JSONSchema.Value.Schema...)
		req.ResponseFormat = chat.NewOptChatCompletionResponseFormat(format)
	}

	if opts.thinking != nil {
		req.Thinking = chat.NewOptThinking(*opts.thinking)
	}
	if opts.reasoningEffort != nil {
		req.ReasoningEffort = chat.NewOptReasoningEffort(*opts.reasoningEffort)
	}
	if c.ServiceTier != nil {
		req.ServiceTier = chat.NewOptRequestServiceTier(chat.RequestServiceTier(*c.ServiceTier))
	}
	if opts.parallelToolCalls != nil {
		req.ParallelToolCalls = chat.NewOptBool(*opts.parallelToolCalls)
	}
	if c.User != nil {
		req.User = chat.NewOptString(*c.User)
	}

	for i, msg := range in {
		converted, e := toArkMessage(msg)
		if e != nil {
			return nil, fmt.Errorf("message %d: %w", i, e)
		}
		req.Messages = append(req.Messages, converted)
	}
	tools, err := cm.buildChatTools(options)
	if err != nil {
		return nil, err
	}
	req.Tools, req.ToolChoice = tools.definitions, tools.choice
	return &chatRequest{payload: req, callbackTools: tools.callbackTools}, nil
}

func toArkMessage(msg *schema.Message) (chat.ChatCompletionRequestMessage, error) {
	if msg == nil {
		return chat.ChatCompletionRequestMessage{}, fmt.Errorf("message must not be nil")
	}
	content, err := toArkContent(msg)
	if err != nil {
		return chat.ChatCompletionRequestMessage{}, err
	}
	name := chat.OptString{}
	if msg.Name != "" {
		name = chat.NewOptString(msg.Name)
	}
	var sum chat.ChatCompletionRequestMessageSum
	switch msg.Role {
	case schema.User:
		sum = chat.NewChatCompletionRequestUserMessageChatCompletionRequestMessageSum(chat.ChatCompletionRequestUserMessage{
			Role: chat.ChatCompletionRequestUserMessageRoleUser, Content: chat.NewOptNilChatCompletionMessageContent(content), Name: name,
		})
	case schema.System:
		sum = chat.NewChatCompletionRequestSystemMessageChatCompletionRequestMessageSum(chat.ChatCompletionRequestSystemMessage{
			Role: chat.ChatCompletionRequestSystemMessageRoleSystem, Content: content, Name: name,
		})
	case Developer:
		sum = chat.NewChatCompletionRequestDeveloperMessageChatCompletionRequestMessageSum(chat.ChatCompletionRequestDeveloperMessage{
			Role: chat.ChatCompletionRequestDeveloperMessageRoleDeveloper, Content: content, Name: name,
		})
	case schema.Assistant:
		m := chat.ChatCompletionRequestAssistantMessage{
			Role: chat.ChatCompletionRequestAssistantMessageRoleAssistant, Content: chat.NewOptNilChatCompletionMessageContent(content), Name: name,
		}
		reasoning := msg.ReasoningContent
		encrypted, hasEncrypted := GetEncryptedContent(msg)
		var partReasoning, partEncrypted string
		for _, part := range msg.AssistantGenMultiContent {
			if part.Type == schema.ChatMessagePartTypeReasoning {
				partReasoning += part.Reasoning.Text
				if !hasEncrypted && part.Reasoning.Signature != "" {
					if partEncrypted != "" {
						return chat.ChatCompletionRequestMessage{}, fmt.Errorf("Chat Completions cannot merge encrypted reasoning from multiple output parts")
					}
					partEncrypted = part.Reasoning.Signature
				}
			}
		}
		if reasoning == "" {
			reasoning = partReasoning
		}
		if !hasEncrypted {
			encrypted = partEncrypted
		}
		if reasoning != "" {
			m.ReasoningContent = chat.NewOptNilChatCompletionMessageContent(chat.NewStringChatCompletionMessageContent(reasoning))
		}
		if hasEncrypted || encrypted != "" {
			m.EncryptedContent = chat.NewOptString(encrypted)
		}
		for _, tc := range msg.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, chat.ChatCompletionRequestMessageToolCall{
				ID: chat.NewOptNilString(tc.ID), Type: chat.ToolTypeFunction,
				Function: chat.ChatCompletionRequestMessageToolCallFunction{Name: tc.Function.Name, Arguments: chat.NewOptString(tc.Function.Arguments)},
			})
		}
		sum = chat.NewChatCompletionRequestAssistantMessageChatCompletionRequestMessageSum(m)
	case schema.Tool:
		sum = chat.NewChatCompletionRequestToolMessageChatCompletionRequestMessageSum(chat.ChatCompletionRequestToolMessage{
			Role: chat.ChatCompletionRequestToolMessageRoleTool, Content: chat.NewOptNilChatCompletionMessageContent(content),
			ToolCallID: chat.NewOptNilString(msg.ToolCallID), Name: name,
		})
	default:
		return chat.ChatCompletionRequestMessage{}, fmt.Errorf("unsupported role %q", msg.Role)
	}
	return chat.ChatCompletionRequestMessage{OneOf: sum}, nil
}

func optFloat(v *float32) chat.OptFloat64 {
	if v == nil {
		return chat.OptFloat64{}
	}
	// Keep the decimal value emitted by the legacy float32 JSON encoder when
	// widening into the new SDK's float64 fields (e.g. 0.7, not 0.699999988...).
	return chat.NewOptFloat64(float64FromFloat32(*v))
}
func optInt(v *int, name string) (chat.OptInt32, error) {
	if v == nil {
		return chat.OptInt32{}, nil
	}
	if *v < math.MinInt32 || *v > math.MaxInt32 {
		return chat.OptInt32{}, fmt.Errorf("%s overflows int32", name)
	}
	return chat.NewOptInt32(int32(*v)), nil
}
func dereferenceOrZero[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}

func callbackConfig(req *chat.ChatCompletionRequest) *fmodel.Config {
	var stop []string
	if s, ok := req.Stop.Get(); ok {
		if v, ok := s.GetStringArray(); ok {
			stop = v
		} else if v, ok := s.GetString(); ok {
			stop = []string{v}
		}
	}
	return &fmodel.Config{Model: req.Model, MaxTokens: int(req.MaxTokens.Or(req.MaxCompletionTokens.Or(0))), Temperature: float32(req.Temperature.Or(0)), TopP: float32(req.TopP.Or(0)), Stop: stop}
}

func callbackThinking(req *chat.ChatCompletionRequest) *chat.Thinking {
	if !req.Thinking.IsSet() {
		return nil
	}
	return &req.Thinking.Value
}
