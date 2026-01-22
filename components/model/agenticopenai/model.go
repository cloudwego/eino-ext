/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agenticopenai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/openai/openai-go/v3/azure"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

var _ model.AgenticModel = (*Model)(nil)

type Config struct {
	// ByAzure specifies whether to use Azure OpenAI service.
	// Optional.
	ByAzure bool

	// BaseURL specifies the base URL for the OpenAI service endpoint.
	// Optional.
	BaseURL string

	// APIKey specifies the API key for authentication.
	// Required.
	APIKey string

	// Timeout specifies the maximum duration to wait for API responses.
	// Optional.
	Timeout *time.Duration

	// HTTPClient specifies the HTTP client used to send requests.
	// Optional.
	HTTPClient *http.Client

	// MaxRetries specifies the maximum number of retry attempts for failed requests.
	// Optional.
	MaxRetries *int

	// Model specifies the ID of the model to use for the response.
	// Required.
	Model string

	// MaxTokens specifies the maximum number of tokens to generate in the response.
	// Optional.
	MaxTokens *int

	// Temperature controls the randomness of the model's output.
	// Higher values (e.g., 0.8) make the output more random, while lower values (e.g., 0.2) make it more focused and deterministic.
	// Range: 0.0 to 2.0.
	// Optional.
	Temperature *float32

	// TopP controls diversity via nucleus sampling.
	// It specifies the cumulative probability threshold for token selection.
	// Recommended to use either Temperature or TopP, but not both.
	// Range: 0.0 to 1.0.
	// Optional.
	TopP *float32

	// ServiceTier specifies the latency tier for processing the request.
	// Optional.
	ServiceTier *responses.ResponseNewParamsServiceTier

	// Text specifies configuration for text generation output.
	// Optional.
	Text *responses.ResponseTextConfigParam

	// Reasoning specifies configuration for reasoning models.
	// Optional.
	Reasoning *responses.ReasoningParam

	// Store specifies whether to store the response on the server.
	// Optional.
	Store *bool

	// MaxToolCalls specifies the maximum number of tool calls allowed in a single turn.
	// Optional.
	MaxToolCalls *int

	// ParallelToolCalls specifies whether to allow multiple tool calls in a single turn.
	// Optional.
	ParallelToolCalls *bool

	// Include specifies a list of additional fields to include in the response.
	// Optional.
	Include []responses.ResponseIncludable

	// ServerTools specifies server-side tools available to the model.
	// Optional.
	ServerTools []*ServerToolConfig

	// MCPTools specifies Model Context Protocol tools available to the model.
	// Optional.
	MCPTools []*responses.ToolMcpParam

	// CustomHeader specifies custom HTTP headers to include in API requests.
	// CustomHeader allows passing additional metadata or authentication information.
	// Optional.
	CustomHeader map[string]string

	// ExtraFields specifies additional fields that will be directly added to the HTTP request body.
	// This allows for vendor-specific or future parameters not yet explicitly supported.
	// Optional.
	ExtraFields map[string]any
}

type ServerToolConfig struct {
	WebSearch *responses.WebSearchToolParam
}

func New(_ context.Context, config *Config) (*Model, error) {
	if config == nil {
		config = &Config{}
	}

	c, err := buildClient(config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func buildClient(config *Config) (*Model, error) {
	var opts []option.RequestOption

	if config.Timeout != nil {
		opts = append(opts, option.WithRequestTimeout(*config.Timeout))
	}
	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if config.APIKey != "" {
		if config.ByAzure {
			opts = append(opts, azure.WithAPIKey(config.APIKey))
		} else {
			opts = append(opts, option.WithAPIKey(config.APIKey))
		}
	}
	if config.MaxRetries != nil {
		opts = append(opts, option.WithMaxRetries(*config.MaxRetries))
	}

	client := responses.NewResponseService(opts...)

	cm := &Model{
		cli:               client,
		model:             config.Model,
		maxTokens:         config.MaxTokens,
		temperature:       config.Temperature,
		topP:              config.TopP,
		serviceTier:       config.ServiceTier,
		text:              config.Text,
		reasoning:         config.Reasoning,
		store:             config.Store,
		maxToolCalls:      config.MaxToolCalls,
		parallelToolCalls: config.ParallelToolCalls,
		include:           config.Include,
		serverTools:       config.ServerTools,
		mcpTools:          config.MCPTools,
		customHeader:      config.CustomHeader,
		extraFields:       config.ExtraFields,
	}

	return cm, nil
}

type Model struct {
	cli responses.ResponseService

	rawFunctionTools []*schema.ToolInfo
	functionTools    []responses.ToolUnionParam

	model             string
	maxTokens         *int
	temperature       *float32
	topP              *float32
	serviceTier       *responses.ResponseNewParamsServiceTier
	text              *responses.ResponseTextConfigParam
	reasoning         *responses.ReasoningParam
	store             *bool
	maxToolCalls      *int
	parallelToolCalls *bool
	include           []responses.ResponseIncludable
	serverTools       []*ServerToolConfig
	mcpTools          []*responses.ToolMcpParam

	customHeader map[string]string
	extraFields  map[string]any
}

func (m *Model) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (
	outMsg *schema.AgenticMessage, err error) {

	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfAgenticModel)

	options, specOptions, err := m.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, reqOpts, err := m.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate request: %w", err)
	}

	config := toCallbackConfig(req)

	tools := m.rawFunctionTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    tools,
		Config:   config,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	responseObject, err := m.cli.New(ctx, *req, reqOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	outMsg, err = toOutputMessage(responseObject)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output to message: %w", err)
	}

	callbacks.OnEnd(ctx, &model.AgenticCallbackOutput{
		Message:    outMsg,
		Config:     config,
		TokenUsage: toModelTokenUsage(outMsg.ResponseMeta),
	})

	return outMsg, nil
}

func (m *Model) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (
	outStream *schema.StreamReader[*schema.AgenticMessage], err error) {

	ctx = callbacks.EnsureRunInfo(ctx, m.GetType(), components.ComponentOfAgenticModel)

	options, specOptions, err := m.getOptions(opts)
	if err != nil {
		return nil, err
	}

	req, reqOpts, err := m.genRequestAndOptions(input, options, specOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to generate request: %w", err)
	}

	config := toCallbackConfig(req)
	tools := m.rawFunctionTools
	if options.Tools != nil {
		tools = options.Tools
	}

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    tools,
		Config:   config,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	respStreamReader := m.cli.NewStreaming(ctx, *req, reqOpts...)

	sr, sw := schema.Pipe[*model.AgenticCallbackOutput](1)

	go func() {
		defer func() {
			pe := recover()
			if pe != nil {
				_ = sw.Send(nil, newPanicErr(pe, debug.Stack()))
			}

			_ = respStreamReader.Close()
			sw.Close()
		}()

		receivedStreamingResponse(respStreamReader, config, sw)

	}()

	ctx, nsr := callbacks.OnEndWithStreamOutput(ctx, schema.StreamReaderWithConvert(sr,
		func(src *model.AgenticCallbackOutput) (callbacks.CallbackOutput, error) {
			if src.Extra == nil {
				src.Extra = make(map[string]any)
			}
			return src, nil
		},
	))

	outStream = schema.StreamReaderWithConvert(nsr,
		func(src callbacks.CallbackOutput) (*schema.AgenticMessage, error) {
			s := src.(*model.AgenticCallbackOutput)
			if s.Message == nil {
				return nil, schema.ErrNoValue
			}
			return s.Message, nil
		},
	)

	return outStream, err
}

func (m *Model) WithTools(functionTools []*schema.ToolInfo) (model.AgenticModel, error) {
	if len(functionTools) == 0 {
		return nil, errors.New("function tools are required")
	}

	fts, err := toFunctionTools(functionTools)
	if err != nil {
		return nil, fmt.Errorf("failed to convert function tools: %w", err)
	}

	m_ := *m
	m_.rawFunctionTools = functionTools
	m_.functionTools = fts

	return &m_, nil
}

func (m *Model) GetType() string {
	return implType
}

func (m *Model) IsCallbacksEnabled() bool {
	return true
}

type CacheInfo struct {
	// ResponseID return by ResponsesAPI, it's specifies the id of prefix that can be used with [WithCache.HeadPreviousResponseID] option.
	ResponseID string
	// Usage specifies the token usage of prefix
	Usage schema.TokenUsage
}

func toCallbackConfig(req *responses.ResponseNewParams) *model.AgenticConfig {
	return &model.AgenticConfig{
		Model:       req.Model,
		Temperature: float32(req.Temperature.Value),
		TopP:        float32(req.TopP.Value),
	}
}

func toFunctionTools(functionTools []*schema.ToolInfo) ([]responses.ToolUnionParam, error) {
	tools := make([]responses.ToolUnionParam, len(functionTools))
	for i := range functionTools {
		ti := functionTools[i]

		paramsJSONSchema, err := ti.ParamsOneOf.ToJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool parameters to JSON schema: %w", err)
		}

		b, err := sonic.Marshal(paramsJSONSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON schema: %w", err)
		}

		var params map[string]any
		err = sonic.Unmarshal(b, &params)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON schema: %w", err)
		}

		tools[i] = responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        ti.Name,
				Description: newOpenaiStrOpt(ti.Desc),
				Parameters:  params,
			},
		}
	}

	return tools, nil
}

func toServerTools(serverTools []*ServerToolConfig) ([]responses.ToolUnionParam, error) {
	tools := make([]responses.ToolUnionParam, len(serverTools))

	for i := range serverTools {
		ti := serverTools[i]
		switch {
		case ti.WebSearch != nil:
			tools[i] = responses.ToolUnionParam{
				OfWebSearch: ti.WebSearch,
			}

		default:
			return nil, fmt.Errorf("unknown server tool type")
		}
	}

	return tools, nil
}

func (m *Model) getOptions(opts []model.Option) (*model.Options, *openaiOptions, error) {
	options := model.GetCommonOptions(&model.Options{
		Temperature: m.temperature,
		Model:       &m.model,
		TopP:        m.topP,
		MaxTokens:   m.maxTokens,
		Tools:       nil,
	}, opts...)

	specOptions := model.GetImplSpecificOptions(&openaiOptions{
		reasoning:         m.reasoning,
		store:             m.store,
		text:              m.text,
		maxToolCalls:      m.maxToolCalls,
		parallelToolCalls: m.parallelToolCalls,
		serverTools:       m.serverTools,
		mcpTools:          m.mcpTools,
		customHeaders:     m.customHeader,
	}, opts...)

	err := m.checkOptions(options)
	if err != nil {
		return options, specOptions, err
	}

	return options, specOptions, nil
}

func (m *Model) checkOptions(mOpts *model.Options) error {
	if mOpts.Stop != nil {
		return fmt.Errorf("'Stop' option is not supported")
	}
	if mOpts.ToolChoice != nil {
		return fmt.Errorf("'ToolChoice' option is not supported")
	}
	return nil
}

func (m *Model) genRequestAndOptions(in []*schema.AgenticMessage, options *model.Options,
	specOptions *openaiOptions) (req *responses.ResponseNewParams, reqOpts []option.RequestOption, err error) {

	req = &responses.ResponseNewParams{}

	err = m.prePopulateConfig(req, options, specOptions)
	if err != nil {
		return req, nil, fmt.Errorf("failed to pre-populate config: %w", err)
	}

	err = m.populateInput(in, req)
	if err != nil {
		return req, nil, fmt.Errorf("failed to populate input: %w", err)
	}

	err = m.populateTools(req, options, specOptions)
	if err != nil {
		return req, nil, fmt.Errorf("failed to populate tools: %w", err)
	}

	err = m.populateToolChoice(req, options)
	if err != nil {
		return req, nil, fmt.Errorf("failed to populate tool choice: %w", err)
	}

	for k, v := range specOptions.customHeaders {
		reqOpts = append(reqOpts, option.WithHeaderAdd(k, v))
	}

	for k, v := range specOptions.extraFields {
		reqOpts = append(reqOpts, option.WithJSONSet(k, v))
	}

	return req, reqOpts, nil
}

func (m *Model) prePopulateConfig(responseReq *responses.ResponseNewParams, options *model.Options,
	specOptions *openaiOptions) error {

	// instance configuration
	if m.serviceTier != nil {
		responseReq.ServiceTier = *m.serviceTier
	}
	responseReq.Include = m.include

	// options configuration
	if options.TopP != nil {
		responseReq.TopP = newOpenaiOpt(ptrOf(float64(*options.TopP)))
	}
	if options.Temperature != nil {
		responseReq.Temperature = newOpenaiOpt(ptrOf(float64(*options.Temperature)))
	}
	if options.Model != nil {
		responseReq.Model = *options.Model
	}
	var maxTokens *int64
	if options.MaxTokens != nil {
		maxTokens = ptrOf(int64(*options.MaxTokens))
	}
	responseReq.MaxOutputTokens = newOpenaiOpt(maxTokens)

	// specific options configuration
	if specOptions.reasoning != nil {
		responseReq.Reasoning = *specOptions.reasoning
	}
	if specOptions.text != nil {
		responseReq.Text = *specOptions.text
	}
	if specOptions.maxToolCalls != nil {
		responseReq.MaxToolCalls = param.NewOpt(int64(*specOptions.maxToolCalls))
	}
	responseReq.ParallelToolCalls = newOpenaiOpt(specOptions.parallelToolCalls)
	responseReq.PromptCacheKey = newOpenaiOpt(specOptions.promptCacheKey)
	responseReq.Store = newOpenaiOpt(specOptions.store)

	return nil
}

func (m *Model) populateInput(in []*schema.AgenticMessage, responseReq *responses.ResponseNewParams) (err error) {
	if len(in) == 0 {
		return nil
	}

	itemList := make([]responses.ResponseInputItemUnionParam, 0, len(in))

	for _, msg := range in {
		var inputItems []responses.ResponseInputItemUnionParam

		switch msg.Role {
		case schema.AgenticRoleTypeUser:
			inputItems, err = toUserRoleInputItems(msg)
			if err != nil {
				return err
			}

		case schema.AgenticRoleTypeAssistant:
			inputItems, err = toAssistantRoleInputItems(msg)
			if err != nil {
				return err
			}

		case schema.AgenticRoleTypeSystem:
			inputItems, err = toSystemRoleInputItems(msg)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid role in message: %s", msg.Role)
		}

		itemList = append(itemList, inputItems...)
	}

	responseReq.Input = responses.ResponseNewParamsInputUnion{
		OfInputItemList: itemList,
	}

	return nil
}

func (m *Model) populateToolChoice(responseReq *responses.ResponseNewParams, options *model.Options) (err error) {
	toolChoice := options.AgenticToolChoice
	if toolChoice == nil {
		return nil
	}

	switch toolChoice.Type {
	case schema.ToolChoiceForbidden:
		responseReq.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsNone),
		}

	case schema.ToolChoiceAllowed:
		if toolChoice.Allowed == nil || toolChoice.Allowed.Tools == nil {
			responseReq.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
				OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsAuto),
			}
			return nil
		}
		tools, err := toAllowedTools(toolChoice.Allowed.Tools)
		if err != nil {
			return err
		}
		responseReq.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
			OfAllowedTools: &responses.ToolChoiceAllowedParam{
				Mode:  responses.ToolChoiceAllowedModeAuto,
				Tools: tools,
			},
		}

	case schema.ToolChoiceForced:
		if toolChoice.Forced == nil || toolChoice.Forced.Tools == nil {
			responseReq.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
				OfToolChoiceMode: param.NewOpt(responses.ToolChoiceOptionsRequired),
			}
			return nil
		}
		tools, err := toAllowedTools(toolChoice.Forced.Tools)
		if err != nil {
			return err
		}
		responseReq.ToolChoice = responses.ResponseNewParamsToolChoiceUnion{
			OfAllowedTools: &responses.ToolChoiceAllowedParam{
				Mode:  responses.ToolChoiceAllowedModeRequired,
				Tools: tools,
			},
		}

	default:
		return fmt.Errorf("invalid tool choice: %s", *options.ToolChoice)
	}

	return nil
}

func toAllowedTools(tools []*schema.AllowedTool) ([]map[string]any, error) {
	allowedTools := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		switch {
		case tool.FunctionName != "":
			allowedTools = append(allowedTools, map[string]any{
				"type": "function",
				"name": tool.FunctionName,
			})

		case tool.MCPTool != nil:
			tool_ := map[string]any{
				"type":         "mcp",
				"server_label": tool.MCPTool.ServerLabel,
			}
			if tool.MCPTool.Name != "" {
				tool_["name"] = tool.MCPTool.Name
			}
			allowedTools = append(allowedTools, tool_)

		case tool.ServerTool != nil:
			allowedTools = append(allowedTools, map[string]any{
				"type": tool.ServerTool.Name,
			})

		default:
			return nil, fmt.Errorf("unknown allowed tool type")
		}
	}

	return allowedTools, nil
}

func (m *Model) populateTools(responseReq *responses.ResponseNewParams, options *model.Options, specOptions *openaiOptions) (err error) {
	var functionTools []responses.ToolUnionParam
	if options.Tools != nil {
		functionTools, err = toFunctionTools(options.Tools)
		if err != nil {
			return err
		}
	} else {
		functionTools = m.functionTools
	}

	responseReq.Tools = append(responseReq.Tools, functionTools...)

	serverTools, err := toServerTools(specOptions.serverTools)
	if err != nil {
		return err
	}

	responseReq.Tools = append(responseReq.Tools, serverTools...)

	if len(specOptions.mcpTools) > 0 {
		mcpTools := make([]responses.ToolUnionParam, 0, len(specOptions.mcpTools))
		for _, tool := range specOptions.mcpTools {
			mcpTools = append(mcpTools, responses.ToolUnionParam{
				OfMcp: tool,
			})
		}
		responseReq.Tools = append(responseReq.Tools, mcpTools...)
	}

	return nil
}

func toModelTokenUsage(meta *schema.AgenticResponseMeta) *model.TokenUsage {
	if meta == nil || meta.TokenUsage == nil {
		return nil
	}

	usage := meta.TokenUsage

	return &model.TokenUsage{
		PromptTokens: usage.PromptTokens,
		PromptTokenDetails: model.PromptTokenDetails{
			CachedTokens: usage.PromptTokenDetails.CachedTokens,
		},
		CompletionTokens: usage.CompletionTokens,
		CompletionTokensDetails: model.CompletionTokensDetails{
			ReasoningTokens: usage.CompletionTokensDetails.ReasoningTokens,
		},
		TotalTokens: usage.TotalTokens,
	}
}
