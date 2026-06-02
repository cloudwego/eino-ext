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
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/openai/openai-go/v3/azure"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
)

var _ model.AgenticModel = (*ResponsesModel)(nil)

type ResponsesConfig struct {
	// ByAzure specifies whether to use Azure OpenAI service.
	// Optional.
	ByAzure bool

	// BaseURL specifies the base URL for the OpenAI service endpoint.
	// Optional. Default: https://api.openai.com/v1
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

	// Truncation specifies how to handle context that exceeds the model's context window.
	// Optional.
	Truncation *responses.ResponseNewParamsTruncation

	// EnableAutoCache controls whether auto-caching for multi-turn conversations is active for the model.
	// When enabled, conversation turns are stored, and the model automatically maintains context
	// by locating the most recent cached message in the input (via Response ID in ResponseMeta).
	// This cached message and all preceding inputs are excluded from the request.
	// If the cached message becomes invalid, you can call InvalidateMessageCaches to temporarily invalidate the cache.
	// Unless Store is set to false explicitly for a request, auto-cache forces Store to true.
	// Optional.
	EnableAutoCache bool

	// PromptCacheRetention specifies the retention policy for the prompt cache.
	// Optional.
	PromptCacheRetention *responses.ResponseNewParamsPromptCacheRetention

	// CustomHeaders specifies custom HTTP headers to include in API requests.
	// CustomHeaders allows passing additional metadata or authentication information.
	// Optional.
	CustomHeaders map[string]string

	// ExtraFields specifies extra fields to include in the request body.
	// These fields will be merged into the top-level JSON request body, overriding any existing fields with the same key.
	// Optional.
	//
	// Example:
	//
	//	ExtraFields: map[string]any{
	//	    "reasoning_effort": "high",
	//	    "service_tier": "default",
	//	}
	//
	// The resulting request body will be:
	//
	//	{
	//	    "model": "o1",
	//	    "messages": [...],
	//	    "reasoning_effort": "high",
	//	    "service_tier": "default"
	//	}
	ExtraFields map[string]any
}

type ResponsesServerToolConfig struct {
	WebSearch       *responses.WebSearchToolParam
	FileSearch      *responses.FileSearchToolParam
	CodeInterpreter *responses.ToolCodeInterpreterParam
	Shell           *responses.FunctionShellToolParam
}

func NewResponsesModel(_ context.Context, config *ResponsesConfig) (*ResponsesModel, error) {
	if config == nil {
		config = &ResponsesConfig{}
	}

	c, err := buildClient(config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func buildClient(config *ResponsesConfig) (*ResponsesModel, error) {
	var opts []option.RequestOption

	if config.Timeout != nil {
		opts = append(opts, option.WithRequestTimeout(*config.Timeout))
	}
	if config.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(config.HTTPClient))
	}
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	opts = append(opts, option.WithBaseURL(baseURL))
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

	cm := &ResponsesModel{
		cli:                  client,
		model:                config.Model,
		maxTokens:            config.MaxTokens,
		temperature:          config.Temperature,
		topP:                 config.TopP,
		serviceTier:          config.ServiceTier,
		text:                 config.Text,
		reasoning:            config.Reasoning,
		store:                config.Store,
		maxToolCalls:         config.MaxToolCalls,
		parallelToolCalls:    config.ParallelToolCalls,
		include:              config.Include,
		truncation:           config.Truncation,
		enableAutoCache:      config.EnableAutoCache,
		promptCacheRetention: config.PromptCacheRetention,
		customHeader:         config.CustomHeaders,
		extraFields:          config.ExtraFields,
	}

	return cm, nil
}

type ResponsesModel struct {
	cli responses.ResponseService

	model                string
	maxTokens            *int
	temperature          *float32
	topP                 *float32
	serviceTier          *responses.ResponseNewParamsServiceTier
	text                 *responses.ResponseTextConfigParam
	reasoning            *responses.ReasoningParam
	store                *bool
	maxToolCalls         *int
	parallelToolCalls    *bool
	include              []responses.ResponseIncludable
	truncation           *responses.ResponseNewParamsTruncation
	enableAutoCache      bool
	promptCacheRetention *responses.ResponseNewParamsPromptCacheRetention

	customHeader map[string]string
	extraFields  map[string]any
}

func (m *ResponsesModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (
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

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    options.Tools,
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

	outMsg, err = toOutputMessage(responseObject, options)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output to message: %w", err)
	}

	if m.enableAutoCache {
		setAutoCached(outMsg)
	}

	callbacks.OnEnd(ctx, &model.AgenticCallbackOutput{
		Message:    outMsg,
		Config:     config,
		TokenUsage: toModelTokenUsage(outMsg.ResponseMeta),
	})

	return outMsg, nil
}

func (m *ResponsesModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (
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

	ctx = callbacks.OnStart(ctx, &model.AgenticCallbackInput{
		Messages: input,
		Tools:    options.Tools,
		Config:   config,
	})

	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	respStreamReader := m.cli.NewStreaming(ctx, *req, reqOpts...)
	if err = respStreamReader.Err(); err != nil {
		return nil, fmt.Errorf("failed to create streaming response: %w", err)
	}

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

		receivedStreamingResponse(respStreamReader, config, sw, options)

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
			if m.enableAutoCache {
				setAutoCached(s.Message)
			}
			return s.Message, nil
		},
	)

	return outStream, err
}

func (m *ResponsesModel) GetType() string {
	return responsesImplType
}

func (m *ResponsesModel) IsCallbacksEnabled() bool {
	return true
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
		ft, err := toFunctionTool(functionTools[i])
		if err != nil {
			return nil, err
		}
		tools[i] = responses.ToolUnionParam{
			OfFunction: ft,
		}
	}
	return tools, nil
}

func toFunctionTool(ti *schema.ToolInfo) (*responses.FunctionToolParam, error) {
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

	return &responses.FunctionToolParam{
		Name:        ti.Name,
		Description: newOpenaiStrOpt(ti.Desc),
		Parameters:  params,
	}, nil
}

func fromFunctionTools(tools []responses.ToolUnion) ([]*schema.ToolInfo, error) {
	ret := make([]*schema.ToolInfo, 0, len(tools))
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}

		b, err := sonic.Marshal(t.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON schema from function tool: %w", err)
		}
		params := &jsonschema.Schema{}
		err = sonic.Unmarshal(b, &params)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON schema from function tool: %w", err)
		}

		ret = append(ret, &schema.ToolInfo{
			Name:        t.Name,
			Desc:        t.Description,
			ParamsOneOf: schema.NewParamsOneOfByJSONSchema(params),
		})
	}
	return ret, nil
}

func toDeferredFunctionTools(tools []*schema.ToolInfo) ([]responses.ToolUnionParam, error) {
	toolParams, err := toFunctionTools(tools)
	if err != nil {
		return nil, err
	}
	for _, toolParam := range toolParams {
		if toolParam.OfFunction != nil {
			toolParam.OfFunction.DeferLoading = param.NewOpt(true)
		}
	}
	return toolParams, nil
}

func toServerTools(serverTools []*ResponsesServerToolConfig) ([]responses.ToolUnionParam, error) {
	tools := make([]responses.ToolUnionParam, len(serverTools))

	for i := range serverTools {
		ti := serverTools[i]
		switch {
		case ti.WebSearch != nil:
			tools[i] = responses.ToolUnionParam{
				OfWebSearch: ti.WebSearch,
			}
		case ti.FileSearch != nil:
			tools[i] = responses.ToolUnionParam{
				OfFileSearch: ti.FileSearch,
			}
		case ti.CodeInterpreter != nil:
			tools[i] = responses.ToolUnionParam{
				OfCodeInterpreter: ti.CodeInterpreter,
			}
		case ti.Shell != nil:
			tools[i] = responses.ToolUnionParam{
				OfShell: ti.Shell,
			}
		default:
			return nil, fmt.Errorf("unknown server tool type")
		}
	}

	return tools, nil
}

func (m *ResponsesModel) getOptions(opts []model.Option) (*model.Options, *options, error) {
	commonOpts := model.GetCommonOptions(&model.Options{
		Temperature:   m.temperature,
		Model:         &m.model,
		TopP:          m.topP,
		MaxTokens:     m.maxTokens,
		Tools:         nil,
		DeferredTools: nil,
	}, opts...)

	specOptions := model.GetImplSpecificOptions(&options{
		reasoning:         m.reasoning,
		store:             m.store,
		text:              m.text,
		maxToolCalls:      m.maxToolCalls,
		parallelToolCalls: m.parallelToolCalls,
		truncation:        m.truncation,
		customHeaders:     m.customHeader,
	}, opts...)

	err := m.checkOptions(commonOpts)
	if err != nil {
		return commonOpts, specOptions, err
	}

	return commonOpts, specOptions, nil
}

func (m *ResponsesModel) checkOptions(mOpts *model.Options) error {
	if mOpts.Stop != nil {
		return fmt.Errorf("'Stop' option is not supported")
	}
	if mOpts.ToolChoice != nil {
		return fmt.Errorf("'ToolChoice' option is not supported")
	}
	return nil
}

func (m *ResponsesModel) genRequestAndOptions(in []*schema.AgenticMessage, options *model.Options,
	specOptions *options) (req *responses.ResponseNewParams, reqOpts []option.RequestOption, err error) {

	req = &responses.ResponseNewParams{}

	err = m.prePopulateConfig(req, options, specOptions)
	if err != nil {
		return req, nil, fmt.Errorf("failed to pre-populate config: %w", err)
	}

	in, err = m.populateCache(in, req, specOptions)
	if err != nil {
		return req, nil, fmt.Errorf("failed to populate cache: %w", err)
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

func (m *ResponsesModel) prePopulateConfig(responseReq *responses.ResponseNewParams, options *model.Options,
	specOptions *options) error {

	// instance configuration
	if m.serviceTier != nil {
		responseReq.ServiceTier = *m.serviceTier
	}
	responseReq.Include = m.include
	if m.promptCacheRetention != nil {
		responseReq.PromptCacheRetention = *m.promptCacheRetention
	}

	// options configuration
	if options.TopP != nil {
		responseReq.TopP = param.NewOpt(float64(*options.TopP))
	}
	if options.Temperature != nil {
		responseReq.Temperature = param.NewOpt(float64(*options.Temperature))
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
	if specOptions.truncation != nil {
		responseReq.Truncation = *specOptions.truncation
	}

	return nil
}

// populateCache resolves the PreviousResponseID for cache continuation.
// Priority: auto-discovered response ID in messages > WithHeadPreviousResponseID option > none.
// When enableAutoCache is true and an explicit WithResponsesStore(false) is not set, Store is forced to true.
func (m *ResponsesModel) populateCache(in []*schema.AgenticMessage, responseReq *responses.ResponseNewParams,
	specOpts *options) ([]*schema.AgenticMessage, error) {

	var (
		enableCache = m.enableAutoCache
		headRespID  = specOpts.headPreviousResponseID
	)

	var (
		preRespID *string
		inputIdx  int
	)

	if enableCache {
		for i := len(in) - 1; i >= 0; i-- {
			msg := in[i]
			if msg.Extra == nil {
				continue
			}
			if isAutoCached, ok := msg.Extra[keyOfResponseAutoCached].(bool); !ok || !isAutoCached {
				continue
			}
			if msg.ResponseMeta == nil || msg.ResponseMeta.OpenAIExtension == nil {
				continue
			}
			if msg.ResponseMeta.OpenAIExtension.ID == "" {
				continue
			}
			inputIdx = i
			preRespID = &msg.ResponseMeta.OpenAIExtension.ID
			break
		}
	}

	if preRespID != nil {
		if inputIdx+1 >= len(in) {
			return in, fmt.Errorf("incremental input not found after response ID")
		}
		in = in[inputIdx+1:]
	}

	// ResponseID has a higher priority than HeadPreviousResponseID
	if preRespID == nil || *preRespID == "" {
		preRespID = headRespID
	}

	if preRespID != nil && *preRespID != "" {
		responseReq.PreviousResponseID = param.NewOpt(*preRespID)
	}

	if enableCache && specOpts.store == nil {
		responseReq.Store = param.NewOpt(true)
	}

	return in, nil
}

func (m *ResponsesModel) populateInput(in []*schema.AgenticMessage, responseReq *responses.ResponseNewParams) (err error) {
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

func (m *ResponsesModel) populateToolChoice(responseReq *responses.ResponseNewParams, options *model.Options) (err error) {
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
		return fmt.Errorf("invalid tool choice: %s", toolChoice.Type)
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

func (m *ResponsesModel) populateTools(responseReq *responses.ResponseNewParams, options *model.Options, specOptions *options) (err error) {
	var functionTools []responses.ToolUnionParam
	if options.Tools != nil {
		functionTools, err = toFunctionTools(options.Tools)
		if err != nil {
			return err
		}
	}

	if len(options.DeferredTools) > 0 {
		var deferredTools []responses.ToolUnionParam
		deferredTools, err = toDeferredFunctionTools(options.DeferredTools)
		if err != nil {
			return err
		}
		functionTools = append(functionTools, deferredTools...)
		if options.ToolSearchTool == nil {
			functionTools = append(functionTools, responses.ToolUnionParam{
				OfToolSearch: &responses.ToolSearchToolParam{}, // add hosted tool search automatically if deferred tools has been set
			})
		}
	}

	if options.ToolSearchTool != nil {
		var ft *responses.FunctionToolParam
		ft, err = toFunctionTool(options.ToolSearchTool)
		if err != nil {
			return fmt.Errorf("failed to convert tool search tool: %w", err)
		}
		functionTools = append(functionTools, responses.ToolUnionParam{
			OfToolSearch: &responses.ToolSearchToolParam{
				Description: ft.Description,
				Parameters:  ft.Parameters,
				Execution:   responses.ToolSearchToolExecutionClient,
			},
		})
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
