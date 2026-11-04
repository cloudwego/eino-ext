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

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/chat"
)

type chatTools struct {
	definitions   []chat.ChatCompletionTool
	choice        chat.OptChatCompletionToolChoice
	callbackTools []*schema.ToolInfo
}

// Select before encoding so wire definitions and callback metadata describe
// the same tools. The model and both option sets remain read-only.
func (cm *ChatModel) buildChatTools(common *model.Options) (chatTools, error) {
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
