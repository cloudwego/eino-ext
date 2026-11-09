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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/bytedance/sonic"
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

type chatTools struct {
	definitions   []chat.ChatCompletionTool
	choice        chat.OptChatCompletionToolChoice
	callbackTools []*schema.ToolInfo
}

// Select before encoding so wire definitions and callback metadata describe
// the same tools. The model and both option sets remain read-only.
func (cm *ChatModel) buildChatTools(common *fmodel.Options) (chatTools, error) {
	tools := cm.rawTools
	if common.Tools != nil {
		tools = common.Tools
	}
	selected, err := selectToolInfos(tools, common.ToolChoice, common.AllowedToolNames)
	if err != nil {
		return chatTools{}, err
	}
	if common.ToolChoice != nil && *common.ToolChoice == schema.ToolChoiceForced && len(selected) == 0 {
		return chatTools{}, fmt.Errorf("tool_choice is forced but no tools are provided")
	}
	definitions, err := toTools(selected, cm.config.ToolStrict)
	if err != nil {
		return chatTools{}, err
	}
	return chatTools{definitions: definitions, choice: toChatToolChoice(common.ToolChoice, common.AllowedToolNames), callbackTools: selected}, nil
}

func toChatToolChoice(choice *schema.ToolChoice, names []string) chat.OptChatCompletionToolChoice {
	if choice == nil {
		return chat.OptChatCompletionToolChoice{}
	}
	mode := chat.ToolChoiceModeAuto
	switch *choice {
	case schema.ToolChoiceForbidden:
		mode = chat.ToolChoiceModeNone
	case schema.ToolChoiceForced:
		if len(names) == 1 {
			return chat.NewOptChatCompletionToolChoice(chat.NewChatCompletionNamedToolChoiceChatCompletionToolChoice(chat.ChatCompletionNamedToolChoice{
				Type: chat.ChatCompletionNamedToolChoiceTypeFunction, Function: chat.ChatCompletionToolChoiceFunction{Name: names[0]},
			}))
		}
		mode = chat.ToolChoiceModeRequired
	}
	return chat.NewOptChatCompletionToolChoice(chat.NewToolChoiceModeChatCompletionToolChoice(mode))
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

		value, err := toolStrict(ti, strict)
		if err != nil {
			return nil, err
		}
		if value != nil {
			function.Strict = chat.NewOptBool(*value)
		}
		data, err := toolParametersJSON(ti)
		if err != nil {
			return nil, err
		}
		if data != nil {
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

func prepareRequestPayload(ctx context.Context, messages []*schema.Message, req *chat.ChatCompletionRequest, modifier RequestPayloadModifier) (*chat.ChatCompletionRequest, map[string]interface{}, error) {
	if modifier == nil {
		return req, nil, nil
	}
	raw, err := sonic.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request payload: %w", err)
	}
	raw, err = modifier(ctx, messages, raw)
	if err != nil {
		return nil, nil, fmt.Errorf("modify request payload: %w", err)
	}
	var fields map[string]json.RawMessage
	if err = sonic.Unmarshal(raw, &fields); err != nil {
		return nil, nil, fmt.Errorf("decode modified request payload: %w", err)
	}
	if fields == nil {
		return nil, nil, fmt.Errorf("modified request payload must be a JSON object")
	}
	var model string
	if value, ok := fields["model"]; !ok || sonic.Unmarshal(value, &model) != nil || model == "" {
		return nil, nil, fmt.Errorf("modified request payload must contain a non-empty string model")
	}
	var input []json.RawMessage
	messagesRaw, ok := fields["messages"]
	if !ok || bytes.Equal(bytes.TrimSpace(messagesRaw), []byte("null")) || sonic.Unmarshal(messagesRaw, &input) != nil {
		return nil, nil, fmt.Errorf("modified request payload must contain a messages array")
	}
	if value, ok := fields["n"]; ok && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		var n int
		if sonic.Unmarshal(value, &n) != nil || n != 1 {
			return nil, nil, fmt.Errorf("Eino ChatModel supports one completion: n must be omitted or 1")
		}
	}
	stream := req.Stream.Or(false)
	fields["stream"], _ = sonic.Marshal(stream)
	if stream {
		var options map[string]json.RawMessage
		if value, ok := fields["stream_options"]; ok {
			if err = sonic.Unmarshal(value, &options); err != nil {
				return nil, nil, fmt.Errorf("modified stream_options must be an object: %w", err)
			}
		}
		if options == nil {
			options = make(map[string]json.RawMessage)
		}
		options["include_usage"] = json.RawMessage("true")
		fields["stream_options"], _ = sonic.Marshal(options)
	}
	extra := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		extra[key] = value
	}
	// The SDK's public WithExtraBody option sends every modified JSON field,
	// including fields unknown to generated models. Start with an empty typed
	// request so deleting an optional field does not restore its configured value.
	wire := &chat.ChatCompletionRequest{Model: model, Stream: chat.NewOptBool(stream)}
	// Decode only callback configuration. The original JSON remains the wire
	// source, including unknown fields and exact numbers.
	for key, target := range map[string]interface{}{
		"max_tokens": &wire.MaxTokens, "max_completion_tokens": &wire.MaxCompletionTokens,
		"temperature": &wire.Temperature, "top_p": &wire.TopP, "stop": &wire.Stop, "thinking": &wire.Thinking,
	} {
		if value, exists := fields[key]; exists && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if err := sonic.Unmarshal(value, target); err != nil {
				return nil, nil, fmt.Errorf("modified %s: %w", key, err)
			}
		}
	}
	return wire, extra, nil
}
