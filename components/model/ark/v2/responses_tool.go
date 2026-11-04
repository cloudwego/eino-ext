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
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

// Map Eino tools directly to Responses; raw entries are needed only for strict,
// which is missing from the pinned SDK's FunctionTool definition.
func toResponsesTools(tools []*schema.ToolInfo, strict *bool) ([]responses.Tool, []json.RawMessage, error) {
	converted := make([]responses.Tool, 0, len(tools))
	stricts := make([]*bool, 0, len(tools))
	hasStrict := false
	for _, ti := range tools {
		if ti == nil {
			return nil, nil, fmt.Errorf("tool info cannot be nil")
		}
		function := responses.FunctionTool{Type: responses.FunctionToolTypeFunction, Name: ti.Name}
		if ti.Desc != "" {
			function.Description = responses.NewOptString(ti.Desc)
		}
		setting, err := toolStrict(ti, strict)
		if err != nil {
			return nil, nil, err
		}
		stricts = append(stricts, setting)
		hasStrict = hasStrict || setting != nil
		data, err := toolParametersJSON(ti)
		if err != nil {
			return nil, nil, err
		}
		if data != nil {
			var params responses.FunctionToolParameters
			if err = sonic.Unmarshal(data, &params); err != nil {
				return nil, nil, fmt.Errorf("tool %s schema object: %w", ti.Name, err)
			}
			function.Parameters = responses.NewOptFunctionToolParameters(params)
		}
		converted = append(converted, responses.Tool{OneOf: responses.NewFunctionToolToolSum(function)})
	}
	if !hasStrict {
		return converted, nil, nil
	}
	rawTools := make([]json.RawMessage, 0, len(converted))
	for i := range converted {
		raw, err := sonic.Marshal(&converted[i].OneOf.FunctionTool)
		if err != nil {
			return nil, nil, fmt.Errorf("serialize Responses function: %w", err)
		}
		if stricts[i] != nil {
			var fields map[string]json.RawMessage
			if err = sonic.Unmarshal(raw, &fields); err != nil {
				return nil, nil, err
			}
			fields["strict"], _ = sonic.Marshal(*stricts[i])
			raw, err = sonic.Marshal(fields)
			if err != nil {
				return nil, nil, err
			}
		}
		rawTools = append(rawTools, raw)
	}
	return converted, rawTools, nil
}

func populateResponsesToolChoice(req *responses.ResponsesRequest, choice *schema.ToolChoice) error {
	if choice == nil {
		return nil
	}
	mode := responses.ToolChoiceOptionsAuto
	switch *choice {
	case schema.ToolChoiceForbidden:
		mode = responses.ToolChoiceOptionsNone
	case schema.ToolChoiceForced:
		if len(req.Tools) == 0 {
			return fmt.Errorf("tool_choice is forced but no tools are provided")
		}
		if len(req.Tools) == 1 && req.Tools[0].OneOf.FunctionTool.Name != "" {
			req.ToolChoice = responses.NewOptToolChoice(responses.NewTypedToolChoiceToolChoice(responses.TypedToolChoice{
				OneOf: responses.NewToolChoiceFunctionTypedToolChoiceSum(responses.ToolChoiceFunction{
					Type: responses.ToolChoiceFunctionTypeFunction, Name: req.Tools[0].OneOf.FunctionTool.Name,
				}),
			}))
			return nil
		}
		mode = responses.ToolChoiceOptionsRequired
	}
	req.ToolChoice = responses.NewOptToolChoice(responses.NewToolChoiceOptionsToolChoice(mode))
	return nil
}
