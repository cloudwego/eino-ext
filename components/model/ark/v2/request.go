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
	"strconv"

	"github.com/bytedance/sonic"
	fmodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

func (cm *ChatModel) genRequest(in []*schema.Message, options *fmodel.Options, opts *arkOptions) (*chat.ChatCompletionRequest, error) {
	c := &cm.config
	req := &chat.ChatCompletionRequest{
		Model: dereferenceOrZero(options.Model), Temperature: optFloat(options.Temperature),
		TopP: optFloat(options.TopP), FrequencyPenalty: optFloat(c.FrequencyPenalty),
		PresencePenalty: optFloat(c.PresencePenalty), RepetitionPenalty: optFloat(c.RepetitionPenalty),
		Tools: append([]chat.ChatCompletionTool(nil), cm.tools...),
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
		// Keep the request's schema independent from the configured schema bytes.
		data, e := sonic.Marshal(c.ResponseFormat)
		if e != nil {
			return nil, fmt.Errorf("marshal response format: %w", e)
		}
		var format chat.ChatCompletionResponseFormat
		if e = sonic.Unmarshal(data, &format); e != nil {
			return nil, fmt.Errorf("decode response format: %w", e)
		}
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
	if options.Tools != nil {
		req.Tools, err = toTools(options.Tools, c.ToolStrict)
		if err != nil {
			return nil, err
		}
	}
	for i, msg := range in {
		converted, e := toArkMessage(msg)
		if e != nil {
			return nil, fmt.Errorf("message %d: %w", i, e)
		}
		req.Messages = append(req.Messages, converted)
	}
	if err = populateToolChoice(req, options.ToolChoice, options.AllowedToolNames); err != nil {
		return nil, err
	}
	return req, nil
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
	case schema.RoleType("developer"):
		sum = chat.NewChatCompletionRequestDeveloperMessageChatCompletionRequestMessageSum(chat.ChatCompletionRequestDeveloperMessage{
			Role: chat.ChatCompletionRequestDeveloperMessageRoleDeveloper, Content: content, Name: name,
		})
	case schema.Assistant:
		m := chat.ChatCompletionRequestAssistantMessage{
			Role: chat.ChatCompletionRequestAssistantMessageRoleAssistant, Content: chat.NewOptNilChatCompletionMessageContent(content), Name: name,
		}
		reasoning := msg.ReasoningContent
		if reasoning == "" {
			reasoning, _ = GetReasoningContent(msg)
		}
		if reasoning != "" {
			m.ReasoningContent = chat.NewOptNilChatCompletionMessageContent(chat.NewStringChatCompletionMessageContent(reasoning))
		}
		if encrypted, ok := GetEncryptedContent(msg); ok {
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

func toTools(tools []*schema.ToolInfo, strict *bool) ([]chat.ChatCompletionTool, error) {
	if tools == nil {
		return nil, nil
	}
	result := make([]chat.ChatCompletionTool, 0, len(tools))
	for _, ti := range tools {
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil")
		}
		function := chat.FunctionObject{Name: ti.Name}
		if ti.Desc != "" {
			function.Description = chat.NewOptString(ti.Desc)
		}
		if strict != nil {
			function.Strict = chat.NewOptBool(*strict)
		}
		if value, exists := ti.Extra[toolStrictKey]; exists {
			override, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("tool %s strict must be a bool, got %T", ti.Name, value)
			}
			function.Strict = chat.NewOptBool(override)
		}
		if ti.ParamsOneOf != nil {
			params, err := ti.ParamsOneOf.ToJSONSchema()
			if err != nil {
				return nil, fmt.Errorf("tool %s parameters: %w", ti.Name, err)
			}
			data, err := sonic.Marshal(params)
			if err != nil {
				return nil, fmt.Errorf("tool %s schema: %w", ti.Name, err)
			}
			var obj chat.FunctionObjectParameters
			if err = sonic.Unmarshal(data, &obj); err != nil {
				return nil, fmt.Errorf("tool %s schema object: %w", ti.Name, err)
			}
			function.Parameters = chat.NewOptFunctionObjectParameters(obj)
		}
		result = append(result, chat.ChatCompletionTool{Type: chat.ToolTypeFunction, Function: function})
	}
	return result, nil
}

func populateToolChoice(req *chat.ChatCompletionRequest, choice *schema.ToolChoice, names []string) error {
	if choice == nil {
		return nil
	}
	mode := chat.ToolChoiceModeAuto
	switch *choice {
	case schema.ToolChoiceForbidden:
		mode = chat.ToolChoiceModeNone
	case schema.ToolChoiceForced:
		if len(req.Tools) == 0 {
			return fmt.Errorf("tool_choice is forced but no tools are provided")
		}
		if len(names) > 1 {
			return fmt.Errorf("only one allowed tool name can be configured")
		}
		var name string
		if len(names) == 1 {
			name = names[0]
			found := false
			for _, tool := range req.Tools {
				if tool.Function.Name == name {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("allowed tool name %q not found in tools list", name)
			}
		} else if len(req.Tools) == 1 {
			name = req.Tools[0].Function.Name
		}
		if name != "" {
			req.ToolChoice = chat.NewOptChatCompletionToolChoice(chat.NewChatCompletionNamedToolChoiceChatCompletionToolChoice(chat.ChatCompletionNamedToolChoice{
				Type: chat.ChatCompletionNamedToolChoiceTypeFunction, Function: chat.ChatCompletionToolChoiceFunction{Name: name},
			}))
			return nil
		}
		mode = chat.ToolChoiceModeRequired
	}
	req.ToolChoice = chat.NewOptChatCompletionToolChoice(chat.NewToolChoiceModeChatCompletionToolChoice(mode))
	return nil
}

func optFloat(v *float32) chat.OptFloat64 {
	if v == nil {
		return chat.OptFloat64{}
	}
	// Keep the decimal value emitted by the legacy float32 JSON encoder when
	// widening into the new SDK's float64 fields (e.g. 0.7, not 0.699999988...).
	value, _ := strconv.ParseFloat(strconv.FormatFloat(float64(*v), 'g', -1, 32), 64)
	return chat.NewOptFloat64(value)
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
func newPanicErr(info any, stack []byte) error {
	return fmt.Errorf("panic error: %v, stack: %s", info, stack)
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
	return &fmodel.Config{Model: req.Model, MaxTokens: int(req.MaxTokens.Or(0)), Temperature: float32(req.Temperature.Or(0)), TopP: float32(req.TopP.Or(0)), Stop: stop}
}

func callbackThinking(req *chat.ChatCompletionRequest) *chat.Thinking {
	if !req.Thinking.IsSet() {
		return nil
	}
	return &req.Thinking.Value
}
