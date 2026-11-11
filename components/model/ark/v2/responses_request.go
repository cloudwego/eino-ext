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
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/volcengine/ark-runtime-go/arkruntime/model/responses"
)

func (cm *ResponsesAPIChatModel) getOptions(opts ...model.Option) (*model.Options, *arkOptions) {
	return model.GetCommonOptions(&model.Options{Model: &cm.config.Model, MaxTokens: cm.config.MaxOutputTokens,
			Temperature: cm.config.Temperature, TopP: cm.config.TopP, ToolChoice: cm.toolChoice}, opts...),
		model.GetImplSpecificOptions(&arkOptions{customHeaders: cm.config.CustomHeader, responsesThinking: cm.config.Thinking,
			responsesReasoningEffort: cm.config.ReasoningEffort, parallelToolCalls: cm.config.ParallelToolCalls,
			cache: &CacheOption{SessionCache: cm.config.SessionCache}, enableWebSearch: cm.config.EnableToolWebSearch,
			maxToolCalls:           cm.config.MaxToolCalls,
			requestPayloadModifier: cm.config.RequestPayloadModifier, responseMessageModifier: cm.config.ResponseMessageModifier,
			responseChunkMessageModifier: cm.config.ResponseChunkMessageModifier}, opts...)
}

// A built request owns its wire payload and derived callback data. Caller
// configuration and parsed options remain inputs throughout construction.
type responsesRequest struct {
	payload       *responses.ResponsesRequest
	extra         map[string]interface{}
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
	if opts.parallelToolCalls != nil {
		req.ParallelToolCalls = responses.NewOptBool(*opts.parallelToolCalls)
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
	var extra map[string]interface{}
	tools, err := cm.buildResponsesTools(common, opts, req.PreviousResponseID.IsSet())
	if err != nil {
		return nil, err
	}
	req.ToolChoice = tools.choice
	if tools.definitions != nil {
		if extra == nil {
			extra = make(map[string]interface{})
		}
		extra["tools"] = tools.definitions
	}
	return &responsesRequest{payload: req, extra: extra, callbackTools: tools.callbackTools}, nil
}

func prepareResponsesPayload(ctx context.Context, in []*schema.Message, req *responses.ResponsesRequest, extra map[string]interface{}, modifier RequestPayloadModifier) (*responses.ResponsesRequest, map[string]interface{}, error) {
	if modifier == nil {
		return req, extra, nil
	}
	raw, err := sonic.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	var fields map[string]json.RawMessage
	if err = sonic.Unmarshal(raw, &fields); err != nil {
		return nil, nil, err
	}
	for key, value := range extra {
		fields[key], err = sonic.Marshal(value)
		if err != nil {
			return nil, nil, err
		}
	}
	originalInput := fields["input"]
	raw, err = sonic.Marshal(fields)
	if err != nil {
		return nil, nil, err
	}
	raw, err = modifier(ctx, in, raw)
	if err != nil {
		return nil, nil, fmt.Errorf("modify request payload: %w", err)
	}
	fields = nil
	if err = sonic.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, nil, fmt.Errorf("modified request payload must be a JSON object")
	}
	var name string
	if value, ok := fields["model"]; !ok || sonic.Unmarshal(value, &name) != nil || name == "" {
		return nil, nil, fmt.Errorf("modified request payload must contain a non-empty string model")
	}
	input, ok := fields["input"]
	if !ok || bytes.Equal(bytes.TrimSpace(input), []byte("null")) {
		return nil, nil, fmt.Errorf("modified request payload must contain input")
	}
	fields["stream"], _ = sonic.Marshal(req.Stream.Or(false))
	body := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		body[key] = value
	}
	wire := &responses.ResponsesRequest{Model: name, Stream: req.Stream}
	if bytes.Equal(input, originalInput) {
		wire.Input = req.Input
		body["input"] = responsesInputWithFiles(&wire.Input, input)
	} else {
		// Decode edited input only for SDK file preprocessing. Preserve the raw
		// JSON for unknown fields and exact numbers in the final payload.
		if err = sonic.Unmarshal(input, &wire.Input); err != nil {
			return nil, nil, fmt.Errorf("modified input: %w", err)
		}
		body["input"] = responsesInputWithFiles(&wire.Input, input)
	}
	// Cache metadata must describe the payload actually sent after modification.
	for key, target := range map[string]interface{}{"store": &wire.Store, "expire_at": &wire.ExpireAt, "caching": &wire.Caching} {
		if value, exists := fields[key]; exists {
			if err = sonic.Unmarshal(value, target); err != nil {
				return nil, nil, fmt.Errorf("modified %s: %w", key, err)
			}
		}
	}
	// These fields also describe the effective request in output callbacks.
	for key, target := range map[string]interface{}{"max_output_tokens": &wire.MaxOutputTokens, "temperature": &wire.Temperature, "top_p": &wire.TopP, "thinking": &wire.Thinking} {
		if value, exists := fields[key]; exists && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if err = sonic.Unmarshal(value, target); err != nil {
				return nil, nil, fmt.Errorf("modified %s: %w", key, err)
			}
		}
	}
	return wire, body, nil
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

// Modified JSON must retain fields the SDK does not know about, while local
// file URLs must reflect SDK preprocessing. Patch only those media sources.
type responsesFileInput struct {
	input *responses.ResponsesInput
	raw   json.RawMessage
	files []responsesFilePart
}

type responsesFilePart struct{ item, content int }

func responsesInputWithFiles(input *responses.ResponsesInput, raw json.RawMessage) interface{} {
	items, _ := input.GetInputItemArray()
	var files []responsesFilePart
	for i := range items {
		message, ok := items[i].OneOf.GetItemEasyMessage()
		if !ok {
			continue
		}
		parts, _ := message.Content.GetContentItemArray()
		for j := range parts {
			_, url, _ := responsesFileSource(parts[j])
			if strings.HasPrefix(url, "file://") {
				files = append(files, responsesFilePart{item: i, content: j})
			}
		}
	}
	if len(files) == 0 {
		return raw
	}
	return &responsesFileInput{input: input, raw: raw, files: files}
}

func (in *responsesFileInput) MarshalJSON() ([]byte, error) {
	var rawItems []json.RawMessage
	if err := sonic.Unmarshal(in.raw, &rawItems); err != nil {
		return nil, err
	}
	items, _ := in.input.GetInputItemArray()
	for _, file := range in.files {
		part := items[file.item].OneOf.ItemEasyMessage.Content.ContentItemArray[file.content]
		key, url, id := responsesFileSource(part)
		if url != "" || !id.IsSet() {
			continue
		}
		var message map[string]json.RawMessage
		if err := sonic.Unmarshal(rawItems[file.item], &message); err != nil {
			return nil, err
		}
		var parts []map[string]json.RawMessage
		if err := sonic.Unmarshal(message["content"], &parts); err != nil {
			return nil, err
		}
		delete(parts[file.content], key)
		var err error
		parts[file.content]["file_id"], err = sonic.Marshal(id.Value)
		if err != nil {
			return nil, err
		}
		message["content"], err = sonic.Marshal(parts)
		if err != nil {
			return nil, err
		}
		rawItems[file.item], err = sonic.Marshal(message)
		if err != nil {
			return nil, err
		}
	}
	return sonic.Marshal(rawItems)
}

func responsesFileSource(part responses.ContentItem) (key, url string, id responses.OptString) {
	switch part.OneOf.Type {
	case responses.ContentItemImageContentItemSum:
		image := part.OneOf.ContentItemImage
		return "image_url", image.ImageURL.Or(""), image.FileID
	case responses.ContentItemVideoContentItemSum:
		video := part.OneOf.ContentItemVideo
		return "video_url", video.VideoURL, video.FileID
	case responses.ContentItemFileContentItemSum:
		file := part.OneOf.ContentItemFile
		return "file_url", file.FileURL.Or(""), file.FileID
	default:
		return "", "", responses.OptString{}
	}
}
