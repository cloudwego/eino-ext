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
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func (cm *ResponsesAPIChatModel) getOptions(opts ...model.Option) (*model.Options, *arkOptions) {
	return model.GetCommonOptions(&model.Options{Model: &cm.config.Model, MaxTokens: cm.config.MaxOutputTokens,
			Temperature: cm.config.Temperature, TopP: cm.config.TopP, ToolChoice: cm.toolChoice}, opts...),
		model.GetImplSpecificOptions(&arkOptions{customHeaders: cm.config.CustomHeader, responsesThinking: cm.config.Thinking,
			responsesReasoningEffort: cm.config.ReasoningEffort,
			cache:                    &CacheOption{SessionCache: cm.config.SessionCache}, enableWebSearch: cm.config.EnableToolWebSearch,
			maxToolCalls: cm.config.MaxToolCalls}, opts...)
}

// A built request owns its wire payload and derived callback data. Caller
// configuration and parsed options remain inputs throughout construction.
type responsesRequest struct {
	payload       *responses.ResponsesRequest
	callbackTools []*schema.ToolInfo
}

func (cm *ResponsesAPIChatModel) genRequest(in []*schema.Message, common *model.Options, opts *arkOptions) (*responsesRequest, error) {
	if len(common.Stop) > 0 {
		return nil, fmt.Errorf("Responses API does not support stop sequences")
	}
	req := &responses.ResponsesRequest{Model: dereferenceOrZero(common.Model), Store: responses.NewOptBool(false)}
	if common.MaxTokens != nil {
		if *common.MaxTokens <= 0 {
			return nil, fmt.Errorf("max_output_tokens must be positive")
		}
		req.MaxOutputTokens = responses.NewOptInt64(int64(*common.MaxTokens))
	}
	if common.Temperature != nil {
		req.Temperature = responses.NewOptFloat64(float64FromFloat32(*common.Temperature))
	}
	if common.TopP != nil {
		req.TopP = responses.NewOptFloat64(float64FromFloat32(*common.TopP))
	}
	if opts.responsesThinking != nil {
		req.Thinking = responses.NewOptThinking(*opts.responsesThinking)
	}
	if opts.responsesReasoningEffort != nil {
		req.Reasoning = responses.NewOptReasoning(responses.Reasoning{Effort: responses.NewOptReasoningEffort(*opts.responsesReasoningEffort)})
	}
	if opts.maxToolCalls != nil {
		req.MaxToolCalls = responses.NewOptInt64(*opts.maxToolCalls)
	}
	if cm.config.ServiceTier != nil {
		req.ServiceTier = responses.NewOptServiceTier(responses.ServiceTier(*cm.config.ServiceTier))
	}
	if cm.config.ResponseFormat != nil {
		req.Text = responses.NewOptResponseTextConfig(responses.ResponseTextConfig{Format: *cm.config.ResponseFormat})
	}
	var err error
	in, err = populateResponsesCache(in, req, opts.cache)
	if err != nil {
		return nil, err
	}
	items, err := toResponsesInput(in)
	if err != nil {
		return nil, err
	}
	req.Input = responses.NewInputItemArrayResponsesInput(items)
	tools, err := cm.buildResponsesTools(common, opts, req.PreviousResponseID.IsSet())
	if err != nil {
		return nil, err
	}
	req.Tools, req.ToolChoice = tools.definitions, tools.choice
	return &responsesRequest{payload: req, callbackTools: tools.callbackTools}, nil
}

func responsesCallbackConfig(req *responses.ResponsesRequest) *model.Config {
	return &model.Config{Model: req.Model, MaxTokens: int(req.MaxOutputTokens.Or(0)),
		Temperature: float32(req.Temperature.Or(0)), TopP: float32(req.TopP.Or(0))}
}

func responsesCallbackThinking(req *responses.ResponsesRequest) *responses.Thinking {
	if !req.Thinking.IsSet() {
		return nil
	}
	return &req.Thinking.Value
}

func responsesHeaders(opts *arkOptions) map[string]string {
	if opts.enableWebSearch == nil {
		return opts.customHeaders
	}
	headers := make(map[string]string, len(opts.customHeaders)+1)
	hasBeta := false
	for key, value := range opts.customHeaders {
		headers[key] = value
		if strings.EqualFold(key, "ark-beta-web-search") {
			hasBeta = true
		}
	}
	if !hasBeta {
		headers["ark-beta-web-search"] = "true"
	}
	return headers
}

// Tool definitions have one wire representation. Callback tools are local
// function metadata; they cannot describe an arbitrary server-side cache.
type responsesTools struct {
	definitions   []responses.Tool
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
	definitions, err := toResponsesTools(tools, search)
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

func toResponsesTools(tools []*schema.ToolInfo, search *responses.WebSearchTool) ([]responses.Tool, error) {
	if tools == nil && search == nil {
		return nil, nil
	}
	wireTools := make([]responses.Tool, 0, len(tools)+1)
	for _, ti := range tools {
		if ti == nil {
			return nil, fmt.Errorf("tool info cannot be nil")
		}
		data, err := toolParametersJSON(ti)
		if err != nil {
			return nil, err
		}
		function := responses.FunctionTool{Type: responses.FunctionToolTypeFunction, Name: ti.Name}
		if ti.Desc != "" {
			function.Description = responses.NewOptString(ti.Desc)
		}
		if len(data) > 0 && !bytes.Equal(data, []byte("null")) {
			if err := function.Parameters.UnmarshalJSON(data); err != nil {
				return nil, fmt.Errorf("tool %s schema must be an object: %w", ti.Name, err)
			}
		}
		wireTools = append(wireTools, responses.Tool{OneOf: responses.NewFunctionToolToolSum(function)})
	}
	if search != nil {
		tool := *search
		tool.Type = responses.WebSearchToolTypeWebSearch
		wireTools = append(wireTools, responses.Tool{OneOf: responses.NewWebSearchToolToolSum(tool)})
	}
	return wireTools, nil
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
