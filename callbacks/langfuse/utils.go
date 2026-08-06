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

package langfuse

import (
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/libs/acl/langfuse"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func convModelCallbackInput(in []callbacks.CallbackInput) []*model.CallbackInput {
	ret := make([]*model.CallbackInput, len(in))
	for i, c := range in {
		ret[i] = model.ConvCallbackInput(c)
	}
	return ret
}

type modelInput struct {
	config     *model.Config
	messages   []*schema.Message
	extra      map[string]interface{}
	tools      []*schema.ToolInfo
	toolChoice *schema.ToolChoice
}

func extractModelInput(ins []*model.CallbackInput) (out *modelInput, err error) {
	out = &modelInput{}
	var mas [][]*schema.Message
	for _, in := range ins {
		if in == nil {
			continue
		}
		if len(in.Messages) > 0 {
			mas = append(mas, in.Messages)
		}
		if len(in.Extra) > 0 {
			out.extra = in.Extra
		}
		if in.Config != nil {
			out.config = in.Config
		}
		if len(in.Tools) > 0 {
			out.tools = in.Tools
		}
		if in.ToolChoice != nil {
			out.toolChoice = in.ToolChoice
		}
	}
	if len(mas) == 0 {
		out.messages = []*schema.Message{}
		return out, nil
	}
	out.messages, err = concatMessageArray(mas)
	if err != nil {
		return nil, fmt.Errorf("concat messages failed: %v", err)
	}
	return out, nil
}

func convModelCallbackOutput(out []callbacks.CallbackOutput) []*model.CallbackOutput {
	ret := make([]*model.CallbackOutput, len(out))
	for i, c := range out {
		ret[i] = model.ConvCallbackOutput(c)
	}
	return ret
}

func extractModelOutput(outs []*model.CallbackOutput) (message *schema.Message, extra map[string]interface{}, err error) {
	var mas []*schema.Message
	for _, out := range outs {
		if out == nil {
			continue
		}
		if out.Message != nil {
			mas = append(mas, out.Message)
		}
		if out.Extra != nil {
			extra = out.Extra
		}
	}
	if len(mas) == 0 {
		return &schema.Message{}, extra, nil
	}
	message, err = schema.ConcatMessages(mas)
	if err != nil {
		return nil, nil, fmt.Errorf("concat message failed: %v", err)
	}

	return message, extra, nil
}

func mapUsageDetail(usage *schema.TokenUsage) *langfuse.UsageDetail {
	if usage == nil {
		return nil
	}
	return &langfuse.UsageDetail{
		CompletionTokens: usage.CompletionTokens,
		PromptTokens:     usage.PromptTokens,
		TotalTokens:      usage.TotalTokens,
		CompletionTokensDetails: &langfuse.CompletionTokensDetails{
			ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		},
		PromptTokensDetails: &langfuse.PromptTokensDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
	}
}

func concatMessageArray(mas [][]*schema.Message) ([]*schema.Message, error) {
	arrayLen := len(mas[0])

	ret := make([]*schema.Message, arrayLen)
	slicesToConcat := make([][]*schema.Message, arrayLen)

	for _, ma := range mas {
		if len(ma) != arrayLen {
			return nil, fmt.Errorf("unexpected array length. "+
				"Got %d, expected %d", len(ma), arrayLen)
		}

		for i := 0; i < arrayLen; i++ {
			m := ma[i]
			if m != nil {
				slicesToConcat[i] = append(slicesToConcat[i], m)
			}
		}
	}

	for i, slice := range slicesToConcat {
		if len(slice) == 0 {
			ret[i] = nil
		} else if len(slice) == 1 {
			ret[i] = slice[0]
		} else {
			cm, err := schema.ConcatMessages(slice)
			if err != nil {
				return nil, err
			}

			ret[i] = cm
		}
	}

	return ret, nil
}

// openaiTool is Chat Completions tool shape for generation Input.tools
// (Langfuse Playground extractTools reads input.tools).
type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

func toolInfosToOpenAITools(tools []*schema.ToolInfo) ([]openaiTool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]openaiTool, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		var params any
		if t.ParamsOneOf != nil {
			js, err := t.ToJSONSchema()
			if err != nil {
				return nil, fmt.Errorf("convert tool %q parameters: %w", t.Name, err)
			}
			params = js
		}
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, openaiTool{
			Type: "function",
			Function: openaiToolFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  params,
			},
		})
	}
	return out, nil
}

func mapToolChoiceOpenAI(tc *schema.ToolChoice) string {
	if tc == nil {
		return "auto"
	}
	switch *tc {
	case schema.ToolChoiceForced:
		return "required"
	case schema.ToolChoiceForbidden:
		return "none"
	default:
		return "auto"
	}
}

// buildOpenAIChatInput builds generation Input as OpenAI chat request JSON:
//
//	{"messages":[...],"tools":[...],"tool_choice":"auto"}
//
// Caller must leave InMessages empty so acl consumer does not overwrite Input.
func buildOpenAIChatInput(messages []*schema.Message, tools []*schema.ToolInfo, toolChoice *schema.ToolChoice) (string, error) {
	oaiTools, err := toolInfosToOpenAITools(tools)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"messages": messages,
	}
	if len(oaiTools) > 0 {
		payload["tools"] = oaiTools
		payload["tool_choice"] = mapToolChoiceOpenAI(toolChoice)
	}
	return sonic.MarshalString(payload)
}
