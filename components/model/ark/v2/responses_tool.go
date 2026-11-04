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
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Tool definitions have one wire representation. Callback tools are local
// function metadata; they cannot describe an arbitrary server-side cache.
type responsesTools struct {
	definitions   json.RawMessage
	choice        responses.OptToolChoice
	callbackTools []*schema.ToolInfo
}

// Build tool settings from read-only configuration and options. Cached calls
// inherit their original definitions, including the initial name restriction.
func (cm *ResponsesAPIChatModel) buildResponsesTools(common *model.Options, opts *arkOptions, inherited bool) (responsesTools, error) {
	tools := cm.rawTools
	if common.Tools != nil {
		tools = common.Tools
	}
	named := common.ToolChoice != nil && len(common.AllowedToolNames) > 0 && (*common.ToolChoice == schema.ToolChoiceAllowed || *common.ToolChoice == schema.ToolChoiceForced)
	if inherited {
		if named {
			tools = responsesCallbackTools(tools, common.AllowedToolNames)
		}
		// The cache service rejects tool_choice as well as tools on a
		// continuation. Keep options read-only and let it use the saved context.
		return responsesTools{callbackTools: tools}, nil
	}

	tools, err := selectToolInfos(tools, common.ToolChoice, common.AllowedToolNames)
	if err != nil {
		return responsesTools{}, err
	}
	search := opts.enableWebSearch
	if named {
		search = nil
	}
	if common.ToolChoice != nil && *common.ToolChoice == schema.ToolChoiceForced && len(tools) == 0 && search == nil {
		return responsesTools{}, fmt.Errorf("tool_choice is forced but no tools are provided")
	}
	choice := toResponsesToolChoice(common.ToolChoice, common.AllowedToolNames)
	definitions, err := toResponsesTools(tools, search, cm.config.ToolStrict)
	if err != nil {
		return responsesTools{}, err
	}
	return responsesTools{definitions: definitions, choice: choice, callbackTools: tools}, nil
}

// Cached functions need no local definition. Match any available metadata
// without validating schemas or treating absent local names as unknown tools.
func responsesCallbackTools(tools []*schema.ToolInfo, names []string) []*schema.ToolInfo {
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	var selected []*schema.ToolInfo
	for _, tool := range tools {
		if tool != nil {
			if _, ok := allowed[tool.Name]; ok {
				selected = append(selected, tool)
			}
		}
	}
	return selected
}

// The pinned SDK's function tool lacks strict. Encode every function using this
// representation so strict does not select a different serialization path.
type responsesFunctionTool struct {
	Type        responses.FunctionToolType `json:"type"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Parameters  json.RawMessage            `json:"parameters,omitempty"`
	Strict      *bool                      `json:"strict,omitempty"`
}

func toResponsesTools(tools []*schema.ToolInfo, search *responses.WebSearchTool, strict *bool) (json.RawMessage, error) {
	if tools == nil && search == nil {
		return nil, nil
	}
	wireTools := make([]interface{}, 0, len(tools)+1)
	for _, ti := range tools {
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil")
		}
		setting, err := toolStrict(ti, strict)
		if err != nil {
			return nil, err
		}
		data, err := toolParametersJSON(ti)
		if err != nil {
			return nil, err
		}
		// Preserve the SDK's object-or-null contract without decoding schema
		// JSON into a second representation.
		if len(data) > 0 && data[0] != '{' && !bytes.Equal(data, []byte("null")) {
			return nil, fmt.Errorf("tool %s schema must be an object", ti.Name)
		}
		wireTools = append(wireTools, &responsesFunctionTool{
			Type: responses.FunctionToolTypeFunction, Name: ti.Name,
			Description: ti.Desc, Parameters: data, Strict: setting,
		})
	}
	if search != nil {
		tool := *search
		tool.Type = responses.WebSearchToolTypeWebSearch
		wireTools = append(wireTools, &tool)
	}
	raw, err := sonic.Marshal(wireTools)
	if err != nil {
		return nil, fmt.Errorf("serialize Responses tools: %w", err)
	}
	return json.RawMessage(raw), nil
}

func toResponsesToolChoice(choice *schema.ToolChoice, names []string) responses.OptToolChoice {
	if choice == nil {
		return responses.OptToolChoice{}
	}
	mode := responses.ToolChoiceOptionsAuto
	switch *choice {
	case schema.ToolChoiceForbidden:
		mode = responses.ToolChoiceOptionsNone
	case schema.ToolChoiceForced:
		if len(names) == 1 {
			return responses.NewOptToolChoice(responses.NewTypedToolChoiceToolChoice(responses.TypedToolChoice{
				OneOf: responses.NewToolChoiceFunctionTypedToolChoiceSum(responses.ToolChoiceFunction{
					Type: responses.ToolChoiceFunctionTypeFunction, Name: names[0],
				}),
			}))
		}
		mode = responses.ToolChoiceOptionsRequired
	}
	return responses.NewOptToolChoice(responses.NewToolChoiceOptionsToolChoice(mode))
}
